package service

// WBS 5.9b 交易更新（PUT）的真实 SQLite 集成测试。
// 需 CGO（gcc），由 CI 在 ubuntu-latest 执行。复用 tx_integration_test.go 的 testEnv
// 与 tx_delete_integration_test.go 的 insertUser / addMember / auditCount helper。
// 语义依据：docs/FastLedger-交易更新语义设计.md。

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"fastledger/internal/model"
)

func (e *testEnv) income(req CreateRequest) CreateRequest {
	req.TxType = int(model.TxTypeIncome)
	req.AccountId = e.assetID
	req.CategoryId = e.incomeCat
	req.TxTime = time.Now().Unix()
	req.Source = 0
	return req
}

func (e *testEnv) transfer(req CreateRequest) CreateRequest {
	req.TxType = int(model.TxTypeTransfer)
	req.AccountId = e.assetID
	req.ToAccountId = e.toAssetID
	req.CategoryId = 0
	req.TxTime = time.Now().Unix()
	req.Source = 0
	return req
}

// txRowByID 读取交易行（不过滤删除态，供墓碑断言使用）。
func txRowByID(e *testEnv, id int64) model.Transaction {
	e.t.Helper()
	var r model.Transaction
	has, err := e.engine.Where("id = ?", id).Get(&r)
	if err != nil || !has {
		e.t.Fatalf("读交易 %d 失败 err=%v has=%v", id, err, has)
	}
	return r
}

// upd 构造一个合法的全量更新命令（默认与"改金额为 2500"等价的支出命令）。
func (e *testEnv) upd(clientUuid string, txType int8, mutate func(*UpdateRequest)) UpdateRequest {
	r := UpdateRequest{
		ClientUuid:     clientUuid,
		TxType:         int(txType),
		AccountId:      e.assetID,
		ToAccountId:    0,
		Amount:         2500,
		CategoryId:     e.expenseCat,
		PaymentChannel: 0,
		TxTime:         time.Now().Unix(),
		TimezoneOffset: 0,
		Comment:        "更新后的备注",
	}
	if txType == model.TxTypeTransfer {
		r.CategoryId = 0
		r.ToAccountId = e.toAssetID
	}
	if txType == model.TxTypeIncome {
		r.CategoryId = e.incomeCat
	}
	if mutate != nil {
		mutate(&r)
	}
	return r
}

// TestUpdateSingleAmountNetBalance 单笔改金额：余额净额重算（-8000 → -2500）。
func TestUpdateSingleAmountNetBalance(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "u-1", Amount: 8000}))
	must(t, err)
	txID := res.TransactionIds[0]

	out, err := e.txs.UpdateReceipt(ctx, e.uid, e.bookID, txID, e.upd("u-op-1", model.TxTypeExpense, nil))
	must(t, err)
	if got := e.balance(e.assetID); got != -2500 {
		t.Fatalf("更新后资产余额应 -2500（净额 -8000+2500），实际 %d", got)
	}
	if out.Replayed {
		t.Fatal("首次更新不应是重放")
	}
	row := txRowByID(e, txID)
	if row.Amount != 2500 || row.SyncVersion != out.Version {
		t.Fatalf("行未按预期更新：amount=%d syncVersion=%d 期望 version=%d", row.Amount, row.SyncVersion, out.Version)
	}
	if n := auditCount(e, model.AuditActionUpdate); n != 1 {
		t.Fatalf("审计应有 1 行 update，实际 %d", n)
	}
	// before_json 必须是变更前快照（含旧金额 8000）
	var before model.Transaction
	var logs []model.AuditLog
	if err := e.engine.Where("book_id = ? AND entity_type = 'transaction' AND action = ?",
		e.bookID, model.AuditActionUpdate).Find(&logs); err != nil {
		t.Fatalf("读审计失败：%v", err)
	}
	if err := json.Unmarshal([]byte(logs[0].BeforeJson), &before); err != nil {
		t.Fatalf("before_json 反序列化失败：%v", err)
	}
	if before.Amount != 8000 || before.Id != txID {
		t.Fatalf("before_json 应为变更前快照（amount=8000 id=%d），实际 amount=%d id=%d", txID, before.Amount, before.Id)
	}
}

