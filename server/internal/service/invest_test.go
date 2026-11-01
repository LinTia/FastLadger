package service

// 投资/持仓/净资产（WBS 6.4 / 6.5）服务层测试：真实 SQLite（CI/服务器执行）。
//
// 覆盖：标的 CRUD + 引用保护、持仓 decimal 数量/价格（禁 float）、非负校验、
// 版本递增、审计留痕、录价刷新持仓现价、市值/盈亏计算、
// 净资产公式（负债账户正=欠款；总资产=Σ资产+Σ持仓市值）。
import (
	"testing"
	"time"

	"fastledger/internal/model"
	"fastledger/internal/version"
)

func (e *testEnv) invest() *InvestService { return NewInvestService(e.engine, version.NewRepo()) }

func TestInstrumentCrudAndReferenceProtection(t *testing.T) {
	e := newTestEnv(t)
	iv := e.invest()
	i, err := iv.CreateInstrument(e.uid, e.bookID, InstrumentInput{Name: "贵州茅台", Type: model.InstrumentTypeStock, Market: "SH", Code: "600519"})
	if err != nil {
		t.Fatalf("建标的失败：%v", err)
	}
	if i.SyncVersion == 0 {
		t.Error("建标的应递增版本")
	}
	// 类型非法
	if _, err := iv.CreateInstrument(e.uid, e.bookID, InstrumentInput{Name: "x", Type: 9}); err == nil {
		t.Fatal("类型 9 应被拒绝")
	}
	// 有持仓时禁止删除
	if _, err := iv.AddHolding(e.uid, e.bookID, HoldingInput{
		AccountId: e.assetID, InstrumentId: i.Id, Quantity: "100", CostAmount: 170000000, CurrentPrice: "1700.00",
	}); err != nil {
		t.Fatalf("建持仓失败：%v", err)
	}
	if err := iv.DeleteInstrument(e.uid, e.bookID, i.Id); err != ErrInstrumentInUse {
		t.Fatalf("有持仓的标的删除应 ErrInstrumentInUse，得到 %v", err)
	}
	// 审计：create + holding create ≥ 2 条
	var n int64
	if _, err := e.engine.SQL(`SELECT COUNT(1) FROM audit_logs WHERE book_id = ? AND entity_type IN (?, ?)`,
		e.bookID, model.AuditEntityInstrument, model.AuditEntityHolding).Get(&n); err != nil {
		t.Fatalf("查审计失败：%v", err)
	}
	if n < 2 {
		t.Errorf("审计条数应 ≥2，得到 %d", n)
	}
}

func TestHoldingDecimalAndValidation(t *testing.T) {
	e := newTestEnv(t)
	iv := e.invest()
	i, _ := iv.CreateInstrument(e.uid, e.bookID, InstrumentInput{Name: "沪深300ETF", Type: model.InstrumentTypeFund, Code: "510300"})

	// 数量非法：非数字 / 负数 / 零
	for _, q := range []string{"abc", "-1", "0"} {
		if _, err := iv.AddHolding(e.uid, e.bookID, HoldingInput{AccountId: e.assetID, InstrumentId: i.Id, Quantity: q}); err == nil {
			t.Errorf("数量 %q 应被拒绝", q)
		}
	}
	// 价格负数
	if _, err := iv.AddHolding(e.uid, e.bookID, HoldingInput{AccountId: e.assetID, InstrumentId: i.Id, Quantity: "100", CurrentPrice: "-5"}); err == nil {
		t.Fatal("负价格应被拒绝")
	}
	// 成本负数
	if _, err := iv.AddHolding(e.uid, e.bookID, HoldingInput{AccountId: e.assetID, InstrumentId: i.Id, Quantity: "100", CostAmount: -1}); err == nil {
		t.Fatal("负成本应被拒绝")
	}
	// 账户不存在
	if _, err := iv.AddHolding(e.uid, e.bookID, HoldingInput{AccountId: 999999, InstrumentId: i.Id, Quantity: "100"}); err != ErrAccountNotFound {
		t.Fatalf("账户不存在应 ErrAccountNotFound，得到 %v", err)
	}
	// 高精度数量往返：1000.50 份 + 现价 3.99 元 → 市值 3991.995 元 = 399199.5 分 → Round(0) = 399200 分
	h, err := iv.AddHolding(e.uid, e.bookID, HoldingInput{
		AccountId: e.assetID, InstrumentId: i.Id, Quantity: "1000.50", CostAmount: 350000, CurrentPrice: "3.99",
	})
	if err != nil {
		t.Fatalf("建持仓失败：%v", err)
	}
	if h.Quantity != "1000.5" { // decimal 规范化
		t.Errorf("数量应规范化为 1000.5，得到 %s", h.Quantity)
	}
	views, err := iv.ListHoldings(e.uid, e.bookID)
	if err != nil {
		t.Fatalf("ListHoldings 失败：%v", err)
	}
	if len(views) != 1 {
		t.Fatalf("期望 1 条持仓，得到 %d", len(views))
	}
	if views[0].MarketValue != 399200 {
		t.Errorf("市值期望 399200 分（3992.00 元），得到 %d", views[0].MarketValue)
	}
	if views[0].ProfitLoss != 399200-350000 {
		t.Errorf("盈亏期望 49200 分，得到 %d", views[0].ProfitLoss)
	}
	if views[0].InstrumentName != "沪深300ETF" {
		t.Errorf("标的名映射错误：%q", views[0].InstrumentName)
	}
}

