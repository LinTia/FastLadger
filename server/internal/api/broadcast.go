package api

// WS 广播接线（**只在 api 适配层**，service 不导入 ws，避免 service ↔ ws 循环依赖）。
//
// 硬约束（指南 §14.2 与 PRD §12.1 决策 2）：
//  1. 广播发生在业务**提交之后**；
//  2. 广播失败**不得影响已提交的业务结果**——只记日志，绝不把成功改成失败或重做；
//  3. 事件本身必须**脱敏**：只含 event / bookId / version 与极少量标识（如 targetUid），
//     不含金额、账户、备注；真正的数据由客户端走授权 pull 获取。
//
// 事件契约（《落地方案》§5.2 权威表）：
//
//	| event               | 类型 | forceFullResync | data                     |
//	|---------------------|------|-----------------|--------------------------|
//	| data_changed        | 信号 | false           | {}                       |
//	| visibility_changed  | 协调 | true            | {}                       |
//	| member_removed      | 协调 | true            | {"targetUid":9}          |
//	| member_role_changed | 协调 | true            | {"targetUid":9,"role":2} |
//	| book_deleted        | 协调 | true            | {}                       |
//
// 命名规则（一条可执行规则，取代"点号/下划线混用"）：
//   - 事件名（枚举值）**一律下划线**；
//   - JSON 字段名**一律 camelCase**（bookId / version / targetUid / forceFullResync）。
//
// 为什么收敛成 `data_changed` 单一信号事件：WS 本来只该表达"有新版本，去 pull"。
// 交易/转账/账户/分类/持仓（未来）等**任何使版本递增**的写，客户端要做的事完全相同
// （触发一次去抖 pull），因此不需要为每种资源各设一个事件名——事件名越多，
// 客户端就越可能漏实现其中一个，而漏掉的后果是"实时刷新偶尔失灵"这种难查的问题。
//
// `data_changed` 的 data **刻意为空**：不含金额、账户、备注。明细一律走授权 pull，
// 关闭互看的 member 也能安全收到本事件（只泄露"有更新"，不泄露"更新了什么"）。
// 不要因为"顺便带上更省一次请求"而把明细塞进 data —— 那会绕过可见性过滤。
import (
	"encoding/json"
	"log"

	"fastledger/internal/service"
	"fastledger/internal/ws"
)

// 事件名（枚举值一律下划线；见文件头命名规则）
const (
	// eventDataChanged 信号事件：任何使版本递增的写（交易/转账/账户/分类/持仓）
	eventDataChanged = "data_changed"
	// eventVisibilityChanged 协调事件：可见性开关实际变化
	eventVisibilityChanged = "visibility_changed"
	// eventMemberRemoved 协调事件：成员被移除
	eventMemberRemoved = "member_removed"
	// eventMemberRoleChanged 协调事件：成员角色变化
	eventMemberRoleChanged = "member_role_changed"
	// eventBookDeleted 协调事件：账本被删除
	eventBookDeleted = "book_deleted"
)

// bookEventEnvelope 统一事件信封（字段顺序固定，便于抓包比对与客户端快照测试）。
//
// 信封固定五个字段，`forceFullResync` **始终出现**（信号事件为 false）：
// 客户端可以无条件读取该字段，不必区分"字段缺失"与"false"。
type bookEventEnvelope struct {
	Event  string `json:"event"`
	BookId int64  `json:"bookId"`
	// Version 事件发生后的账本版本水位（协调事件不递增，只回报当前值）
	Version int64 `json:"version"`
	// ForceFullResync 仅协调事件为 true：客户端须 `afterVersion=0` 定向全量重拉该账本
	ForceFullResync bool `json:"forceFullResync"`
	// Data 业务字段的**唯一**承载位（信号事件刻意为 {}，详见文件头）
	Data map[string]interface{} `json:"data"`
}

