package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"fastledger/internal/config"
)

// WBS 5.8 云端 LLM 文本兜底。
//
// 契约要点（docs/FastLedger-OCR双模式部署与切换契约.md 同批补充）：
//   - 默认关闭：EBK_LLM_BASE_URL 为空即禁用；配置则 key/model 必须齐全（config fail-fast）。
//   - 双侧开关：服务端配置是第一道；每次调用还必须由请求体显式 consent=true。
//   - 仅传文本：本路径只接收/转发文本，不存在图片通道；转发前做敏感数字串脱敏。
//   - 不自动入账：只产出候选，字段校验全部服务端复核，绝不信任 LLM 输出的形状。
//   - 独立配额：llm:<uid> 每分钟桶，与 ocr:<uid> 完全隔离。

var (
	ErrLLMDisabled        = errors.New("llm fallback disabled")
	ErrLLMConsentRequired = errors.New("llm fallback consent required")
)

// maxLLMCandidates LLM 候选数量上界；超出直接丢弃，防提示注入撑爆响应。
const maxLLMCandidates = 20

// sensitiveDigits 手机号（11）/银行卡（16-19）/身份证数字段（17-18）等纯数字串的脱敏阈值。
// 金额（含小数点/千分位逗号/¥ 前缀）不会被整段匹配；日期时间串最长 14 位——
// 因此阈值取 11 会误伤 "20261101120000" 这类时间戳，可接受：脱敏宁可多不可漏。
var sensitiveDigits = regexp.MustCompile(`[0-9]{11,}`)

// maskSensitiveText 出站文本脱敏：≥11 位纯数字串替换为 前3 + **** + 后4。
// 身份证末位 X 情形数字段长 17，同样命中。金额不受影响（带分隔符，非 11 位连续数字）。
func maskSensitiveText(text string) string {
	return sensitiveDigits.ReplaceAllStringFunc(text, func(s string) string {
		if len(s) <= 7 {
			return s
		}
		return s[:3] + "****" + s[len(s)-4:]
	})
}

// llmSystemPrompt 约束 LLM 只输出 JSON 候选数组；金额必须整数分。
const llmSystemPrompt = `你是记账助手。从用户提供的支付/账单文本中提取交易候选。只输出 JSON，格式：` +
	`{"candidates":[{"amountCents":整数分,"direction":"income"或"expense","occurredDate":"YYYY-MM-DD","description":"简短描述","confidence":0到1}]}` +
	`。规则：金额一律换算为整数分（不用小数）；方向不确定就不输出该候选；日期不确定用空字符串；` +
	`不要编造、不要猜测、不要输出 JSON 以外的任何内容。`

type llmChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type llmChatRequest struct {
	Model       string           `json:"model"`
	Messages    []llmChatMessage `json:"messages"`
	Temperature float64          `json:"temperature"`
	Stream      bool             `json:"stream"`
}

