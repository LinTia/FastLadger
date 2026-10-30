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
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
)

const (
	// sendBufferSize 单连接发送缓冲（256 条通知）：WS 只是"尽力通知"，
	// 满则由客户端周期性 pull 兜底（指南 §14.1 约定 5）。
	sendBufferSize = 256

	// MaxBookIDsPerFrame 单帧 subscribe 消息允许携带的账本 ID 上限（**去重后**计数）。
	//
	// 为什么必须是常量而不是"按消息大小限制"：每个 ID 都要触发一次 SQLite 鉴权查询，
	// 而连接池 MaxOpenConns=1 —— 一帧 N 个 ID 就是 N 次**串行**查询，
	// 期间所有广播/其它请求的查库全被堵住（表现为"所有人收不到通知"）。
	// 64 KB 的 readLimit 能装下数千个 ID，字节级上限根本挡不住这种放大。
	// 家庭记账场景单人参与的账本数远小于 64，取 64 既不影响正常使用，
	// 又把单帧可放大的查询量封死在常量级。
	MaxBookIDsPerFrame = 64
	// MaxSubscriptionsPerClient 单个连接可同时订阅的账本数上限。
	//
	// 单连接的订阅越多，BroadcastToBook 遍历它、RefreshVisibility 重算它的成本越高，
	// 而这两条路径都在 Hub 锁内/锁下，等于让一个连接拖慢所有人。
	MaxSubscriptionsPerClient = 200
	// MaxSubscriptionsPerUser 单个用户（同一 uid 的**全部**在线连接合计）订阅条目上限。
	//
	// 按"连接 × 账本"计：RefreshVisibility 对每个在线连接各查一次库（同 uid 只查一次），
	// 因此同一用户开再多设备也不能把总量放大到无界。
	MaxSubscriptionsPerUser = 600
	// subscribeBurst 订阅消息令牌桶容量（同一连接连续订阅帧的爆发上限）
	subscribeBurst = 32
	// subscribeRefill 令牌桶补充速率（每秒补充的令牌数）
	subscribeRefill = 10
	// maxSubscribeAttempts 因权限 generation 变化而重新鉴权的最大次数。
	//
	// 取有限值而不是无限重试：恶意/异常路径下 generation 可能被持续推进，
	// 无限重试会把唯一的 SQLite 连接占满；达到上限即返回明确错误让客户端稍后重试。
	maxSubscribeAttempts = 4
)

// 订阅被拒的原因（ServeWS 会把它转成显式错误帧回给客户端，**绝不静默截断**）。
var (
	// ErrNoVisibilityChecker 服务端未注入可见性校验器（fail-closed）
	ErrNoVisibilityChecker = errors.New("服务端未配置账本可见性校验，拒绝订阅")
	// ErrTooManyBookIDs 单帧账本 ID 数超过上限（GPT-R49）
	ErrTooManyBookIDs = errors.New("单帧订阅的账本数量超过上限")
	// ErrSubscriptionLimitPerConnection 单连接订阅总数超过上限（GPT-R49）
	ErrSubscriptionLimitPerConnection = errors.New("单连接订阅的账本数量超过上限")
	// ErrSubscriptionLimitPerUser 单用户订阅总数超过上限（GPT-R49）
	ErrSubscriptionLimitPerUser = errors.New("单用户订阅的账本数量超过上限")
	// ErrSubscribeRateLimited 订阅消息过于频繁（GPT-R49）
	ErrSubscribeRateLimited = errors.New("订阅消息过于频繁，请稍后重试")
	// ErrSubscribeGenerationStale 鉴权期间账本权限反复变更，重试次数耗尽（GPT-R50）
	ErrSubscribeGenerationStale = errors.New("订阅期间账本权限持续变更，请稍后重新订阅")
	// ErrSubscribeClientGone 连接已注销，不再接受订阅写入
	ErrSubscribeClientGone = errors.New("连接已注销")
	// ErrTooManyConnections 全局 WebSocket 连接数已达上限（SYNC-01）
	ErrTooManyConnections = errors.New("服务端在线连接数已达上限，请稍后重试")
	// ErrTooManyConnectionsForUser 单用户 WebSocket 连接数已达上限（SYNC-01）
	ErrTooManyConnectionsForUser = errors.New("该账号的在线连接数已达上限，请先断开其它设备")
	// errStaleGeneration 内部哨兵：本次鉴权结果已过期，需重新鉴权（不外泄给调用方）
	errStaleGeneration = errors.New("stale book permission generation")
)

