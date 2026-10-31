package authzreview

// zz_audit_adversarial_test.go —— 审计域整改（163184f / 5a97c63 / 9c06ab9）的**对抗性验证**。
//
// 定位：与 zz_authz_adversarial*.go 同一套约定——每条探针都从"如果我是不放心的复审员，
// 我还能怎么证明它其实没修好"出发。绝大多数用例断言的是"修复应成立的不变量"（绿=守卫），
// 只有明确标注的才允许是红灯（残留缺口证据）。
//
// 覆盖：
//
//	ZZA1-回滚：审计必须随事务一起回滚（去掉 audit_logs 表诱导 Commit 前失败）
//	ZZA1-失败路径：无权限 / 并发删除失败时零审计
//	ZZA1-before_json：改名审计里的旧名确实是旧值
//	ZZA3：满员拒绝的错误语义不变（仍 ErrBookFull）、用户未创建、邀请未被消耗
//	ZZA4：逐个失败分支核对审计归属（不得落到"调用方自称"的账本）
//	ZZA8：被拒绝的写入零审计；成功写入的 entity_type/entity_id 对得上
//	ZZA5：ipBucket 与 loginKey 同口径（mapped 形态同桶、不可解析 fail-closed）
//	ZZA2：已删账本失败关闭不得误伤活账本路径

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"fastledger/internal/model"
	"fastledger/internal/service"
	"fastledger/internal/version"
)

// zzAuditRows 统计某账本某动作的审计行数。
func zzAuditRows(t *testing.T, e *env, where string, args ...interface{}) int64 {
	t.Helper()
	return countAudit(t, e.engine, where, args...)
}

// ---------- A1：事务回滚不留审计 ----------

// TestZZA1AuditRollsBackWithTransaction 审计写失败必须整笔回滚（不能"删了没记"）。
//
// 手法：临时把 audit_logs 表删掉，使同事务内的 insertAudit 必然失败。
// 期望：Delete / Create 返回错误，且业务写入随事务回滚（账本仍存活、账户未创建）。
// 这是"审计与业务同事务"的唯一**可诱导**证据（提交本身无法从公开 API 制造失败）。
func TestZZA1AuditRollsBackWithTransaction(t *testing.T) {
	e := newEnv(t)
	acc := service.NewAccountService(e.engine, version.NewRepo())

	if _, err := e.engine.Exec("DROP TABLE audit_logs"); err != nil {
		t.Fatalf("准备失败：无法删除 audit_logs 表：%v", err)
	}

	// ① 删除账本：审计写不进去 → 整笔失败 → 账本必须仍然存活
	if err := e.books.Delete(e.owner, e.bookID); err == nil {
		t.Fatal("audit_logs 不可写时删除账本仍返回成功：审计失败没有阻断业务" +
			"（会出现「删了却没留痕」的不可追溯状态）")
	}
	var stillAlive int64
	if _, err := e.engine.SQL("SELECT COUNT(1) FROM books WHERE id = ? AND deleted = 0",
		e.bookID).Get(&stillAlive); err != nil {
		t.Fatalf("查询账本状态失败：%v", err)
	}
	if stillAlive != 1 {
		t.Fatal("审计写入失败但账本已被软删：事务没有覆盖审计，业务与审计不同生共死")
	}

	// ② 创建账户：审计写不进去 → 整笔失败 → 账户必须不存在
	if _, err := acc.Create(e.owner, e.bookID, service.AccountCreateInput{
		Name: "回滚探针账户", Category: model.CategoryCash, Currency: "CNY",
	}); err == nil {
		t.Fatal("audit_logs 不可写时创建账户仍返回成功")
	}
	var accCount int64
	if _, err := e.engine.SQL("SELECT COUNT(1) FROM accounts WHERE book_id = ? AND name = ?",
		e.bookID, "回滚探针账户").Get(&accCount); err != nil {
		t.Fatalf("查询账户失败：%v", err)
	}
	if accCount != 0 {
		t.Fatal("审计写入失败但账户已落库：创建账户的审计不在同一事务内")
	}
}

