package api

// R52 多端会话的 HTTP 适配层回归测试（真实 SQLite + 真实路由；需 CGO）。
//
// 覆盖（对齐 WBS 5.10 / PRD §10.10 的验收口径）：
//   - 登录落会话：列表含该会话、标记 current、User-Agent 透传；
//   - 撤销后 HTTP 401 且文案可区分（"会话已撤销" ≠ 普通"登录已失效"）；
//   - 跨用户撤销 404，且不泄露会话存在性；
//   - 撤销当前会话允许（等价本机登出），重新登录恢复；
//   - revoke-others 保留当前会话、撤销其它设备；
//   - WS：已撤销令牌握手被拒；在线连接被 4003 主动断开。
//
// 本文件与 internal/service/session_test.go 分层：这里验证"HTTP 契约 + WS 端到端"，
// 服务层数据行为在 service 包内覆盖。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"fastledger/internal/config"
	"fastledger/internal/database"
	"fastledger/internal/migrate"
	"fastledger/internal/service"
	"fastledger/internal/ws"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"xorm.io/xorm"
)

// sessionResp 统一响应信封
type sessionResp struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// sessionAPIEnv 真实 SQLite + 真实路由 + 独立 Hub 的测试环境
type sessionAPIEnv struct {
	t      *testing.T
	engine *xorm.Engine
	hub    *ws.Hub
	r      *gin.Engine
	srv    *httptest.Server
	as     *service.AuthService
}

func newSessionAPIEnv(t *testing.T) *sessionAPIEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine, err := database.Init(t.TempDir() + "/session_api.db")
	if err != nil {
		t.Fatalf("database.Init 失败（CGO 是否可用？）：%v", err)
	}
	t.Cleanup(func() { _ = engine.Close() })
	if _, err := migrate.Apply(engine); err != nil {
		t.Fatalf("migrate.Apply 失败：%v", err)
	}
	as := service.NewAuthService(engine, "session-api-test-secret-0123456789abcdef", time.Hour)

	// 独立 Hub（不污染进程级 DefaultHub；同时替换全局引用供 ServeWS 使用）
	hub := ws.NewHub(func(uid, bookID int64) (bool, error) { return true, nil })
	old := ws.DefaultHub
	ws.DefaultHub = hub
	t.Cleanup(func() { ws.DefaultHub = old })

	e := &sessionAPIEnv{t: t, engine: engine, hub: hub, as: as}
	e.r = NewRouter(Deps{
		// EnableRegister=true：用例经公开注册建档（本包其余用例的既有口径）
		Config:        &config.Config{EnableRegister: true},
		AuthService:   as,
		Hub:           hub,
		SchemaVersion: 1,
	})
	e.srv = httptest.NewServer(e.r)
	t.Cleanup(e.srv.Close)
	return e
}

// register 注册一个用户（口令统一，测试专用）
func (e *sessionAPIEnv) register(name string) {
	e.t.Helper()
	w := e.postJSON("/api/v1/auth/register",
		`{"username":"`+name+`","password":"password123"}`, "", "")
	if w.Code != http.StatusOK {
		e.t.Fatalf("注册 %s 失败：%d %s", name, w.Code, w.Body.String())
	}
}

// postJSON 发起 JSON 请求（authorization / ua 可为空）
func (e *sessionAPIEnv) postJSON(path, body, authorization, ua string) *httptest.ResponseRecorder {
	e.t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if authorization != "" {
		req.Header.Set("Authorization", "Bearer "+authorization)
	}
	if ua != "" {
		req.Header.Set("User-Agent", ua)
	}
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	return w
}

// do 任意方法的 JSON 请求，返回信封
func (e *sessionAPIEnv) do(method, path, token string) sessionResp {
	e.t.Helper()
	var body *strings.Reader
	if method == http.MethodDelete || method == http.MethodGet {
		body = strings.NewReader("")
	} else {
		body = strings.NewReader("{}")
	}
	req := httptest.NewRequest(method, path, body)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	var resp sessionResp
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		e.t.Fatalf("响应不是合法 JSON：%d %s", w.Code, w.Body.String())
	}
	return resp
}

// login 走 HTTP 登录并返回令牌（ua 透传给会话记录）
func (e *sessionAPIEnv) login(name, ua string) string {
	e.t.Helper()
	w := e.postJSON("/api/v1/auth/login",
		`{"username":"`+name+`","password":"password123"}`, "", ua)
	if w.Code != http.StatusOK {
		e.t.Fatalf("登录失败：%d %s", w.Code, w.Body.String())
	}
	var resp sessionResp
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		e.t.Fatalf("登录响应解析失败：%v", err)
	}
	var data struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.Unmarshal(resp.Data, &data); err != nil || data.AccessToken == "" {
		e.t.Fatalf("登录响应缺 accessToken：%s", string(resp.Data))
	}
	return data.AccessToken
}

