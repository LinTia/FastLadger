package service

// 个人资料与账号安全测试（PRD §4.12）：邮箱更新（格式/唯一/清除）、
// 密码修改（旧口令校验/强度/会话全撤/PAT 不受影响）。

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestUpdateEmail(t *testing.T) {
	e := newTestEnv(t)
	auth := NewAuthService(e.engine, "profile-test-secret-0123456789abcd", time.Hour)
	u0, err := auth.Register("tester", "password123")
	if err != nil {
		t.Fatalf("注册失败：%v", err)
	}
	uid := u0.Id

	// 合法更新
	u, err := auth.UpdateEmail(uid, "me@example.com")
	if err != nil {
		t.Fatalf("UpdateEmail 失败：%v", err)
	}
	if u.Email != "me@example.com" {
		t.Fatalf("邮箱未生效：%q", u.Email)
	}
	// 用户名不在可改范围（接口面即契约：UpdateEmail 只收 email），保持不变
	if u.Username != "tester" {
		t.Fatalf("用户名不应被改动：%q", u.Username)
	}

	// 格式非法：无 @ / 多个 @ / 含空白 / 超长
	for _, bad := range []string{"noat", "a@b@c", "a b@c.com", strings.Repeat("x", 130) + "@x.com"} {
		if _, err := auth.UpdateEmail(uid, bad); !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("非法邮箱 %q 应 ErrInvalidArgument，实际 %v", bad, err)
		}
	}

	// 唯一冲突：第二个用户占用同邮箱 → 409 语义
	u2, err := auth.Register("tester2", "password123")
	if err != nil {
		t.Fatalf("注册第二个用户失败：%v", err)
	}
	if _, err := auth.UpdateEmail(u2.Id, "me@example.com"); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("邮箱重复应 ErrEmailTaken，实际 %v", err)
	}

	// 清除（空串）允许，且不与唯一约束冲突
	if _, err := auth.UpdateEmail(uid, ""); err != nil {
		t.Fatalf("清除邮箱失败：%v", err)
	}
}

func TestChangePassword(t *testing.T) {
	e := newTestEnv(t)
	auth := NewAuthService(e.engine, "profile-test-secret-0123456789abcd", time.Hour)
	u0, err := auth.Register("tester", "password123")
	if err != nil {
		t.Fatalf("注册失败：%v", err)
	}
	uid := u0.Id

	oldToken, err := auth.LoginFromIP("tester", "password123", "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("登录失败：%v", err)
	}

	// 旧口令错误 → ErrBadCredentials
	if _, err := auth.ChangePassword(uid, "wrong-old", "newpassword456"); !errors.Is(err, ErrBadCredentials) {
		t.Fatalf("旧口令错误应 ErrBadCredentials，实际 %v", err)
	}

	// 新口令过短 → ErrWeakPassword
	if _, err := auth.ChangePassword(uid, "password123", "short"); !errors.Is(err, ErrWeakPassword) {
		t.Fatalf("弱口令应 ErrWeakPassword，实际 %v", err)
	}

	// 正常改密 → 撤销全部会话（≥1），旧令牌立即失效
	revoked, err := auth.ChangePassword(uid, "password123", "newpassword456")
	if err != nil {
		t.Fatalf("ChangePassword 失败：%v", err)
	}
	if revoked < 1 {
		t.Fatalf("应撤销至少 1 个会话，实际 %d", revoked)
	}
	if _, err := auth.Verify(oldToken); !errors.Is(err, ErrTokenRevoked) {
		t.Fatalf("改密后旧令牌应 ErrTokenRevoked，实际 %v", err)
	}

	// 新口令可登录
	if _, err := auth.LoginFromIP("tester", "newpassword456", "127.0.0.1", "test-agent"); err != nil {
		t.Fatalf("新口令登录失败：%v", err)
	}

	// PAT 不受改密影响（长期凭证语义，可单独撤销）
	pat := NewPATService(e.engine)
	plain, _, err := pat.Create(uid, "改密后仍有效")
	if err != nil {
		t.Fatalf("Create PAT 失败：%v", err)
	}
	if _, err := auth.ChangePassword(uid, "newpassword456", "newpassword789"); err != nil {
		t.Fatalf("再次改密失败：%v", err)
	}
	if _, err := pat.VerifyPAT(plain); err != nil {
		t.Fatalf("PAT 不应受改密影响：%v", err)
	}
}
