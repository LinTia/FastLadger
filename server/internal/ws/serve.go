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
	"log"
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
	// TokenExpContextKey 认证中间件写入上下文的 JWT exp（Unix 秒）。
	// 与 uidContextKey 同理：ws 不能导入 api，故由 ws 导出该契约常量，
	// api.wsAuth 直接引用它（而不是各写一份字面量），避免两边漂移。
	TokenExpContextKey = "token_exp"
	// closeTokenExpired 令牌到期时服务端主动发送的关闭码。
	// 4000-4999 是 WebSocket 规范留给应用层自定义的区间；
	// 客户端据此"重新登录后再重连"，而不是当成网络抖动无脑重连。
	closeTokenExpired = 4001
	// closeTooManyConnections 连接数超上限时服务端主动发送的关闭码（SYNC-01）。
	// 客户端据此**退避重连**（说明服务端在限连接，重试要拉开间隔），
	// 而不是立即重连形成冲击。
	closeTooManyConnections = 4002
)

// wsErrorMsg 服务端主动下行的错误通知（订阅被拒等）。
//
// 为什么必须有（GPT-R49）：订阅超限/被限流时若静默忽略，客户端会以为订阅已生效，
// 之后永远收不到该账本的事件——现象是"偶尔不实时"这种极难定位的问题。
// 因此所有拒绝都必须回一条显式错误，让客户端能提示用户或退避重试。
type wsErrorMsg struct {
	Event string `json:"event"`
	Code  string `json:"code"`
	Msg   string `json:"msg"`
}

// sendError 把一条错误通知放入发送队列（由写循环统一写出）。
//
// 刻意**不**在读循环里直接写 conn：gorilla 一条连接只允许一个并发写者，
// 写职责已归属写循环。Send 满则丢弃并计入 Dropped（WS 只是尽力通知，
// 客户端有周期 pull 与重连必 pull 兜底）。
func sendError(c *Client, code, msg string) {
	payload, err := json.Marshal(wsErrorMsg{Event: "error", Code: code, Msg: msg})
	if err != nil {
		log.Printf("ws: 错误通知序列化失败（已丢弃）code=%s err=%v", code, err)
		return
	}
	select {
	case c.Send <- payload:
	default:
		c.mu.Lock()
		c.Dropped++
		c.mu.Unlock()
	}
}

