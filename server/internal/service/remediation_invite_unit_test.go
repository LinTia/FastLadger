package service

// 邀请码生命周期缺陷（GPT-R41 邮箱绑定 / R40 撤销 / R42 审计 / R43 已删账本）
// 的**可本地执行**回归测试（无 CGO、无真实 SQLite）。
//
// 本机没有 gcc，mattn/go-sqlite3 只能编译不能运行，因此这里只覆盖【不依赖数据库】
// 的那部分证据：
//   - 邮箱规范化与绑定取值规则的纯函数行为（矩阵 + 反例）；
//   - 源码结构断言：绑定字段必须真的被读写，"看似可用实则无效"不得重现。
//
// 真实数据行为（兑换闭环、并发竞争、审计行数）见
// remediation_invite_integration_test.go（需 CGO，由 CI 执行）。本文件不冒充那部分证据。

import (
	"errors"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// GPT-R41：邮箱规范化
// ---------------------------------------------------------------------------

// TestR41EmailNormalizationMatrix 邮箱必须按"去首尾空白 + 转小写"的单一口径收敛。
//
// 这些写法在现实里是同一个邮箱：邮件服务商普遍把 local-part 当大小写不敏感处理。
// 若规范化缺失，会出现"签发端写 Alice@x.com、兑换端填 alice@x.com 判为不匹配"，
// 也就是本 Finding 描述的"看似可用实则无效的字段"。
func TestR41EmailNormalizationMatrix(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"   ", ""},
		{"Alice@Example.COM", "alice@example.com"},
		{"\tBob.Smith@Mail.example.org\n", "bob.smith@mail.example.org"},
		{" already@lower.dev ", "already@lower.dev"},
		{"ALICE@EXAMPLE.COM", "alice@example.com"},
	}
	for _, c := range cases {
		if got := normalizeBindingEmail(c.in); got != c.want {
			t.Errorf("normalizeBindingEmail(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	// 幂等性：规范化后的值再过一次函数必须不变（否则"逐次规范化"会漂移）
	for _, c := range cases {
		once := normalizeBindingEmail(c.in)
		if twice := normalizeBindingEmail(once); twice != once {
			t.Errorf("规范化不幂等：%q → %q → %q", c.in, once, twice)
		}
	}
}

// TestR41EmailValidationRejectsMalformed 结构非法的邮箱必须被拒。
//
// 为什么要校验：绑定一个【根本无法完成比对】的字符串（拼错的 "alice#x.com"、
// 手滑粘贴进来的多段文本），会让邀请变成"绑了但谁也兑不了"，这正是本 Finding 的病根
// ——字段可用但无效。宁可签发失败并立刻暴露，也不要留下一张废邀请。
func TestR41EmailValidationRejectsMalformed(t *testing.T) {
	bad := []string{
		"alice",               // 缺 @ 与域名
		"alice@",              // 缺域名
		"@example.com",        // 缺 local-part
		"alice@@example.com",  // 多个 @
		"alice@exam ple.com",  // 域名含空格
		"al ice@example.com",  // local-part 含空格
		"alice@example",       // 域名不带点
		"alice@.com",          // 域名以点开头
		"alice@example.com.",  // 域名以点结尾
		"alice\n@example.com", // 含换行（粘贴污染）
	}
	for _, raw := range bad {
		if err := validateBindingEmail(normalizeBindingEmail(raw)); err == nil {
			t.Errorf("邮箱 %q 应被拒绝，实际通过", raw)
		}
	}
	good := []string{"", "a@b.c", "alice@example.com", "alice+tag@sub.example.com", "张三@example.com"}
	for _, raw := range good {
		if err := validateBindingEmail(normalizeBindingEmail(raw)); err != nil {
			t.Errorf("邮箱 %q 应被接受，实际报错：%v", raw, err)
		}
	}
}

// TestR41BindingRuleIsExclusive 绑定维度取值规则：允许"都不绑 / 只绑用户名 / 只绑邮箱"，
// 【禁止】同时绑定两者并静默取舍。
func TestR41BindingRuleIsExclusive(t *testing.T) {
	// ① 都不绑
	u, e, err := resolveInviteBinding("", "")
	if err != nil || u != "" || e != "" {
		t.Fatalf("都不绑应得到 (空, 空, nil)，实际 (%q, %q, %v)", u, e, err)
	}
	// ② 只绑用户名：用户名大小写【保留】原样（用户名语义是 EqualFold 比较，不是小写存储）
	u, e, err = resolveInviteBinding("  Alice  ", "")
	if err != nil || u != "Alice" || e != "" {
		t.Fatalf("只绑用户名应得到 (\"Alice\", \"\", nil)，实际 (%q, %q, %v)", u, e, err)
	}
	// ③ 只绑邮箱：规范化后落库
	u, e, err = resolveInviteBinding("", "  Alice@Example.COM ")
	if err != nil || u != "" || e != "alice@example.com" {
		t.Fatalf("只绑邮箱应得到 (\"\", \"alice@example.com\", nil)，实际 (%q, %q, %v)", u, e, err)
	}
	// ④ 两者都绑 → 明确拒绝，不得取其一
	u, e, err = resolveInviteBinding("Alice", "alice@example.com")
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("同时绑定两者应返回 ErrInvalidArgument，实际 %v", err)
	}
	if u != "" || e != "" {
		t.Fatalf("拒绝时不得返回可用的取值，实际 (%q, %q)", u, e)
	}
	// ⑤ 邮箱非法 → 拒绝（即使用户名合法）
	if _, _, err := resolveInviteBinding("Alice", "not-an-email"); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("非法邮箱应返回 ErrInvalidArgument，实际 %v", err)
	}
}