// TestZZA1FailedDeleteLeavesNoAudit 删除失败（无权限 / 已删）不得留下 delete 审计行。
func TestZZA1FailedDeleteLeavesNoAudit(t *testing.T) {
	e := newEnv(t)
	before := zzAuditRows(t, e, "entity_type = ? AND action = ?", model.AuditEntityBook, model.AuditActionDelete)

	// admin 不是 owner：删除必须被拒
	if err := e.books.Delete(e.admin, e.bookID); !errors.Is(err, service.ErrForbidden) {
		t.Fatalf("admin 删除账本应被拒（ErrForbidden），实际 %v", err)
	}
	// 非成员：同样被拒
	if err := e.books.Delete(e.member, e.bookID); err == nil {
		t.Fatal("普通成员竟然删掉了账本")
	}
	after := zzAuditRows(t, e, "entity_type = ? AND action = ?", model.AuditEntityBook, model.AuditActionDelete)
	if after != before {
		t.Fatalf("被拒绝的删除写入了 %d 行 delete 审计（期望 0 行新增）——审计不得为未发生的变更留痕",
			after-before)
	}

	// 先正常删除一次，再删第二次：第二次必须失败且不再新增审计
	if err := e.books.Delete(e.owner, e.bookID); err != nil {
		t.Fatalf("owner 删除账本失败：%v", err)
	}
	first := zzAuditRows(t, e, "entity_type = ? AND action = ?", model.AuditEntityBook, model.AuditActionDelete)
	if err := e.books.Delete(e.owner, e.bookID); err == nil {
		t.Fatal("重复删除同一个账本竟然返回成功")
	}
	second := zzAuditRows(t, e, "entity_type = ? AND action = ?", model.AuditEntityBook, model.AuditActionDelete)
	if second != first {
		t.Fatalf("重复删除失败却新增了 %d 行审计", second-first)
	}
}

// TestZZA1RenameAuditBeforeJsonIsOldName 改名审计的 before_json 必须是**旧**名字。
//
// 攻击点：如果 before_json 取的是被覆盖之后的值，"改名"这件事就无法回答"从什么改到什么"。
// 同时验证：同名提交不写审计；可见性变更走 config_change 而不是 update（两条语义分离）。
func TestZZA1RenameAuditBeforeJsonIsOldName(t *testing.T) {
	e := newEnv(t)
	if _, err := e.books.Update(e.owner, e.bookID, "改名后的账本", nil); err != nil {
		t.Fatalf("改名失败：%v", err)
	}
	var beforeJSON string
	has, err := e.engine.SQL(
		"SELECT before_json FROM audit_logs WHERE entity_type = ? AND action = ? AND entity_id = ? ORDER BY id DESC LIMIT 1",
		model.AuditEntityBook, model.AuditActionUpdate, e.bookID).Get(&beforeJSON)
	if err != nil || !has {
		t.Fatalf("改名没有留下 update 审计（has=%v err=%v）", has, err)
	}
	if !strings.Contains(beforeJSON, "家账") || strings.Contains(beforeJSON, "改名后的账本") {
		t.Fatalf("改名审计的 before_json 不是旧名：%s（期望含旧名「家账」且不含新名）", beforeJSON)
	}

	// 同名提交：无实际变更 → 不写 update 行
	n1 := zzAuditRows(t, e, "entity_type = ? AND action = ?", model.AuditEntityBook, model.AuditActionUpdate)
	if _, err := e.books.Update(e.owner, e.bookID, "改名后的账本", nil); err != nil {
		t.Fatalf("同名提交失败：%v", err)
	}
	n2 := zzAuditRows(t, e, "entity_type = ? AND action = ?", model.AuditEntityBook, model.AuditActionUpdate)
	if n2 != n1 {
		t.Fatalf("同名提交写入了 %d 行 update 审计（期望 0 行新增）", n2-n1)
	}

	// 可见性变更：必须是 config_change，且不得冒充 update
	if _, err := e.books.SetVisibility(e.owner, e.bookID, false); err != nil {
		t.Fatalf("改可见性失败：%v", err)
	}
	if n := zzAuditRows(t, e, "entity_type = ? AND action = ?", model.AuditEntityBook,
		model.AuditActionConfigChange); n == 0 {
		t.Fatal("可见性变更没有 config_change 审计")
	}
	n3 := zzAuditRows(t, e, "entity_type = ? AND action = ?", model.AuditEntityBook, model.AuditActionUpdate)
	if n3 != n1 {
		t.Fatalf("可见性变更被记成了 update（update 行 %d → %d）：两条语义必须分离", n1, n3)
	}
}

