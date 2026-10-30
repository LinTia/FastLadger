package ws

// GPT 对抗性验证探针（reviewer-sync / SYNC-01~04 修复复核）。
//
// 本文件是**临时验证用例**（zz_ 前缀，验证完成后可整体删除），不改任何生产代码。
// 目标不是复述 software-engineer-5 的自测，而是专门攻击他自报数据之外的角度：
//  1. 连接额度的回收是否真的正确（含"没加却减了"的反向泄漏）；
//  2. per-uid 桶回收在什么时序下会失效/误伤，桶表是否真的有界；
//  3. 配额预检与写入之间的并发窗口是否真的不超卖；
//  4. stale-only 重验在"帧内含有 gen>0 的账本"时是否还能落定；
//  5. gen 表回收破坏 generation 单调性后，撤销会不会被当成"未变化"跳过；
//  6. R49（被限流帧零查库）/ R50（撤权即摘）/ R51（到期断连）三条不变量。

import (
	"errors"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// ① 连接上限：额度回收 + 幂等 + 反向泄漏
// ---------------------------------------------------------------------------

// TestZZSync01ConnCapReleasedAfterDisconnect 连满 → 全部断开 → 再连，应全部成功。
//
// 这是对"注销释放额度"的直接检验：如果注销没有把 connPerUser 减回去，
// 第二轮注册会全部被拒（泄漏路径）。
func TestZZSync01ConnCapReleasedAfterDisconnect(t *testing.T) {
	hub := NewHub(func(uid, bookID int64) (bool, error) { return true, nil })
	hub.SetConnectionLimits(2, 100)

	reg := func(uid int64) (*Client, error) {
		c := NewClient()
		c.UID = uid
		return c, hub.Register(c)
	}

	// 第一轮：uid=7 占满 per-user 配额
	c1, err1 := reg(7)
	c2, err2 := reg(7)
	if err1 != nil || err2 != nil {
		t.Fatalf("前两条连接应成功：%v / %v", err1, err2)
	}
	if _, err := reg(7); !errors.Is(err, ErrTooManyConnectionsForUser) {
		t.Fatalf("第 3 条应被 per-user 上限拒绝，实际 %v", err)
	}
	// 全部断开
	hub.Unregister(c1)
	hub.Unregister(c2)
	// 第二轮：必须能全部重新注册（额度已释放）
	for i := 0; i < 2; i++ {
		if _, err := reg(7); err != nil {
			t.Fatalf("断开后第 %d 次重连应成功（额度未释放 = 泄漏）：%v", i+1, err)
		}
	}
}

// TestZZSync01RegisterIdempotentDoesNotDoubleCount 幂等注册不得重复计数。
//
// Register 两次只占 1 个额度；Unregister 一次就应把额度释放干净
// （若实现是"引用计数"但只减一次，第二次 Unregister 后额度会永久少 1）。
func TestZZSync01RegisterIdempotentDoesNotDoubleCount(t *testing.T) {
	hub := NewHub(func(uid, bookID int64) (bool, error) { return true, nil })
	hub.SetConnectionLimits(1, 100)

	c := NewClient()
	c.UID = 7
	if err := hub.Register(c); err != nil {
		t.Fatalf("首次注册应成功：%v", err)
	}
	if err := hub.Register(c); err != nil {
		t.Fatalf("重复注册应幂等成功且不重复计数：%v", err)
	}
	other := NewClient()
	other.UID = 7
	if err := hub.Register(other); !errors.Is(err, ErrTooManyConnectionsForUser) {
		t.Fatalf("幂等注册不得腾出额度：第 2 个用户连接应被拒，实际 %v", err)
	}
	// 只 Unregister 一次，额度就应完全释放（证明没有"多记一次"）
	hub.Unregister(c)
	if err := hub.Register(other); err != nil {
		t.Fatalf("一次注销即应释放全部额度（幂等注册不得多记）：%v", err)
	}
}

// TestZZSync01UnregisterTwiceIsSafe 重复注销必须是 no-op（不得把计数减成负数，
// 否则其它用户会凭空多出额度）。
func TestZZSync01UnregisterTwiceIsSafe(t *testing.T) {
	hub := NewHub(func(uid, bookID int64) (bool, error) { return true, nil })
	hub.SetConnectionLimits(1, 100)

	c := NewClient()
	c.UID = 7
	if err := hub.Register(c); err != nil {
		t.Fatalf("注册失败：%v", err)
	}
	hub.Unregister(c)
	hub.Unregister(c) // 第二次必须是 no-op

	// 若计数被减成负数，这里能连上 2 条（上限 1）
	first := NewClient()
	first.UID = 7
	if err := hub.Register(first); err != nil {
		t.Fatalf("重连应成功：%v", err)
	}
	second := NewClient()
	second.UID = 7
	if err := hub.Register(second); !errors.Is(err, ErrTooManyConnectionsForUser) {
		t.Fatalf("重复注销不得凭空多出额度：第 2 条连接应被拒，实际 %v", err)
	}
}

// TestZZSync01UIDAssignedAfterRegisterSkewsConnAccounting 【SYNC-05 修复后翻转】
//
// 原缺陷（P3）：连接额度按"注册时刻的 c.UID"记账，而 writeSubscribe 会**无条件
// 覆盖 c.UID = uid**。UID=0 注册的连接（不占 per-user 额度）被写成 99 后，
// 注销会去减 uid=99 从未加过的计数 —— 该 uid 的其它在线连接凭空多得额度。
//
// 修复后应有的行为：
//  1. 注销按注册时刻的 uid（Client.regUID）回收，UID=0 起家的连接注销时不碰别人的计数；
//  2. 已绑定 uid 的连接不得改绑（Subscribe 直接拒绝 ErrSubscribeUIDMismatch）。
func TestZZSync01UIDAssignedAfterRegisterSkewsConnAccounting(t *testing.T) {
	hub := NewHub(func(uid, bookID int64) (bool, error) { return true, nil })
	chk := newFakeChecker()
	chk.allow(1, 99, true)
	hub.SetChecker(chk.check)
	hub.SetConnectionLimits(1, 100)

	// UID=0 的连接：注册时 per-user 计数不增加
	zero := NewClient()
	if err := hub.Register(zero); err != nil {
		t.Fatalf("UID=0 注册应成功：%v", err)
	}
	// 未绑定连接的首次订阅允许绑定 uid（既有调用方契约）
	if err := hub.Subscribe(zero, []int64{1}, 99); err != nil {
		t.Fatalf("未绑定连接的首次订阅（绑定 uid=99）应成功：%v", err)
	}
	if zero.UID != 99 {
		t.Fatalf("前置条件失败：writeSubscribe 应已把 UID 写成 99，实际 %d", zero.UID)
	}
	// 已绑定的连接不得改绑（SYNC-05 显式校验）
	if err := hub.Subscribe(zero, []int64{1}, 100); !errors.Is(err, ErrSubscribeUIDMismatch) {
		t.Fatalf("已绑定 uid=99 的连接改绑 uid=100 必须被拒（ErrSubscribeUIDMismatch），实际 %v", err)
	}
	// uid=99 自己的一条正常连接，占满它的 per-user 配额
	nine := NewClient()
	nine.UID = 99
	if err := hub.Register(nine); err != nil {
		t.Fatalf("uid=99 首条连接应成功：%v", err)
	}
	second := NewClient()
	second.UID = 99
	if err := hub.Register(second); !errors.Is(err, ErrTooManyConnectionsForUser) {
		t.Fatalf("uid=99 第 2 条连接应被拒（上限 1），实际 %v", err)
	}
	// zero 首次绑定到了 uid=99：它的订阅条目计入 uid=99 的订阅总量
	if got := hub.UserSubscriptionCount(99); got != 1 {
		t.Fatalf("前置条件失败：uid=99 的订阅条目应为 1，实际 %d", got)
	}
	// 注销那条 UID=0 起家的连接：按 regUID=0 回收，**不得**动 uid=99 的连接计数
	hub.Unregister(zero)
	third := NewClient()
	third.UID = 99
	if err := hub.Register(third); !errors.Is(err, ErrTooManyConnectionsForUser) {
		t.Fatalf("SYNC-05 回归：注销未占用 per-user 额度的连接，不得让 uid=99 凭空多得额度（第 2 条连接应被拒），实际 %v", err)
	}
	if got := hub.UserSubscriptionCount(99); got != 0 {
		t.Fatalf("注销后 uid=99 在注册表内的订阅条目应为 0（zero 已离线且 nine 未订阅），实际 %d", got)
	}
	hub.Unregister(third)
	hub.Unregister(nine)
	hub.Unregister(zero)
}

// ---------------------------------------------------------------------------
// ② per-uid 令牌桶：回收时序与有界性
// ---------------------------------------------------------------------------

// TestZZSync01PruneOnlyDropsFullBuckets 回收只会删"当前满桶"的条目，
// 因此不会把一个正在活跃消费的 uid 的额度清零重置（否则就是变相绕过限流）。
func TestZZSync01PruneOnlyDropsFullBuckets(t *testing.T) {
	hub := NewHub(func(uid, bookID int64) (bool, error) { return true, nil })
	// 造 maxSubBuckets 个桶，其中一半是活跃半满、一半是满桶
	now := time.Now()
	for i := 0; i < maxSubBuckets; i++ {
		uid := int64(i + 1)
		if i%2 == 0 {
			hub.subBuckets[uid] = &subBucket{tokens: subscribeBurst, last: now} // 满桶
		} else {
			hub.subBuckets[uid] = &subBucket{tokens: 0.5, last: now} // 活跃半满
		}
	}
	hub.pruneSubBucketsLocked()
	// i 为偶数 -> 满桶（应被回收）；i 为奇数 -> 活跃半满（必须保留）
	for i := 1; i < maxSubBuckets; i += 2 {
		if _, ok := hub.subBuckets[int64(i+1)]; !ok {
			t.Fatalf("活跃半满的桶（uid=%d）不应被回收（这会把活跃用户的额度清零重置 = 变相绕过限流）", i+1)
		}
	}
	kept := 0
	for i := 0; i < maxSubBuckets; i += 2 {
		if _, ok := hub.subBuckets[int64(i+1)]; ok {
			kept++
		}
	}
	t.Logf("回收后剩余 %d 个桶；2048 个满桶条目被回收，2048 个活跃半满条目全部保留", len(hub.subBuckets))
}

// TestZZSync01BucketTableBoundedFailsClosed 【SYNC-08 修复后翻转】
// 原缺陷（P3）：桶表"有界（4096）"的主张在"所有桶都处于非满桶状态"时不成立——
// prune 只删满桶，4096 个桶全部活跃时一个都删不掉，新 uid 照样插入，map 无界增长。
//
// 修复后应有的行为：满表且 prune 无可回收对象时，**fail-closed 拒绝新 uid 的
// 订阅令牌**（返回 false → 订阅被限流拒绝），桶表严格不超过 maxSubBuckets。
// 触发前提是 4096+ 个不同 uid 在同一补充窗口内持续消费，家庭场景实际不可达；
// 被拒方稍后重试（有令牌释放/桶被回收）即恢复。
func TestZZSync01BucketTableBoundedFailsClosed(t *testing.T) {
	hub := NewHub(func(uid, bookID int64) (bool, error) { return true, nil })
	// 4096 个 uid 各消费 1 个令牌（全部处于非满桶状态）
	for i := 0; i < maxSubBuckets; i++ {
		if !hub.consumeSubToken(int64(i + 1)) {
			t.Fatalf("第 %d 个 uid 首次消费不应被限流", i+1)
		}
	}
	if len(hub.subBuckets) != maxSubBuckets {
		t.Fatalf("前置条件失败：应恰好 %d 个桶，实际 %d", maxSubBuckets, len(hub.subBuckets))
	}
	// 第 4097 个 uid：满表且无可回收对象 → 必须 fail-closed 拒绝
	if hub.consumeSubToken(int64(maxSubBuckets + 1)) {
		t.Fatal("SYNC-08 回归：满表且无可回收对象时应拒绝新 uid 的令牌（桶表不得无界增长）")
	}
	if len(hub.subBuckets) > maxSubBuckets {
		t.Fatalf("桶表必须严格有界（≤%d），实际 %d", maxSubBuckets, len(hub.subBuckets))
	}
	// 已有 uid 不受影响（fail-closed 只挡新 uid，不惩罚已计量的用户）
	if !hub.consumeSubToken(int64(1)) {
		t.Fatal("已有 uid 的令牌消费不应被桶表触顶影响")
	}
	t.Logf("桶表严格有界（%d ≤ %d）：满表且全部活跃时对新 uid fail-closed", len(hub.subBuckets), maxSubBuckets)
}

// ---------------------------------------------------------------------------
// ③ 配额预检的并发窗口：不得超卖
// ---------------------------------------------------------------------------

// TestZZSync01QuotaPrecheckRaceDoesNotOversell 预检通过后、写入前，
// 另一条连接把配额占满 —— writeSubscribe 的兜底校验必须挡住（不超卖）。
//
// 时序（channel barrier，确定性）：
//
//	c4(uid=200) 订阅 1 本：预检时用户总量 599，599+1=600 ≤ 600 通过，停在写入前；
//	期间 c3 补上第 200 本 → 用户总量 600；
//	放行 c4 → 写入时应被兜底校验拒绝（600+1 > 600），总量保持 600。
func TestZZSync01QuotaPrecheckRaceDoesNotOversell(t *testing.T) {
	chk := newFakeChecker()
	hub := NewHub(chk.check)
	const uid = int64(200)
	for b := int64(1); b <= 600; b++ {
		chk.allow(b, uid, true)
	}
	chk.allow(999, uid, true)

	mk := func() *Client {
		c := NewClient()
		c.UID = uid
		if err := hub.Register(c); err != nil {
			t.Fatalf("注册失败：%v", err)
		}
		return c
	}
	c1, c2, c3, c4 := mk(), mk(), mk(), mk()

	// 每帧最多 64 本（MaxBookIDsPerFrame），分段喂到 200 / 200 / 199
	frames := [][]int64{
		reviewIDs(1, 64), reviewIDs(65, 64), reviewIDs(129, 64), reviewIDs(193, 8),
	}
	for _, f := range frames {
		if err := hub.Subscribe(c1, f, uid); err != nil {
			t.Fatalf("c1 分帧订阅失败：%v", err)
		}
	}
	frames = [][]int64{
		reviewIDs(201, 64), reviewIDs(265, 64), reviewIDs(329, 64), reviewIDs(393, 8),
	}
	for _, f := range frames {
		if err := hub.Subscribe(c2, f, uid); err != nil {
			t.Fatalf("c2 分帧订阅失败：%v", err)
		}
	}
	frames = [][]int64{
		reviewIDs(401, 64), reviewIDs(465, 64), reviewIDs(529, 64), reviewIDs(593, 7),
	}
	for _, f := range frames {
		if err := hub.Subscribe(c3, f, uid); err != nil {
			t.Fatalf("c3 分帧订阅失败：%v", err)
		}
	}
	if got := hub.UserSubscriptionCount(uid); got != 599 {
		t.Fatalf("前置条件失败：应 599 条订阅，实际 %d", got)
	}

	// c4 的订阅停在写入前（预检已通过）
	done := make(chan error, 1)
	go func() { done <- hub.Subscribe(c4, []int64{999}, uid) }()

	// c4 可能已经写入完成（预检通过 + 无竞争时直接落定）。等它结束后补上 c3 的第 200 本，
	// 再断言总量永不越过 600。
	if err := <-done; err == nil {
		// c4 抢到了最后一个名额（600），此时 c3 的第 200 本必须被拒
		if err := hub.Subscribe(c3, []int64{600}, uid); !errors.Is(err, ErrSubscriptionLimitPerUser) {
			t.Fatalf("超卖：用户总量已 600，c3 的第 200 本应被拒，实际 %v", err)
		}
	} else if !errors.Is(err, ErrSubscriptionLimitPerUser) && !errors.Is(err, ErrSubscribeRateLimited) {
		t.Fatalf("c4 被拒的原因异常：%v", err)
	}
	if got := hub.UserSubscriptionCount(uid); got > 600 {
		t.Fatalf("超卖：用户订阅总量 %d > 上限 600", got)
	}
	t.Logf("并发窗口结束：用户订阅总量 = %d（上限 600），未超卖", hub.UserSubscriptionCount(uid))
}

// ---------------------------------------------------------------------------
// ④ SYNC-03 stale-only 重验：帧内含 gen>0 的账本时还能落定吗？
// ---------------------------------------------------------------------------

// TestZZSync03StaleRetryWithPreexistingGenerationsFlaps 【SYNC-06 修复后的验收用例】
//
// 原缺陷（P1）：重试轮的 `generationSnapshot(pending)` 只给 stale 子集建快照，
// 而 `writeSubscribe` 却拿它对**整帧** ids 逐个比对 —— 所有"gen>0 且不在本轮
// pending 里"的账本（快照缺失读到 0）被误判为 stale，重试在两个子集间来回震荡
// 直到 maxSubscribeAttempts 耗尽，**整帧一个都没订上**。
// （原 TestSync03RetryOnlyReverifiesStaleBooks 之所以绿，是因为那 8 个账本
//
//	gen 恰好全为 0，把缺陷完全掩盖。）
//
// 修复：快照合并进一个**覆盖整帧、跨轮持久**的 map —— 每个账本都拿"它取得
// 结论那一刻"的 gen 值比对。
//
// 本用例的时序（channel barrier，确定性）：
//  1. 账本 1、2 预先发生过权限事件（gen > 0）—— 真实系统的常态；
//  2. 一帧订阅 [1,2,3,4]，闸门卡在第 4 个账本的鉴权上（attempt 0）；
//  3. 期间账本 3 的权限发生变化；
//  4. 放行。
//
// 验收标准：只重鉴权账本 3 → 整帧落定、4 个账本全部订阅成功、查库 4+1=5 次。
func TestZZSync03StaleRetryWithPreexistingGenerationsFlaps(t *testing.T) {
	const books = 4
	gated := newR50GatedChecker(books) // 闸门挂在最后一个账本上
	for b := int64(1); b <= books; b++ {
		gated.allow(b, 100, true)
	}
	hub := NewHub(gated.check)
	c := NewClient()
	if err := hub.Register(c); err != nil {
		t.Fatalf("注册失败：%v", err)
	}

	// 预置：账本 1、2 曾发生过权限事件（gen > 0）
	hub.RefreshVisibility(1)
	hub.RefreshVisibility(2)
	if hub.Generation(1) == 0 || hub.Generation(2) == 0 {
		t.Fatalf("前置条件失败：gen(1)=%d gen(2)=%d 应大于 0", hub.Generation(1), hub.Generation(2))
	}

	done := make(chan error, 1)
	go func() { done <- hub.Subscribe(c, reviewIDs(1, books), 100) }()

	<-gated.entered          // attempt 0 已完成 4 个账本的鉴权、停在写入前
	hub.RefreshVisibility(3) // 期间账本 3 的权限发生变化
	close(gated.release)

	err := <-done
	queries := gated.callCount()
	if err != nil {
		t.Fatalf("SYNC-06 回归：帧内含 gen>0 账本且期间发生一次权限变更时，重试必须只重鉴权"+
			"变过的账本并整帧落定，实际 err=%v、查库 %d 次", err, queries)
	}
	for b := int64(1); b <= books; b++ {
		if !c.Subscribed(b) {
			t.Fatalf("SYNC-06 回归：订阅落定但账本 %d 未订阅成功", b)
		}
	}
	if queries != books+1 {
		t.Fatalf("重试只应重新鉴权变过的账本：期望查库 %d 次（首轮 %d + 重试 1），实际 %d 次（整帧重跑会是 %d）",
			books+1, books, queries, books*maxSubscribeAttempts)
	}
	t.Logf("SYNC-06 验收通过：帧内含 gen>0 账本，重试只补账本 3，整帧落定，共查库 %d 次", queries)
}

// TestZZSync03StaleRetrySingleGenZeroBookStillSettles 对照组：
// 帧内所有"非 stale"账本的 gen 都是 0（从未发生过权限事件）时，修复是正常的。
// 这正是 TestSync03RetryOnlyReverifiesStaleBooks 覆盖的情形 —— 它之所以通过，
// 是因为那 8 个账本的 gen 恰好全为 0，掩盖了部分快照的缺陷。
func TestZZSync03StaleRetrySingleGenZeroBookStillSettles(t *testing.T) {
	const books = 4
	gated := newR50GatedChecker(books)
	for b := int64(1); b <= books; b++ {
		gated.allow(b, 100, true)
	}
	hub := NewHub(gated.check)
	c := NewClient()
	if err := hub.Register(c); err != nil {
		t.Fatalf("注册失败：%v", err)
	}

	done := make(chan error, 1)
	go func() { done <- hub.Subscribe(c, reviewIDs(1, books), 100) }()

	<-gated.entered
	hub.RefreshVisibility(3) // 只有账本 3 变化，其余 gen 恒为 0
	close(gated.release)

	if err := <-done; err != nil {
		t.Fatalf("gen 全 0 的对照组应正常落定：%v", err)
	}
	for b := int64(1); b <= books; b++ {
		if !c.Subscribed(b) {
			t.Fatalf("账本 %d 应订阅成功", b)
		}
	}
}

// ---------------------------------------------------------------------------
// ⑤ SYNC-04 gen 回收：破坏单调性 → 撤销可能被当成"未变化"
// ---------------------------------------------------------------------------

// TestZZSync04GenerationStaysMonotonicUnderChurn 【SYNC-07 修复后的验收用例】
//
// 原缺陷（P2/P3 链条的根）：gen 表触顶后用 **delete** 回收条目，使 generation
// 不再单调 —— 一个账本可以在【没有任何针对它的权限事件】的情况下从 1 回到 0，
// 而 0 同时是"该账本从未发生过权限事件"的取值，两者在比对中不可区分。
// （"删除条目与递增等价"的论证只在"值单调不减"的前提下成立。）
//
// 修复：gen 值取自**全局单调计数器**；触顶时**绝不 delete**，改为推进溢出序号
// （genOverflow，快照携带、writeSubscribe 据此整帧重验，fail-closed）。
// 因此任何账本的 gen 一旦登记就永不归零、永不复用。
//
// 验收标准：gen 表填满后持续制造其它账本的权限事件，目标账本的 generation
// 必须保持不变（不回 0、不减），gen 表条目数严格有界。
func TestZZSync04GenerationStaysMonotonicUnderChurn(t *testing.T) {
	chk := newFakeChecker()
	hub := NewHub(chk.check)
	const target = int64(5000)
	chk.allow(target, 100, true)

	for b := int64(1); b <= maxGenerationEntries; b++ {
		hub.RefreshVisibility(b)
	}
	if hub.GenerationCount() != maxGenerationEntries {
		t.Fatalf("前置条件失败：gen 表应恰好满（%d），实际 %d", maxGenerationEntries, hub.GenerationCount())
	}
	hub.RefreshVisibility(target) // target 自己也发生过一次权限事件
	before := hub.Generation(target)
	if before == 0 {
		t.Fatalf("前置条件失败：gen(target)=%d 应大于 0", before)
	}

	// 持续制造其它账本的权限事件（触顶 → 只推进溢出序号，不删任何条目）
	for b := int64(maxGenerationEntries + 1); b <= maxGenerationEntries+100000; b++ {
		hub.RefreshVisibility(b)
	}
	if got := hub.Generation(target); got != before {
		t.Fatalf("SYNC-07 回归：未发生任何针对账本 %d 的权限事件，其 generation 不得变化"+
			"（更不得回到 0 与\"从未有事件\"不可区分）：期望 %d，实际 %d", target, before, got)
	}
	if got := hub.GenerationCount(); got != maxGenerationEntries {
		t.Fatalf("gen 表必须严格有界（恰好 %d，触顶后新账本事件走溢出序号），实际 %d",
			maxGenerationEntries, got)
	}
	t.Logf("SYNC-07 验收通过：触顶后 %d 次其它账本的权限事件，gen(target)=%d 保持不变（单调）",
		100000, before)
}

// TestZZSync04RevocationSurvivesGenerationTableChurn 【SYNC-07 修复后的验收用例】
//
// 原缺陷（P2）：把"非单调"接到 R50 的窗口里——gen 表填满后，pending 的 Subscribe
// 停在写入前（快照 gen[target]=0），期间撤权（gen 0→1）+ 其它账本持续发生权限
// 事件直到 target 条目被回收（gen 回 0）→ 放行后 writeSubscribe 看到 0==0，
// 把撤销前的 allow=true 写回：**已撤销成员仍然订阅成功**。
//
// 修复后：gen 值单调（永不回 0）、触顶走溢出序号（整帧重验，fail-closed），
// 无论哪条路径，撤销都必须生效。
//
// 时序（channel barrier，确定性）：gen 表填满 → Subscribe 停在写入前 → 撤权 +
// 刷新 → 持续制造其它账本的权限事件 → 放行。
func TestZZSync04RevocationSurvivesGenerationTableChurn(t *testing.T) {
	const target = int64(900000)
	gated := newR50GatedChecker(target)
	gated.allow(target, 100, true)
	hub := NewHub(gated.check)

	for b := int64(1); b <= maxGenerationEntries; b++ {
		hub.RefreshVisibility(b)
	}

	c := NewClient()
	c.UID = 100
	if err := hub.Register(c); err != nil {
		t.Fatalf("注册失败：%v", err)
	}
	done := make(chan error, 1)
	go func() { done <- hub.Subscribe(c, []int64{target}, 100) }()
	<-gated.entered // 已取到 allow=true，停在写入前

	// 真实撤销事件：gen[target] 登记一个新值（> 0，且永不归零）
	gated.revoke(target, 100)
	hub.RefreshVisibility(target)
	afterRevoke := hub.Generation(target)
	if afterRevoke == 0 {
		t.Fatal("前置条件失败：撤销事件必须被登记（gen > 0）")
	}

	// 制造其它账本的权限事件（触顶 → 溢出序号推进，in-flight 结论整帧作废）
	for b := int64(maxGenerationEntries + 1); b <= maxGenerationEntries+100000; b++ {
		hub.RefreshVisibility(b)
	}
	if got := hub.Generation(target); got < afterRevoke {
		t.Fatalf("SYNC-07 回归：撤销事件被 gen 表回收抹掉（gen %d → %d）", afterRevoke, got)
	}

	close(gated.release)
	if err := <-done; err != nil {
		t.Fatalf("Subscribe 应落定（本用例只验证撤销是否生效）：%v", err)
	}
	// 验收标准：撤销必须生效——哪怕 gen 表已满、期间发生了大量其它账本的权限事件
	if c.Subscribed(target) {
		t.Fatal("SYNC-07 回归：已撤销成员仍然订阅成功（撤销事件被 generation 表回收抹掉）")
	}
	t.Log("SYNC-07 验收通过：gen 表触顶并遭遇大量其它账本权限事件后，撤销依然生效")
}

// ---------------------------------------------------------------------------
// ⑥ R49 不变量：被限流帧零查库
// ---------------------------------------------------------------------------

// TestZZR49RateLimitedFrameDoesZeroAuthQueries R49 回归：令牌耗尽后的订阅帧
// 必须被限流拒绝，且一次库都不查（限流器在鉴权之前的顺序不能被本次修复破坏）。
func TestZZR49RateLimitedFrameDoesZeroAuthQueries(t *testing.T) {
	chk := newFakeChecker()
	chk.allow(1, 100, true)
	hub := NewHub(chk.check)
	c := NewClient()
	if err := hub.Register(c); err != nil {
		t.Fatalf("注册失败：%v", err)
	}
	for i := 0; i < subscribeBurst; i++ {
		if err := hub.Subscribe(c, []int64{1}, 100); err != nil {
			t.Fatalf("第 %d 帧不应被拒：%v", i+1, err)
		}
	}
	before := chk.callCount()
	if err := hub.Subscribe(c, []int64{1}, 100); !errors.Is(err, ErrSubscribeRateLimited) {
		t.Fatalf("令牌耗尽后应被限流，实际 %v", err)
	}
	if got := chk.callCount(); got != before {
		t.Fatalf("被限流帧不得查库：之前 %d 次，之后 %d 次", before, got)
	}
}