const (
	// maxSubBuckets 按 uid 计量的订阅令牌桶容量上限（SYNC-01）。
	//
	// 桶的键是 uid，条目数因此有界于"曾经订阅过的用户数"，本就不会爆炸；
	// 上限是兜底：满桶时**无损**回收"当前满桶"的条目（满桶状态可由
	// tokens=burst 完全重建），不淘汰活跃用户的桶。
	maxSubBuckets = 4096
	// maxGenerationEntries 权限 generation 表的条目上限（SYNC-04/07）。
	//
	// 每次成员/可见性/删除事件都会让对应账本的 generation 登记一项，且账本删除
	// 并不会让条目消失 —— 不设上限就是一个只增不减的 map。上限取
	// 1<<20：家庭场景可能出现的"发生过权限事件的账本数"远小于此，正常永不触顶；
	// 触顶后的行为见 bumpGenerationLocked（**绝不 delete**，改为溢出计数）。
	maxGenerationEntries = 1 << 20
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

// SubscriptionCount 当前订阅的账本数（观测/测试用）。
func (c *Client) SubscriptionCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.BookIds)
}

// subBucket 按 uid 计量的订阅令牌桶状态（SYNC-01：限流必须按 uid 而不是按连接）。
type subBucket struct {
	tokens float64
	last   time.Time
}

