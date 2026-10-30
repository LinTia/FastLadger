package ws

// SYNC-01 ~ SYNC-04（独立复审 · WebSocket 放大面）的定向回归。
//
// 四条缺陷与修复的对应关系：
//
//	SYNC-01（P1）单用户可无限放大：连接无上限 + 订阅限流按连接计 + 配额检查在鉴权之后
//	  → Register 加 per-uid / 全局连接上限（config 注入，超限拒绝并回 4002 关闭帧）；
//	    订阅令牌桶改为按 uid 计量；配额预检移到任何查库之前。
//	SYNC-02（P2）超限帧不扣令牌：normalize 在扣令牌之前，解析开销最大的路径反而不限速
//	  → 对调顺序：先扣令牌再 normalize；被限流帧依旧零查库。
//	SYNC-03（P2）generation 重试放大：整帧重跑鉴权而整帧只扣 1 个令牌
//	  → 重试只重新鉴权 generation 变过的账本 + 每次重试也扣令牌。
//	SYNC-04（P3）hub.gen 无回收 → gen 表设条目上限，触顶回收（删除条目与递增等价，
//	    只会让 in-flight 结论被重新验证，方向安全）。
//
// 并发同步一律 channel barrier（确定性），禁止 sleep。

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestSync01RegisterRejectsBeyondPerUserConnCap 单用户连接数上限：第 N+1 条被拒、别的 uid 不受影响
func TestSync01RegisterRejectsBeyondPerUserConnCap(t *testing.T) {
	hub := NewHub(func(uid, bookID int64) (bool, error) { return true, nil })
	hub.SetConnectionLimits(3, 100)

	for i := 0; i < 3; i++ {
		c := NewClient()
		c.UID = 777
		if err := hub.Register(c); err != nil {
			t.Fatalf("同 uid 第 %d 条连接不应被拒：%v", i+1, err)
		}
	}
	fourth := NewClient()
	fourth.UID = 777
	if err := hub.Register(fourth); !errors.Is(err, ErrTooManyConnectionsForUser) {
		t.Fatalf("同 uid 第 4 条连接应被拒（ErrTooManyConnectionsForUser），实际 %v", err)
	}
	if got := hub.Count(); got != 3 {
		t.Fatalf("被拒的连接不得进入注册表，实际在线 %d", got)
	}

	// 别的 uid 不受该用户的额度影响
	other := NewClient()
	other.UID = 778
	if err := hub.Register(other); err != nil {
		t.Fatalf("其它 uid 不应被别人的连接数挡住：%v", err)
	}

	// 断开一条后额度释放，可再次接入
	hub.Unregister(other)
	again := NewClient()
	again.UID = 779
	if err := hub.Register(again); err != nil {
		t.Fatalf("注销释放额度后应能再次注册：%v", err)
	}
}

// TestSync01RegisterRejectsBeyondGlobalConnCap 全局连接数上限：任何 uid 的第 N+1 条都被拒
func TestSync01RegisterRejectsBeyondGlobalConnCap(t *testing.T) {
	hub := NewHub(func(uid, bookID int64) (bool, error) { return true, nil })
	hub.SetConnectionLimits(100, 3)

	for i := 0; i < 3; i++ {
		c := NewClient()
		c.UID = int64(900 + i) // 每条都是不同 uid：只能被全局上限拦住
		if err := hub.Register(c); err != nil {
			t.Fatalf("全局上限内第 %d 条连接不应被拒：%v", i+1, err)
		}
	}
	c := NewClient()
	c.UID = 999
	if err := hub.Register(c); !errors.Is(err, ErrTooManyConnections) {
		t.Fatalf("超出全局上限应被拒（ErrTooManyConnections），实际 %v", err)
	}
	if got := hub.Count(); got != 3 {
		t.Fatalf("被拒的连接不得进入注册表，实际在线 %d", got)
	}
}

