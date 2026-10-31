package service

// 审计域第二轮（A1 / A3 / A4 / A5 / A8 / A9）的**正式回归测试**。
//
// 与本包其它审计用例的分工：
//   - audit_integration_test.go：WBS 4.3 既有写入点（登录 / 权限变更 / 可见性）；
//   - remediation_invite_integration_test.go：邀请生命周期的审计闭环（GPT-R42）；
//   - 本文件：本次审计缺口整改的证据——账本 create/delete/update、账户与分类的三种写操作、
//     "账本已满"拒绝、匿名兑换的审计归属、mark-used 的过期条件、限流桶的网段聚合。
//
// 断言口径统一为"答案能回答 PRD §10.9 的提问"：
// 给定任一写操作 → 执行后查询审计日志 → 可查到**操作者、时间、实体与变更前后状态**。
// 因此每条用例除了"有没有行"，还要核对 actor / entity_type / entity_id / book_id 归属，
// 以及 delete 的 before_json 是否为【变更前整行】。
//
// 环境：真实 SQLite（CGO + gcc）。本机 Application Control 会间歇拦下新生成的测试
// 二进制（"An Application Control policy has blocked this file"），重试即可。

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"fastledger/internal/model"
	"fastledger/internal/version"
)

// waAuditRows 按条件读取审计行（id 升序），条件为 SQL 片段 + 参数。
func waAuditRows(t *testing.T, e *testEnv, cond string, args ...interface{}) []model.AuditLog {
	t.Helper()
	rows := make([]model.AuditLog, 0, 4)
	if err := e.engine.Where(cond, args...).OrderBy("id ASC").Find(&rows); err != nil {
		t.Fatalf("读取审计行失败（%s）：%v", cond, err)
	}
	return rows
}

// waCountAudit 按条件统计审计行数。
func waCountAudit(t *testing.T, e *testEnv, cond string, args ...interface{}) int64 {
	t.Helper()
	var n int64
	if _, err := e.engine.SQL("SELECT COUNT(1) FROM audit_logs WHERE "+cond, args...).Get(&n); err != nil {
		t.Fatalf("统计审计行失败（%s）：%v", cond, err)
	}
	return n
}

// waBookRow 读取账本整行（含已删），用于核对"变更后状态"。
func waBookRow(t *testing.T, e *testEnv, bookID int64) model.Book {
	t.Helper()
	var b model.Book
	has, err := e.engine.ID(bookID).Get(&b)
	if err != nil {
		t.Fatalf("读账本 %d 失败：%v", bookID, err)
	}
	if !has {
		t.Fatalf("账本 %d 不存在", bookID)
	}
	return b
}

// ---------------------------------------------------------------------------
// A1：账本创建 / 删除 / 改名必须留痕
// ---------------------------------------------------------------------------

// TestWA1CreateLeavesAudit 账本创建必须留一行可追溯的审计。
//
// 断言口径（只收紧不放松）：
//   - 恰好 1 行，而不是 "≥1 行"——多写的行同样是缺陷（重复留痕会让人误判操作次数）；
//   - action=create、entity_type=book、entity_id=book_id=本账本 id；
//   - actor_uid = 创建者（不是 0 系统哨兵）；
//   - book_id **不是** AuditBookInstance 哨兵：建账本是账本域写操作，不是实例级事件；
//   - create 没有"变更前"状态，before_json 必须为空。
func TestWA1CreateLeavesAudit(t *testing.T) {
	e := newTestEnv(t)
	bs := NewBookService(e.engine, version.NewRepo())

	b, err := bs.Create(e.uid, "新建账本", model.BookTypeFamily)
	must(t, err)

	rows := waAuditRows(t, e, "entity_type = ? AND entity_id = ? AND action = ?",
		model.AuditEntityBook, b.Id, model.AuditActionCreate)
	if len(rows) != 1 {
		t.Fatalf("账本 %d 创建后应恰好 1 行 create 审计，实际 %d 行", b.Id, len(rows))
	}
	row := rows[0]
	if row.BookId != b.Id {
		t.Fatalf("创建审计的 book_id 应为账本自身 %d（不是实例哨兵 0），实际 %d", b.Id, row.BookId)
	}
	if row.ActorUid != e.uid {
		t.Fatalf("创建审计的 actor_uid 应为创建者 %d，实际 %d", e.uid, row.ActorUid)
	}
	if row.Source != "server" {
		t.Fatalf("审计来源应记为 server，实际 %q", row.Source)
	}
	if row.At <= 0 {
		t.Fatal("创建审计缺少时间戳（PRD §10.9 要求可查到'时间'）")
	}
	if row.BeforeJson != "" {
		t.Fatalf("create 没有变更前状态，before_json 应为空，实际 %q", row.BeforeJson)
	}
}