// consumeSubToken 扣减某 uid 的一个"订阅消息"令牌；令牌不足即被限流
// （调用方应直接拒绝，**不要**再去查库）。
//
// 为什么按 uid 而不是按连接（GPT-R49 原实现是 Client 级）：令牌桶按连接计，
// 同一用户多开 N 条连接就把鉴权查询量放大 N 倍 —— 而连接本身才是要被
// SYNC-01 挡住的东西，限流器不能再被它绕过。与 service 的邀请限流桶
// （key 是 IP/接口）key 空间完全隔离：这里的键是 uid，且桶只存在于 Hub 内。
func (h *Hub) consumeSubToken(uid int64) bool {
	h.bucketMu.Lock()
	defer h.bucketMu.Unlock()
	if h.subBuckets == nil {
		h.subBuckets = make(map[int64]*subBucket) // 字面量构造的 Hub 兜底
	}
	if uid <= 0 {
		uid = 0 // 无 uid 的调用方共享一个桶（生产不会出现，防御性收敛）
	}
	now := time.Now()
	b := h.subBuckets[uid]
	if b == nil {
		if len(h.subBuckets) >= maxSubBuckets {
			h.pruneSubBucketsLocked()
		}
		b = &subBucket{tokens: subscribeBurst, last: now}
		h.subBuckets[uid] = b
	}
	if elapsed := now.Sub(b.last).Seconds(); elapsed > 0 {
		b.tokens += elapsed * subscribeRefill
		if b.tokens > subscribeBurst {
			b.tokens = subscribeBurst
		}
		b.last = now
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// pruneSubBucketsLocked 回收"当前满桶"的条目（**必须在持有 h.bucketMu 时调用**）。
//
// 满桶状态可以无损重建：一个 tokens 已回到 subscribeBurst 的桶，删掉之后
// 下次消费会按满桶重建 —— 语义完全一致，因此这是无损回收而不是淘汰活跃用户。
func (h *Hub) pruneSubBucketsLocked() {
	for uid, b := range h.subBuckets {
		if len(h.subBuckets) < maxSubBuckets/2 {
			return
		}
		if b.tokens >= subscribeBurst {
			delete(h.subBuckets, uid)
		}
	}
}

// ClearSubscriptions 清空该连接的全部订阅与可见性缓存。
//
// 调用时机：令牌到期 / 连接结束（GPT-R51）。Unregister 只把连接移出注册表，
// 但 BroadcastToBook 与 RefreshVisibility 可能已经取到该 client 的指针，
// 清空能让它们在锁下看到"无任何订阅"，从而不再投递、不再查库。
func (c *Client) ClearSubscriptions() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for b := range c.BookIds {
		delete(c.BookIds, b)
	}
	for b := range c.canView {
		delete(c.canView, b)
	}
	for b := range c.visSeq {
		delete(c.visSeq, b)
	}
}

// Hub 客户端注册表。
type Hub struct {
	// mu 保护 clients / checker / gen（锁顺序的第一层）
	mu      sync.RWMutex
	clients map[*Client]bool
	// checker 可见性/成员资格校验；未注入时 **fail-closed**（拒绝全部订阅）
	checker VisibilityChecker
	// gen 账本 → 权限 generation（成员/可见性每次变更 +1，见 GPT-R50）。
	//
	// 为什么需要它：Subscribe 的查库发生在 Hub 锁**外**（不能在锁内查库），
	// 于是"鉴权完成 → 写入订阅"之间存在窗口。期间若发生撤权，
	// RefreshVisibility 看不到这个"还没写 BookIds"的 pending 连接（它不在任何订阅表里），
	// 旧的 allow=true 随后被写回，等于撤权后订阅又被恢复。
	// generation 让 Subscribe 能在写入【前】发现自己拿到的是过期结论并重新鉴权。
	//
	// SYNC-07：值取自全局单调计数器 genSeq —— **0 唯一表示"从未发生过权限事件"，
	// 且任何账本的值一旦登记就永不归零、永不复用**（触顶时不删除条目，见
	// bumpGenerationLocked）。这样"值变化 ⇒ 期间发生过权限变更"的方向性才成立。
	gen map[int64]uint64
	// genSeq 全局单调计数器：所有 gen 值与 genOverflow 都从这里取值（SYNC-07）
	genSeq uint64
	// genOverflow gen 表触顶后"无法为新账本登记事件"的序号（SYNC-07）。
	// 快照里带着它：writeSubscribe 发现它变了就让整帧重新鉴权（fail-closed）。
	genOverflow uint64

	// ---- 连接数上限（SYNC-01）----
	// maxConnPerUser / maxConnTotal 连接数上限；**0 = 不限**。
	// NewHub 的缺省是不限（保持既有行为、测试不受影响）；生产由 api.NewRouter
	// 从 config 注入（EBK_WS_MAX_CONN_PER_USER / EBK_WS_MAX_CONN_TOTAL）。
	maxConnPerUser int
	maxConnTotal   int
	// connPerUser 各 uid 当前在线连接数（与 clients 同一把 h.mu 保护）。
	// 按"注册时刻的 UID"记账：生产路径 ServeWS 在 Register 之前就绑定了认证 uid。
	connPerUser map[int64]int

	// ---- 订阅消息限流（按 uid 计量，SYNC-01）----
	// subBuckets 键是 uid。与 clients 不同锁（bucketMu），避免订阅限流的记账
	// 和广播路径的读锁互相影响。
	subBuckets map[int64]*subBucket
	bucketMu   sync.Mutex
}

// NewHub 构造 Hub（checker 允许为 nil，此时订阅一律被拒绝并记日志）。
//
// 连接数上限缺省为"不限"（0），由 api.NewRouter 启动时用 config 注入 ——
// 这样直接构造 Hub 的既有调用方（含测试）行为不变，生产路径一定有上限。
func NewHub(checker VisibilityChecker) *Hub {
	return &Hub{
		clients:     make(map[*Client]bool),
		checker:     checker,
		gen:         make(map[int64]uint64),
		connPerUser: make(map[int64]int),
		subBuckets:  make(map[int64]*subBucket),
	}
}

// DefaultHub 进程级默认 Hub（api 适配层广播、ServeWS 注册都用它）。
var DefaultHub = NewHub(nil)

// SetConnectionLimits 设置连接数上限（SYNC-01；**≤0 表示该维度不限**）。
//
// 生产接线点：api.NewRouter 用 config.WSMaxConnPerUser / WSMaxConnTotal 调用
// （config.Load 已保证两个值恒为正整数）；测试可注入更小的值。
func (h *Hub) SetConnectionLimits(perUser, total int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.maxConnPerUser, h.maxConnTotal = perUser, total
}

// ConnectionLimits 读取当前连接数上限（per-uid, 全局；观测/测试用）。
//
// 主要供接线守卫用例使用：断言生产接线（api.NewRouter → config）真的把上限
// 注入了 Hub —— 若有人重构路由时删掉这段接线，守卫用例直接红。
func (h *Hub) ConnectionLimits() (perUser, total int) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.maxConnPerUser, h.maxConnTotal
}

