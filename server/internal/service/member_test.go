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
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestIPBucketNormalization 限流桶键必须按网段聚合（A5：IPv4 /24、IPv6 /64）。
//
// 口径与 auth.go 的 loginKey 一致：原样透传完整地址（旧行为）会让 IPv6 隐私扩展
// 下的地址轮转把 10 次/小时的配额摊薄成 0；不可解析值必须 fail-closed 进 unknown 桶。
func TestIPBucketNormalization(t *testing.T) {
	cases := map[string]string{
		"":                 "unknown",
		"   ":              "unknown",
		"garbage":          "unknown",
		"999.999.999.999":  "unknown",
		"192.168.1.10":     "192.168.1.0/24",
		" 10.0.0.2 ":       "10.0.0.0/24",
		"2001:db8::1":      "2001:db8::/64",
		"::ffff:127.0.0.1": "127.0.0.0/24", // IPv4-mapped 先 Unmap 再按 IPv4 口径聚合
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
	// 邀请"单次使用"的硬校验：条件更新后必须判 1 行。
	//
	// A9：标记已用的 UPDATE 不再手写条件串，而是复用 inviteUsableCond
	// （used_at = 0 AND revoked = 0 AND expires_at > ?）——单一实现，与撤销路径、
	// 过期判定同源。
	//
	// 断言用**标识符全等**（AST 级）而不是 strings.Contains：
	// 子串匹配会让 `inviteUsableCondWeak` 这类同名前缀常量双关命中
	// （reviewer-authz G-3），也能被注释里的同名文本满足。AST 里没有注释，
	// Ident.Name 又必须全等，因此只有真引用才能通过。
	const condIdent = "inviteUsableCond"
	if !serviceFuncReferencesIdent(t, src, "JoinViaInvite", condIdent) {
		t.Fatalf("JoinViaInvite 标记已用的 UPDATE 必须复用 %s（D-R08 / A9：单一条件口径）", condIdent)
	}
	cond, ok := serviceConstString(t, src, condIdent)
	if !ok {
		t.Fatalf("member.go 里找不到字符串常量 %s", condIdent)
	}
	for _, want := range []string{"used_at = 0", "revoked = 0", "expires_at > ?"} {
		if !strings.Contains(cond, want) {
			t.Fatalf("%s 缺少 %q：单一条件口径被拆散（值=%q）", condIdent, want, cond)
		}
	}
}

// serviceParseSource 把源码文本解析成 AST（不带注释）。
//
// 存在理由：源码级守卫若用 strings.Contains 匹配标识符，会被两类东西骗过——
// 同名前缀的另一个标识符（`inviteUsableCondWeak`）与注释/字符串里的同名文本。
// 解析成 AST 之后，注释不在树里、标识符必须**全等**才命中，这两个口子同时关闭。
func serviceParseSource(t *testing.T, src string) *ast.File {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "service_source.go", src, 0)
	if err != nil {
		t.Fatalf("解析源码失败：%v", err)
	}
	return f
}

// serviceFuncReferencesIdent 报告函数 fnName 的函数体内是否**精确**引用了标识符 ident。
func serviceFuncReferencesIdent(t *testing.T, src, fnName, ident string) bool {
	t.Helper()
	f := serviceParseSource(t, src)
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Body == nil || fn.Name.Name != fnName {
			continue
		}
		hit := false
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if id, isIdent := n.(*ast.Ident); isIdent && id.Name == ident {
				hit = true
			}
			return !hit
		})
		return hit
	}
	t.Fatalf("源码里找不到函数 %s", fnName)
	return false
}

// serviceConstString 返回包级字符串常量 name 的值（未找到返回 false）。
func serviceConstString(t *testing.T, src, name string) (string, bool) {
	t.Helper()
	f := serviceParseSource(t, src)
	val, found := "", false
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, s := range gd.Specs {
			vs, ok := s.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, n := range vs.Names {
				if n.Name != name || i >= len(vs.Values) {
					continue
				}
				lit, ok := vs.Values[i].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				if unq, err := strconv.Unquote(lit.Value); err == nil {
					val, found = unq, true
				}
			}
		}
	}
	return val, found
}

// TestServiceSourceGuardRejectsPrefixDoubleEntendre 源码守卫自身的突变测试（reviewer-authz G-3）。
//
// 断言的是"守卫必须拒绝哪些东西"，而不是生产代码本身：
//   - 同名前缀的另一个常量（inviteUsableCondWeak）不得算作对 inviteUsableCond 的引用；
//   - 注释里的同名文本、字符串字面量里的同名文本同样不得算引用；
//   - 但真正的引用必须仍被识别（守卫过严会让所有调用点误报）。
//
// 为什么值得单独写：这类守卫一旦退化成子串匹配，被它保护的不变量会集体失效，
// 而且**没有任何红灯**——只有给守卫本身喂"绕过写法"才能发现它已经失效。
func TestServiceSourceGuardRejectsPrefixDoubleEntendre(t *testing.T) {
	const base = `package service

// 注释里写 inviteUsableCond 不算引用

const inviteUsableCondWeak = "used_at = 0"

func hero() {
	_ = inviteUsableCondWeak
	_ = "inviteUsableCond"
}
`
	if serviceFuncReferencesIdent(t, base, "hero", "inviteUsableCond") {
		t.Fatal("前缀双关/注释/字符串字面量被当成了真引用：源码守卫必须做标识符全等匹配")
	}
	// 常量读取同理：只认常量名全等，不认前缀
	if _, ok := serviceConstString(t, base, "inviteUsableCond"); ok {
		t.Fatal("serviceConstString 用前缀匹配命中了 inviteUsableCondWeak")
	}
	// 对照：真正的引用与真正的常量定义必须被识别（防止守卫过严）
	real := `package service

const inviteUsableCond = "used_at = 0 AND revoked = 0 AND expires_at > ?"

func hero() {
	_ = inviteUsableCond
}
`
	if !serviceFuncReferencesIdent(t, real, "hero", "inviteUsableCond") {
		t.Fatal("真正的引用未被识别：AST 全等匹配把合法代码也拒了（守卫过严）")
	}
	if v, ok := serviceConstString(t, real, "inviteUsableCond"); !ok || !strings.Contains(v, "expires_at > ?") {
		t.Fatalf("未取到常量值：ok=%v val=%q", ok, v)
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
