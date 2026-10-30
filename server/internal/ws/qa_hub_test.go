package ws

// QA 独立验收（阶段 3.3：WebSocket Hub）—— 只补工程师用例没覆盖到的断言。
//
// 重点：
//  1. **广播路径对 checker 的调用次数为 0**（不查库的硬约束，用计数断言而不是"看起来没查"）；
//  2. 非成员订阅被拒且不产生任何缓存（订阅失败不得留下 canView=true）；
//  3. RefreshVisibility 的作用域正确（只动目标账本）、且能被后续 Subscribe 覆盖；
//  4. Unregister 不 close Send（否则并发广播会 panic）；
//  5. 并发 Subscribe / Broadcast / Refresh / Unregister 不 panic、不死锁；
//  6. **原已知竞态窗口已修复**：RefreshVisibility 的"快照-查库-回写"三段式曾会覆盖期间
//     更新的 Subscribe 结果，现由 Client.visSeq 序号保护（用例改为断言"不再覆盖"）。

import (
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// qaCountingChecker 计数型假校验器：值可从外部改，调用次数原子累加
type qaCountingChecker struct {
	calls  int64
	mu     sync.Mutex
	values map[string]bool // key: "uid:book" → canView
	errs   map[string]bool // key: "uid:book" → 返回错误（非成员）
}

func newQAChecker() *qaCountingChecker {
	return &qaCountingChecker{values: map[string]bool{}, errs: map[string]bool{}}
}

func qaKey(uid, book int64) string {
	return strconv.FormatInt(uid, 10) + ":" + strconv.FormatInt(book, 10)
}

func (c *qaCountingChecker) set(uid, book int64, canView bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values[qaKey(uid, book)] = canView
	delete(c.errs, qaKey(uid, book))
}

func (c *qaCountingChecker) notMember(uid, book int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.values, qaKey(uid, book))
	c.errs[qaKey(uid, book)] = true
}

func (c *qaCountingChecker) check(uid, bookID int64) (bool, error) {
	atomic.AddInt64(&c.calls, 1)
	c.mu.Lock()
	defer c.mu.Unlock()
	k := qaKey(uid, bookID)
	if c.errs[k] {
		return false, errors.New("not a member")
	}
	v, ok := c.values[k]
	if !ok {
		return false, errors.New("not a member")
	}
	return v, nil
}

func (c *qaCountingChecker) callCount() int64 { return atomic.LoadInt64(&c.calls) }

// qaDrain 排空 Send（测试结束前避免缓冲残留影响后续断言）
func qaDrain(c *Client) {
	for {
		select {
		case <-c.Send:
			continue
		default:
		}
		return
	}
}

// TestQAS3BroadcastNeverCallsChecker 广播路径对 checker 的调用次数必须是 0。
//
// 硬约束来源：家庭 NAS 上 SQLite 只有 1 个连接，Hub 锁内查库会把一次慢查询
// 放大成"所有人收不到通知"。因此广播只能读 Client.canView 缓存。
func TestQAS3BroadcastNeverCallsChecker(t *testing.T) {
	chk := newQAChecker()
	chk.set(100, 1, true)
	chk.set(200, 1, false)
	hub := NewHub(chk.check)

	a, b := NewClient(), NewClient()
	hub.Register(a)
	hub.Register(b)
	hub.Subscribe(a, []int64{1}, 100)
	hub.Subscribe(b, []int64{1}, 200)

	// 订阅阶段各查一次库（2 次），之后广播不得再增加
	afterSubscribe := chk.callCount()
	if afterSubscribe != 2 {
		t.Fatalf("订阅两次应查库 2 次，实际 %d", afterSubscribe)
	}

	for i := 0; i < 100; i++ {
		hub.BroadcastToBook(1, []byte("full"), []byte("sensitive"))
	}
	if got := chk.callCount(); got != afterSubscribe {
		t.Fatalf("广播不得查库：订阅后 %d 次，100 次广播后 %d 次", afterSubscribe, got)
	}

	qaDrain(a)
	qaDrain(b)
}

