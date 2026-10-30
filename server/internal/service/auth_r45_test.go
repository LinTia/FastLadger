package service

// GPT-R45：注册输入边界（用户名长度 + bcrypt 的 72 字节上限）。
//
// 两条边界的口径（写死在断言里，防止后来者悄悄改口径）：
//   - 用户名：按【字符】（Unicode 码点）计，1~64；另加 UTF-8【字节】≤255 的兜底；
//   - 口令：按【字节】计，>=8 且 <=72（bcrypt 硬上限，不是产品决策）。
//
// 边界用例取 71 / 72 / 73 字节，正好卡在临界两侧——这是要求里点名的边界。

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// TestR45PasswordBcryptByteBoundary 71 / 72 / 73 字节：72 可注册，73 必须 400 语义。
func TestR45PasswordBcryptByteBoundary(t *testing.T) {
	cases := []struct {
		name     string
		password string
		wantErr  error
	}{
		{"71 字节", strings.Repeat("a", 71), nil},
		{"72 字节（bcrypt 上限，必须放行）", strings.Repeat("a", 72), nil},
		{"73 字节（超上限，必须拒绝）", strings.Repeat("a", 73), ErrPasswordTooLong},
		{"74 字节", strings.Repeat("a", 74), ErrPasswordTooLong},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, hash, err := normalizeAndHash("alice", c.password)
			if c.wantErr == nil {
				if err != nil {
					t.Fatalf("%d 字节口令应被接受，实际 err=%v", len(c.password), err)
				}
				if !strings.HasPrefix(hash, "$2") {
					t.Fatalf("应产出 bcrypt 哈希，实际 %q", hash)
				}
				return
			}
			if !errors.Is(err, c.wantErr) {
				t.Fatalf("%d 字节口令应返回 %v，实际 %v", len(c.password), c.wantErr, err)
			}
			// 关键：绝不能是 500 语义（不能是"内部错误"）——必须是可识别的客户端错误，
			// 且不能是"哈希失败"这类服务端故障的措辞。
			if errors.Is(err, ErrInvalidArgument) && !errors.Is(err, ErrPasswordTooLong) {
				t.Fatalf("口令超长应落在 ErrPasswordTooLong，实际 %v", err)
			}
		})
	}
}

// TestR45PasswordLimitIsBytesNotRunes 上限按【字节】计：24 个汉字 = 72 字节放行，
// 25 个汉字 = 75 字节拒绝（只有 25 个字符，按字符计会误放行）。
func TestR45PasswordLimitIsBytesNotRunes(t *testing.T) {
	ok24 := strings.Repeat("汉", 24) // 72 字节
	if got := len(ok24); got != 72 {
		t.Fatalf("用例构造错误：应为 72 字节，实际 %d", got)
	}
	if _, _, err := normalizeAndHash("alice", ok24); err != nil {
		t.Fatalf("72 字节（24 个汉字）的口令必须被接受，实际 %v", err)
	}
	bad25 := strings.Repeat("汉", 25) // 75 字节 / 25 字符
	if got := len(bad25); got != 75 {
		t.Fatalf("用例构造错误：应为 75 字节，实际 %d", got)
	}
	if _, _, err := normalizeAndHash("alice", bad25); !errors.Is(err, ErrPasswordTooLong) {
		t.Fatalf("75 字节（25 个汉字）的口令必须因【字节】超限被拒，实际 %v", err)
	}
}

// TestR45UsernameBoundary 用户名边界：字符口径为主，字节口径兜底。
func TestR45UsernameBoundary(t *testing.T) {
	cases := []struct {
		name     string
		username string
		wantErr  error
	}{
		{"1 个字符", "a", nil},
		{"64 个 ASCII 字符", strings.Repeat("a", 64), nil},
		{"65 个 ASCII 字符", strings.Repeat("a", 65), ErrUsernameTooLong},
		{"64 个汉字（192 字节，字符数合法）", strings.Repeat("汉", 64), nil},
		{"65 个汉字（字符数超限）", strings.Repeat("汉", 65), ErrUsernameTooLong},
		// 字节兜底：64 个 emoji = 256 字节，字符数合法但字节数越界
		{"64 个 emoji（256 字节）", strings.Repeat("😀", 64), ErrUsernameTooLong},
		{"63 个 emoji（252 字节，两条都合规）", strings.Repeat("😀", 63), nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, err := normalizeAndHash(c.username, "12345678")
			if c.wantErr == nil {
				if err != nil {
					t.Fatalf("用户名 %q 应被接受，实际 %v", c.username, err)
				}
				return
			}
			if !errors.Is(err, c.wantErr) {
				t.Fatalf("用户名 %q 应返回 %v，实际 %v", c.username, c.wantErr, err)
			}
		})
	}
}