// SetChecker 注入可见性校验器（main 启动时调用一次；测试可注入假实现）。
func (h *Hub) SetChecker(checker VisibilityChecker) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checker = checker
}

// Register 注册连接（幂等：重复注册无副作用）。
//
// SYNC-01：注册即配额审查 —— 超出"单用户连接数"或"全局连接数"上限时**拒绝注册**
// 并返回明确错误；ServeWS 收到错误会在握手后立即向客户端回错误帧 + 关闭帧
// （close code 4002），绝不静默断开。
//
// 计数口径：per-uid 按"注册时刻的 c.UID"记账（生产路径 ServeWS 在 Register 前
// 已绑定认证 uid，之后不变）。
func (h *Hub) Register(c *Client) error {
	if c == nil {
		return ErrSubscribeClientGone
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[c] {
		return nil // 幂等：已在注册表里，不重复计数
	}
	// 读 UID 需要短暂取 c.mu（锁顺序 Hub.mu → Client.mu，单向）
	c.mu.Lock()
	uid := c.UID
	c.mu.Unlock()
	if h.maxConnTotal > 0 && len(h.clients) >= h.maxConnTotal {
		return fmt.Errorf("%w：当前在线 %d，上限 %d", ErrTooManyConnections, len(h.clients), h.maxConnTotal)
	}
	if h.maxConnPerUser > 0 && uid > 0 && h.connPerUser[uid] >= h.maxConnPerUser {
		return fmt.Errorf("%w：该账号当前在线 %d 条连接，上限 %d",
			ErrTooManyConnectionsForUser, h.connPerUser[uid], h.maxConnPerUser)
	}
	h.clients[c] = true
	if uid > 0 {
		h.connPerUser[uid]++
	}
	return nil
}

// Unregister 注销连接：从注册表删除并回收 per-uid 连接计数，**不关闭 Send**
// （见 Client.Send 约定）。对未注册的连接是 no-op。
func (h *Hub) Unregister(c *Client) {
	if c == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.clients[c] {
		return
	}
	delete(h.clients, c)
	c.mu.Lock()
	uid := c.UID
	c.mu.Unlock()
	// 防御：UID 在注册后理论不变（生产路径成立）；即便被改也不会把计数减成负数
	if uid > 0 && h.connPerUser[uid] > 0 {
		h.connPerUser[uid]--
	}
}

// Count 在线连接数（观测/测试用）。
func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Generation 读取某账本当前的权限 generation（观测/测试用）。
//
// 每次 RefreshVisibility（成员移除/角色变更/可见性变更/账本删除）都会 +1，
// **包括没有任何订阅者时**——那正是 pending Subscribe 需要被作废的时刻（GPT-R50）。
func (h *Hub) Generation(bookID int64) uint64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.gen[bookID] // nil map 读取安全，返回 0
}

// GenerationCount 当前 gen 表的条目数（观测/测试用；SYNC-04 的有界性验证）。
func (h *Hub) GenerationCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.gen)
}

// UserSubscriptionCount 某用户当前在全部在线连接上的订阅条目总数
// （连接 × 账本；观测/测试与运维排障用）。
func (h *Hub) UserSubscriptionCount(uid int64) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.userSubscriptionCountLocked(uid)
}