// TestUpdateCommentOnlySkipsAccountBump 仅改备注：余额净额为 0，账户行不得被触碰。
func TestUpdateCommentOnlySkipsAccountBump(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "u-cmt", Amount: 8000}))
	must(t, err)
	txID := res.TransactionIds[0]

	var beforeAcct model.Account
	has, err := e.engine.Where("id = ?", e.assetID).Get(&beforeAcct)
	must(t, err)
	if !has {
		t.Fatal("账户不存在")
	}

	// 仅改备注（金额与原值一致 → 净额为 0）
	_, err = e.txs.UpdateReceipt(ctx, e.uid, e.bookID, txID,
		e.upd("u-op-cmt", model.TxTypeExpense, func(r *UpdateRequest) { r.Amount = 8000 }))
	must(t, err)
	if got := e.balance(e.assetID); got != -8000 {
		t.Fatalf("仅改备注不应改变余额，实际 %d", got)
	}
	var afterAcct model.Account
	_, _ = e.engine.Where("id = ?", e.assetID).Get(&afterAcct)
	if afterAcct.SyncVersion != beforeAcct.SyncVersion || afterAcct.UpdatedAt != beforeAcct.UpdatedAt {
		t.Fatalf("净额为 0 的账户不应被更新：sync_version %d→%d updated_at %d→%d",
			beforeAcct.SyncVersion, afterAcct.SyncVersion, beforeAcct.UpdatedAt, afterAcct.UpdatedAt)
	}
}

// TestUpdateSingleAccountMovesBalance 单笔换账户：旧账户回滚、新账户施加。
func TestUpdateSingleAccountMovesBalance(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "u-move", Amount: 1000}))
	must(t, err)
	txID := res.TransactionIds[0]

	_, err = e.txs.UpdateReceipt(ctx, e.uid, e.bookID, txID,
		e.upd("u-op-move", model.TxTypeExpense, func(r *UpdateRequest) { r.AccountId = e.toAssetID; r.Amount = 1000 }))
	must(t, err)
	if got := e.balance(e.assetID); got != 0 {
		t.Fatalf("旧账户余额应回滚为 0，实际 %d", got)
	}
	if got := e.balance(e.toAssetID); got != -1000 {
		t.Fatalf("新账户余额应 -1000，实际 %d", got)
	}
}

// TestUpdateLiabilitySign 负债账户符号口径：信用卡支出欠款增加，改金额后按净额重算。
func TestUpdateLiabilitySign(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	req := e.expense(CreateRequest{ClientUuid: "u-liab", Amount: 5000})
	req.AccountId = e.liabID
	res, err := e.txs.Create(ctx, e.uid, e.bookID, req)
	must(t, err)
	txID := res.TransactionIds[0]
	if got := e.balance(e.liabID); got != 5000 {
		t.Fatalf("创建后负债余额应 +5000（正=欠款），实际 %d", got)
	}

	_, err = e.txs.UpdateReceipt(ctx, e.uid, e.bookID, txID,
		e.upd("u-op-liab", model.TxTypeExpense, func(r *UpdateRequest) { r.AccountId = e.liabID; r.Amount = 3000 }))
	must(t, err)
	if got := e.balance(e.liabID); got != 3000 {
		t.Fatalf("更新后负债余额应 +3000，实际 %d", got)
	}
}

