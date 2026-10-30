package api

// GPT-R45（HTTP 适配层一侧）：注册输入错误必须是明确的 400，绝不能变成 500。
//
// 服务侧（service.normalizeAndHash）已经在【开事务之前】拦住超长用户名与超过 bcrypt
// 72 字节上限的口令；这里验证的是最后一公里：这些错误必须落到 400 分支，
// 而不是掉进 mapSvcErr 的 default 变成"服务内部错误"。
//
// 用替身注入错误的原因（不是"用 mock 替代真实性验证"）：被测对象是本文件的**错误映射**，
// 而 service 层的真实校验另有 TestR45* 系列用例覆盖（含真实 SQLite 的零落库断言）。
// 这里每一条断言（状态码 / 响应文案 / 不泄露内部细节）都按生产口径核查。

import (
	"bytes"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"fastledger/internal/model"
	"fastledger/internal/service"

	"github.com/gin-gonic/gin"
)

// stubAuthAPI 认证服务替身：注入确定性错误，用于验证 HTTP 层的错误映射。
// 同时被 auth_handler_r46_test.go 复用（同一包内共享）。
type stubAuthAPI struct {
	loginErr    error
	loginToken  string
	registerErr error
	user        *model.User
	// lastUser / lastPass 记录注册入口透传的原始入参
	lastUser string
	lastPass string
}

func (s *stubAuthAPI) Register(username, password string) (*model.User, error) {
	s.lastUser = username
	s.lastPass = password
	if s.registerErr != nil {
		return nil, s.registerErr
	}
	if s.user != nil {
		return s.user, nil
	}
	return &model.User{Id: 1, Username: username}, nil
}

func (s *stubAuthAPI) Login(username, password string) (string, error) {
	if s.loginErr != nil {
		return "", s.loginErr
	}
	if s.loginToken != "" {
		return s.loginToken, nil
	}
	return "token-xxx", nil
}

func (s *stubAuthAPI) GetByID(uid int64) (*model.User, error) {
	if s.user != nil {
		return s.user, nil
	}
	return &model.User{Id: uid, Username: "stub"}, nil
}

// newAuthTestRouter 只挂被测的两个 handler（不引入 JWT 中间件，避免无关耦合）
func newAuthTestRouter(svc authAPI, enableRegister bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := &authHandler{authSvc: svc, enableRegister: enableRegister}
	r := gin.New()
	r.POST("/api/v1/auth/register", h.register)
	r.POST("/api/v1/auth/login", h.login)
	return r
}

// postJSON 发起 JSON POST，返回响应记录器
func postJSON(t *testing.T, r *gin.Engine, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// captureLog 捕获标准日志输出（用于断言"内部错误留下了服务端日志"）
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(os.Stderr)
		log.SetFlags(log.LstdFlags)
	})
	return &buf
}

// TestR45RegisterInputErrorsReturn400 超长口令 / 超长用户名 / 弱口令 → 400，且文案可行动。
func TestR45RegisterInputErrorsReturn400(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		wantCode int
		wantMsg  string
	}{
		{
			name:     "口令超过 bcrypt 72 字节上限",
			err:      service.ErrPasswordTooLong,
			wantCode: http.StatusBadRequest,
			wantMsg:  "72",
		},
		{
			name:     "用户名超长",
			err:      service.ErrUsernameTooLong,
			wantCode: http.StatusBadRequest,
			wantMsg:  "64",
		},
		{
			name:     "口令过短",
			err:      service.ErrWeakPassword,
			wantCode: http.StatusBadRequest,
			wantMsg:  "8",
		},
		{
			name:     "用户名冲突（既有映射不得被破坏）",
			err:      service.ErrUserExists,
			wantCode: http.StatusConflict,
			wantMsg:  "已存在",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stub := &stubAuthAPI{registerErr: c.err}
			w := postJSON(t, newAuthTestRouter(stub, true), "/api/v1/auth/register",
				`{"username":"alice","password":"12345678"}`)
			if w.Code != c.wantCode {
				t.Fatalf("应返回 %d，实际 %d（body=%s）", c.wantCode, w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), c.wantMsg) {
				t.Fatalf("响应文案应包含 %q，实际 %s", c.wantMsg, w.Body.String())
			}
			// 输入错误绝不能是 500（GPT-R45 的核心）
			if w.Code == http.StatusInternalServerError {
				t.Fatalf("输入错误不得变成 500：%s", w.Body.String())
			}
		})
	}
}

// TestR45RegisterPassesRawInputToService 注册入口必须把原始输入原样交给 service 校验，
// 不能在 HTTP 层偷偷截断口令（那会让"用户以为的长密码"与"实际生效的密码"不一致）。
func TestR45RegisterPassesRawInputToService(t *testing.T) {
	stub := &stubAuthAPI{}
	long := strings.Repeat("a", 73)
	w := postJSON(t, newAuthTestRouter(stub, true), "/api/v1/auth/register",
		`{"username":"alice","password":"`+long+`"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("替身不报错时应返回 200，实际 %d（body=%s）", w.Code, w.Body.String())
	}
	if stub.lastUser != "alice" || stub.lastPass != long {
		t.Fatalf("注册入参被篡改：user=%q pass(%d 字节)", stub.lastUser, len(stub.lastPass))
	}
}

// TestR45RegisterInternalErrorStays500ButLeaksNothing 真正的内部错误仍是 500，
// 但响应体不得回显内部细节（bcrypt/驱动/表名等）。
func TestR45RegisterInternalErrorStays500ButLeaksNothing(t *testing.T) {
	secret := "bcrypt: password length exceeds 72 bytes / table users"
	stub := &stubAuthAPI{registerErr: errors.New(secret)}
	w := postJSON(t, newAuthTestRouter(stub, true), "/api/v1/auth/register",
		`{"username":"alice","password":"12345678"}`)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("内部错误应仍是 500，实际 %d", w.Code)
	}
	if strings.Contains(w.Body.String(), secret) || strings.Contains(w.Body.String(), "bcrypt") {
		t.Fatalf("响应体泄露内部细节：%s", w.Body.String())
	}
}

// TestR45RegisterDisabledReturns403 关闭公开注册时优先返回 403（既有契约不得被改动）
func TestR45RegisterDisabledReturns403(t *testing.T) {
	stub := &stubAuthAPI{}
	w := postJSON(t, newAuthTestRouter(stub, false), "/api/v1/auth/register",
		`{"username":"alice","password":"12345678"}`)
	if w.Code != http.StatusForbidden {
		t.Fatalf("关闭注册时应返回 403，实际 %d", w.Code)
	}
	if stub.lastUser != "" {
		t.Fatal("关闭注册时不得调用服务层")
	}
}
