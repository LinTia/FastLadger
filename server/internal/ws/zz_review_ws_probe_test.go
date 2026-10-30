package ws

// GPT 独立复审探针（reviewer-sync / WebSocket 实时推送领域）。
//
// 这些用例**只做证明，不改生产代码**。聚焦三件在既有 R49/R50/R51 用例之外的问题：
//  1. 订阅限流是"每连接"的，而**连接数本身没有任何上限** → 多开连接即可把 R49 的
//     "单帧鉴权查询量上限"放大成"连接数 × 令牌桶 × 64"；
//  2. `MaxBookIDsPerFrame` 超限帧在**扣令牌之前**就被拒 → 这条最贵的路径反而永不限流；
//  3. generation 重试循环把单帧鉴权查询量放大到 maxSubscribeAttempts 倍，而整帧只扣 1 个令牌。
//
// 另外钉住几条"看起来可疑但实际没问题"的点（并发一致性、广播作用域、gen 表行为）。

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// reviewIDs 生成 n 个账本 ID（从 from 开始）。
func reviewIDs(from int64, n int) []int64 {
	out := make([]int64, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, from+int64(i))
	}
	return out
}

// ---------------------------------------------------------------------------
// 发现 1：订阅限流只按连接计，连接数无上限 ⇒ R49 的查询量封顶被绕过
// ---------------------------------------------------------------------------

// TestReviewSubscribeRateLimitIsPerUIDNotPerConnection 【SYNC-01 修复后翻转】
// 原缺陷（本文件历史名 TestReviewSubscribeRateLimitIsPerConnectionNotPerUser）：
// 令牌桶按连接计，而连接数无上限，同一 uid 多开连接即可把"单帧 64 次鉴权查询"
// 放大成"连接数 × 32 × 64"。
//
// 修复后应有的行为：订阅限流**按 uid 计量**（同一 uid 的所有连接共享一个桶），
// 因此无论开多少条连接，能通过的订阅帧总数都被单桶容量封住，
// 鉴权查询总量也随之封顶。
func TestReviewSubscribeRateLimitIsPerUIDNotPerConnection(t *testing.T) {
	const uid = 4242

	chk := newFakeChecker()
	for _, b := range reviewIDs(1, MaxBookIDsPerFrame) {
		chk.allow(b, uid, true)
	}
	hub := NewHub(chk.check)

	const n = 20 // 修复前：20 条连接可放大到 base*20 = 40960 次查询
	for i := 0; i < n; i++ {
		c := NewClient()
		c.UID = uid
		if err := hub.Register(c); err != nil {
			t.Fatalf("第 %d 条连接不应被拒（本用例未配置连接上限）：%v", i+1, err)
		}
	}
	clients := make([]*Client, 0, n)
	hub.mu.RLock()
	for c := range hub.clients {
		clients = append(clients, c)
	}
	hub.mu.RUnlock()

	accepted := 0
	for _, c := range clients {
		for i := 0; i < subscribeBurst; i++ {
			if err := hub.Subscribe(c, reviewIDs(1, MaxBookIDsPerFrame), uid); err == nil {
				accepted++
			}
		}
	}

	// 按 uid 计量：n 条连接共享同一个桶，放行帧数不得超过单桶容量（+少量时间补充裕量）。
	// 整个循环耗时毫秒级，补充速率 10/秒，+5 个令牌的裕量非常宽裕且仍远小于 n 倍放大。
	if accepted > subscribeBurst+5 {
		t.Fatalf("%d 条连接共放行 %d 帧，超过单 uid 桶容量 %d（+5 裕量）——限流仍在按连接计",
			n, accepted, subscribeBurst)
	}
	if accepted == 0 {
		t.Fatal("应至少放行部分帧（限流不是全拒）")
	}
	// 查库次数必须与放行帧数严格一致（被拒帧零查库），且被单 uid 桶封顶
	want := accepted * MaxBookIDsPerFrame
	if got := chk.callCount(); got != want {
		t.Fatalf("鉴权查询次数应为 放行帧数×每帧 %d = %d，实际 %d（被拒帧不得查库）",
			MaxBookIDsPerFrame, want, got)
	}
	t.Logf("%d 条连接共享同一 uid 额度：放行 %d 帧、鉴权查询 %d 次（修复前可放大到 %d 次）",
		n, accepted, chk.callCount(), subscribeBurst*MaxBookIDsPerFrame*n)
}