// TestWA1DeleteLeavesAudit 账本删除必须留痕，且 before_json 是【变更前整行】。
//
// 这是本次整改的核心项：账本删除（整本数据对外消失、所有成员连带失效、所有在途邀请
// 同时作废）此前是全仓唯一"零审计"的高危写操作。软删本身回答不了"删之前这本账叫什么、
// 属于谁、可见性开关是开是关"，因此 before_json 必须落整行原文。
func TestWA1DeleteLeavesAudit(t *testing.T) {
	e := newTestEnv(t)
	bs := NewBookService(e.engine, version.NewRepo())

	b, err := bs.Create(e.uid, "待删账本", model.BookTypeFamily)
	must(t, err)
	if got := waBookRow(t, e, b.Id); got.Deleted {
		t.Fatal("前提失败：新建账本不应是已删除状态")
	}
	must(t, bs.Delete(e.uid, b.Id))

	rows := waAuditRows(t, e, "entity_type = ? AND entity_id = ? AND action = ?",
		model.AuditEntityBook, b.Id, model.AuditActionDelete)
	if len(rows) != 1 {
		t.Fatalf("账本 %d 被 owner 删除后应恰好 1 行 delete 审计，实际 %d 行", b.Id, len(rows))
	}
	row := rows[0]
	if row.BookId != b.Id || row.EntityId != b.Id {
		t.Fatalf("删除审计的归属错误：book_id=%d entity_id=%d，期望均为 %d", row.BookId, row.EntityId, b.Id)
	}
	if row.ActorUid != e.uid {
		t.Fatalf("删除审计的 actor_uid 应为执行删除的 owner %d，实际 %d（0 会被误读成系统行为）",
			e.uid, row.ActorUid)
	}
	if row.At <= 0 {
		t.Fatal("删除审计缺少时间戳")
	}

	// before_json 必须是变更前的整行：名称、归属、可见性开关、以及 deleted=false 都在里面。
	// 字段名按 model.Book 的序列化结果（books 表模型未带 json tag，故为 Go 字段名）。
	for _, want := range []string{
		`"Id":` + fmt.Sprint(b.Id),
		`"Name":"待删账本"`,
		`"OwnerUid":` + fmt.Sprint(e.uid),
		`"MembersCanViewOthers":true`,
		`"Deleted":false`, // 变更前状态：删除前一刻还没删
	} {
		if !strings.Contains(row.BeforeJson, want) {
			t.Fatalf("删除审计的 before_json 不是变更前整行：缺 %s\n实际：%s", want, row.BeforeJson)
		}
	}
	// 变更后状态：库里确实已经是墓碑
	if !waBookRow(t, e, b.Id).Deleted {
		t.Fatal("审计写了软删，但账本并未被软删")
	}
}

