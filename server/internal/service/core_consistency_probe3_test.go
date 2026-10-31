package service

// 记账核心数据一致性复审探针（第三轮）：并发幂等、元数据更新、账户层级悬空引用。

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"fastledger/internal/model"
	"fastledger/internal/version"
)

// ---------- 探针 23：并发用同一 clientUuid 删除 —— 余额只许回滚一次 ----------

func TestProbeCore3ConcurrentSameUUIDDelete(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "p23-create", Amount: 5000}))
	must(t, err)
	txID := res.TransactionIds[0]

	const n = 10
	var wg sync.WaitGroup
	vers := make([]int64, n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			vers[i], errs[i] = e.txs.Delete(ctx, e.uid, e.bookID, txID, "p23-same")
		}(i)
	}
	wg.Wait()
	fail := 0
	for i := range errs {
		if errs[i] != nil {
			fail++
			t.Logf("goroutine %d 失败：%v", i, errs[i])
		}
	}
	if got := e.balance(e.assetID); got != 0 {
		t.Errorf("【可疑】%d 个并发同 uuid 删除后余额应为 0（只回滚一次），实际 %d", n-fail, got)
	}
	if n := auditCount(e, model.AuditActionDelete); n != 1 {
		t.Errorf("【可疑】并发同 uuid 删除应只写 1 行 delete 审计，实际 %d", n)
	}
	t.Logf("并发 delete：成功 %d / 失败 %d，版本=%v", n-fail, fail, vers)
}

// ---------- 探针 24：并发用同一 clientUuid 创建 —— 只许记一笔 ----------

func TestProbeCore3ConcurrentSameUUIDCreate(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	req := e.expense(CreateRequest{ClientUuid: "p24-same", Amount: 7000})
	const n = 10
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = e.txs.Create(ctx, e.uid, e.bookID, req)
		}(i)
	}
	wg.Wait()
	for i := range errs {
		if errs[i] != nil {
			t.Logf("goroutine %d：%v", i, errs[i])
		}
	}
	if got := e.balance(e.assetID); got != -7000 {
		t.Errorf("【可疑】并发同 uuid 创建后余额应为 -7000（只记一笔），实际 %d", got)
	}
	if c := e.txCount(); c != 1 {
		t.Errorf("【可疑】并发同 uuid 创建应只产生 1 笔交易，实际 %d", c)
	}
}

// ---------- 探针 25：删除/恢复是否更新 transactions.updated_at ----------

func TestProbeCore3UpdatedAtOnDeleteRestore(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "p25-create", Amount: 800}))
	must(t, err)
	txID := res.TransactionIds[0]
	var before int64
	if _, err := e.engine.SQL("SELECT updated_at FROM transactions WHERE id = ?", txID).Get(&before); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1100 * time.Millisecond)
	if _, err := e.txs.Delete(ctx, e.uid, e.bookID, txID, "p25-del"); err != nil {
		t.Fatal(err)
	}
	var afterDel int64
	if _, err := e.engine.SQL("SELECT updated_at FROM transactions WHERE id = ?", txID).Get(&afterDel); err != nil {
		t.Fatal(err)
	}
	t.Logf("created updated_at=%d → 删除后 updated_at=%d（差 %d 秒）", before, afterDel, afterDel-before)
	if afterDel == before {
		t.Logf("结论：删除走的是裸 SQL UPDATE SET deleted/deleted_at/sync_version，" +
			"绕过了 xorm 的 `xorm:\"updated\"` 自动列，updated_at 不刷新（元数据滞后，P2）")
	}
}

// ---------- 探针 26：父账户删除后不得留下悬空 parent_id ----------
//
// 【GPT-C06 已修复（裁决方案一：解挂）】子账户引用检查只挡**活跃**子账户（R53 原口径）；
// 删除父账户时同一事务内把墓碑子账户的 parent_id 置 0——既消除悬空引用，
// 又保住删除出口（账户墓碑没有硬清入口，"任何状态都阻止"会让父账户永久不可删）。