// bumpGenerationLocked 登记某账本的一次权限事件（**必须在持有 h.mu 写锁时调用**）。
//
// SYNC-07：值取自全局单调计数器（而不是"本账本 +1"），因此：
//   - 0 唯一表示"该账本从未发生过权限事件"；
//   - 任何账本的值一旦登记就**永不归零、永不复用**；
//   - "快照值 != 当前值"严格等价于"这期间发生过针对它的权限事件"。
//
// 触顶时**绝不 delete 条目**（那会把 gen 拉回 0，与"从未有事件"不可区分，
// 已被验证探针证伪：可把已撤销成员的订阅重新写回）。改为推进 genOverflow：
// 快照里带着溢出序号，writeSubscribe 发现变化就让整帧重新鉴权 —— fail-closed，
// 代价只是几次额外查询；触发前提是 100 万个不同账本都发生过权限事件，实际不可达。
func (h *Hub) bumpGenerationLocked(bookID int64) {
	if h.gen == nil {
		// 字面量构造的 Hub（&Hub{}）：补出来，避免"assignment to entry in nil map"
		h.gen = make(map[int64]uint64)
	}
	h.genSeq++
	if _, ok := h.gen[bookID]; !ok && len(h.gen) >= maxGenerationEntries {
		h.genOverflow = h.genSeq
		return
	}
	h.gen[bookID] = h.genSeq
}

// genSnapshot 一次订阅的权限快照（写入前用于判断"结论是否已过期"）。
//
// SYNC-06：perBook **必须覆盖整帧**（而不是只覆盖本轮重验的子集）——
// writeSubscribe 对整帧逐账本比对，快照缺项会读到 0，把"gen>0 且未在本轮
// 重验"的账本误判为 stale，造成重试在两个子集间震荡直到耗尽。
// 因此 Subscribe 持有一个覆盖整帧的持久快照，每轮只把本轮 pending 的最新值
// 合并进来：每个账本都拿"它取得结论那一刻"的 gen 值比对。
// SYNC-07：overflow 让 gen 表触顶后的新账本事件也能作废 in-flight 结论。
type genSnapshot struct {
	perBook  map[int64]uint64
	overflow uint64
}

// generationSnapshot 读取若干账本当前的 generation 与溢出序号（读锁，不做 IO）。
func (h *Hub) generationSnapshot(ids []int64) genSnapshot {
	h.mu.RLock()
	defer h.mu.RUnlock()
	snap := genSnapshot{
		perBook:  make(map[int64]uint64, len(ids)),
		overflow: h.genOverflow,
	}
	for _, id := range ids {
		snap.perBook[id] = h.gen[id]
	}
	return snap
}

// userSubscriptionCountLocked 统计某用户在**全部在线连接**上的订阅条目总数
// （连接 × 账本；**必须在持有 h.mu 时调用**）。
//
// 锁顺序：本函数在持有 Hub.mu 的前提下逐个取 Client.mu，仍是 Hub → Client 单向序。
// 调用方务必在取 c.mu **之前**调用它（见 writeSubscribe）。
func (h *Hub) userSubscriptionCountLocked(uid int64) int {
	total := 0
	for c := range h.clients {
		c.mu.Lock()
		if c.UID == uid {
			total += len(c.BookIds)
		}
		c.mu.Unlock()
	}
	return total
}

// normalizeBookIDs 规范化客户端上报的账本 ID：剔除非法值 → **去重** → 施加单帧上限。
//
// 必须在任何查库【之前】完成：去重决定了查库次数，上限决定了单帧能放大的最大查询量。
// 超限返回错误（调用方回显给客户端），绝不静默截断——截断会让客户端以为订阅成功了。
func normalizeBookIDs(bookIDs []int64) ([]int64, error) {
	seen := make(map[int64]struct{}, len(bookIDs))
	ids := make([]int64, 0, len(bookIDs))
	for _, b := range bookIDs {
		if b <= 0 {
			continue // 非法 ID：直接丢弃，不查库、不回显"账本是否存在"
		}
		if _, dup := seen[b]; dup {
			continue // 重复 ID：只鉴权一次
		}
		seen[b] = struct{}{}
		ids = append(ids, b)
	}
	if len(ids) > MaxBookIDsPerFrame {
		return nil, fmt.Errorf("%w：本次 %d 个（去重后），上限 %d",
			ErrTooManyBookIDs, len(ids), MaxBookIDsPerFrame)
	}
	return ids, nil
}