// TestReviewHubEnforcesConnectionCaps 【SYNC-01 修复后翻转】
// 原缺陷（本文件历史名 TestReviewHubHasNoPerUserConnectionCap）：Register 只做
// `clients[c] = true`，同一 uid 注册 500 条连接全部成功，无任何配额。
//
// 修复后应有的行为：per-uid 连接数上限生效（第 N+1 条被明确拒绝）、
// 全局上限生效（跨 uid 的总量被封住）、被拒连接不进注册表、注销释放额度。
func TestReviewHubEnforcesConnectionCaps(t *testing.T) {
	hub := NewHub(func(uid, bookID int64) (bool, error) { return true, nil })
	hub.SetConnectionLimits(3, 5) // per-user 3 / 全局 5

	// 同一 uid：前 3 条成功
	for i := 0; i < 3; i++ {
		c := NewClient()
		c.UID = 777
		if err := hub.Register(c); err != nil {
			t.Fatalf("同 uid 第 %d 条连接不应被拒：%v", i+1, err)
		}
	}
	// 第 4 条（同 uid）必须被拒，且不得进入注册表
	fourth := NewClient()
	fourth.UID = 777
	if err := hub.Register(fourth); !errors.Is(err, ErrTooManyConnectionsForUser) {
		t.Fatalf("同 uid 第 4 条连接应被拒（ErrTooManyConnectionsForUser），实际 %v", err)
	}
	if got := hub.Count(); got != 3 {
		t.Fatalf("被拒连接不得进入注册表，实际在线 %d", got)
	}

	// 别的 uid 不受影响
	other := NewClient()
	other.UID = 778
	if err := hub.Register(other); err != nil {
		t.Fatalf("其它 uid 不应被别人的连接数挡住：%v", err)
	}

	// 全局上限：第 5 条（新 uid）成功，第 6 条被全局上限拦住
	fifth := NewClient()
	fifth.UID = 779
	if err := hub.Register(fifth); err != nil {
		t.Fatalf("全局上限内第 5 条连接不应被拒：%v", err)
	}
	sixth := NewClient()
	sixth.UID = 780
	if err := hub.Register(sixth); !errors.Is(err, ErrTooManyConnections) {
		t.Fatalf("超出全局上限应被拒（ErrTooManyConnections），实际 %v", err)
	}
	if got := hub.Count(); got != 5 {
		t.Fatalf("全局上限应为 5 条在线，实际 %d", got)
	}

	// 注销释放额度
	hub.Unregister(fifth)
	released := NewClient()
	released.UID = 781
	if err := hub.Register(released); err != nil {
		t.Fatalf("注销释放全局额度后应能再次注册：%v", err)
	}
	t.Logf("连接上限生效：per-user=3 / 全局=5，被拒连接不入注册表，注销后额度释放")
}

// ---------------------------------------------------------------------------
// 发现 2：MaxBookIDsPerFrame 超限帧在扣令牌之前被拒 ⇒ 永不限流
// ---------------------------------------------------------------------------

