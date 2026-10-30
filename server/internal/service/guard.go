package service

// 账本成员资格校验（服务端权威判定）。
//
// 约束：`book_id` / `uid` 一律来自可信上下文（路径参数 + 认证中间件），
// 绝不信任请求体里的覆盖字段；非成员一律返回 ErrNotMember（HTTP 403）。
import (
	"fastledger/internal/model"

	"xorm.io/xorm"
)

// requireMemberTx 在【调用方已有的事务内】复核成员资格。
func requireMemberTx(sess *xorm.Session, bookID, uid int64) error {
	var role int64
	has, err := sess.SQL(
		`SELECT m.role FROM book_members m
		 INNER JOIN books b ON b.id = m.book_id AND b.deleted = 0
		 WHERE m.book_id = ? AND m.uid = ? AND m.deleted = 0`, bookID, uid).Get(&role)
	if err != nil {
		return err
	}
	if !has || role < int64(model.RoleOwner) || role > int64(model.RoleMember) {
		return ErrNotMember
	}
	return nil
}

// requireMember 非事务版本的成员资格复核（只读路径使用）。
func requireMember(engine *xorm.Engine, bookID, uid int64) error {
	sess := engine.NewSession()
	defer sess.Close()
	return requireMemberTx(sess, bookID, uid)
}

// requireManagerTx 写账本级共享资源（账户/分类/账本设置）需要 owner 或 admin。
// 权限矩阵依据《落地方案》§5.5【修订版】。
func requireManagerTx(sess *xorm.Session, bookID, uid int64) (int8, error) {
	var role int64
	has, err := sess.SQL(
		`SELECT m.role FROM book_members m
		 INNER JOIN books b ON b.id = m.book_id AND b.deleted = 0
		 WHERE m.book_id = ? AND m.uid = ? AND m.deleted = 0`, bookID, uid).Get(&role)
	if err != nil {
		return 0, err
	}
	if !has || role < int64(model.RoleOwner) || role > int64(model.RoleMember) {
		return 0, ErrNotMember
	}
	r := int8(role)
	if r != model.RoleOwner && r != model.RoleAdmin {
		return r, ErrForbidden
	}
	return r, nil
}