// TestWA1UpdateLeavesAudit 账本改名必须留痕（与可见性开关 config_change 的口径统一）。
//
// 顺带固定的调查结论：**books 不在同步流**（SyncBatch 没有 Books 字段、
// syncEntityTables 只有 transactions/accounts/categories/holdings、books 表也没有
// sync_version 列），因此改名**不需要**递增 sync_version——客户端通过
// `GET /api/v1/books` 读取账本名称，不靠版本游标。本用例反向守住这一点：
// 改名不得在 book_versions 上留下任何推进（否则"版本边界分页"会莫名其妙地多跳一格）。
func TestWA1UpdateLeavesAudit(t *testing.T) {
	e := newTestEnv(t)
	bs := NewBookService(e.engine, version.NewRepo())

	verBefore, err := bs.CurrentVersion(e.bookID)
	must(t, err)

	res, err := bs.Update(e.uid, e.bookID, "  改过的名字  ", nil)
	must(t, err)
	if !res.NameChanged {
		t.Fatal("前提失败：改名应返回 NameChanged=true")
	}

	rows := waAuditRows(t, e, "entity_type = ? AND entity_id = ? AND action = ?",
		model.AuditEntityBook, e.bookID, model.AuditActionUpdate)
	if len(rows) != 1 {
		t.Fatalf("改名后应恰好 1 行 update 审计，实际 %d 行", len(rows))
	}
	row := rows[0]
	if row.ActorUid != e.uid || row.BookId != e.bookID {
		t.Fatalf("改名审计归属错误：actor=%d book_id=%d", row.ActorUid, row.BookId)
	}
	if !strings.Contains(row.BeforeJson, `"name":"家账"`) {
		t.Fatalf("改名审计的 before_json 应记录**变更前**名称（家账），实际 %q", row.BeforeJson)
	}
	// 名称已落库（去首尾空白）
	got := waBookRow(t, e, e.bookID)
	if got.Name != "改过的名字" {
		t.Fatalf("改名未生效：%q", got.Name)
	}
	// books 不是同步实体：改名不得推进 sync_version
	verAfter, err := bs.CurrentVersion(e.bookID)
	must(t, err)
	if verAfter != verBefore {
		t.Fatalf("改名把账本版本水位从 %d 推到了 %d：books 不在同步流内，"+
			"推进版本只会让版本边界分页白跳一格", verBefore, verAfter)
	}
}

// TestWA1UpdateSameNameWritesNoAudit 名称没变（或传空串）不写审计：
// 没有发生变更，就不该产生一条"看起来改了"的留痕（与 SetVisibility 幂等口径一致）。
func TestWA1UpdateSameNameWritesNoAudit(t *testing.T) {
	e := newTestEnv(t)
	bs := NewBookService(e.engine, version.NewRepo())

	res, err := bs.Update(e.uid, e.bookID, "家账", nil)
	must(t, err)
	if res.NameChanged {
		t.Fatal("同名提交不应产生实际变更")
	}
	if n := waCountAudit(t, e, "entity_type = ? AND action = ? AND book_id = ?",
		model.AuditEntityBook, model.AuditActionUpdate, e.bookID); n != 0 {
		t.Fatalf("同名提交不应写审计（没有变更可记），实际 %d 行", n)
	}
}

// ---------------------------------------------------------------------------
// A8：账户 / 分类的 create / update / delete 必须留痕
// ---------------------------------------------------------------------------