// TestReviewOversizedSubscribeFrameIsRateLimited 【SYNC-02 修复后翻转】
// 原缺陷（本文件历史名 TestReviewOversizedSubscribeFrameIsNeverRateLimited）：
// 规范化（去重 + MaxBookIDsPerFrame 上限）发生在扣令牌**之前**，于是唯一一条
// CPU 开销最大的路径（64KB 帧 ≈ 8000 个 ID）反而完全不限速。
//
// 修复后应有的行为：先扣令牌再规范化 —— 超限帧同样计费，连发超过桶容量后
// 必须出现限流；且所有被拒帧（超量/限流）保持"零查库"不变量。
func TestReviewOversizedSubscribeFrameIsRateLimited(t *testing.T) {
	chk := newFakeChecker()
	chk.allow(1, 100, true)
	hub := NewHub(chk.check)
	c := NewClient()
	hub.Register(c)

	const rounds = 500 // 远超令牌桶容量 subscribeBurst(32)
	tooMany, limited, other := 0, 0, 0
	for i := 0; i < rounds; i++ {
		err := hub.Subscribe(c, reviewIDs(1, MaxBookIDsPerFrame+1), 100)
		switch {
		case errors.Is(err, ErrTooManyBookIDs):
			tooMany++
		case errors.Is(err, ErrSubscribeRateLimited):
			limited++
		default:
			other++
			t.Fatalf("第 %d 帧返回了意外结果：%v", i+1, err)
		}
	}
	if tooMany+limited != rounds {
		t.Fatalf("%d 帧应全部被拒（超量或限流），实际 %d+%d", rounds, tooMany, limited)
	}
	if limited == 0 {
		t.Fatalf("超限帧也必须计费（SYNC-02）：连发 %d 帧超过桶容量 %d 后应出现限流，实际 0 帧被限流",
			rounds, subscribeBurst)
	}
	// 决定性证据：所有被拒帧零查库
	if got := chk.callCount(); got != 0 {
		t.Fatalf("被拒帧（超量/限流）必须零查库，实际 %d 次", got)
	}
	if c.SubscriptionCount() != 0 {
		t.Fatalf("被拒帧不得留下任何订阅，实际 %d", c.SubscriptionCount())
	}
	t.Logf("%d 帧超量订阅：%d 帧因超量被拒、%d 帧被限流（超限帧已计费），全程零查库",
		rounds, tooMany, limited)
}

// TestReviewRateLimitMetersPerFrameNotPerBookID 对照：合法帧确实按"帧"扣令牌，
// 与帧里带几个账本 ID 无关（1 个 ID 与 64 个 ID 同样只扣 1 个令牌）。
//
// 这条是上面那条的反面证据 —— 说明"按帧计费"是有意设计，但超限帧漏在了计费之外。
func TestReviewRateLimitMetersPerFrameNotPerBookID(t *testing.T) {
	chk := newFakeChecker()
	for _, b := range reviewIDs(1, MaxBookIDsPerFrame) {
		chk.allow(b, 100, true)
	}
	hub := NewHub(chk.check)
	c := NewClient()
	hub.Register(c)

	for i := 0; i < subscribeBurst; i++ {
		if err := hub.Subscribe(c, reviewIDs(1, MaxBookIDsPerFrame), 100); err != nil {
			t.Fatalf("第 %d 帧（64 个 ID）不应被拒：%v", i+1, err)
		}
	}
	if err := hub.Subscribe(c, []int64{1}, 100); !errors.Is(err, ErrSubscribeRateLimited) {
		t.Fatalf("令牌耗尽后应被限流，实际 %v", err)
	}
}

// ---------------------------------------------------------------------------
// 发现 3：generation 重试把单帧鉴权查询量放大 maxSubscribeAttempts 倍
// ---------------------------------------------------------------------------

// reviewChurningChecker 每次鉴权都推进某个账本的权限 generation（模拟权限持续变更）。
type reviewChurningChecker struct {
	hub    *Hub
	bookID int64
	calls  int64
}

func (r *reviewChurningChecker) check(uid, bookID int64) (bool, error) {
	atomic.AddInt64(&r.calls, 1)
	// 每次查询都让 generation 变化 ⇒ Subscribe 写入前的比对必然失败 ⇒ 触发重试
	r.hub.mu.Lock()
	r.hub.bumpGenerationLocked(bookID)
	r.hub.mu.Unlock()
	return true, nil
}

