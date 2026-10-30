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
)

// GPT-R40：邀请撤销的失败语义，全部**继承**既有错误树（用 %w 包装），
// 因此不需要改动 mapSvcErr，也不会新开一套 HTTP 状态码契约。
var (
	// ErrInviteAlreadyUsed 邀请已被使用（不可再撤销）
	ErrInviteAlreadyUsed = fmt.Errorf("%w：邀请码已被使用", ErrInviteInvalid)
	// ErrInviteAlreadyRevoked 邀请已被撤销（重复撤销不会被当成成功）
	ErrInviteAlreadyRevoked = fmt.Errorf("%w：邀请码已被撤销", ErrInviteInvalid)
	// ErrInviteExpired 邀请已过期（过期判定见 inviteExpired，全局只有这一个口径）
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
func (s *MemberService) List(bookID int64) ([]MemberDTO, error) {
	var out []MemberDTO
	err := s.engine.SQL(
		`SELECT m.uid, u.username, m.role, m.can_view_others AS can_view_others,
		        u.is_instance_admin AS is_instance_admin
		 FROM book_members m INNER JOIN users u ON u.id = m.uid
		 WHERE m.book_id = ? AND m.deleted = 0 AND u.deleted = 0
		 ORDER BY m.role ASC, m.id ASC`, bookID).Find(&out)
	return out, err
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

	now := time.Now().Unix()
	n, err := sess.Where(
		"id = ? AND book_id = ? AND used_at = 0 AND revoked = 0 AND expires_at > ?",
		inviteID, bookID, now).
		Cols("revoked").Update(&model.Invite{Revoked: true})
	if err != nil {
		return err
	}
	if n != 1 {
		// 未命中：把原因查清楚再返回给调用方（"无效"太含糊，排查时无从下手）。
		// 这次读在本事务内，与上面的 UPDATE 共享同一快照，不存在交错。
		var inv model.Invite
		has, rErr := sess.Where("id = ? AND book_id = ?", inviteID, bookID).Get(&inv)
		if rErr != nil {
			return rErr
		}
		reason := inviteRejectReason(has, &inv, now)
		_ = sess.Rollback()
		return inviteRejectError(reason)
	}

	if err := sess.Commit(); err != nil {
		return err
	}
	return nil
}

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
	default:
		return ErrInviteInvalid
	}
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
	if !has || inv.BookId != bookID || inv.Revoked || inv.UsedAt > 0 || inv.ExpiresAt <= time.Now().Unix() {
		_ = sess.Rollback()
		return nil, ErrInviteInvalid
	}
	if inv.BoundUsername != "" && !strings.EqualFold(inv.BoundUsername, uname) {
		_ = sess.Rollback()
		return nil, ErrInviteBoundMismatch
	}
	// GPT-R41：邮箱绑定校验。两侧都已过同一个规范化函数，因此这里是精确的
	// "同一个人"判定；邀请绑了邮箱而兑换方留空时，空串必然不等于绑定值 → 拒绝，
	// 不会出现"不填邮箱反而能绕过绑定"的洞。
	if inv.BoundEmail != "" && inv.BoundEmail != email {
		_ = sess.Rollback()
		return nil, ErrInviteBoundMismatch
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
			return nil, ErrInviteEmailUnavailable
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
		_ = sess.Rollback()
		return nil, ErrInviteInvalid
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