// TestUpdateTransferBothSidesAndAccounts 转账改金额 + 改对手账户：整笔双侧联动、净额正确。
func TestUpdateTransferBothSidesAndAccounts(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, e.transfer(CreateRequest{ClientUuid: "u-tf", Amount: 5000}))
	must(t, err)
	outID, inID := res.TransactionIds[0], res.TransactionIds[1]

	// 改金额：5000 → 8000
	_, err = e.txs.UpdateReceipt(ctx, e.uid, e.bookID, outID, e.upd("u-op-tf1", model.TxTypeTransfer, func(r *UpdateRequest) { r.Amount = 8000 }))
	must(t, err)
	if got := e.balance(e.assetID); got != -8000 {
		t.Fatalf("转出账户余额应 -8000，实际 %d", got)
	}
	if got := e.balance(e.toAssetID); got != 8000 {
		t.Fatalf("转入账户余额应 8000，实际 %d", got)
	}
	for _, id := range []int64{outID, inID} {
		if row := txRowByID(e, id); row.Amount != 8000 {
			t.Fatalf("转账行 %d 金额应 8000，实际 %d", id, row.Amount)
		}
	}

	// 改对手账户（金额保持 8000）：asset→toAsset 改为 toAsset→asset（从 txID=转入侧行发起）
	_, err = e.txs.UpdateReceipt(ctx, e.uid, e.bookID, inID,
		e.upd("u-op-tf2", model.TxTypeTransfer, func(r *UpdateRequest) {
			r.Amount = 8000
			r.AccountId = e.toAssetID
			r.ToAccountId = e.assetID
		}))
	must(t, err)
	// 旧净额（asset -8000, toAsset +8000）回退后按新方向施加（toAsset -8000, asset +8000）
	if got := e.balance(e.assetID); got != 8000 {
		t.Fatalf("反向转账后 asset 余额应 8000，实际 %d", got)
	}
	if got := e.balance(e.toAssetID); got != -8000 {
		t.Fatalf("反向转账后 toAsset 余额应 -8000，实际 %d", got)
	}
	outRow, inRow := txRowByID(e, outID), txRowByID(e, inID)
	if outRow.AccountId != e.toAssetID || outRow.ToAccountId != e.assetID ||
		inRow.AccountId != e.assetID || inRow.ToAccountId != e.toAssetID {
		t.Fatalf("转账两侧账户未正确镜像：out(%d→%d) in(%d→%d)",
			outRow.AccountId, outRow.ToAccountId, inRow.AccountId, inRow.ToAccountId)
	}
	// 配对键不动
	if outRow.RelatedId != inID || inRow.RelatedId != outID {
		t.Fatalf("related_id 不应被更新：out.related=%d in.related=%d", outRow.RelatedId, inRow.RelatedId)
	}
	// 更新后的转账仍可整笔删除（配对完整性未被破坏）
	if _, err := e.txs.Delete(ctx, e.uid, e.bookID, outID, "u-tf-del"); err != nil {
		t.Fatalf("更新后的转账应仍可整笔删除：%v", err)
	}
	if got := e.balance(e.assetID); got != 0 {
		t.Fatalf("删除后 asset 余额应 0，实际 %d", got)
	}
}

// TestUpdateTypeChangeRejected 类型不可改：400 引导删除重建（设计 §2）。
func TestUpdateTypeChangeRejected(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "u-type", Amount: 1000}))
	must(t, err)
	txID := res.TransactionIds[0]

	// 改为收入：命令本身合法，仅在行级"类型不可改"校验处被拒
	_, err = e.txs.UpdateReceipt(ctx, e.uid, e.bookID, txID, e.upd("u-op-type", model.TxTypeIncome, nil))
	if !errors.Is(err, ErrInvalidCreate) || !strings.Contains(err.Error(), "类型不可修改") {
		t.Fatalf("改类型应返回 ErrInvalidCreate（含引导文案），实际 err=%v", err)
	}
	// 改为转账：命令本身合法（无分类、有对手账户），同样在行级被拒
	_, err = e.txs.UpdateReceipt(ctx, e.uid, e.bookID, txID,
		e.upd("u-op-type2", model.TxTypeTransfer, func(r *UpdateRequest) { r.TxType = int(model.TxTypeTransfer) }))
	if !errors.Is(err, ErrInvalidCreate) || !strings.Contains(err.Error(), "类型不可修改") {
		t.Fatalf("改类型（转账）应返回 ErrInvalidCreate，实际 err=%v", err)
	}
	if got := e.balance(e.assetID); got != -1000 {
		t.Fatalf("被拒的更新不应改变余额，实际 %d", got)
	}
}

