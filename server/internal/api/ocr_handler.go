package api

import (
	"errors"
	"io"
	"net/http"

	"fastledger/internal/service"
	"github.com/gin-gonic/gin"
)

type ocrHandler struct {
	svc      *service.OCRService
	maxBytes int64
}
type parseOCRRequest struct {
	Text string `json:"text"`
}

func (h *ocrHandler) recognize(c *gin.Context) {
	// Multipart boundaries and headers add overhead; cap body above the image limit,
	// then enforce the exact image byte limit after parsing the image field.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.maxBytes+(64<<10))
	if err := c.Request.ParseMultipartForm(1 << 20); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			fail(c, http.StatusRequestEntityTooLarge, "图片请求过大")
		} else {
			fail(c, http.StatusBadRequest, "multipart 请求格式错误")
		}
		return
	}
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}
	if h.svc == nil {
		fail(c, http.StatusServiceUnavailable, "OCR 服务暂时不可用，请稍后重试")
		return
	}
	file, header, err := c.Request.FormFile("image")
	if err != nil {
		fail(c, http.StatusBadRequest, "请上传图片文件")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, h.maxBytes+1))
	if err != nil {
		fail(c, http.StatusBadRequest, "图片读取失败")
		return
	}
	if int64(len(data)) > h.maxBytes {
		fail(c, http.StatusRequestEntityTooLarge, "图片超过大小限制")
		return
	}
	result, err := h.svc.Recognize(c.Request.Context(), currentUID(c), data, header.Header.Get("Content-Type"))
	if err != nil {
		h.writeError(c, err)
		return
	}
	ok(c, result)
}
func (h *ocrHandler) parse(c *gin.Context) {
	if h.svc == nil {
		fail(c, http.StatusServiceUnavailable, "OCR 服务暂时不可用，请稍后重试")
		return
	}
	var req parseOCRRequest
	if !bindJSON(c, &req) {
		return
	}
	result, err := h.svc.Parse(c.Request.Context(), currentUID(c), req.Text)
	if err != nil {
		h.writeError(c, err)
		return
	}
	ok(c, result)
}

// llmParseRequest WBS 5.8：云端 LLM 文本兜底。consent 必须显式 true（双侧开关的用户侧）。
type llmParseRequest struct {
	Text    string `json:"text"`
	Consent bool   `json:"consent"`
}

func (h *ocrHandler) llmParse(c *gin.Context) {
	if h.svc == nil {
		fail(c, http.StatusServiceUnavailable, "OCR 服务暂时不可用，请稍后重试")
		return
	}
	var req llmParseRequest
	if !bindJSON(c, &req) {
		return
	}
	result, err := h.svc.LLMParse(c.Request.Context(), currentUID(c), req.Text, req.Consent)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrLLMConsentRequired):
			fail(c, http.StatusBadRequest, "使用云端文本兜底需要您明示同意（consent=true）；文本脱敏后仅传给您配置的 LLM 服务")
		case errors.Is(err, service.ErrLLMDisabled):
			fail(c, http.StatusServiceUnavailable, "云端文本兜底未启用（服务端未配置）")
		case errors.Is(err, service.ErrOCRRateLimited):
			fail(c, http.StatusTooManyRequests, "请求过于频繁，请稍后再试")
		case errors.Is(err, service.ErrOCRInvalid):
			fail(c, http.StatusBadRequest, "请求参数非法")
		default:
			fail(c, http.StatusServiceUnavailable, "云端文本兜底暂时不可用，请稍后重试或改用手动记账")
		}
		return
	}
	ok(c, result)
}
func (h *ocrHandler) writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrOCRRateLimited):
		fail(c, http.StatusTooManyRequests, "OCR 请求过于频繁，请稍后再试")
	case errors.Is(err, service.ErrOCRImageTooLarge):
		fail(c, http.StatusRequestEntityTooLarge, "图片超过大小限制")
	case errors.Is(err, service.ErrOCRInvalid):
		fail(c, http.StatusBadRequest, "请求参数非法")
	default:
		fail(c, http.StatusServiceUnavailable, "OCR 服务暂时不可用，请稍后重试")
	}
}
func ocrImageLimit(bytes int64) int64 {
	if bytes <= 0 {
		return 10 * 1024 * 1024
	}
	return bytes
}
