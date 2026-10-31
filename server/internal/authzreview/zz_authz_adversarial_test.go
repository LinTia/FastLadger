package authzreview

// zz_authz_adversarial_test.go —— 对 A5/A6/A7 修复（9c1f986 / b61a57a）的**对抗性验证探针**。
//
// 定位：这些不是缺陷固化测试（修复是好的），而是对修复本身的二次攻击验证：
// 每条探针都从"如果我是攻击者/运维，我还能怎么绕或被误伤"出发。
// 第二波收尾（R-1/R-2）后，原两条红灯（ZZ-2/ZZ-3）已翻转为回归守卫，全部用例为绿。
//
//   - ZZ-1（绿）登录限流对 IPv4 轮转（含伪造 XFF）有效；
//   - ZZ-2（绿，原红）IPv6 /64 内轮转不再绕过登录限流（loginKey 已按 /24//64 聚合）；
//   - ZZ-3（绿，原红）占位 token 化：归还只删自己那条，不误删他人失败计数、不留幽灵占位；
//   - ZZ-4（绿）429 与 401 在响应体上可区分（客户端不会把限流当凭证错误重试）；
//   - ZZ-5（绿）生产路径限流恒开：config.Load 正值默认 + NewRouter 真实接线生效；
//   - ZZ-6（绿）sync push 限流按"调用"计数成立（60/min × maxPushItems=200 的吞吐上界由此可算）。

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"fastledger/internal/api"
	"fastledger/internal/config"
	"fastledger/internal/service"
	"fastledger/internal/version"

	"github.com/gin-gonic/gin"
	"xorm.io/xorm"
)

// zzLoginRouter 带真实用户、默认限流参数的登录路由。
func zzLoginRouter(t *testing.T) (*gin.Engine, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	return authzLoginRouter(t, engine(t))
}

