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
	"net/http/httptest"
	"os"
	"runtime"
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

// readAPIHandlerSource 读取本包内的源文件（源码级守卫用，仿 member_test.readServiceSource）
func readAPIHandlerSource(t *testing.T, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("无法定位测试文件路径")
	}
	dir := file[:strings.LastIndex(file, "/")+1]
	b, err := os.ReadFile(dir + name)
	if err != nil {
		// Windows 路径分隔符兜底
		dir = file[:strings.LastIndex(file, "\\")+1]
		b, err = os.ReadFile(dir + name)
		if err != nil {
			t.Fatalf("读取 %s 失败：%v", name, err)
		}
	}
	return string(b)
}

// TestA6HandlerMustUseLoginFromIP 源码级守卫（R-5）：登录 handler 必须调用 LoginFromIP。
//
// 为什么要钉死：service.Login（2 参兼容入口）会把所有调用归入同一个共享桶
// （login:invalid）——一旦有人在生产代码里改用它，所有无来源/同来源的流量共享
// 10 次/15min 配额，互相锁死；这正是登录失败限流设计的反面。handler 只允许走
// LoginFromIP（按真实对端网段分桶）。
func TestA6HandlerMustUseLoginFromIP(t *testing.T) {
	src := readAPIHandlerSource(t, "auth_handler.go")
	if !strings.Contains(src, "LoginFromIP(req.Username, req.Password") {
		t.Fatal("auth_handler.go 的 login 未调用 LoginFromIP（限流按来源分桶的入口）")
	}
	// 2 参 Login 是兼容入口（归共享桶），生产代码禁止调用：
	// "Login(" 紧跟左括号才是 2 参形态；LoginFromIP 不匹配该子串
	if i := strings.Index(src, ".Login("); i >= 0 {
		line := 1 + strings.Count(src[:i], "\n")
		t.Fatalf("auth_handler.go 第 %d 行附近调用了 2 参 Login（共享桶，生产禁用）", line)
	}
}

// TestA6RateLimitLogIsThrottledPerIP R-3：429 拒绝必须有【节流】的服务端日志——
// 同一来源 1 分钟内最多 1 条；不同来源各自记录。
func TestA6RateLimitLogIsThrottledPerIP(t *testing.T) {
	logBuf := captureLog(t)
	stub := &stubAuthAPI{loginErr: service.ErrRateLimited}
	r := newAuthTestRouter(stub, true)

	post := func(remote string) {
		req := httptest.NewRequest("POST", "/api/v1/auth/login",
			strings.NewReader(`{"username":"alice","password":"12345678"}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = remote
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 429 {
			t.Fatalf("前提失败：应返回 429，实际 %d", w.Code)
		}
	}

	const ip1, ip2 = "203.0.113.50:1", "203.0.113.51:1"
	for i := 0; i < 5; i++ {
		post(ip1) // 同一来源打 5 次：日志必须只出现 1 条（节流）
	}
	post(ip2) // 另一来源：应各记 1 条

	logs := logBuf.String()
	if got := strings.Count(logs, "登录失败限流已触发"); got != 2 {
		t.Fatalf("两个来源各 1 条 429 日志，实际 %d 条：%s", got, logs)
	}
	if n := strings.Count(logs, "ip="+ip1[:strings.Index(ip1, ":")]); n != 1 {
		t.Fatalf("同一来源的 429 日志应被节流为 1 条，实际 %d 条：%s", n, logs)
	}
	// 节流表必须有界（防内存放大）
	rateLimitLogState.Lock()
	size := len(rateLimitLogState.seen)
	rateLimitLogState.Unlock()
	if size > rateLimitLogMaxIPs {
		t.Fatalf("429 节流表超过容量上限：%d > %d", size, rateLimitLogMaxIPs)
	}
	// 日志里不得出现用户名/口令
	if strings.Contains(logs, "alice") || strings.Contains(logs, "12345678") {
		t.Fatalf("429 日志泄露了请求内容：%s", logs)
	}
}
