package service

// A6 登录失败限流的纯逻辑回归（不碰数据库，本机 / CI 均可跑）。
//
// 端到端口径（配额内的 401 分布、login_failed 审计构成、XFF 不可换桶）
// 由 internal/authzreview 的用例覆盖；这里钉住两块最容易被改坏的底层语义：
//  1. Release 的"成功归还占位"语义——成功不占失败配额的基石；
//  2. loginKey 的分桶口径——缺 IP 必须归 unknown 桶（fail-closed），绝不绕过限流。

import (
	"testing"
	"time"
)

// TestA6LimiterReleaseRefundsQuota Release 必须只归还一次占位，且清空后不留空壳 key。
func TestA6LimiterReleaseRefundsQuota(t *testing.T) {
	l := NewLimiter()
	const max = 2
	const window = time.Minute
	const key = "login:203.0.113.9"

	if !l.Allow(key, max, window) {
		t.Fatal("第 1 次占位应成功")
	}
	if !l.Allow(key, max, window) {
		t.Fatal("第 2 次占位应成功")
	}
	if l.Allow(key, max, window) {
		t.Fatal("配额用满后第 3 次应被拒")
	}
	// 模拟"认证成功"：归还一次占位 ⇒ 又有一个空位
	l.Release(key)
	if !l.Allow(key, max, window) {
		t.Fatal("归还占位后应能再次占位（成功不占失败配额）")
	}
	if l.Allow(key, max, window) {
		t.Fatal("归还的只有一个空位，再次占位后应仍被拒")
	}
	// 无记录的 key 上 Release 必须是幂等空操作（不得 panic / 不得制造条目）
	l.Release("login:never-seen")
	if got := l.Size(); got != 1 {
		t.Fatalf("Release 空 key 不得制造条目，Size=%d", got)
	}
	// 全部归还后 key 应被删除（不留空壳，R44 内存有界）
	l.Release(key)
	l.Release(key)
	if got := l.Size(); got != 0 {
		t.Fatalf("占位全部归还后应删除该 key（不留空壳），Size=%d", got)
	}
	// 删除后配额重新从零计起
	for i := 0; i < max; i++ {
		if !l.Allow(key, max, window) {
			t.Fatalf("配额重置后第 %d 次应被允许", i+1)
		}
	}
	if l.Allow(key, max, window) {
		t.Fatal("重置后配额再次用满应被拒")
	}
}

// TestA6LoginKeyBuckets 桶键口径：按真实来源分桶；缺 IP 归 unknown（fail-closed）。
func TestA6LoginKeyBuckets(t *testing.T) {
	cases := map[string]string{
		"":              "login:unknown", // 无来源（单测/CLI 直调）：统一桶，绝不绕过
		"   ":           "login:unknown",
		"203.0.113.9":   "login:203.0.113.9",
		" 203.0.113.9 ": "login:203.0.113.9", // TrimSpace 后同一桶
		"2001:db8::1":   "login:2001:db8::1",
	}
	for in, want := range cases {
		if got := loginKey(in); got != want {
			t.Fatalf("loginKey(%q) = %q, want %q", in, got, want)
		}
	}
	// 不同来源必须是不同桶（一个 IP 的失败不得连坐另一个 IP）
	if loginKey("203.0.113.9") == loginKey("198.51.100.55") {
		t.Fatal("不同来源不得共用桶")
	}
}

// TestA6ConfigureLoginThrottleRejectsInvalid 装配层传入非法参数必须 panic
//（带着 0 配额运行等于把限流整个关掉，宁可不启动）。
func TestA6ConfigureLoginThrottleRejectsInvalid(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("非法参数（0 配额 / 0 窗口）应 panic")
		}
	}()
	s := NewAuthService(nil, "x", 0)
	s.ConfigureLoginThrottle(0, time.Minute)
}