// TestSync01ConnectionCapE2ERejectedWithCloseCode 真实连接端到端：超限连接在握手后
// 立即收到关闭码 4002，且不得进入注册表。
func TestSync01ConnectionCapE2ERejectedWithCloseCode(t *testing.T) {
	old := DefaultHub
	hub := NewHub(func(uid, bookID int64) (bool, error) { return true, nil })
	hub.SetConnectionLimits(2, 100)
	DefaultHub = hub
	t.Cleanup(func() { DefaultHub = old })

	srv := newQATestServer(t)
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"

	var conns []*websocket.Conn
	for i := 0; i < 2; i++ {
		conn, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			t.Fatalf("上限内第 %d 条连接不应被拒：%v", i+1, err)
		}
		conns = append(conns, conn)
	}
	waitCount(t, hub, 2)

	// 第 3 条：HTTP 升级已完成（Dial 成功），随后被服务端以 4002 关闭
	third, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("超限连接应在建立后拒绝（而非握手失败），实际 %v", err)
	}
	_ = third.SetReadDeadline(time.Now().Add(3 * time.Second))
	var closeErr error
	for {
		if _, _, readErr := third.ReadMessage(); readErr != nil {
			closeErr = readErr
			break
		}
	}
	if !websocket.IsCloseError(closeErr, closeTooManyConnections) {
		t.Fatalf("超限连接应以关闭码 %d 被关闭（客户端据此退避重连），实际 %v",
			closeTooManyConnections, closeErr)
	}
	if got := hub.Count(); got != 2 {
		t.Fatalf("被拒连接不得进入注册表，实际在线 %d", got)
	}

	for _, c := range conns {
		_ = c.Close()
	}
	rCloseWSAndDrain(t, nil, hub)
}

// TestSync01QuotaRejectedFrameDoesZeroAuthQueries 配额检查必须先于鉴权查询：
// 配额拒绝的帧一次库都不查（修复前"先查库、后配额"，被拒帧照样烧掉 64 次查询）。
func TestSync01QuotaRejectedFrameDoesZeroAuthQueries(t *testing.T) {
	hub := NewHub(func(uid, bookID int64) (bool, error) { return true, nil })

	// ---- 阶段 1：用"无条件放行"的 checker 把两个维度的配额填满（不计查询数）----
	// 用户维度：9 条连接 × 64 = 576（600 上限内；单连接上限 200 决定了
	// 一条连接最多 192，所以必须多条连接）
	for i := 0; i < MaxSubscriptionsPerUser/MaxBookIDsPerFrame; i++ {
		c := NewClient()
		c.UID = 100
		if err := hub.Register(c); err != nil {
			t.Fatalf("第 %d 条连接不应被拒：%v", i+1, err)
		}
		if err := hub.Subscribe(c, reviewIDs(int64(i*MaxBookIDsPerFrame+1), MaxBookIDsPerFrame), 100); err != nil {
			t.Fatalf("填充第 %d 帧不应被拒：%v", i+1, err)
		}
	}
	// 连接维度：uid=200 的一条连接填到 192（200 上限内）
	tenth := NewClient()
	tenth.UID = 200
	if err := hub.Register(tenth); err != nil {
		t.Fatalf("注册失败：%v", err)
	}
	for i := 0; i < MaxSubscriptionsPerClient/MaxBookIDsPerFrame; i++ {
		if err := hub.Subscribe(tenth, reviewIDs(int64(i*MaxBookIDsPerFrame+1), MaxBookIDsPerFrame), 200); err != nil {
			t.Fatalf("填充第 %d 帧不应被拒：%v", i+1, err)
		}
	}

	// ---- 阶段 2：换上**计数** checker —— 以下两个被拒帧必须一次库都不查 ----
	chk := newFakeChecker() // 什么都没 allow：若真的查了库，callCount 必然增长
	hub.SetChecker(chk.check)

	// 用户维度：576 + 64 > 600 → 必须在查库之前被拒
	eleventh := NewClient()
	eleventh.UID = 100
	if err := hub.Register(eleventh); err != nil {
		t.Fatalf("新连接不应被拒（本用例未配置连接上限）：%v", err)
	}
	err := hub.Subscribe(eleventh, reviewIDs(1, MaxBookIDsPerFrame), 100)
	if !errors.Is(err, ErrSubscriptionLimitPerUser) {
		t.Fatalf("应因单用户订阅配额被拒，实际 %v", err)
	}
	// 连接维度：192 + 64 > 200 → 同样必须在查库之前被拒
	err = hub.Subscribe(tenth, reviewIDs(MaxSubscriptionsPerClient+1, MaxBookIDsPerFrame), 200)
	if !errors.Is(err, ErrSubscriptionLimitPerConnection) {
		t.Fatalf("应因单连接订阅配额被拒，实际 %v", err)
	}
	if got := chk.callCount(); got != 0 {
		t.Fatalf("配额拒绝必须发生在查库之前（两帧合计零查库），实际查了 %d 次", got)
	}
	if got := hub.UserSubscriptionCount(100); got != MaxSubscriptionsPerUser/MaxBookIDsPerFrame*MaxBookIDsPerFrame {
		t.Fatalf("被拒帧不得改变订阅状态：uid=100 应仍为 %d 条，实际 %d",
			MaxSubscriptionsPerUser/MaxBookIDsPerFrame*MaxBookIDsPerFrame, got)
	}
}

