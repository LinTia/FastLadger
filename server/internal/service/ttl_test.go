package service

// GPT-R27：天数 → 秒数换算的溢出与非法值防护（纯函数，不需要数据库）。

import (
	"math"
	"testing"
)

func TestDaysToSecondsRejectsNonPositive(t *testing.T) {
	for _, d := range []int{0, -1, -30, -36500} {
		if got, err := daysToSeconds(d); err == nil {
			t.Fatalf("days=%d 应报错，实际通过（返回 %d）", d, got)
		}
	}
}

func TestDaysToSecondsNormalValues(t *testing.T) {
	cases := []struct {
		days int
		want int64
	}{
		{1, 86400},
		{7, 7 * 86400},
		{30, 30 * 86400},
		{365, 365 * 86400},
		{maxRetentionDays, int64(maxRetentionDays) * 86400},
	}
	for _, c := range cases {
		got, err := daysToSeconds(c.days)
		if err != nil {
			t.Fatalf("days=%d 不应报错：%v", c.days, err)
		}
		if got != c.want {
			t.Fatalf("days=%d 得到 %d，期望 %d", c.days, got, c.want)
		}
	}
}

// TestDaysToSecondsRejectsAboveCap 上限可过、上限+1 必须报错。
func TestDaysToSecondsRejectsAboveCap(t *testing.T) {
	if _, err := daysToSeconds(maxRetentionDays); err != nil {
		t.Fatalf("上限 %d 天应可过，实际报错：%v", maxRetentionDays, err)
	}
	if got, err := daysToSeconds(maxRetentionDays + 1); err == nil {
		t.Fatalf("上限+1 应报错，实际通过（返回 %d）", got)
	}
}

// TestDaysToSecondsRejectsOverflowScale 会触发 int64 溢出的巨量天数必须被拦下。
//
// 这是 R27 的核心：int64(days)*86400 在 days > ~1.07e14 时溢出。若不做上限检查，
// 溢出后的值参与 now-span 会得到【未来】时间戳，清理任务将误删全表。
func TestDaysToSecondsRejectsOverflowScale(t *testing.T) {
	// 这些值若裸乘都会溢出 int64
	for _, d := range []int{1 << 47, 1 << 50, math.MaxInt32, math.MaxInt32 * 1024} {
		if got, err := daysToSeconds(d); err == nil {
			t.Fatalf("days=%d 会溢出却未被拦截（返回 %d）", d, got)
		}
	}
	// 且上限本身绝不会溢出（证明上限取值是安全的）
	if int64(maxRetentionDays) > math.MaxInt64/secondsPerDay {
		t.Fatalf("maxRetentionDays=%d 本身就会溢出，上限取值不安全", maxRetentionDays)
	}
}

func TestRetentionCutoffRejectsBadNow(t *testing.T) {
	for _, now := range []int64{0, -1, -86400} {
		if got, err := retentionCutoff(30, now); err == nil {
			t.Fatalf("now=%d 应报错，实际通过（返回 %d）", now, got)
		}
	}
}

// TestRetentionCutoffNeverInFuture 截止时刻永远不得落到"未来"——
// 否则清理会误伤刚写入的数据。
func TestRetentionCutoffNeverInFuture(t *testing.T) {
	now := int64(1_700_000_000)
	// 只取在当前 now 下不会早于 Unix 纪元的天数（now/86400 ≈ 19675 天）
	for _, d := range []int{1, 7, 30, 365, 3650, 10000} {
		cutoff, err := retentionCutoff(d, now)
		if err != nil {
			t.Fatalf("days=%d 不应报错：%v", d, err)
		}
		if cutoff > now {
			t.Fatalf("days=%d 的截止时间 %d 落到了未来（now=%d）——会导致误删", d, cutoff, now)
		}
		if cutoff < 0 {
			t.Fatalf("days=%d 的截止时间 %d 为负——语义非法", d, cutoff)
		}
	}
}

// TestRetentionCutoffCapMayExceedEpoch 记录一个已知边界：
// 上限 36500 天（100 年）的跨度 3.15e9 秒大于当前 Unix 时间戳（约 1.7e9），
// 因此"保留 100 年"在当下会走到"截止时间早于纪元"分支而【显式报错】。
//
// 这是刻意选择：宁可对一个不现实的保留期明确失败，也不返回一个会被
// 误读成"可以删"的截止时间。待时间戳推进到 3.15e9（约公元 2070 年）后
// 该组合自然可用；业务实际取值（回收站 30 天、审计 365 天）远低于此界。
func TestRetentionCutoffCapMayExceedEpoch(t *testing.T) {
	now := int64(1_700_000_000)
	if _, err := retentionCutoff(maxRetentionDays, now); err == nil {
		t.Fatal("保留 100 年使截止时间早于 Unix 纪元，应显式报错而非返回可用截止时间")
	}
	// 业务真实取值必须不受影响
	for _, d := range []int{30, 365} {
		if _, err := retentionCutoff(d, now); err != nil {
			t.Fatalf("days=%d（业务真实取值）不应报错：%v", d, err)
		}
	}
}

// TestRetentionCutoffRejectsEpochUnderflow 截止时间早于 Unix 纪元时显式报错，
// 而不是返回 0（返回 0 会匹配 deleted_at=0 的行，语义完全相反）。
func TestRetentionCutoffRejectsEpochUnderflow(t *testing.T) {
	// now 很小而跨度很大 → now - span < 0
	if got, err := retentionCutoff(maxRetentionDays, 1000); err == nil {
		t.Fatalf("截止时间早于纪元应报错，实际通过（返回 %d）", got)
	}
}

func TestExpiryAfterRejectsBadInput(t *testing.T) {
	if _, err := expiryAfter(0, 1_700_000_000); err == nil {
		t.Fatal("days=0 应报错")
	}
	if _, err := expiryAfter(7, 0); err == nil {
		t.Fatal("now=0 应报错")
	}
	if _, err := expiryAfter(maxRetentionDays+1, 1_700_000_000); err == nil {
		t.Fatal("超过上限的天数应报错")
	}
}

func TestExpiryAfterForwardIsCorrect(t *testing.T) {
	now := int64(1_700_000_000)
	got, err := expiryAfter(7, now)
	if err != nil {
		t.Fatalf("不应报错：%v", err)
	}
	if want := now + 7*86400; got != want {
		t.Fatalf("得到 %d，期望 %d", got, want)
	}
}
