package ws

// WS handler：接收连接 + 读写循环（照抄《新手开发完整指南》§14.3）。
//
// 生命周期与清理约定（R18，务必逐条对齐）：
//  1. 读/写循环共用一个取消上下文，主 defer 中 cancel() + Unregister() + conn.Close() 各一次；
//     读循环 goroutine 只 cancel()，**不重复** Unregister；
//  2. Send 永不 close，写循环靠 ctx/写错误退出；
//  3. 半开连接防护（D-R03）：SetReadLimit + 初始 SetReadDeadline + SetPongHandler 续期，
//     否则对端静默丢弃会让连接与 goroutine 长期挂住；
//  4. 只有写循环这一个 goroutine 写 conn（gorilla 不允许并发写）。
import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	// readLimit 单条消息上限 64KB（订阅消息只含几个账本 ID，远小于此）
	readLimit = 64 * 1024
	// pongWait 读截止：超过该时长没有 pong 即认为连接已死
	pongWait = 60 * time.Second
	// pingPeriod 服务端主动 ping 间隔（必须小于 pongWait，否则自己先把连接判死）
	pingPeriod = 30 * time.Second
	// writeWait 单次写超时
	writeWait = 10 * time.Second
	// uidContextKey 认证中间件写入上下文的键。
	// 与 api 包的 uidKey 字面量一致；ws **不能**导入 api（api 已导入 ws，会形成循环依赖），
	// 故这里以常量形式声明同一契约，改动时必须同步 api.uidKey。
	uidContextKey = "uid"
)

// upgrader 显式定义，避免不同入口下默认行为不一致。
//
// CheckOrigin 自托管场景先放宽（Flutter/桌面客户端不带 Origin）；
// 生产部署建议改为 Origin 白名单校验，并与 Caddy 反代的站点域名保持一致。
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// subscribeMsg 客户端上行唯一动作：订阅账本（账本 ID 由服务端校验，不信任客户端）
type subscribeMsg struct {
	Action string  `json:"action"`
	Books  []int64 `json:"books"`
}

// ServeWS 处理 GET /api/v1/ws。
//
// 鉴权由 api.wsAuth 中间件完成（浏览器 WS 无法设置 Authorization 头，故 wsAuth
// 优先读 Header，回退读 ?token=）。这里只做"取 uid → 升级 → 读写循环"。
func ServeWS(c *gin.Context) {
	uidVal, ok := c.Get(uidContextKey)
	if !ok {
		// 未鉴权：直接拒绝升级（不写业务响应体，避免和 Upgrade 的握手响应混在一起）
		c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "msg": "缺少登录凭证"})
		return
	}
	uid, ok := uidVal.(int64)
	if !ok || uid <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "msg": "缺少登录凭证"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		// Upgrade 失败时它已经写过 HTTP 响应，这里只记日志，不能再写响应体
		return
	}

	// 半开连接防护（D-R03）
	conn.SetReadLimit(readLimit)
	if err := conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		_ = conn.Close()
		return
	}
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	client := NewClient()
	client.UID = uid // 连接绑定认证用户（R05）：广播按这个 uid 做可见性过滤
	DefaultHub.Register(client)

	// 读/写循环共用一个取消上下文，任一方结束都恰好清理一次（R18）
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		DefaultHub.Unregister(client)
		_ = conn.Close()
	}()

	// 读循环：处理 subscribe（经 Hub 受锁 + 服务端校验，R05/R18）
	go func() {
		defer cancel()
		for {
			_, msg, readErr := conn.ReadMessage()
			if readErr != nil {
				return
			}
			var sub subscribeMsg
			if json.Unmarshal(msg, &sub) == nil && sub.Action == "subscribe" {
				DefaultHub.Subscribe(client, sub.Books, uid)
			}
		}
	}()

	// 写循环：发消息 + 定时 ping（保活、检测断线）
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case m := <-client.Send: // Send 永不 close，无需 ok 分支
			if err := conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, m); err != nil {
				return
			}
		case <-ticker.C:
			if err := conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				return
			}
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