// broadcastBookEvent 向订阅该账本的在线成员推一条脱敏事件。
//
// 出错只记日志：WS 只是"尽力通知"，客户端有周期性 pull 与重连必 pull 兜底（指南 §14.1 约定 5）。
func broadcastBookEvent(bookID int64, env bookEventEnvelope) {
	if bookID <= 0 {
		return
	}
	env.BookId = bookID
	if env.Data == nil {
		env.Data = map[string]interface{}{}
	}
	msg, err := json.Marshal(env)
	if err != nil {
		log.Printf("ws 广播序列化失败：book_id=%d event=%s err=%v", bookID, env.Event, err)
		return
	}
	// 事件已脱敏：完整载荷与脱敏载荷相同
	ws.DefaultHub.BroadcastToBook(bookID, msg, msg)
}

// currentBookVersion 读取当前版本水位；失败只记日志并返回 0
// （信封里 version=0 表示"服务端没能给出水位"，客户端仍会按 forceFullResync 全量重拉）。
func currentBookVersion(books *service.BookService, bookID int64) int64 {
	if books == nil {
		return 0
	}
	v, err := books.CurrentVersion(bookID)
	if err != nil {
		log.Printf("读取账本版本失败（WS 事件将带 version=0）：book_id=%d err=%v", bookID, err)
		return 0
	}
	return v
}

// notifyDataChanged 有数据变更（版本已递增）后通知"有新版本，去 pull"。
//
// version 必须传**本次写入产生的版本**（`TxService.Create` 的返回值、账户/分类写接口返回的
// `syncVersion`），而不是事后读的水位——并发写时两者可能不同。
//
// 不带任何明细：客户端收到后触发一次（去抖）pull，且**不直接推进已同步游标**（指南 §22 R07）。
func notifyDataChanged(bookID, version int64) {
	broadcastBookEvent(bookID, bookEventEnvelope{
		Event:           eventDataChanged,
		Version:         version,
		ForceFullResync: false,
	})
}

// notifyVisibilityChanged 可见性开关变更后：先重算在线连接的 canView 缓存，再广播。
//
// 顺序很重要：先刷新缓存能保证**同类事件的后续广播**立刻按新规则选择载荷，
// 而本事件本身要求客户端 afterVersion=0 定向全量重拉（PRD §12.1 决策 3/4）。
func notifyVisibilityChanged(bookID, version int64) {
	ws.DefaultHub.RefreshVisibility(bookID)
	broadcastBookEvent(bookID, bookEventEnvelope{
		Event:           eventVisibilityChanged,
		Version:         version,
		ForceFullResync: true,
	})
}

// notifyMemberRemoved 成员被移除：客户端应主动清除该账本本地数据并停止订阅（决策 5）。
//
// RefreshVisibility 会顺带把已非成员的连接从订阅表里摘掉（见 internal/ws/hub.go），
// 否则被移除者仍会收到该账本的 `data_changed`——那等于持续泄露"账本仍活跃"。
func notifyMemberRemoved(bookID, version, targetUid int64) {
	ws.DefaultHub.RefreshVisibility(bookID)
	broadcastBookEvent(bookID, bookEventEnvelope{
		Event:           eventMemberRemoved,
		Version:         version,
		ForceFullResync: true,
		Data:            map[string]interface{}{"targetUid": targetUid},
	})
}

// notifyMemberRoleChanged 成员角色变化：可见范围可能变化，要求定向全量重拉
func notifyMemberRoleChanged(bookID, version, targetUid int64, role int8) {
	ws.DefaultHub.RefreshVisibility(bookID)
	broadcastBookEvent(bookID, bookEventEnvelope{
		Event:           eventMemberRoleChanged,
		Version:         version,
		ForceFullResync: true,
		Data:            map[string]interface{}{"targetUid": targetUid, "role": role},
	})
}

// notifyBookDeleted 账本被删除：客户端应清空该账本本地数据并停止订阅
func notifyBookDeleted(bookID, version int64) {
	ws.DefaultHub.RefreshVisibility(bookID)
	broadcastBookEvent(bookID, bookEventEnvelope{
		Event:           eventBookDeleted,
		Version:         version,
		ForceFullResync: true,
	})
}
