package service

// 复核收尾（db66e27）的边界探针：C06 多级嵌套解挂、解挂的事务回滚、
// pushStatusOf 404 分支、C02 文案中性化。

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"fastledger/internal/model"
	"fastledger/internal/version"
)

// TestZZF06MultiLevelTombstoneChain 多级嵌套墓碑（父→子→孙全软删）后删最顶层父账户：
// ① R53 不变量必须保持——不得有任何行 parent_id 指向已删除的父账户；
// ② 直接子级的 parent_id 被置 0（解挂）；
// ③ 观察孙级：解挂 UPDATE 只匹配直接子级，孙级 parent_id 仍指向墓碑子级行（该行仍存在，
//
//	不是指向"已删除的行"，且无恢复入口 → 惰性引用，不构成 R53 意义上的悬空）。
func TestZZF06MultiLevelTombstoneChain(t *testing.T) {
	e := newTestEnv(t)
	as := NewAccountService(e.engine, version.NewRepo())
	p, err := as.Create(e.uid, e.bookID, AccountCreateInput{Name: "ZZF6祖父", Category: model.CategoryCash, Currency: "CNY"})
	must(t, err)
	c, err := as.Create(e.uid, e.bookID, AccountCreateInput{Name: "ZZF6父", Category: model.CategoryCash, Currency: "CNY", ParentId: p.Id})
	must(t, err)
	g, err := as.Create(e.uid, e.bookID, AccountCreateInput{Name: "ZZF6孙", Category: model.CategoryCash, Currency: "CNY", ParentId: c.Id})
	must(t, err)

	// 自底向上软删：删 c 前必须先软删 g（R53 活跃子账户检查）
	if _, err := as.Delete(e.uid, e.bookID, g.Id); err != nil {
		t.Fatalf("软删孙失败：%v", err)
	}
	if _, err := as.Delete(e.uid, e.bookID, c.Id); err != nil {
		t.Fatalf("软删父(中层)失败：%v", err)
	}
	// 删最顶层
	if _, err := as.Delete(e.uid, e.bookID, p.Id); err != nil {
		t.Fatalf("GPT-C06（方案一）：多级墓碑链下删除顶层父账户应成功，实际 %v", err)
	}

	// ① R53 不变量：无任何行（任何状态）parent_id 指向已删除的 p
	var refP int64
	if _, err := e.engine.SQL("SELECT COUNT(1) FROM accounts WHERE parent_id = ?", p.Id).Get(&refP); err != nil {
		t.Fatal(err)
	}
	if refP != 0 {
		t.Errorf("【可疑】删除顶层父账户后仍有 %d 行 parent_id 指向它（R53 不变量被破坏）", refP)
	}
	// ② 直接子级已解挂
	var cPid int64
	if _, err := e.engine.SQL("SELECT parent_id FROM accounts WHERE id = ?", c.Id).Get(&cPid); err != nil {
		t.Fatal(err)
	}
	if cPid != 0 {
		t.Errorf("GPT-C06（方案一）：直接子级 parent_id 应为 0，实际 %d", cPid)
	}
	// ③ 孙级观察：仍指向墓碑父(中层)行
	var gPid int64
	if _, err := e.engine.SQL("SELECT parent_id FROM accounts WHERE id = ?", g.Id).Get(&gPid); err != nil {
		t.Fatal(err)
	}
	var cExists int64
	if _, err := e.engine.SQL("SELECT COUNT(1) FROM accounts WHERE id = ?", c.Id).Get(&cExists); err != nil {
		t.Fatal(err)
	}
	var cDeleted bool
	if _, err := e.engine.SQL("SELECT deleted FROM accounts WHERE id = ?", c.Id).Get(&cDeleted); err != nil {
		t.Fatal(err)
	}
	t.Logf("孙级 parent_id=%d（指向墓碑父行 id=%d，该行存在=%d deleted=%v）", gPid, c.Id, cExists, cDeleted)
	if gPid == p.Id {
		t.Errorf("【可疑】孙级仍指向已删除的顶层父账户")
	}
	if gPid == c.Id && cExists == 1 && cDeleted {
		t.Logf("结论：解挂只处理【直接子级】；孙级引用的是仍存在的墓碑行而非已删除行，" +
			"R53 不变量（无行指向被删者）成立；因账户无恢复入口，该引用惰性、不可见（List 过滤 deleted=0）。" +
			"属可接受语义，但 account.go 注释宜写明「仅解挂直接子级」")
	}
	// 全链对客户端不可见
	var visible int64
	if _, err := e.engine.SQL("SELECT COUNT(1) FROM accounts WHERE book_id = ? AND deleted = 0 AND id IN (?, ?, ?)",
		e.bookID, p.Id, c.Id, g.Id).Get(&visible); err != nil {
		t.Fatal(err)
	}
	if visible != 0 {
		t.Errorf("【可疑】墓碑链不应出现在可见列表中，实际 %d 行", visible)
	}
}