// Subscribe 经 Hub 受锁更新订阅（R18：所有订阅写都走同一把锁）。
//
// 每个 bookID 先由服务端校验成员资格与可见性（R05），**查库在 Hub 锁外完成**，
// 拿到结果后再进锁写入 —— 既保证"订阅写的唯一入口在 Hub"，
// 又不会让一次慢查询卡住所有广播。
//
// 处理顺序（每一环都在"花钱"之前把不该发生的请求挡掉，被拒的帧**零查库**）：
//  1. **频率限制（按 uid 计量，SYNC-01/02）**：先扣令牌再解析 —— 否则解析开销
//     最大的超限帧（64KB ≈ 8000 个 ID）反而是唯一不限速的路径；限流按 uid 而非
//     连接，否则多开连接即可绕过；
//  2. 规范化输入：剔除非法 ID、**去重**、单帧数量上限（MaxBookIDsPerFrame）；
//  3. **配额预检（SYNC-01）**：单连接 / 单用户订阅总数若已放不下这一帧，
//     在任何查库之前直接拒绝（旧实现是"先查库、后配额"——钱花了才查票）；
//  4. 鉴权 → 写入（见下）。
//     绝不静默截断：截断会让客户端以为订阅成功，之后永远等不到事件，
//     现象是"偶尔不实时"这种极难定位的问题。
//
// GPT-R50（撤权竞态）：查库在锁外 ⇒ "鉴权完成 → 写入订阅"之间是敞开的窗口。
// 期间一旦发生撤权，RefreshVisibility **看不到**这个还没写 BookIds 的连接，
// 旧的 allow=true 若被直接写回，撤权就白做了。因此：
//   - 鉴权【前】快照各账本的权限 generation；
//   - 写入【前】在 Hub 锁内比对 —— 变了就丢弃本次结论、重新鉴权（最多 maxSubscribeAttempts 次）；
//   - **重试只重新鉴权 generation 变过的账本（SYNC-03）**，且重试同样扣令牌：
//     否则一次订阅可借"权限持续变更"把单帧查询量放大 maxSubscribeAttempts 倍；
//   - 撤权路径 RefreshVisibility **即使没有订阅者也要递增 generation**（见该方法注释）。
//
// 即：不得把"锁外鉴权结果"直接写回。
func (h *Hub) Subscribe(c *Client, bookIDs []int64, uid int64) error {
	if c == nil {
		return ErrSubscribeClientGone
	}
	// 1) 频率限制（按 uid）：令牌不足直接拒绝，零解析、零查库
	if !h.consumeSubToken(uid) {
		return ErrSubscribeRateLimited
	}
	// 2) 规范化输入（去重 + 单帧上限）—— 在任何查库之前完成
	ids, err := normalizeBookIDs(bookIDs)
	if err != nil {
		return err
	}
	// 3) 配额预检：放不下这一帧就别查库（SYNC-01 的关键顺序修正）
	if err := h.checkSubscriptionQuota(c, uid, ids); err != nil {
		return err
	}

	// 4) 取校验器（只在读锁内读指针，不做任何 IO）
	h.mu.RLock()
	checker := h.checker
	h.mu.RUnlock()
	if checker == nil {
		// fail-closed：没有校验器就不能相信客户端上报的账本 ID
		log.Printf("ws: 未注入可见性校验器，拒绝全部订阅（fail-closed）uid=%d", uid)
		return ErrNoVisibilityChecker
	}

	// 5) 鉴权 → 写入（写入前比对 generation，过期就只重鉴权变过的账本）
	allowed := make(map[int64]bool, len(ids))
	// SYNC-06：快照**覆盖整帧且跨轮持久**——每轮只把本轮 pending 的最新值合并进来。
	// 每个账本都拿"它取得结论那一刻"的 gen 值比对，既保住 stale-only 省查询，
	// 又不会因重试轮快照缺项把"gen>0 且未在本轮重验"的账本误判为 stale（震荡根因）。
	snap := genSnapshot{perBook: make(map[int64]uint64, len(ids))}
	pending := ids // 本轮需要鉴权的账本；首轮是整帧，重试只含 generation 变过的
	var lastErr error
	for attempt := 0; attempt < maxSubscribeAttempts; attempt++ {
		if attempt > 0 {
			// SYNC-03：重试也要计费。整帧只扣 1 个令牌、重试却整帧重查的时代，
			// 单帧查询量 = 64 × 4 且几乎免费；现在每次重试都消耗一个令牌，
			// 放大被令牌桶的速率硬性兜住。
			if !h.consumeSubToken(uid) {
				return ErrSubscribeRateLimited
			}
		}
		gs := h.generationSnapshot(pending)
		for k, v := range gs.perBook {
			snap.perBook[k] = v
		}
		snap.overflow = gs.overflow
		// 锁外查库校验（每个 bookID 独立判定，失败/非成员一律不订阅）
		for _, b := range pending {
			canView, cerr := checker(uid, b)
			if cerr != nil {
				// 非成员 / 查询失败：静默跳过，不回显"账本是否存在"
				// 注意：先清掉上一轮可能留下的旧结论，防止过期结论残留（R50）
				delete(allowed, b)
				continue
			}
			allowed[b] = canView
		}
		settled, stale, werr := h.writeSubscribe(c, ids, allowed, snap, uid)
		if settled {
			return werr
		}
		// 未落定 = 期间发生了权限变更：只重鉴权变过的账本，未变化账本的结论继续有效
		for _, b := range stale {
			delete(allowed, b) // 旧结论作废：重鉴权若发现已非成员，不得残留旧值
		}
		pending = stale
		lastErr = werr
	}
	// 重试次数耗尽：返回明确错误（客户端随后重试即可），绝不写回过期结论
	if lastErr != nil {
		return fmt.Errorf("%w：已重试 %d 次", ErrSubscribeGenerationStale, maxSubscribeAttempts)
	}
	return ErrSubscribeGenerationStale
}

