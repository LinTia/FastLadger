package service

// 预算（WBS 6.3）服务层测试：真实 SQLite（CI/服务器执行）。
//
// 覆盖：CRUD 基线、权限（member 写拒绝）、唯一性（同分类/总预算 409）、
// 校验（金额/周期/类型挂靠/收入分类拒绝）、进度聚合（大类+小类、转账不计、
// 半开区间）、可见性（关互看 member 只统计本人）、审计留痕。
import (
	"testing"
	"time"

	"fastledger/internal/model"
	"fastledger/internal/version"
)

func (e *testEnv) createBudget(t *testing.T, uid int64, in BudgetInput) *model.Budget {
	t.Helper()
	b, err := e.budgets().Create(uid, e.bookID, in)
	if err != nil {
		t.Fatalf("创建预算失败：%v", err)
	}
	return b
}

func (e *testEnv) budgets() *BudgetService { return NewBudgetService(e.engine) }

func TestBudgetCrudBaseline(t *testing.T) {
	e := newTestEnv(t)
	// 总预算 + 分类预算
	total := e.createBudget(t, e.uid, BudgetInput{BudgetType: model.BudgetTypeTotal, Amount: 300000})
	cat := e.createBudget(t, e.uid, BudgetInput{BudgetType: model.BudgetTypeCategory, CategoryId: e.expenseCat, Amount: 150000, Name: "餐饮预算"})
	if total.Id == 0 || cat.Id == 0 {
		t.Fatal("预算 ID 缺失")
	}

	items, err := e.budgets().List(e.uid, e.bookID)
	if err != nil {
		t.Fatalf("List 失败：%v", err)
	}
	if len(items) != 2 {
		t.Fatalf("期望 2 条预算，得到 %d", len(items))
	}

	// 更新金额
	updated, err := e.budgets().Update(e.uid, e.bookID, cat.Id, BudgetInput{Amount: 200000, Name: "餐饮预算"})
	if err != nil {
		t.Fatalf("Update 失败：%v", err)
	}
	if updated.Amount != 200000 {
		t.Errorf("更新后金额错误：%d", updated.Amount)
	}

	// 删除
	if err := e.budgets().Delete(e.uid, e.bookID, cat.Id); err != nil {
		t.Fatalf("Delete 失败：%v", err)
	}
	items, _ = e.budgets().List(e.uid, e.bookID)
	if len(items) != 1 {
		t.Fatalf("删除后应剩 1 条，得到 %d", len(items))
	}

	// 审计留痕（create/update/delete 各一条）
	var n int64
	if _, err := e.engine.SQL(
		`SELECT COUNT(1) FROM audit_logs WHERE entity_type = ? AND book_id = ?`,
		model.AuditEntityBudget, e.bookID).Get(&n); err != nil {
		t.Fatalf("查审计失败：%v", err)
	}
	if n != 4 { // total 创建 + cat 创建 + cat 更新 + cat 删除 = 4
		t.Fatalf("审计条数期望 4，得到 %d", n)
	}
}

