package ws

// GPT-R50【P2】Subscribe 锁外鉴权与 RefreshVisibility 快照之间存在撤权竞态 —— 定向回归。
//
// 报告中的时序（**确定性复现，不用随机 sleep**）：
//
//	Subscribe  ── 取到 allow=true（锁外查库） ─ ─ ─ ─ ─ ─ ─ ─ ┐ 写入 BookIds
//	                                                          │
//	撤权 + RefreshVisibility ── 快照：该 client 还没订阅 → targets 为空 → 直接返回
//	                                                          │
//	                                                          └→ 旧 allow=true 落盘
//	                                                             撤权被抵消
//
// 根因：RefreshVisibility 用"订阅表"来判断谁需要重算，而 pending 的 Subscribe
// 恰恰还没进订阅表；锁外鉴权的结论又没有版本号，写回时无从判断它是否已过期。
//
// 修复：引入账本权限 generation（Hub.gen）。
//   - RefreshVisibility **即使 targets 为空也递增 generation**；
//   - Subscribe 在鉴权【前】快照 generation，写入【前】比对，不一致就丢弃旧结论并重新鉴权。
//
// 本文件用 **channel barrier**（而非 sleep）精确停在"已鉴权、未写入"那一刻。

import (
	"errors"
	"sync"
	"testing"
	"time"
)

// r50GatedChecker 带闸门的校验器：对 gateBook 的调用可以先放行/先卡住，
// 用来把 Subscribe 精确停在"已取到 allow=true、尚未写入订阅"的窗口里。
type r50GatedChecker struct {
	inner    *fakeChecker
	gateBook int64
	entered  chan struct{} // 首次进入闸门时通知（sync.Once 语义）
	release  chan struct{} // 关闭即放行
	once     sync.Once
}

func newR50GatedChecker(gateBook int64) *r50GatedChecker {
	return &r50GatedChecker{
		inner:    newFakeChecker(),
		gateBook: gateBook,
		entered:  make(chan struct{}),
		release:  make(chan struct{}),
	}
}

func (g *r50GatedChecker) allow(bookID, uid int64, canView bool) { g.inner.allow(bookID, uid, canView) }

func (g *r50GatedChecker) revoke(bookID, uid int64) { g.inner.revoke(bookID, uid) }

func (g *r50GatedChecker) callCount() int { return g.inner.callCount() }

// check 对 gateBook 的调用：先按**当时**的权限取结果，再通知调用方"我已取到结论"，
// 随后卡在闸门上 —— 这正是"锁外鉴权已完成、订阅写入尚未发生"的窗口。
func (g *r50GatedChecker) check(uid, bookID int64) (bool, error) {
	v, err := g.inner.check(uid, bookID)
	if bookID != g.gateBook {
		return v, err
	}
	g.once.Do(func() { close(g.entered) })
	<-g.release
	return v, err
}

// TestR50RevokedSubscriptionNotRestored 报告时序的确定性复现。
//
// 步骤：① Subscribe 取得 allow=true 并停在写入前 → ② 撤权 + RefreshVisibility（此时
// 订阅表为空）→ ③ 放行 Subscribe → ④ 断言最终**没有**订阅被撤权的账本。
func TestR50RevokedSubscriptionNotRestored(t *testing.T) {
	gated := newR50GatedChecker(1)
	gated.allow(1, 100, true)
	hub := NewHub(gated.check)

	c := NewClient()
	hub.Register(c)

	done := make(chan error, 1)
	go func() {
		done <- hub.Subscribe(c, []int64{1}, 100)
	}()

	// ① 等 Subscribe 取到 allow=true（此刻它还没写 BookIds）
	<-gated.entered
	if c.Subscribed(1) {
		t.Fatal("前置条件失败：Subscribe 应停在写入之前")
	}

	// ② 期间发生撤权：成员被移除，api 层调用 RefreshVisibility。
	//    此时订阅表里还没有这个 client（targets 为空），旧的 Refresh 会直接 return。
	gated.revoke(1, 100)
	hub.RefreshVisibility(1)
	if c.Subscribed(1) {
		t.Fatal("RefreshVisibility 阶段不应订阅成功")
	}

	// ③ 放行：Subscribe 拿着旧的 allow=true 去写入
	close(gated.release)
	if err := <-done; err != nil {
		t.Fatalf("撤权后重新鉴权应当安静地不订阅（返回 nil），实际 %v", err)
	}

	// ④ 断言：撤权必须生效 —— 账本 1 不得出现在订阅表里
	if c.Subscribed(1) {
		t.Fatal("撤权后订阅又被恢复：旧的 allow=true 被写回（GPT-R50 回归）")
	}
	if c.CanView(1) {
		t.Fatal("撤权后不得留下 canView=true 缓存")
	}

	// 后续广播不得送达该连接（撤权后继续收到事件 = 泄露"账本仍活跃"）
	hub.BroadcastToBook(1, []byte("full"), []byte("sensitive"))
	select {
	case m := <-c.Send:
		t.Fatalf("撤权后仍收到广播：%s", m)
	case <-waitBrief():
	}
}

