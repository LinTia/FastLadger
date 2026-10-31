package service

// A6/R-1/R-2 登录失败限流的纯逻辑回归（不碰数据库，本机 / CI 均可跑）。
//
// 端到端口径（配额内的 401 分布、login_failed 审计构成、XFF 不可换桶、IPv6 /64 聚合
// 的端到端效果）由 internal/authzreview 的用例覆盖；这里钉住底层语义：
//  1. AllowWithToken/ReleaseToken 的占位语义（R-2）——归还的必须是【自己】的占位；
//  2. loginKey 的聚合分桶口径（R-1）——IPv4 /24、IPv6 /64，缺 IP/不可解析归共享桶。
//
// 注意：Size() 是 key（桶）数，Occupancy() 才是占位（命中）数——配额看后者。

import (
	"testing"
	"time"
)

// TestA6TokenReleaseOnlyOwnsItsSlot 占位 token 化：只归还自己那次，不碰同 key 的其他占位（R-2）。
func TestA6TokenReleaseOnlyOwnsItsSlot(t *testing.T) {
	l := NewLimiter()
	const max = 2
	const window = time.Minute
	const key = "login:203.0.113.0/24"

	tA, ok := l.AllowWithToken(key, max, window)
	if !ok {
		t.Fatal("第 1 次占位应成功")
	}
	tB, ok := l.AllowWithToken(key, max, window)
	if !ok {
		t.Fatal("第 2 次占位应成功")
	}
	if got := l.Occupancy(); got != 2 {
		t.Fatalf("两次占位后应剩 2 个占位，实际 %d", got)
	}
	// 配额用满
	if _, ok := l.AllowWithToken(key, max, window); ok {
		t.Fatal("配额用满后第 3 次应被拒")
	}
	// 只归还 A 的占位：B 的占位必须原样保留
	l.ReleaseToken(tA)
	if got := l.Occupancy(); got != 1 {
		t.Fatalf("归还 A 后应剩 1 个占位（B 的），实际 %d", got)
	}
	tC, ok := l.AllowWithToken(key, max, window)
	if !ok {
		t.Fatal("归还 A 后应能再占一个位")
	}
	if got := l.Occupancy(); got != 2 {
		t.Fatalf("再占位后应为 2 个占位，实际 %d", got)
	}
	// token 可重复归还：幂等空操作（不得误删 B/C 的占位）
	l.ReleaseToken(tA)
	l.ReleaseToken(tA)
	if got := l.Occupancy(); got != 2 {
		t.Fatalf("重复归还不得影响其他占位，占位数=%d", got)
	}
	// 未知 / 空 token：幂等空操作
	l.ReleaseToken(key + "\x00bogus")
	l.ReleaseToken("")
	l.ReleaseToken("no-separator-token")
	if got := l.Occupancy(); got != 2 {
		t.Fatalf("未知 token 不得影响状态，占位数=%d", got)
	}
	// 每个占位只能被它自己的 token 归还
	l.ReleaseToken(tB)
	if got := l.Occupancy(); got != 1 {
		t.Fatalf("归还 B 后应剩 1 个占位，实际 %d", got)
	}
	l.ReleaseToken(tC)
	if got := l.Occupancy(); got != 0 {
		t.Fatalf("全部归还后占位数应为 0，实际 %d", got)
	}
	// 占位清空后空壳 key 也应被删除（R44：内存有界）
	if got := l.Size(); got != 0 {
		t.Fatalf("占位清空后应删除空壳 key，Size=%d", got)
	}
	// 配额从零重新计起
	for i := 0; i < max; i++ {
		if _, ok := l.AllowWithToken(key, max, window); !ok {
			t.Fatalf("重置后第 %d 次应被允许", i+1)
		}
	}
	if _, ok := l.AllowWithToken(key, max, window); ok {
		t.Fatal("重置后配额再次用满应被拒")
	}
}