// ---------- A8：被拒绝的写入零审计 + 实体归属正确 ----------

// TestZZA8RejectedWritesLeaveNoAudit 独立构造每一种"被拒绝"的账户/分类写入，逐一断言零留痕。
func TestZZA8RejectedWritesLeaveNoAudit(t *testing.T) {
	e := newEnv(t)
	acc := service.NewAccountService(e.engine, version.NewRepo())
	cat := service.NewCategoryService(e.engine, version.NewRepo())
	txs := service.NewTxService(e.engine, version.NewRepo())

	before := countAudit(t, e.engine, "book_id = ? AND entity_type IN (?, ?)",
		e.bookID, model.AuditEntityAccount, model.AuditEntityCategory)

	// ① 普通成员（非 owner/admin）建账户 → 403
	if _, err := acc.Create(e.member, e.bookID, service.AccountCreateInput{
		Name: "偷建账户", Category: model.CategoryCash, Currency: "CNY",
	}); err == nil {
		t.Fatal("member 竟然建出了账户")
	}
	// ② 普通成员建分类 → 403
	if _, err := cat.Create(e.member, e.bookID, service.CategoryCreateInput{
		Name: "偷建分类", CatType: model.CategoryTypeExpense,
	}); err == nil {
		t.Fatal("member 竟然建出了分类")
	}
	// ③ 仍被交易引用的账户不可删（R31）——用真实交易制造引用
	acct, err := acc.Create(e.owner, e.bookID, service.AccountCreateInput{
		Name: "有引用的账户", Category: model.CategoryCash, Currency: "CNY",
	})
	if err != nil {
		t.Fatalf("建账户失败：%v", err)
	}
	catRow, err := cat.Create(e.owner, e.bookID, service.CategoryCreateInput{
		Name: "支出分类X", CatType: model.CategoryTypeExpense,
	})
	if err != nil {
		t.Fatalf("建分类失败：%v", err)
	}
	now := time.Now().Unix()
	if _, err := txs.Create(nil, e.owner, e.bookID, service.CreateRequest{
		ClientUuid: "zz-a8-ref", Amount: 1234, TxType: int(model.TxTypeExpense),
		AccountId: acct.Id, CategoryId: catRow.Id, TxTime: now,
	}); err != nil {
		t.Fatalf("建交易失败：%v", err)
	}
	mid := countAudit(t, e.engine, "book_id = ? AND entity_type IN (?, ?)",
		e.bookID, model.AuditEntityAccount, model.AuditEntityCategory)

	if _, err := acc.Delete(e.owner, e.bookID, acct.Id); err == nil {
		t.Fatal("仍被交易引用的账户被删掉了（R31 失效）")
	}
	if _, err := cat.Delete(e.owner, e.bookID, catRow.Id); err == nil {
		t.Fatal("仍被交易引用的分类被删掉了")
	}
	// ④ 有活跃子账户的父账户不可删（R53）
	parent, err := acc.Create(e.owner, e.bookID, service.AccountCreateInput{
		Name: "父账户", Category: model.CategoryCash, Currency: "CNY",
	})
	if err != nil {
		t.Fatalf("建父账户失败：%v", err)
	}
	child, err := acc.Create(e.owner, e.bookID, service.AccountCreateInput{
		Name: "子账户", Category: model.CategoryCash, Currency: "CNY", ParentId: parent.Id,
	})
	if err != nil {
		t.Fatalf("建子账户失败：%v", err)
	}
	if _, err := acc.Delete(e.owner, e.bookID, parent.Id); err == nil {
		t.Fatal("有活跃子账户的父账户被删掉了（R53 失效）")
	}
	// ⑤ 无权限删除（member 删自己的账本资产）→ 403
	if _, err := acc.Delete(e.member, e.bookID, child.Id); err == nil {
		t.Fatal("member 竟然删掉了账户")
	}

	after := countAudit(t, e.engine, "book_id = ? AND entity_type IN (?, ?)",
		e.bookID, model.AuditEntityAccount, model.AuditEntityCategory)
	// 期望：全程只有 4 次**成功** create（acct / catRow / parent / child）各留 1 行；
	// 被拒的 5 次写入（member 建账户、member 建分类、R31 删账户、R31 删分类、R53 删父账户、
	// member 删账户）**一行都不该留**。其中 acct/catRow 的 create 已经计入 mid。
	if got := after - before; got != 4 {
		t.Fatalf("被拒绝的写入留下了审计行：全程新增 %d 行，期望恰 4 行（只有 4 次成功 create）；"+
			"被 R31/R53/权限拦下的路径必须零留痕", got)
	}
	if want := mid + 2; after != want {
		t.Fatalf("被拒删除前后审计行数变化异常：mid=%d after=%d（期望 after=mid+2=%d，"+
			"即 parent/child 两次成功 create）", mid, after, want)
	}
	// 被拒删除本身不得产生 delete 行（delete 只应出现在真正成功的删除路径上）
	if n := countAudit(t, e.engine, "book_id = ? AND action = ? AND entity_type IN (?, ?)",
		e.bookID, model.AuditActionDelete, model.AuditEntityAccount, model.AuditEntityCategory); n != 0 {
		t.Fatalf("六次被拒的删除/创建里出现了 %d 行 delete 审计：拒绝必须零留痕", n)
	}
}

