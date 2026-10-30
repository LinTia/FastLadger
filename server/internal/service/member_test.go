package service

// 成员 / 邀请服务的纯逻辑回归测试（无 DB 依赖，CGO 关闭环境下可跑）。
//
// 覆盖 D-R07 / D-R08 / D-R09 / WB-R01 的**可静态验证部分**：
//   - WB-R01：限流按 IP 分桶（不再全局共享一个计数器）、空 IP 归入 unknown；
//   - D-R07/D-R08：JoinViaInvite 必须接收 bookID 与 clientIP（签名守卫，防止退回全局 key）；
//   - D-R09：member.go 的成员写操作必须核对 RowsAffected==1（源码级守卫，防止退回丢弃计数）；
//   - 重构后 normalizeAndHash 的校验行为不退化。
//
// 真实事务回滚 / 并发竞争仍需 CGO 环境下的集成测试，本文件不冒充那部分证据。

import (
	"os"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestIPBucketNormalization(t *testing.T) {
	cases := map[string]string{
		"":                 "unknown",
		"   ":              "unknown",
		"192.168.1.10":     "192.168.1.10",
		" 10.0.0.2 ":       "10.0.0.2",
		"2001:db8::1":      "2001:db8::1",
		"::ffff:127.0.0.1": "::ffff:127.0.0.1",
	}
	for in, want := range cases {
		if got := ipBucket(in); got != want {
			t.Fatalf("ipBucket(%q) = %q, want %q", in, got, want)
		}
	}
}

// WB-R01 核心回归：不同 IP 各自独立计数，一个 IP 耗尽不能让其他人被限流。
func TestInviteJoinRateLimitIsPerIP(t *testing.T) {
	l := NewLimiter()
	const perIP = 10

	for i := 0; i < perIP; i++ {
		if !l.Allow("invite:join:ip:"+ipBucket("1.1.1.1"), perIP, time.Hour) {
			t.Fatalf("第 %d 次应被允许", i+1)
		}
	}
	if l.Allow("invite:join:ip:"+ipBucket("1.1.1.1"), perIP, time.Hour) {
		t.Fatal("同一 IP 第 11 次应被限流")
	}
	// 关键断言：另一个 IP 仍可用满额度（原全局 key 实现下这里会一起被限流）
	for i := 0; i < perIP; i++ {
		if !l.Allow("invite:join:ip:"+ipBucket("2.2.2.2"), perIP, time.Hour) {
			t.Fatalf("另一个 IP 的第 %d 次不应被前一个 IP 影响", i+1)
		}
	}
}

// 空 IP 不能绕过限流：应统一归入 unknown 桶并受限。
func TestInviteJoinRateLimitEmptyIPNotBypass(t *testing.T) {
	l := NewLimiter()
	const perIP = 10
	for i := 0; i < perIP; i++ {
		if !l.Allow("invite:join:ip:"+ipBucket(""), perIP, time.Hour) {
			t.Fatalf("第 %d 次应被允许", i+1)
		}
	}
	if l.Allow("invite:join:ip:"+ipBucket("   "), perIP, time.Hour) {
		t.Fatal("空白 IP 与空 IP 应共用 unknown 桶，第 11 次必须被限流")
	}
}

// D-R07 / D-R08 / WB-R01 / GPT-R41 签名守卫：
// JoinViaInvite 必须接收 (bookID, code, username, email, password, clientIP)。
//
//   - 若被改回 3 参数（无 bookID / 无 IP），说明限流又退回全局 key 或丢失账本校验；
//   - 若 email 被移除，说明 GPT-R41 的邮箱绑定闭环又被拆掉（邀请能绑邮箱却无从校验），
//     这两类退化都必须让本用例失败。
//
// 逐参数断言而非只数个数：个数对了但顺序错了（例如 email 与 password 互换）
// 同样是契约破坏，只是不会编译报错。
func TestJoinViaInviteSignatureRequiresBookAndIP(t *testing.T) {
	mt, ok := reflect.TypeOf(&MemberService{}).MethodByName("JoinViaInvite")
	if !ok {
		t.Fatal("MemberService 缺少 JoinViaInvite 方法")
	}
	ft := mt.Func.Type()
	if ft.NumIn() != 7 { // receiver + 6 params（GPT-R41 起新增 email）
		t.Fatalf("JoinViaInvite 入参个数 = %d，期望 7（receiver + bookID/code/username/email/password/clientIP）", ft.NumIn())
	}
	want := []reflect.Kind{
		reflect.Int64,  // bookID
		reflect.String, // code
		reflect.String, // username
		reflect.String, // email（GPT-R41）
		reflect.String, // password
		reflect.String, // clientIP
	}
	for i, kind := range want {
		if got := ft.In(i + 1).Kind(); got != kind {
			t.Fatalf("JoinViaInvite 第 %d 个参数应为 %s，实际 %s", i+1, kind, got)
		}
	}
}

// D-R09 源码级守卫：成员写操作必须核对影响行数，不能再用 `_, err = ...Update(...)` 丢弃计数。
func TestMemberWritesCheckRowsAffected(t *testing.T) {
	src := readServiceSource(t, "member.go")

	if strings.Contains(src, "_, err = s.engine.Where") {
		t.Fatal("member.go 仍存在丢弃影响行数的写法 `_, err = s.engine.Where(...)`（D-R09 回归）")
	}
	if got := strings.Count(src, "if n != 1 {"); got < 2 {
		t.Fatalf("member.go 中 `if n != 1 {` 出现 %d 次，ChangeRole 与 Remove 应各有一处", got)
	}
	// 邀请"单次使用"的硬校验：条件更新后必须判 1 行
	if !strings.Contains(src, "AND used_at = 0") {
		t.Fatal("member.go 的邀请标记更新必须带 `AND used_at = 0` 条件（D-R08）")
	}
}

// 回归：重构后 normalizeAndHash 的校验行为不变（auth.Register 与 JoinViaInvite 共用）。
func TestNormalizeAndHashValidation(t *testing.T) {
	if _, _, err := normalizeAndHash("   ", "12345678"); err == nil {
		t.Fatal("空用户名必须被拒绝")
	}
	if _, _, err := normalizeAndHash("alice", "1234567"); err == nil {
		t.Fatal("7 位口令必须被拒绝（minPasswordLen=8）")
	}
	uname, hash, err := normalizeAndHash("  alice  ", "12345678")
	if err != nil {
		t.Fatalf("合法输入不应报错：%v", err)
	}
	if uname != "alice" {
		t.Fatalf("用户名应被 trim，实际 %q", uname)
	}
	if !strings.HasPrefix(hash, "$2") || len(hash) < 50 {
		t.Fatalf("应产出 bcrypt 哈希，实际 %q", hash)
	}
}

// readServiceSource 读取本包内的源文件，用于源码级回归守卫。
func readServiceSource(t *testing.T, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("无法定位测试文件路径")
	}
	dir := file[:strings.LastIndex(file, "/")+1]
	b, err := os.ReadFile(dir + name)
	if err != nil {
		// Windows 路径分隔符兜底
		dir = file[:strings.LastIndex(file, "\\")+1]
		b, err = os.ReadFile(dir + name)
		if err != nil {
			t.Fatalf("读取 %s 失败：%v", name, err)
		}
	}
	return string(b)
}