// TestA6TokenReleaseSurvivesConcurrentSuccessAndFailure R-2 的原始场景（ZZ-3）：
// A 占位 → B 占位后失败（不归还）→ A 成功归还 —— B 的失败计数必须保留、A 不得留幽灵占位。
//
// 判别手法：A、B 占位时间相差 10 分钟（窗口 15 分钟）。token 语义下存活的是
// B 的失败（t+10min，t+25min 才过期）；旧的"删最近一条"实现存活的是 A 的占位
// （t0，t+15min 就过期）。在 t+16min 探测"还能再失败几次"：
//   - token 语义：B 仍在窗内（1 条）→ 再记 1 条失败即满（max=2）→ 第 2 次探测被拒；
//   - 旧实现：A 已过窗（0 条）→ 2 次探测都被放行。
//
// 该判别必须用注入时钟做确定性断言（sleep 只会在慢机器上偶发失败）。
func TestA6TokenReleaseSurvivesConcurrentSuccessAndFailure(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := base
	l := newLimiter(1000, func() time.Time { return now })

	const key = "login:203.0.113.0/24"
	const window = 15 * time.Minute

	tA, ok := l.AllowWithToken(key, 2, window) // t0：A 占位（in-flight）
	if !ok {
		t.Fatal("前提失败：A 占位被拒")
	}
	now = base.Add(10 * time.Minute)
	if _, ok := l.AllowWithToken(key, 2, window); !ok { // t+10min：B 占位后【失败】（不归还）
		t.Fatal("前提失败：B 占位被拒")
	}
	l.ReleaseToken(tA) // t+10min：A 成功，归还【自己的】占位

	// 关键判别：t+16min 时 B 的失败（t+10min）仍在窗内（t+25min 才过期）
	now = base.Add(16 * time.Minute)
	p1, ok := l.AllowWithToken(key, 2, window) // 探测 1：允许，且它是一次【失败】（保留）
	if !ok {
		t.Fatalf("t+16min 第一次探测应被允许（B 的失败仍在窗内占 1 条），实际被拒；占位数=%d", l.Occupancy())
	}
	if _, ok := l.AllowWithToken(key, 2, window); ok {
		t.Fatalf("t+16min 第二次探测应被拒（B 的失败 + 探测1 = 2 条已打满 max=2）——" +
			"若被放行，说明存活的不是 B 的失败计数（Release 语义退回了'删最近一条'）")
	}
	// 探测 1 的占位归还（模拟它是成功请求）：B 的失败仍在
	l.ReleaseToken(p1)
	if got := l.Occupancy(); got != 1 {
		t.Fatalf("归还探测占位后应只剩 B 的失败 1 条，实际 %d", got)
	}
	// B 的失败在 t+25min 过期：之后配额应恢复
	now = base.Add(26 * time.Minute)
	if _, ok := l.AllowWithToken(key, 2, window); !ok {
		t.Fatalf("t+26min B 的失败已过窗，配额应恢复，实际仍被拒；占位数=%d", l.Occupancy())
	}
}

// TestA6LoginKeyAggregatesByPrefix 桶键按网段聚合（R-1）：IPv4 /24、IPv6 /64。
func TestA6LoginKeyAggregatesByPrefix(t *testing.T) {
	cases := map[string]string{
		"":                     "login:invalid", // 无来源（单测/CLI 直调）：共享桶，绝不绕过
		"   ":                  "login:invalid", // 空白同上
		"not-an-ip":            "login:invalid", // 不可解析：fail-closed
		"203.0.113.9":          "login:203.0.113.0/24",
		" 203.0.113.77 ":       "login:203.0.113.0/24", // 同 /24 聚合（TrimSpace 后）
		"198.51.100.55":        "login:198.51.100.0/24",
		"2001:db8::1":          "login:2001:db8::/64",
		"2001:db8::ffff":       "login:2001:db8::/64", // 同 /64 聚合
		"2001:db8:0:1::1":      "login:2001:db8:0:1::/64",
		"::ffff:203.0.113.200": "login:203.0.113.0/24", // IPv4-mapped 按 IPv4 口径
	}
	for in, want := range cases {
		if got := loginKey(in); got != want {
			t.Fatalf("loginKey(%q) = %q, want %q", in, got, want)
		}
	}
	// 不同 /24、不同 /64 必须是不同桶；同 /64 内轮转地址必须是同一个桶（ZZ-2 的修复面）
	if loginKey("203.0.113.9") == loginKey("203.0.114.9") {
		t.Fatal("不同 /24 不得共用桶")
	}
	if loginKey("2001:db8::1") != loginKey("2001:db8::dead") {
		t.Fatal("同一 /64 内的地址必须聚合到同一个桶（IPv6 轮转防护）")
	}
	if loginKey("2001:db8::1") == loginKey("2001:db8:0:1::1") {
		t.Fatal("不同 /64 不得共用桶")
	}
}

// TestA6ConfigureLoginThrottleRejectsInvalid 装配层传入非法参数必须 panic
// （带着 0 配额 / 0 窗口运行等于把限流整个关掉，宁可不启动）。
func TestA6ConfigureLoginThrottleRejectsInvalid(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("非法参数（0 配额 / 0 窗口）应 panic")
		}
	}()
	s := NewAuthService(nil, "x", 0)
	s.ConfigureLoginThrottle(0, time.Minute)
}
