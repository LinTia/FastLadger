package service

// 成员 / 角色 / 权限 / 邀请入户。
//
// 权限矩阵以《落地方案》§5.5【修订版】为准（[1.9] 裁决，对齐 PRD §12 决策#2）：
//
//	操作                owner  admin  member
//	改账本设置/删除账本   ✅     ❌     ❌
//	邀请/移除成员、改角色  ✅     ✅*    ❌      *admin 不能动 owner
//	增删改自己的交易      ✅     ✅     ✅
//	改/删他人交易        ✅     ✅     ❌      ← admin 由"可配置"改为 ✅
//	管理标的/持仓/资产    ✅     ✅     自己的持仓
//
// 所有判定在服务端执行，不信任客户端上报的角色或 book_id。
import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"fastledger/internal/model"

	"golang.org/x/crypto/bcrypt"
	"xorm.io/xorm"
)

// maxBookMembers 账本成员上限（PRD §12 决策 6：MVP 上限 20 人，家庭/小圈子足够）。
//
// 注意：统计口径是 `book_members.deleted = 0`（软删退出的成员不占名额，
// 但其历史记录保留，满足"不可篡改审计"）。
const maxBookMembers = 20

var (
	// ErrInviteInvalid 邀请码无效（不存在/已用/已撤销/过期）
	ErrInviteInvalid = errors.New("邀请码无效或已过期")
	// ErrInviteBoundMismatch 邀请码绑定的用户名/邮箱不匹配
	ErrInviteBoundMismatch = errors.New("邀请码已绑定指定用户，信息不匹配")
	// ErrCannotOperateOwner 不能操作 owner
	ErrCannotOperateOwner = errors.New("不能对账本所有者执行该操作")
	// ErrRateLimited 触发限流
	ErrRateLimited = errors.New("操作过于频繁，请稍后再试")
	// ErrBookFull 账本成员已达上限（PRD §12 决策 6，HTTP 409）
	ErrBookFull = errors.New("该账本成员已达上限")
)

// ErrInviteBindingConflict 同时绑定邮箱与用户名（GPT-R41：二选一，不允许静默取舍）
var ErrInviteBindingConflict = fmt.Errorf("%w：邀请码只能绑定邮箱或用户名其中之一", ErrInvalidArgument)

// ErrInviteEmailUnavailable 邮箱无法用于本次入户（已被其他账号占用）。
//
// 刻意并入 ErrInviteBoundMismatch 语义（HTTP 400、对外提示"信息不匹配"）：
// 兑换入口是**未认证**的，明确回答"该邮箱已注册"等于开放邮箱枚举——
// 与登录入口不区分"用户名不存在"与"口令错误"（ErrBadCredentials）同理。
var ErrInviteEmailUnavailable = fmt.Errorf("%w：邮箱已被其他账号占用", ErrInviteBoundMismatch)

// 邀请事件的粗粒度原因码（GPT-R40 起统一用于失败分支；GPT-R42 复用它们做审计 reason）。
//
// 取值刻意【粗】：只回答"属于哪一类失败"，绝不携带可以用来缩小猜测空间的细节
// （不写"绑定的邮箱是 alice@…"、不写尝试过的用户名）。
const (
	// inviteReasonNone 无失败原因（成功事件用）
	inviteReasonNone = ""
	// inviteReasonNotFound 邀请不存在（含不属于该账本）
	inviteReasonNotFound = "invite_not_found"
	// inviteReasonAlreadyUsed 邀请已被使用
	inviteReasonAlreadyUsed = "invite_already_used"
	// inviteReasonRevoked 邀请已被撤销
	inviteReasonRevoked = "invite_revoked"
	// inviteReasonExpired 邀请已过期（判定时刻 expires_at <= now）
	inviteReasonExpired = "invite_expired"
	// inviteReasonBoundMismatch 与绑定的用户名/邮箱不匹配（GPT-R41 闭环的失败态）
	inviteReasonBoundMismatch = "bound_mismatch"
	// inviteReasonEmailTaken 邮箱已被其他账号占用（对外并入 bound_mismatch 文案）
	inviteReasonEmailTaken = "email_taken"
	// inviteReasonBookDeleted 账本已被删除（GPT-R43）
	inviteReasonBookDeleted = "book_deleted"
)

// GPT-R40：邀请撤销的失败语义，全部**继承**既有错误树（用 %w 包装），
// 因此不需要改动 mapSvcErr，也不会新开一套 HTTP 状态码契约。
var (
	// ErrInviteAlreadyUsed 邀请已被使用（不可再撤销）
	ErrInviteAlreadyUsed = fmt.Errorf("%w：邀请码已被使用", ErrInviteInvalid)
	// ErrInviteAlreadyRevoked 邀请已被撤销（重复撤销不会被当成成功）
	ErrInviteAlreadyRevoked = fmt.Errorf("%w：邀请码已被撤销", ErrInviteInvalid)
	// ErrInviteExpired 邀请已过期（过期判定见 inviteRejectReason，全局只有这一个口径）
	ErrInviteExpired = fmt.Errorf("%w：邀请码已过期", ErrInviteInvalid)
	// ErrInviteNotFound 邀请不存在（含"存在但不属于该账本"——两者对外不可区分，
	// 否则等于允许攻击者用 A 账本的凭证去枚举 B 账本的邀请主键）
	ErrInviteNotFound = fmt.Errorf("%w：邀请码不存在", ErrInviteInvalid)
	// ErrInviteBookUnavailable 目标账本不存在或已被删除（GPT-R43）
	ErrInviteBookUnavailable = fmt.Errorf("%w：账本不可用", ErrInviteInvalid)
)