// TestZZA8EntityAttribution 成功的账户/分类写入，entity_type / entity_id / book_id / actor 必须对得上。
func TestZZA8EntityAttribution(t *testing.T) {
	e := newEnv(t)
	acc := service.NewAccountService(e.engine, version.NewRepo())
	cat := service.NewCategoryService(e.engine, version.NewRepo())

	a, err := acc.Create(e.owner, e.bookID, service.AccountCreateInput{
		Name: "归属核对账户", Category: model.CategoryCash, Currency: "CNY",
	})
	if err != nil {
		t.Fatalf("建账户失败：%v", err)
	}
	c, err := cat.Create(e.owner, e.bookID, service.CategoryCreateInput{
		Name: "归属核对分类", CatType: model.CategoryTypeExpense,
	})
	if err != nil {
		t.Fatalf("建分类失败：%v", err)
	}

	var row model.AuditLog
	has, err := e.engine.Where("entity_type = ? AND action = ? AND entity_id = ?",
		model.AuditEntityAccount, model.AuditActionCreate, a.Id).Get(&row)
	if err != nil || !has {
		t.Fatalf("账户 create 审计缺失或 entity_id 不对（has=%v err=%v）", has, err)
	}
	if row.BookId != e.bookID || row.ActorUid != e.owner {
		t.Fatalf("账户 create 审计归属错误：book_id=%d actor=%d（期望 %d/%d）",
			row.BookId, row.ActorUid, e.bookID, e.owner)
	}
	var crow model.AuditLog
	has, err = e.engine.Where("entity_type = ? AND action = ? AND entity_id = ?",
		model.AuditEntityCategory, model.AuditActionCreate, c.Id).Get(&crow)
	if err != nil || !has {
		t.Fatalf("分类 create 审计缺失或 entity_id 不对（has=%v err=%v）", has, err)
	}
	if crow.BookId != e.bookID || crow.ActorUid != e.owner {
		t.Fatalf("分类 create 审计归属错误：book_id=%d actor=%d", crow.BookId, crow.ActorUid)
	}

	// 删除必须带整行 before_json，且含"删除前 deleted:false"的证据
	if _, err := acc.Delete(e.owner, e.bookID, a.Id); err != nil {
		t.Fatalf("删账户失败：%v", err)
	}
	var del model.AuditLog
	has, err = e.engine.Where("entity_type = ? AND action = ? AND entity_id = ?",
		model.AuditEntityAccount, model.AuditActionDelete, a.Id).Get(&del)
	if err != nil || !has {
		t.Fatalf("账户 delete 审计缺失（has=%v err=%v）", has, err)
	}
	if !strings.Contains(del.BeforeJson, `"deleted":false`) ||
		!strings.Contains(del.BeforeJson, "归属核对账户") {
		t.Fatalf("账户 delete 的 before_json 不是变更前整行：%s", del.BeforeJson)
	}
}

