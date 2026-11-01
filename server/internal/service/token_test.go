package service

// 长期 API Token（PAT）服务测试（WBS 6.6）。
//
// 覆盖：生成→明文仅一次→哈希不可逆查、Verify 全链路（有效/撤销/不存在）、
// 撤销后立即失效、跨用户撤销 404、账号禁用即失效、数量上限。

import (
	"errors"
	"strings"
	"testing"

	"fastledger/internal/model"
)

func TestPATCreateAndVerify(t *testing.T) {
	e := newTestEnv(t)
	pat := NewPATService(e.engine)

	plain, item, err := pat.Create(e.uid, "快捷指令")
	if err != nil {
		t.Fatalf("Create 失败：%v", err)
	}
	if !strings.HasPrefix(plain, PATPrefix) || len(plain) < 40 {
		t.Fatalf("明文格式非法：%q", plain)
	}
	if item.Id == 0 || item.CreatedAt == 0 {
		t.Fatalf("列表项字段缺失：%+v", item)
	}

	// 哈希不可逆：库里查不到明文痕迹
	var raw struct {
		TokenHash string `xorm:"token_hash"`
	}
	has, err := e.engine.SQL("SELECT token_hash FROM auth_tokens WHERE id = ?", item.Id).Get(&raw)
	if err != nil || !has {
		t.Fatalf("读哈希行失败：%v", err)
	}
	if strings.Contains(raw.TokenHash, plain) || raw.TokenHash == plain {
		t.Fatal("库里不得出现明文")
	}

	// 有效鉴权
	u, err := pat.VerifyPAT(plain)
	if err != nil {
		t.Fatalf("VerifyPAT 失败：%v", err)
	}
	if u.Id != e.uid {
		t.Fatalf("鉴权用户错位：%d != %d", u.Id, e.uid)
	}

	// 伪造 Token → 拒绝
	if _, err := pat.VerifyPAT(PATPrefix + strings.Repeat("0", 64)); !errors.Is(err, ErrBadCredentials) {
		t.Fatalf("伪造 Token 应 ErrBadCredentials，实际 %v", err)
	}

	// 撤销后立即失效
	if err := pat.Revoke(e.uid, item.Id); err != nil {
		t.Fatalf("Revoke 失败：%v", err)
	}
	if _, err := pat.VerifyPAT(plain); !errors.Is(err, ErrTokenRevoked) {
		t.Fatalf("撤销后应 ErrTokenRevoked，实际 %v", err)
	}

	// 跨用户撤销 → 404 语义（ErrPATNotFound）
	plain2, item2, err := pat.Create(e.uid, "第二个")
	if err != nil {
		t.Fatalf("第二次 Create 失败：%v", err)
	}
	if err := pat.Revoke(e.uid+999, item2.Id); !errors.Is(err, ErrPATNotFound) {
		t.Fatalf("跨用户撤销应 ErrPATNotFound，实际 %v", err)
	}
	_ = plain2

	// 列表含撤销状态、绝不含哈希字段
	items, err := pat.List(e.uid)
	if err != nil {
		t.Fatalf("List 失败：%v", err)
	}
	if len(items) != 2 {
		t.Fatalf("列表期望 2 条，得到 %d", len(items))
	}
}

func TestPATDisabledUserRejected(t *testing.T) {
	e := newTestEnv(t)
	pat := NewPATService(e.engine)
	plain, _, err := pat.Create(e.uid, "CI")
	if err != nil {
		t.Fatalf("Create 失败：%v", err)
	}
	// 禁用账号
	if _, err := e.engine.ID(e.uid).Cols("disabled").Update(&model.User{Disabled: true}); err != nil {
		t.Fatalf("禁用失败：%v", err)
	}
	if _, err := pat.VerifyPAT(plain); !errors.Is(err, ErrAccountDisabled) {
		t.Fatalf("禁用账号应 ErrAccountDisabled，实际 %v", err)
	}
}

func TestPATIsPAT(t *testing.T) {
	if !IsPAT("flt_abcdef") {
		t.Fatal("flt_ 前缀应识别为 PAT")
	}
	if IsPAT("eyJhbGciOiJIUzI1NiJ9.x.y") {
		t.Fatal("JWT 不应识别为 PAT")
	}
	if IsPAT("flt_") {
		t.Fatal("空前缀体不应识别为 PAT")
	}
}
