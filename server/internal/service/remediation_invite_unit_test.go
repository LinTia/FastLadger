package service

// 邀请码生命周期缺陷（GPT-R41 邮箱绑定 / R40 撤销 / R42 审计 / R43 已删账本）
// 的**可本地执行**回归测试（无 CGO、无真实 SQLite）。
//
// 本机没有 gcc，mattn/go-sqlite3 只能编译不能运行，因此这里只覆盖【不依赖数据库】
// 的那部分证据：
//   - 邮箱规范化与绑定取值规则的纯函数行为（矩阵 + 反例）；
//   - 源码结构断言：绑定字段必须真的被读写，「看似可用实则无效」不得重现；
//   - 邀请审计 payload 的最小化约定（audit policy 的结构性部分）。
//
// 真实数据行为（兑换闭环、并发竞争、审计行数）见
// remediation_invite_integration_test.go（需 CGO，由 CI 执行）。本文件不冒充那部分证据。

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"fastledger/internal/model"
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
	if got := strings.Count(body, "inviteReasonBoundMismatch"); got != 2 {
		t.Fatalf("用户名与邮箱两个绑定分支都必须走 inviteReasonBoundMismatch（实际 %d 处）", got)
	}
	// 分支必须真的返回"信息不匹配"错误（经由 inviteFailed → inviteRejectError）
	if got := strings.Count(body, "inviteFailed("); got < 4 {
		t.Fatalf("JoinViaInvite 的失败分支必须统一走 inviteFailed（得到 %d 处）", got)
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

// ---------------------------------------------------------------------------
// GPT-R40：邀请撤销（源码结构断言——本地可验证，无需数据库）
// ---------------------------------------------------------------------------

// TestR40RevokeUsesConditionalUpdateWithRowsAffected 撤销必须是"条件 UPDATE + 影响行数"，
// 不得退回"先 SELECT 再 UPDATE"。
//
// 为什么是源码断言而不是跑一次：真实 SQLite 在 MaxOpenConns=1 下会把两个事务
// 串行化，并发窗口很难靠 goroutine 调度稳定复现；而"是否把状态判定写进 WHERE"
// 是【确定性】的。真实端的行为验证（同样的数据竞争）在
// remediation_invite_integration_test.go 的 TestR40RevokeAndRedeemRaceExactlyOneWins。
func TestR40RevokeUsesConditionalUpdateWithRowsAffected(t *testing.T) {
	src := readServiceSource(t, "member.go")
	body := src[strings.Index(src, "func (s *MemberService) RevokeInvite"):]
	body = body[:minStringEnd(body, 0)]

	// ① 四个状态条件必须写进同一个 WHERE（少一个都会在并发下漏放行）
	for _, cond := range []string{
		"id = ?", "book_id = ?", "used_at = 0", "revoked = 0", "expires_at > ?",
	} {
		if !strings.Contains(body, cond) {
			t.Fatalf("撤销的 WHERE 缺少 %q：未经条件约束的 UPDATE 会让已用/已撤销的邀请再次被改写", cond)
		}
	}
	// ② 必须核对影响行数
	if !strings.Contains(body, "if n != 1 {") {
		t.Fatal("RevokeInvite 必须核对 RowsAffected==1（D-R09 同源要求）")
	}
	// ③ 权限必须在 Begin 之后读到（GPT-R28）
	beginLine := strings.Index(body, "sess.Begin()")
	roleLine := strings.Index(body, "roleOfTx(sess")
	if roleLine < 0 || beginLine < 0 || beginLine > roleLine {
		t.Fatalf("RevokeInvite 的顺序应为 Begin → roleOfTx，实际 begin=%d role=%d", beginLine, roleLine)
	}
	// ④ 不得出现事务外的 RoleOf（engine 版）
	if strings.Contains(body, "RoleOf(s.engine") || strings.Contains(body, "s.RoleOf(") {
		t.Fatal("RevokeInvite 不得使用事务外的 RoleOf（GPT-R28 回归）")
	}
	// ⑤ 兑换侧必须带 revoked = 0，才能与撤销互斥
	join := src[strings.Index(src, "func (s *MemberService) JoinViaInvite"):]
	join = join[:minStringEnd(join, 0)]
	if !strings.Contains(join, "used_at = 0 AND revoked = 0") {
		t.Fatal("JoinViaInvite 标记已用的 UPDATE 必须带 revoked = 0，否则撤销与兑换可以双成功（GPT-R40）")
	}
}

// TestR40InviteRejectReasonIsSingleSource 所有邀请路径只能用一个 reason 判定函数，
// 不得各写一套 `used_at > 0 || revoked || expires_at <= now`（两处口径会各自漂移）。
func TestR40InviteRejectReasonIsSingleSource(t *testing.T) {
	// 纯函数口径矩阵：与 production code 的判定顺序一一对应
	now := int64(1793260800)
	live := &model.Invite{Id: 1, ExpiresAt: now + 10}
	cases := []struct {
		name   string
		has    bool
		inv    *model.Invite
		reason string
	}{
		{"不存在", false, live, inviteReasonNotFound},
		{"已使用", true, &model.Invite{Id: 1, UsedAt: now, ExpiresAt: now + 10}, inviteReasonAlreadyUsed},
		{"已撤销", true, &model.Invite{Id: 1, Revoked: true, ExpiresAt: now + 10}, inviteReasonRevoked},
		{"过期", true, &model.Invite{Id: 1, ExpiresAt: now - 1}, inviteReasonExpired},
		{"可用", true, live, inviteReasonNone},
	}
	for _, c := range cases {
		if got := inviteRejectReason(c.has, c.inv, now); got != c.reason {
			t.Errorf("%s：reason=%q，期望 %q", c.name, got, c.reason)
		}
	}
	// 优先级：既过期又已用时优先报"已用"（更贴近调用方的操作意图）
	if got := inviteRejectReason(true, &model.Invite{UsedAt: now, ExpiresAt: now - 1}, now); got != inviteReasonAlreadyUsed {
		t.Errorf("同时已用且过期应报 %q，实际 %q", inviteReasonAlreadyUsed, got)
	}
	// reason → error 必须是既有错误树的成员（否则会漏进 500）。
	//
	// 两类归因：状态类（不存在/已用/已撤销/过期）继承 ErrInviteInvalid；
	// 绑定类（不匹配/邮箱被占用）继承 ErrInviteBoundMismatch —— 后者对外同样是 400，
	// 但不能提示"邮箱已被注册"（未认证入口禁止枚举，见 ErrInviteEmailUnavailable）。
	for _, c := range []struct {
		reason string
		base   error
		anchor error
	}{
		{inviteReasonNotFound, ErrInviteNotFound, ErrInviteInvalid},
		{inviteReasonAlreadyUsed, ErrInviteAlreadyUsed, ErrInviteInvalid},
		{inviteReasonRevoked, ErrInviteAlreadyRevoked, ErrInviteInvalid},
		{inviteReasonExpired, ErrInviteExpired, ErrInviteInvalid},
		{inviteReasonBoundMismatch, ErrInviteBoundMismatch, ErrInviteBoundMismatch},
		{inviteReasonEmailTaken, ErrInviteEmailUnavailable, ErrInviteBoundMismatch},
	} {
		err := inviteRejectError(c.reason)
		if !errors.Is(err, c.base) {
			t.Errorf("%s 的翻译错误应为 %v，实际 %v", c.reason, c.base, err)
		}
		if !errors.Is(err, c.anchor) {
			t.Errorf("%s 的翻译错误必须继承 %v（否则 mapSvcErr 会漏判成 500）", c.reason, c.anchor)
		}
	}
	// 任何 reason 都不能默认落到一个未知错误上
	if got := inviteRejectError("不存在的 reason"); !errors.Is(got, ErrInviteInvalid) {
		t.Errorf("未知 reason 必须兜底为 ErrInviteInvalid，实际 %v", got)
	}
}

// ---------------------------------------------------------------------------
// GPT-R42：邀请生命周期审计（payload 最小化约定，本地可验证部分）
// ---------------------------------------------------------------------------

// inviteAuditAllowedKeys 审计 payload 允许出现的字段白名单。
//
// 白名单而不是黑名单：以后谁想往 payload 里加东西，必须【显式】改这张表，
// 顺带迫使他在同一个 commit 里说明"为什么这个字段不是敏感信息"。
var inviteAuditAllowedKeys = []string{
	"inviteEvent", "result", "reason", "inviteId", "role",
	"boundTarget", "expiresAt", "joinerUid", "joinerUsername", "clientIP",
}

// TestR42AuditPayloadNeverContainsSecrets 审计里绝不能出现邀请码 / 口令 / 邮箱本体。
//
// 邀请码是入户凭证，价值等同口令：一旦落进审计表，任何能读库的人都能直接入户。
// 掩码也不安全——记前 4 位会把暴力搜索空间缩小若干数量级，还免费给出命中确认信号。
func TestR42AuditPayloadNeverContainsSecrets(t *testing.T) {
	const fakeCode = "9f3c1d77a2b84e5f9f3c1d77a2b84e5f"
	secrets := []string{fakeCode, fakeCode[:4], "alice@example.com", "p@ssw0rd-very-secret"}

	// 结构守卫：payload 的入参结构体本身就不许有能装这些玩意的字段
	ft := reflect.TypeOf(inviteAuditFields{})
	for i := 0; i < ft.NumField(); i++ {
		name := strings.ToLower(ft.Field(i).Name)
		for _, kw := range []string{"code", "mail", "password", "passwd", "secret", "token"} {
			if strings.Contains(name, kw) {
				t.Errorf("inviteAuditFields 含有可疑字段 %s：审计不得承载凭证类信息", ft.Field(i).Name)
			}
		}
	}

	// 行为守卫：成功 / 失败两种 payload 都不得夹带秘密，且字段不越白名单
	for _, c := range []struct {
		name   string
		result string
		reason string
		f      inviteAuditFields
	}{
		{"成功兑换", inviteResultAccepted, inviteReasonNone, inviteAuditFields{
			InviteID: 7, Role: model.RoleMember, BoundTarget: inviteBoundEmail,
			JoinerUid: 42, JoinerUsername: "alice", ClientIP: "10.0.0.7",
		}},
		{"失败尝试", inviteResultRejected, inviteReasonExpired, inviteAuditFields{
			InviteID: 7, Role: 0, ClientIP: "10.0.0.7",
		}},
	} {
		blob, err := json.Marshal(inviteAuditPayload(inviteEventRedeem, c.result, c.reason, c.f))
		if err != nil {
			t.Fatalf("%s：payload 序列化失败：%v", c.name, err)
		}
		s := string(blob)
		for _, secret := range secrets {
			if strings.Contains(s, secret) {
				t.Errorf("%s：审计 payload 泄漏敏感值 %q：%s", c.name, secret, s)
			}
		}
		var keys map[string]interface{}
		if err := json.Unmarshal(blob, &keys); err != nil {
			t.Fatalf("%s：payload 不是合法 JSON 对象：%v", c.name, err)
		}
		allowed := map[string]bool{}
		for _, k := range inviteAuditAllowedKeys {
			allowed[k] = true
		}
		for k := range keys {
			if !allowed[k] {
				t.Errorf("%s：payload 出现白名单外的字段 %q（请先论证它不敏感）：%s", c.name, k, s)
			}
		}
	}
}

// TestR42PayloadDiscriminatesResultAndReason 成功与失败共用一个结构，
// 必须靠 result / reason 区分，不能靠"某个字段有没有出现"这种隐式约定。
func TestR42PayloadDiscriminatesResultAndReason(t *testing.T) {
	accepted := inviteAuditPayload(inviteEventRedeem, inviteResultAccepted, inviteReasonNone,
		inviteAuditFields{InviteID: 1, JoinerUid: 42, ClientIP: "10.0.0.7"})
	if accepted["result"] != inviteResultAccepted {
		t.Fatalf("成功事件 result 应为 accepted，实际 %v", accepted["result"])
	}
	if _, has := accepted["reason"]; has {
		t.Fatalf("成功事件不应带 reason：%v", accepted)
	}
	// 成功事件已有 actor_uid / joinerUid 可追溯，不再复制 IP（最小收集）
	if _, has := accepted["clientIP"]; has {
		t.Fatalf("成功事件不应落 IP：%v", accepted)
	}

	rejected := inviteAuditPayload(inviteEventRedeem, inviteResultRejected, inviteReasonExpired,
		inviteAuditFields{InviteID: 1, JoinerUid: 42, JoinerUsername: "alice", ClientIP: "10.0.0.7"})
	if rejected["result"] != inviteResultRejected || rejected["reason"] != inviteReasonExpired {
		t.Fatalf("失败事件应带 result=rejected 与 reason=%s，实际 %v", inviteReasonExpired, rejected)
	}
	// 失败事件不得记录本次尝试用到的身份（否则审计表自己变成枚举字典）
	for _, banned := range []string{"joinerUid", "joinerUsername"} {
		if _, has := rejected[banned]; has {
			t.Errorf("失败审计不得携带 %s：%v", banned, rejected)
		}
	}
	if rejected["clientIP"] != "10.0.0.7" {
		t.Fatalf("失败事件应保留 clientIP 作为唯一取证线索，实际 %v", rejected)
	}
}

// TestR42AuditActionReusesExistingEnum 邀请事件不得新增枚举值：
// 《枚举字典》是唯一权威，本次整改不扩展它，因此动作只能落在 create / delete 上。
func TestR42AuditActionReusesExistingEnum(t *testing.T) {
	cases := map[string]string{
		inviteEventCreate: model.AuditActionCreate,
		inviteEventRedeem: model.AuditActionCreate,
		inviteEventRevoke: model.AuditActionDelete,
	}
	for event, want := range cases {
		if got := inviteAuditAction(event); got != want {
			t.Errorf("事件 %s 的审计动作应为 %q，实际 %q", event, want, got)
		}
	}
	// 兜底：任何未知事件都不能产生字典外的动作
	if got := inviteAuditAction("some_unknown_event"); got != model.AuditActionCreate {
		t.Errorf("未知事件不得产出字典外的动作，实际 %q", got)
	}
}

// TestR42AuditWrittenInSameTransaction 成功且仅成功时才有 accepted 审计，
// 且那行审计必须写在 Commit 之前（同事务）——源码顺序断言。
//
// 为什么不能直接"跑一次失败看有没有行"：失败路径的事务整体回滚，
// 无论写不写审计都查不到；真正要防的是"业务提交了、审计却写在事务外”。
func TestR42AuditWrittenInSameTransaction(t *testing.T) {
	for _, c := range []struct{ file, fn string }{
		{"member.go", "CreateInvite"},
		{"member.go", "RevokeInvite"},
	} {
		f, fset := remediationParse(t, c.file)
		fn := qas3Func(f, c.fn)
		if fn == nil {
			t.Errorf("%s 中找不到 %s", c.file, c.fn)
			continue
		}
		if got := remediationCallCount(fn, "insertAudit"); got != 1 {
			t.Errorf("%s.%s 应恰好写 1 条审计，实际 %d", c.file, c.fn, got)
		}
		auditLine := remediationFirstCallLine(fn, fset, "insertAudit")
		commitLine := remediationFirstCallLine(fn, fset, "Commit")
		if auditLine == 0 || commitLine == 0 {
			t.Errorf("%s.%s 缺少 insertAudit(%d) 或 Commit(%d)", c.file, c.fn, auditLine, commitLine)
			continue
		}
		if auditLine > commitLine {
			t.Errorf("%s.%s：insertAudit(L%d) 必须早于 Commit(L%d)，否则两者不在同一事务",
				c.file, c.fn, auditLine, commitLine)
		}
	}

	// JoinViaInvite 的成功审计同样必须在 Commit 之前
	f, fset := remediationParse(t, "member.go")
	fn := qas3Func(f, "JoinViaInvite")
	if fn == nil {
		t.Fatal("member.go 中找不到 JoinViaInvite")
	}
	if got := remediationCallCount(fn, "insertAudit"); got != 1 {
		t.Fatalf("JoinViaInvite 应恰好写 1 条成功审计，实际 %d", got)
	}
	auditLine := remediationFirstCallLine(fn, fset, "insertAudit")
	commitLine := remediationFirstCallLine(fn, fset, "Commit")
	if !(auditLine > 0 && commitLine > 0 && auditLine < commitLine) {
		t.Fatalf("JoinViaInvite：insertAudit(L%d) 必须早于 Commit(L%d)", auditLine, commitLine)
	}
}

// TestR42FailurePathsUseAppendedAudit 失败路径必须在【回滚之后】补审计
// （事务内的审计会随回滚一起消失，等于"拒绝了却没留下痕迹"）。
func TestR42FailurePathsUseAppendedAudit(t *testing.T) {
	f, _ := remediationParse(t, "member.go")
	fn := qas3Func(f, "inviteFailed")
	if fn == nil {
		t.Fatal("member.go 中找不到 inviteFailed")
	}
	if got := remediationCallCount(fn, "appendInviteRejectedAudit"); got != 1 {
		t.Fatalf("inviteFailed 应经由 appendInviteRejectedAudit 补审计，实际 %d", got)
	}
	if remediationCallCount(fn, "insertAudit") != 0 {
		t.Fatal("inviteFailed 不得使用事务内的 insertAudit：必然被回滚掉")
	}
	// 真正的写入点也必须用 AppendAudit（事务外独立写）一次
	appender := qas3Func(f, "appendInviteRejectedAudit")
	if appender == nil {
		t.Fatal("member.go 中找不到 appendInviteRejectedAudit")
	}
	if got := remediationCallCount(appender, "AppendAudit"); got != 1 {
		t.Fatalf("appendInviteRejectedAudit 应调用 AppendAudit 一次，实际 %d", got)
	}
	// 失败统一出口不得吞掉审计失败：必须用 errors.Join 把两个错误都带回去
	src := readServiceSource(t, "member.go")
	body := src[strings.Index(src, "func (s *MemberService) inviteFailed"):]
	body = body[:minStringEnd(body, 0)]
	if !strings.Contains(body, "errors.Join") {
		t.Fatal("inviteFailed 必须用 errors.Join 保留审计失败，不得静默吞掉（否则『查不到』会被当成『没问题』）")
	}
}

// HTTP 层的 boundEmail / email 入参与路由注册由 internal/api/book_handler_r41_test.go
// 与 internal/api/invite_lifecycle_api_test.go 校验（service 包不能反向 import api，故按包拆分）。