// ---------- A3：满员拒绝的错误语义与副作用 ----------

// TestZZA3BookFullSemanticsUnchanged 满员拒绝：对外仍是 ErrBookFull、用户不创建、邀请不被消耗。
func TestZZA3BookFullSemanticsUnchanged(t *testing.T) {
	e := newEnv(t)
	now := time.Now().Unix()
	for i := 0; i < 17; i++ { // 3 + 17 = 20 = maxBookMembers
		u := &model.User{Username: "zz_bf_" + strconv.Itoa(i) + "_" + time.Now().Format("150405.000000000"), PasswordHash: "x"}
		if _, err := e.engine.Insert(u); err != nil {
			t.Fatalf("插入用户失败：%v", err)
		}
		if _, err := e.engine.Insert(&model.BookMember{BookId: e.bookID, Uid: u.Id,
			Role: model.RoleMember, CanViewOthers: true, CreatedAt: now, UpdatedAt: now}); err != nil {
			t.Fatalf("加成员失败：%v", err)
		}
	}
	inv, err := e.members.CreateInvite(e.owner, e.bookID, model.RoleMember, "", "")
	if err != nil {
		t.Fatalf("签发邀请失败：%v", err)
	}
	before := countAudit(t, e.engine, "book_id = ? AND before_json LIKE ?", e.bookID, "%book_full%")

	u, jerr := e.members.JoinViaInvite(e.bookID, inv.Code, "zz_bf_newcomer", "", "password123", "10.9.0.1")
	if !errors.Is(jerr, service.ErrBookFull) {
		t.Fatalf("满员兑换应返回 ErrBookFull（对外映射 409），实际 %v", jerr)
	}
	if u != nil {
		t.Fatal("满员兑换仍然创建了用户对象")
	}
	// 用户未创建
	var userCount int64
	if _, err := e.engine.SQL("SELECT COUNT(1) FROM users WHERE username = ?", "zz_bf_newcomer").
		Get(&userCount); err != nil {
		t.Fatalf("统计用户失败：%v", err)
	}
	if userCount != 0 {
		t.Fatal("满员兑换回滚不彻底：留下了孤儿用户")
	}
	// 邀请未被消耗
	var fresh model.Invite
	if _, err := e.engine.ID(inv.Id).Get(&fresh); err != nil {
		t.Fatalf("读邀请失败：%v", err)
	}
	if fresh.UsedAt != 0 || fresh.UsedBy != 0 || fresh.Revoked {
		t.Fatalf("满员拒绝却改动了邀请状态：used_at=%d used_by=%d revoked=%v",
			fresh.UsedAt, fresh.UsedBy, fresh.Revoked)
	}
	// 审计恰好 +1 且 reason=book_full
	after := countAudit(t, e.engine, "book_id = ? AND before_json LIKE ?", e.bookID, "%book_full%")
	if after != before+1 {
		t.Fatalf("满员拒绝应恰新增 1 行 reason=book_full 审计，实际 %d", after-before)
	}
}