// TestQAS3SubscribeRejectsNonMemberWithoutCache 非成员订阅必须被拒，且不得留下任何可见性缓存。
//
// 若订阅失败却留下 canView=true（或 BookIds=true），该连接会收到别人的账本事件。
func TestQAS3SubscribeRejectsNonMemberWithoutCache(t *testing.T) {
	chk := newQAChecker()
	chk.set(100, 1, true)
	chk.notMember(777, 1)
	hub := NewHub(chk.check)

	c := NewClient()
	hub.Register(c)
	hub.Subscribe(c, []int64{1}, 777)

	if c.Subscribed(1) {
		t.Fatal("非成员不得订阅成功")
	}
	if c.CanView(1) {
		t.Fatal("非成员不得留下 canView=true 缓存（会收到别人的明细）")
	}
	// 广播不得投递给非成员
	hub.BroadcastToBook(1, []byte("full"), []byte("sensitive"))
	if n := len(c.Send); n != 0 {
		t.Fatalf("非成员不得收到该账本的消息，实际缓冲 %d 条", n)
	}
}

// TestQAS3SubscribeWithNoErrorButDeniedCanView checker 返回 (false,nil) 表示"是成员但看不到他人明细"：
// 订阅应成功、但缓存为 false（fail-closed，只发脱敏）。
func TestQAS3SubscribeWithNoErrorButDeniedCanView(t *testing.T) {
	chk := newQAChecker()
	chk.set(100, 1, false)
	hub := NewHub(chk.check)

	c := NewClient()
	hub.Register(c)
	hub.Subscribe(c, []int64{1}, 100)

	if !c.Subscribed(1) {
		t.Fatal("成员应能订阅")
	}
	if c.CanView(1) {
		t.Fatal("关闭互看时 canView 必须为 false")
	}
	hub.BroadcastToBook(1, []byte("full"), []byte("sensitive"))
	select {
	case m := <-c.Send:
		if string(m) != "sensitive" {
			t.Fatalf("关闭互看应收到脱敏载荷，实际 %s", m)
		}
	case <-time.After(time.Second):
		t.Fatal("等待投递超时")
	}
}

// TestQAS3RefreshVisibilityScope RefreshVisibility 只改目标账本，其它账本的缓存不动。
func TestQAS3RefreshVisibilityScope(t *testing.T) {
	chk := newQAChecker()
	chk.set(100, 1, true)
	chk.set(100, 2, true)
	hub := NewHub(chk.check)

	c := NewClient()
	hub.Register(c)
	hub.Subscribe(c, []int64{1, 2}, 100)

	chk.set(100, 1, false) // 只关账本 1 的互看
	hub.RefreshVisibility(1)

	if c.CanView(1) {
		t.Fatal("账本 1 应变为不可互看")
	}
	if !c.CanView(2) {
		t.Fatal("账本 2 的缓存不应被账本 1 的刷新影响")
	}

	// 无效 bookID 不得 panic / 不得改任何缓存
	hub.RefreshVisibility(0)
	hub.RefreshVisibility(-1)
	if !c.CanView(2) {
		t.Fatal("非法 bookID 不应改动缓存")
	}
}

// TestQAS3UnregisterKeepsSendOpen Unregister 只做 delete，绝不 close(Send)。
//
// 若 close 了：并发广播的 `c.Send <- payload` 会 panic（send on closed channel）。
func TestQAS3UnregisterKeepsSendOpen(t *testing.T) {
	chk := newQAChecker()
	chk.set(100, 1, true)
	hub := NewHub(chk.check)

	c := NewClient()
	hub.Register(c)
	hub.Subscribe(c, []int64{1}, 100)
	hub.Unregister(c)

	if hub.Count() != 0 {
		t.Fatalf("注销后在线数应为 0，实际 %d", hub.Count())
	}
	// 注销后广播：不 panic、不投递
	hub.BroadcastToBook(1, []byte("full"), []byte("sensitive"))
	if n := len(c.Send); n != 0 {
		t.Fatalf("已注销连接不应再收到消息，实际 %d 条", n)
	}
	// 通道仍然可写（未 close 的证据）：能塞进去说明没关闭
	select {
	case c.Send <- []byte("probe"):
	default:
		t.Fatal("Send 缓冲意外已满")
	}
	select {
	case m := <-c.Send:
		if string(m) != "probe" {
			t.Fatalf("probe 内容异常：%s", m)
		}
	default:
		t.Fatal("Send 已被 close（读不到自己的 probe）")
	}
}