// listSessions 解析 GET /auth/sessions 的会话列表
func (e *sessionAPIEnv) listSessions(token string, includeRevoked bool) []sessionDTOView {
	e.t.Helper()
	path := "/api/v1/auth/sessions"
	if includeRevoked {
		path += "?includeRevoked=1"
	}
	resp := e.do(http.MethodGet, path, token)
	if resp.Code != 0 {
		e.t.Fatalf("查询会话列表失败：%d %s", resp.Code, resp.Msg)
	}
	var data struct {
		Sessions []sessionDTOView `json:"sessions"`
	}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		e.t.Fatalf("会话列表解析失败：%s", string(resp.Data))
	}
	return data.Sessions
}

// sessionDTOView 与 session_handler.go 的 sessionDTO 字段对齐（测试侧独立声明，防漂移靠断言）
type sessionDTOView struct {
	Id         int64  `json:"id"`
	Jti        string `json:"jti"`
	CreatedAt  int64  `json:"createdAt"`
	ExpiresAt  int64  `json:"expiresAt"`
	RevokedAt  int64  `json:"revokedAt"`
	LastSeenAt int64  `json:"lastSeenAt"`
	UserAgent  string `json:"userAgent"`
	Current    bool   `json:"current"`
}

// TestSessionListMarksCurrent 登录落会话 + 列表标记 current + UA 透传。
func TestSessionListMarksCurrent(t *testing.T) {
	e := newSessionAPIEnv(t)
	e.register("api-sess-user")
	tok := e.login("api-sess-user", "FastLedger-iOS/1.0")

	items := e.listSessions(tok, false)
	if len(items) != 1 {
		t.Fatalf("应有恰好 1 条会话，实际 %d：%v", len(items), items)
	}
	it := items[0]
	if !it.Current {
		t.Fatal("唯一会话应标记 current")
	}
	if it.Jti == "" || it.CreatedAt <= 0 || it.ExpiresAt <= it.CreatedAt {
		t.Fatalf("会话字段不完整：%+v", it)
	}
	if it.UserAgent != "FastLedger-iOS/1.0" {
		t.Fatalf("User-Agent 未透传：%q", it.UserAgent)
	}
	if it.RevokedAt != 0 {
		t.Fatalf("有效会话不应带撤销时刻：%+v", it)
	}
}

// TestRevokeSessionThenHTTP401 撤销后：原令牌 401 且文案可区分、有效列表清空、重登恢复。
//
// 用第二台设备（tok2）观察撤销结果——被撤销的令牌连"查列表"本身都会 401，
// 这正是要验证的强校验语义。
func TestRevokeSessionThenHTTP401(t *testing.T) {
	e := newSessionAPIEnv(t)
	e.register("api-revoke-user")
	tok1 := e.login("api-revoke-user", "UA-revoke/1.0")
	tok2 := e.login("api-revoke-user", "UA-revoke/1.0")
	jti1 := e.listSessions(tok1, false)[0].Jti

	resp := e.do(http.MethodDelete, "/api/v1/auth/sessions/"+jti1, tok1)
	if resp.Code != 0 {
		t.Fatalf("撤销应成功（允许撤销当前会话，等价本机登出）：%d %s", resp.Code, resp.Msg)
	}
	var data struct {
		Revoked bool `json:"revoked"`
	}
	_ = json.Unmarshal(resp.Data, &data)
	if !data.Revoked {
		t.Fatalf("撤销响应应标记 revoked：%s", string(resp.Data))
	}

	// 撤销后：同令牌调任意受保护接口 → 401 且文案可区分（含"会话已撤销"字样）
	w := e.authedGet(tok1)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("撤销后应 401，实际 %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "会话已撤销") {
		t.Fatalf("撤销类 401 必须可区分（应含'会话已撤销'）：%s", w.Body.String())
	}

	// 另一台设备的视角：有效列表里不应再有它；includeRevoked=1 才能看到撤销时刻
	for _, it := range e.listSessions(tok2, false) {
		if it.Jti == jti1 {
			t.Fatalf("撤销后有效列表不应包含该会话：%v", it)
		}
	}
	var sawRevoked bool
	for _, it := range e.listSessions(tok2, true) {
		if it.Jti == jti1 {
			sawRevoked = true
			if it.RevokedAt <= 0 {
				t.Fatalf("includeRevoked=1 应看到撤销时刻：%+v", it)
			}
		}
	}
	if !sawRevoked {
		t.Fatal("includeRevoked=1 的列表应包含已撤销会话")
	}

	// 重新登录恢复（新令牌可用），被撤销的旧令牌保持 401
	tok3 := e.login("api-revoke-user", "UA-revoke/1.0")
	if resp := e.do(http.MethodGet, "/api/v1/users/me", tok3); resp.Code != 0 {
		t.Fatalf("重新登录后应可用：%d %s", resp.Code, resp.Msg)
	}
	if w := e.authedGet(tok1); w.Code != http.StatusUnauthorized {
		t.Fatalf("旧令牌应保持 401，实际 %d", w.Code)
	}
}

