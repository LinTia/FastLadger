package api

// QA 独立验收（阶段 3 适配层）—— 只补工程师用例未覆盖的契约点。
//
//  1. /healthz 必须同时回报 instanceId + restoreEpoch（客户端靠它判断"服务端是否回退过"）；
//  2. 未注入实例元信息时必须**省略**这两个字段，而不是回填空值（回填空值会让客户端误判）；
//  3. WS 鉴权：空 token / 纯空白 token / "Bearer" 空值时一律 401，不存在"无 token 也放行"的分支；
//  4. sync/pull 在服务未初始化时映射为 400 而不是 500（配置缺失不是"服务内部错误"）。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"fastledger/internal/config"
	"fastledger/internal/instance"
	"fastledger/internal/service"

	"github.com/gin-gonic/gin"
)

// TestQAS3HealthzExposesInstanceGeneration /healthz 必须回报恢复世代
func TestQAS3HealthzExposesInstanceGeneration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(Deps{
		Config:   &config.Config{},
		Instance: &instance.Meta{InstanceId: "inst-abc", RestoreEpoch: 7},
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("/healthz 应返回 200，实际 %d", w.Code)
	}
	var resp struct {
		Data map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("响应不是 JSON：%s", w.Body.String())
	}
	if got, _ := resp.Data["instanceId"].(string); got != "inst-abc" {
		t.Errorf("instanceId 应为 inst-abc，实际 %v", resp.Data["instanceId"])
	}
	if got, _ := resp.Data["restoreEpoch"].(float64); got != 7 {
		t.Errorf("restoreEpoch 应为 7，实际 %v", resp.Data["restoreEpoch"])
	}
}

// TestQAS3HealthzOmitsGenerationWhenUnknown 未注入元信息时省略字段（不回填空值）
func TestQAS3HealthzOmitsGenerationWhenUnknown(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(Deps{Config: &config.Config{}})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("/healthz 应返回 200，实际 %d", w.Code)
	}
	var resp struct {
		Data map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("响应不是 JSON：%s", w.Body.String())
	}
	for _, k := range []string{"instanceId", "restoreEpoch"} {
		if _, ok := resp.Data[k]; ok {
			t.Errorf("未注入元信息时不得回报 %q（空值会让客户端误判世代），实际 %v", k, resp.Data[k])
		}
	}
}

// TestQAS3WSAuthNeverBypasses 任何"看起来像 token 但其实是空"的输入都必须 401。
func TestQAS3WSAuthNeverBypasses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(Deps{Config: &config.Config{}})

	cases := []struct {
		name   string
		target string
		header string
	}{
		{"无凭证", "/api/v1/ws", ""},
		{"空 query token", "/api/v1/ws?token=", ""},
		{"纯空白 query token", "/api/v1/ws?token=%20%20", ""},
		{"只有 Bearer 无值", "/api/v1/ws", "Bearer"},
		{"Bearer 后空值", "/api/v1/ws", "Bearer "},
		{"Bearer 后空白", "/api/v1/ws", "Bearer    "},
		{"头为非法 scheme 且无 query", "/api/v1/ws", "Basic abc"},
	}
	for _, c := range cases {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, c.target, nil)
		if c.header != "" {
			req.Header.Set("Authorization", c.header)
		}
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s：应 401，实际 %d（body=%s）", c.name, w.Code, w.Body.String())
		}
	}
}

// TestQAS3PullUninitializedServiceIs400 同步服务未初始化 → 400（不是 500，也不是空批次假成功）
func TestQAS3PullUninitializedServiceIs400(t *testing.T) {
	// engine 未注入：Pull 必须明确报 ErrInvalidArgument，而不是返回空批次让客户端"以为已同步"
	h := &syncHandler{svc: service.NewSyncService(nil, nil, nil)}
	c, w := stage3Ctx(t, http.MethodGet, "/api/v1/books/3/sync/pull",
		"", gin.Params{{Key: "id", Value: "3"}}, 7)
	h.pull(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("服务未初始化应映射为 400，实际 %d（body=%s）", w.Code, w.Body.String())
	}
}