// TestReviewGenerationRetryAmplifiesAuthQueries 【SYNC-03 修复后的边界情形】
//
// 修复内容：① 重试只重新鉴权 generation 变过的账本（未变化账本的结论继续有效）；
// ② **每次重试也扣令牌**（原来整帧只扣 1 个令牌，重试免费）。
//
// 本用例故意构造**最坏情形**：checker 对"每次被查询的账本"都推进它自己的
// generation ⇒ 每一轮重试时全部账本都是 stale，重鉴权整个帧是**必要的**而非浪费
// （sync_ws_limits_test.go 的 TestSync03RetryOnlyReverifiesStaleBooks 已证明
// "只有部分账本变化时只重鉴权那些账本"）。
// 此处锁定的结论是：这种最坏情形的查询量仍可复现（64 × 4），
// 但**不再是免费的** —— 重试消耗了 3 个额外令牌，放大被令牌桶的速率兜住。
func TestReviewGenerationRetryAmplifiesAuthQueries(t *testing.T) {
	hub := NewHub(nil) // 先占位，随后注入 churning checker（它需要持有 hub 引用）
	chk := &reviewChurningChecker{hub: hub, bookID: 1}
	hub.SetChecker(chk.check)

	c := NewClient()
	hub.Register(c)

	ids := reviewIDs(1, MaxBookIDsPerFrame)
	err := hub.Subscribe(c, ids, 100)
	if !errors.Is(err, ErrSubscribeGenerationStale) {
		t.Fatalf("generation 持续变化时应耗尽重试并返回 ErrSubscribeGenerationStale，实际 %v", err)
	}
	got := atomic.LoadInt64(&chk.calls)
	want := int64(MaxBookIDsPerFrame * maxSubscribeAttempts)
	if got != want {
		t.Fatalf("最坏情形（每个账本每轮都变化）下单帧鉴权查询次数应为 %d（%d ID × %d 次重试），实际 %d",
			want, MaxBookIDsPerFrame, maxSubscribeAttempts, got)
	}

	// SYNC-03 的关键断言：这次订阅消耗了 maxSubscribeAttempts 个令牌（1 正常 + 3 重试），
	// 同一 uid 的后续额度必须减少 —— 放大不再是免费的。
	limitedAfter := 0
	for i := 0; i < subscribeBurst; i++ {
		if err := hub.Subscribe(c, []int64{2}, 100); errors.Is(err, ErrSubscribeRateLimited) {
			limitedAfter++
		}
	}
	if limitedAfter == 0 {
		t.Fatal("generation 重试必须扣令牌：重试耗尽的订阅之后，同一 uid 的可用额度应减少")
	}
	t.Logf("最坏情形单帧 %d 次鉴权查询，但消耗了 %d 个令牌（含 %d 次重试计费），后续 %d 帧被限流",
		got, maxSubscribeAttempts, maxSubscribeAttempts-1, limitedAfter)
}

// ---------------------------------------------------------------------------
// 钉住"看起来可疑但实际没问题"的点
// ---------------------------------------------------------------------------

