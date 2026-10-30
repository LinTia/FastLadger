// Package ws WebSocket 实时推送（阶段 3.3）。
//
// 口径照抄《新手开发完整指南》§14.1/§14.3，并补上本阶段的一条硬约束：
//
//	**广播路径不得在 Hub 锁内查库**（家庭 NAS 上 SQLite 只有 1 个连接，
//	锁内查库会把一次慢查询放大成"所有人收不到通知"）。
//
// 因此 Client 额外缓存每个已订阅账本的 canView（能否看到他人明细）结果：
//   - Subscribe 时由服务端校验并写入缓存（查库发生在 Hub 锁**外**）；
//   - BroadcastToBook 只读缓存决定发"完整载荷"还是"脱敏载荷"；
//   - 可见性/成员变化时由 RefreshVisibility 批量重算缓存（先在锁外查库，再逐 client 写入）。
//
// 锁顺序（R18，**固定单向，禁止反转**）：
//
//	Hub.mu → Client.mu
//
// 任何路径都不得持有 Client.mu 后再去取 Hub.mu，否则 Subscribe/Broadcast 与
// RefreshVisibility 之间会死锁。本文件所有加锁处均遵守该顺序。
//
// 依赖方向：ws 可以依赖 service（用于可见性判定，生产由 main 注入 service.CanViewDetail）；
// service **不得**导入 ws，广播调用点只放在 api 适配层。
package ws

import (
	"log"
	"sync"
)

const (
	// sendBufferSize 单连接发送缓冲（256 条通知）：WS 只是"尽力通知"，
	// 满则由客户端周期性 pull 兜底（指南 §14.1 约定 5）。
	sendBufferSize = 256
)

// VisibilityChecker 订阅资格 + 可见性校验。
//
// 生产实现由 main 注入 service.CanViewDetail：
//
//	ws.DefaultHub.SetChecker(func(uid, bookID int64) (bool, error) {
//	    return service.CanViewDetail(engine, uid, bookID)
//	})
//
// 返回 (false, ErrNotMember) 表示非成员 —— Subscribe 一律拒绝订阅（不泄露账本是否存在）。
// 返回 (canView, nil) 表示是成员，canView 决定广播时发完整还是脱敏载荷。
// 用函数类型而非接口，是为了让单测注入假实现（本机 CGO 不可用，无法跑真实 SQLite）。
type VisibilityChecker func(uid, bookID int64) (bool, error)

// Client 一个客户端连接。
type Client struct {
	// mu 保护 BookIds / canView / Dropped（Hub.mu 之下才可取，见包注释的锁顺序）
	mu sync.Mutex
	// BookIds 仅保存"已通过服务端校验"的订阅账本（客户端上报的账本 ID 不被信任）
	BookIds map[int64]bool
	// Send 约定：**永不 close**。多广播者并发发送，关闭 channel 会在其他 goroutine
	// 发送时 panic；连接结束只做 Unregister，channel 交给 GC 回收。
	Send chan []byte
	// Dropped Send 满导致丢弃的通知数（c.mu 下维护，用于告警"客户端该全量重同步了"）
	Dropped int64
	// UID 连接绑定的认证用户（R05）
	UID int64
	// canView 账本 → 能否看到【他人】明细（订阅时由服务端校验结果缓存，
	// 广播只读它，避免在 Hub 锁内查库）
	canView map[int64]bool
	// visSeq 账本 → 可见性写入序号（**乐观写保护**）。
	//
	// RefreshVisibility 的三段式（快照 → 锁外查库 → 回写）之间存在窗口：若在"快照之后、
	// 回写之前"有 Subscribe 写入了更新鲜的可见性结果，旧的 Refresh 结果会把新值覆盖回去，
	// 该连接就会按过期权限收发明细。Subscribe 每次写某账本的 canView 时把 visSeq 自增，
	// RefreshVisibility 回写前比对自己快照到的序号，不一致即放弃本次写入（让更新的结果生效）。
	visSeq map[int64]uint64
}

// NewClient 构造一个连接级客户端（Send/BookIds/canView/visSeq 均已初始化）。
func NewClient() *Client {
	return &Client{
		BookIds: make(map[int64]bool),
		Send:    make(chan []byte, sendBufferSize),
		canView: make(map[int64]bool),
		visSeq:  make(map[int64]uint64),
	}
}

// ensureLocked 补齐可能为 nil 的 map（**必须在持有 c.mu 时调用**）。
//
// 为什么需要：`Client` 也可能被字面量构造（`&Client{}`，测试与将来的其他入口），
// 此时 BookIds/canView/visSeq 都是 nil；对 nil map 赋值会 panic，对 nil map 读取则安全。
// 统一走本方法可以保证 Subscribe / BroadcastToBook / RefreshVisibility 三条路径都不会 panic。
func (c *Client) ensureLocked() {
	if c.BookIds == nil {
		c.BookIds = make(map[int64]bool)
	}
	if c.canView == nil {
		c.canView = make(map[int64]bool)
	}
	if c.visSeq == nil {
		c.visSeq = make(map[int64]uint64)
	}
	// 注意：**不补 Send**。Send 只在构造时创建一次（NewClient），运行期从不替换，
	// 这样写循环可以无锁读取该字段；字面量构造出的连接若没有 Send，
	// BroadcastToBook 会跳过它（宁可少投递，也不能凭空造一个无人读取的缓冲）。
}

