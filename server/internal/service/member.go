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

const daySeconds = int64(24 * 60 * 60)

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
func NewMemberService(engine *xorm.Engine, auth *AuthService, inviteTTLDays int) *MemberService {
	return &MemberService{
		engine:       engine,
		auth:         auth,
		inviteTTLSec: int64(inviteTTLDays) * daySeconds,
		limiter:      NewLimiter(),
	}
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
	actorRole, err := RoleOf(s.engine, bookID, actor)
	if err != nil {
		return err
	}
	if !CanManageMembers(actorRole) {
		return ErrForbidden
	}
	if newRole != model.RoleAdmin && newRole != model.RoleMember {
		// 归入 ErrInvalidArgument：由 mapSvcErr 映射为 400，而不是 500
		return fmt.Errorf("%w：只能将成员设为管理员或普通成员", ErrInvalidArgument)
	}
	targetRole, err := RoleOf(s.engine, bookID, targetUid)
	if err != nil {
		return err
	}
	if targetRole == model.RoleOwner {
		return ErrCannotOperateOwner
	}
	if actorRole == model.RoleAdmin && targetRole == model.RoleOwner {
		return ErrCannotOperateOwner
	}
	n, err := s.engine.Where("book_id = ? AND uid = ? AND deleted = 0", bookID, targetUid).
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
	return nil
}

// Remove 移除成员（软删，保留历史记录）
func (s *MemberService) Remove(actor, bookID, targetUid int64) error {
	actorRole, err := RoleOf(s.engine, bookID, actor)
	if err != nil {
		return err
	}
	if !CanManageMembers(actorRole) {
		return ErrForbidden
	}
	targetRole, err := RoleOf(s.engine, bookID, targetUid)
	if err != nil {
		return err
	}
	if targetRole == model.RoleOwner {
		return ErrCannotOperateOwner
	}
	now := time.Now().Unix()
	n, err := s.engine.Where("book_id = ? AND uid = ? AND deleted = 0", bookID, targetUid).
		Cols("deleted", "deleted_at", "updated_at").
		Update(&model.BookMember{Deleted: true, DeletedAt: now, UpdatedAt: now})
	if err != nil {
		return err
	}
	if n != 1 {
		// 与 ChangeRole 同理（D-R09）：目标已被并发移除时 0 行，不能当成功返回。
		return fmt.Errorf("%w：目标成员已不存在或已被移除", ErrConcurrentConflict)
	}
	return nil
}

// CreateInvite 生成邀请码（限流：每账本每分钟 ≤5）
func (s *MemberService) CreateInvite(actor, bookID int64, role int8, boundUsername string) (*model.Invite, error) {
	actorRole, err := RoleOf(s.engine, bookID, actor)
	if err != nil {
		return nil, err
	}
	if !CanManageMembers(actorRole) {
		return nil, ErrForbidden
	}
	if role != model.RoleAdmin && role != model.RoleMember {
		role = model.RoleMember
	}
	if !s.limiter.Allow(fmt.Sprintf("invite:create:book:%d", bookID), 5, time.Minute) {
		return nil, ErrRateLimited
	}

	code, err := randomCode(32)
	if err != nil {
		return nil, err
	}
	inv := &model.Invite{
		BookId:        bookID,
		Code:          code,
		Role:          role,
		CreatedBy:     actor,
		ExpiresAt:     time.Now().Unix() + s.inviteTTLSec,
		BoundUsername: strings.TrimSpace(boundUsername),
		CreatedAt:     time.Now().Unix(),
	}
	if _, err := s.engine.Insert(inv); err != nil {
		return nil, err
	}
	return inv, nil
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
func (s *MemberService) JoinViaInvite(bookID int64, code, username, password, clientIP string) (*model.User, error) {
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

	now := time.Now().Unix()
	if _, err := sess.Insert(&model.BookMember{
		BookId: inv.BookId, Uid: u.Id, Role: inv.Role, CanViewOthers: true, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		_ = sess.Rollback()
		return nil, err
	}

	// 单次使用：条件更新必须核对影响行数（D-R08）
	n, err := sess.Where("id = ? AND used_at = 0", inv.Id).
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