// RoleOf 查询成员角色；非成员返回 ErrNotMember（服务端权威判定）
func RoleOf(engine *xorm.Engine, bookID, uid int64) (int8, error) {
	var m model.BookMember
	has, err := engine.Where("book_id = ? AND uid = ? AND deleted = 0", bookID, uid).Get(&m)
	if err != nil {
		return 0, err
	}
	if !has {
		return 0, ErrNotMember
	}
	return m.Role, nil
}

// RoleOf 成员角色（服务端权威判定，非成员返回 ErrNotMember）
func (s *MemberService) RoleOf(bookID, uid int64) (int8, error) {
	return RoleOf(s.engine, bookID, uid)
}

// CanManageMembers owner 或 admin 可邀请/移除成员与改角色
func CanManageMembers(role int8) bool {
	return role == model.RoleOwner || role == model.RoleAdmin
}

// CanEditOthersTx owner 与 admin 可编辑/删除他人记录；member 仅本人
func CanEditOthersTx(role int8) bool {
	return role == model.RoleOwner || role == model.RoleAdmin
}

// MemberDTO 成员列表项
type MemberDTO struct {
	Uid             int64  `json:"uid"`
	Username        string `json:"username"`
	Role            int8   `json:"role"`
	CanViewOthers   bool   `json:"canViewOthers"`
	IsInstanceAdmin bool   `json:"isInstanceAdmin"`
}

// MemberService 成员服务
type MemberService struct {
	engine       *xorm.Engine
	inviteTTLSec int64
	auth         *AuthService
	limiter      *Limiter
}

// NewMemberService 构造成员服务
//
// GPT-R27：邀请有效期天数在此【一次性校验】。原先是 `int64(days)*86400` 裸乘，
// 天数极大时会溢出成负数，表现为"邀请码刚发出就已过期"，且无任何报错。
// 现在非法配置直接构造失败（服务启动不起来），不会带着错误的 TTL 运行。
func NewMemberService(engine *xorm.Engine, auth *AuthService, inviteTTLDays int) (*MemberService, error) {
	ttlSec, err := daysToSeconds(inviteTTLDays)
	if err != nil {
		return nil, fmt.Errorf("邀请有效期配置非法：%w", err)
	}
	return &MemberService{
		engine:       engine,
		auth:         auth,
		inviteTTLSec: ttlSec,
		limiter:      NewLimiter(),
	}, nil
}

// List 账本成员列表（调用方需已校验成员资格）
//
// GPT-R43：账本软删之后不再允许任何成员管理动作。列表虽然只读，
// 但它是客户端判断"这个账本还有谁"的依据——继续返回等于让已删账本保持"还活着"的表象，
// 也让被清除数据的客户端又有理由重新拉一遍。
func (s *MemberService) List(bookID int64) ([]MemberDTO, error) {
	sess := s.engine.NewSession()
	defer sess.Close()
	if err := requireActiveBookTx(sess, bookID); err != nil {
		return nil, err
	}
	var out []MemberDTO
	err := sess.SQL(
		`SELECT m.uid, u.username, m.role, m.can_view_others AS can_view_others,
		        u.is_instance_admin AS is_instance_admin
		 FROM book_members m INNER JOIN users u ON u.id = m.uid
		 WHERE m.book_id = ? AND m.deleted = 0 AND u.deleted = 0
		 ORDER BY m.role ASC, m.id ASC`, bookID).Find(&out)
	return out, err
}

// requireActiveBookTx 账本必须仍然存在（books.deleted = 0）。
//
// 为什么不能只看 book_members.deleted：成员关系是账本的【子对象】，
// 账本被删了之后残留的成员行不能反过来证明"这个账本还能操作"——
// 那会让已删账本继续签发邀请、继续接收成员（GPT-R43）。
//
// 写事务路径另有 roleOfTx / requireMemberTx，它们的 SQL 本身就 JOIN 了
// `books b ON b.id = m.book_id AND b.deleted = 0`（单一口径），
// 因此 ChangeRole / Remove / RevokeInvite 天然满足本约束；
// 本函数服务的是那些【不经过成员资格判定】的入口（List、JoinViaInvite），
// 以及需要把"账本没了"独立于权限错误表达出来的入口（CreateInvite）。
func requireActiveBookTx(sess *xorm.Session, bookID int64) error {
	// books.deleted 声明为 BOOLEAN，驱动会返回 bool；【不要】把它扫进 int64
	// 再在 Go 侧判等——那会直接 Scan 失败（真实 SQLite 上已复现，见 CI）。
	// 刻意把条件写进 SQL、用 COUNT 取整数，避开对列类型的依赖。
	var n int64
	has, err := sess.SQL("SELECT COUNT(1) FROM books WHERE id = ? AND deleted = 0", bookID).Get(&n)
	if err != nil {
		return err
	}
	if !has || n != 1 {
		return fmt.Errorf("%w：账本不存在或已被删除", ErrEntityDeleted)
	}
	return nil
}