// ---------- A4：逐分支核对审计归属 ----------

// TestZZA4EveryFailureBranchAttributionIsSafe 逐个失败分支核对：绝不写进"调用方自称"的外来账本。
//
// 覆盖分支：邀请不存在 / 跨账本邀请 / 过期 / 撤销 / 已用 / 绑定不匹配 / 账本已删 / 满员。
// 断言方式：每一轮都先记录"目标账本 A"与"存活账本 B"的审计行数，任何一轮都不得让 A 增加
// 与本次尝试相关的行（A 是攻击者随便填的 book_id）。
func TestZZA4EveryFailureBranchAttributionIsSafe(t *testing.T) {
	e := newEnv(t)
	// 外来账本 A（攻击者会拿它的 id 当路径参数）
	foreign, err := e.books.Create(e.owner, "外来账本A", model.BookTypeFamily)
	if err != nil {
		t.Fatalf("建外来账本失败：%v", err)
	}

	type attempt struct {
		name  string
		code  string
		email string // 兑换时提供的邮箱（email_taken 分支必须提供绑定邮箱才会走到占用检查）
	}
	var attempts []attempt

	// ① 邀请不存在
	attempts = append(attempts, attempt{name: "notfound", code: "0123456789abcdef0123456789abcdef"})

	// ② 跨账本邀请（邀请签在本账本，路径却填 A）——单独覆盖在下方的专用断言里
	// ③~⑦ 各类在本账本上制造的失败状态
	mk := func(state string) attempt {
		inv, err := e.members.CreateInvite(e.owner, e.bookID, model.RoleMember, "", "")
		if err != nil {
			t.Fatalf("签发邀请失败：%v", err)
		}
		switch state {
		case "expired":
			if _, err := e.engine.Exec("UPDATE invites SET expires_at = ? WHERE id = ?",
				time.Now().Unix()-60, inv.Id); err != nil {
				t.Fatalf("置过期失败：%v", err)
			}
		case "revoked":
			if err := e.members.RevokeInvite(e.owner, e.bookID, inv.Id); err != nil {
				t.Fatalf("撤销失败：%v", err)
			}
		case "used":
			if _, err := e.engine.Exec("UPDATE invites SET used_at = ? WHERE id = ?",
				time.Now().Unix(), inv.Id); err != nil {
				t.Fatalf("置已用失败：%v", err)
			}
		}
		return attempt{name: state, code: inv.Code}
	}
	for _, s := range []string{"expired", "revoked", "used"} {
		attempts = append(attempts, mk(s))
	}
	// 绑定不匹配
	if inv, err := e.members.CreateInvite(e.owner, e.bookID, model.RoleMember, "zz_bound_user", ""); err == nil {
		attempts = append(attempts, attempt{name: "bound_username", code: inv.Code})
	} else {
		t.Fatalf("签发绑定邀请失败：%v", err)
	}
	// 绑定邮箱撞库（email_taken）
	existing := &model.User{Username: "zz_taken_mail", PasswordHash: "x", Email: "zz-taken@example.com"}
	if _, err := e.engine.Insert(existing); err != nil {
		t.Fatalf("插入占位邮箱用户失败：%v", err)
	}
	if inv, err := e.members.CreateInvite(e.owner, e.bookID, model.RoleMember, "", "zz-taken@example.com"); err == nil {
		attempts = append(attempts, attempt{name: "email_taken", code: inv.Code, email: "zz-taken@example.com"})
	} else {
		t.Fatalf("签发邮箱邀请失败：%v", err)
	}

	// 第一轮：全部拿"外来账本 A"当路径参数（攻击者视角）——A 的审计流必须一行不增。
	// 每个尝试用不同的 /24（ipBucket 已聚合，同 /24 会共享 10 次/小时配额）。
	for i, a := range attempts {
		beforeForeign := countAudit(t, e.engine, "book_id = ?", foreign.Id)
		_, jerr := e.members.JoinViaInvite(foreign.Id, a.code, "zz_attacker_"+strconv.Itoa(i),
			a.email, "password123", "10."+strconv.Itoa(9+i)+".0.1")
		if jerr == nil {
			t.Fatalf("[%s] 用外来账本 A 兑换竟然成功（越权入户）", a.name)
		}
		afterForeign := countAudit(t, e.engine, "book_id = ?", foreign.Id)
		if afterForeign != beforeForeign {
			t.Fatalf("[%s] 失败尝试往外来账本 %d 的审计流写了 %d 行（%v）——"+
				"匿名者不得以 10 次/小时/IP 的代价污染任意账本的审计",
				a.name, foreign.Id, afterForeign-beforeForeign, jerr)
		}
	}

	// 未归因的尝试必须落在实例哨兵 0，并带 bookAttribution 标记
	if n := countAudit(t, e.engine, "book_id = ? AND before_json LIKE ?",
		model.AuditBookInstance, "%unattributable%"); n == 0 {
		t.Fatal("无法归因的兑换尝试没有落 0 号桶 + bookAttribution 标记")
	}

	// 第二轮：把每个邀请拿到**它自己所属的账本**（本账本）上兑换，验证"归因正确的分支
	// 落到邀请所属账本"，而不是被错误地也塞进 0 号桶。
	wantReason := map[string]string{
		"expired":        "invite_expired",
		"revoked":        "invite_revoked",
		"used":           "invite_already_used",
		"bound_username": "bound_mismatch",
		"email_taken":    "email_taken",
	}
	for i, a := range attempts {
		reason, ok := wantReason[a.name]
		if !ok {
			continue // notfound 分支在本账本上同样无法归因，已在上一轮覆盖
		}
		beforeOwn := countAudit(t, e.engine, "book_id = ? AND before_json LIKE ?", e.bookID, "%"+reason+"%")
		beforeForeign := countAudit(t, e.engine, "book_id = ?", foreign.Id)
		_, jerr := e.members.JoinViaInvite(e.bookID, a.code, "zz_own_"+strconv.Itoa(i),
			a.email, "password123", "10."+strconv.Itoa(40+i)+".0.1")
		if jerr == nil {
			t.Fatalf("[%s] 本账本上的失败邀请竟然兑换成功", a.name)
		}
		if got := countAudit(t, e.engine, "book_id = ? AND before_json LIKE ?", e.bookID,
			"%"+reason+"%"); got != beforeOwn+1 {
			t.Fatalf("[%s] reason=%s 的失败审计没有落到邀请所属账本（%d → %d，期望 +1）",
				a.name, reason, beforeOwn, got)
		}
		if got := countAudit(t, e.engine, "book_id = ?", foreign.Id); got != beforeForeign {
			t.Fatalf("[%s] 本账本的失败尝试污染了外来账本的审计流", a.name)
		}
	}
}