// checkSubscriptionQuota 在任何查库【之前】校验"这一帧放不放得下"（SYNC-01）。
//
// 旧实现顺序是"逐账本查库鉴权 → 进锁查配额"：被配额拒绝的帧照样烧掉
// MaxBookIDsPerFrame 次 SQLite 查询（MaxOpenConns=1，等于白占唯一的连接）。
// 现在配额前置，被拒帧零查库；写入时（writeSubscribe）保留同样的校验作为
// 并发兜底 —— 预检到写入之间配额状态可能被其它连接改变。
//
// 全程锁顺序 Hub.mu → Client.mu；不做任何 IO。
func (h *Hub) checkSubscriptionQuota(c *Client, uid int64, ids []int64) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.clients[c] {
		return ErrSubscribeClientGone
	}
	// 用户维度总量（持 Hub.mu，逐个取 Client.mu；必须在锁 c.mu 之前取，
	// 避免在持有 c.mu 时访问 Hub——反向锁序会与 Subscribe/Broadcast 死锁）
	userTotal := h.userSubscriptionCountLocked(uid)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.ensureLocked()
	fresh := 0
	for _, b := range ids {
		if !c.BookIds[b] {
			fresh++
		}
	}
	if len(c.BookIds)+fresh > MaxSubscriptionsPerClient {
		return fmt.Errorf("%w：已有 %d + 本帧新增 %d > %d",
			ErrSubscriptionLimitPerConnection, len(c.BookIds), fresh, MaxSubscriptionsPerClient)
	}
	if userTotal+fresh > MaxSubscriptionsPerUser {
		return fmt.Errorf("%w：该用户已有 %d + 本帧新增 %d > %d",
			ErrSubscriptionLimitPerUser, userTotal, fresh, MaxSubscriptionsPerUser)
	}
	return nil
}