// TestSync01RateLimitMetersPerUID 订阅限流按 uid 计量：同一 uid 的第二条连接
// 必须共享额度（修复前每条连接各自一个桶，多开连接即可绕过）。
func TestSync01RateLimitMetersPerUID(t *testing.T) {
	hub := NewHub(func(uid, bookID int64) (bool, error) { return true, nil })

	first := NewClient()
	first.UID = 7
	if err := hub.Register(first); err != nil {
		t.Fatalf("注册失败：%v", err)
	}
	for i := 0; i < subscribeBurst; i++ {
		if err := hub.Subscribe(first, []int64{int64(i + 1)}, 7); err != nil {
			t.Fatalf("第 %d 帧不应被拒：%v", i+1, err)
		}
	}

	// 同一 uid 的第二条连接：若限流仍按连接计，它会拿到一个全新的满桶
	second := NewClient()
	second.UID = 7
	if err := hub.Register(second); err != nil {
		t.Fatalf("注册失败：%v", err)
	}
	if err := hub.Subscribe(second, []int64{999}, 7); !errors.Is(err, ErrSubscribeRateLimited) {
		t.Fatalf("同一 uid 的第二条连接必须共享限流额度（SYNC-01），实际 %v", err)
	}

	// 其它 uid 有自己的桶，不受影响
	other := NewClient()
	other.UID = 8
	if err := hub.Register(other); err != nil {
		t.Fatalf("注册失败：%v", err)
	}
	if err := hub.Subscribe(other, []int64{1}, 8); err != nil {
		t.Fatalf("其它 uid 不应被别人的额度影响：%v", err)
	}
}

// TestSync02OversizedFrameConsumesToken 超限帧也要先扣令牌（SYNC-02）：
// 连发超过桶容量后必须出现限流，且全程零查库。
func TestSync02OversizedFrameConsumesToken(t *testing.T) {
	chk := newFakeChecker()
	chk.allow(1, 100, true)
	hub := NewHub(chk.check)
	c := NewClient()
	hub.Register(c)

	for i := 0; i < subscribeBurst; i++ {
		err := hub.Subscribe(c, reviewIDs(1, MaxBookIDsPerFrame+1), 100)
		if !errors.Is(err, ErrTooManyBookIDs) {
			t.Fatalf("第 %d 帧应因超量被拒（令牌照扣），实际 %v", i+1, err)
		}
	}
	// 令牌已被超量帧消耗干净：下一帧（同样是超量帧）必须被限流
	if err := hub.Subscribe(c, reviewIDs(1, MaxBookIDsPerFrame+1), 100); !errors.Is(err, ErrSubscribeRateLimited) {
		t.Fatalf("超量帧消耗令牌后，后续帧应被限流（SYNC-02），实际 %v", err)
	}
	if got := chk.callCount(); got != 0 {
		t.Fatalf("被拒帧（超量/限流）必须零查库，实际 %d 次", got)
	}
	if c.SubscriptionCount() != 0 {
		t.Fatalf("被拒帧不得留下任何订阅，实际 %d", c.SubscriptionCount())
	}
}