// ---------- A5：ipBucket 与 loginKey 同口径 ----------

// TestZZA5InviteJoinBucketMirrorsLoginKey 邀请入口的桶键必须与登录同口径（mapped 同桶、垃圾来源 fail-closed）。
func TestZZA5InviteJoinBucketMirrorsLoginKey(t *testing.T) {
	r, bookID := zzJoinRouter(t)

	// ① 原生 IPv4 打满 10 次/小时配额后，换 IPv4-mapped 形态必须仍被限流（Unmap 生效）
	for i := 0; i < 10; i++ {
		zzPostJoin(r, bookID, "zz_a5_native_"+strconv.Itoa(i), "198.18.7.9:9100")
	}
	mappedLimited := 0
	for i := 0; i < 3; i++ {
		if c := zzPostJoin(r, bookID, "zz_a5_mapped_"+strconv.Itoa(i), "[::ffff:198.18.7.9]:9100"); c == http.StatusTooManyRequests {
			mappedLimited++
		}
	}
	if mappedLimited == 0 {
		t.Fatal("IPv4-mapped IPv6 形态拿到了新配额：ipBucket 未做 Unmap，与 loginKey 口径不一致")
	}

	// ② 不可解析来源必须 fail-closed（共享 unknown 桶）
	junk := []string{"garbage-host:1234", "no-port", "999.999.999.999:1"}
	limited := 0
	for round := 0; round < 5; round++ {
		for _, addr := range junk {
			if c := zzPostJoin(r, bookID, "zz_a5_junk_"+strconv.Itoa(round)+"_"+addr, addr); c == http.StatusTooManyRequests {
				limited++
			}
		}
	}
	if limited == 0 {
		t.Fatal("不可解析来源的 15 次兑换没有一次 429：ipBucket 对解析失败值可能按原样分桶（fail-open）")
	}
}