// TestR41EmailBindingWiredIntoJoinSource 兑换端必须真的比较 BoundEmail，
// 且只能与【规范化后】的值比较——源码守卫，防止字段再次退化成"能存不用"。
func TestR41EmailBindingWiredIntoJoinSource(t *testing.T) {
	src := readServiceSource(t, "member.go")
	body := src[strings.Index(src, "func (s *MemberService) JoinViaInvite"):]
	body = body[:minStringEnd(body, 0)]

	if !strings.Contains(body, "inv.BoundEmail != \"\"") {
		t.Fatal("JoinViaInvite 未校验 BoundEmail：邀请绑了邮箱却人人可兑（GPT-R41 回归）")
	}
	if !strings.Contains(body, "normalizeBindingEmail(email)") {
		t.Fatal("JoinViaInvite 必须用 normalizeBindingEmail 规范化入参邮箱后比较，否则大小写差异会误判")
	}
	if !strings.Contains(body, "ErrInviteBoundMismatch") {
		t.Fatal("邮箱不匹配必须返回 ErrInviteBoundMismatch，不得静默放行")
	}
	// 兼容守卫：用户名绑定必须继续生效（不能被邮箱改造覆盖掉）
	if !strings.Contains(body, "inv.BoundUsername != \"\"") {
		t.Fatal("用户名绑定校验不得被邮箱改造移除")
	}
	// 邮箱占用检查不得被删（否则唯一索引冲突会退化成 500）
	if !strings.Contains(body, "lower(email) = ?") {
		t.Fatal("缺少邮箱占用检查：唯一索引冲突会被映射成 500")
	}
	// 邮箱必须落到用户档案，否则"绑定闭环"只有一半
	if !strings.Contains(body, "Cols(\"email\")") {
		t.Fatal("邮箱必须写入用户档案，否则校验通过但档案无邮箱（半截状态）")
	}
}

// TestR41BoundEmailAcceptedByCreateInviteSource 签发端必须同时接收并落库 boundEmail。
func TestR41BoundEmailAcceptedByCreateInviteSource(t *testing.T) {
	src := readServiceSource(t, "member.go")
	body := src[strings.Index(src, "func (s *MemberService) CreateInvite"):]
	body = body[:minStringEnd(body, 0)]
	if !strings.Contains(body, "resolveInviteBinding") {
		t.Fatal("CreateInvite 未走 resolveInviteBinding：绑定维度就没有统一取值规则")
	}
	if !strings.Contains(body, "BoundEmail:") {
		t.Fatal("CreateInvite 未把绑定邮箱落到 invites.bound_email（GPT-R41 回归）")
	}
	if !strings.Contains(body, "boundEmail") {
		t.Fatal("CreateInvite 缺少 boundEmail 入参")
	}
}

// HTTP 层的 boundEmail / email 入参与路由注册由 internal/api/remediation_invite_api_test.go
// 校验（service 包不能反向 import api，故按包拆分）。