// zzPostLoginFrom 指定来源地址与可选 XFF 发登录请求。
func zzPostLoginFrom(r *gin.Engine, user, pass, remoteAddr, xff string) int {
	req := newZZLoginRequest(user, pass)
	req.RemoteAddr = remoteAddr
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	w := newZZRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

// newZZLoginRequest 构造一个 Content-Type 正确的登录请求。
func newZZLoginRequest(user, pass string) *http.Request {
	body := `{"username":"` + user + `","password":"` + pass + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

// newZZRecorder httptest 响应记录器。
func newZZRecorder() *httptest.ResponseRecorder { return httptest.NewRecorder() }

// newZZRouterWithConfig 按给定配置构造完整路由（与生产 main.go 同一套接线）。
func newZZRouterWithConfig(e *xorm.Engine, as *service.AuthService, books *service.BookService,
	ms *service.MemberService, txs *service.TxService, syncSvc *service.SyncService,
	cfg *config.Config) *gin.Engine {
	return api.NewRouter(api.Deps{
		Config:        cfg,
		AuthService:   as,
		Books:         books,
		Members:       ms,
		Accounts:      service.NewAccountService(e, version.NewRepo()),
		Categories:    service.NewCategoryService(e, version.NewRepo()),
		Txs:           txs,
		Sync:          syncSvc,
		SchemaVersion: 1,
	})
}

// TestZZ1LoginLimitSurvivesIPv4RotationWithSpoofedXFF IPv4 场景下限流不可被轮转绕过（回归守卫）。
//
// 期望：同一 IPv4 来源 + 每次不同的伪造 X-Forwarded-For（合法 IP 文本，gin 会接受的形态），
//
//	连打 25 次错误口令必须出现 429 —— 因为 SetTrustedProxies(nil) 后 ClientIP 恒取
//	TCP 对端，请求头拼不出新桶。
func TestZZ1LoginLimitSurvivesIPv4RotationWithSpoofedXFF(t *testing.T) {
	r, user := zzLoginRouter(t)

	limited := 0
	for i := 0; i < 25; i++ {
		xff := "198.51.100." + strconv.Itoa(i+1)
		if c := zzPostLoginFrom(r, user, "wrong-"+strconv.Itoa(i),
			"203.0.113.7:5000", xff); c == http.StatusTooManyRequests {
			limited++
		}
	}
	if limited == 0 {
		t.Fatalf("同一 IPv4 来源 + 25 个不同的伪造 XFF，没有一次 429：" +
			"登录限流仍可被请求头轮转绕过，请核对 router.go 的 SetTrustedProxies(nil)")
	}
}

// TestZZ2LoginLimitBypassedByIPv6WithinOneSlash64 IPv6 同网段轮转不得摊薄登录限流（R-1，已修复→回归守卫）。
//
// 原缺陷：loginKey = "login:" + **完整 ClientIP**，未做前缀分桶——IPv6 隐私扩展
// （RFC 4941）/ 代理池让一台机器在自己 /64 内拥有海量地址，每换一个地址就是全新配额桶，
// 15 分钟 10 次的配额被 /64 内的地址数摊薄，实际等于"没限"（验证员实测：对照 15 次
// 出 429、/64 内轮转 15 个地址 0 次 429）。
//
// 修复后（loginKey 按 /24(IPv4)//64(IPv6) 聚合，见 auth.go）：同一 /64 内的所有地址
// 共享一个配额桶——对照（::1）打满配额后，同 /64 内轮转的地址必须全部吃到 429。
// 粒度口径已写进《落地方案》§8.4.2：IPv6 环境下限流粒度是 /64 而非单地址。
func TestZZ2LoginLimitBypassedByIPv6WithinOneSlash64(t *testing.T) {
	r, user := zzLoginRouter(t)

	// 对照 1：同一地址 15 次 → 配额 10，必须出现 429（证明限流器本身在工作）
	same := 0
	for i := 0; i < 15; i++ {
		if c := zzPostLoginFrom(r, user, "wrong-same-"+strconv.Itoa(i),
			"[2001:db8::1]:6000", ""); c == http.StatusTooManyRequests {
			same++
		}
	}
	if same == 0 {
		t.Fatalf("对照失败：同一 IPv6 地址 15 次错误口令没有一次 429，限流器未生效（请核对本用例前提）")
	}

	// 被测：同一 /64 内轮转 15 个地址 —— 修复后必须与对照【同一个桶】，
	// 而对照已把该桶的配额打满，所以 15 次必须全部 429（一个 401 都不该有）
	rotated := 0
	unauthorized := 0
	for i := 0; i < 15; i++ {
		addr := "[2001:db8::" + strconv.Itoa(i+2) + "]:6000"
		switch c := zzPostLoginFrom(r, user, "wrong-rot-"+strconv.Itoa(i), addr, ""); {
		case c == http.StatusTooManyRequests:
			rotated++
		case c == http.StatusUnauthorized:
			unauthorized++
		}
	}
	if rotated != 15 || unauthorized != 0 {
		t.Fatalf("同 /64 内轮转 15 个地址应全部 429（与对照同桶、配额已被对照打满），"+
			"实际 429=%d 401=%d —— loginKey 的 /64 聚合可能已失效，请核对 auth.go 的 loginKey", rotated, unauthorized)
	}
}

// TestZZ3ReleaseOnlyOwnsItsSlot 占位必须可区分、归还只删自己那条（R-2，已修复→回归守卫）。
//
// 原缺陷：Release(key) 删的是【最近一条】占位。同一 key（NAT/办公室共享出口）上
// "A 占位 → B 占位后失败（失败不归还）→ A 成功归还"的时序里，被删的是 B 的失败计数、
// 留下的是 A 的成功幽灵占位——偏差方向是"少算失败"，攻击者可借同 IP 上并发的成功
// 登录稀释自己的失败计数（P3，偏差 ±1/请求）。
//
// 修复后：占位 token 化——AllowWithToken 返回凭据，ReleaseToken 只删【自己】那一次，
// 与同 key 上其他请求的成败完全无关。本用例直接断言：
//  1. 归还 A 后，B 的失败计数仍在（Size 只减 1，且后续占位行为一致）；
//  2. 归还是幂等的（重复归还、归还未知 token 均为空操作）；
//  3. 带时间差的判别版（"存活的到底是哪条"）见 service 包
//     TestA6TokenReleaseSurvivesConcurrentSuccessAndFailure（注入时钟，确定性）。
func TestZZ3ReleaseOnlyOwnsItsSlot(t *testing.T) {
	l := service.NewLimiter()
	const key = "login:203.0.113.0/24"
	const max = 10

	tA, ok := l.AllowWithToken(key, max, 15*time.Minute) // A 占位（in-flight）
	if !ok {
		t.Fatal("前提失败：A 占位被拒")
	}
	tB, ok := l.AllowWithToken(key, max, 15*time.Minute) // B 占位后【失败】（不归还）
	if !ok {
		t.Fatal("前提失败：B 占位被拒")
	}
	// A 成功：只归还 A 自己的占位
	l.ReleaseToken(tA)
	if got := l.Occupancy(); got != 1 {
		t.Fatalf("A 归还后应剩 1 个占位（B 的失败计数），实际 %d —— B 的失败被误删说明归还语义仍不精确", got)
	}
	// 归还是幂等的：重复归还 A、归还未知 token 都不得影响 B 的计数
	l.ReleaseToken(tA)
	l.ReleaseToken(tB + "-bogus")
	l.ReleaseToken("")
	if got := l.Occupancy(); got != 1 {
		t.Fatalf("幂等归还不得改变状态，占位数=%d", got)
	}
	// B 的失败仍占据配额：再占 9 个（B + 9 = max）后应拒绝第 11 次占位
	for i := 0; i < max-1; i++ {
		if _, ok := l.AllowWithToken(key, max, 15*time.Minute); !ok {
			t.Fatalf("第 %d 个占位不应被拒（当前 %d < %d）", i+3, i+1, max)
		}
	}
	if _, ok := l.AllowWithToken(key, max, 15*time.Minute); ok {
		t.Fatalf("配额应恰在 %d 条时收口（B 的失败计数必须被完整保留）", max)
	}
	// B 的占位只能被它自己的 token 归还（其余请求的成败不影响它）
	l.ReleaseToken(tB)
	if got := l.Occupancy(); got != max-1 {
		t.Fatalf("归还 B 的占位后应剩 %d 个占位，实际 %d", max-1, got)
	}
}

// TestZZ4RateLimitedResponseIsDistinguishableFromBadCredentials 429 必须与 401 可区分（回归守卫）。
//
// 期望：配额打满后响应为 429 且文案与凭证错误不同 —— 客户端才能正确"退避"而不是
//
//	把限流当成密码错误反复重试（反复重试只会让锁定更久）。
func TestZZ4RateLimitedResponseIsDistinguishableFromBadCredentials(t *testing.T) {
	r, user := zzLoginRouter(t)
	e := engine(t)

	// 先打满配额（默认 10）
	var lastCode int
	var lastBody string
	for i := 0; i < 12; i++ {
		req := newZZLoginRequest(user, "wrong-"+strconv.Itoa(i))
		req.RemoteAddr = "203.0.113.11:7000"
		w := newZZRecorder()
		r.ServeHTTP(w, req)
		lastCode, lastBody = w.Code, w.Body.String()
		_ = e
	}
	if lastCode != http.StatusTooManyRequests {
		t.Fatalf("打满配额后应返回 429，实际 %d（body=%s）", lastCode, lastBody)
	}
	if strings.Contains(lastBody, "用户名或密码错误") {
		t.Fatalf("429 响应不得复用 401 的凭证错误文案（会让客户端当成密码错反复重试），body=%s", lastBody)
	}
	if !strings.Contains(lastBody, "频繁") && !strings.Contains(lastBody, "稍后") {
		t.Fatalf("429 响应应明确表达'限流/退避'语义，body=%s", lastBody)
	}
}

// TestZZ5ThrottleWiredAndAlwaysOnInProductionPath 生产路径限流必须恒开（守卫测试）。
//
// 两段断言：
//  1. config.Load 在未设置相关环境变量时给出**正值**默认（10/15/60/300）——
//     生产唯一 HTTP 入口 cmd/server → config.LoadBase + ValidateRuntime，都走 newConfig，
//     因此"默认关闭、装配层按配置开启"的取舍在生产上是恒开的；
//  2. NewRouter 拿到正值配置后限流**真的生效**（防止将来接线被删无人发现）：
//     用 SyncPushPerMinute=2 构造路由，第 3 次 push 调用必须 429。
func TestZZ5ThrottleWiredAndAlwaysOnInProductionPath(t *testing.T) {
	// —— 第 1 段：config 默认值必须为正（生产恒开的前提）——
	for _, k := range []string{
		"EBK_LOGIN_MAX_ATTEMPTS", "EBK_LOGIN_WINDOW_MINUTES",
		"EBK_SYNC_PUSH_PER_MINUTE", "EBK_SYNC_PULL_PER_MINUTE",
	} {
		t.Setenv(k, "")
	}
	t.Setenv("EBK_JWT_SECRET", "zz-guard-secret-0123456789abcdef-0123456789")
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load 失败：%v", err)
	}
	if cfg.LoginMaxAttempts <= 0 || cfg.LoginWindowMinutes <= 0 ||
		cfg.SyncPushPerMinute <= 0 || cfg.SyncPullPerMinute <= 0 {
		t.Fatalf("生产默认配置必须为正（限流恒开的前提），实际 login=%d/%dmin push=%d pull=%d",
			cfg.LoginMaxAttempts, cfg.LoginWindowMinutes, cfg.SyncPushPerMinute, cfg.SyncPullPerMinute)
	}

	// —— 第 2 段：NewRouter 接线后限流真的生效 ——
	gin.SetMode(gin.TestMode)
	e := engine(t)
	as := service.NewAuthService(e, "zz-guard-secret-0123456789abcdef", time.Hour)
	books := service.NewBookService(e, version.NewRepo())
	ms, err := service.NewMemberService(e, as, 7)
	if err != nil {
		t.Fatalf("NewMemberService 失败：%v", err)
	}
	txs := service.NewTxService(e, version.NewRepo())
	syncSvc := service.NewSyncService(e, txs, nil)
	r := newZZRouterWithConfig(e, as, books, ms, txs, syncSvc, &config.Config{
		LoginMaxAttempts:   10,
		LoginWindowMinutes: 15,
		SyncPushPerMinute:  2, // 故意取极小值：第 3 次 push 必须被限
		SyncPullPerMinute:  300,
	})

	limited := 0
	for i := 0; i < 4; i++ {
		if _, err := syncSvc.Push(nil, 1, 1, nil); errors.Is(err, service.ErrRateLimited) {
			limited++
		}
	}
	if limited == 0 {
		t.Fatalf("NewRouter 按正值 Sync 配置接线后，push 限流仍未生效（4 次调用无一被拒）：" +
			"接线可能已失效，请核对 router.go 的 EnableThrottle 调用与 EnableThrottle 本体")
	}
	_ = r
}

// TestZZ6PushThrottleCountsPerCall 吞吐上界核算的前提：push 限流按【调用】计数。
//
// 60 次/分钟 × maxPushItems=200 条/次 = 单用户每分钟最多 12000 条交易写入
// （每条一次事务 + 一次版本递增 + 一行审计 + 一行幂等回执）。
// 本用例断言"按调用计数"成立：即使 items 为空（不产生任何写入），也消耗一个配额槽；
// 超出配额后返回 ErrRateLimited。据此可以精确核算写路径吞吐上界并写进文档。
func TestZZ6PushThrottleCountsPerCall(t *testing.T) {
	e := engine(t)
	txs := service.NewTxService(e, version.NewRepo())
	s := service.NewSyncService(e, txs, nil)
	s.EnableThrottle(2, 300) // 极小配额：第 3 次调用必被限

	errs := make([]error, 0, 3)
	for i := 0; i < 3; i++ {
		_, err := s.Push(nil, 1, 1, nil) // items 为空：合法进入限流，之后才因空 items 报参数错误
		errs = append(errs, err)
	}
	if !errors.Is(errs[0], service.ErrInvalidArgument) || !errors.Is(errs[1], service.ErrInvalidArgument) {
		t.Fatalf("前两次空 items 调用应报参数错误（并消耗配额），实际 %v", errs)
	}
	if !errors.Is(errs[2], service.ErrRateLimited) {
		t.Fatalf("第 3 次调用应被限流（ErrRateLimited），实际 %v —— push 限流若按 items 条数计，吞吐上界核算就不成立", errs[2])
	}
}
