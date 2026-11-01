package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fastledger/internal/config"
)

// WBS 5.8 云端 LLM 文本兜底 handler 级测试（复用 ocrE2EEnv：真实 SQLite + 真实登录）。

func TestLLMParseRouteBehindAuth(t *testing.T) {
	r := NewRouter(Deps{Config: &config.Config{}, SchemaVersion: 1})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ocr/llm-parse", nil)
	r.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("llm-parse 未带 token 应 401，实际 %d", w.Code)
	}
}

// postJSONAt 带 JWT 的 JSON POST（任意路径）。
func (e *ocrE2EEnv) postJSONAt(path, token, body string) *httptest.ResponseRecorder {
	e.t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	return w
}

func TestLLMParseConsentAndDisabled(t *testing.T) {
	e := newOCRE2EEnv(t, &config.Config{EnableRegister: true})
	token := e.token("llmuser")

	counts := func() (tx, audit int) {
		e.t.Helper()
		if _, err := e.engine.SQL("SELECT COUNT(*) FROM transactions").Get(&tx); err != nil {
			e.t.Fatalf("查询交易失败：%v", err)
		}
		if _, err := e.engine.SQL("SELECT COUNT(*) FROM audit_logs").Get(&audit); err != nil {
			e.t.Fatalf("查询审计失败：%v", err)
		}
		return
	}
	tx0, audit0 := counts()

	// 1) 缺 consent → 400（用户侧开关未打开）
	w := e.postJSONAt("/api/v1/ocr/llm-parse", token, `{"text":"支出12.34元"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("缺 consent 应 400，实际 %d %s", w.Code, w.Body.String())
	}

	// 2) consent=true 但服务端未配置 → 503 未启用（默认关闭语义）
	w = e.postJSONAt("/api/v1/ocr/llm-parse", token, `{"text":"支出12.34元","consent":true}`)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("未配置 LLM 应 503，实际 %d %s", w.Code, w.Body.String())
	}

	// 3) 零账务副作用（与 OCR 同口径）：注册登录基线之后，llm-parse 不产生交易/审计
	tx1, audit1 := counts()
	if tx1 != tx0 || audit1 != audit0 {
		t.Fatalf("llm-parse 不得写账务：transactions %d→%d audit %d→%d", tx0, tx1, audit0, audit1)
	}
}

func TestLLMParseUpstreamOK(t *testing.T) {
	// 假 OpenAI 兼容上游，验证端到端 200
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"candidates\":[{\"amountCents\":1234,\"direction\":\"expense\",\"occurredDate\":\"\",\"description\":\"测试\",\"confidence\":0.8}]}"}}]}`))
	}))
	defer upstream.Close()
	cfg := &config.Config{EnableRegister: true, LLMBaseURL: upstream.URL, LLMAPIKey: "k", LLMModel: "m"}
	e := newOCRE2EEnv(t, cfg)
	token := e.token("llmuser2")

	w := e.postJSONAt("/api/v1/ocr/llm-parse", token, `{"text":"支出12.34元","consent":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("应 200，实际 %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"amountCents":1234`) {
		t.Fatalf("响应缺候选：%s", w.Body.String())
	}
}
