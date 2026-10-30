package ws

// WS Hub 的纯逻辑测试（不依赖数据库 / 不建真实连接）。
//
// 用假的 VisibilityChecker 注入，验证阶段 3.3/3.4 的四条硬约束：
//  1. Subscribe 只接受**服务端校验通过**的账本（非成员一律拒绝）；
//  2. 广播按接收者可见性选择"完整载荷 / 脱敏载荷"，且读的是缓存而非查库；
//  3. Send 满时非阻塞丢弃并累加 Dropped（不阻塞广播、不 close channel）；
//  4. RefreshVisibility 能在可见性变化后重算缓存（幂等、可反复调用）。
//
// 并发安全（锁顺序 Hub.mu → Client.mu）由 `go test -race` 下的
// TestHubConcurrentSubscribeBroadcastRefresh 覆盖。
import (
	"errors"
	"sync"
	"testing"
	"time"
)

// fakeChecker 可编程的可见性校验器；bookID→uid→canView
type fakeChecker struct {
	mu      sync.Mutex
	allowed map[int64]map[int64]bool
	calls   int
}

func newFakeChecker() *fakeChecker {
	return &fakeChecker{allowed: make(map[int64]map[int64]bool)}
}

// allow 设定某 uid 在某账本上的可见性（不设定 = 非成员）
func (f *fakeChecker) allow(bookID, uid int64, canView bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.allowed[bookID] == nil {
		f.allowed[bookID] = make(map[int64]bool)
	}
	f.allowed[bookID][uid] = canView
}

// revoke 让某 uid 不再是该账本成员
func (f *fakeChecker) revoke(bookID, uid int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.allowed[bookID] != nil {
		delete(f.allowed[bookID], uid)
	}
}

func (f *fakeChecker) check(uid, bookID int64) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	books, ok := f.allowed[bookID]
	if !ok {
		return false, errors.New("not a member")
	}
	canView, ok := books[uid]
	if !ok {
		return false, errors.New("not a member")
	}
	return canView, nil
}

func (f *fakeChecker) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// recvOne 读取一条投递（带超时，避免测试永久挂住）
func recvOne(t *testing.T, c *Client) []byte {
	t.Helper()
	select {
	case m := <-c.Send:
		return m
	case <-time.After(time.Second):
		t.Fatal("等待投递超时")
		return nil
	}
}

// TestSubscribeRejectsNonMember 非成员一律不能订阅（不泄露账本是否存在）
func TestSubscribeRejectsNonMember(t *testing.T) {
	checker := newFakeChecker()
	checker.allow(1, 100, true) // uid=100 是账本 1 的成员且可看全部
	hub := NewHub(checker.check)

	c := NewClient()
	hub.Register(c)
	hub.Subscribe(c, []int64{1, 2, 0, -3}, 100)

	if !c.Subscribed(1) {
		t.Fatal("应订阅成功账本 1")
	}
	if !c.CanView(1) {
		t.Fatal("账本 1 的 canView 缓存应为 true")
	}
	for _, bad := range []int64{2, 0, -3} {
		if c.Subscribed(bad) {
			t.Fatalf("非法/非成员账本 %d 不应被订阅", bad)
		}
		if c.CanView(bad) {
			t.Fatalf("非法/非成员账本 %d 不应有可见性缓存", bad)
		}
	}
	if c.UID != 100 {
		t.Fatalf("订阅后应绑定 uid，实际 %d", c.UID)
	}
}

// TestSubscribeFailClosedWithoutChecker 未注入校验器时 fail-closed
func TestSubscribeFailClosedWithoutChecker(t *testing.T) {
	hub := NewHub(nil)
	c := NewClient()
	hub.Register(c)
	hub.Subscribe(c, []int64{1}, 100)
	if c.Subscribed(1) {
		t.Fatal("没有校验器时必须拒绝订阅（fail-closed）")
	}
}