func TestBudgetPermissionsAndValidation(t *testing.T) {
	e := newTestEnv(t)
	// member 写 → 拒绝
	u2 := &model.User{Username: "budget-member", PasswordHash: "x"}
	if _, err := e.engine.Insert(u2); err != nil {
		t.Fatalf("插入用户失败：%v", err)
	}
	e.addMemberReport(u2.Id, model.RoleMember)
	if _, err := e.budgets().Create(u2.Id, e.bookID, BudgetInput{BudgetType: model.BudgetTypeTotal, Amount: 100000}); err == nil {
		t.Fatal("member 建预算应被拒绝")
	}
	// member 读 → 允许
	if _, err := e.budgets().List(u2.Id, e.bookID); err != nil {
		t.Fatalf("member 读预算不应报错：%v", err)
	}

	// 金额非法
	if _, err := e.budgets().Create(e.uid, e.bookID, BudgetInput{BudgetType: model.BudgetTypeTotal, Amount: 0}); err == nil {
		t.Fatal("金额 0 应被拒绝")
	}
	// 周期非法（P2 预留）
	if _, err := e.budgets().Create(e.uid, e.bookID, BudgetInput{BudgetType: model.BudgetTypeTotal, Amount: 100000, PeriodType: 2}); err == nil {
		t.Fatal("period=季 应被拒绝（MVP 固定自然月）")
	}
	// 总预算挂分类 → 拒绝
	if _, err := e.budgets().Create(e.uid, e.bookID, BudgetInput{BudgetType: model.BudgetTypeTotal, CategoryId: e.expenseCat, Amount: 100000}); err == nil {
		t.Fatal("总预算挂分类应被拒绝")
	}
	// 分类预算挂收入分类 → 拒绝
	if _, err := e.budgets().Create(e.uid, e.bookID, BudgetInput{BudgetType: model.BudgetTypeCategory, CategoryId: e.incomeCat, Amount: 100000}); err == nil {
		t.Fatal("预算挂收入分类应被拒绝")
	}

	// 唯一性：同分类第二条、总预算第二条 → 409 语义
	e.createBudget(t, e.uid, BudgetInput{BudgetType: model.BudgetTypeTotal, Amount: 300000})
	if _, err := e.budgets().Create(e.uid, e.bookID, BudgetInput{BudgetType: model.BudgetTypeTotal, Amount: 400000}); err != ErrBudgetConflict {
		t.Fatalf("重复总预算应返回 ErrBudgetConflict，得到 %v", err)
	}
	e.createBudget(t, e.uid, BudgetInput{BudgetType: model.BudgetTypeCategory, CategoryId: e.expenseCat, Amount: 150000})
	if _, err := e.budgets().Create(e.uid, e.bookID, BudgetInput{BudgetType: model.BudgetTypeCategory, CategoryId: e.expenseCat, Amount: 100000}); err != ErrBudgetConflict {
		t.Fatalf("重复分类预算应返回 ErrBudgetConflict，得到 %v", err)
	}
}

func TestBudgetProgressAggregation(t *testing.T) {
	e := newTestEnv(t)
	cs := NewCategoryService(e.engine, version.NewRepo())
	food, err := cs.Create(e.uid, e.bookID, CategoryCreateInput{Name: "餐饮", CatType: model.CategoryTypeExpense})
	if err != nil {
		t.Fatalf("建大类失败：%v", err)
	}
	lunch, err := cs.Create(e.uid, e.bookID, CategoryCreateInput{Name: "午餐", ParentId: food.Id, CatType: model.CategoryTypeExpense})
	if err != nil {
		t.Fatalf("建小类失败：%v", err)
	}

	// 本月：午餐 400（小类）+ 餐饮 600（大类）= 1000；转账 500 不计
	start, end := currentMonthRangeCST()
	e.createTxReport(int(model.TxTypeExpense), 400, e.assetID, 0, lunch.Id, start+3600, "bp-1")
	e.createTxReport(int(model.TxTypeExpense), 600, e.assetID, 0, food.Id, start+7200, "bp-2")
	e.createTxReport(int(model.TxTypeTransfer), 500, e.assetID, e.toAssetID, 0, start+9000, "bp-3")
	// 上月支出不计入本月
	lastMonth := time.Unix(start, 0).AddDate(0, -1, 0)
	e.createTxReport(int(model.TxTypeExpense), 999, e.assetID, 0, food.Id, lastMonth.Add(time.Hour).Unix(), "bp-4")
	_ = end

	// 大类预算 1200 分（12 元）：已用 1000 分 → 83%（接近）
	e.createBudget(t, e.uid, BudgetInput{BudgetType: model.BudgetTypeCategory, CategoryId: food.Id, Amount: 1200})
	items, err := e.budgets().List(e.uid, e.bookID)
	if err != nil {
		t.Fatalf("List 失败：%v", err)
	}
	var view *BudgetView
	for i := range items {
		if items[i].CategoryId == food.Id {
			view = &items[i]
		}
	}
	if view == nil {
		t.Fatal("未找到餐饮预算")
	}
	if view.Spent != 1000 {
		t.Errorf("大类预算已用应为大类+小类 1000（转账不计），得到 %d", view.Spent)
	}
	if view.Percent != 83 {
		t.Errorf("百分比期望 83，得到 %d", view.Percent)
	}
	if view.Status != "near" {
		t.Errorf("83%% 应为 near，得到 %s", view.Status)
	}
	_ = view.CategoryName

	// 总预算 100000 分（1000 元）：本月已用 1000 分 → 1%
	e.createBudget(t, e.uid, BudgetInput{BudgetType: model.BudgetTypeTotal, Amount: 100000})
	items, _ = e.budgets().List(e.uid, e.bookID)
	for _, it := range items {
		if it.BudgetType == model.BudgetTypeTotal {
			if it.Spent != 1000 {
				t.Errorf("总预算已用期望 1000（本月全部支出），得到 %d", it.Spent)
			}
			if it.Status != "normal" {
				t.Errorf("1%% 应为 normal，得到 %s", it.Status)
			}
		}
	}

	// 超支态：把大类预算改小
	items, _ = e.budgets().List(e.uid, e.bookID)
	for i := range items {
		if items[i].CategoryId == food.Id {
			if _, err := e.budgets().Update(e.uid, e.bookID, items[i].Id, BudgetInput{Amount: 500}); err != nil {
				t.Fatalf("Update 失败：%v", err)
			}
		}
	}
	items, _ = e.budgets().List(e.uid, e.bookID)
	for _, it := range items {
		if it.CategoryId == food.Id {
			if it.Status != "over" {
				t.Errorf("200%% 应为 over，得到 %s（percent=%d）", it.Status, it.Percent)
			}
		}
	}
}