// TestQAS3ConcurrentFullLifecycle 并发 Subscribe / Broadcast / Refresh / Unregister：
// 不 panic、不死锁、能正常收尾（配合 -race 才能验证数据竞争，本机无 CGO 无法开 -race）。
func TestQAS3ConcurrentFullLifecycle(t *testing.T) {
	chk := newQAChecker()
	for uid := int64(1); uid <= 8; uid++ {
		for book := int64(1); book <= 3; book++ {
			chk.set(uid, book, uid%2 == 0)
		}
	}
	hub := NewHub(chk.check)

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// 持续广播
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					hub.BroadcastToBook(int64(i%3+1), []byte("full"), []byte("sensitive"))
				}
			}
		}()
	}
	// 持续刷新可见性
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				hub.RefreshVisibility(int64(1))
			}
		}
	}()

	// 连接反复建立 / 订阅 / 注销
	for uid := int64(1); uid <= 8; uid++ {
		wg.Add(1)
		go func(uid int64) {
			defer wg.Done()
			for i := 0; i < 30; i++ {
				c := NewClient()
				hub.Register(c)
				hub.Subscribe(c, []int64{1, 2, 3}, uid)
				qaDrain(c)
				hub.Unregister(c)
			}
		}(uid)
	}

	// 给广播/刷新一点时间运行，再收尾
	time.Sleep(30 * time.Millisecond)
	close(stop)
	wg.Wait()

	if hub.Count() != 0 {
		t.Fatalf("全部注销后在线数应为 0，实际 %d", hub.Count())
	}
}

// TestQAS3KnownRaceRefreshOverwritesNewerSubscribe 【原已知竞态窗口 → 已修复的回归用例】
//
// 历史问题：RefreshVisibility 的三段式（持读锁快照 → 释放 → 锁外查库 → 逐个写缓存）
// 之间存在窗口，若在"快照之后、写缓存之前"有 Subscribe 写入了**更新**的可见性结果，
// 旧的 Refresh 结果会把新值覆盖回去（last-write-wins，无序号保护），
// 该连接会按**过期**的 canView 收发明细，直到下一次 RefreshVisibility 或重新订阅。
//
// 修复（P3-1）：Client 增加 visSeq（每账本的可见性写入序号）。
//   - Subscribe 每次写 canView 时 visSeq[book]++；
//   - RefreshVisibility 在查库**之前**快照各 client 的 seq，回写前比对，
//     不一致即放弃本次写入（让更新的 Subscribe 结果生效）。
//
// 本用例现在断言"不再覆盖"：给 checker 加一个可卡住的闸门，精确构造
// "Refresh 已快照、正在查库 → Subscribe 写入新值 → Refresh 才回写"的时序。
func TestQAS3KnownRaceRefreshOverwritesNewerSubscribe(t *testing.T) {
	gate := make(chan struct{})
	var seq int64

	chk := newQAChecker()
	chk.set(100, 1, true)

	// 第 2 次调用 = RefreshVisibility 的锁外查库：先读到当时的旧值，再卡住
	hub := NewHub(func(uid, bookID int64) (bool, error) {
		n := atomic.AddInt64(&seq, 1)
		if n != 2 {
			return chk.check(uid, bookID)
		}
		v, err := chk.check(uid, bookID)
		<-gate
		return v, err // 返回"卡住之前"读到的旧值
	})

	c := NewClient()
	hub.Register(c)
	hub.Subscribe(c, []int64{1}, 100) // 第 1 次调用
	if !c.CanView(1) {
		t.Fatal("前置条件：订阅后应为 true")
	}

	refreshed := make(chan struct{})
	go func() {
		hub.RefreshVisibility(1) // 第 2 次调用 → 卡在锁外查库
		close(refreshed)
	}()

	// RefreshVisibility 在查库前已经释放 Hub 读锁，因此这里的 Subscribe 不会被它阻塞。
	// 等一小会儿确保它已进入"已快照、正在查库"的状态。
	time.Sleep(50 * time.Millisecond)

	// 期间可见性被关闭，且本连接重新订阅 → 读到**新**值 false 并写入缓存
	chk.set(100, 1, false)
	hub.Subscribe(c, []int64{1}, 100) // 第 3 次调用
	if c.CanView(1) {
		t.Fatal("重新订阅后应读到最新值 false")
	}

	// 放行旧的 Refresh 结果 → 它必须发现自己已过期（visSeq 变了）并放弃写入
	close(gate)
	<-refreshed

	if c.CanView(1) {
		t.Fatal("旧 Refresh 结果覆盖了更新的 Subscribe 结果：该连接会按过期权限收发明细（P3-1 回归）")
	}
}
