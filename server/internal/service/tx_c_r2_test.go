package service

// GPT-C01~C06 整改的针对性回归用例（第二轮修复）。
//
// 与 core_consistency_probe*_test.go 的关系：探针文件里的 4 条原"缺陷固化/观察"
// 用例已翻转断言方向成为回归测试；本文件补每个修复点的针对性口径：
//   - C01：终态不一致的重放必须 404；【主路径保护】删除成功后的离线重试（终态一致）
//     必须仍然成功重放——防止有人把状态检查挪到幂等检查之前；
//   - C02：txRowFlow 先 type 后 direction 的分派矩阵（纯函数单测）；
//   - C03：删除/恢复必须在同一条 UPDATE 里刷 updated_at；
//   - C04：DeleteReceipt/RestoreReceipt 的 replayed 标记（HTTP 层据此不广播）；
//   - C05：分页 OFFSET 溢出边界（含 pagingOffset 纯函数单测）；
//   - C06：软删子账户阻止父账户删除的全生命周期（软删 → 阻止 → 硬清后放行）。

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"fastledger/internal/model"
	"fastledger/internal/version"
)

// ---------- C01 幂等重放的终态一致性 ----------

// TestC01ReplayRejectedWhenTerminalStateMismatched 删除→恢复→再删除（同一 clientUuid）：
// 第三次 delete 命中首次删除的回执，但交易已被恢复（deleted=0），终态不符 → 404，
// 余额与行状态不得被改动，版本号不得回退。
func TestC01ReplayRejectedWhenTerminalStateMismatched(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "c01-create", Amount: 4200}))
	must(t, err)
	txID := res.TransactionIds[0]

	if _, err := e.txs.Delete(ctx, e.uid, e.bookID, txID, "c01-reuse"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.txs.Restore(ctx, e.uid, e.bookID, txID, "c01-other"); err != nil {
		t.Fatal(err)
	}
	ver, err := e.txs.Delete(ctx, e.uid, e.bookID, txID, "c01-reuse")
	if !errors.Is(err, ErrTransactionNotFound) {
		t.Fatalf("终态不符的重放应返回 ErrTransactionNotFound（404），实际 ver=%d err=%v", ver, err)
	}
	if got := e.balance(e.assetID); got != -4200 {
		t.Errorf("被拒的重放不得反转余额，实际 %d（期望 -4200）", got)
	}
	var deleted bool
	if _, err := e.engine.SQL("SELECT deleted FROM transactions WHERE id = ?", txID).Get(&deleted); err != nil {
		t.Fatal(err)
	}
	if deleted {
		t.Errorf("被拒的重放不得改动目标行，实际 deleted=%v", deleted)
	}
}

