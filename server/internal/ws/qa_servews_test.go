package ws

// QA 验收（阶段 3.3 连接生命周期）：用真实 WebSocket 握手验证 ServeWS 的闭环。
//
// 不依赖数据库、不依赖 CGO：可见性校验由 setChecker 注入的假实现提供。
// 覆盖：升级成功 → 订阅生效 → 广播投递 → 客户端断开后连接被注销（在线数归 0）。

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// newQATestServer 起一个只挂 /ws 的测试服务（uid 由假中间件注入，绕开 JWT）
func newQATestServer(t *testing.T) *httptest.Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/ws", func(c *gin.Context) {
		c.Set("uid", int64(7))
		ServeWS(c)
	})
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

// waitCount 轮询等待在线连接数达到期望值（避免依赖固定 sleep）
func waitCount(t *testing.T, hub *Hub, want int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if hub.Count() == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("在线连接数未达到 %d，实际 %d", want, hub.Count())
}

// TestQAS3ServeWSSubscribeAndBroadcast 真实连接：握手 → 订阅 → 收广播 → 断开后注销
func TestQAS3ServeWSSubscribeAndBroadcast(t *testing.T) {
	chk := newQAChecker()
	chk.set(7, 1, false) // 账本 1：是成员但关闭互看
	old := DefaultHub
	DefaultHub = NewHub(chk.check)
	t.Cleanup(func() { DefaultHub = old })

	srv := newQATestServer(t)
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("WebSocket 握手失败：%v", err)
	}
	waitCount(t, DefaultHub, 1)

	// 订阅账本 1（非成员 2 应被静默忽略）
	if err := conn.WriteJSON(map[string]interface{}{"action": "subscribe", "books": []int64{1, 2}}); err != nil {
		t.Fatalf("发送订阅失败：%v", err)
	}
	// 等服务端读循环处理订阅（订阅写发生在 Hub 锁内，时间极短）
	time.Sleep(150 * time.Millisecond)

	// 只设一次读超时：gorilla 的客户端连接在任一次读失败后即不可再用，
	// 因此不能靠"循环 + 短超时"重试。
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	DefaultHub.BroadcastToBook(1, []byte("full-payload"), []byte("sensitive-payload"))
	_, got, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("订阅后未收到广播（订阅链路未生效）：%v", err)
	}
	// canView=false → 必须收到脱敏载荷
	if string(got) != "sensitive-payload" {
		t.Fatalf("关闭互看应收到脱敏载荷，实际 %s", got)
	}

	// 关闭连接 → 服务端读循环报错 → cancel → defer 注销
	_ = conn.Close()
	waitCount(t, DefaultHub, 0)
}

// TestQAS3ServeWSRejectsSubscribeWithoutChecker 未注入 checker 时订阅一律失败（fail-closed），
// 但连接本身仍可建立（鉴权已通过）。
func TestQAS3ServeWSRejectsSubscribeWithoutChecker(t *testing.T) {
	old := DefaultHub
	DefaultHub = NewHub(nil) // 无 checker
	t.Cleanup(func() { DefaultHub = old })

	srv := newQATestServer(t)
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("握手失败：%v", err)
	}
	defer func() { _ = conn.Close() }()
	waitCount(t, DefaultHub, 1)

	if err := conn.WriteJSON(map[string]interface{}{"action": "subscribe", "books": []int64{1}}); err != nil {
		t.Fatalf("发送订阅失败：%v", err)
	}
	time.Sleep(50 * time.Millisecond)
	DefaultHub.mu.RLock()
	defer DefaultHub.mu.RUnlock()
	for c := range DefaultHub.clients {
		if c.Subscribed(1) {
			t.Fatal("没有 checker 时不得订阅成功（fail-closed）")
		}
	}
}

// TestQAS3ServeWSRejectsUnauthenticated 上下文中没有 uid 时必须 401 且不升级
func TestQAS3ServeWSRejectsUnauthenticated(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/ws", ServeWS) // 没有注入 uid
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/ws")
	if err != nil {
		t.Fatalf("请求失败：%v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("未认证应 401，实际 %d", resp.StatusCode)
	}
}
