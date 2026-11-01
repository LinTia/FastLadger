package ws

// R52 多端会话：KickJti / Client.Kick 的单元测试（纯内存，无 DB）。
//
// 端到端行为（撤销 → 4003 关闭帧）由 internal/api/session_api_test.go 覆盖；
// 这里钉死 Hub 侧的定位与幂等语义：
//   - KickJti 只命中绑定该 jti 的连接，别的连接不受影响；
//   - 返回值 = 真正触发断开的连接数；重复踢同一会话返回 0；
//   - 空 jti 是 no-op（缺 jti 的连接不可被定位，也绝不能被"空串批量误伤"）。

import (
	"testing"
	"time"
)

func registerClient(t *testing.T, h *Hub, jti string) *Client {
	t.Helper()
	c := NewClient()
	c.SetJti(jti)
	if err := h.Register(c); err != nil {
		t.Fatalf("注册连接失败：%v", err)
	}
	return c
}

func kickedWithin(c *Client, d time.Duration) bool {
	select {
	case <-c.Kicked():
		return true
	case <-time.After(d):
		return false
	}
}

// TestKickJtiOnlyTargetsMatchingSession 只断开目标会话的连接
func TestKickJtiOnlyTargetsMatchingSession(t *testing.T) {
	h := NewHub(nil)
	a1 := registerClient(t, h, "jti-device-1")
	a2 := registerClient(t, h, "jti-device-1") // 同一会话的两条连接（如同浏览器双标签页）
	b := registerClient(t, h, "jti-device-2")

	if n := h.KickJti("jti-device-1"); n != 2 {
		t.Fatalf("应断开 2 条连接，实际 %d", n)
	}
	if !kickedWithin(a1, time.Second) || !kickedWithin(a2, time.Second) {
		t.Fatal("目标会话的连接应收到踢出信号")
	}
	if kickedWithin(b, 100*time.Millisecond) {
		t.Fatal("其它会话的连接不得被误伤")
	}
	if n := h.KickJti("jti-device-1"); n != 0 {
		t.Fatalf("重复踢同一会话应返回 0（幂等），实际 %d", n)
	}
	if n := h.KickJti("jti-device-2"); n != 1 {
		t.Fatalf("其它会话仍可正常踢出，实际 %d", n)
	}
}

// TestKickJtiUnknownAndEmpty 未知 jti / 空 jti 都是 no-op
func TestKickJtiUnknownAndEmpty(t *testing.T) {
	h := NewHub(nil)
	c := registerClient(t, h, "jti-known")
	if n := h.KickJti("jti-unknown"); n != 0 {
		t.Fatalf("未知 jti 应返回 0，实际 %d", n)
	}
	if n := h.KickJti(""); n != 0 {
		t.Fatalf("空 jti 应为 no-op，实际 %d", n)
	}
	if kickedWithin(c, 100*time.Millisecond) {
		t.Fatal("空/未知 jti 不得触发任何断开")
	}
}

// TestClientKickIdempotent Kick 幂等：只第一次返回 true，信号通道只关一次
func TestClientKickIdempotent(t *testing.T) {
	h := NewHub(nil)
	c := registerClient(t, h, "jti-x")
	if !c.Kick() {
		t.Fatal("首次 Kick 应返回 true")
	}
	if c.Kick() {
		t.Fatal("重复 Kick 应返回 false（幂等）")
	}
	if !kickedWithin(c, time.Second) {
		t.Fatal("信号通道应已被关闭")
	}
}

// TestKickJtiAfterUnregister 注销后的连接不参与定位（不得复活指针）
func TestKickJtiAfterUnregister(t *testing.T) {
	h := NewHub(nil)
	c := registerClient(t, h, "jti-gone")
	h.Unregister(c)
	if n := h.KickJti("jti-gone"); n != 0 {
		t.Fatalf("已注销连接不应被命中，实际 %d", n)
	}
}