// TestC01ReplayRejectedWhenTerminalStateMismatchedTransfer 转账整组口径：
// 删除转账（uuid A）→ 恢复（uuid B）→ 再删（uuid A）→ 终态不符，必须 404。
func TestC01ReplayRejectedWhenTerminalStateMismatchedTransfer(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, CreateRequest{
		ClientUuid: "c01t-create", TxType: int(model.TxTypeTransfer),
		AccountId: e.assetID, ToAccountId: e.toAssetID, Amount: 1500,
		CategoryId: 0, PaymentChannel: 0, TxTime: time.Now().Unix(), Source: 0,
	})
	must(t, err)
	outID := res.TransactionIds[0]

	if _, err := e.txs.Delete(ctx, e.uid, e.bookID, outID, "c01t-A"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.txs.Restore(ctx, e.uid, e.bookID, outID, "c01t-B"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.txs.Delete(ctx, e.uid, e.bookID, outID, "c01t-A"); !errors.Is(err, ErrTransactionNotFound) {
		t.Fatalf("转账终态不符的重放应返回 ErrTransactionNotFound（404），实际 %v", err)
	}
	if e.balance(e.assetID) != -1500 || e.balance(e.toAssetID) != 1500 {
		t.Errorf("被拒的重放不得反转两侧余额：asset=%d to=%d（期望 -1500 / 1500）",
			e.balance(e.assetID), e.balance(e.toAssetID))
	}
}

// TestC01ReplayStillWorksAfterDelete 主路径保护（任务警告的回归钉）：
// 删除成功后客户端离线重试同一 clientUuid——行已是墓碑，但与 delete 回执的预期终态
// 【一致】，必须仍然重放成功（返回同一版本），余额只回滚一次。
// 若有人把状态检查挪到幂等检查之前，本用例立刻红。
func TestC01ReplayStillWorksAfterDelete(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "c01r-create", Amount: 6100}))
	must(t, err)
	txID := res.TransactionIds[0]

	v1, replayed1, err := e.txs.DeleteReceipt(ctx, e.uid, e.bookID, txID, "c01r-retry")
	if err != nil || replayed1 {
		t.Fatalf("首次删除应成功且非重放：ver=%d replayed=%v err=%v", v1, replayed1, err)
	}
	v2, replayed2, err := e.txs.DeleteReceipt(ctx, e.uid, e.bookID, txID, "c01r-retry")
	if err != nil {
		t.Fatalf("删除成功后的同键重试（离线重放主路径）不得失败：%v", err)
	}
	if !replayed2 {
		t.Error("同键重试应标记 replayed=true（调用方据此跳过广播）")
	}
	if v2 != v1 {
		t.Errorf("重放应返回原版本 %d，实际 %d（版本倒退/漂移都会拉坏客户端游标）", v1, v2)
	}
	if got := e.balance(e.assetID); got != 0 {
		t.Errorf("重放不得二次反转余额，实际 %d（期望 0）", got)
	}
	// 恢复方向同样保护：restore 成功后再重试 restore
	v3, replayed3, err := e.txs.RestoreReceipt(ctx, e.uid, e.bookID, txID, "c01r-res")
	if err != nil || replayed3 {
		t.Fatalf("首次恢复应成功且非重放：ver=%d replayed=%v err=%v", v3, replayed3, err)
	}
	v4, replayed4, err := e.txs.RestoreReceipt(ctx, e.uid, e.bookID, txID, "c01r-res")
	if err != nil || !replayed4 || v4 != v3 {
		t.Fatalf("恢复后的同键重试应成功重放（ver=%d replayed=%v err=%v，期望 ver=%d replayed=true）",
			v4, replayed4, err, v3)
	}
	if got := e.balance(e.assetID); got != -6100 {
		t.Errorf("恢复重放不得二次反转余额，实际 %d（期望 -6100）", got)
	}
}

// ---------- C02 txRowFlow 先 type 后 direction ----------

// TestC02TxRowFlowDispatchMatrix txRowFlow 分派矩阵（纯函数单测）：
// 转账只认 direction（0/越界即 ErrTxDirectionMissing）；非转账必须 direction=0；
// 被污染的单笔行显式报错，绝不静默按错误符号计算。
func TestC02TxRowFlowDispatchMatrix(t *testing.T) {
	base := model.Transaction{Id: 7, Amount: 1000}
	cases := []struct {
		name    string
		mut     func(r *model.Transaction)
		want    int64
		wantErr bool
	}{
		{"支出 direction=0 → -amount", func(r *model.Transaction) { r.TxType = model.TxTypeExpense; r.Direction = model.TxDirectionSingle }, -1000, false},
		{"收入 direction=0 → +amount", func(r *model.Transaction) { r.TxType = model.TxTypeIncome; r.Direction = model.TxDirectionSingle }, 1000, false},
		{"转账转出 direction=1 → -amount", func(r *model.Transaction) { r.TxType = model.TxTypeTransfer; r.Direction = model.TxDirectionOut }, -1000, false},
		{"转账转入 direction=2 → +amount", func(r *model.Transaction) { r.TxType = model.TxTypeTransfer; r.Direction = model.TxDirectionIn }, 1000, false},
		{"转账缺方向 → fail-closed", func(r *model.Transaction) { r.TxType = model.TxTypeTransfer; r.Direction = model.TxDirectionSingle }, 0, true},
		{"支出被污染 direction=1 → fail-closed", func(r *model.Transaction) { r.TxType = model.TxTypeExpense; r.Direction = model.TxDirectionOut }, 0, true},
		{"支出被污染 direction=2 → fail-closed", func(r *model.Transaction) { r.TxType = model.TxTypeExpense; r.Direction = model.TxDirectionIn }, 0, true},
		{"收入被污染 direction=2 → fail-closed", func(r *model.Transaction) { r.TxType = model.TxTypeIncome; r.Direction = model.TxDirectionIn }, 0, true},
		{"类型非法 → fail-closed", func(r *model.Transaction) { r.TxType = 9; r.Direction = model.TxDirectionSingle }, 0, true},
	}
	for _, c := range cases {
		r := base
		c.mut(&r)
		got, err := txRowFlow(&r)
		if c.wantErr {
			if !errors.Is(err, ErrTxDirectionMissing) {
				t.Errorf("%s：应 fail-closed 返回 ErrTxDirectionMissing，实际 flow=%d err=%v", c.name, got, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s：不应报错，实际 %v", c.name, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s：flow 期望 %d，实际 %d", c.name, c.want, got)
		}
	}
}

// TestC02PollutedSingleRowDeleteRejected 集成口径：direction 被污染的单笔支出，
// 删除必须 fail-closed（409），余额保持 -3000 不被多扣。
func TestC02PollutedSingleRowDeleteRejected(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "c02-create", Amount: 3000}))
	must(t, err)
	txID := res.TransactionIds[0]
	if _, err := e.engine.Exec("UPDATE transactions SET direction = ? WHERE id = ?",
		model.TxDirectionIn, txID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.txs.Delete(ctx, e.uid, e.bookID, txID, "c02-del"); !errors.Is(err, ErrTxDirectionMissing) {
		t.Fatalf("被污染的单笔删除应 ErrTxDirectionMissing（409），实际 %v", err)
	}
	if got := e.balance(e.assetID); got != -3000 {
		t.Errorf("被拒的删除不得改动余额，实际 %d（期望 -3000）", got)
	}
}