// writeRejectAndClose 在**写循环尚未启动**的阶段（注册被拒）直接写 conn：
// 此时还没有任何其它 goroutine 会写这条连接（读循环也未启动），
// 因此不违反 gorilla 的单写者约束。先回一条结构化错误帧，再发关闭帧，
// 让客户端能区分"被限流拒绝"与"网络抖动"。
func writeRejectAndClose(conn *websocket.Conn, closeCode int, code, msg string) {
	if payload, err := json.Marshal(wsErrorMsg{Event: "error", Code: code, Msg: msg}); err == nil {
		_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
		_ = conn.WriteMessage(websocket.TextMessage, payload)
	}
	_ = conn.WriteControl(websocket.CloseMessage,
		websocket.FormatCloseMessage(closeCode, code),
		time.Now().Add(writeWait))
}

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
// 优先读 Header，回退读 ?token=）。这里只做"取凭证 → 升级 → 读写循环"。
//
// GPT-R51：握手只发生一次，Upgrade 之后这条连接上不再有任何 HTTP 中间件，
// token 将被永久信任——连接可以靠 ping/pong 一直活到远超 exp 之后。
// 因此 wsAuth 必须把 JWT 的 exp 一并放进上下文（TokenExpContextKey），
// 由本函数把它作为**连接生命周期的硬上限**：到期即主动关闭并清理订阅。
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
	// 令牌有效期（exp）同样是"凭证的一部分"：缺了它就无法给连接设上限，
	// 一律 fail-closed 拒绝升级（宁可让客户端重新登录，也不放一条永生连接进来）。
	expVal, ok := c.Get(TokenExpContextKey)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "msg": "缺少登录凭证"})
		return
	}
	expUnix, ok := expVal.(int64)
	if !ok || expUnix <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "msg": "缺少登录凭证"})
		return
	}
	if expUnix <= time.Now().Unix() {
		c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "msg": "登录已失效，请重新登录"})
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

	// hub 在这里**一次性**取全局 Hub，此后本条连接的注册/注销/订阅都用这一个引用。
	//
	// 为什么不能"用到时再读 DefaultHub"：测试会临时替换 DefaultHub 并在 t.Cleanup
	// 里写回原值；而 WebSocket 是 hijacked 连接，httptest.Server.Close()
	// **不等待**被劫持的连接（ServeWS 的 goroutine 可能还活着）。
	// 于是出现了真实的数据竞争（CI 的 -race 抓到）：写是 t.Cleanup 的
	// `DefaultHub = old`，读是 ServeWS 收尾时的 `DefaultHub.Unregister`——
	// 同一份代码两次运行分别记在不同用例名下，正是因为"谁在读"随调度漂移。
	//
	// 捕获一次之后，握手完成到连接结束之间**不再有任何对全局变量的读**，
	// 竞争窗口从"整条连接生命周期"收敛为零。进程运行期 DefaultHub 不会被替换，
	// 因此这里捕获一次与逐次读取行为等价。
	hub := DefaultHub

	client := NewClient()
	client.UID = uid // 连接绑定认证用户（R05）：广播按这个 uid 做可见性过滤
	if err := hub.Register(client); err != nil {
		// SYNC-01：连接数超上限（单用户 / 全局）。必须在握手后立即明确拒绝并
		// 关闭——带结构化错误帧 + 关闭码 4002，客户端据此退避重连，而不是把
		// 这次断开当成网络抖动去冲击重连。
		log.Printf("ws: 拒绝连接（超上限）uid=%d err=%v", uid, err)
		writeRejectAndClose(conn, closeTooManyConnections, "connection_rejected", err.Error())
		_ = conn.Close()
		return
	}

	// 读/写循环共用一个取消上下文，任一方结束都恰好清理一次（R18）。
	//
	// GPT-R51：上下文的截止时间就是令牌过期时刻 —— 到期后 ctx.Done() 触发，
	// 写循环退出 → defer 注销订阅并关闭 conn → 阻塞在 ReadMessage 的读循环也随之报错退出。
	ctx, cancel := context.WithDeadline(context.Background(), time.Unix(expUnix, 0))
	defer func() {
		cancel()
		hub.Unregister(client)      // 用的是连接建立时捕获的 Hub 引用
		client.ClearSubscriptions() // 令牌到期/连接结束：订阅不留残余
		_ = conn.Close()
	}()

	// 读循环：处理 subscribe（经 Hub 受锁 + 服务端校验，R05/R18）
	//
	// GPT-R49：订阅被拒（超量/限流/无权）一律回一条显式错误帧，不静默吞掉。
	go func() {
		defer cancel()
		for {
			_, msg, readErr := conn.ReadMessage()
			if readErr != nil {
				return
			}
			var sub subscribeMsg
			if json.Unmarshal(msg, &sub) == nil && sub.Action == "subscribe" {
				if err := hub.Subscribe(client, sub.Books, uid); err != nil {
					sendError(client, "subscribe_rejected", err.Error())
				}
			}
		}
	}()

	// 写循环：发消息 + 定时 ping（保活、检测断线）
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			// 令牌到期（或读循环已结束）：主动发一次关闭帧告知客户端"必须重新鉴权"，
			// 而不是让它把这次断开当成网络抖动去无脑重连。
			_ = conn.WriteControl(websocket.CloseMessage,
				websocket.FormatCloseMessage(closeTokenExpired, "token expired"),
				time.Now().Add(writeWait))
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
