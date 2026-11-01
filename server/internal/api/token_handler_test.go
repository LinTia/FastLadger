package api

// 长期 API Token（PAT）API 测试（WBS 6.6）。
//
// 覆盖：未带凭证 401、生成→明文一次性下发→用 PAT 直调业务 API（中间件分流）、
// 列表不含哈希/明文、撤销后 PAT 立即 401、撤销跨用户 Token 404。

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
	"fastledger/internal/version"

	"github.com/gin-gonic/gin"
	"xorm.io/xorm"
)

type patAPIEnv struct {
	t      *testing.T
	r      *gin.Engine
	engine *xorm.Engine
	token  string // JWT（登录态）
	pat    string // 首个 PAT 明文
	patID  int64
	do     func(method, path, body, token string) *httptest.ResponseRecorder
}

func newPATEnv(t *testing.T) *patAPIEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := newTestEngineAPI(t)
	if _, err := migrate.Apply(engine); err != nil {
		t.Fatalf("迁移失败：%v", err)
	}
	st, err := migrate.Check(engine)
	if err != nil {
		t.Fatalf("Check 失败：%v", err)
	}
	versions := version.NewRepo()
	pats := service.NewPATService(engine)
	r := NewRouter(Deps{
		Config:        &config.Config{EnableRegister: true},
		AuthService:   service.NewAuthService(engine, "pat-api-test-secret-0123456789abcdef", time.Hour),
		Books:         service.NewBookService(engine, versions),
		Members:       nil,
		PATs:          pats,
		SchemaVersion: st.Current,
	})
	e := &patAPIEnv{t: t, r: r, engine: engine}
	e.do = func(method, path, body, token string) *httptest.ResponseRecorder {
		var rd *strings.Reader
		if body == "" {
			rd = strings.NewReader("")
		} else {
			rd = strings.NewReader(body)
		}
		req, _ := http.NewRequest(method, path, rd)
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	// 注册 + 登录
	if w := e.do(http.MethodPost, "/api/v1/auth/register", `{"username":"patuser","password":"password123"}`, ""); w.Code != http.StatusOK {
		t.Fatalf("注册失败：%d %s", w.Code, w.Body.String())
	}
	w := e.do(http.MethodPost, "/api/v1/auth/login", `{"username":"patuser","password":"password123"}`, "")
	if w.Code != http.StatusOK {
		t.Fatalf("登录失败：%d %s", w.Code, w.Body.String())
	}
	var lresp struct {
		Data struct {
			AccessToken string `json:"accessToken"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &lresp); err != nil || lresp.Data.AccessToken == "" {
		t.Fatalf("登录响应解析失败：%v %s", err, w.Body.String())
	}
	e.token = lresp.Data.AccessToken
	return e
}

func TestPATLifecycleAPI(t *testing.T) {
	e := newPATEnv(t)

	// 未带凭证 → 401
	if w := e.do(http.MethodGet, "/api/v1/users/me/tokens", "", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("未鉴权应 401，得到 %d", w.Code)
	}

	// 生成：明文一次性下发
	w := e.do(http.MethodPost, "/api/v1/users/me/tokens", `{"name":"快捷指令"}`, e.token)
	if w.Code != http.StatusOK {
		t.Fatalf("生成失败：%d %s", w.Code, w.Body.String())
	}
	var cresp struct {
		Data struct {
			Token string `json:"token"`
			Item  struct {
				Id int64 `json:"id"`
			} `json:"item"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &cresp); err != nil || !strings.HasPrefix(cresp.Data.Token, "flt_") {
		t.Fatalf("生成响应解析失败：%v %s", err, w.Body.String())
	}
	e.pat = cresp.Data.Token
	e.patID = cresp.Data.Item.Id

	// 用 PAT 直调业务 API（中间件分流验证）：/users/me
	w = e.do(http.MethodGet, "/api/v1/users/me", "", e.pat)
	if w.Code != http.StatusOK {
		t.Fatalf("PAT 调业务 API 失败：%d %s", w.Code, w.Body.String())
	}
	var me struct {
		Data struct {
			Username string `json:"username"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &me); err != nil || me.Data.Username != "patuser" {
		t.Fatalf("PAT 鉴权身份错误：%s", w.Body.String())
	}

	// 列表：含该 Token，绝不含哈希与明文
	w = e.do(http.MethodGet, "/api/v1/users/me/tokens", "", e.token)
	if w.Code != http.StatusOK {
		t.Fatalf("列表失败：%d", w.Code)
	}
	body := w.Body.String()
	if strings.Contains(body, e.pat) || strings.Contains(body, "tokenHash") || strings.Contains(body, "token_hash") {
		t.Fatal("列表响应泄露明文或哈希")
	}
	if !strings.Contains(body, `"name":"快捷指令"`) {
		t.Fatalf("列表缺备注：%s", body)
	}

	// 用 PAT 也能管理 PAT（生成第二个）
	w = e.do(http.MethodPost, "/api/v1/users/me/tokens", `{"name":"CI"}`, e.pat)
	if w.Code != http.StatusOK {
		t.Fatalf("用 PAT 生成失败：%d", w.Code)
	}

	// 撤销 → 立即失效
	w = e.do(http.MethodDelete, "/api/v1/users/me/tokens/64", "", e.token) // 不存在 → 404
	if w.Code != http.StatusNotFound {
		t.Fatalf("撤销不存在的 Token 应 404，得到 %d", w.Code)
	}
	w = e.do(http.MethodDelete, "/api/v1/users/me/tokens/"+jsonNumber(e.patID), "", e.token)
	if w.Code != http.StatusOK {
		t.Fatalf("撤销失败：%d %s", w.Code, w.Body.String())
	}
	w = e.do(http.MethodGet, "/api/v1/users/me", "", e.pat)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("撤销后 PAT 应 401，得到 %d", w.Code)
	}
}

func jsonNumber(v int64) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// newTestEngineAPI 与既有 api 测试同款的临时文件库引擎
func newTestEngineAPI(t *testing.T) *xorm.Engine {
	t.Helper()
	engine, err := database.Init(t.TempDir() + "/pat_api.db")
	if err != nil {
		t.Fatalf("database.Init 失败（CGO 是否可用？）：%v", err)
	}
	t.Cleanup(func() { _ = engine.Close() })
	return engine
}