// TestUnregisteredClientCannotSubscribe 已注销的连接不再接收订阅写入
func TestUnregisteredClientCannotSubscribe(t *testing.T) {
	checker := newFakeChecker()
	checker.allow(1, 100, true)
	hub := NewHub(checker.check)

	c := NewClient()
	hub.Register(c)
	hub.Unregister(c)
	hub.Subscribe(c, []int64{1}, 100)
	if c.Subscribed(1) {
		t.Fatal("已注销的连接不应被写入订阅")
	}
	if hub.Count() != 0 {
		t.Fatalf("注销后在线数应为 0，实际 %d", hub.Count())
	}
}

// TestBroadcastRoutesByCanView 按可见性选择完整/脱敏载荷
func TestBroadcastRoutesByCanView(t *testing.T) {
	checker := newFakeChecker()
	checker.allow(1, 100, true)  // 可看他人明细
	checker.allow(1, 200, false) // 关闭互看下的 member
	hub := NewHub(checker.check)

	open := NewClient()
	restricted := NewClient()
	outsider := NewClient() // 未订阅
	hub.Register(open)
	hub.Register(restricted)
	hub.Register(outsider)
	hub.Subscribe(open, []int64{1}, 100)
	hub.Subscribe(restricted, []int64{1}, 200)

	full := []byte(`{"event":"visibility_changed","detail":true}`)
	sensitive := []byte(`{"event":"visibility_changed"}`)
	hub.BroadcastToBook(1, full, sensitive)

	if got := string(recvOne(t, open)); got != string(full) {
		t.Fatalf("可看全部的成员应收到完整载荷，实际 %s", got)
	}
	if got := string(recvOne(t, restricted)); got != string(sensitive) {
		t.Fatalf("关闭互看的成员应收到脱敏载荷，实际 %s", got)
	}
	select {
	case m := <-outsider.Send:
		t.Fatalf("未订阅的连接不应收到消息：%s", m)
	case <-time.After(50 * time.Millisecond):
	}

	// 广播不得在 Hub 锁内查库：一次广播对 checker 的调用数必须还是订阅时的次数（2 次）
	if checker.callCount() != 2 {
		t.Fatalf("广播不应查库，checker 调用次数应为 2，实际 %d", checker.callCount())
	}
}

// TestBroadcastNonBlockingDrop Send 满时非阻塞丢弃 + Dropped 累加（且不 panic）
func TestBroadcastNonBlockingDrop(t *testing.T) {
	checker := newFakeChecker()
	checker.allow(1, 100, true)
	hub := NewHub(checker.check)

	c := NewClient()
	hub.Register(c)
	hub.Subscribe(c, []int64{1}, 100)

	// 灌满发送缓冲
	for i := 0; i < sendBufferSize; i++ {
		c.Send <- []byte("x")
	}
	hub.BroadcastToBook(1, []byte("full"), []byte("sensitive"))

	if got := c.DroppedCount(); got != 1 {
		t.Fatalf("缓冲满时应累加 Dropped（期望 1，实际 %d）", got)
	}
	// Send 永不 close：仍能继续投递（只是这次被丢弃）
	if got := len(c.Send); got != sendBufferSize {
		t.Fatalf("缓冲长度不应变化，实际 %d", got)
	}
}

// TestRefreshVisibility 可见性变化后重算缓存，后续广播立即改用脱敏载荷
func TestRefreshVisibility(t *testing.T) {
	checker := newFakeChecker()
	checker.allow(1, 100, true)
	hub := NewHub(checker.check)

	c := NewClient()
	hub.Register(c)
	hub.Subscribe(c, []int64{1}, 100)
	if !c.CanView(1) {
		t.Fatal("订阅后可看全部")
	}

	// 关闭互看 → 重算
	checker.allow(1, 100, false)
	hub.RefreshVisibility(1)
	if c.CanView(1) {
		t.Fatal("RefreshVisibility 后应变为不可看他人明细")
	}
	full, sensitive := []byte("full"), []byte("sensitive")
	hub.BroadcastToBook(1, full, sensitive)
	if got := string(recvOne(t, c)); got != string(sensitive) {
		t.Fatalf("重算后应收到脱敏载荷，实际 %s", got)
	}

	// 再打开互看 → 恢复完整载荷
	checker.allow(1, 100, true)
	hub.RefreshVisibility(1)
	hub.BroadcastToBook(1, full, sensitive)
	if got := string(recvOne(t, c)); got != string(full) {
		t.Fatalf("重新打开互看后应收到完整载荷，实际 %s", got)
	}

	// 被移除成员 → 降级为脱敏（fail-closed）
	checker.revoke(1, 100)
	hub.RefreshVisibility(1)
	if c.CanView(1) {
		t.Fatal("被移除成员的可见性必须降级为 false")
	}

	// 无订阅者的账本调用不得 panic
	hub.RefreshVisibility(99)
}