// TestUpdateReplayIdempotent 同键同内容重放：版本不变、余额不变、不再写审计。
func TestUpdateReplayIdempotent(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "u-rp", Amount: 4000}))
	must(t, err)
	txID := res.TransactionIds[0]

	req := e.upd("u-op-rp", model.TxTypeExpense, func(r *UpdateRequest) { r.Amount = 4000 })
	first, err := e.txs.UpdateReceipt(ctx, e.uid, e.bookID, txID, req)
	must(t, err)
	second, err := e.txs.UpdateReceipt(ctx, e.uid, e.bookID, txID, req)
	must(t, err)
	if !second.Replayed || second.Version != first.Version {
		t.Fatalf("重放应返回 replayed=true 且同一版本：first=%+v second=%+v", first, second)
	}
	if got := e.balance(e.assetID); got != -4000 {
		t.Fatalf("重放不应改变余额，实际 %d", got)
	}
	if n := auditCount(e, model.AuditActionUpdate); n != 1 {
		t.Fatalf("重放不应再写审计，实际 %d 行", n)
	}
}

// TestUpdateSameUUIDDifferentContent 同键不同内容 → ErrCreateConflict（409）。
func TestUpdateSameUUIDDifferentContent(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "u-cf", Amount: 1000}))
	must(t, err)
	txID := res.TransactionIds[0]

	_, err = e.txs.UpdateReceipt(ctx, e.uid, e.bookID, txID, e.upd("u-op-cf", model.TxTypeExpense, nil))
	must(t, err)
	_, err = e.txs.UpdateReceipt(ctx, e.uid, e.bookID, txID, e.upd("u-op-cf", model.TxTypeExpense, func(r *UpdateRequest) { r.Amount = 9999 }))
	if !errors.Is(err, ErrCreateConflict) {
		t.Fatalf("同键不同内容应 ErrCreateConflict，实际 err=%v", err)
	}
}

// TestUpdatePermission member 仅能改本人；owner 可改成员的；非成员 403。
func TestUpdatePermission(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "u-perm", Amount: 1000}))
	must(t, err)
	txID := res.TransactionIds[0]

	memberUID := insertUser(t, e, "member1")
	addMember(t, e, memberUID, model.RoleMember)
	outsiderUID := insertUser(t, e, "outsider")

	// member 改 owner 的交易 → 403
	if _, err := e.txs.UpdateReceipt(ctx, memberUID, e.bookID, txID, e.upd("u-op-p1", model.TxTypeExpense, nil)); !errors.Is(err, ErrForbidden) {
		t.Fatalf("member 改他人交易应 ErrForbidden，实际 err=%v", err)
	}
	// 非成员 → 403
	if _, err := e.txs.UpdateReceipt(ctx, outsiderUID, e.bookID, txID, e.upd("u-op-p2", model.TxTypeExpense, nil)); !errors.Is(err, ErrForbidden) {
		t.Fatalf("非成员应 ErrForbidden，实际 err=%v", err)
	}
	// owner 改 member 的交易 → 允许
	mres, err := e.txs.Create(ctx, memberUID, e.bookID, e.expense(CreateRequest{ClientUuid: "u-perm-m", Amount: 2000}))
	must(t, err)
	if _, err := e.txs.UpdateReceipt(ctx, e.uid, e.bookID, mres.TransactionIds[0], e.upd("u-op-p3", model.TxTypeExpense, nil)); err != nil {
		t.Fatalf("owner 应可改成员交易：%v", err)
	}
	// member 改本人的 → 允许
	if _, err := e.txs.UpdateReceipt(ctx, memberUID, e.bookID, mres.TransactionIds[0], e.upd("u-op-p4", model.TxTypeExpense, func(r *UpdateRequest) { r.Amount = 2100 })); err != nil {
		t.Fatalf("member 应可改本人交易：%v", err)
	}
}

// TestUpdateDeletedTx404 已删交易不可更新：与删除/恢复同口径 404。
func TestUpdateDeletedTx404(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "u-d404", Amount: 1000}))
	must(t, err)
	txID := res.TransactionIds[0]
	_, err = e.txs.Delete(ctx, e.uid, e.bookID, txID, "u-op-del")
	must(t, err)

	if _, err := e.txs.UpdateReceipt(ctx, e.uid, e.bookID, txID, e.upd("u-op-404", model.TxTypeExpense, nil)); !errors.Is(err, ErrTransactionNotFound) {
		t.Fatalf("已删交易更新应 ErrTransactionNotFound，实际 err=%v", err)
	}
}