func TestSetPriceRefreshesHoldings(t *testing.T) {
	e := newTestEnv(t)
	iv := e.invest()
	i, _ := iv.CreateInstrument(e.uid, e.bookID, InstrumentInput{Name: "黄金ETF", Type: model.InstrumentTypePreciousMetal})
	if _, err := iv.AddHolding(e.uid, e.bookID, HoldingInput{
		AccountId: e.assetID, InstrumentId: i.Id, Quantity: "10", CostAmount: 50000, CurrentPrice: "5.00",
	}); err != nil {
		t.Fatalf("建持仓失败：%v", err)
	}
	// 录新价 6.50 → 持仓现价刷新 + 市值 65.00 元
	if err := iv.SetPrice(e.uid, e.bookID, i.Id, "6.50", 0); err != nil {
		t.Fatalf("SetPrice 失败：%v", err)
	}
	views, _ := iv.ListHoldings(e.uid, e.bookID)
	if views[0].CurrentPrice != "6.5" {
		t.Errorf("持仓现价应刷新为 6.5，得到 %s", views[0].CurrentPrice)
	}
	if views[0].MarketValue != 6500 {
		t.Errorf("市值期望 6500 分，得到 %d", views[0].MarketValue)
	}
	// 负价格拒绝
	if err := iv.SetPrice(e.uid, e.bookID, i.Id, "-1", 0); err == nil {
		t.Fatal("负价格应被拒绝")
	}
	// 不存在的标的
	if err := iv.SetPrice(e.uid, e.bookID, 999999, "1", 0); err != ErrInstrumentNotFound {
		t.Fatalf("应 ErrInstrumentNotFound，得到 %v", err)
	}
	// 价格历史 append
	var n int64
	if _, err := e.engine.SQL("SELECT COUNT(1) FROM instrument_prices WHERE instrument_id = ?", i.Id).Get(&n); err != nil {
		t.Fatalf("查行情失败：%v", err)
	}
	if n != 1 {
		t.Errorf("行情历史期望 1 行，得到 %d", n)
	}
}

func TestNetWorthFormula(t *testing.T) {
	e := newTestEnv(t)
	iv := e.invest()
	// 造数：资产端 income 2586050；负债端信用卡 expense 243680（正=欠款）
	card, err := NewAccountService(e.engine, version.NewRepo()).Create(e.uid, e.bookID,
		AccountCreateInput{Name: "信用卡", Category: model.CategoryCreditCard, Currency: "CNY"})
	if err != nil {
		t.Fatalf("建信用卡失败：%v", err)
	}
	if _, err := e.txs.Create(nil, e.uid, e.bookID, CreateRequest{
		ClientUuid: "nw-1", TxType: int(model.TxTypeIncome), Amount: 2586050, AccountId: e.assetID, CategoryId: e.incomeCat, TxTime: time.Now().Unix(),
	}); err != nil {
		t.Fatalf("建收入失败：%v", err)
	}
	if _, err := e.txs.Create(nil, e.uid, e.bookID, CreateRequest{
		ClientUuid: "nw-2", TxType: int(model.TxTypeExpense), Amount: 243680, AccountId: card.Id, CategoryId: e.expenseCat, TxTime: time.Now().Unix(),
	}); err != nil {
		t.Fatalf("建支出失败：%v", err)
	}
	// 投资账户 + 持仓
	acc, err := NewAccountService(e.engine, version.NewRepo()).Create(e.uid, e.bookID,
		AccountCreateInput{Name: "券商账户", Category: model.CategoryInvestment, Currency: "CNY"})
	if err != nil {
		t.Fatalf("建投资账户失败：%v", err)
	}
	i, _ := iv.CreateInstrument(e.uid, e.bookID, InstrumentInput{Name: "测试基金", Type: model.InstrumentTypeFund})
	if _, err := iv.AddHolding(e.uid, e.bookID, HoldingInput{
		AccountId: acc.Id, InstrumentId: i.Id, Quantity: "100", CostAmount: 100000, CurrentPrice: "12.34",
	}); err != nil {
		t.Fatalf("建持仓失败：%v", err)
	}

	sum, err := iv.NetWorth(e.uid, e.bookID)
	if err != nil {
		t.Fatalf("NetWorth 失败：%v", err)
	}
	// 持仓市值 = 100 × 12.34 × 100 = 123400 分
	if sum.HoldingsValue != 123400 {
		t.Errorf("持仓市值期望 123400 分，得到 %d", sum.HoldingsValue)
	}
	// 总资产 = 2586050（现金）+ 123400（市值）
	if sum.TotalAssets != 2586050+123400 {
		t.Errorf("总资产期望 %d，得到 %d", 2586050+123400, sum.TotalAssets)
	}
	// 总负债 = 243680（信用卡欠款，正数）
	if sum.TotalLiabilities != 243680 {
		t.Errorf("总负债期望 243680，得到 %d", sum.TotalLiabilities)
	}
	if sum.NetWorth != sum.TotalAssets-sum.TotalLiabilities {
		t.Errorf("净资产 = 总资产 − 总负债 不成立：%d != %d − %d",
			sum.NetWorth, sum.TotalAssets, sum.TotalLiabilities)
	}
	// member 可读（共享配置视图）
	u2 := &model.User{Username: "nw-member", PasswordHash: "x"}
	if _, err := e.engine.Insert(u2); err != nil {
		t.Fatalf("插入用户失败：%v", err)
	}
	e.addMemberReport(u2.Id, model.RoleMember)
	if _, err := iv.NetWorth(u2.Id, e.bookID); err != nil {
		t.Fatalf("member 读净资产不应报错：%v", err)
	}
}
