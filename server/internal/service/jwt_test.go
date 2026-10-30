package service

// JWT 与权限矩阵的运行时验证（不依赖数据库，无需 CGO）。
// 覆盖 R09 的安全要求：限定 HS256、拒绝 alg=none、拒绝篡改、拒绝过期、安全读取 uid。
import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"fastledger/internal/model"
)

const testSecret = "unit-test-secret-please-do-not-use-in-prod-0123456789"

func TestJWTRoundTrip(t *testing.T) {
	tok, err := SignJWT(testSecret, 42, time.Hour)
	if err != nil {
		t.Fatalf("签发失败：%v", err)
	}
	c, err := ParseJWT(testSecret, tok)
	if err != nil {
		t.Fatalf("解析失败：%v", err)
	}
	if c.Uid != 42 {
		t.Fatalf("uid 不符：期望 42，得到 %d", c.Uid)
	}
	if c.Jti == "" {
		t.Fatal("jti 为空（不利于后续吊销）")
	}
}

// TestJWTRejectAlgNone 拒绝 alg=none（经典降级攻击）
func TestJWTRejectAlgNone(t *testing.T) {
	hdr := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	pl := base64.RawURLEncoding.EncodeToString([]byte(`{"uid":42,"jti":"x","exp":9999999999}`))
	forged := hdr + "." + pl + "."
	if _, err := ParseJWT(testSecret, forged); err != ErrTokenBadAlg {
		t.Fatalf("应拒绝 alg=none，实际：%v", err)
	}
}

// TestJWTRejectTampered 篡改载荷后签名校验必须失败
func TestJWTRejectTampered(t *testing.T) {
	tok, _ := SignJWT(testSecret, 42, time.Hour)
	parts := strings.Split(tok, ".")
	// 把 uid 从 42 改成 43
	bad := base64.RawURLEncoding.EncodeToString([]byte(`{"uid":43,"jti":"x","exp":9999999999}`))
	tampered := parts[0] + "." + bad + "." + parts[2]
	if _, err := ParseJWT(testSecret, tampered); err != ErrTokenBadSign {
		t.Fatalf("应拒绝篡改签名，实际：%v", err)
	}
}

// TestJWTRejectExpired 过期 token 必须失效
func TestJWTRejectExpired(t *testing.T) {
	tok, err := SignJWT(testSecret, 42, -time.Minute)
	if err != nil {
		t.Fatalf("签发失败：%v", err)
	}
	if _, err := ParseJWT(testSecret, tok); err != ErrTokenExpired {
		t.Fatalf("应拒绝过期 token，实际：%v", err)
	}
}

// TestJWTRejectBadSecret 用错误密钥签发的 token 必须被拒
func TestJWTRejectBadSecret(t *testing.T) {
	tok, _ := SignJWT("another-secret-that-is-long-enough-0123456789", 42, time.Hour)
	if _, err := ParseJWT(testSecret, tok); err != ErrTokenBadSign {
		t.Fatalf("应拒绝错误密钥，实际：%v", err)
	}
}

// TestPermissionMatrix 权限矩阵严格对齐《落地方案》§5.5【修订版】（[1.9] 裁决）
func TestPermissionMatrix(t *testing.T) {
	cases := []struct {
		role int8
		name string
		// 改/删他人交易：owner ✅ / admin ✅（由"可配置"改为 ✅）/ member ❌
		editOthers bool
		// 邀请/移除成员、改角色：owner ✅ / admin ✅ / member ❌
		manageMembers bool
	}{
		{model.RoleOwner, "owner", true, true},
		{model.RoleAdmin, "admin", true, true},
		{model.RoleMember, "member", false, false},
	}
	for _, c := range cases {
		if got := CanEditOthersTx(c.role); got != c.editOthers {
			t.Errorf("%s 改/删他人交易：期望 %v，得到 %v", c.name, c.editOthers, got)
		}
		if got := CanManageMembers(c.role); got != c.manageMembers {
			t.Errorf("%s 管理成员：期望 %v，得到 %v", c.name, c.manageMembers, got)
		}
	}
}

// TestLimiter 邀请限流：窗口内超限后拒绝
func TestLimiter(t *testing.T) {
	l := NewLimiter()
	for i := 0; i < 5; i++ {
		if !l.Allow("k", 5, time.Minute) {
			t.Fatalf("第 %d 次应放行", i+1)
		}
	}
	if l.Allow("k", 5, time.Minute) {
		t.Fatal("第 6 次应被限流拒绝")
	}
}