// ChangeRole 修改成员角色：owner/admin 可操作，但 admin 不能改 owner，也不能把他人提为 owner
func (s *MemberService) ChangeRole(actor, bookID, targetUid int64, newRole int8) error {
	if newRole != model.RoleAdmin && newRole != model.RoleMember {
		// 归入 ErrInvalidArgument：由 mapSvcErr 映射为 400，而不是 500
		return fmt.Errorf("%w：只能将成员设为管理员或普通成员", ErrInvalidArgument)
	}
	// 权限复核 / 数据变更 / 审计必须在【同一事务】内完成（WBS 4.3 / 4.6）。
	//
	// GPT-R28：actor 的角色必须在事务内重读。事务外读角色会留下 TOCTOU 窗口——
	// 请求通过检查后被并发降权或移除，却仍能用过期权限继续改别人的角色。
	// GPT-R29：before_json 用的旧角色同样在事务内读，保证"变更前状态"与实际变更对应。
	sess := s.engine.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	defer func() { _ = sess.Rollback() }()

	actorRole, err := roleOfTx(sess, bookID, actor)
	if err != nil {
		return err
	}
	if !CanManageMembers(actorRole) {
		return ErrForbidden
	}
	targetRole, err := roleOfTx(sess, bookID, targetUid)
	if err != nil {
		return err
	}
	if targetRole == model.RoleOwner {
		return ErrCannotOperateOwner
	}
	if actorRole == model.RoleAdmin && targetRole == model.RoleOwner {
		return ErrCannotOperateOwner
	}

	n, err := sess.Where("book_id = ? AND uid = ? AND deleted = 0", bookID, targetUid).
		Cols("role", "updated_at").
		Update(&model.BookMember{Role: newRole, UpdatedAt: time.Now().Unix()})
	if err != nil {
		return err
	}
	if n != 1 {
		// 目标在「读角色」与「写入」之间被并发移除：写条件 deleted=0 生效导致 0 行。
		// 不能 return nil 冒充成功（D-R09）——映射为 409，让调用方刷新后重试。
		return fmt.Errorf("%w：目标成员已不存在或已被移除", ErrConcurrentConflict)
	}
	// before_json 记录变更前的角色：事后要能回答"这个人原来是什么权限"
	if err := insertAudit(sess, bookID, actor, model.AuditActionPermissionChange,
		model.AuditEntityMember, targetUid,
		map[string]interface{}{"uid": targetUid, "role": targetRole}, "server"); err != nil {
		return err
	}
	if err := sess.Commit(); err != nil {
		return err
	}
	return nil
}

// Remove 移除成员（软删，保留历史记录）
func (s *MemberService) Remove(actor, bookID, targetUid int64) error {
	// 与 ChangeRole 同理：权限复核 / 移除 / 审计在同一事务（WBS 4.3 / 4.6；GPT-R28 / R29）
	sess := s.engine.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	defer func() { _ = sess.Rollback() }()

	actorRole, err := roleOfTx(sess, bookID, actor)
	if err != nil {
		return err
	}
	if !CanManageMembers(actorRole) {
		return ErrForbidden
	}
	targetRole, err := roleOfTx(sess, bookID, targetUid)
	if err != nil {
		return err
	}
	if targetRole == model.RoleOwner {
		return ErrCannotOperateOwner
	}

	now := time.Now().Unix()
	n, err := sess.Where("book_id = ? AND uid = ? AND deleted = 0", bookID, targetUid).
		Cols("deleted", "deleted_at", "updated_at").
		Update(&model.BookMember{Deleted: true, DeletedAt: now, UpdatedAt: now})
	if err != nil {
		return err
	}
	if n != 1 {
		// 与 ChangeRole 同理（D-R09）：目标已被并发移除时 0 行，不能当成功返回。
		return fmt.Errorf("%w：目标成员已不存在或已被移除", ErrConcurrentConflict)
	}
	// before_json 保留被移除前的角色：事后要能回答"移除的是谁、当时什么权限"
	if err := insertAudit(sess, bookID, actor, model.AuditActionPermissionChange,
		model.AuditEntityMember, targetUid,
		map[string]interface{}{"uid": targetUid, "role": targetRole}, "server"); err != nil {
		return err
	}
	if err := sess.Commit(); err != nil {
		return err
	}
	return nil
}

// CreateInvite 生成邀请码（限流：每账本每分钟 ≤5）
//
// GPT-R28：actor 的权限必须在【写事务内】重读。邀请码是入户凭证，若拿一个事务外
// 读到的过期角色签发，已被降权/移除的管理员仍能继续拉人进账本。
//
// GPT-R41：绑定维度在【消耗配额之前】先规范化并校验（见 resolveInviteBinding）——
// 非法邮箱不该占用账本的邀请配额，否则连续几次格式错误就能把签发入口堵死一分钟。
func (s *MemberService) CreateInvite(actor, bookID int64, role int8, boundUsername, boundEmail string) (*model.Invite, error) {
	if role != model.RoleAdmin && role != model.RoleMember {
		role = model.RoleMember
	}
	boundUser, boundMail, err := resolveInviteBinding(boundUsername, boundEmail)
	if err != nil {
		return nil, err
	}
	if !s.limiter.Allow(fmt.Sprintf("invite:create:book:%d", bookID), 5, time.Minute) {
		return nil, ErrRateLimited
	}

	sess := s.engine.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return nil, err
	}
	defer func() { _ = sess.Rollback() }()

	actorRole, err := roleOfTx(sess, bookID, actor)
	if err != nil {
		return nil, err
	}
	if !CanManageMembers(actorRole) {
		return nil, ErrForbidden
	}
	// GPT-R43：账本必须仍然有效。roleOfTx 的 JOIN 其实已经保证了这一点，
	// 这里再显式判一次不是为了重复兜底，而是为了让「账本已删」
	// 与"没有权限"成为两种可分辨的结果——排查时这两件事的含义完全不同。
	if err := requireActiveBookTx(sess, bookID); err != nil {
		return nil, err
	}

	code, err := randomCode(32)
	if err != nil {
		return nil, err
	}
	inv := &model.Invite{
		BookId:    bookID,
		Code:      code,
		Role:      role,
		CreatedBy: actor,
		ExpiresAt: time.Now().Unix() + s.inviteTTLSec,
		// GPT-R41：两处绑定字段都存【规范化后的值】，
		// 兑换端只需再做一次同样的规范化即可精确比较，不会再出现
		// "Alice@x.com 签发的邀请 alice@x.com 兑不掉"。
		BoundUsername: boundUser,
		BoundEmail:    boundMail,
		CreatedAt:     time.Now().Unix(),
	}
	if _, err := sess.Insert(inv); err != nil {
		return nil, err
	}
	if err := insertAudit(sess, bookID, actor, inviteAuditAction(inviteEventCreate),
		model.AuditEntityMember, inv.Id,
		inviteAuditPayload(inviteEventCreate, inviteResultAccepted, inviteReasonNone, inviteAuditFields{
			InviteID:    inv.Id,
			Role:        inv.Role,
			BoundTarget: inviteBoundTargetOf(boundUser, boundMail),
			ExpiresAt:   inv.ExpiresAt,
		}), "server"); err != nil {
		return nil, err
	}
	if err := sess.Commit(); err != nil {
		return nil, err
	}
	return inv, nil
}