// TestWA8AccountAudit 账户三种写操作都要留痕；delete 的 before_json 是变更前整行。
func TestWA8AccountAudit(t *testing.T) {
	e := newTestEnv(t)
	as := NewAccountService(e.engine, version.NewRepo())

	acc, err := as.Create(e.uid, e.bookID, AccountCreateInput{
		Name: "工资卡", Category: model.CategoryDemandDeposit, Currency: "CNY",
	})
	must(t, err)

	// ① create
	created := waAuditRows(t, e, "entity_type = ? AND entity_id = ? AND action = ?",
		model.AuditEntityAccount, acc.Id, model.AuditActionCreate)
	if len(created) != 1 {
		t.Fatalf("建账户应恰好 1 行 create 审计，实际 %d 行", len(created))
	}
	if created[0].BookId != e.bookID || created[0].ActorUid != e.uid {
		t.Fatalf("账户 create 审计归属错误：book_id=%d actor=%d", created[0].BookId, created[0].ActorUid)
	}
	if !strings.Contains(created[0].BeforeJson, `"name":"工资卡"`) {
		t.Fatalf("账户 create 审计应记录被创建实体的关键字段，实际 %q", created[0].BeforeJson)
	}

	// ② update：before_json 记**变更前**名称
	newName := "工资卡（改名）"
	if _, err := as.Update(e.uid, e.bookID, acc.Id, AccountUpdateInput{Name: &newName}); err != nil {
		t.Fatalf("改账户失败：%v", err)
	}
	updated := waAuditRows(t, e, "entity_type = ? AND entity_id = ? AND action = ?",
		model.AuditEntityAccount, acc.Id, model.AuditActionUpdate)
	if len(updated) != 1 {
		t.Fatalf("改账户应恰好 1 行 update 审计，实际 %d 行", len(updated))
	}
	if !strings.Contains(updated[0].BeforeJson, `"name":"工资卡"`) {
		t.Fatalf("账户 update 审计的 before_json 应为变更前名称，实际 %q", updated[0].BeforeJson)
	}

	// ③ delete：before_json 是整行（含 balance / category / deleted=false）
	if _, err := as.Delete(e.uid, e.bookID, acc.Id); err != nil {
		t.Fatalf("删账户失败：%v", err)
	}
	deleted := waAuditRows(t, e, "entity_type = ? AND entity_id = ? AND action = ?",
		model.AuditEntityAccount, acc.Id, model.AuditActionDelete)
	if len(deleted) != 1 {
		t.Fatalf("删账户应恰好 1 行 delete 审计，实际 %d 行", len(deleted))
	}
	before := deleted[0].BeforeJson
	for _, want := range []string{
		`"id":` + fmt.Sprint(acc.Id),
		`"name":"工资卡（改名）"`, // 变更前整行 = 删除前一刻的状态
		`"category":` + fmt.Sprint(model.CategoryDemandDeposit),
		`"balance":0`,
		`"deleted":false`,
	} {
		if !strings.Contains(before, want) {
			t.Fatalf("账户 delete 的 before_json 不是变更前整行：缺 %s\n实际：%s", want, before)
		}
	}
}

// TestWA8CategoryAudit 分类三种写操作都要留痕；delete 的 before_json 是变更前整行。
func TestWA8CategoryAudit(t *testing.T) {
	e := newTestEnv(t)
	cs := NewCategoryService(e.engine, version.NewRepo())

	cat, err := cs.Create(e.uid, e.bookID, CategoryCreateInput{
		Name: "交通", CatType: model.CategoryTypeExpense,
	})
	must(t, err)

	created := waAuditRows(t, e, "entity_type = ? AND entity_id = ? AND action = ?",
		model.AuditEntityCategory, cat.Id, model.AuditActionCreate)
	if len(created) != 1 {
		t.Fatalf("建分类应恰好 1 行 create 审计，实际 %d 行", len(created))
	}
	if created[0].BookId != e.bookID || created[0].ActorUid != e.uid {
		t.Fatalf("分类 create 审计归属错误：book_id=%d actor=%d", created[0].BookId, created[0].ActorUid)
	}
	if !strings.Contains(created[0].BeforeJson, `"catType":`+fmt.Sprint(model.CategoryTypeExpense)) {
		t.Fatalf("分类 create 审计应记录收支类型，实际 %q", created[0].BeforeJson)
	}

	newName := "交通出行"
	if _, err := cs.Update(e.uid, e.bookID, cat.Id, CategoryUpdateInput{Name: &newName}); err != nil {
		t.Fatalf("改分类失败：%v", err)
	}
	updated := waAuditRows(t, e, "entity_type = ? AND entity_id = ? AND action = ?",
		model.AuditEntityCategory, cat.Id, model.AuditActionUpdate)
	if len(updated) != 1 {
		t.Fatalf("改分类应恰好 1 行 update 审计，实际 %d 行", len(updated))
	}
	if !strings.Contains(updated[0].BeforeJson, `"name":"交通"`) {
		t.Fatalf("分类 update 审计的 before_json 应为变更前名称，实际 %q", updated[0].BeforeJson)
	}

	if _, err := cs.Delete(e.uid, e.bookID, cat.Id); err != nil {
		t.Fatalf("删分类失败：%v", err)
	}
	deleted := waAuditRows(t, e, "entity_type = ? AND entity_id = ? AND action = ?",
		model.AuditEntityCategory, cat.Id, model.AuditActionDelete)
	if len(deleted) != 1 {
		t.Fatalf("删分类应恰好 1 行 delete 审计，实际 %d 行", len(deleted))
	}
	before := deleted[0].BeforeJson
	for _, want := range []string{
		`"id":` + fmt.Sprint(cat.Id),
		`"name":"交通出行"`,
		`"catType":` + fmt.Sprint(model.CategoryTypeExpense),
		`"deleted":false`,
	} {
		if !strings.Contains(before, want) {
			t.Fatalf("分类 delete 的 before_json 不是变更前整行：缺 %s\n实际：%s", want, before)
		}
	}
}

