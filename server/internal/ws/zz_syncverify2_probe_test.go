package ws

// GPT 第二轮回归验证探针（reviewer-sync / SYNC-06/07/05/08 回修复核）。
//
// 临时验证用例（zz_ 前缀，验证完成后可整体删除），不改任何生产代码。
// 重点攻击软件工程师-5 自报数据之外的角度：
//  1. SYNC-06：持久整帧快照是否真的消除震荡、且 stale-only 的省查询是否保住；
//  2. SYNC-07：gen 值改由全局单调计数器分配后，单调性/0 语义/溢出检测/撤权是否成立；
//  3. SYNC-07 的溢出序号快照携带是否会在重试轮把已发生的溢出事件"吸收"掉（漏检）；
//  4. SYNC-05：regUID 记账 + 改绑拒绝的边界（含再注册、零令牌零查库）；
//  5. SYNC-08：桶表是否严格 ≤4096、既有 uid 是否受影响；
//  6. 重试扣令牌的计费是否仍然精确。

import (
	"errors"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// SYNC-06：持久整帧快照
// ---------------------------------------------------------------------------

// zz2ChurnOnly1 只对账本 1 制造权限持续变更的 checker（账本 2 保持稳定，
// 这样才能把"一次 churning 订阅消耗的令牌数"与"后续正常订阅"分开计量）。
type zz2ChurnOnly1 struct {
	hub   *Hub
	calls int64
}

func (z *zz2ChurnOnly1) check(uid, bookID int64) (bool, error) {
	z.calls++
	if bookID == 1 {
		z.hub.mu.Lock()
		z.hub.bumpGenerationLocked(bookID)
		z.hub.mu.Unlock()
	}
	return true, nil
}

// TestZZ2Sync06RetryTokenAccountingPreserved 重试扣令牌的计费必须精确：
// 一次 churning 订阅消耗 1 + (maxSubscribeAttempts-1) 个令牌，
// 同一 uid 剩余额度必须恰好减少这么多（不多不少）。
func TestZZ2Sync06RetryTokenAccountingPreserved(t *testing.T) {
	chk := &zz2ChurnOnly1{hub: nil}
	hub := NewHub(chk.check)
	chk.hub = hub
	c := NewClient()
	if err := hub.Register(c); err != nil {
		t.Fatalf("注册失败：%v", err)
	}
	if err := hub.Subscribe(c, []int64{1}, 100); !errors.Is(err, ErrSubscribeGenerationStale) {
		t.Fatalf("应耗尽重试：%v", err)
	}
	if got := chk.calls; got != int64(maxSubscribeAttempts) {
		t.Fatalf("前置条件失败：churning 订阅应查库 %d 次，实际 %d", maxSubscribeAttempts, got)
	}
	// 剩余额度 = burst - maxSubscribeAttempts（时间补充在微秒级循环里可忽略）
	ok, limited := 0, 0
	for i := 0; i < subscribeBurst; i++ {
		if err := hub.Subscribe(c, []int64{2}, 100); errors.Is(err, ErrSubscribeRateLimited) {
			limited++
		} else {
			ok++
		}
	}
	want := subscribeBurst - maxSubscribeAttempts
	if ok != want {
		t.Fatalf("重试计费不精确：一次 churning 订阅应消耗 %d 个令牌（剩余可用 %d），实际可用 %d / 限流 %d",
			maxSubscribeAttempts, want, ok, limited)
	}
}

// ---------------------------------------------------------------------------
// SYNC-07：全局单调计数器 + 溢出序号
// ---------------------------------------------------------------------------

// TestZZ2Sync07GenValuesGloballyMonotonic gen 值必须全局单调：
// 任何账本的值一旦登记就永不归零、永不复用；不同账本的值也严格递增
// （全局计数器语义）；0 唯一表示"从未发生过权限事件"。
func TestZZ2Sync07GenValuesGloballyMonotonic(t *testing.T) {
	hub := NewHub(func(uid, bookID int64) (bool, error) { return true, nil })
	if got := hub.Generation(42); got != 0 {
		t.Fatalf("从未登记过事件的账本 gen 必须为 0，实际 %d", got)
	}
	last := uint64(0)
	for round := 0; round < 50; round++ {
		bookID := int64(round%3 + 1) // 三个账本交替登记
		hub.RefreshVisibility(bookID)
		got := hub.Generation(bookID)
		if got == 0 {
			t.Fatalf("账本 %d 登记后 gen 不得为 0（0 = 从未登记）", bookID)
		}
		if got <= last {
			t.Fatalf("gen 值必须全局严格递增（不可复用/归零）：上次 %d，本次账本 %d = %d", last, bookID, got)
		}
		last = got
	}
	// 未登记过的账本仍是 0
	if got := hub.Generation(999); got != 0 {
		t.Fatalf("未登记账本的 gen 应为 0，实际 %d", got)
	}
}

// zz2SimulateOverflowEvent 模拟"gen 表已触顶时，某个新账本发生了权限事件"。
//
// 这正是 bumpGenerationLocked 在触顶状态下执行的分支（hub.go:492-495）：
// 不登记条目、只推进溢出序号。这里直接操作内部状态，是为了在不真实填满
// 1<<20 张表的前提下精确验证溢出检测逻辑。
func zz2SimulateOverflowEvent(hub *Hub) {
	hub.mu.Lock()
	hub.genSeq++
	hub.genOverflow = hub.genSeq
	hub.mu.Unlock()
}

// TestZZ2Sync07OverflowAdvanceInvalidatesWholeFrame 溢出序号推进必须作废【整帧】：
// 快照里的溢出序号与当前不一致时，writeSubscribe 应把整帧判 stale（fail-closed），
// 重验后落定 —— 而不是只重验部分账本。
func TestZZ2Sync07OverflowAdvanceInvalidatesWholeFrame(t *testing.T) {
	gated := newR50GatedChecker(2)
	gated.allow(1, 100, true)
	gated.allow(2, 100, true)
	hub := NewHub(gated.check)
	// 模拟"表已满、且已发生 1000 次权限事件"的起点状态
	hub.mu.Lock()
	hub.genSeq, hub.genOverflow = 1000, 1000
	hub.mu.Unlock()

	c := NewClient()
	c.UID = 100
	if err := hub.Register(c); err != nil {
		t.Fatalf("注册失败：%v", err)
	}
	done := make(chan error, 1)
	go func() { done <- hub.Subscribe(c, []int64{1, 2}, 100) }()
	<-gated.entered // round 0 已取到两个账本的结论、停在写入前
	zz2SimulateOverflowEvent(hub)
	close(gated.release)

	if err := <-done; err != nil {
		t.Fatalf("溢出推进后订阅应重验并落定：%v", err)
	}
	if got := gated.callCount(); got != 4 {
		t.Fatalf("溢出推进必须作废整帧：期望查库 4 次（首轮 2 + 整帧重验 2），实际 %d", got)
	}
	if !c.Subscribed(1) || !c.Subscribed(2) {
		t.Fatalf("重验后两账本都应订阅成功：1=%v 2=%v", c.Subscribed(1), c.Subscribed(2))
	}
}

// TestZZ2Sync07OverflowChurnDoesNotMaskRevocation 触顶 + 溢出推进之后，
// 撤权必须仍然生效（SYNC-07 要修的核心场景在新机制下的回归）。
func TestZZ2Sync07OverflowChurnDoesNotMaskRevocation(t *testing.T) {
	gated := newR50GatedChecker(2)
	gated.allow(1, 100, true)
	gated.allow(2, 100, true)
	hub := NewHub(gated.check)
	hub.mu.Lock()
	hub.genSeq, hub.genOverflow = 1000, 1000
	hub.mu.Unlock()

	c := NewClient()
	c.UID = 100
	if err := hub.Register(c); err != nil {
		t.Fatalf("注册失败：%v", err)
	}
	done := make(chan error, 1)
	go func() { done <- hub.Subscribe(c, []int64{1, 2}, 100) }()
	<-gated.entered
	gated.revoke(1, 100)          // 撤权（checker 层生效）
	zz2SimulateOverflowEvent(hub) // 触顶状态下同期的其它权限事件
	close(gated.release)

	if err := <-done; err != nil {
		t.Fatalf("订阅应重验后落定：%v", err)
	}
	if c.Subscribed(1) {
		t.Fatal("SYNC-07 回归：触顶 + 溢出推进之后，撤销仍被抹掉了（已撤销成员订阅成功）")
	}
	if !c.Subscribed(2) {
		t.Fatal("无权变化的账本 2 应订阅成功")
	}
}

// ---------------------------------------------------------------------------
// SYNC-07 遗留怀疑：溢出序号在重试轮被覆盖为最新值（代码推导，未做成确定性复现）
// ---------------------------------------------------------------------------

// TestZZ2Sync09OverflowAbsorbedByRetrySnapshot 【理论缺陷 P3 —— 代码推导，无法确定性复现】
//
// 你问的"溢出推进了但某条 in-flight 快照没带上溢出序号，导致漏检"——方向反了：
// 实现是**每一轮都把 snap.overflow 覆盖成本轮快照的最新值**（hub.go:622-625），
// 所以不存在"没带上"；真正的洞是反过来——**正因为每轮覆盖成最新值，
// 重试轮会把"上一轮快照之后、本轮快照之前"发生的溢出类事件吸收掉**。
//
// 精确时序（需要 gen 表触顶，即 1<<20 个账本发生过权限事件，家庭场景实际不可达）：
//
//	t0 round0 快照 overflow=O0；账本 1 的结论在此刻取得（allow=true）；
//	t1 round0 写入前比对：账本 2 的 per-book gen 变了 → stale=[2]（溢出未变，不触发整帧）；
//	t2 账本 1 被撤权 —— 它不在 gen 表内，事件只能推进溢出序号 O0→O1；
//	t3 round1 快照：snap.overflow 被覆盖成 O1（事件被吸收）；
//	t4 round1 只重验账本 2；
//	t5 round1 写入前比对：溢出 O1==O1 ✓、账本 1 的 per-book 0==0 ✓ → 判"未变化"，
//	   把 t0 时刻的 allow=true 写回 → 已撤销成员订阅成功。
//
// 本用例只能验证到"这一路径的前半段"（stale 由 per-book 触发、溢出在重试轮被覆盖），
// t2 的注入窗口落在两轮之间，测试侧无法在不 hook 内部的情况下确定性命中，
// 因此这里只固化前置条件并留注释；修复建议见下。
//
// 修复建议（一行级）：snap.overflow 只在【本轮 pending 覆盖整帧】时才更新为最新值。
// 因为溢出类事件无法归因到具体账本，只有整帧重验之后才能安全地吸收最新溢出序号；
// 否则它必须继续作废整帧（fail-closed）。
func TestZZ2Sync09OverflowAbsorbedByRetrySnapshot(t *testing.T) {
	gated := newR50GatedChecker(2)
	gated.allow(1, 100, true)
	gated.allow(2, 100, true)
	hub := NewHub(gated.check)

	hub.RefreshVisibility(2) // 账本 2 已登记（在表内）；账本 1 从未登记（不在表内）
	c := NewClient()
	c.UID = 100
	if err := hub.Register(c); err != nil {
		t.Fatalf("注册失败：%v", err)
	}
	done := make(chan error, 1)
	go func() { done <- hub.Subscribe(c, []int64{1, 2}, 100) }()
	<-gated.entered
	hub.RefreshVisibility(2) // 触发账本 2 stale → 重试只含账本 2
	close(gated.release)

	if err := <-done; err != nil {
		t.Fatalf("订阅应落定：%v", err)
	}
	// 前半段验证：本轮确实只重验了账本 2（查库 2 + 1 = 3 次）
	if got := gated.callCount(); got != 3 {
		t.Fatalf("前置条件失败：应只重验账本 2（查库 3 次），实际 %d", got)
	}
	if !c.Subscribed(1) || !c.Subscribed(2) {
		t.Fatalf("两账本都应订阅成功：1=%v 2=%v", c.Subscribed(1), c.Subscribed(2))
	}
	t.Log("已固化前置路径：stale 仅由 per-book 触发、溢出序号在重试轮被覆盖为最新值。" +
		"t2 注入窗口在两轮之间，需 hook 内部才能确定性复现（见函数注释的时序推导）。")
}

// ---------------------------------------------------------------------------
// SYNC-05：regUID 记账 + 改绑拒绝
// ---------------------------------------------------------------------------

// zz2HookChecker 可注入副作用的 checker（用于验证"改绑请求零查库零扣令牌"）。
type zz2HookChecker struct {
	inner *fakeChecker
	hub   *Hub
	calls int64
}

func (z *zz2HookChecker) check(uid, bookID int64) (bool, error) {
	z.hub.mu.Lock()
	z.hub.genSeq++ // 每次查库都推进计数器，便于断言"确实没查库"
	z.hub.mu.Unlock()
	z.inner.mu.Lock()
	z.inner.calls++
	z.inner.mu.Unlock()
	return z.inner.check(uid, bookID)
}

// TestZZ2Sync05UIDMismatchRejectedBeforeCosts 改绑请求必须在扣令牌/查库之前被拒。
func TestZZ2Sync05UIDMismatchRejectedBeforeCosts(t *testing.T) {
	chk := &zz2HookChecker{inner: newFakeChecker(), hub: nil}
	chk.inner.allow(1, 99, true)
	hub := NewHub(chk.check)
	chk.hub = hub
	hub.SetChecker(chk.check)

	c := NewClient()
	c.UID = 99
	if err := hub.Register(c); err != nil {
		t.Fatalf("注册失败：%v", err)
	}
	if err := hub.Subscribe(c, []int64{1}, 99); err != nil {
		t.Fatalf("正常订阅应成功：%v", err)
	}
	before := chk.inner.callCount()
	if err := hub.Subscribe(c, []int64{1}, 100); !errors.Is(err, ErrSubscribeUIDMismatch) {
		t.Fatalf("改绑必须被拒（ErrSubscribeUIDMismatch），实际 %v", err)
	}
	if got := chk.inner.callCount(); got != before {
		t.Fatalf("改绑请求不得查库：之前 %d 次，之后 %d 次", before, got)
	}
	// 改绑也不得扣令牌：同一 uid 连发 burst 帧应全部可用
	for i := 0; i < subscribeBurst; i++ {
		if err := hub.Subscribe(c, []int64{1}, 100); !errors.Is(err, ErrSubscribeUIDMismatch) {
			t.Fatalf("第 %d 帧改绑仍应被拒且不扣令牌：%v", i+1, err)
		}
	}
}

// TestZZ2Sync05RebindAfterUnregisterIsCapped 再注册边界：
// UID=0 起家的连接绑定 uid=99 后注销（按 regUID=0 回收，不动任何人的计数）；
// 再次注册时 regUID 取当前 c.UID=99 —— 此时它**必须**占用 uid=99 的连接额度，
// 否则就变成另一条泄漏路径（同一逻辑连接反复注册却从不计数）。
func TestZZ2Sync05RebindAfterUnregisterIsCapped(t *testing.T) {
	chk := newFakeChecker()
	chk.allow(1, 99, true)
	hub := NewHub(chk.check)
	hub.SetConnectionLimits(1, 100)

	zero := NewClient() // UID=0
	if err := hub.Register(zero); err != nil {
		t.Fatalf("UID=0 注册应成功：%v", err)
	}
	if err := hub.Subscribe(zero, []int64{1}, 99); err != nil {
		t.Fatalf("首次订阅绑定 uid=99 应成功：%v", err)
	}
	hub.Unregister(zero) // regUID=0 → 不回收任何人的连接额度（正确）

	// 再注册：regUID 取当前 c.UID=99，必须占用额度
	if err := hub.Register(zero); err != nil {
		t.Fatalf("再注册应成功：%v", err)
	}
	other := NewClient()
	other.UID = 99
	if err := hub.Register(other); !errors.Is(err, ErrTooManyConnectionsForUser) {
		t.Fatalf("再注册的连接必须占用 uid=99 的连接额度（否则同一连接可反复注册绕过上限），实际 %v", err)
	}
	// 注销时按 regUID=99 回收，额度正确释放
	hub.Unregister(zero)
	if err := hub.Register(other); err != nil {
		t.Fatalf("注销后额度应释放：%v", err)
	}
}

// ---------------------------------------------------------------------------
// SYNC-08：桶表严格有界
// ---------------------------------------------------------------------------

// TestZZ2Sync08BucketTableStrictlyBounded 4096 个活跃桶占满后：
// 新 uid 必须被 fail-closed 拒绝、桶表不越界；已有 uid 不受影响。
func TestZZ2Sync08BucketTableStrictlyBounded(t *testing.T) {
	hub := NewHub(func(uid, bookID int64) (bool, error) { return true, nil })
	for i := 0; i < maxSubBuckets; i++ {
		if !hub.consumeSubToken(int64(i + 1)) {
			t.Fatalf("第 %d 个 uid 首次消费不应被拒", i+1)
		}
	}
	if len(hub.subBuckets) != maxSubBuckets {
		t.Fatalf("前置条件失败：应恰好 %d 个桶，实际 %d", maxSubBuckets, len(hub.subBuckets))
	}
	// 第 4097 个 uid：必须被拒（fail-closed），桶表不得越界
	if hub.consumeSubToken(int64(maxSubBuckets + 1)) {
		t.Fatal("SYNC-08 回归：满表且无可回收对象时，新 uid 的订阅令牌必须被拒绝")
	}
	if len(hub.subBuckets) > maxSubBuckets {
		t.Fatalf("桶表越界：%d > %d", len(hub.subBuckets), maxSubBuckets)
	}
	// 已有 uid 不受影响：仍可消费
	if !hub.consumeSubToken(int64(1)) {
		t.Fatal("既有 uid 不得被满表误伤")
	}
	// 释放一个桶（等价于该 uid 的桶回到满桶被回收）后，新 uid 应能进入
	hub.subBuckets[int64(2)] = &subBucket{tokens: subscribeBurst, last: time.Now()}
	if !hub.consumeSubToken(int64(maxSubBuckets + 1)) {
		t.Fatal("出现可回收满桶后，新 uid 应能获得令牌（fail-closed 不是永久封禁）")
	}
	if len(hub.subBuckets) > maxSubBuckets {
		t.Fatalf("回收后桶表仍越界：%d > %d", len(hub.subBuckets), maxSubBuckets)
	}
}