// ---------- A2：已删账本失败关闭不得误伤活账本 ----------

// TestZZA2FailureClosedWithoutCollateralDamage A2 改动不能误伤活账本的三条读路径。
func TestZZA2FailureClosedWithoutCollateralDamage(t *testing.T) {
	e := newEnv(t)
	txs := service.NewTxService(e.engine, version.NewRepo())
	syncSvc := service.NewSyncService(e.engine, txs, nil)
	cat := service.NewCategoryService(e.engine, version.NewRepo())

	acct, err := service.NewAccountService(e.engine, version.NewRepo()).Create(e.owner, e.bookID,
		service.AccountCreateInput{Name: "活账本账户", Category: model.CategoryCash, Currency: "CNY"})
	if err != nil {
		t.Fatalf("建账户失败：%v", err)
	}
	catRow, err := cat.Create(e.owner, e.bookID, service.CategoryCreateInput{
		Name: "活账本分类", CatType: model.CategoryTypeExpense})
	if err != nil {
		t.Fatalf("建分类失败：%v", err)
	}
	if _, err := txs.Create(nil, e.owner, e.bookID, service.CreateRequest{
		ClientUuid: "zz-a2-live", Amount: 500, TxType: int(model.TxTypeExpense),
		AccountId: acct.Id, CategoryId: catRow.Id, TxTime: time.Now().Unix(),
	}); err != nil {
		t.Fatalf("建交易失败：%v", err)
	}

	// 活账本：List / Get / Pull 全部正常
	list, total, err := txs.List(e.owner, e.bookID, service.TxListQuery{Page: 1, Count: 10})
	if err != nil || total != 1 || len(list) != 1 {
		t.Fatalf("活账本 List 异常：total=%d len=%d err=%v", total, len(list), err)
	}
	if _, err := txs.Get(e.owner, e.bookID, list[0].Id); err != nil {
		t.Fatalf("活账本 Get 异常：%v", err)
	}
	if _, err := syncSvc.Pull(e.owner, e.bookID, 0, 100); err != nil {
		t.Fatalf("活账本 Pull 异常：%v", err)
	}

	// 删账本之后：三条路径都必须以"非成员"失败关闭（不是 panic / 500 / 半成功）
	if err := e.books.Delete(e.owner, e.bookID); err != nil {
		t.Fatalf("删账本失败：%v", err)
	}
	if _, _, err := txs.List(e.owner, e.bookID, service.TxListQuery{Page: 1, Count: 10}); !errors.Is(err, service.ErrNotMember) {
		t.Fatalf("已删账本 List 应报 ErrNotMember，实际 %v", err)
	}
	if _, err := txs.Get(e.owner, e.bookID, list[0].Id); !errors.Is(err, service.ErrNotMember) {
		t.Fatalf("已删账本 Get 应报 ErrNotMember，实际 %v", err)
	}
	if _, err := syncSvc.Pull(e.owner, e.bookID, 0, 100); !errors.Is(err, service.ErrNotMember) {
		t.Fatalf("已删账本 Pull 应报 ErrNotMember，实际 %v", err)
	}
}