// TestWA8RejectedWritesLeaveNoAudit 被拒绝的写操作（权限不足 / 仍被引用）不写审计：
// 没有发生变更，就不该留下"看起来改了"的行。
func TestWA8RejectedWritesLeaveNoAudit(t *testing.T) {
	e := newTestEnv(t)
	as := NewAccountService(e.engine, version.NewRepo())
	// newTestEnv 的建库准备本身就建了 3 个账户（各留 1 行 create 审计），
	// 因此这里用"前后差值"而不是"绝对 0 行"来判断"被拒绝的写入没有留痕"。
	accountsBefore := waCountAudit(t, e, "entity_type = ? AND book_id = ?",
		model.AuditEntityAccount, e.bookID)

	// 普通成员无权建账户（权限矩阵 §5.5）
	stranger := insertUser(t, e, "a8-stranger")
	if _, err := as.Create(stranger, e.bookID, AccountCreateInput{
		Name: "无权账户", Category: model.CategoryCash, Currency: "CNY",
	}); !errors.Is(err, ErrNotMember) {
		t.Fatalf("非成员建账户应报 ErrNotMember，实际 %v", err)
	}
	if got := waCountAudit(t, e, "entity_type = ? AND book_id = ?",
		model.AuditEntityAccount, e.bookID); got != accountsBefore {
		t.Fatalf("被拒绝的建账户不应写审计：%d → %d 行", accountsBefore, got)
	}

	// 仍被交易引用的账户不可删；被拒绝时同样不得留 delete 行
	req := e.expense(CreateRequest{ClientUuid: "a8-tx-1", Amount: 100})
	if _, err := e.txs.Create(context.Background(), e.uid, e.bookID, req); err != nil {
		t.Fatalf("建交易失败：%v", err)
	}
	if _, err := as.Delete(e.uid, e.bookID, e.assetID); !errors.Is(err, ErrAccountInUse) {
		t.Fatalf("仍被交易引用的账户不可删，实际 %v", err)
	}
	if n := waCountAudit(t, e, "entity_type = ? AND action = ? AND entity_id = ?",
		model.AuditEntityAccount, model.AuditActionDelete, e.assetID); n != 0 {
		t.Fatalf("被拒绝的删账户不应写审计，实际 %d 行", n)
	}
}

// ---------------------------------------------------------------------------
// A3：账本已满拒绝兑换必须留痕
// ---------------------------------------------------------------------------

// TestWA3BookFullLeavesAudit 成员上限拒绝也必须写 result=rejected + reason=book_full。
//
// 这是兑换路径上唯一曾经绕过 inviteFailed 的失败出口：账本顶到 20 人之后，
// 所有入户尝试在审计里完全不可见，而"有人持续试图挤进已满的账本"恰恰是最该被
// 看见的信号之一。对外错误语义不得变化（仍是 ErrBookFull → HTTP 409）。
func TestWA3BookFullLeavesAudit(t *testing.T) {
	e := newTestEnv(t)
	ms := newMemberSvc(t, e)

	// 把成员数顶到上限 20（newTestEnv 的账本已有 owner 1 人）
	for i := 1; i < maxBookMembers; i++ {
		addMember(t, e, insertUser(t, e, "a3-fill-"+fmt.Sprint(i)), model.RoleMember)
	}
	var memberCount int64
	if _, err := e.engine.SQL(
		"SELECT COUNT(1) FROM book_members WHERE book_id = ? AND deleted = 0", e.bookID).
		Get(&memberCount); err != nil {
		t.Fatalf("统计成员失败：%v", err)
	}
	if memberCount != maxBookMembers {
		t.Fatalf("前提失败：成员数 %d，应为 %d", memberCount, maxBookMembers)
	}

	inv, err := ms.CreateInvite(e.uid, e.bookID, model.RoleMember, "", "")
	must(t, err)
	before := waCountAudit(t, e, "book_id = ? AND before_json LIKE ?", e.bookID, "%invite_redeem%")

	if _, err := ms.JoinViaInvite(e.bookID, inv.Code, "a3-newcomer", "", "password123", "10.1.0.1"); !errors.Is(err, ErrBookFull) {
		t.Fatalf("满员兑换应仍返回 ErrBookFull（对外语义不得变化），实际 %v", err)
	}

	after := waCountAudit(t, e, "book_id = ? AND before_json LIKE ?", e.bookID, "%invite_redeem%")
	if after != before+1 {
		t.Fatalf("满员拒绝没有留痕：之前 %d 行、之后 %d 行，期望恰好新增 1 行", before, after)
	}
	rows := waAuditRows(t, e, "book_id = ? AND entity_type = ? AND entity_id = ? AND before_json LIKE ?",
		e.bookID, model.AuditEntityMember, inv.Id, "%"+inviteReasonBookFull+"%")
	if len(rows) != 1 {
		t.Fatalf("满员拒绝应留 1 行 reason=book_full 的审计，实际 %d 行", len(rows))
	}
	if !strings.Contains(rows[0].BeforeJson, `"result":"`+inviteResultRejected+`"`) {
		t.Fatalf("满员拒绝的审计应是 rejected 结果，实际 %q", rows[0].BeforeJson)
	}
}