// TestR50SubscribeSucceedsWithoutRevocation 对照组：同样的闸门，但期间**没有**撤权，
// 订阅必须成功。
//
// 为什么必须有这条：没有对照组，"断言订阅失败"可能只是因为闸门把流程卡坏了
// （比如 Subscribe 根本没写）。有了对照组才能证明上面的失败确实是撤权导致的。
func TestR50SubscribeSucceedsWithoutRevocation(t *testing.T) {
	gated := newR50GatedChecker(1)
	gated.allow(1, 100, true)
	hub := NewHub(gated.check)

	c := NewClient()
	hub.Register(c)

	done := make(chan error, 1)
	go func() {
		done <- hub.Subscribe(c, []int64{1}, 100)
	}()

	<-gated.entered
	// 期间没有任何权限变更 → generation 未变 → 旧结论有效
	close(gated.release)
	if err := <-done; err != nil {
		t.Fatalf("无撤权时订阅应成功，实际 %v", err)
	}
	if !c.Subscribed(1) {
		t.Fatal("无撤权时订阅应成功（对照组失败说明闸门本身有问题）")
	}
	if !c.CanView(1) {
		t.Fatal("对照组：canView 应为 true")
	}
	hub.BroadcastToBook(1, []byte("full"), []byte("sensitive"))
	select {
	case m := <-c.Send:
		if string(m) != "full" {
			t.Fatalf("对照组应收到完整载荷，实际 %s", m)
		}
	case <-waitBrief():
		t.Fatal("对照组：订阅成功却没收到广播")
	}
}

// TestR50RefreshVisibilityBumpsGenerationWithoutTargets RefreshVisibility
// **即使没有任何订阅者**也必须登记权限事件（推进该账本的 generation）。
//
// 这是修复的核心：pending 的 Subscribe 不在订阅表里，只有 generation 变化
// 才能让它的旧结论作废。若因为"没人订阅"就跳过，R50 的窗口依然存在。
//
// 注（SYNC-07 之后）：generation 值取自全局单调计数器，因此断言口径是
// "每次刷新都让值**变化**且 >0、且不影响其它账本"，而不是具体的数值。
func TestR50RefreshVisibilityBumpsGenerationWithoutTargets(t *testing.T) {
	hub := NewHub(r49AllAllowChecker) // 一个订阅者都没有

	if got := hub.Generation(1); got != 0 {
		t.Fatalf("初始 generation 应为 0，实际 %d", got)
	}
	prev := uint64(0)
	for i := 1; i <= 3; i++ {
		hub.RefreshVisibility(1)
		got := hub.Generation(1)
		if got == 0 {
			t.Fatalf("第 %d 次 RefreshVisibility 后账本 1 必须已登记权限事件（>0），实际 0", i)
		}
		if got <= prev {
			t.Fatalf("第 %d 次 RefreshVisibility 后 generation 必须严格推进：之前 %d，实际 %d", i, prev, got)
		}
		prev = got
	}
	// 别的账本不受影响（generation 按账本各自登记）
	hub.RefreshVisibility(2)
	if got := hub.Generation(1); got != prev {
		t.Fatalf("刷新账本 2 不得改动账本 1 的 generation，实际 %d（期望 %d）", got, prev)
	}
	if got := hub.Generation(2); got == 0 {
		t.Fatal("账本 2 的 generation 应已登记（>0）")
	}
	// 非法 bookID 不登记
	hub.RefreshVisibility(0)
	if got := hub.Generation(0); got != 0 {
		t.Fatalf("非法 bookID 不得登记权限事件，实际 %d", got)
	}
}

// TestR50SubscribeGenerationSnapshotInvalidatesStaleDecision 更直接的一条：
// Subscribe 停在"已鉴权、未写入"时，只要期间 RefreshVisibility 递增过 generation，
// 它就必须重新鉴权（checker 被调用第二次），而不是把第一次的结论写回。
func TestR50SubscribeGenerationSnapshotInvalidatesStaleDecision(t *testing.T) {
	gated := newR50GatedChecker(1)
	gated.allow(1, 100, true)
	hub := NewHub(gated.check)

	c := NewClient()
	hub.Register(c)

	done := make(chan error, 1)
	go func() {
		done <- hub.Subscribe(c, []int64{1}, 100)
	}()

	<-gated.entered
	hub.RefreshVisibility(1) // 仅递增 generation，权限未变
	close(gated.release)
	if err := <-done; err != nil {
		t.Fatalf("权限未变时重新鉴权应成功，实际 %v", err)
	}
	if got := gated.callCount(); got != 2 {
		t.Fatalf("generation 变化后必须重新鉴权（checker 应被调用 2 次），实际 %d 次", got)
	}
	if !c.Subscribed(1) {
		t.Fatal("权限未变：重新鉴权后应订阅成功")
	}
}

// TestR50SubscribeGivesUpWhenGenerationKeepsAdvancing generation 若被持续推进，
// 重新鉴权必须有上限（不能无限重试把唯一的 SQLite 连接占满），
// 且**不能**把过期结论写回。
func TestR50SubscribeGivesUpWhenGenerationKeepsAdvancing(t *testing.T) {
	hub := NewHub(r49AllAllowChecker)
	c := NewClient()
	hub.Register(c)

	var calls int
	// 每次鉴权都顺手推进一次 generation：让 Subscribe 永远追不上
	hub.SetChecker(func(uid, bookID int64) (bool, error) {
		calls++
		hub.RefreshVisibility(1)
		return true, nil
	})

	err := hub.Subscribe(c, []int64{1}, 100)
	if !errors.Is(err, ErrSubscribeGenerationStale) {
		t.Fatalf("generation 持续变化时应返回 ErrSubscribeGenerationStale，实际 %v", err)
	}
	if calls != maxSubscribeAttempts {
		t.Fatalf("重新鉴权次数应为 maxSubscribeAttempts=%d，实际 %d",
			maxSubscribeAttempts, calls)
	}
	if c.Subscribed(1) {
		t.Fatal("重试耗尽时不得写入任何订阅（绝不写回过期结论）")
	}
}

// waitBrief 返回一个"短暂等待"用的 channel（避免每个用例各写一遍 time.After）
func waitBrief() <-chan time.Time { return time.After(100 * time.Millisecond) }