// RevokeInvite 撤销邀请码（GPT-R40）。
//
// 四条硬约束：
//  1. **权限在写事务内重读**（GPT-R28，沿用 roleOfTx）：撤销决定"谁能入户"，
//     用事务外的旧角色签发会让刚被降权/移除的管理员继续作废别人的邀请。
//     roleOfTx 的 JOIN 同时保证账本仍然有效（books.deleted=0，GPT-R43）。
//  2. **条件 UPDATE + RowsAffected**，而不是"先读后写"：两个并发请求若都先读到
//     "可用"，后一个的写入会把前一个覆盖成同样的值并**误报成功**。
//     这里把状态判定写进 WHERE，命中与否由数据库裁定：RowsAffected==1 才算撤销成功。
//  3. **与兑换互斥**：JoinViaInvite 标记已用的 UPDATE 带 `used_at = 0 AND revoked = 0`，
//     撤销的 UPDATE 带同样两个条件。二者都只认"自己读到的状态仍然成立"，
//     因此同一邀请被同时撤销和兑换时【至多一个】能提交成功。
//  4. **幂等性是显式的**：重复撤销返回 ErrInviteAlreadyRevoked，而不是假装成功——
//     "已经生效"和"这次生效"在审计里必须能区分（GPT-R42）。
//
// 只能撤销"尚未使用、未撤销、未过期"的邀请：过期邀请已经不可兑换，
// 再给它写一次撤销只会产生一行"什么都没改变"的伪事件。
func (s *MemberService) RevokeInvite(actor, bookID, inviteID int64) error {
	sess := s.engine.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	defer func() { _ = sess.Rollback() }()

	actorRole, err := roleOfTx(sess, bookID, actor)
	if err != nil {
		return err
	}
	if !CanManageMembers(actorRole) {
		return ErrForbidden
	}

	// 先把邀请读进来，只为【成功时描述它被作废前长什么样】（role / 绑定维度 / 有效期）。
	//
	// 强调：这次读【不参与任何判定】。判定全部写在下面 UPDATE 的 WHERE 里，
	// 读到的值只用于事后记账——否则就退回成 GPT-R40 要消灭的"先读后写"。
	var inv model.Invite
	hasInvite, err := sess.Where("id = ? AND book_id = ?", inviteID, bookID).Get(&inv)
	if err != nil {
		return err
	}

	now := time.Now().Unix()
	n, err := sess.Where("id = ? AND book_id = ? AND "+inviteUsableCond, inviteID, bookID, now).
		Cols("revoked").Update(&model.Invite{Revoked: true})
	if err != nil {
		return err
	}
	if n != 1 {
		// 未命中：用事务内那份只读快照裁定原因（"无效"太含糊，排查时无从下手）。
		_ = sess.Rollback()
		// GPT-R42：撤销失败同样留痕——"谁在什么时候试图废止一个不可废止的邀请"
		// 对排查误操作与安全观察都有价值。actor 是已认证用户，故不再是哨兵 0。
		return s.inviteFailed(inviteEventRevoke, bookID, actor, inviteID,
			inviteRejectReason(hasInvite, &inv, now), "")
	}

	// GPT-R42：成功撤销与审计【同事务】——成功也是一个需要解释的状态变更。
	if err := insertAudit(sess, bookID, actor, inviteAuditAction(inviteEventRevoke),
		model.AuditEntityMember, inviteID,
		inviteAuditPayload(inviteEventRevoke, inviteResultAccepted, inviteReasonNone, inviteAuditFields{
			InviteID:    inviteID,
			Role:        inv.Role,
			BoundTarget: inviteBoundTargetOf(inv.BoundUsername, inv.BoundEmail),
			ExpiresAt:   inv.ExpiresAt,
		}), "server"); err != nil {
		return err
	}

	if err := sess.Commit(); err != nil {
		return err
	}
	return nil
}

// inviteFailed 失败路径的统一出口：把"业务错误"与"审计结果"一起带回。
//
// 为什么用 errors.Join 而不是二选一：
//   - 只返回审计错误 → 对外把"邀请无效"变成 500，反而泄露了内部情况；
//   - 只返回业务错误 → 审计写失败被彻底吞掉，事后查不到这段窗口发生过什么；
//   - errors.Join 让 errors.Is(err, ErrInviteInvalid) 继续成立（HTTP 映射不变），
//     同时把审计异常留在错误链里，服务日志能看到。这是一次失败被完整地记录，
//     而不是挑一半报上去。
func (s *MemberService) inviteFailed(event string, bookID, actorUID, inviteID int64, reason, clientIP string) error {
	biz := inviteRejectError(reason)
	auditErr := s.appendInviteRejectedAudit(bookID, actorUID, inviteID, event, reason,
		inviteAuditFields{ClientIP: clientIP})
	if auditErr != nil {
		return errors.Join(biz, fmt.Errorf("邀请审计写入失败：%w", auditErr))
	}
	return biz
}

// inviteIDOrZero 邀请不可归因（不存在 / 不属于本账本）时，entity_id 记 0，
// 避免把一次无主的尝试扣到某个不相关的主键上。
func inviteIDOrZero(found bool, id int64) int64 {
	if !found {
		return 0
	}
	return id
}

