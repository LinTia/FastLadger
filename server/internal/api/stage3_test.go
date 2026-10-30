package api

// 阶段 3（同步 / 可见性 / WS 鉴权）的适配层测试——只覆盖不需要数据库的部分。
//
// 本机无 gcc（CGO 不可用），无法跑真实 SQLite，因此这里验证：
//  1. 新路由已接线且只注册一次（sync/pull、sync/push、visibility、ws）；
//  2. WS 鉴权在**没有 Authorization 头**时仍能通过 ?token= 进入校验（回退分支生效）；
//  3. sync/push 的整批非法（items 为空/超长）返回 400，而不是 500 或"假成功"；
//  4. 可见性请求缺字段时 400（不能把"没传"当成 false 悄悄关掉/打开开关）。
import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fastledger/internal/config"
	"fastledger/internal/service"

	"github.com/gin-gonic/gin"
)

// TestRouterRegistersStage3Routes 阶段 3 路由接线
func TestRouterRegistersStage3Routes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(Deps{Config: &config.Config{}, SchemaVersion: 3})

	got := make(map[string]int, len(r.Routes()))
	for _, ri := range r.Routes() {
		got[ri.Method+" "+ri.Path]++
	}
	for _, want := range []string{
		"GET /api/v1/books/:id/sync/pull",
		"POST /api/v1/books/:id/sync/push",
		"PUT /api/v1/books/:id/visibility",
		"GET /api/v1/ws",
	} {
		if got[want] != 1 {
			t.Errorf("路由 %s 应注册且仅注册一次，实际 %d 次", want, got[want])
		}
	}
	// WS 只能有一个入口，且不得出现"第二个创建入口"
	for _, forbidden := range []string{
		"POST /api/v1/books/:id/sync/push/batch",
		"POST /api/v1/books/:id/transactions/batch",
		"POST /api/v1/books/:id/transfer",
		"GET /api/v1/books/:id/sync/ws",
	} {
		if got[forbidden] != 0 {
			t.Errorf("禁止的路由已注册：%s", forbidden)
		}
	}
}

// TestWSAuthQueryTokenFallback WS 无 Authorization 头时必须走 ?token= 回退分支
//
// 判据：两条路径都进到"校验 token"这一步（返回"登录已失效"），
// 而完全没带凭证时返回"缺少登录凭证"。若回退分支失效，?token= 会得到"缺少登录凭证"。
func TestWSAuthQueryTokenFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// authSvc=nil：任何 token 都校验失败，但足以区分"有没有进入校验"
	r := NewRouter(Deps{Config: &config.Config{}})

	cases := []struct {
		name     string
		target   string
		header   string
		wantMsg  string
		wantCode int
	}{
		{"无任何凭证", "/api/v1/ws", "", "缺少登录凭证", http.StatusUnauthorized},
		{"仅 query token", "/api/v1/ws?token=abc", "", "登录已失效，请重新登录", http.StatusUnauthorized},
		{"仅 Authorization 头", "/api/v1/ws", "Bearer abc", "登录已失效，请重新登录", http.StatusUnauthorized},
		{"头优先于 query", "/api/v1/ws?token=from-query", "Basic abc", "登录已失效，请重新登录", http.StatusUnauthorized},
		{"空白 query token", "/api/v1/ws?token=%20", "", "缺少登录凭证", http.StatusUnauthorized},
	}
	for _, c := range cases {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, c.target, nil)
		if c.header != "" {
			req.Header.Set("Authorization", c.header)
		}
		r.ServeHTTP(w, req)

		if w.Code != c.wantCode {
			t.Errorf("%s：状态码期望 %d，实际 %d（body=%s）", c.name, c.wantCode, w.Code, w.Body.String())
			continue
		}
		var resp struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Errorf("%s：响应不是 JSON：%s", c.name, w.Body.String())
			continue
		}
		if resp.Msg != c.wantMsg {
			t.Errorf("%s：msg 期望 %q，实际 %q", c.name, c.wantMsg, resp.Msg)
		}
		if resp.Code != c.wantCode {
			t.Errorf("%s：body code 应与 HTTP 状态一致（期望 %d，实际 %d）", c.name, c.wantCode, resp.Code)
		}
	}
}

