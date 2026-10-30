package service

// 真实 SQLite 集成测试：覆盖此前一直"跑不了/没覆盖"的 9 项业务级断言。
//
// 这些用例依赖真实 CGO（gcc），本地无 gcc 的机器无法运行；由
// .github/workflows/ci.yml 在 ubuntu-latest runner 上执行。
// 它们与 database_test.go / migrate_test.go 一起，把"SQLite 是否真的可用"从
// [no test files] 变成真实跑起来的集成验证。

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"fastledger/internal/database"
	"fastledger/internal/migrate"
	"fastledger/internal/model"
	"fastledger/internal/version"

	"xorm.io/xorm"
)

// testEnv 一个干净的账本环境：真实临时库 + 应用全部迁移 + owner 成员 + 资产/负债账户 + 收/支分类。
type testEnv struct {
	t          *testing.T
	engine     *xorm.Engine
	uid        int64
	bookID     int64
	txs        *TxService
	assetID    int64
	toAssetID  int64
	liabID     int64
	expenseCat int64
	incomeCat  int64
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	engine := newTestEngine(t)
	uid, bookID := seedBook(t, engine)

	as := NewAccountService(engine, version.NewRepo())
	cs := NewCategoryService(engine, version.NewRepo())

	asset, err := as.Create(uid, bookID, AccountCreateInput{Name: "现金", Category: model.CategoryCash, Currency: "CNY"})
	must(t, err)
	toAsset, err := as.Create(uid, bookID, AccountCreateInput{Name: "银行卡", Category: model.CategoryDemandDeposit, Currency: "CNY"})
	must(t, err)
	liab, err := as.Create(uid, bookID, AccountCreateInput{Name: "信用卡", Category: model.CategoryCreditCard, Currency: "CNY"})
	must(t, err)
	exp, err := cs.Create(uid, bookID, CategoryCreateInput{Name: "餐饮", CatType: model.CategoryTypeExpense})
	must(t, err)
	inc, err := cs.Create(uid, bookID, CategoryCreateInput{Name: "工资", CatType: model.CategoryTypeIncome})
	must(t, err)

	return &testEnv{
		t:          t,
		engine:     engine,
		uid:        uid,
		bookID:     bookID,
		txs:        NewTxService(engine, version.NewRepo()),
		assetID:    asset.Id,
		toAssetID:  toAsset.Id,
		liabID:     liab.Id,
		expenseCat: exp.Id,
		incomeCat:  inc.Id,
	}
}

func newTestEngine(t *testing.T) *xorm.Engine {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "ledger_test.db")
	engine, err := database.Init(dbPath)
	if err != nil {
		t.Fatalf("database.Init 失败（CGO 是否可用？）：%v", err)
	}
	t.Cleanup(func() { _ = engine.Close() })
	if _, err := migrate.Apply(engine); err != nil {
		t.Fatalf("migrate.Apply 失败：%v", err)
	}
	return engine
}