func TestProbeCore3ParentDeleteLeavesDanglingChild(t *testing.T) {
	e := newTestEnv(t)
	as := NewAccountService(e.engine, version.NewRepo())
	parent, err := as.Create(e.uid, e.bookID, AccountCreateInput{Name: "父账户", Category: model.CategoryCash, Currency: "CNY"})
	must(t, err)
	child, err := as.Create(e.uid, e.bookID, AccountCreateInput{Name: "子账户", Category: model.CategoryCash, Currency: "CNY", ParentId: parent.Id})
	must(t, err)
	// 先把子账户软删（正式入口，留下墓碑）
	if _, err := as.Delete(e.uid, e.bookID, child.Id); err != nil {
		t.Fatalf("软删子账户失败：%v", err)
	}
	// 父账户可删（墓被子账户被解挂，不再构成悬空）
	if _, err := as.Delete(e.uid, e.bookID, parent.Id); err != nil {
		t.Fatalf("GPT-C06（方案一）：软删子账户后父账户应可删除，实际 %v", err)
	}
	var pid int64
	if _, err := e.engine.SQL(
		"SELECT parent_id FROM accounts WHERE parent_id = ? AND id = ?", parent.Id, child.Id).Get(&pid); err != nil {
		t.Fatal(err)
	}
	if pid != 0 {
		t.Errorf("GPT-C06（方案一）：父账户删除后墓碑子账户仍带 parent_id=%d —— 悬空引用未消除", pid)
	}
}

// ---------- 探针 27：转账两侧 sync_version 不一致时是否放行（观察口径） ----------

func TestProbeCore3TransferMismatchedSyncVersion(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, CreateRequest{
		ClientUuid: "p27-create", TxType: int(model.TxTypeTransfer),
		AccountId: e.assetID, ToAccountId: e.toAssetID, Amount: 2000,
		CategoryId: 0, PaymentChannel: 0, TxTime: time.Now().Unix(), Source: 0,
	})
	must(t, err)
	outID, inID := res.TransactionIds[0], res.TransactionIds[1]
	// 人为把转入侧版本改大（模拟跨版本脏数据）
	if _, err := e.engine.Exec("UPDATE transactions SET sync_version = sync_version + 50 WHERE id = ?", inID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.txs.Delete(ctx, e.uid, e.bookID, outID, "p27-del"); err != nil {
		t.Logf("两侧版本不一致时删除被拒：%v", err)
	} else {
		t.Logf("结论：两侧 sync_version 不一致仍放行删除（配对校验不比对版本）；" +
			"删除后两侧统一到同一新版本，不会残留不一致（可接受）")
		if e.balance(e.assetID) != 0 || e.balance(e.toAssetID) != 0 {
			t.Errorf("【可疑】删除后余额未归零：%d / %d", e.balance(e.assetID), e.balance(e.toAssetID))
		}
	}
}

// ---------- 探针 28：单笔行被脏数据写入 direction 时，余额反向会静默算反 ----------
//
// 【GPT-C02 已修复】txRowFlow 现在**先按 TxType 分派**：非转账行必须 direction=0，
// 被污染即 fail-closed 报错（复用 ErrTxDirectionMissing 的 409 映射），绝不再默默按
// type/direction 混合解释把现金流符号算反。
// 与"迁移 0006 只回填 related_id>0 的转账"是两个独立问题（见复审报告第 3 部分）。

func TestProbeCore3SingleRowPollutedDirection(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "p28-create", Amount: 3000}))
	must(t, err)
	txID := res.TransactionIds[0]
	// 支出（type=2）余额此刻为 -3000；删除应回滚到 0
	// 人为把 direction 污染成"转入侧 2"（历史脏数据 / 误修复可能留下）
	if _, err := e.engine.Exec("UPDATE transactions SET direction = ? WHERE id = ?",
		model.TxDirectionIn, txID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.txs.Delete(ctx, e.uid, e.bookID, txID, "p28-del"); !errors.Is(err, ErrTxDirectionMissing) {
		t.Fatalf("GPT-C02：direction 被污染的单笔删除应 fail-closed 返回 ErrTxDirectionMissing（409），实际 %v", err)
	}
	if got := e.balance(e.assetID); got != -3000 {
		t.Errorf("GPT-C02：被拒的删除不得改动余额，实际 %d（期望 -3000）", got)
	}
}

// ---------- 探针 29：删除后跨账本越权 / 非成员 ----------

func TestProbeCore3CrossBookAndNonMember(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "p29-create", Amount: 400}))
	must(t, err)
	txID := res.TransactionIds[0]

	// 非成员
	outsider := insertUser(t, e, "p29-outsider")
	if _, err := e.txs.Delete(ctx, outsider, e.bookID, txID, "p29-out"); !errors.Is(err, ErrForbidden) {
		t.Errorf("【可疑】非成员删除应 ErrForbidden，实际 %v", err)
	}
	// 另一个账本（owner 本人）里不存在该交易
	bs := NewBookService(e.engine, version.NewRepo())
	b2, err := bs.Create(e.uid, "另一本", model.BookTypePersonal)
	must(t, err)
	if _, err := e.txs.Delete(ctx, e.uid, b2.Id, txID, "p29-cross"); !errors.Is(err, ErrTransactionNotFound) {
		t.Errorf("【可疑】跨账本删除应 ErrTransactionNotFound，实际 %v", err)
	}
}