// TestR45UsernameIsTrimmedBeforeLengthCheck 先 trim 再判长度：
// 前后空白不该把合法用户名挤成"超长"，也不该让全空白绕过非空校验。
func TestR45UsernameIsTrimmedBeforeLengthCheck(t *testing.T) {
	padded := "  " + strings.Repeat("a", 64) + "  "
	uname, _, err := normalizeAndHash(padded, "12345678")
	if err != nil {
		t.Fatalf("trim 后 64 字符应合法，实际 %v", err)
	}
	if uname != strings.Repeat("a", 64) {
		t.Fatalf("应返回 trim 后的用户名，实际 %q", uname)
	}
	if _, _, err := normalizeAndHash("   ", "12345678"); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("全空白用户名应返回 ErrInvalidArgument（400），实际 %v", err)
	}
}

// TestR45ValidateUsernameRejectsLegacyLongName 直接覆盖 validateUsername 的两条口径，
// 防止后来者只留字符口径而漏掉字节兜底（或反之）。
func TestR45ValidateUsernameRejectsLegacyLongName(t *testing.T) {
	if err := validateUsername(strings.Repeat("字", maxUsernameRunes)); err != nil {
		t.Fatalf("%d 个汉字应合法，实际 %v", maxUsernameRunes, err)
	}
	if err := validateUsername(strings.Repeat("字", maxUsernameRunes+1)); !errors.Is(err, ErrUsernameTooLong) {
		t.Fatalf("超过 %d 个字符必须被拒，实际 %v", maxUsernameRunes, err)
	}
	// 字节兜底独立生效：64 个 4 字节字符 = 256 字节 > maxUsernameBytes(255)
	emoji := strings.Repeat("😀", maxUsernameRunes)
	if got := len(emoji); got != 256 {
		t.Fatalf("用例构造错误：应为 256 字节，实际 %d", got)
	}
	if err := validateUsername(emoji); !errors.Is(err, ErrUsernameTooLong) {
		t.Fatalf("字节数兜底必须生效（256 > %d），实际 %v", maxUsernameBytes, err)
	}
}

// TestR45RegisterRejectsInvalidInputWithoutPersisting 真实 SQLite：
// 非法输入必须【零落库】——校验发生在开事务之前，库里一条 users 记录都不该出现。
func TestR45RegisterRejectsInvalidInputWithoutPersisting(t *testing.T) {
	requireSQLite(t)
	e := newTestEnv(t)
	svc := NewAuthService(e.engine, testJWTSecret, time.Hour)

	// 库里已有 owner（seedBook 建的），先记下基线
	var before int64
	if _, err := e.engine.SQL("SELECT COUNT(1) FROM users").Get(&before); err != nil {
		t.Fatalf("统计用户失败：%v", err)
	}

	bad := []struct {
		name     string
		username string
		password string
		want     error
	}{
		{"口令 73 字节", "bob", strings.Repeat("a", 73), ErrPasswordTooLong},
		{"用户名 65 字符", strings.Repeat("a", 65), "12345678", ErrUsernameTooLong},
		{"用户名 64 个 emoji", strings.Repeat("😀", 64), "12345678", ErrUsernameTooLong},
		{"口令过短", "bob", "1234567", ErrWeakPassword},
	}
	for _, c := range bad {
		t.Run(c.name, func(t *testing.T) {
			u, err := svc.Register(c.username, c.password)
			if u != nil {
				t.Fatalf("失败路径不得返回用户对象，实际 %+v", u)
			}
			if !errors.Is(err, c.want) {
				t.Fatalf("应返回 %v，实际 %v", c.want, err)
			}
			var after int64
			if _, err := e.engine.SQL("SELECT COUNT(1) FROM users").Get(&after); err != nil {
				t.Fatalf("统计用户失败：%v", err)
			}
			if after != before {
				t.Fatalf("非法输入不得落库：用户数从 %d 变成 %d", before, after)
			}
		})
	}

	// 合法输入仍可正常注册（收紧校验不得误伤）
	u, err := svc.Register("carol", strings.Repeat("a", 72))
	if err != nil {
		t.Fatalf("72 字节口令应能正常注册，实际 %v", err)
	}
	if u == nil || u.Username != "carol" {
		t.Fatalf("注册结果不符：%+v", u)
	}
	// 之后同名注册应命中唯一性冲突，而不是绕过校验重复插入
	if _, err := svc.Register("carol", strings.Repeat("a", 72)); !errors.Is(err, ErrUserExists) {
		t.Fatalf("重复用户名应返回 ErrUserExists，实际 %v", err)
	}
}

// TestR45RegisterRejectsReservedShape 附加边界：用户名内嵌空白不 trim（只 trim 首尾），
// 空口令与超长口令都不允许。
func TestR45RegisterRejectsReservedShape(t *testing.T) {
	if _, _, err := normalizeAndHash("alice", ""); !errors.Is(err, ErrWeakPassword) {
		t.Fatalf("空口令应返回 ErrWeakPassword，实际 %v", err)
	}
	// 含空格的口令本身是合法字节，长度校验不受影响（不额外收紧既有行为）
	if _, _, err := normalizeAndHash("alice", "a b c d e f g"); err != nil {
		t.Fatalf("含空格的 13 字节口令应合法，实际 %v", err)
	}
}