// TestUpdateBaseVersionOptimisticLock baseVersion 乐观锁：过期版本 → 409，匹配版本 → 成功。
func TestUpdateBaseVersionOptimisticLock(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "u-bv", Amount: 1000}))
	must(t, err)
	txID := res.TransactionIds[0]
	stale := txRowByID(e, txID).SyncVersion

	// 用当前行版本 → 成功
	if _, err := e.txs.UpdateReceipt(ctx, e.uid, e.bookID, txID,
		e.upd("u-op-bv1", model.TxTypeExpense, func(r *UpdateRequest) { r.BaseVersion = stale })); err != nil {
		t.Fatalf("基于最新版本的更新应成功：%v", err)
	}
	// 用旧版本重试（行已被上次更新推进）→ 409
	if _, err := e.txs.UpdateReceipt(ctx, e.uid, e.bookID, txID,
		e.upd("u-op-bv2", model.TxTypeExpense, func(r *UpdateRequest) { r.BaseVersion = stale })); !errors.Is(err, ErrConcurrentConflict) {
		t.Fatalf("过期 baseVersion 应 ErrConcurrentConflict，实际 err=%v", err)
	}
	if got := e.balance(e.assetID); got != -2500 {
		t.Fatalf("被拒的更新不应改变余额（应 -2500），实际 %d", got)
	}
}

// TestUpdateValidation 命令级校验：缺键/非法金额/转账与单笔字段约束。
func TestUpdateValidation(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "u-val", Amount: 1000}))
	must(t, err)
	txID := res.TransactionIds[0]

	cases := []struct {
		name string
		req  UpdateRequest
	}{
		{"缺 clientUuid", e.upd("", model.TxTypeExpense, nil)},
		{"金额非正", e.upd("v1", model.TxTypeExpense, func(r *UpdateRequest) { r.Amount = 0 })},
		{"金额超上限", e.upd("v2", model.TxTypeExpense, func(r *UpdateRequest) { r.Amount = maxAmountCents + 1 })},
		{"单笔带对手账户", e.upd("v3", model.TxTypeExpense, func(r *UpdateRequest) { r.ToAccountId = e.toAssetID })},
		{"转账带分类", e.upd("v4", model.TxTypeTransfer, func(r *UpdateRequest) { r.CategoryId = e.expenseCat })},
		{"转账同账户", e.upd("v5", model.TxTypeTransfer, func(r *UpdateRequest) { r.ToAccountId = e.assetID })},
	}
	for _, tc := range cases {
		if _, err := e.txs.UpdateReceipt(ctx, e.uid, e.bookID, txID, tc.req); err == nil {
			t.Fatalf("%s：应返回错误", tc.name)
		}
	}
	if got := e.balance(e.assetID); got != -1000 {
		t.Fatalf("被拒的更新不应改变余额，实际 %d", got)
	}
}

// TestUpdateSingleVersionBumpOnce 单笔更新只递增一次版本，交易行与账户行共用。
func TestUpdateSingleVersionBumpOnce(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "u-ver", Amount: 1000}))
	must(t, err)
	txID := res.TransactionIds[0]
	before := txRowByID(e, txID).SyncVersion

	out, err := e.txs.UpdateReceipt(ctx, e.uid, e.bookID, txID,
		e.upd("u-op-ver", model.TxTypeExpense, func(r *UpdateRequest) { r.Amount = 1200 }))
	must(t, err)
	if out.Version != before+1 {
		t.Fatalf("更新应恰好递增一个版本：%d → %d", before, out.Version)
	}
	row := txRowByID(e, txID)
	if row.SyncVersion != out.Version {
		t.Fatalf("交易行 sync_version 应等于回执版本：%d vs %d", row.SyncVersion, out.Version)
	}
	// 账户余额变了，账户行也应使用同一版本
	var acct model.Account
	if _, err := e.engine.Where("id = ?", e.assetID).Get(&acct); err != nil {
		t.Fatalf("读账户失败：%v", err)
	}
	if acct.SyncVersion != out.Version {
		t.Fatalf("账户行 sync_version 应等于回执版本：%d vs %d", acct.SyncVersion, out.Version)
	}
}