// inviteUsableCond 邀请"此刻仍然可操作"的 SQL 条件（撤销的 UPDATE 用它做 WHERE 尾巴）。
//
// 与 inviteRejectReason 同为一个过期口径的两面：那边按 `expires_at <= now` 判"已过期"，
// 这边按 `expires_at > now` 判"仍可操作"。把它抽成常量而不是在两处各写一遍，
// 是为了让"改了过期语义"必须同时改这两处——否则就会出现
// "兑换说它过期了、撤销却说它还能撤销"这种自相矛盾的状态机（GPT-R42 策略一）。
const inviteUsableCond = "used_at = 0 AND revoked = 0 AND expires_at > ?"

// inviteRejectReason 按"只读快照"裁定一条邀请此刻不能被操作的原因。
//
// 刻意做成一个函数：**所有路径**（兑换、撤销）都必须走它，避免两处各写一遍
// `used_at > 0 || revoked || expires_at <= now` 然后某一处漏掉一个条件。
func inviteRejectReason(has bool, inv *model.Invite, now int64) string {
	switch {
	case !has:
		return inviteReasonNotFound
	case inv.UsedAt > 0:
		return inviteReasonAlreadyUsed
	case inv.Revoked:
		return inviteReasonRevoked
	case inv.ExpiresAt <= now:
		return inviteReasonExpired
	default:
		return inviteReasonNone
	}
}

// inviteRejectError 把粗粒度原因码翻译成对外错误（全部继承既有错误树）
func inviteRejectError(reason string) error {
	switch reason {
	case inviteReasonNotFound:
		return ErrInviteNotFound
	case inviteReasonAlreadyUsed:
		return ErrInviteAlreadyUsed
	case inviteReasonRevoked:
		return ErrInviteAlreadyRevoked
	case inviteReasonExpired:
		return ErrInviteExpired
	case inviteReasonBoundMismatch:
		return ErrInviteBoundMismatch
	case inviteReasonEmailTaken:
		// 继承 ErrInviteBoundMismatch：对外文案统一，防止邮箱枚举（见 ErrInviteEmailUnavailable）
		return ErrInviteEmailUnavailable
	case inviteReasonBookDeleted:
		// 并入"邀请无效"：兑换入口未认证，不能让它变成账本存在性的探测器
		return ErrInviteBookUnavailable
	default:
		return ErrInviteInvalid
	}
}

// ---- GPT-R42：邀请生命周期审计 ---------------------------------------------
//
// 【策略一：过期不做后台标记，一律"查询时判定"（惰性）】
//
// 本仓没有、也不为此引入守护线程：没有定时器去把过期邀请逐个 UPDATE 成"已过期"，
// 也不写"过期时刻"这种无主体的审计行。理由：
//  1. 审计回答的是"发生了什么"，不是"时间到了"——无人触碰时并没有事件发生，
//     凭空生成一行等于制造噪声，还会让人误以为有人来过；
//  2. 后台扫描要为每条过期邀请补一行与用户行为无关的日志，审计表随时间线性膨胀，
//     却没有对应的可追溯价值；
//  3. 判定只有 inviteRejectReason 一处实现，兑换（JoinViaInvite）与撤销
//     （RevokeInvite）共用同一个函数、同一个 expires_at <= now 口径，
//     不存在"这里按查询时、那里按标记位"的两套实现互相打架。
//
// 因此：过期这件事只在【有人拿它做动作】时才留下痕迹（reason = invite_expired）。
//
// 【策略二：审计内容最小化】
//
// 记：邀请主键、目标角色、由此产生的用户/成员主键、绑定到了哪一维（username/email/none）、
//    事件类型与结果。
// 不记：邀请码（哪怕掩码前缀）、口令、绑定邮箱本体、绑定用户名本体。
//   - 邀请码等价于入户凭证，审计里留全文，任何能读库的人都能直接入户；
//     记"前 4 位"同样不可接受：它把暴力搜索空间缩小若干数量级，还免费给出命中确认信号。
//   - 邮箱/用户名属于 PII，且本场景只需要回答"是否绑定过、绑的是哪一维"。
//
// 【策略三：失败事件的取舍】
//
// 失败写一条 result=rejected + 粗粒度 reason 的行，方便事后看"有没有人在撞门户"；
//   - 不记尝试用的邀请码/用户名/邮箱：否则审计表自己变成一份枚举字典；
//   - 成功事件已有 actor_uid 可追溯，就不再落 IP；失败事件没有主语，
//     IP 是唯一可用的取证线索，因此只在这种情况下记 ipBucket(clientIP)；
//   - **限流拒绝不写审计**：那条请求根本没碰到邀请状态，若每写一行就等于给攻击者
//     一个"按请求放大审计表"的开关（超过配额后每个请求一行），由限流器本身兜住。
//
// 【策略四：动作/实体落在既有枚举上，不扩展枚举字典】
//
// 《枚举字典》是枚举的唯一权威，本次不新增常量；邀请事件复用：
//
//	entity_type = AuditEntityMember("book_member")，entity_id = invites.id
//	create      → 邀请创建 / 兑换入户（一条成员关系被建立）
//	delete      → 邀请撤销（入户凭证被废止）
//
// 与既有的 permission_change（同为 book_member，但 entity_id = 成员 uid）
// 靠 action 区分，查询时带上 action 即无歧义。

// 邀请事件类型（审计 payload 的 inviteEvent 字段）
const (
	inviteEventCreate = "invite_create"
	inviteEventRedeem = "invite_redeem"
	inviteEventRevoke = "invite_revoke"
)

// 事件结果
const (
	inviteResultAccepted = "accepted"
	inviteResultRejected = "rejected"
)