// TestBroadcastWithNilPayload 两个载荷都为空时静默返回（不得把 nil 投出去）
func TestBroadcastWithNilPayload(t *testing.T) {
	hub := NewHub(nil)
	c := NewClient()
	hub.Register(c)
	c.BookIds[1] = true
	hub.BroadcastToBook(1, nil, nil)
	if len(c.Send) != 0 {
		t.Fatalf("空载荷不应投递，实际缓冲 %d", len(c.Send))
	}
}

// TestHubConcurrentSubscribeBroadcastRefresh 并发压测（配合 -race 验证锁顺序不反转）
func TestHubConcurrentSubscribeBroadcastRefresh(t *testing.T) {
	checker := newFakeChecker()
	for book := int64(1); book <= 4; book++ {
		for uid := int64(1); uid <= 4; uid++ {
			checker.allow(book, uid, uid%2 == 0)
		}
	}
	hub := NewHub(checker.check)

	clients := make([]*Client, 0, 8)
	for uid := int64(1); uid <= 8; uid++ {
		c := NewClient()
		c.UID = uid
		hub.Register(c)
		clients = append(clients, c)
	}

	var wg sync.WaitGroup
	for _, c := range clients {
		wg.Add(1)
		go func(c *Client) {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				hub.Subscribe(c, []int64{1, 2, 3, 4}, c.UID)
			}
		}(c)
	}
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				hub.BroadcastToBook(int64(i%4+1), []byte("full"), []byte("sensitive"))
			}
		}(i)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			hub.RefreshVisibility(int64(i%4 + 1))
		}
	}()
	wg.Wait()

	// 排空缓冲，确认没有 panic 与死锁
	for _, c := range clients {
		for {
			select {
			case <-c.Send:
				continue
			default:
			}
			break
		}
	}
	for _, c := range clients {
		hub.Unregister(c)
	}
	if hub.Count() != 0 {
		t.Fatalf("全部注销后在线数应为 0，实际 %d", hub.Count())
	}
}

// TestLiteralClientDoesNotPanic 字面量构造的连接（BookIds/canView/visSeq 全为 nil）不得 panic。
//
// P3-3：Subscribe / BroadcastToBook / RefreshVisibility 访问这三个 map 前都走 ensureLocked()。
// 若漏了任何一处，`&Client{}`（测试或未来其他入口）会因"assignment to entry in nil map"崩溃。
func TestLiteralClientDoesNotPanic(t *testing.T) {
	checker := newFakeChecker()
	checker.allow(1, 100, true)
	hub := NewHub(checker.check)

	bare := &Client{UID: 100} // Send 也是 nil：广播必须跳过它而不是永久阻塞
	hub.Register(bare)

	hub.Subscribe(bare, []int64{1}, 100)
	if !bare.Subscribed(1) || !bare.CanView(1) {
		t.Fatal("字面量构造的连接也必须能正常订阅（ensureLocked 补齐 map）")
	}

	// 没有 Send 时广播不得 panic / 不得阻塞
	done := make(chan struct{})
	go func() {
		hub.BroadcastToBook(1, []byte("full"), []byte("sensitive"))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("广播被 nil Send 阻塞")
	}

	// RefreshVisibility 写缓存同样要补 map
	checker.allow(1, 100, false)
	hub.RefreshVisibility(1)
	if bare.CanView(1) {
		t.Fatal("RefreshVisibility 应把可见性更新为 false")
	}
}