// llmChatResponse OpenAI 兼容响应只取所需字段；顶层字段繁多，不做 DisallowUnknownFields，
// content 的结构校验在 extractLLMCandidates 里严格进行。
type llmChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// LLMParse 云端 LLM 文本兜底：脱敏 → OpenAI 兼容 chat/completions → 服务端逐字段复核。
// 返回候选可能为空（LLM 没把握时输出空数组，与 parse 的"不猜"口径一致）。
func (s *OCRService) LLMParse(ctx context.Context, uid int64, text string, consent bool) (*OCRParseResult, error) {
	if !consent {
		return nil, ErrLLMConsentRequired
	}
	if uid <= 0 {
		return nil, ErrOCRInvalid
	}
	if s.llmURL == "" || s.llmKey == "" || s.llmModel == "" {
		return nil, ErrLLMDisabled
	}
	if !s.llmLimiter.Allow(fmt.Sprintf("llm:%d", uid), s.llmRatePerMin, time.Minute) {
		return nil, ErrOCRRateLimited
	}
	runes := []rune(text)
	if len(runes) == 0 || len(runes) > 20000 {
		return nil, ErrOCRInvalid
	}

	u, err := url.Parse(s.llmURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, ErrLLMDisabled
	}
	endpoint := strings.TrimRight(s.llmURL, "/") + "/chat/completions"

	reqBody, err := json.Marshal(llmChatRequest{
		Model: s.llmModel,
		Messages: []llmChatMessage{
			{Role: "system", Content: llmSystemPrompt},
			{Role: "user", Content: maskSensitiveText(text)},
		},
		Temperature: 0,
		Stream:      false,
	})
	if err != nil {
		return nil, ErrLLMUnavailable()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, ErrLLMUnavailable()
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.llmKey)
	resp, err := s.llmClient.Do(req)
	if err != nil {
		return nil, ErrLLMUnavailable()
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, ErrLLMUnavailable()
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil || len(data) > 1<<20 {
		return nil, ErrLLMUnavailable()
	}
	var chat llmChatResponse
	if err := json.Unmarshal(data, &chat); err != nil || len(chat.Choices) == 0 {
		return nil, ErrLLMUnavailable()
	}
	content := chat.Choices[0].Message.Content
	candidates, err := extractLLMCandidates(content)
	if err != nil {
		return nil, ErrLLMUnavailable()
	}
	return &OCRParseResult{Candidates: candidates}, nil
}

// ErrLLMUnavailable 下游 LLM 异常的统一错误（与 OCR 的 503 语义同型，但 handler 文案不同）。
func ErrLLMUnavailable() error { return errors.New("llm fallback unavailable") }

// extractLLMCandidates 从 LLM 返回的自由文本中提取 JSON 并逐候选复核。
// 任何字段非法只丢弃该候选（不猜、不修正、不取绝对值），整体非 JSON 才算失败。
func extractLLMCandidates(content string) ([]OCRCandidate, error) {
	start := strings.IndexByte(content, '{')
	end := strings.LastIndexByte(content, '}')
	if start < 0 || end <= start {
		return nil, errors.New("llm content has no json object")
	}
	var parsed struct {
		Candidates []struct {
			AmountCents  int64   `json:"amountCents"`
			Direction    string  `json:"direction"`
			OccurredDate string  `json:"occurredDate"`
			Description  string  `json:"description"`
			Confidence   float64 `json:"confidence"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal([]byte(content[start:end+1]), &parsed); err != nil {
		return nil, err
	}
	out := make([]OCRCandidate, 0, len(parsed.Candidates))
	for _, c := range parsed.Candidates {
		if len(out) >= maxLLMCandidates {
			break
		}
		if c.AmountCents <= 0 || c.AmountCents > maxAmountCents {
			continue
		}
		if c.Direction != "income" && c.Direction != "expense" {
			continue
		}
		if c.OccurredDate != "" {
			if _, err := time.Parse("2006-01-02", c.OccurredDate); err != nil {
				continue
			}
		}
		if !isFinite(c.Confidence) || c.Confidence < 0 || c.Confidence > 1 {
			continue
		}
		desc := strings.TrimSpace(c.Description)
		if runes := []rune(desc); len(runes) > 255 {
			desc = string(runes[:255])
		}
		out = append(out, OCRCandidate{
			AmountCents: c.AmountCents, Direction: c.Direction,
			OccurredDate: c.OccurredDate, Description: desc, Confidence: c.Confidence,
		})
	}
	return out, nil
}

// wireLLM 从配置装配 LLM 兜底参数（NewOCRService 内调用）。
func wireLLM(cfg *config.Config) (baseURL, key, model string, client *http.Client, rate int) {
	baseURL, key, model, rate = "", "", "", 5
	timeout := 15000
	if cfg == nil {
		return
	}
	if v := strings.TrimSpace(cfg.LLMBaseURL); v != "" {
		baseURL = v
	}
	if v := strings.TrimSpace(cfg.LLMAPIKey); v != "" {
		key = v
	}
	if v := strings.TrimSpace(cfg.LLMModel); v != "" {
		model = v
	}
	if cfg.LLMTimeoutMS > 0 {
		timeout = cfg.LLMTimeoutMS
	}
	if cfg.LLMRatePerMin > 0 {
		rate = cfg.LLMRatePerMin
	}
	client = &http.Client{Timeout: time.Duration(timeout) * time.Millisecond,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	return
}
