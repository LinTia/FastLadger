package api

// GPT-R46：登录错误映射——凭证错误统一 401，内部错误 500/503 + 安全日志。
//
// 覆盖四类分支：
//  1. 口令错误 → 401（与"账号不存在"同文案，防枚举）；
//  2. 账号禁用 → 401（同样文案，防"哪些号被封了"的枚举）；
//  3. 依赖不可用（DB 故障 / 登录审计写入失败 / 令牌签发失败）→ 503 + 服务端日志；
//  4. 其它未预期错误 → 500 + 服务端日志。
// 3 与 4 都额外断言：响应体不含内部细节，且日志里留下了排查线索但不记口令。
//
// 与登录审计的协调（明确写进断言）：审计写入失败属于【依赖不可用】，
// 绝不能回落 401——否则客户端只提示"用户名或密码错误"，运维看不到任何线索。

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"fastledger/internal/service"
)

// TestR46CredentialErrorsAreUniform401 凭证类错误一律 401，且"口令错"与"账号禁用"文案一致。
func TestR46CredentialErrorsAreUniform401(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"用户不存在", service.ErrBadCredentials},
		{"口令错误", fmt.Errorf("%w：比对失败", service.ErrBadCredentials)},
		{"账号已禁用", service.ErrAccountDisabled},
	}
	wantBody := ""
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stub := &stubAuthAPI{loginErr: c.err}
			w := postJSON(t, newAuthTestRouter(stub, true), "/api/v1/auth/login",
				`{"username":"alice","password":"wrong-password"}`)
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("%s 应返回 401，实际 %d（body=%s）", c.name, w.Code, w.Body.String())
			}
			body := w.Body.String()
			if !strings.Contains(body, "用户名或密码错误") {
				t.Fatalf("%s 的响应文案应为统一的凭证错误，实际 %s", c.name, body)
			}
			// 不得出现"账号不存在/已禁用"之类的区分性措辞（那是账号枚举的入口）
			for _, leak := range []string{"不存在", "禁用", "disabled", "not found"} {
				if strings.Contains(body, leak) {
					t.Fatalf("%s 的响应泄露了账号状态（%q）：%s", c.name, leak, body)
				}
			}
			if wantBody == "" {
				wantBody = body
			} else if body != wantBody {
				t.Fatalf("%s 的响应与其它凭证错误不一致（可据此枚举账号）：%s vs %s", c.name, body, wantBody)
			}
		})
	}
}

// TestR46InternalErrorsAre5xxAndLeakNothing 内部错误 → 503/500，响应体不含细节，
// 但服务端日志留下了排查线索，且日志里不记口令。
func TestR46InternalErrorsAre5xxAndLeakNothing(t *testing.T) {
	auditFail := fmt.Errorf("%w：登录审计写入失败，已拒绝签发令牌：%w",
		service.ErrAuthUnavailable, errors.New("audit_logs insert: disk I/O error"))

	cases := []struct {
		name     string
		err      error
		wantCode int
		secret   string // 任何情况下都不许出现在响应体里
	}{
		{
			name:     "登录审计写入失败",
			err:      auditFail,
			wantCode: http.StatusServiceUnavailable,
			secret:   "audit_logs insert: disk I/O error",
		},
		{
			name:     "数据库故障",
			err:      fmt.Errorf("%w：查询用户失败：%w", service.ErrAuthUnavailable, errors.New("database is locked")),
			wantCode: http.StatusServiceUnavailable,
			secret:   "database is locked",
		},
		{
			name:     "令牌签发失败",
			err:      fmt.Errorf("%w：签发令牌失败：%w", service.ErrAuthUnavailable, errors.New("jwt: signing error")),
			wantCode: http.StatusServiceUnavailable,
			secret:   "jwt: signing error",
		},
		{
			name:     "未预期的内部错误",
			err:      errors.New("panic recovered at auth.go:142"),
			wantCode: http.StatusInternalServerError,
			secret:   "auth.go:142",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			logBuf := captureLog(t)
			stub := &stubAuthAPI{loginErr: c.err}
			w := postJSON(t, newAuthTestRouter(stub, true), "/api/v1/auth/login",
				`{"username":"alice","password":"super-secret-password"}`)
			if w.Code != c.wantCode {
				t.Fatalf("%s 应返回 %d，实际 %d（body=%s）", c.name, c.wantCode, w.Code, w.Body.String())
			}
			// ① 响应体不得回显内部细节
			body := w.Body.String()
			if strings.Contains(body, c.secret) {
				t.Fatalf("%s 的响应体泄露内部细节：%s", c.name, body)
			}
			if strings.Contains(body, "用户名或密码错误") {
				t.Fatalf("%s 不得回落成凭证错误的文案（会让用户反复重试）：%s", c.name, body)
			}
			// ② 服务端必须留下安全日志（含错误本身，供排查）
			logs := logBuf.String()
			if !strings.Contains(logs, "登录内部错误") {
				t.Fatalf("%s 未记录安全日志，日志内容：%q", c.name, logs)
			}
			if !strings.Contains(logs, c.secret) {
				t.Fatalf("%s 的日志缺少排查所需信息（%q），实际：%q", c.name, c.secret, logs)
			}
			// ③ 日志里绝不能出现口令
			if strings.Contains(logs, "super-secret-password") {
				t.Fatalf("%s 的日志记录了口令：%q", c.name, logs)
			}
		})
	}
}

// TestR46LoginSuccessUnchanged 成功路径不受影响（200 + accessToken）。
func TestR46LoginSuccessUnchanged(t *testing.T) {
	stub := &stubAuthAPI{loginToken: "jwt-token-abc"}
	w := postJSON(t, newAuthTestRouter(stub, true), "/api/v1/auth/login",
		`{"username":"alice","password":"12345678"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("成功登录应返回 200，实际 %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "jwt-token-abc") {
		t.Fatalf("响应应带回 accessToken，实际 %s", w.Body.String())
	}
}

// TestR46BadJSONStill400 请求体非法仍是 400（收紧错误映射不得破坏既有输入校验）
func TestR46BadJSONStill400(t *testing.T) {
	stub := &stubAuthAPI{loginErr: service.ErrBadCredentials}
	w := postJSON(t, newAuthTestRouter(stub, true), "/api/v1/auth/login", `{not-json`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("非法 JSON 应返回 400，实际 %d", w.Code)
	}
}