// authedGet GET /users/me 的原始响应（供状态码断言）
func (e *sessionAPIEnv) authedGet(token string) *httptest.ResponseRecorder {
	e.t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	return w
}

// TestRevokeSessionCrossUserIs404 跨用户撤销 → 404，目标会话不受影响。
func TestRevokeSessionCrossUserIs404(t *testing.T) {
	e := newSessionAPIEnv(t)
	e.register("api-cross-a")
	e.register("api-cross-b")
	tokA := e.login("api-cross-a", "UA-A/1.0")
	tokB := e.login("api-cross-b", "UA-B/1.0")
	jtiA := e.listSessions(tokA, false)[0].Jti

	resp := e.do(http.MethodDelete, "/api/v1/auth/sessions/"+jtiA, tokB)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("跨用户撤销应 404（不区分不存在），实际 %d %s", resp.Code, resp.Msg)
	}
	// A 的会话原封不动
	if resp := e.do(http.MethodGet, "/api/v1/users/me", tokA); resp.Code != 0 {
		t.Fatalf("被针对的会话不得受影响：%d %s", resp.Code, resp.Msg)
	}
}

// TestRevokeOthersKeepsCurrent "登出其它设备"：当前会话保留，其它会话撤销。
func TestRevokeOthersKeepsCurrent(t *testing.T) {
	e := newSessionAPIEnv(t)
	e.register("api-others-user")
	tok1 := e.login("api-others-user", "UA-device1/1.0")
	tok2 := e.login("api-others-user", "UA-device2/1.0")

	resp := e.do(http.MethodDelete, "/api/v1/auth/sessions", tok1)
	if resp.Code != 0 {
		t.Fatalf("批量撤销失败：%d %s", resp.Code, resp.Msg)
	}
	var data struct {
		Revoked int `json:"revoked"`
	}
	_ = json.Unmarshal(resp.Data, &data)
	if data.Revoked != 1 {
		t.Fatalf("应恰好撤销 1 台其它设备，实际 %d（%s）", data.Revoked, string(resp.Data))
	}
	if resp := e.do(http.MethodGet, "/api/v1/users/me", tok1); resp.Code != 0 {
		t.Fatalf("当前会话应保留：%d %s", resp.Code, resp.Msg)
	}
	if w := e.authedGet(tok2); w.Code != http.StatusUnauthorized {
		t.Fatalf("其它设备应 401，实际 %d", w.Code)
	}
}

// TestRevokedTokenCannotHandshakeWS 已撤销令牌的 WS 握手必须被拒——
// 不得因"JWT 签名尚未过期"而放行（PRD §10.10 第 3 条）。
func TestRevokedTokenCannotHandshakeWS(t *testing.T) {
	e := newSessionAPIEnv(t)
	e.register("api-ws-deny")
	tok := e.login("api-ws-deny", "UA-ws/1.0")
	jti := e.listSessions(tok, false)[0].Jti

	if resp := e.do(http.MethodDelete, "/api/v1/auth/sessions/"+jti, tok); resp.Code != 0 {
		t.Fatalf("撤销失败：%d %s", resp.Code, resp.Msg)
	}
	// 握手侧：wsAuth 的会话强校验直接 401，不会升级协议
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ws?token="+tok, nil)
	e.r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("已撤销令牌的 WS 握手应 401，实际 %d", w.Code)
	}
}

// TestWSConnectionKickedOnRevoke 端到端：在线 WS 连接在会话撤销时被 4003 主动断开。
//
// 流程：登录 → 携 token 建立 WS → 通过 HTTP 撤销该会话 → WS 收到关闭码 4003 →
// 同令牌再握手被拒。这条用例钉死"撤销后 WS 连接仍存活"是缺陷（WBS 5.10 禁止项）。
func TestWSConnectionKickedOnRevoke(t *testing.T) {
	e := newSessionAPIEnv(t)
	e.register("api-ws-kick")
	tok := e.login("api-ws-kick", "UA-wskick/1.0")
	jti := e.listSessions(tok, false)[0].Jti

	wsURL := "ws" + strings.TrimPrefix(e.srv.URL, "http") + "/api/v1/ws?token=" + tok
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WS 握手失败：%v", err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	if resp := e.do(http.MethodDelete, "/api/v1/auth/sessions/"+jti, tok); resp.Code != 0 {
		t.Fatalf("撤销失败：%d %s", resp.Code, resp.Msg)
	}

	// 读循环应收到 4003 关闭帧（而不是静默断开或等到令牌过期）
	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Fatal("撤销后 WS 连接应被主动断开，却收到了正常消息")
	}
	closeErr, ok := err.(*websocket.CloseError)
	if !ok {
		t.Fatalf("应收到 CloseError，实际：%v", err)
	}
	if closeErr.Code != 4003 {
		t.Fatalf("关闭码应为 4003（session revoked），实际 %d（text=%q）", closeErr.Code, closeErr.Text)
	}

	// 同令牌再握手：401（会话已撤销）
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ws?token="+tok, nil)
	e.r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("撤销后再次握手应 401，实际 %d", w.Code)
	}
}