// TestBearerTokenParsing Authorization 头解析（大小写/空格/非法 scheme）
func TestBearerTokenParsing(t *testing.T) {
	cases := []struct {
		header string
		want   string
	}{
		{"Bearer abc", "abc"},
		{"bearer abc", "abc"},
		{"BEARER   abc  ", "abc"},
		{"  Bearer abc", "abc"},
		{"Bearer ", ""},
		{"abc", ""},
		{"", ""},
		{"Basic abc", ""},
		{"Bearer abc def", "abc def"},
	}
	for _, c := range cases {
		if got := bearerToken(c.header); got != c.want {
			t.Errorf("bearerToken(%q)=%q，期望 %q", c.header, got, c.want)
		}
	}
}

// stage3Ctx 构造带路径参数与认证上下文的 gin 上下文
func stage3Ctx(t *testing.T, method, target, body string, params gin.Params, uid int64) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, target, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = params
	if uid > 0 {
		c.Set(uidKey, uid)
	}
	return c, w
}

// TestPushRejectsEmptyItems push 空 items → 400（不是 500、也不是"假成功"）
func TestPushRejectsEmptyItems(t *testing.T) {
	h := &syncHandler{svc: &service.SyncService{}}
	c, w := stage3Ctx(t, http.MethodPost, "/api/v1/books/3/sync/push", `{"items":[]}`,
		gin.Params{{Key: "id", Value: "3"}}, 7)
	h.push(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("空 items 应返回 400，实际 %d（body=%s）", w.Code, w.Body.String())
	}
}

// TestPushRejectsTooManyItems 单次超长（>200）→ 400
func TestPushRejectsTooManyItems(t *testing.T) {
	var sb strings.Builder
	sb.WriteString(`{"items":[`)
	for i := 0; i < 201; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		fmt.Fprintf(&sb, `{"clientUuid":"u-%d"}`, i)
	}
	sb.WriteString(`]}`)

	h := &syncHandler{svc: &service.SyncService{}}
	c, w := stage3Ctx(t, http.MethodPost, "/api/v1/books/3/sync/push", sb.String(),
		gin.Params{{Key: "id", Value: "3"}}, 7)
	h.push(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("超长 items 应返回 400，实际 %d（body=%s）", w.Code, w.Body.String())
	}
}

// TestPushBadJSONBody 请求体不是合法 JSON → 400（bindJSON 不得吞错）
func TestPushBadJSONBody(t *testing.T) {
	h := &syncHandler{svc: &service.SyncService{}}
	c, w := stage3Ctx(t, http.MethodPost, "/api/v1/books/3/sync/push", `{"items":`,
		gin.Params{{Key: "id", Value: "3"}}, 7)
	h.push(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("非法 JSON 应返回 400，实际 %d", w.Code)
	}
}

// TestPullRejectsBadLimit pull 的 limit 非数字 → 400（解析失败不得静默当 0）
func TestPullRejectsBadLimit(t *testing.T) {
	h := &syncHandler{svc: &service.SyncService{}}
	c, w := stage3Ctx(t, http.MethodGet, "/api/v1/books/3/sync/pull?limit=abc", "",
		gin.Params{{Key: "id", Value: "3"}}, 7)
	h.pull(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("非法 limit 应返回 400，实际 %d", w.Code)
	}
}

// TestVisibilityRequiresField 可见性缺字段 → 400；非法 ID → 400
func TestVisibilityRequiresField(t *testing.T) {
	bh := &bookHandler{}

	c, w := stage3Ctx(t, http.MethodPut, "/api/v1/books/3/visibility", `{}`,
		gin.Params{{Key: "id", Value: "3"}}, 7)
	bh.setVisibility(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("缺 membersCanViewOthers 应返回 400，实际 %d（body=%s）", w.Code, w.Body.String())
	}

	c, w = stage3Ctx(t, http.MethodPut, "/api/v1/books/0/visibility", `{"membersCanViewOthers":true}`,
		gin.Params{{Key: "id", Value: "0"}}, 7)
	bh.setVisibility(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("非法账本 ID 应返回 400，实际 %d", w.Code)
	}
}