// 绑定维度（只记维度，不记本体）
const (
	inviteBoundNone     = "none"
	inviteBoundUsername = "username"
	inviteBoundEmail    = "email"
)

// inviteAuditFields 构造邀请审计 payload 的入参；零值字段不落库。
type inviteAuditFields struct {
	InviteID       int64
	Role           int8
	BoundTarget    string
	ExpiresAt      int64
	JoinerUid      int64
	JoinerUsername string
	ClientIP       string
}

// inviteAuditPayload 生成审计 payload（会被 JSON 序列化进 before_json）。
//
// result 与 reason 是固定两件套：成功事件 reason 为空串、失败事件 result=rejected。
// 这样"这条是不是失败"永远可以一眼判定，不需要靠"有没有某个字段"去猜。
func inviteAuditPayload(event, result, reason string, f inviteAuditFields) map[string]interface{} {
	p := map[string]interface{}{
		"inviteEvent": event,
		"result":      result,
	}
	if reason != inviteReasonNone {
		p["reason"] = reason
	}
	if f.InviteID != 0 {
		p["inviteId"] = f.InviteID
	}
	if f.Role != 0 {
		p["role"] = f.Role
	}
	if f.BoundTarget != "" {
		p["boundTarget"] = f.BoundTarget
	}
	if f.ExpiresAt != 0 {
		p["expiresAt"] = f.ExpiresAt
	}
	// 身份信息只出现在【成功】事件里（失败事件记身份 = 把审计做成枚举字典）
	if result == inviteResultAccepted {
		if f.JoinerUid != 0 {
			p["joinerUid"] = f.JoinerUid
		}
		if f.JoinerUsername != "" {
			p["joinerUsername"] = f.JoinerUsername
		}
	}
	if result == inviteResultRejected && f.ClientIP != "" {
		p["clientIP"] = f.ClientIP
	}
	return p
}

// inviteBoundTargetOf 由两个绑定字段推出"绑到了哪一维"（不输出具体的邮箱/用户名）
func inviteBoundTargetOf(boundUsername, boundEmail string) string {
	switch {
	case boundUsername != "":
		return inviteBoundUsername
	case boundEmail != "":
		return inviteBoundEmail
	default:
		return inviteBoundNone
	}
}

// appendInviteRejectedAudit 在【事务回滚之后】补一条被拒事件的审计。
//
// 为什么不用 insertAudit（事务内）：失败路径必须回滚——不回滚就等于"拒绝了一半"，
// 而回滚会把同事务里写的审计一起抹掉，结果是"拒绝了且什么都没留下"。
// 因此这里走导出的 AppendAudit，在独立连接上落一行。
func (s *MemberService) appendInviteRejectedAudit(
	bookID, actorUID, inviteID int64, event, reason string, f inviteAuditFields) error {
	f.InviteID = inviteID
	return AppendAudit(s.engine, bookID, actorUID, inviteAuditAction(event),
		model.AuditEntityMember, inviteID,
		inviteAuditPayload(event, inviteResultRejected, reason, f), "server")
}

// inviteAuditAction 事件类型 → 审计动作（成功链路用；失败链路同样复用）
func inviteAuditAction(event string) string {
	if event == inviteEventRevoke {
		// 撤销 = 废止入户凭证，取"删除"语义（与"恢复一笔交易"restore 无冲突，
		// 因为实体类型是 book_member，不代表交易被恢复）
		return model.AuditActionDelete
	}
	// 创建邀请 / 兑换入户：一条成员关系被建立
	return model.AuditActionCreate
}