func seedBook(t *testing.T, engine *xorm.Engine) (int64, int64) {
	t.Helper()
	u := &model.User{Username: "owner", PasswordHash: "x"}
	if _, err := engine.Insert(u); err != nil {
		t.Fatalf("插入用户失败：%v", err)
	}
	bs := NewBookService(engine, version.NewRepo())
	b, err := bs.Create(u.Id, "家账", model.BookTypeFamily)
	if err != nil {
		t.Fatalf("建账本失败：%v", err)
	}
	return u.Id, b.Id
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func (e *testEnv) balance(accountID int64) int64 {
	var a model.Account
	has, err := e.engine.Where("id = ? AND deleted = 0", accountID).Get(&a)
	if err != nil || !has {
		e.t.Fatalf("读账户失败 id=%d err=%v has=%v", accountID, err, has)
	}
	return a.Balance
}

func (e *testEnv) txCount() int64 {
	var n int64
	if _, err := e.engine.SQL("SELECT COUNT(1) FROM transactions WHERE book_id = ? AND deleted = 0", e.bookID).Get(&n); err != nil {
		e.t.Fatalf("统计交易失败：%v", err)
	}
	return n
}

func (e *testEnv) expense(req CreateRequest) CreateRequest {
	req.TxType = int(model.TxTypeExpense)
	req.AccountId = e.assetID
	req.CategoryId = e.expenseCat
	if req.PaymentChannel == 0 {
		req.PaymentChannel = 0
	}
	req.TxTime = time.Now().Unix()
	req.Source = 0
	return req
}

// TestExpenseDecreasesAssetBalance 支出使资产余额减少，且只写 1 条记录。
func TestExpenseDecreasesAssetBalance(t *testing.T) {
	e := newTestEnv(t)
	_, err := e.txs.Create(context.Background(), e.uid, e.bookID, e.expense(CreateRequest{
		ClientUuid: "exp-1", Amount: 10000,
	}))
	must(t, err)
	if got := e.balance(e.assetID); got != -10000 {
		t.Fatalf("资产账户余额应为 -10000，实际 %d", got)
	}
	if n := e.txCount(); n != 1 {
		t.Fatalf("交易数应为 1，实际 %d", n)
	}
}

// TestLiabilitySignExpenseIncreasesBalanceRepaymentClears 负债符号：
// 刷卡消费使"欠款"（正余额）增加；以收入入账信用卡还款使欠款归零。
func TestLiabilitySignExpenseIncreasesBalanceRepaymentClears(t *testing.T) {
	e := newTestEnv(t)
	_, err := e.txs.Create(context.Background(), e.uid, e.bookID, CreateRequest{
		ClientUuid: "liab-exp", TxType: int(model.TxTypeExpense),
		AccountId: e.liabID, Amount: 10000, CategoryId: e.expenseCat,
		PaymentChannel: int(model.PaymentChannelPOS), TxTime: time.Now().Unix(), Source: 0,
	})
	must(t, err)
	if got := e.balance(e.liabID); got != 10000 {
		t.Fatalf("刷卡后负债余额应为 +10000（欠款更多），实际 %d", got)
	}
	_, err = e.txs.Create(context.Background(), e.uid, e.bookID, CreateRequest{
		ClientUuid: "liab-repay", TxType: int(model.TxTypeIncome),
		AccountId: e.liabID, Amount: 10000, CategoryId: e.incomeCat,
		PaymentChannel: 0, TxTime: time.Now().Unix(), Source: 0,
	})
	must(t, err)
	if got := e.balance(e.liabID); got != 0 {
		t.Fatalf("还款后负债余额应为 0，实际 %d", got)
	}
}

// TestTransferBilateralAndRelatedPairing 转账一次写两条、两侧余额反向变动，
// 且两条记录的 related_id 互为对方主键（阶段 4 整笔删除/恢复依赖此分组键）。
func TestTransferBilateralAndRelatedPairing(t *testing.T) {
	e := newTestEnv(t)
	res, err := e.txs.Create(context.Background(), e.uid, e.bookID, CreateRequest{
		ClientUuid: "xfer-1", TxType: int(model.TxTypeTransfer),
		AccountId: e.assetID, ToAccountId: e.toAssetID, Amount: 5000,
		CategoryId: 0, PaymentChannel: 0, TxTime: time.Now().Unix(), Source: 0,
	})
	must(t, err)
	if len(res.TransactionIds) != 2 {
		t.Fatalf("转账应返回 2 个交易 ID，实际 %d", len(res.TransactionIds))
	}
	if got := e.balance(e.assetID); got != -5000 {
		t.Fatalf("转出账户余额应为 -5000，实际 %d", got)
	}
	if got := e.balance(e.toAssetID); got != 5000 {
		t.Fatalf("转入账户余额应为 +5000，实际 %d", got)
	}
	var rows []model.Transaction
	if err := e.engine.Where("book_id = ? AND deleted = 0 AND type = ?", e.bookID, model.TxTypeTransfer).Find(&rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("转账应写 2 行，实际 %d", len(rows))
	}
	var out, in *model.Transaction
	for i := range rows {
		if rows[i].AccountId == e.assetID {
			out = &rows[i]
		} else {
			in = &rows[i]
		}
	}
	if out == nil || in == nil {
		t.Fatal("未找到转出/转入记录")
	}
	if out.RelatedId != in.Id || in.RelatedId != out.Id {
		t.Fatalf("related_id 双向配对错误：out.rel=%d in.id=%d | in.rel=%d out.id=%d",
			out.RelatedId, in.Id, in.RelatedId, out.Id)
	}
}

// TestIdempotencySameClientUUIDNoDoubleCharge 同 clientUuid 重试：只产生 1 笔交易，
// 余额只减一次，第二次为重放（Replayed=true）。
func TestIdempotencySameClientUUIDNoDoubleCharge(t *testing.T) {
	e := newTestEnv(t)
	req := e.expense(CreateRequest{ClientUuid: "idem-1", Amount: 10000})
	r1, err := e.txs.Create(context.Background(), e.uid, e.bookID, req)
	must(t, err)
	if r1.Replayed {
		t.Fatal("首次提交不应是 Replayed")
	}
	r2, err := e.txs.Create(context.Background(), e.uid, e.bookID, req)
	must(t, err)
	if !r2.Replayed {
		t.Fatal("第二次同 clientUuid 应重放（Replayed=true）")
	}
	if e.balance(e.assetID) != -10000 {
		t.Fatalf("两次提交后余额应仍为 -10000，实际 %d", e.balance(e.assetID))
	}
	if n := e.txCount(); n != 1 {
		t.Fatalf("同 clientUuid 应只产生 1 笔交易，实际 %d", n)
	}
}

// TestIdempotencyConflictDifferentContent 同 clientUuid 异内容：返回 ErrCreateConflict（409），
// 且不改变已提交的余额。
func TestIdempotencyConflictDifferentContent(t *testing.T) {
	e := newTestEnv(t)
	_, err := e.txs.Create(context.Background(), e.uid, e.bookID, e.expense(CreateRequest{
		ClientUuid: "conflict-1", Amount: 10000,
	}))
	must(t, err)
	_, err = e.txs.Create(context.Background(), e.uid, e.bookID, e.expense(CreateRequest{
		ClientUuid: "conflict-1", Amount: 20000,
	}))
	if !errors.Is(err, ErrCreateConflict) {
		t.Fatalf("同 clientUuid 异内容应返回 ErrCreateConflict，实际 err=%v", err)
	}
	if e.balance(e.assetID) != -10000 {
		t.Fatalf("冲突请求不应改变余额，应为 -10000，实际 %d", e.balance(e.assetID))
	}
}

// TestTransferMissingAccountRollsBackNoPartialWrite 对手账户不存在：整笔失败，
// 不留下任何交易行，转出账户余额不受影响（验证"校验失败绝不部分写入"）。
func TestTransferMissingAccountRollsBackNoPartialWrite(t *testing.T) {
	e := newTestEnv(t)
	_, err := e.txs.Create(context.Background(), e.uid, e.bookID, CreateRequest{
		ClientUuid: "bad-xfer", TxType: int(model.TxTypeTransfer),
		AccountId: e.assetID, ToAccountId: 999999, Amount: 5000,
		CategoryId: 0, PaymentChannel: 0, TxTime: time.Now().Unix(), Source: 0,
	})
	if err == nil {
		t.Fatal("对手账户不存在应报错")
	}
	if n := e.txCount(); n != 0 {
		t.Fatalf("失败转账不应留下任何交易行，实际 %d", n)
	}
	if got := e.balance(e.assetID); got != 0 {
		t.Fatalf("失败转账后资产账户余额应为 0，实际 %d", got)
	}
}

// TestConcurrentDistinctClientUUIDConsistentBalance 并发提交（不同 clientUuid）：
// 在 MaxOpenConns=1 串行化下，最终余额与交易数必须严格等于"每笔原子写入的汇总"，
// 不允许出现丢失更新（lost update）。
func TestConcurrentDistinctClientUUIDConsistentBalance(t *testing.T) {
	e := newTestEnv(t)
	const n = 30
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := e.txs.Create(context.Background(), e.uid, e.bookID, e.expense(CreateRequest{
				ClientUuid: fmt.Sprintf("conc-%d", i), Amount: 100,
			}))
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("并发创建不应失败：%v", err)
		}
	}
	if got := e.balance(e.assetID); got != -int64(n)*100 {
		t.Fatalf("%d 笔并发支出后余额应为 -%d，实际 %d", n, n*100, got)
	}
	if cnt := e.txCount(); cnt != n {
		t.Fatalf("交易数应为 %d，实际 %d", n, cnt)
	}
}

// TestListFilterByTypeQuotedColumn List 按 type 过滤：验证 transactions 的 'type' 列
// （xorm 显式引用）可正确写入并被查询，返回正确的交易子集。
func TestListFilterByTypeQuotedColumn(t *testing.T) {
	e := newTestEnv(t)
	_, err := e.txs.Create(context.Background(), e.uid, e.bookID, e.expense(CreateRequest{
		ClientUuid: "lf-exp", Amount: 100,
	}))
	must(t, err)
	_, err = e.txs.Create(context.Background(), e.uid, e.bookID, CreateRequest{
		ClientUuid: "lf-inc", TxType: int(model.TxTypeIncome),
		AccountId: e.assetID, Amount: 100, CategoryId: e.incomeCat,
		PaymentChannel: 0, TxTime: time.Now().Unix(), Source: 0,
	})
	must(t, err)
	exp, _, err := e.txs.List(e.uid, e.bookID, TxListQuery{TxType: int8(model.TxTypeExpense)})
	must(t, err)
	if len(exp) != 1 {
		t.Fatalf("按 type=2 过滤应只返回 1 条，实际 %d", len(exp))
	}
}
