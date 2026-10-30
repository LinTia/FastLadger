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

var (
	// ErrInviteInvalid 邀请码无效（不存在/已用/已撤销/过期）
	ErrInviteInvalid = errors.New("邀请码无效或已过期")
	// ErrInviteBoundMismatch 邀请码绑定的用户名/邮箱不匹配
	ErrInviteBoundMismatch = errors.New("邀请码已绑定指定用户，信息不匹配")
	// ErrCannotOperateOwner 不能操作 owner
	ErrCannotOperateOwner = errors.New("不能对账本所有者执行该操作")
	// ErrRateLimited 触发限流
	ErrRateLimited = errors.New("操作过于频繁，请稍后再试")
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
	_, err = s.engine.Where("book_id = ? AND uid = ? AND deleted = 0", bookID, targetUid).
		Cols("role", "updated_at").
		Update(&model.BookMember{Role: newRole, UpdatedAt: time.Now().Unix()})
	return err
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
	_, err = s.engine.Where("book_id = ? AND uid = ? AND deleted = 0", bookID, targetUid).
		Cols("deleted", "deleted_at", "updated_at").
		Update(&model.BookMember{Deleted: true, DeletedAt: now, UpdatedAt: now})
	return err
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

// JoinViaInvite 凭邀请码注册并加入：原子建 user + book_members，并标记邀请已使用
func (s *MemberService) JoinViaInvite(code, username, password string) (*model.User, error) {
	if !s.limiter.Allow("invite:join", 10, time.Hour) {
		return nil, ErrRateLimited
	}
	var inv model.Invite
	has, err := s.engine.Where("code = ?", strings.TrimSpace(code)).Get(&inv)
	if err != nil {
		return nil, err
	}
	if !has || inv.Revoked || inv.UsedAt > 0 || inv.ExpiresAt <= time.Now().Unix() {
		return nil, ErrInviteInvalid
	}
	if inv.BoundUsername != "" && !strings.EqualFold(inv.BoundUsername, strings.TrimSpace(username)) {
		return nil, ErrInviteBoundMismatch
	}
	if len(password) < minPasswordLen {
		return nil, ErrWeakPassword
	}

	u, err := s.auth.Register(username, password)
	if err != nil {
		return nil, err
	}

	sess := s.engine.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	if _, err := sess.Insert(&model.BookMember{
		BookId: inv.BookId, Uid: u.Id, Role: inv.Role, CanViewOthers: true, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		_ = sess.Rollback()
		return nil, err
	}
	if _, err := sess.Where("id = ? AND used_at = 0", inv.Id).
		Cols("used_at", "used_by").Update(&model.Invite{UsedAt: now, UsedBy: u.Id}); err != nil {
		_ = sess.Rollback()
		return nil, err
	}
	if err := sess.Commit(); err != nil {
		return nil, err
	}
	return u, nil
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
