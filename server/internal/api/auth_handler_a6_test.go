package api

// A6（登录失败限流）的 HTTP 适配层回归。
//
// service 层的分桶/计数逻辑在 internal/service 的用例里覆盖；这里钉住两件事：
//  1. ErrRateLimited 必须映射成 429——它不是凭证错误（不能混进 R46 的统一 401 文案，
//     那会让客户端以为是密码错而反复重试），也不是内部错误（不该触发告警）；
//  2. 登录入口必须把 gin 的 ClientIP()（受信代理口径下 = TCP 对端）透传给 service，
//     限流桶键绝不能取自请求头。

import (
	"errors"
	"strings"
	"testing"

	"fastledger/internal/service"
)

// TestA6RateLimitedIs429 限流错误 → 429，且不落安全日志（它不是内部错误）。
func TestA6RateLimitedIs429(t *testing.T) {
	logBuf := captureLog(t)
	stub := &stubAuthAPI{loginErr: service.ErrRateLimited}
	w := postJSON(t, newAuthTestRouter(stub, true), "/api/v1/auth/login",
		`{"username":"alice","password":"12345678"}`)
	if w.Code != 429 {
		t.Fatalf("限流应返回 429，实际 %d（body=%s）", w.Code, w.Body.String())
	}
	body := w.Body.String()
	// 不得混入 R46 的统一 401 文案（那会把"被限流"伪装成"密码错"）
	if strings.Contains(body, "用户名或密码错误") {
		t.Fatalf("429 响应不得使用凭证错误文案：%s", body)
	}
	if strings.Contains(body, "内部错误") || strings.Contains(body, "不可用") {
		t.Fatalf("429 响应不得表述为服务故障：%s", body)
	}
	if logs := logBuf.String(); strings.Contains(logs, "登录内部错误") {
		t.Fatalf("限流不是内部错误，不应写安全日志：%q", logs)
	}
}

// TestA6RateLimitedWrappedStill429 包一层 %w 也必须仍能识别为限流
// （service 返回 fmt.Errorf("%w") 包装形态时不得退化成 500）。
func TestA6RateLimitedWrappedStill429(t *testing.T) {
	stub := &stubAuthAPI{loginErr: errors.Join(errors.New("前置校验"), service.ErrRateLimited)}
	w := postJSON(t, newAuthTestRouter(stub, true), "/api/v1/auth/login",
		`{"username":"alice","password":"12345678"}`)
	if w.Code != 429 {
		t.Fatalf("包装后的限流错误仍应返回 429，实际 %d", w.Code)
	}
}

// TestA6LoginPassesRealClientIP 登录入口透传给 service 的必须是 gin 的 ClientIP()。
// httptest.NewRequest 的默认 RemoteAddr = 192.0.2.1:1234，且未带任何代理头
// （受信代理口径下 ClientIP 恒等于 RemoteAddr）。
func TestA6LoginPassesRealClientIP(t *testing.T) {
	stub := &stubAuthAPI{}
	w := postJSON(t, newAuthTestRouter(stub, true), "/api/v1/auth/login",
		`{"username":"alice","password":"12345678"}`)
	if w.Code != 200 {
		t.Fatalf("替身不报错时应返回 200，实际 %d", w.Code)
	}
	if stub.lastLoginIP == "" {
		t.Fatal("登录入口必须把 ClientIP 透传给 service（限流按它分桶），实际传了空值")
	}
	if strings.HasPrefix(stub.lastLoginIP, "198.51.100.") {
		// 防御性断言：这里根本没设置 XFF，若出现伪造网段说明取错了来源
		t.Fatalf("ClientIP 不应来自伪造网段：%q", stub.lastLoginIP)
	}
}