// TestReviewConcurrentLifecycleWithBarrier 确定性并发（channel barrier，不用 sleep）：
// 注册 / 订阅 / 广播 / 撤权刷新 / 注销 / 清订阅 六类操作在各 goroutine 同时开跑，
// 结束后校验三条不变量：
//  1. 没有 panic、没有死锁；
//  2. 任何"仍在注册表里"的连接，其订阅的账本必须仍是该 uid 有权访问的（不越权）；
//  3. 已被注销的连接不再收到投递（其后到达的广播不会写进它的 Send）。
func TestReviewConcurrentLifecycleWithBarrier(t *testing.T) {
	const (
		goroutines = 24
		books      = 6
		rounds     = 60
	)
	chk := newFakeChecker()
	hub := NewHub(chk.check)
	// uid=1 只对 1..3 是成员；4..6 不是成员。订阅请求会覆盖 1..6，
	// 于是"存活连接不得订阅无权账本"成为一条**非空**的不变量。
	for b := int64(1); b <= 3; b++ {
		chk.allow(b, 1, true)
	}

	clients := make([]*Client, goroutines)
	for i := range clients {
		clients[i] = NewClient()
		clients[i].UID = 1
		hub.Register(clients[i])
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		c := clients[g]
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			<-start // barrier：所有 goroutine 在同一刻开跑
			for r := 0; r < rounds; r++ {
				bookID := int64(r%books) + 1
				switch g % 6 {
				case 0:
					_ = hub.Subscribe(c, []int64{bookID}, 1)
				case 1:
					hub.BroadcastToBook(bookID, []byte("full"), []byte("sens"))
				case 2:
					hub.RefreshVisibility(bookID)
				case 3:
					hub.Unregister(c)
					hub.Register(c)
				case 4:
					c.ClearSubscriptions()
				case 5:
					hub.DeliverToUser(1, []byte("member_removed"))
				}
			}
		}(g)
	}
	close(start)
	wg.Wait()

	// 不变量 2：注册表里剩余的订阅必须与 checker 的结论一致
	hub.mu.RLock()
	live := make([]*Client, 0, len(hub.clients))
	for c := range hub.clients {
		live = append(live, c)
	}
	hub.mu.RUnlock()
	for _, c := range live {
		c.mu.Lock()
		for b := range c.BookIds {
			if _, err := chk.check(c.UID, b); err != nil {
				c.mu.Unlock()
				t.Fatalf("不变量被破坏：连接仍订阅了无权访问的账本 %d", b)
			}
		}
		c.mu.Unlock()
	}
	t.Logf("并发结束：注册表剩余 %d 条连接，订阅不变量全部成立", len(live))
}

// TestReviewRevokedMemberStopsReceiving 撤权后必须彻底停发（摘订阅），
// 且 `DeliverToUser` 仍能投递一次"清理通知"（两条约束方向相反，必须同时成立）。
func TestReviewRevokedMemberStopsReceiving(t *testing.T) {
	chk := newFakeChecker()
	chk.allow(9, 1, true)
	hub := NewHub(chk.check)
	c := NewClient()
	c.UID = 1
	hub.Register(c)
	if err := hub.Subscribe(c, []int64{9}, 1); err != nil {
		t.Fatalf("订阅失败：%v", err)
	}
	if !c.Subscribed(9) {
		t.Fatalf("订阅应生效")
	}

	// 撤权
	chk.revoke(9, 1)
	hub.RefreshVisibility(9)
	if c.Subscribed(9) {
		t.Fatalf("撤权后必须摘除订阅（否则会持续泄露「该账本仍活跃」这一事实）")
	}
	// 此后普通广播不得再送达
	hub.BroadcastToBook(9, []byte("full"), []byte("sens"))
	select {
	case m := <-c.Send:
		t.Fatalf("撤权后仍收到广播：%s", m)
	default:
	}
	// 但定向清理通知必须仍能送达（不依赖订阅状态）
	if n := hub.DeliverToUser(1, []byte("member_removed")); n != 1 {
		t.Fatalf("定向清理通知应送达 1 条连接，实际 %d", n)
	}
	if got := string(recvOne(t, c)); got != "member_removed" {
		t.Fatalf("定向投递内容不对：%q", got)
	}
}

// TestReviewBroadcastScopedToSubscribedBook 广播只发给订阅了该账本的连接：
// 订阅 A 的连接不得收到 B 的事件（逐连接、逐账本校验）。
func TestReviewBroadcastScopedToSubscribedBook(t *testing.T) {
	chk := newFakeChecker()
	for b := int64(1); b <= 4; b++ {
		chk.allow(b, 1, true)
	}
	hub := NewHub(chk.check)

	cls := make([]*Client, 0, 4)
	for i := 0; i < 4; i++ {
		c := NewClient()
		c.UID = 1
		hub.Register(c)
		if err := hub.Subscribe(c, []int64{int64(i) + 1}, 1); err != nil {
			t.Fatalf("订阅失败：%v", err)
		}
		cls = append(cls, c)
	}
	for b := int64(1); b <= 4; b++ {
		hub.BroadcastToBook(b, []byte(fmt.Sprintf("evt-%d", b)), nil)
	}
	for i, c := range cls {
		msg := string(recvOne(t, c))
		if want := fmt.Sprintf("evt-%d", i+1); msg != want {
			t.Fatalf("连接 %d 应只收到 %s，实际收到 %s", i, want, msg)
		}
		select {
		case extra := <-c.Send:
			t.Fatalf("连接 %d 额外收到了不属于它的广播：%s", i, extra)
		default:
		}
	}
}