// JoinViaInvite 凭邀请码注册并加入。
//
// 三条硬约束：
//  1. **原子**（D-R07）：建 user + 写 book_members + 标记邀请已用，全在**同一个事务**内；
//     不再调用会自行提交的 auth.Register，任一步失败整笔回滚，不产生孤儿用户。
//  2. **单次使用**（D-R08）：邀请在事务内重读，且标记已用的 UPDATE 必须 `RowsAffected==1`；
//     为 0 说明已被并发抢用，回滚并返回 ErrInviteInvalid。
//  3. **按 IP 限流**（WB-R01 / PRD §12.1 L572）：同一 IP 每小时 ≤10 次尝试。
//     原实现用全局 key "invite:join"，任意一人耗尽计数即可让**全实例**一小时无法入户。
//  4. **绑定闭环**（GPT-R41）：邀请若绑定了用户名或邮箱，兑换者必须提供【匹配】的
//     值；邮箱按 normalizeBindingEmail 规范化后比较（去首尾空白 + 转小写），
//     用户名沿用既有的大小写不敏感比较（strings.EqualFold）。
//
// 三条硬约束（同上）里的第 3 条限流见正文注释。
func (s *MemberService) JoinViaInvite(bookID int64, code, username, email, password, clientIP string) (*model.User, error) {
	// GPT-R41：邮箱先规范化再落地/比较。邀请签发端与兑换端共用同一个规范化函数，
	// 因此"同一个邮箱"在任何一端都收敛到同一个字符串，大小写差异不会造成误判。
	// 放在限流与事务之前还有一个原因：格式非法的输入不该消耗兑换配额。
	email = normalizeBindingEmail(email)
	if err := validateBindingEmail(email); err != nil {
		return nil, err
	}
	if !s.limiter.Allow("invite:join:ip:"+ipBucket(clientIP), 10, time.Hour) {
		return nil, ErrRateLimited
	}

	// 校验口令并算哈希；bcrypt 放在事务外，避免长时间占住唯一连接
	uname, hash, err := normalizeAndHash(username, password)
	if err != nil {
		return nil, err
	}

	sess := s.engine.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return nil, err
	}

	// 邀请的读必须在事务内：读与写不能跨事务，否则两个请求都能读到 used_at=0（D-R08 竞争窗口）
	var inv model.Invite
	has, err := sess.Where("code = ?", strings.TrimSpace(code)).Get(&inv)
	if err != nil {
		_ = sess.Rollback()
		return nil, err
	}
	// "存在但不属于本账本"与"不存在"对外不可区分、也必须不可区分：
	// 否则攻击者可以拿已知邀请去枚举别的账本里有哪些主键存在。
	found := has && inv.BookId == bookID
	if reason := inviteRejectReason(found, &inv, time.Now().Unix()); reason != inviteReasonNone {
		_ = sess.Rollback()
		return nil, s.inviteFailed(inviteEventRedeem, bookID, model.AuditActorSystem,
			inviteIDOrZero(found, inv.Id), reason, ipBucket(clientIP))
	}
	// GPT-R43：账本状态的检查必须在【事务内】、且在写任何东西之前。
	//
	// 放在事务外会留下这样的窗口：请求进来时账本还在，随即被 owner 删除，
	// 于是这一边继续给一个已经不存在的账本塞进新成员——留下无主的用户与成员行。
	// 这里与后续的 INSERT 共享同一快照，删除一旦先提交，本次判定立刻变成"账本没了"。
	if err := requireActiveBookTx(sess, inv.BookId); err != nil {
		_ = sess.Rollback()
		// 对外仍然并入"邀请无效"：兑换入口未认证，不能让它成为账本存在性的探测器。
		return nil, s.inviteFailed(inviteEventRedeem, bookID, model.AuditActorSystem, inv.Id,
			inviteReasonBookDeleted, ipBucket(clientIP))
	}
	if inv.BoundUsername != "" && !strings.EqualFold(inv.BoundUsername, uname) {
		_ = sess.Rollback()
		return nil, s.inviteFailed(inviteEventRedeem, bookID, model.AuditActorSystem, inv.Id,
			inviteReasonBoundMismatch, ipBucket(clientIP))
	}
	// GPT-R41：邮箱绑定校验。两侧都已过同一个规范化函数，因此这里是精确的
	// "同一个人"判定；邀请绑了邮箱而兑换方留空时，空串必然不等于绑定值 → 拒绝，
	// 不会出现"不填邮箱反而能绕过绑定"的洞。
	if inv.BoundEmail != "" && inv.BoundEmail != email {
		_ = sess.Rollback()
		return nil, s.inviteFailed(inviteEventRedeem, bookID, model.AuditActorSystem, inv.Id,
			inviteReasonBoundMismatch, ipBucket(clientIP))
	}

	// GPT-R41：邮箱占用检查必须在事务内、建号之前。
	//
	// users 上有 `UNIQUE(email) WHERE email <> ''`，若等到 INSERT 才发现冲突，
	// 抛出来的是驱动层的原始约束错误（会被映射成 500 且对外无信息量）；
	// 提前判定并返回一个【明确】的错误，才能在回滚后给出可言说的失败原因。
	// 注意：这里刻意不区分"邮箱被谁占用"，对外只表现为"信息不匹配"（见 ErrInviteEmailUnavailable）。
	if email != "" {
		var emailTaken int64
		if _, err := sess.SQL(
			`SELECT COUNT(1) FROM users WHERE lower(email) = ? AND deleted = 0`, email).
			Get(&emailTaken); err != nil {
			_ = sess.Rollback()
			return nil, err
		}
		if emailTaken > 0 {
			_ = sess.Rollback()
			return nil, s.inviteFailed(inviteEventRedeem, bookID, model.AuditActorSystem, inv.Id,
				inviteReasonEmailTaken, ipBucket(clientIP))
		}
	}

	// 成员上限（PRD §12 决策 6：MVP 账本成员上限 20 人）。
	// 必须在**同一事务内**统计：两个邀请并发入户时，若在事务外计数会双双重置为
	// "未满 20"，把人数顶过上限。口径为 deleted=0（已软删退出的不占名额）。
	var memberCount int64
	if _, cerr := sess.SQL(
		`SELECT COUNT(1) FROM book_members WHERE book_id = ? AND deleted = 0`,
		inv.BookId).Get(&memberCount); cerr != nil {
		_ = sess.Rollback()
		return nil, cerr
	}
	if memberCount >= maxBookMembers {
		_ = sess.Rollback()
		return nil, ErrBookFull
	}

	u, err := s.auth.insertUserInTx(sess, uname, hash)
	if err != nil {
		_ = sess.Rollback()
		return nil, err
	}

	// GPT-R41：把邮箱落到用户档案，补齐"绑定闭环"的另一半。
	//
	// 只校验不落库会留下两个后遗症：① 兑换完成后无从回答"这个账号对应哪个邮箱"，
	// 绑定关系只存在于邀请记录里；② 老库若已有不同大小写的写法，
	// lower(email)  uniqueness 的比对口径也会漂移。
	// insertUserInTx 由 auth 拥有且不支持邮箱入参，这里在同一事务内补一次 UPDATE，
	// 既不动 auth.go，又保证"建号 + 落邮箱"原子。
	if email != "" {
		if _, err := sess.Where("id = ?", u.Id).Cols("email").
			Update(&model.User{Email: email}); err != nil {
			_ = sess.Rollback()
			return nil, err
		}
		u.Email = email
	}

	now := time.Now().Unix()
	if _, err := sess.Insert(&model.BookMember{
		BookId: inv.BookId, Uid: u.Id, Role: inv.Role, CanViewOthers: true, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		_ = sess.Rollback()
		return nil, err
	}

	// 单次使用：条件更新必须核对影响行数（D-R08）。
	//
	// GPT-R40：WHERE 里带上 `revoked = 0`，使"兑换"与"撤销"互为对方的失败条件——
	// 撤销抢先提交时本 UPDATE 命中 0 行，整笔回滚，不会出现"撤销了却又入户"。
	n, err := sess.Where("id = ? AND used_at = 0 AND revoked = 0", inv.Id).
		Cols("used_at", "used_by").Update(&model.Invite{UsedAt: now, UsedBy: u.Id})
	if err != nil {
		_ = sess.Rollback()
		return nil, err
	}
	if n != 1 {
		// GPT-R40：命中 0 行说明"撤销或另一个兑换"抢先提交了（redeemed 条件含 revoked = 0）。
		// 整笔回滚，不留下半截入户；并按 Erlang 失败语义返回 ErrInviteInvalid 以免泄露状态。
		_ = sess.Rollback()
		return nil, s.inviteFailed(inviteEventRedeem, bookID, model.AuditActorSystem, inv.Id,
			inviteReasonAlreadyUsed, ipBucket(clientIP))
	}

	// GPT-R42：成功兑换必须【与状态变更同事务】留痕。
	// actor 取新建用户：兑换是未认证入口，此刻他是唯一可归因的主体。
	// payload 里不放邀请码与邮箱——放谁都能入户/拿到身份，见本文件顶部的审计策略说明。
	if err := insertAudit(sess, inv.BookId, u.Id, inviteAuditAction(inviteEventRedeem),
		model.AuditEntityMember, inv.Id,
		inviteAuditPayload(inviteEventRedeem, inviteResultAccepted, inviteReasonNone, inviteAuditFields{
			InviteID:       inv.Id,
			Role:           inv.Role,
			BoundTarget:    inviteBoundTargetOf(inv.BoundUsername, inv.BoundEmail),
			JoinerUid:      u.Id,
			JoinerUsername: u.Username,
		}), "server"); err != nil {
		_ = sess.Rollback()
		return nil, err
	}

	if err := sess.Commit(); err != nil {
		return nil, err
	}
	return u, nil
}

