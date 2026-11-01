package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"time"

	"fastledger/internal/config"
)

var (
	ErrOCRRateLimited   = errors.New("ocr rate limited")
	ErrOCRUnavailable   = errors.New("ocr backend unavailable")
	ErrOCRImageTooLarge = errors.New("ocr image too large")
	ErrOCRInvalid       = errors.New("ocr invalid input")
)

type OCRLine struct {
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
}
type OCRRecognizeResult struct {
	Engine string    `json:"engine"`
	Lines  []OCRLine `json:"lines"`
}
type OCRCandidate struct {
	AmountCents  int64   `json:"amountCents"`
	Direction    string  `json:"direction"`
	OccurredDate string  `json:"occurredDate"`
	Description  string  `json:"description"`
	Confidence   float64 `json:"confidence"`
}
type OCRParseResult struct {
	Candidates []OCRCandidate `json:"candidates"`
}
type OCRService struct {
	fallbackURL   string
	client        *http.Client
	maxImageBytes int64
	ratePerMin    int
	limiter       *Limiter
	// LLM 文本兜底（WBS 5.8）：默认关闭（llmURL 空），独立配额桶
	llmURL        string
	llmKey        string
	llmModel      string
	llmClient     *http.Client
	llmRatePerMin int
	llmLimiter    *Limiter
}

func NewOCRService(cfg *config.Config) *OCRService {
	timeout, maxBytes, rate, endpoint := 8000, int64(10*1024*1024), 10, ""
	if cfg != nil {
		if cfg.OCRTimeoutMS > 0 {
			timeout = cfg.OCRTimeoutMS
		}
		if cfg.OCRMaxImageBytes > 0 {
			maxBytes = cfg.OCRMaxImageBytes
		}
		if cfg.OCRRatePerMin > 0 {
			rate = cfg.OCRRatePerMin
		}
		endpoint = strings.TrimSpace(cfg.OCRFallbackURL)
	}
	llmURL, llmKey, llmModel, llmClient, llmRate := wireLLM(cfg)
	return &OCRService{
		fallbackURL: endpoint,
		client: &http.Client{Timeout: time.Duration(timeout) * time.Millisecond,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }},
		maxImageBytes: maxBytes, ratePerMin: rate, limiter: NewLimiter(),
		llmURL: llmURL, llmKey: llmKey, llmModel: llmModel,
		llmClient: llmClient, llmRatePerMin: llmRate, llmLimiter: NewLimiter(),
	}
}

func (s *OCRService) allow(uid int64) error {
	if uid <= 0 {
		return ErrOCRInvalid
	}
	if !s.limiter.Allow(fmt.Sprintf("ocr:%d", uid), s.ratePerMin, time.Minute) {
		return ErrOCRRateLimited
	}
	return nil
}

// ocrDownstreamEngine Python 兜底服务响应中的 engine 标识（契约见 docs/FastLedger-OCR双模式部署与切换契约.md）。
// 外部 API 对客户端统一返回 engine="server"，该值只用于校验下游响应确实来自 rapidocr_onnxruntime。
const ocrDownstreamEngine = "rapidocr_onnxruntime"

// sniffImageMIME 依内容魔数（而非可伪造的请求头）判定图片 MIME。
// 只放行下游 Python 白名单接受且 Pillow 能解码的类型；其它一律拒绝（fail-closed）。
func sniffImageMIME(data []byte) string {
	switch {
	case len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}):
		return "image/png"
	case len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF:
		return "image/jpeg"
	case len(data) >= 12 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")):
		return "image/webp"
	case len(data) >= 2 && data[0] == 'B' && data[1] == 'M':
		return "image/bmp"
	case len(data) >= 4 && ((data[0] == 'I' && data[1] == 'I') || (data[0] == 'M' && data[1] == 'M')):
		return "image/tiff"
	default:
		return ""
	}
}

func (s *OCRService) Recognize(ctx context.Context, uid int64, image []byte, _ string) (*OCRRecognizeResult, error) {
	if len(image) == 0 || int64(len(image)) > s.maxImageBytes {
		return nil, ErrOCRImageTooLarge
	}
	// MIME 必须来自内容嗅探：客户端 multipart 头完全可伪造（QA Round1 P1）。
	contentType := sniffImageMIME(image)
	if contentType == "" {
		return nil, ErrOCRInvalid
	}
	// 配额放在校验之后：垃圾请求根本不会触达 CPU 密集的下游推理，不应消耗配额。
	if err := s.allow(uid); err != nil {
		return nil, err
	}
	if s.fallbackURL == "" {
		return nil, ErrOCRUnavailable
	}
	u, err := url.Parse(s.fallbackURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, ErrOCRUnavailable
	}
	endpoint := strings.TrimRight(s.fallbackURL, "/") + "/ocr"
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	// 转发 part 的 Content-Type 用嗅探结果——Python 白名单只收 image/*，
	// 固定 octet-stream 会被 415 拒掉（QA Round1 P1 互操作阻断）。
	part, err := mw.CreatePart(textproto.MIMEHeader{
		"Content-Disposition": {`form-data; name="image"; filename="upload"`},
		"Content-Type":        {contentType},
	})
	if err != nil {
		return nil, ErrOCRUnavailable
	}
	if _, err = part.Write(image); err != nil {
		return nil, ErrOCRUnavailable
	}
	if err = mw.Close(); err != nil {
		return nil, ErrOCRUnavailable
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return nil, ErrOCRUnavailable
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, ErrOCRUnavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, ErrOCRUnavailable
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil || len(data) > 1<<20 {
		return nil, ErrOCRUnavailable
	}
	// 契约：Python 返回 {"engine":"rapidocr_onnxruntime","lines":[...]}。
	// 保留 DisallowUnknownFields（防字段漂移），但必须显式接收 engine 并校验值——
	// 否则每个成功响应都会被 strict decoder 拒成 503（QA Round1 P1 互操作阻断）。
	var payload struct {
		Engine string    `json:"engine"`
		Lines  []OCRLine `json:"lines"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil || payload.Lines == nil {
		return nil, ErrOCRUnavailable
	}
	if payload.Engine != ocrDownstreamEngine {
		return nil, ErrOCRUnavailable
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, ErrOCRUnavailable
	}
	for _, line := range payload.Lines {
		if !isFinite(line.Confidence) || line.Confidence < 0 || line.Confidence > 1 {
			return nil, ErrOCRUnavailable
		}
	}
	return &OCRRecognizeResult{Engine: "server", Lines: payload.Lines}, nil
}

func (s *OCRService) Parse(ctx context.Context, uid int64, text string) (*OCRParseResult, error) {
	if err := s.allow(uid); err != nil {
		return nil, err
	}
	if len([]rune(text)) > 20000 {
		return nil, ErrOCRInvalid
	}
	return &OCRParseResult{Candidates: parseOCRText(text)}, nil
}

func isFinite(f float64) bool { return !math.IsNaN(f) && !math.IsInf(f, 0) }
