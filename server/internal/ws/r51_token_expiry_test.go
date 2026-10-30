package ws

// GPT-R51【P2】WebSocket 只在握手时校验 JWT，连接可在 token 过期后继续存活 —— 定向回归。
//
// 现状：wsAuth 在握手时校验一次 JWT，Upgrade 之后 ServeWS 不再持有 claims/token，
// 也不再检查 exp；连接生命周期完全由 socket/ping 决定 —— 令牌 24h 过期，
// 连接却可以挂上几天。
//
// 修复：
//  1. wsAuth 把 JWT 的 exp 写进上下文（键为 ws.TokenExpContextKey，契约常量由 ws 导出）；
//  2. ServeWS 以该 exp 作为**连接生命周期的硬上限**（context.WithDeadline），
//     到期即退出写循环 → 注销订阅 + 主动发关闭帧（4001）+ 关闭连接；
//  3. 上下文中没有 exp（或已过期）一律 fail-closed 拒绝升级：
//     宁可让客户端重新登录，也不放一条永生连接进来。
//
// 测试口径：**短 TTL 令牌（2 秒）的真实 WebSocket 连接**，
// 断言"到期后连接被关闭 + 订阅被清理"。用轮询等待而不是长 sleep。

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// 关于"为什么不在这里签发真实 JWT"的说明：
// ServeWS 消费的是 wsAuth 交给上下文的 exp（ws.TokenExpContextKey），
// 令牌 → exp 这一步由 api.wsAuth（`c.Set(ws.TokenExpContextKey, claims.Exp)`）完成，
// 并有独立的源码级契约断言（见 r51_wsauth_contract_test.go）。
// 本文件因此直接以"短 TTL 令牌的 exp"（now+2s）驱动真实 WebSocket 连接，
// 这样既保持 ws 包可独立编译运行（不依赖 service/数据库），
// 又完整验证了"到期即关闭连接并清理订阅"这条生命周期。

// r51WaitSubscribed 轮询等待订阅生效（避免依赖固定 sleep）
func r51WaitSubscribed(t *testing.T, c *Client, bookID int64) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if c.Subscribed(bookID) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("订阅账本 %d 未生效（超时）", bookID)
}

// r51OnlyClient 取出 Hub 中唯一的在线连接
func r51OnlyClient(t *testing.T, hub *Hub) *Client {
	t.Helper()
	hub.mu.RLock()
	defer hub.mu.RUnlock()
	if len(hub.clients) != 1 {
		t.Fatalf("期望只有 1 个在线连接，实际 %d", len(hub.clients))
	}
	for c := range hub.clients {
		return c
	}
	t.Fatal("Hub 中没有连接")
	return nil
}

// r51ShortTTL 短 TTL 令牌的到期时刻（2 秒后过期）
func r51ShortTTL() time.Time { return time.Now().Add(2 * time.Second) }

// TestR51ShortTTLTokenClosesConnection 短 TTL 令牌（2 秒）：到期后连接必须被服务端关闭。
//
// 到期时刻直接取自"令牌的 exp"，也就是 api.wsAuth 写入上下文的那个值；
// 这里用真实 WebSocket 连接 + 真实到期，而不是 mock 掉连接生命周期。
func TestR51ShortTTLTokenClosesConnection(t *testing.T) {
	chk := newQAChecker()
	chk.set(7, 1, true)
	old := DefaultHub
	DefaultHub = NewHub(chk.check)
	t.Cleanup(func() { DefaultHub = old })

	srv := newQATestServerWithExpiry(t, r51ShortTTL())
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("WebSocket 握手失败：%v", err)
	}
	defer func() { _ = conn.Close() }()
	waitCount(t, DefaultHub, 1)

	if err := conn.WriteJSON(map[string]interface{}{"action": "subscribe", "books": []int64{1}}); err != nil {
		t.Fatalf("发送订阅失败：%v", err)
	}
	client := r51OnlyClient(t, DefaultHub)
	r51WaitSubscribed(t, client, 1)

	// 令牌未到期前：连接必须还活着（证明不是"一握手就被关掉"）
	if DefaultHub.Count() != 1 {
		t.Fatalf("令牌有效期内连接应保持在线，实际在线数 %d", DefaultHub.Count())
	}

	// 到期后：服务端主动注销（waitCount 内部轮询，最长等 5s）
	waitCount(t, DefaultHub, 0)

	// 客户端侧应看到服务端发来的关闭帧（4001），而不是裸 RST
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatal("令牌到期后连接仍可读到数据：连接未被关闭")
	} else if !websocket.IsCloseError(err, closeTokenExpired) {
		t.Fatalf("应收到关闭码 %d（token expired），实际错误 %v", closeTokenExpired, err)
	}
}