// ---- GPT-R41：绑定维度（邮箱 / 用户名）的规范化与取值规则 --------------------

// normalizeBindingEmail 邮箱规范化：去首尾空白 + 转小写。全项目**唯一**的邮箱口径。
//
// 为什么必须统一口径：邮箱 technically 允许 local-part 大小写敏感（RFC 5321），
// 但所有主流邮件服务商都按不敏感处理。若签发端写 `Alice@x.com`、兑换端填
// `alice@x.com`，不规范化就会出现"本人拿不到自己的邀请"；两个用例都过同一个函数，
// 则同一身份在任何一端都收敛到同一个字符串。
//
// 用户名【不】套用本函数：既有的用户名语义是"大小写不敏感比较但保留原样存储"
// （join 时用 strings.EqualFold），改成小写落库会改变既有账号的数据形态。
func normalizeBindingEmail(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

// validateBindingEmail 保守的邮箱结构校验；空串合法（表示"不绑定 / 不填邮箱"）。
//
// 刻意不做 RFC 5322 全量语法校验：自托管场景要容忍各类内部域名和短充分发挥，
// 过严的正则会把合法邮箱挡在门外，反而逼着调用方绕过校验。这里要拦的是
// 【注定无法完成比对】的输入：含空白/多个 '@'、local-part 或域名为空、域名不带点、
// 域名以点开头或结尾。
func validateBindingEmail(email string) error {
	if email == "" {
		return nil
	}
	if strings.ContainsAny(email, " \t\r\n") || strings.Count(email, "@") != 1 {
		return fmt.Errorf("%w：邮箱格式非法", ErrInvalidArgument)
	}
	at := strings.Index(email, "@")
	local, domain := email[:at], email[at+1:]
	if local == "" || domain == "" {
		return fmt.Errorf("%w：邮箱格式非法（缺少用户名或域名）", ErrInvalidArgument)
	}
	if strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") || !strings.Contains(domain, ".") {
		return fmt.Errorf("%w：邮箱格式非法（域名不合法）", ErrInvalidArgument)
	}
	return nil
}

// resolveInviteBinding 解析邀请的绑定维度取值规则（GPT-R41）。
//
// 取值规则（对齐 PRD §12.1 决策#2 与《开发WBS》提案 3 的"可选绑定邮箱或用户名"）：
//
//	boundUsername  boundEmail  结果
//	空             空          不绑定：任何持码者可入户
//	非空           空          绑定用户名：兑换时用户名须匹配（大小写不敏感）
//	空             非空        绑定邮箱：兑换时邮箱须匹配（按规范化值比较）
//	非空           非空        【拒绝】二选一，不替调用方做静默取舍
//
// 为什么"两者都给"必须报错而不是"两个都要匹配"或"优先邮箱"：本缺陷的根因正是
// 「字段看似可用实则无效」——boundEmail 以前能传进来却被静默丢弃。
// 静默忽略其中一个入参，调用方会以为自己设置了双重绑定；显式报错让契约立刻暴露，
// 而不是留一个"不知道哪个生效"的灰色语义。
func resolveInviteBinding(boundUsername, boundEmail string) (username string, email string, err error) {
	username = strings.TrimSpace(boundUsername)
	email = normalizeBindingEmail(boundEmail)
	if vErr := validateBindingEmail(email); vErr != nil {
		return "", "", vErr
	}
	if username != "" && email != "" {
		return "", "", ErrInviteBindingConflict
	}
	return username, email, nil
}

// ipBucket 把客户端 IP 归一化为限流桶键；空值归入 unknown，避免"缺 IP 即绕过"。
func ipBucket(ip string) string {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return "unknown"
	}
	return ip
}

// 便于单元测试替换口令哈希成本（默认沿用 bcrypt.DefaultCost）
var _ = bcrypt.DefaultCost

func randomCode(n int) (string, error) {
	b := make([]byte, n/2+1)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("生成邀请码失败：%w", err)
	}
	return hex.EncodeToString(b)[:n], nil
}