// ---------------------------------------------------------------------------
// A4：未认证拒绝不得把审计写进"调用方自称"的账本
// ---------------------------------------------------------------------------

// TestWA4AnonymousRejectNotOnForeignBook 匿名用 A 账本的路径 + B 账本的邀请码兑换：
// A 的审计流必须零新增行；这条尝试以"无法归因"的形式落到 book_id=0。
//
// 旧实现把 URL 路径参数当审计归属，于是匿名者可以以 10 次/小时/IP 的代价往任意账本的
// 审计流灌行，而且那些行带 actor_uid=0（系统哨兵），排查时会被误读成服务端自身行为。
func TestWA4AnonymousRejectNotOnForeignBook(t *testing.T) {
	e := newTestEnv(t)
	ms := newMemberSvc(t, e)
	bs := NewBookService(e.engine, version.NewRepo())

	other, err := bs.Create(e.uid, "另一本账", model.BookTypeFamily)
	must(t, err)
	inv, err := ms.CreateInvite(e.uid, other.Id, model.RoleMember, "", "")
	must(t, err)

	// 前提：A 账本（e.bookID）此前没有任何匿名兑换尝试的审计
	if n := waCountAudit(t, e, "book_id = ? AND actor_uid = ?", e.bookID, model.AuditActorSystem); n != 0 {
		t.Fatalf("前提失败：A 账本已有 %d 行匿名审计", n)
	}
	bookABefore := waCountAudit(t, e, "book_id = ?", e.bookID)
	instanceBefore := waCountAudit(t, e, "book_id = ?", model.AuditBookInstance)

	// 用 A 的路径去兑换 B 的邀请码
	if _, err := ms.JoinViaInvite(e.bookID, inv.Code, "a4-attacker", "", "password123", "10.2.0.1"); !errors.Is(err, ErrInviteInvalid) {
		t.Fatalf("跨账本兑换应报 ErrInviteInvalid，实际 %v", err)
	}

	// ① A 的审计流零新增
	if got := waCountAudit(t, e, "book_id = ?", e.bookID); got != bookABefore {
		t.Fatalf("跨账本兑换往 A 账本（%d）的审计流写入了行：%d → %d；"+
			"匿名者不应能往任意账本灌行", e.bookID, bookABefore, got)
	}
	// ② B 的审计流也不该多一行（邀请真实归属未知于调用方，属于无法归因）
	if got := waCountAudit(t, e, "book_id = ? AND actor_uid = ?", other.Id, model.AuditActorSystem); got != 0 {
		t.Fatalf("无法归因的尝试被记到了 B 账本（%d）：%d 行", other.Id, got)
	}
	// ③ 这条尝试仍然留痕（取证价值不能丢），落在实例哨兵 0 并显式标注无法归因
	instanceRows := waAuditRows(t, e, "book_id = ? AND before_json LIKE ?",
		model.AuditBookInstance, "%"+inviteReasonNotFound+"%")
	if len(instanceRows) == 0 {
		t.Fatal("无法归因的兑过尝试必须留下一条 book_id=0 的审计（有人在撞门户的取证线索）")
	}
	last := instanceRows[len(instanceRows)-1]
	if !strings.Contains(last.BeforeJson, `"bookAttribution":"`+inviteAttributionUnattributable+`"`) {
		t.Fatalf("无法归因的审计必须自带 bookAttribution 标记（否则 book_id=0 会被误读成登录/备份恢复）：%q",
			last.BeforeJson)
	}
	if got := waCountAudit(t, e, "book_id = ?", model.AuditBookInstance); got != instanceBefore+1 {
		t.Fatalf("实例哨兵桶应恰好新增 1 行，实际 %d → %d", instanceBefore, got)
	}
	// ④ 原邀请未被消耗（跨账本兑换不得产生任何副作用）
	var fresh model.Invite
	if _, err := e.engine.ID(inv.Id).Get(&fresh); err != nil {
		t.Fatalf("读邀请失败：%v", err)
	}
	if fresh.UsedAt != 0 || fresh.UsedBy != 0 {
		t.Fatalf("被拒的跨账本兑换消耗了原邀请：used_at=%d used_by=%d", fresh.UsedAt, fresh.UsedBy)
	}
}