// TestSync03RetryOnlyReverifiesStaleBooks generation 变化后只重新鉴权"变过的账本"。
//
// 时序（channel barrier，确定性）：8 个账本的帧，闸门卡在最后一个账本的鉴权上；
// 卡住期间只有账本 1 的 generation 被推进。修复前：整帧 8 个账本全部重跑（16 次查询）；
// 修复后：只重鉴权账本 1（8 + 1 = 9 次）。
func TestSync03RetryOnlyReverifiesStaleBooks(t *testing.T) {
	const books = 8
	gated := newR50GatedChecker(books) // 闸门挂在最后一个账本上
	for b := int64(1); b <= books; b++ {
		gated.allow(b, 100, true)
	}
	hub := NewHub(gated.check)
	c := NewClient()
	hub.Register(c)

	done := make(chan error, 1)
	go func() {
		done <- hub.Subscribe(c, reviewIDs(1, books), 100)
	}()

	<-gated.entered          // 已完成 8 个账本的鉴权、停在写入前
	hub.RefreshVisibility(1) // 只推进账本 1 的 generation（无订阅者，不触发查库）
	close(gated.release)

	if err := <-done; err != nil {
		t.Fatalf("只有部分账本权限变化时订阅应当成功：%v", err)
	}
	want := books + 1 // 首轮 8 次 + 重试只补账本 1 的 1 次
	if got := gated.callCount(); got != want {
		t.Fatalf("重试只应重新鉴权 generation 变过的账本：期望 %d 次查库，实际 %d 次（整帧重跑会是 %d 次）",
			want, got, books*maxSubscribeAttempts)
	}
	for b := int64(1); b <= books; b++ {
		if !c.Subscribed(b) {
			t.Fatalf("账本 %d 应订阅成功（权限未变的账本不得被重试误伤）", b)
		}
	}
}

// TestSync03RetryConsumesToken generation 重试也要扣令牌（SYNC-03）：
// 权限持续变更时，重试的查询量必须被令牌桶的速率兜住，而不是免费放大。
func TestSync03RetryConsumesToken(t *testing.T) {
	hub := NewHub(nil)
	chk := &reviewChurningChecker{hub: hub, bookID: 1}
	hub.SetChecker(chk.check)
	c := NewClient()
	hub.Register(c)

	if err := hub.Subscribe(c, []int64{1}, 100); !errors.Is(err, ErrSubscribeGenerationStale) {
		t.Fatalf("generation 持续变化时应耗尽重试并返回明确错误，实际 %v", err)
	}
	// 这次订阅消耗了 maxSubscribeAttempts 个令牌（1 次正常 + 3 次重试）：
	// 同一 uid 接着的订阅必须更早被限流。
	limited := 0
	for i := 0; i < subscribeBurst; i++ {
		if err := hub.Subscribe(c, []int64{2}, 100); errors.Is(err, ErrSubscribeRateLimited) {
			limited++
		}
	}
	if limited == 0 {
		t.Fatal("generation 重试必须扣令牌：重试耗尽的订阅之后，同一 uid 的可用额度应减少")
	}
}

// TestSync04GenerationMapBounded gen 表有界（SYNC-04）：刷新远超上限数量的账本后，
// 条目数不再增长；且回收不得破坏 R50 的不变量（撤权仍然生效）。
func TestSync04GenerationMapBounded(t *testing.T) {
	chk := newFakeChecker()
	chk.allow(7, 1, true)
	hub := NewHub(chk.check)

	for b := int64(1); b <= maxGenerationEntries+500; b++ {
		hub.RefreshVisibility(b)
	}
	if got := hub.GenerationCount(); got > maxGenerationEntries {
		t.Fatalf("gen 表必须有界（≤%d），实际 %d（修复前只增不减）", maxGenerationEntries, got)
	}

	// 回收条目等价于一次 generation 变更，方向安全：撤权语义不受影响
	c := NewClient()
	c.UID = 7
	hub.Register(c)
	if err := hub.Subscribe(c, []int64{1}, 7); err != nil {
		t.Fatalf("订阅失败：%v", err)
	}
	chk.revoke(1, 7)
	hub.RefreshVisibility(1)
	if c.Subscribed(1) {
		t.Fatal("gen 表有界化之后，撤权必须仍然生效（R50 不变量）")
	}
}
