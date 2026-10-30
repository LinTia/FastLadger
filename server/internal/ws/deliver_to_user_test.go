package ws

// 定向投递（DeliverToUser）的纯逻辑测试。
//
// 背景（GPT-R24 / P1）：成员被移除时，其订阅会在 RefreshVisibility 中被摘除；
// 若清理事件仍走 BroadcastToBook，会被 `!sub` 过滤，目标永远收不到本该触发
// "清除该账本本地数据"的通知。因此需要一条**不依赖订阅状态**的投递路径。
//
// 本文件只验证 DeliversToUser 自身语义；"先摘订阅再定向投递、最后普通广播"的
// 端到端顺序由 internal/api 的 broadcast 测试覆盖（那里才能调 notifyMemberRemoved）。

import (
	"testing"
	"time"
)

func newTestHub() *Hub {
	return NewHub(func(uid, bookID int64) (bool, error) { return true, nil })
}

// 目标未订阅任何一个账本，也必须能收到定向消息（这正是清理通知的场景）。
func TestDeliverToUserIgnoresSubscription(t *testing.T) {
	h := newTestHub()
	c := NewClient()
	c.UID = 7
	h.Register(c)
	defer h.Unregister(c)

	if c.Subscribed(1) {
		t.Fatal("前置条件失败：该连接不应已订阅任何账本")
	}

	if n := h.DeliverToUser(7, []byte("cleanup")); n != 1 {
		t.Fatalf("入队连接数 = %d，期望 1", n)
	}
	select {
	case msg := <-c.Send:
		if string(msg) != "cleanup" {
			t.Fatalf("收到 %q，期望 cleanup", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("未订阅的连接也应收到定向消息")
	}
}

// 只投给目标 uid，不能误伤其他在线用户（否则清理通知会泄露给无关人）。
func TestDeliverToUserOnlyTargetUID(t *testing.T) {
	h := newTestHub()
	target := NewClient()
	target.UID = 7
	other := NewClient()
	other.UID = 8
	h.Register(target)
	h.Register(other)
	defer h.Unregister(target)
	defer h.Unregister(other)

	if n := h.DeliverToUser(7, []byte("for-7")); n != 1 {
		t.Fatalf("入队连接数 = %d，期望 1", n)
	}
	select {
	case msg := <-target.Send:
		if string(msg) != "for-7" {
			t.Fatalf("目标收到 %q", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("目标未收到消息")
	}
	select {
	case msg := <-other.Send:
		t.Fatalf("其他用户不应收到，却收到 %q", msg)
	case <-time.After(50 * time.Millisecond):
		// 期望：无消息
	}
}

// 同一 uid 的多个连接（多设备）都要收到清理通知。
func TestDeliverToUserAllConnectionsOfSameUser(t *testing.T) {
	h := newTestHub()
	a := NewClient()
	a.UID = 7
	b := NewClient()
	b.UID = 7
	h.Register(a)
	h.Register(b)
	defer h.Unregister(a)
	defer h.Unregister(b)

	if n := h.DeliverToUser(7, []byte("multi")); n != 2 {
		t.Fatalf("入队连接数 = %d，期望 2（同用户的两个连接）", n)
	}
	for i, c := range []*Client{a, b} {
		select {
		case <-c.Send:
		case <-time.After(time.Second):
			t.Fatalf("第 %d 个连接未收到消息", i+1)
		}
	}
}

// 目标离线 / uid 非法：返回 0，不 panic（调用方据此记日志）。
func TestDeliverToUserUnknownOrInvalid(t *testing.T) {
	h := newTestHub()
	c := NewClient()
	c.UID = 7
	h.Register(c)
	defer h.Unregister(c)

	if n := h.DeliverToUser(999, []byte("x")); n != 0 {
		t.Fatalf("离线目标入队数 = %d，期望 0", n)
	}
	if n := h.DeliverToUser(0, []byte("x")); n != 0 {
		t.Fatalf("uid=0 入队数 = %d，期望 0", n)
	}
	if n := h.DeliverToUser(7, nil); n != 0 {
		t.Fatalf("nil 消息入队数 = %d，期望 0", n)
	}
}

// Send 满时非阻塞丢弃并计数（与普通广播同策略），不能阻塞调用方。
func TestDeliverToUserDropsWhenBufferFull(t *testing.T) {
	h := newTestHub()
	c := NewClient()
	c.UID = 7
	h.Register(c)
	defer h.Unregister(c)

	cap1 := cap(c.Send)
	for i := 0; i < cap1; i++ {
		c.Send <- []byte("fill")
	}
	if n := h.DeliverToUser(7, []byte("overflow")); n != 0 {
		t.Fatalf("缓冲已满时入队数 = %d，期望 0", n)
	}
	if got := c.DroppedCount(); got != 1 {
		t.Fatalf("Dropped = %d，期望 1", got)
	}
}