// ---------------------------------------------------------------------------
// A9：mark-used 必须带过期条件
// ---------------------------------------------------------------------------

// TestWA9MarkUsedRejectsExpired 「标记已用」的条件更新必须与撤销、过期判定共用同一口径。
//
// 两部分证据：
//  1. 源码断言：JoinViaInvite 的标记已用 UPDATE 复用 inviteUsableCond（单一实现）；
//  2. 行为断言：拿同一条条件去更新一个已过期的邀请，必须命中 0 行（拒绝），
//     而未过期的邀请命中 1 行——这正是旧实现漏掉 `expires_at > ?` 时无法区分的情形。
func TestWA9MarkUsedRejectsExpired(t *testing.T) {
	e := newTestEnv(t)
	ms := newMemberSvc(t, e)

	src := readServiceSource(t, "member.go")
	joinStart := strings.Index(src, "func (s *MemberService) JoinViaInvite")
	if joinStart < 0 {
		t.Fatal("member.go 中找不到 JoinViaInvite")
	}
	join := src[joinStart:]
	if end := strings.Index(join[1:], "\nfunc "); end >= 0 {
		join = join[:end+1]
	}
	if !strings.Contains(join, "inviteUsableCond") {
		t.Fatal("JoinViaInvite 标记已用的 UPDATE 必须复用 inviteUsableCond" +
			"（旧实现在这里手写条件并漏掉了 expires_at > ?，与撤销/过期判定两套口径打架）")
	}

	// 行为断言：同一条条件对"已过期"与"仍有效"的邀请给出不同结论
	expired, err := ms.CreateInvite(e.uid, e.bookID, model.RoleMember, "", "")
	must(t, err)
	live, err := ms.CreateInvite(e.uid, e.bookID, model.RoleMember, "", "")
	must(t, err)
	if _, err := e.engine.Exec("UPDATE invites SET expires_at = ? WHERE id = ?",
		time.Now().Unix()-60, expired.Id); err != nil {
		t.Fatalf("构造过期邀请失败：%v", err)
	}

	now := time.Now().Unix()
	markUsed := func(inviteID int64) int64 {
		sess := e.engine.NewSession()
		defer sess.Close()
		if err := sess.Begin(); err != nil {
			t.Fatalf("开启事务失败：%v", err)
		}
		defer func() { _ = sess.Rollback() }()
		n, err := sess.Where("id = ? AND "+inviteUsableCond, inviteID, now).
			Cols("used_at", "used_by").Update(&model.Invite{UsedAt: now, UsedBy: e.uid})
		if err != nil {
			t.Fatalf("条件更新失败：%v", err)
		}
		return n
	}
	if n := markUsed(expired.Id); n != 0 {
		t.Fatalf("已过期邀请的标记已用应命中 0 行，实际 %d 行（过期条件缺失）", n)
	}
	if n := markUsed(live.Id); n != 1 {
		t.Fatalf("未过期邀请的标记已用应命中 1 行，实际 %d 行", n)
	}

	// 端到端：过期邀请的兑换仍是 ErrInviteExpired（对外语义不得被条件改写影响）
	if _, err := ms.JoinViaInvite(e.bookID, expired.Code, "a9-late", "", "password123", "10.3.0.1"); !errors.Is(err, ErrInviteExpired) {
		t.Fatalf("过期邀请兑换应报 ErrInviteExpired，实际 %v", err)
	}
}

