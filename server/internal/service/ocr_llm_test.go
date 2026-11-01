package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fastledger/internal/config"
)

// TestMaskSensitiveText 出站脱敏：≥11 位数字串掩码，金额与短数字不受影响。
func TestMaskSensitiveText(t *testing.T) {
	cases := []struct{ in, want string }{
		{"尾号1234", "尾号1234"},
		{"金额12345.67元", "金额12345.67元"},                  // 带小数点，非 11 位连续数字
		{"收款1,234.56", "收款1,234.56"},                    // 千分位
		{"手机号13812345678已到账", "手机号138****5678已到账"},      // 11 位手机号
		{"卡号6222020200112233456支付", "卡号622****3456支付"},  // 19 位卡号
		{"身份证110101199001011234付款", "身份证110****1234付款"}, // 18 位证件号
		{"2026-11-01 12:00", "2026-11-01 12:00"},        // 日期时间串被分隔符打断
	}
	for _, c := range cases {
		if got := maskSensitiveText(c.in); got != c.want {
			t.Errorf("maskSensitiveText(%q)=%q, want %q", c.in, got, c.want)
		}
	}
}

// llmStub 构造一个 OpenAI 兼容的假 LLM 服务，content 为 LLM "输出"的文本。
func llmStub(t *testing.T, content string, capture *map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, _ := io.ReadAll(r.Body)
		if capture != nil {
			m := map[string]any{}
			_ = json.Unmarshal(body, &m)
			*capture = m
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		resp := map[string]any{
			"id": "chatcmpl-x", "object": "chat.completion", "created": 1, "model": "test-model",
			"choices": []map[string]any{{"index": 0, "finish_reason": "stop",
				"message": map[string]any{"role": "assistant", "content": content}}},
			"usage": map[string]any{"total_tokens": 1},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func llmSvc(t *testing.T, srvURL string) *OCRService {
	t.Helper()
	return NewOCRService(&config.Config{
		LLMBaseURL: srvURL, LLMAPIKey: "test-key", LLMModel: "test-model", LLMRatePerMin: 100,
	})
}

func TestLLMParseHappyPath(t *testing.T) {
	var captured map[string]any
	content := "```json\n{\"candidates\":[{\"amountCents\":123456,\"direction\":\"expense\",\"occurredDate\":\"2026-11-01\",\"description\":\"午餐\",\"confidence\":0.9}]}\n```"
	srv := llmStub(t, content, &captured)
	defer srv.Close()
	svc := llmSvc(t, srv.URL)

	res, err := svc.LLMParse(context.Background(), 7, "微信支付 支出¥1,234.56 手机号13812345678", true)
	if err != nil {
		t.Fatalf("不应报错：%v", err)
	}
	if len(res.Candidates) != 1 || res.Candidates[0].AmountCents != 123456 ||
		res.Candidates[0].Direction != "expense" || res.Candidates[0].OccurredDate != "2026-11-01" {
		t.Fatalf("候选不符：%+v", res.Candidates)
	}
	// 仅传文本且已脱敏：手机号必须被掩码后才出站
	userMsg, _ := captured["messages"].([]any)[1].(map[string]any)["content"].(string)
	if !strings.Contains(userMsg, "138****5678") || strings.Contains(userMsg, "13812345678") {
		t.Fatalf("出站文本未脱敏：%q", userMsg)
	}
	if captured["model"] != "test-model" {
		t.Fatalf("model 传递错误：%v", captured["model"])
	}
}

func TestLLMParseConsentRequired(t *testing.T) {
	srv := llmStub(t, "{}", nil)
	defer srv.Close()
	svc := llmSvc(t, srv.URL)
	if _, err := svc.LLMParse(context.Background(), 7, "text", false); !errors.Is(err, ErrLLMConsentRequired) {
		t.Fatalf("无 consent 应拒绝，实际 %v", err)
	}
}

func TestLLMParseDisabled(t *testing.T) {
	svc := NewOCRService(&config.Config{})
	if _, err := svc.LLMParse(context.Background(), 7, "text", true); !errors.Is(err, ErrLLMDisabled) {
		t.Fatalf("未配置应 ErrLLMDisabled，实际 %v", err)
	}
}

func TestLLMParseUpstreamFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	svc := llmSvc(t, srv.URL)
	if _, err := svc.LLMParse(context.Background(), 7, "text", true); err == nil || errors.Is(err, ErrLLMConsentRequired) {
		t.Fatalf("上游 5xx 应报不可用，实际 %v", err)
	}
	// LLM 输出完全非 JSON → 失败
	srv2 := llmStub(t, "抱歉我无法解析", nil)
	defer srv2.Close()
	svc2 := llmSvc(t, srv2.URL)
	if _, err := svc2.LLMParse(context.Background(), 7, "text", true); err == nil {
		t.Fatal("非 JSON 输出应失败")
	}
}

func TestLLMParseCandidateValidation(t *testing.T) {
	// 6 个候选：合法 1 + 非法金额 2（负数/超上界）+ 非法方向 + 非法日期 + 非法置信度
	content := `{"candidates":[
		{"amountCents":500,"direction":"income","occurredDate":"","description":"ok","confidence":0.5},
		{"amountCents":-100,"direction":"expense","confidence":0.5},
		{"amountCents":99999999999,"direction":"expense","confidence":0.5},
		{"amountCents":500,"direction":"transfer","confidence":0.5},
		{"amountCents":500,"direction":"expense","occurredDate":"2026-13-40","confidence":0.5},
		{"amountCents":500,"direction":"expense","confidence":2.5}
	]}`
	srv := llmStub(t, content, nil)
	defer srv.Close()
	svc := llmSvc(t, srv.URL)
	res, err := svc.LLMParse(context.Background(), 7, "text", true)
	if err != nil {
		t.Fatalf("不应报错：%v", err)
	}
	if len(res.Candidates) != 1 || res.Candidates[0].Description != "ok" {
		t.Fatalf("只应保留 1 个合法候选：%+v", res.Candidates)
	}
}

func TestLLMParseRateLimit(t *testing.T) {
	srv := llmStub(t, `{"candidates":[]}`, nil)
	defer srv.Close()
	svc := NewOCRService(&config.Config{
		LLMBaseURL: srv.URL, LLMAPIKey: "test-key", LLMModel: "m", LLMRatePerMin: 2,
	})
	for i := 0; i < 2; i++ {
		if _, err := svc.LLMParse(context.Background(), 9, "text", true); err != nil {
			t.Fatalf("前 2 次不应限流：%v", err)
		}
	}
	if _, err := svc.LLMParse(context.Background(), 9, "text", true); !errors.Is(err, ErrOCRRateLimited) {
		t.Fatalf("第 3 次应 429，实际 %v", err)
	}
	// 与 ocr:<uid> 桶隔离：OCR parse 不受 LLM 消耗影响
	if _, err := svc.Parse(context.Background(), 9, "支出12.34元"); err != nil {
		t.Fatalf("LLM 配额不应影响 OCR 桶：%v", err)
	}
}