// TestZZF06UnlinkRollsBackWithParentDelete ④ 解挂与删除同事务：删除因 R31
// （父账户仍被交易引用）失败时，已执行的解挂必须随事务回滚。
func TestZZF06UnlinkRollsBackWithParentDelete(t *testing.T) {
	e := newTestEnv(t)
	ctx := e.txs // 仅取非 nil 指针占位
	_ = ctx
	as := NewAccountService(e.engine, version.NewRepo())
	p, err := as.Create(e.uid, e.bookID, AccountCreateInput{Name: "ZZF6b父", Category: model.CategoryCash, Currency: "CNY"})
	must(t, err)
	c, err := as.Create(e.uid, e.bookID, AccountCreateInput{Name: "ZZF6b子", Category: model.CategoryCash, Currency: "CNY", ParentId: p.Id})
	must(t, err)
	if _, err := as.Delete(e.uid, e.bookID, c.Id); err != nil {
		t.Fatalf("软删子账户失败：%v", err)
	}
	// 让父账户命中 R31（仍有交易引用）→ 删除在解挂【之后】失败
	if _, err := e.txs.Create(nil, e.uid, e.bookID, CreateRequest{
		ClientUuid: "zzf6b-tx", TxType: int(model.TxTypeExpense),
		AccountId: p.Id, Amount: 100, CategoryId: e.expenseCat,
		PaymentChannel: 0, TxTime: time.Now().Unix(), Source: 0,
	}); err != nil {
		t.Fatalf("在父账户下创建交易失败：%v", err)
	}
	if _, err := as.Delete(e.uid, e.bookID, p.Id); !errors.Is(err, ErrAccountInUse) {
		t.Fatalf("[前置] 父账户应因交易引用被拒（R31），实际 %v", err)
	}
	// 关键断言：解挂已回滚——墓碑子账户的 parent_id 必须仍指向 p
	var pid int64
	if _, err := e.engine.SQL("SELECT parent_id FROM accounts WHERE id = ?", c.Id).Get(&pid); err != nil {
		t.Fatal(err)
	}
	if pid != p.Id {
		t.Errorf("【可疑】删除失败后解挂未被回滚：墓碑子账户 parent_id=%d（期望 %d）——"+
			"解挂与删除不同事务，会留下「父未删、子已解挂」的错位状态", pid, p.Id)
	}
	// 父账户本身也未删
	var pDeleted bool
	if _, err := e.engine.SQL("SELECT deleted FROM accounts WHERE id = ?", p.Id).Get(&pDeleted); err != nil {
		t.Fatal(err)
	}
	if pDeleted {
		t.Fatal("[前置] 父账户不应被删除")
	}
	t.Logf("结论：解挂发生在删除的同一事务内，R31 失败时随事务回滚，无错位状态")
}

// TestZZF06PushStatusNotFoundIsInvalid pushStatusOf 对 ErrTransactionNotFound 必须
// 归 invalid（作废该条），不得落 default 的"可重试"话术造成离线队列无限重试。
func TestZZF06PushStatusNotFoundIsInvalid(t *testing.T) {
	status, code, msg := pushStatusOf(ErrTransactionNotFound)
	if status != PushStatusInvalid {
		t.Errorf("GPT-C01 配套：ErrTransactionNotFound 应映射 invalid，实际 %q（%q / %q）", status, code, msg)
	}
	if status == PushStatusError {
		t.Errorf("【可疑】映射为可重试的 error 会造成离线队列无限重试")
	}
	// 对照：其他 404 族仍保持原口径
	if s, _, _ := pushStatusOf(ErrAccountNotFound); s != PushStatusInvalid {
		t.Errorf("对照失败：ErrAccountNotFound 应仍为 invalid，实际 %q", s)
	}
}

// TestZZF02DirectionMessageNeutral C02 文案中性化：409 文案不得再暗示"一定是转账"。
func TestZZF02DirectionMessageNeutral(t *testing.T) {
	raw, err := readSource("../api/common.go")
	if err != nil {
		t.Fatalf("读取 common.go 失败：%v", err)
	}
	if strings.Contains(raw, "该转账记录缺少方向标记") {
		t.Errorf("GPT-C02：ErrTxDirectionMissing 文案仍暗示一定是转账（旧文案未替换）")
	}
	if !strings.Contains(raw, "该交易记录的方向标记异常") {
		t.Errorf("GPT-C02：未找到中性化后的文案「该交易记录的方向标记异常」")
	}
	// 配平：该分支仍是 409
	if !strings.Contains(raw, "StatusConflict, \"该交易记录的方向标记异常") {
		t.Errorf("GPT-C02：中性化文案的 HTTP 状态码不再是 409")
	}
}

// readSource 读取仓库内源文件（相对当前测试包目录）。
func readSource(rel string) (string, error) {
	b, err := os.ReadFile(rel)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