func TestBudgetProgressVisibility(t *testing.T) {
	e := newTestEnv(t)
	u2 := &model.User{Username: "budget-vis", PasswordHash: "x"}
	if _, err := e.engine.Insert(u2); err != nil {
		t.Fatalf("插入用户失败：%v", err)
	}
	e.addMemberReport(u2.Id, model.RoleMember)

	start, _ := currentMonthRangeCST()
	e.createTxReport(int(model.TxTypeExpense), 700, e.assetID, 0, e.expenseCat, start+3600, "bv-1") // owner 的
	if _, err := e.txs.Create(nil, u2.Id, e.bookID, CreateRequest{
		ClientUuid: "bv-2", TxType: int(model.TxTypeExpense), Amount: 300,
		AccountId: e.assetID, CategoryId: e.expenseCat, TxTime: start + 7200,
	}); err != nil {
		t.Fatalf("member 记账失败：%v", err)
	}

	e.createBudget(t, e.uid, BudgetInput{BudgetType: model.BudgetTypeCategory, CategoryId: e.expenseCat, Amount: 100000})
	// 互看开启：member 看到全本 1000
	items, err := e.budgets().List(u2.Id, e.bookID)
	if err != nil {
		t.Fatalf("List(member) 失败：%v", err)
	}
	if items[0].Spent != 1000 {
		t.Errorf("互看开启 member 应见 1000，得到 %d", items[0].Spent)
	}
	// 关闭互看：member 只统计本人 300
	bs := NewBookService(e.engine, version.NewRepo())
	if _, err := bs.SetVisibility(e.uid, e.bookID, false); err != nil {
		t.Fatalf("SetVisibility 失败：%v", err)
	}
	items, err = e.budgets().List(u2.Id, e.bookID)
	if err != nil {
		t.Fatalf("List(member, 关互看) 失败：%v", err)
	}
	if items[0].Spent != 300 {
		t.Errorf("关闭互看 member 只应统计本人 300，得到 %d（越权聚合！）", items[0].Spent)
	}
	// owner 不受影响
	items, _ = e.budgets().List(e.uid, e.bookID)
	if items[0].Spent != 1000 {
		t.Errorf("owner 应见 1000，得到 %d", items[0].Spent)
	}
}