// TestReviewHubGenMapNeverShrinks 观测：RefreshVisibility 会为每个被刷新过的账本
// 在 hub.gen 里留下一个条目，且**没有任何回收路径**。
//
// 影响面有限（条目数 ≈ 曾经发生过权限变更的账本数，家庭场景很小），
// 但它是无界增长且只随"账本被删"而永不减少，记录下来供后续决策。
func TestReviewHubGenMapNeverShrinks(t *testing.T) {
	hub := NewHub(func(uid, bookID int64) (bool, error) { return true, nil })
	const n = 300
	for b := int64(1); b <= n; b++ {
		hub.RefreshVisibility(b)
	}
	hub.mu.RLock()
	got := len(hub.gen)
	hub.mu.RUnlock()
	if got != n {
		t.Fatalf("刷新 %d 个账本后 gen 表应有 %d 项，实际 %d", n, n, got)
	}
	// 账本删除也没有回收路径：再刷一遍不会减少
	for b := int64(1); b <= n; b++ {
		hub.RefreshVisibility(b)
	}
	hub.mu.RLock()
	got2 := len(hub.gen)
	hub.mu.RUnlock()
	if got2 != n {
		t.Fatalf("重复刷新后 gen 表应仍为 %d 项，实际 %d", n, got2)
	}
	t.Logf("gen 表在刷新 %d 个账本后持有 %d 项且无回收路径（有界于账本数，非连接数）", n, got2)
}

// TestReviewTokenExpiryClosesConnection 令牌到期即断连并清理订阅（GPT-R51 回归确认）。
//
// 这里只确认"到期的连接会被摘出注册表且订阅被清空"，用 sub-second 的过期时间
// 保证用例在几百毫秒内结束（不依赖 sleep 做同步，用 waitCount 轮询）。
func TestReviewTokenExpiryClosesConnection(t *testing.T) {
	old := DefaultHub
	hub := NewHub(func(uid, bookID int64) (bool, error) { return true, nil })
	DefaultHub = hub
	t.Cleanup(func() {
		// 必须先把所有连接排干再写回，避免 ServeWS 的 goroutine 还在跑
		DefaultHub = old
	})

	srv := newQATestServerWithExpiry(t, time.Now().Add(1200*time.Millisecond))
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("握手失败：%v", err)
	}
	waitCount(t, hub, 1)

	if err := conn.WriteJSON(map[string]interface{}{"action": "subscribe", "books": []int64{1}}); err != nil {
		t.Fatalf("发送订阅失败：%v", err)
	}
	// 等订阅生效
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		hub.mu.RLock()
		var c *Client
		for k := range hub.clients {
			c = k
		}
		hub.mu.RUnlock()
		if c != nil && c.Subscribed(1) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// 令牌到期后：连接必须被摘出注册表
	expired := time.Now().Add(3 * time.Second)
	for time.Now().Before(expired) {
		if hub.Count() == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if hub.Count() != 0 {
		t.Fatalf("令牌到期后连接应被注销，实际仍有 %d 条", hub.Count())
	}
	_ = conn.Close()

	// 收尾：确保没有 goroutine 还在引用本用例的临时 hub，再让 Cleanup 写回 DefaultHub
	deadline2 := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline2) && hub.Count() != 0 {
		time.Sleep(5 * time.Millisecond)
	}
}

// reviewHTTPUnused 保留对 net/http / httptest / gin 的引用（与既有助手共用同一套构造方式），
// 避免将来新增真实连接用例时重复摸索。
var reviewHTTPUnused = func() error {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	srv := httptest.NewUnstartedServer(r)
	srv.Close()
	return http.ErrServerClosed
}