// CanView 读取某账本可见性缓存（测试与运维排障用；广播内部走同一份数据）。
func (c *Client) CanView(bookID int64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ensureLocked()
	return c.canView[bookID]
}

// Subscribed 是否已订阅某账本（测试用）。
func (c *Client) Subscribed(bookID int64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ensureLocked()
	return c.BookIds[bookID]
}

// DroppedCount 丢弃通知计数（测试用）。
func (c *Client) DroppedCount() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Dropped
}

// Hub 客户端注册表。
type Hub struct {
	// mu 保护 clients 与 checker（锁顺序的第一层）
	mu      sync.RWMutex
	clients map[*Client]bool
	// checker 可见性/成员资格校验；未注入时 **fail-closed**（拒绝全部订阅）
	checker VisibilityChecker
}

// NewHub 构造 Hub（checker 允许为 nil，此时订阅一律被拒绝并记日志）。
func NewHub(checker VisibilityChecker) *Hub {
	return &Hub{clients: make(map[*Client]bool), checker: checker}
}

// DefaultHub 进程级默认 Hub（api 适配层广播、ServeWS 注册都用它）。
var DefaultHub = NewHub(nil)

// SetChecker 注入可见性校验器（main 启动时调用一次；测试可注入假实现）。
func (h *Hub) SetChecker(checker VisibilityChecker) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checker = checker
}

// Register 注册连接（引用计数语义：重复注册无副作用）。
func (h *Hub) Register(c *Client) {
	if c == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c] = true
}

// Unregister 注销连接：只从 clients 删除，**不关闭 Send**（见 Client.Send 约定）。
func (h *Hub) Unregister(c *Client) {
	if c == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, c)
}