// TestRefreshVisibilityDoesNotOverwriteNewerSubscribe P3-1：序号保护（单线程可复现版）。
//
// 手工模拟"快照之后、回写之前"发生了更新的 Subscribe：直接调用 Subscribe 后再
// 让 Refresh 的回写阶段执行，最终值必须是最新一次 Subscribe 的结果。
func TestRefreshVisibilityDoesNotOverwriteNewerSubscribe(t *testing.T) {
	checker := newFakeChecker()
	checker.allow(1, 100, true)
	hub := NewHub(checker.check)

	c := NewClient()
	hub.Register(c)
	hub.Subscribe(c, []int64{1}, 100)
	if !c.CanView(1) {
		t.Fatal("前置条件：订阅后可看全部")
	}

	// 关闭互看并让连接重新订阅（visSeq[1] 自增）
	checker.allow(1, 100, false)
	hub.Subscribe(c, []int64{1}, 100)
	if c.CanView(1) {
		t.Fatal("重新订阅后应为 false")
	}

	// 此后再刷新一次：结果同样是 false，且 visSeq 继续自增（不变量：写一次 +1）
	hub.RefreshVisibility(1)
	if c.CanView(1) {
		t.Fatal("刷新后应保持 false")
	}

	// 反向：重新打开互看 → Subscribe 写入 true，Refresh 不得把它改回 false
	checker.allow(1, 100, true)
	hub.Subscribe(c, []int64{1}, 100)
	if !c.CanView(1) {
		t.Fatal("重新订阅后应为 true")
	}
	hub.RefreshVisibility(1)
	if !c.CanView(1) {
		t.Fatal("RefreshVisibility 不得覆盖更新的 Subscribe 结果")
	}
}

// TestRefreshVisibilityUnsubscribesRemovedMember N2：被移除成员必须被摘出订阅表。
//
// 只把 canView 降为 false 不够：该连接仍会收到 data_changed，
// 等于持续泄露"这个账本还在被使用"（PRD §12.1 决策 5：被移除成员应停止订阅）。
func TestRefreshVisibilityUnsubscribesRemovedMember(t *testing.T) {
	checker := newFakeChecker()
	checker.allow(1, 100, true)
	checker.allow(1, 200, true)
	hub := NewHub(checker.check)

	kept := NewClient()
	removed := NewClient()
	hub.Register(kept)
	hub.Register(removed)
	hub.Subscribe(kept, []int64{1}, 100)
	hub.Subscribe(removed, []int64{1}, 200)

	// uid=200 被移出账本
	checker.revoke(1, 200)
	hub.RefreshVisibility(1)

	if removed.Subscribed(1) {
		t.Fatal("被移除成员仍在订阅表中：会继续收到该账本的 data_changed")
	}
	if removed.CanView(1) {
		t.Fatal("被移除成员的可见性缓存必须清除")
	}
	if !kept.Subscribed(1) {
		t.Fatal("其他成员不应被误删订阅")
	}

	// 广播只送达仍在订阅的成员
	hub.BroadcastToBook(1, []byte("full"), []byte("sensitive"))
	select {
	case m := <-removed.Send:
		t.Fatalf("被移除成员不得再收到广播：%s", m)
	case <-time.After(50 * time.Millisecond):
	}
	if got := string(recvOne(t, kept)); got != "full" {
		t.Fatalf("仍在订阅的成员应收到载荷，实际 %s", got)
	}

	// 重新订阅（重新加入）后必须恢复正常
	checker.allow(1, 200, false)
	hub.Subscribe(removed, []int64{1}, 200)
	if !removed.Subscribed(1) {
		t.Fatal("重新加入后应能重新订阅")
	}
	if removed.CanView(1) {
		t.Fatal("重新加入且关闭互看时 canView 应为 false")
	}
}