// writeSubscribe 把一次（锁外）鉴权结论写入订阅表。
//
// 返回 (是否落定, generation 变过的账本, 错误)：
//   - (true, nil, nil)                       写入成功
//   - (true, nil, ErrSubscribeClientGone)    连接已注销：结论确定，不必重试
//   - (true, nil, ErrSubscriptionLimit*)     订阅数超限：结论确定，不必重试
//   - (false, stale, errStaleGeneration)     期间权限变更：本次结论作废，
//     调用方只需重新鉴权 stale 中的账本（SYNC-03）
//
// 全程锁顺序 Hub.mu → Client.mu；在持有 c.mu 期间不触碰 Hub。
func (h *Hub) writeSubscribe(c *Client, ids []int64, allowed map[int64]bool,
	snap genSnapshot, uid int64) (bool, []int64, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.clients[c] {
		// 连接已注销（读循环刚结束）：不再写入，避免给已回收的连接挂状态
		return true, nil, ErrSubscribeClientGone
	}
	// SYNC-07：gen 表触顶后新账本的事件登记不进表（只推进溢出序号）——
	// 溢出序号变了就整帧作废（fail-closed，代价是整帧重新鉴权一次）。
	if snap.overflow != h.genOverflow {
		return false, append([]int64(nil), ids...), errStaleGeneration
	}
	// 写入前比对权限 generation：任一账本期间发生过成员/可见性变更 → 该账本结论作废
	var stale []int64
	for _, b := range ids {
		if h.gen[b] != snap.perBook[b] {
			stale = append(stale, b)
		}
	}
	if len(stale) > 0 {
		return false, stale, errStaleGeneration
	}
	// 订阅总数上限（并发兜底，预检见 checkSubscriptionQuota）：
	// 先算用户维度（持 Hub.mu，逐个取 Client.mu），再落本连接的写。
	// 必须在锁 c.mu **之前**取，避免在持有 c.mu 时访问 Hub（反向锁序）。
	userTotal := h.userSubscriptionCountLocked(uid)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.ensureLocked()
	fresh := 0
	for b := range allowed {
		if !c.BookIds[b] {
			fresh++
		}
	}
	if len(c.BookIds)+fresh > MaxSubscriptionsPerClient {
		return true, nil, fmt.Errorf("%w：已有 %d + 新增 %d > %d",
			ErrSubscriptionLimitPerConnection, len(c.BookIds), fresh, MaxSubscriptionsPerClient)
	}
	if userTotal+fresh > MaxSubscriptionsPerUser {
		return true, nil, fmt.Errorf("%w：该用户已有 %d + 新增 %d > %d",
			ErrSubscriptionLimitPerUser, userTotal, fresh, MaxSubscriptionsPerUser)
	}
	for b, canView := range allowed {
		c.BookIds[b] = true
		c.canView[b] = canView
		// 自增写入序号：让"快照更早"的 RefreshVisibility 回写时能发现本次更新并放弃
		c.visSeq[b]++
	}
	c.UID = uid
	return true, nil, nil
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

// DeliverToUser 向指定 uid 的**全部在线连接**定向投递一条消息，**不经订阅过滤**。
//
// 用途：成员被移除时的清理通知（`member_removed`，GPT-R24）。
// 该时刻目标已不再是账本成员，其订阅在 RefreshVisibility 中已被摘除；若仍走
// BroadcastToBook，会被 `!sub` 过滤掉，目标将永远收不到那条本该触发"清除该账本本地数据"
// 的事件。因此清理通知必须走一条**不依赖订阅状态**的投递路径。
//
// 锁顺序与 BroadcastToBook 一致（Hub.mu 读 → Client.mu），不引入反向路径。
// 非阻塞投递：Send 满则丢弃并累加 Dropped（与普通广播同策略——WS 只是尽力通知，
// 客户端还有"重连必 pull / 周期 pull"兜底）。
// 返回实际入队的连接数：0 通常表示目标当前离线（调用方可据此记日志）。
func (h *Hub) DeliverToUser(uid int64, msg []byte) int {
	if uid <= 0 || msg == nil {
		return 0
	}
	delivered := 0
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		c.mu.Lock()
		c.ensureLocked()
		match := c.UID == uid
		send := c.Send
		c.mu.Unlock()
		if !match || send == nil {
			continue
		}
		select {
		case send <- msg:
			delivered++
		default:
			c.mu.Lock()
			c.Dropped++
			c.mu.Unlock()
		}
	}
	return delivered
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
// GPT-R50：第 1 段除了快照，还必须递增该账本的权限 generation，而且
// **即使 targets 为空也不能跳过**。理由见下方第 1 段注释。
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

	// 1) 递增权限 generation + 快照（短写锁，不做 IO）
	//
	// GPT-R50 的关键：**即使 targets 为空也必须递增 generation**。
	// 此刻可能存在一个"已通过鉴权、但还没写入 BookIds"的 pending Subscribe——
	// 它不在任何订阅表里，快照看不见它；若这里因为"没人订阅"就提前返回，
	// 它随后会把旧的 allow=true 写回去，刚刚完成的撤权就被抵消了。
	// generation 递增即让它的旧结论在写入前的比对中失败并重新鉴权。
	h.mu.Lock()
	h.bumpGenerationLocked(bookID)
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
	h.mu.Unlock()
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