// ---------- C03 删除/恢复刷新 updated_at ----------

// TestC03DeleteRestoreTouchUpdatedAt 删除/恢复的裸 SQL UPDATE 必须显式刷 updated_at：
// 删除后 updated_at == deleted_at（同一时间源 now）；恢复后 updated_at > 0 且墓碑清零。
func TestC03DeleteRestoreTouchUpdatedAt(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "c03-create", Amount: 2100}))
	must(t, err)
	txID := res.TransactionIds[0]

	readMeta := func() (updatedAt, deletedAt int64, deleted bool) {
		t.Helper()
		var up, da int64
		var d bool
		if _, err := e.engine.SQL("SELECT updated_at FROM transactions WHERE id = ?", txID).Get(&up); err != nil {
			t.Fatal(err)
		}
		if _, err := e.engine.SQL("SELECT deleted_at FROM transactions WHERE id = ?", txID).Get(&da); err != nil {
			t.Fatal(err)
		}
		if _, err := e.engine.SQL("SELECT deleted FROM transactions WHERE id = ?", txID).Get(&d); err != nil {
			t.Fatal(err)
		}
		return up, da, d
	}

	if _, err := e.txs.Delete(ctx, e.uid, e.bookID, txID, "c03-del"); err != nil {
		t.Fatal(err)
	}
	upDel, tomb, deleted := readMeta()
	if !deleted {
		t.Fatal("[前置] 删除后 deleted 应为 true")
	}
	if tomb <= 0 {
		t.Fatalf("[前置] 删除后 deleted_at 应为墓碑时间，实际 %d", tomb)
	}
	if upDel != tomb {
		t.Errorf("GPT-C03：删除必须刷新 updated_at（与 tombstoneAt 同一时间源）：updated_at=%d deleted_at=%d", upDel, tomb)
	}

	// 时间戳按秒计：跨过 1 秒边界保证恢复的 updated_at 严格大于删除时的
	time.Sleep(1100 * time.Millisecond)
	if _, err := e.txs.Restore(ctx, e.uid, e.bookID, txID, "c03-res"); err != nil {
		t.Fatal(err)
	}
	upRes, tomb2, deleted2 := readMeta()
	if deleted2 || tomb2 != 0 {
		t.Fatalf("[前置] 恢复后 deleted 应为 false 且 deleted_at 清零：deleted=%v deleted_at=%d", deleted2, tomb2)
	}
	if upRes <= upDel {
		t.Errorf("GPT-C03：恢复必须刷新 updated_at：恢复后 %d 应大于删除时的 %d", upRes, upDel)
	}
}

// ---------- C04 重放标记（HTTP 层据此不广播） ----------

// TestC04MutateReceiptReplayedFlag DeleteReceipt/RestoreReceipt 的 replayed 标记口径：
// 首次提交 replayed=false；同键重放 replayed=true 且版本不变（配合 api 层源码守卫
// TestC04MutateReplayIsNotBroadcast 保证重放路径不广播 data_changed）。
func TestC04MutateReceiptReplayedFlag(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	res, err := e.txs.Create(ctx, e.uid, e.bookID, e.expense(CreateRequest{ClientUuid: "c04-create", Amount: 3300}))
	must(t, err)
	txID := res.TransactionIds[0]

	v1, rp1, err := e.txs.DeleteReceipt(ctx, e.uid, e.bookID, txID, "c04-key")
	if err != nil || rp1 {
		t.Fatalf("首次删除：replayed 应为 false，实际 ver=%d replayed=%v err=%v", v1, rp1, err)
	}
	v2, rp2, err := e.txs.DeleteReceipt(ctx, e.uid, e.bookID, txID, "c04-key")
	if err != nil || !rp2 || v2 != v1 {
		t.Fatalf("重放：应 replayed=true 且版本不变，实际 ver=%d replayed=%v err=%v（原 ver=%d）", v2, rp2, err, v1)
	}
	// 走旧签名 Delete 的重放同样返回原版本（兼容层不改变语义）
	v3, err := e.txs.Delete(ctx, e.uid, e.bookID, txID, "c04-key")
	if err != nil || v3 != v1 {
		t.Fatalf("Delete 兼容层重放应返回原版本，实际 ver=%d err=%v（原 ver=%d）", v3, err, v1)
	}
}

