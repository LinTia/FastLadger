package service

// WBS 4.3 非交易审计写入点的真实 SQLite 集成测试。
// 需 CGO（gcc），由 CI 在 ubuntu-latest 执行。复用 tx_integration_test.go 的 testEnv。

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"fastledger/internal/model"
	"fastledger/internal/version"
)

const testJWTSecret = "test-secret-0123456789abcdefghijklmnopq"

// auditCountAction 按动作统计审计条数（**不按账本过滤**：登录/备份恢复是实例级事件）
func auditCountAction(e *testEnv, action string) int64 {
	var n int64
	if _, err := e.engine.SQL("SELECT COUNT(1) FROM audit_logs WHERE action = ?", action).Get(&n); err != nil {
		e.t.Fatalf("统计审计失败：%v", err)
	}
	return n
}

// auditBeforeJSON 取某动作最近一条审计的 before_json
func auditBeforeJSON(e *testEnv, action string, entityID int64) string {
	var s string
	has, err := e.engine.SQL(
		`SELECT before_json FROM audit_logs WHERE action = ? AND entity_id = ? ORDER BY id DESC LIMIT 1`,
		action, entityID).Get(&s)
	if err != nil {
		e.t.Fatalf("读取审计失败：%v", err)
	}
	if !has {
		e.t.Fatalf("未找到 action=%s entity_id=%d 的审计", action, entityID)
	}
	return s
}

// TestLoginWritesAudit 登录写审计，且为【实例级】事件（book_id=0，不隶属任何账本）。
func TestLoginWritesAudit(t *testing.T) {
	e := newTestEnv(t)
	as := NewAuthService(e.engine, testJWTSecret, time.Hour)
	if _, err := as.Register("audit-login", "password123"); err != nil {
		t.Fatalf("注册失败：%v", err)
	}

	if n := auditCountAction(e, model.AuditActionLogin); n != 0 {
		t.Fatalf("登录前不应有登录审计，实际 %d", n)
	}
	tok, err := as.Login("audit-login", "password123")
	must(t, err)
	if tok == "" {
		t.Fatal("登录成功应返回令牌")
	}

	// 实例级事件：book_id 必须是哨兵 0
	var n int64
	if _, err := e.engine.SQL(
		"SELECT COUNT(1) FROM audit_logs WHERE action = ? AND book_id = ?",
		model.AuditActionLogin, model.AuditBookInstance).Get(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("登录应写 1 条实例级（book_id=0）审计，实际 %d", n)
	}
}

// TestLoginFailureWritesNoAudit 口令错误时不得写审计（没有成功登录这件事可记）。
func TestLoginFailureWritesNoAudit(t *testing.T) {
	e := newTestEnv(t)
	as := NewAuthService(e.engine, testJWTSecret, time.Hour)
	if _, err := as.Register("audit-badlogin", "password123"); err != nil {
		t.Fatalf("注册失败：%v", err)
	}
	if _, err := as.Login("audit-badlogin", "wrong-password"); err == nil {
		t.Fatal("错误口令应登录失败")
	}
	if n := auditCountAction(e, model.AuditActionLogin); n != 0 {
		t.Fatalf("登录失败不应写审计，实际 %d", n)
	}
}

// TestChangeRoleWritesAudit 改角色写权限变更审计，before_json 保留【变更前】角色。
func TestChangeRoleWritesAudit(t *testing.T) {
	e := newTestEnv(t)
	as := NewAuthService(e.engine, testJWTSecret, time.Hour)
	ms := NewMemberService(e.engine, as, 7)

	target := insertUser(t, e, "role-target")
	addMember(t, e, target, model.RoleMember)

	must(t, ms.ChangeRole(e.uid, e.bookID, target, model.RoleAdmin))

	if n := auditCountAction(e, model.AuditActionPermissionChange); n != 1 {
		t.Fatalf("改角色应写 1 条权限变更审计，实际 %d", n)
	}
	before := auditBeforeJSON(e, model.AuditActionPermissionChange, target)
	if !strings.Contains(before, fmt.Sprintf(`"role":%d`, model.RoleMember)) {
		t.Fatalf("before_json 应记录变更前角色 %d，实际 %q", model.RoleMember, before)
	}
}

// TestRemoveMemberWritesAudit 移除成员写权限变更审计，且保留被移除前的角色。
func TestRemoveMemberWritesAudit(t *testing.T) {
	e := newTestEnv(t)
	as := NewAuthService(e.engine, testJWTSecret, time.Hour)
	ms := NewMemberService(e.engine, as, 7)

	target := insertUser(t, e, "remove-target")
	addMember(t, e, target, model.RoleMember)

	must(t, ms.Remove(e.uid, e.bookID, target))

	if n := auditCountAction(e, model.AuditActionPermissionChange); n != 1 {
		t.Fatalf("移除成员应写 1 条权限变更审计，实际 %d", n)
	}
	before := auditBeforeJSON(e, model.AuditActionPermissionChange, target)
	if !strings.Contains(before, fmt.Sprintf(`"role":%d`, model.RoleMember)) {
		t.Fatalf("before_json 应保留移除前角色 %d，实际 %q", model.RoleMember, before)
	}
}

// TestSetVisibilityWritesAudit 可见性开关写配置变更审计，before_json 保留旧值。
func TestSetVisibilityWritesAudit(t *testing.T) {
	e := newTestEnv(t)
	bs := NewBookService(e.engine, version.NewRepo())

	b, err := bs.Get(e.bookID)
	must(t, err)
	old := b.MembersCanViewOthers

	changed, err := bs.SetVisibility(e.uid, e.bookID, !old)
	must(t, err)
	if !changed {
		t.Fatal("切换可见性应产生实际变更")
	}
	if n := auditCountAction(e, model.AuditActionConfigChange); n != 1 {
		t.Fatalf("可见性变更应写 1 条配置变更审计，实际 %d", n)
	}
	before := auditBeforeJSON(e, model.AuditActionConfigChange, e.bookID)
	// 旧值必须如实落到 before_json（true/false 皆可，取决于初值）
	if !strings.Contains(before, fmt.Sprintf(`"membersCanViewOthers":%t`, old)) {
		t.Fatalf("before_json 应记录变更前值 %t，实际 %q", old, before)
	}
}

// TestSetVisibilityIdempotentWritesNoAudit 幂等（值未变）时不写审计：
// 没有发生变更，就不该产生一条"看起来改了"的留痕。
func TestSetVisibilityIdempotentWritesNoAudit(t *testing.T) {
	e := newTestEnv(t)
	bs := NewBookService(e.engine, version.NewRepo())
	b, err := bs.Get(e.bookID)
	must(t, err)

	changed, err := bs.SetVisibility(e.uid, e.bookID, b.MembersCanViewOthers)
	must(t, err)
	if changed {
		t.Fatal("设置相同值应返回未变更")
	}
	if n := auditCountAction(e, model.AuditActionConfigChange); n != 0 {
		t.Fatalf("幂等设置不应写审计，实际 %d", n)
	}
}