// ---------------------------------------------------------------------------
// A5：限流桶必须按网段聚合（与登录侧 loginKey 同口径）
// ---------------------------------------------------------------------------

// TestWA5IPBucketAggregates 同 /24（IPv4）与同 /64（IPv6）共享一个桶，跨网段不共享。
//
// 为什么必须聚合（对抗验证 ZZ-9 实测复现）：IPv6 隐私扩展让一台机器在自己 /64 内可
// 轮转出海量地址，按完整地址分桶等于"每换一个地址就是全新配额"，邀请入户 10 次/小时
// 的配额被地址数摊薄成 0。口径必须与 auth.go 的 loginKey 一致，否则两套限流语义打架。
//
// 代价（明确取舍）：同一 /24 或 /64 内的多台设备共享一个配额。家庭自托管场景下，
// 同一出口路由器后面的设备本来就该算同一个来源。
func TestWA5IPBucketAggregates(t *testing.T) {
	// ① 同一 /24 的两个 IPv4 落进同一个桶
	v4a, v4b := ipBucket("203.0.113.7"), ipBucket("203.0.113.200")
	if v4a != v4b {
		t.Fatalf("同 /24 的 IPv4 应共享桶：%q != %q", v4a, v4b)
	}
	// ② 不同 /24 不共享
	if ipBucket("203.0.114.7") == v4a {
		t.Fatalf("不同 /24 的 IPv4 不应共享桶：%q", v4a)
	}
	// ③ 同一 /64 的 IPv6 落进同一个桶
	v6a, v6b := ipBucket("2001:db8:1::2"), ipBucket("2001:db8:1::dead:beef")
	if v6a != v6b {
		t.Fatalf("同 /64 的 IPv6 应共享桶：%q != %q", v6a, v6b)
	}
	// ④ 不同 /64 不共享
	if ipBucket("2001:db8:2::2") == v6a {
		t.Fatalf("不同 /64 的 IPv6 不应共享桶：%q", v6a)
	}
	// ⑤ IPv4-mapped 与原生 IPv4 同桶（否则同一台机器两种写法拿两份配额）
	if ipBucket("::ffff:203.0.113.7") != v4a {
		t.Fatalf("IPv4-mapped 应先 Unmap 再按 IPv4 口径聚合：%q != %q",
			ipBucket("::ffff:203.0.113.7"), v4a)
	}
	// ⑥ 不可解析值 fail-closed 进同一个 unknown 桶
	if ipBucket("") != ipBucket("garbage-host") {
		t.Fatal("不可解析来源应统一落进 unknown 共享桶（fail-closed）")
	}

	// ⑦ 端到端：同 /64 内轮转 12 个地址，配额（10 次/小时）必然被耗尽
	e := newTestEnv(t)
	ms := newMemberSvc(t, e)
	limited := 0
	for i := 0; i < 12; i++ {
		addr := fmt.Sprintf("2001:db8:1::%d", i+2)
		if _, err := ms.JoinViaInvite(e.bookID, "deadbeefdeadbeefdeadbeefdeadbeef",
			fmt.Sprintf("a5-rot-%d", i), "", "password123", addr); errors.Is(err, ErrRateLimited) {
			limited++
		}
	}
	if limited < 2 {
		t.Fatalf("同 /64 内轮转 12 个地址只出现 %d 次限流（期望 ≥2）："+
			"ipBucket 未按网段聚合，IPv6 地址轮转可把 10 次/小时配额摊薄", limited)
	}
}