// ---------- C05 分页 OFFSET 溢出 ----------

// TestC05PagingOffsetBoundary pagingOffset 边界（纯函数单测）：
// page=MaxInt64/count 恰好不溢出；再大一页即 ErrInvalidArgument；非法入参同样拒绝。
func TestC05PagingOffsetBoundary(t *testing.T) {
	maxPage := math.MaxInt64 / 200
	if off, err := pagingOffset(1, 20); err != nil || off != 0 {
		t.Errorf("首页 offset 应为 0，实际 %d err=%v", off, err)
	}
	if off, err := pagingOffset(maxPage+1, 200); err != nil || off <= 0 {
		t.Errorf("page=%d（不溢出边界，offset=%d×200 仍在 int64 内）应成功，实际 offset=%d err=%v", maxPage+1, maxPage, off, err)
	}
	if _, err := pagingOffset(maxPage+2, 200); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("page=%d（溢出一页）应 ErrInvalidArgument，实际 %v", maxPage+2, err)
	}
	for _, c := range [][2]int{{0, 20}, {-1, 20}, {1, 0}, {1, -5}} {
		if _, err := pagingOffset(c[0], c[1]); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("pagingOffset(%d,%d) 应 ErrInvalidArgument，实际 %v", c[0], c[1], err)
		}
	}
	// 集成口径：List / ListDeleted 都走同一防线
	e := newTestEnv(t)
	if _, _, err := e.txs.List(e.uid, e.bookID, TxListQuery{Page: math.MaxInt64, Count: 200}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("List page=MaxInt64 应 ErrInvalidArgument，实际 %v", err)
	}
	if _, _, err := e.txs.ListDeleted(e.uid, e.bookID, math.MaxInt64, 200); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("ListDeleted page=MaxInt64 应 ErrInvalidArgument，实际 %v", err)
	}
}

// ---------- C06 软删子账户阻止父账户删除 ----------

// TestC06ParentDeleteBlockedByAnyStateChild 全生命周期：
// 活跃子账户阻止删除（R53）→ 软删子账户同样阻止（C06）→ 子账户硬清后父账户放行。
func TestC06ParentDeleteBlockedByAnyStateChild(t *testing.T) {
	e := newTestEnv(t)
	as := NewAccountService(e.engine, version.NewRepo())
	parent, err := as.Create(e.uid, e.bookID, AccountCreateInput{Name: "C06父", Category: model.CategoryCash, Currency: "CNY"})
	must(t, err)
	child, err := as.Create(e.uid, e.bookID, AccountCreateInput{Name: "C06子", Category: model.CategoryCash, Currency: "CNY", ParentId: parent.Id})
	must(t, err)

	if _, err := as.Delete(e.uid, e.bookID, parent.Id); !errors.Is(err, ErrAccountHasChildren) {
		t.Errorf("[R53 前置] 活跃子账户应阻止删除，实际 %v", err)
	}
	if _, err := e.engine.Exec("UPDATE accounts SET deleted = 1, deleted_at = ? WHERE id = ?",
		time.Now().Unix(), child.Id); err != nil {
		t.Fatal(err)
	}
	if _, err := as.Delete(e.uid, e.bookID, parent.Id); !errors.Is(err, ErrAccountHasChildren) {
		t.Fatalf("GPT-C06：已软删子账户也应阻止删除（恢复时会带悬空 parent_id），实际 %v", err)
	}
	// 子账户被硬清（回收站过期清除的等价态）后，父账户才可删
	if _, err := e.engine.Exec("DELETE FROM accounts WHERE id = ?", child.Id); err != nil {
		t.Fatal(err)
	}
	if _, err := as.Delete(e.uid, e.bookID, parent.Id); err != nil {
		t.Errorf("子账户硬清后父账户应可删除，实际 %v", err)
	}
}
