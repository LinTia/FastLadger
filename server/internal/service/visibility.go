package service

// 可见性判定（PRD §7.3「可见性规则」+ §12.1 决策「权限变化与本地缓存重建」）。
//
// 四条落地口径（逐条对应文档，不得自创）：
//  1. 【服务端权威】判定只在服务端执行，客户端不自行过滤（PRD §12.1 决策 1）。
//  2. 【粒度】MVP 只启用【账本级】开关 `books.members_can_view_others`；
//     `book_members.can_view_others` 是"成员级覆盖"的预留字段，MVP **不启用**（PRD §7.3 粒度条款）。
//  3. 【角色】owner / admin 永远能看全部明细；member 只受账本级开关约束。
//  4. 【归属哨兵】`transactions.creator_uid = 0` 表示"阶段 2 之前写入的未知归属"
//     （《开发WBS》阶段 3/4 的归属哨兵约束）。**0 绝不能被当成"放行"条件**：
//     `txVisibilityCond` 里同时写 `creator_uid = ?`（uid 恒 > 0，天然排除 0）与
//     `creator_uid <> 0`（防回归的双保险）。任何人把条件改成
//     `creator_uid IN (?, 0)` 这类"放行"写法，都会被这两条对撞出来。
//
// 隐私边界（PRD §7.3 / §12.1 决策 6）：可见性变更仅对**未来同步**生效，
// 成员此前已查看或已导出的数据服务端无法收回；该话术须常驻显示于可见性开关旁（前端职责）。
import (
	"fastledger/internal/model"

	"xorm.io/xorm"
)

// CanViewDetail 当前用户能否看到该账本【他人】的明细。
//
// 返回值语义：
//   - 非成员 → (false, ErrNotMember)：**不区分"账本不存在"与"不是成员"**，
//     避免通过错误码差异探测账本是否存在。
//   - 账本已软删 → (false, nil)：账本级不可见，但不报错（由调用方的成员校验负责 403）。
//   - owner / admin → (true, nil)：权限矩阵《落地方案》§5.5 固定的高权限。
//   - member → 读账本级开关 books.members_can_view_others（MVP 唯一粒度）。
//
// 广播路径（ws.Hub）不允许在 Hub 锁内查库，故它只缓存本函数的结果（见 internal/ws/hub.go）。
func CanViewDetail(engine *xorm.Engine, uid, bookID int64) (bool, error) {
	if engine == nil {
		// 防御性 fail-closed：没有引擎就一律按"看不到"处理，绝不因配置缺失而放行。
		return false, ErrNotMember
	}
	sess := engine.NewSession()
	defer sess.Close()
	return canViewDetailTx(sess, uid, bookID)
}

// canViewDetailTx 在【调用方已有的会话/事务】内做同一判定，供 Pull 的一致快照复用。
func canViewDetailTx(sess *xorm.Session, uid, bookID int64) (bool, error) {
	// 显式列出所需三列，不依赖 SELECT *（判定只关心角色、账本开关、账本是否已删）
	var row struct {
		Role                 int8 `xorm:"role"`
		MembersCanViewOthers bool `xorm:"members_can_view_others"`
		BookDeleted          bool `xorm:"book_deleted"`
	}
	has, err := sess.SQL(
		`SELECT m.role AS role,
		        b.members_can_view_others AS members_can_view_others,
		        b.deleted AS book_deleted
		 FROM book_members m INNER JOIN books b ON b.id = m.book_id
		 WHERE m.book_id = ? AND m.uid = ? AND m.deleted = 0`, bookID, uid).Get(&row)
	if err != nil {
		return false, err
	}
	if !has {
		return false, ErrNotMember
	}
	if row.BookDeleted {
		return false, nil
	}
	if row.Role == model.RoleOwner || row.Role == model.RoleAdmin {
		return true, nil
	}
	// member：账本级开关是 MVP 唯一粒度（book_members.can_view_others 不参与判定）
	return row.MembersCanViewOthers, nil
}

// canViewAllForTx 只读路径的合并助手：先复核成员资格（非成员 403），再取可见性。
//
// 两次查询是刻意的：成员资格与可见性是**两个不同的判定**，合并成一条 SQL 会让
// "账本已删"与"关闭互看"两种情况混在同一个错误里，读路径无法分辨该回 403 还是 404。
func canViewAllForTx(engine *xorm.Engine, uid, bookID int64) (bool, error) {
	if err := requireMember(engine, bookID, uid); err != nil {
		return false, err
	}
	return CanViewDetail(engine, uid, bookID)
}

// txVisibilityCond 交易表的可见性 SQL 片段（供 List / Get / SyncService.Pull 共用）。
//
// canViewAll=true  → 无附加条件（"" , nil），owner/admin/开关打开时的 member 都走这里；
// canViewAll=false → 只返回本人记录，并显式排除归属未知的 0 号哨兵。
//
// 为什么是 `AND creator_uid <> 0`：creator_uid=0 是"阶段 2 之前写入的未知归属"，
// 不属于任何人，因此不能因为"恰好等于某个比较值"而被放行。uid 恒 > 0 时
// `creator_uid = ?` 已天然排除 0，本条件作为**双保险**，防止有人把它改成
// `creator_uid IN (?, 0)` 之类的"放行"写法而越权。
func txVisibilityCond(canViewAll bool, uid int64) (string, []interface{}) {
	if canViewAll {
		return "", nil
	}
	return "creator_uid = ? AND creator_uid <> 0", []interface{}{uid}
}