// Count 在线连接数（观测/测试用）。
func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Subscribe 经 Hub 受锁更新订阅（R18：所有订阅写都走同一把锁）。
//
// 每个 bookID 先由服务端校验成员资格与可见性（R05），**查库在 Hub 锁外完成**，
// 拿到结果后再进锁写入 —— 既保证"订阅写的唯一入口在 Hub"，
// 又不会让一次慢查询卡住所有广播。
func (h *Hub) Subscribe(c *Client, bookIDs []int64, uid int64) {
	if c == nil {
		return
	}
	// 1) 取校验器（只在读锁内读指针，不做任何 IO）
	h.mu.RLock()
	checker := h.checker
	h.mu.RUnlock()

	allowed := make(map[int64]bool, len(bookIDs))
	if checker == nil {
		// fail-closed：没有校验器就不能相信客户端上报的账本 ID
		log.Printf("ws: 未注入可见性校验器，拒绝全部订阅（fail-closed）uid=%d", uid)
	} else {
		// 2) 锁外查库校验（每个 bookID 独立判定，失败/非成员一律不订阅）
		for _, b := range bookIDs {
			if b <= 0 {
				continue
			}
			canView, err := checker(uid, b)
			if err != nil {
				// 非成员 / 查询失败：静默跳过，不回显"账本是否存在"
				continue
			}
			allowed[b] = canView
		}
	}

	// 3) 订阅写全部在 Hub.mu 下完成（锁顺序 Hub.mu → Client.mu）
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.clients[c] {
		// 连接已注销（读循环刚结束）：不再写入，避免给已回收的连接挂状态
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ensureLocked()
	for b, canView := range allowed {
		c.BookIds[b] = true
		c.canView[b] = canView
		// 自增写入序号：让"快照更早"的 RefreshVisibility 回写时能发现本次更新并放弃
		c.visSeq[b]++
	}
	c.UID = uid
}

// BroadcastToBook 给"已订阅该账本"的在线成员推消息。
//
// R05：按接收者权限过滤。canView 缓存为 true 时发 fullMsg，否则发 sensitiveMsg
// （脱敏通知：只含 event/bookId/version，不含金额/账户/备注，由客户端再走授权 pull 拉明细）。
// 缓存缺失按 **false** 处理（fail-closed）：宁可少发一次明细，也不能多发。
//
// 非阻塞投递：Send 满不阻塞广播；丢弃由客户端周期性 pull 兜底。
func (h *Hub) BroadcastToBook(bookID int64, fullMsg, sensitiveMsg []byte) {
	if fullMsg == nil && sensitiveMsg == nil {
		return
	}
	// 只给一条载荷时，两条通道都用它（例如 data_changed 事件本身已脱敏）
	if fullMsg == nil {
		fullMsg = sensitiveMsg
	}
	if sensitiveMsg == nil {
		sensitiveMsg = fullMsg
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		// 锁顺序 Hub.mu(读) → Client.mu
		c.mu.Lock()
		c.ensureLocked()
		sub := c.BookIds[bookID]
		canViewAll := c.canView[bookID]
		send := c.Send
		c.mu.Unlock()
		if !sub || send == nil {
			continue
		}
		payload := sensitiveMsg
		if canViewAll {
			payload = fullMsg
		}
		select {
		case send <- payload:
		default:
			c.mu.Lock()
			c.Dropped++
			c.mu.Unlock()
		}
	}
}

// RefreshVisibility 可见性/成员关系变化后，批量重算受影响连接的 canView 缓存。
//
// 调用时机（api 层）：SetVisibility 成功后、成员被移除/改角色/账本删除后。
// 实现刻意分三段，确保**查库不在 Hub 锁内**：
//  1. 持 Hub 读锁快照"订阅了该账本的 client + 其 uid + 该账本的可见性写入序号"，随后立刻释放；
//  2. 锁外逐个查库算 canView；
//  3. 逐个持 Client.mu 写缓存（此时不再持有 Hub 锁，锁顺序依然是 Hub → Client，
//     因为第 1 段已释放 Hub 锁，不存在"持 Client.mu 再取 Hub.mu"的反向路径）。
//
// 第 3 段带**乐观写保护**：若期间有 Subscribe 写入了更新鲜的结果（visSeq 变了），
// 本次结果已过期，直接放弃写入 —— 否则会用旧权限覆盖新权限（见 Client.visSeq 注释）。
//
// **非成员即摘除订阅**：checker 返回错误表示"已不是该账本成员"（被移除 / 退出 / 账本已删），
// 此时除了把 canView 置 false，还必须 `delete(c.BookIds, bookID)`：
// 否则被移除的连接会继续收到该账本的 data_changed，等于持续泄露"这个账本还在被使用"
// （PRD §12.1 决策 5：被移除成员应停止订阅）；同时它也不会再占用广播遍历的开销。
// 摘除仍在同一把 c.mu 内完成，锁顺序不变（未反向获取 Hub.mu）。
// 注：数据库瞬时故障也会返回 error，此时按 fail-closed 处理（摘订阅）是可接受的降级——
// 客户端重连后会自动重新订阅，反之"该摘没摘"是不可恢复的隐私泄露。
func (h *Hub) RefreshVisibility(bookID int64) {
	if bookID <= 0 {
		return
	}
	type target struct {
		c   *Client
		uid int64
		seq uint64 // 快照时刻该账本在 client 上的可见性写入序号
	}
	// decision 锁外查库的结果：canView + 是否仍是成员
	type decision struct {
		canView bool
		member  bool
	}

	// 1) 快照（短读锁，不做 IO）
	h.mu.RLock()
	checker := h.checker
	targets := make([]target, 0, len(h.clients))
	for c := range h.clients {
		c.mu.Lock()
		c.ensureLocked()
		sub := c.BookIds[bookID]
		uid := c.UID
		seq := c.visSeq[bookID]
		c.mu.Unlock()
		if sub {
			targets = append(targets, target{c: c, uid: uid, seq: seq})
		}
	}
	h.mu.RUnlock()
	if len(targets) == 0 {
		return
	}
	if checker == nil {
		// 没有校验器就无法重算：把所有订阅者的可见性降级为 false（fail-closed），
		// 变为只发脱敏通知，直到下次 Subscribe 重新校验。
		for _, t := range targets {
			t.c.mu.Lock()
			t.c.ensureLocked()
			if t.c.visSeq[bookID] != t.seq {
				t.c.mu.Unlock()
				continue // 期间有更新的 Subscribe 结果，放弃覆盖
			}
			t.c.canView[bookID] = false
			t.c.visSeq[bookID]++
			t.c.mu.Unlock()
		}
		return
	}

	// 2) 锁外查库（同一 uid 只查一次）
	result := make(map[int64]decision, len(targets))
	for _, t := range targets {
		if _, done := result[t.uid]; done {
			continue
		}
		canView, err := checker(t.uid, bookID)
		if err != nil {
			// 已不是成员：既不能看明细，也不该继续订阅
			result[t.uid] = decision{canView: false, member: false}
			continue
		}
		result[t.uid] = decision{canView: canView, member: true}
	}

	// 3) 写缓存（锁顺序 Hub.mu 已释放 → Client.mu），带序号比对
	for _, t := range targets {
		d := result[t.uid]
		t.c.mu.Lock()
		t.c.ensureLocked()
		if t.c.visSeq[bookID] != t.seq {
			// 快照之后 Subscribe 写入了更新的结果：本次结果已过期，放弃写入
			t.c.mu.Unlock()
			continue
		}
		if !d.member {
			// 已不是成员 → 摘除订阅（PRD §12.1 决策 5），并把可见性收紧到最保守
			// 注意：不在这里 close(Send)（约定永不关闭），只丢弃订阅状态。
			delete(t.c.BookIds, bookID)
			delete(t.c.canView, bookID)
			t.c.visSeq[bookID]++
			t.c.mu.Unlock()
			continue
		}
		t.c.canView[bookID] = d.canView
		t.c.visSeq[bookID]++
		t.c.mu.Unlock()
	}
}