// TestR51SubscriptionsClearedOnTokenExpiry 令牌到期后订阅必须被清理干净。
func TestR51SubscriptionsClearedOnTokenExpiry(t *testing.T) {
	chk := newQAChecker()
	chk.set(7, 1, true)
	chk.set(7, 2, true)
	old := DefaultHub
	DefaultHub = NewHub(chk.check)
	t.Cleanup(func() { DefaultHub = old })

	srv := newQATestServerWithExpiry(t, r51ShortTTL())
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("WebSocket 握手失败：%v", err)
	}
	defer func() { _ = conn.Close() }()
	waitCount(t, DefaultHub, 1)

	if err := conn.WriteJSON(map[string]interface{}{"action": "subscribe", "books": []int64{1, 2}}); err != nil {
		t.Fatalf("发送订阅失败：%v", err)
	}
	client := r51OnlyClient(t, DefaultHub)
	r51WaitSubscribed(t, client, 1)
	if !client.Subscribed(2) {
		t.Fatal("前置条件失败：账本 2 未订阅成功")
	}

	waitCount(t, DefaultHub, 0)

	if got := client.SubscriptionCount(); got != 0 {
		t.Fatalf("令牌到期后订阅必须清空，实际仍订阅 %d 个账本", got)
	}
	if client.Subscribed(1) || client.Subscribed(2) {
		t.Fatal("令牌到期后不得残留任何单个账本的订阅")
	}
	// 清理后不得再收到广播（订阅已随连接一起消失）
	DefaultHub.BroadcastToBook(1, []byte("full"), []byte("sensitive"))
	if n := len(client.Send); n != 0 {
		t.Fatalf("连接结束后不应再收到广播，实际缓冲 %d 条", n)
	}
}

// TestR51RejectsHandshakeWithoutTokenExpiry 上下文缺少 exp：fail-closed 拒绝升级。
//
// 若这里放行，连接就没有任何生命周期上限 —— 正是 R51 要消灭的情形。
func TestR51RejectsHandshakeWithoutTokenExpiry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/ws", func(c *gin.Context) {
		c.Set(uidContextKey, int64(7)) // 只有 uid，没有 exp（模拟漏传）
		ServeWS(c)
	})
	srv := httptest.NewServer(r)
	defer srv.Close()

	if code := r51Get(t, srv.URL+"/ws"); code != http.StatusUnauthorized {
		t.Fatalf("缺少 exp 时必须 401（否则连接永生），实际 %d", code)
	}
}

// TestR51RejectsHandshakeWithExpiredToken exp 已过：拒绝升级。
func TestR51RejectsHandshakeWithExpiredToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/ws", func(c *gin.Context) {
		c.Set(uidContextKey, int64(7))
		c.Set(TokenExpContextKey, time.Now().Add(-time.Minute).Unix())
		ServeWS(c)
	})
	srv := httptest.NewServer(r)
	defer srv.Close()

	if code := r51Get(t, srv.URL+"/ws"); code != http.StatusUnauthorized {
		t.Fatalf("令牌已过期时必须 401，实际 %d", code)
	}
}

// TestR51RejectsHandshakeWithMalformedTokenExpiry exp 类型非法（非 int64 / 非正数）：拒绝升级
func TestR51RejectsHandshakeWithMalformedTokenExpiry(t *testing.T) {
	for name, val := range map[string]interface{}{
		"字符串":     "not-a-number",
		"零值":      0,
		"负数":      -1,
		"float64": float64(time.Now().Add(time.Hour).Unix()),
	} {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		v := val
		r.GET("/ws", func(c *gin.Context) {
			c.Set(uidContextKey, int64(7))
			c.Set(TokenExpContextKey, v)
			ServeWS(c)
		})
		srv := httptest.NewServer(r)
		if code := r51Get(t, srv.URL+"/ws"); code != http.StatusUnauthorized {
			srv.Close()
			t.Fatalf("exp=%s（%v）时必须 401（fail-closed），实际 %d", name, v, code)
		}
		srv.Close()
	}
}

// TestR51LongLivedTokenKeepsConnectionAlive 对照组：长期有效的令牌，
// 连接在有效期内必须保持在线（证明 R51 的关闭确实由"到期"触发，而不是别的 bug）。
func TestR51LongLivedTokenKeepsConnectionAlive(t *testing.T) {
	chk := newQAChecker()
	chk.set(7, 1, true)
	old := DefaultHub
	DefaultHub = NewHub(chk.check)
	t.Cleanup(func() { DefaultHub = old })

	srv := newQATestServer(t) // 24h 有效期
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("WebSocket 握手失败：%v", err)
	}
	defer func() { _ = conn.Close() }()
	waitCount(t, DefaultHub, 1)

	if err := conn.WriteJSON(map[string]interface{}{"action": "subscribe", "books": []int64{1}}); err != nil {
		t.Fatalf("发送订阅失败：%v", err)
	}
	client := r51OnlyClient(t, DefaultHub)
	r51WaitSubscribed(t, client, 1)

	// 等一段明显超过"短 TTL"的时间（仍远小于 24h）：连接必须还在
	time.Sleep(300 * time.Millisecond)
	if DefaultHub.Count() != 1 {
		t.Fatalf("有效令牌的连接不应被关闭，实际在线数 %d", DefaultHub.Count())
	}
	if !client.Subscribed(1) {
		t.Fatal("有效令牌的连接订阅不应被清理")
	}

	// 主动断开 → 正常注销（与原有生命周期测试一致）
	_ = conn.Close()
	waitCount(t, DefaultHub, 0)
}

// r51Get 发一个普通 GET（非 WS 握手），返回状态码
func r51Get(t *testing.T, url string) int {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("请求失败：%v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode
}
