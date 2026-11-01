package service

// 报表与导出（WBS 6.1 / 6.2）服务层测试：真实 SQLite（CI 与服务器执行）。
//
// 覆盖：月度聚合正确性（转账不计收支）、分类占比排序与方向隔离、
// 可见性开关（member 关互看只聚合本人）、导出过滤/名称映射/字段取值、
// CSV BOM 与公式注入防护、金额元字符串化。
import (
	"strings"
	"testing"
	"time"

	"fastledger/internal/model"
	"fastledger/internal/version"
)

func mkReportSvc(e *testEnv) *ReportService { return NewReportService(e.engine) }

// addMemberReport 直插一个成员行（测试专用）。
func (e *testEnv) addMemberReport(uid int64, role int8) {
	e.t.Helper()
	now := time.Now().Unix()
	if _, err := e.engine.Insert(&model.BookMember{
		BookId: e.bookID, Uid: uid, Role: role, CanViewOthers: true, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		e.t.Fatalf("添加成员失败：%v", err)
	}
}

func (e *testEnv) createTxReport(txType int, amount int64, accountID, toID, catID int64, txTime int64, clientUuid string) {
	e.t.Helper()
	if _, err := e.txs.Create(nil, e.uid, e.bookID, CreateRequest{
		ClientUuid: clientUuid, TxType: txType, Amount: amount,
		AccountId: accountID, ToAccountId: toID, CategoryId: catID, TxTime: txTime,
	}); err != nil {
		e.t.Fatalf("创建交易失败：%v", err)
	}
}

// cstMonthStart ref 所在月的 1 号 0 点（CST）。
func cstMonthStart(ref time.Time) time.Time {
	l := ref.In(CSTZone())
	return time.Date(l.Year(), l.Month(), 1, 0, 0, 0, 0, CSTZone())
}

func TestReportMonthlyAggregates(t *testing.T) {
	e := newTestEnv(t)
	rs := mkReportSvc(e)
	start := cstMonthStart(time.Now()).Unix()
	// 本月：支出 1234 + 收入 5000 + 转账 10000（转账不计收支）
	e.createTxReport(int(model.TxTypeExpense), 1234, e.assetID, 0, e.expenseCat, start+3600, "rm-1")
	e.createTxReport(int(model.TxTypeIncome), 5000, e.assetID, 0, e.incomeCat, start+7200, "rm-2")
	e.createTxReport(int(model.TxTypeTransfer), 10000, e.assetID, e.toAssetID, 0, start+9000, "rm-3")
	// 上月：支出 777
	lastMonthStart := cstMonthStart(time.Now()).AddDate(0, -1, 0)
	e.createTxReport(int(model.TxTypeExpense), 777, e.assetID, 0, e.expenseCat, lastMonthStart.Add(time.Hour).Unix(), "rm-4")

	points, err := rs.Monthly(e.uid, e.bookID, 2)
	if err != nil {
		t.Fatalf("Monthly 失败：%v", err)
	}
	if len(points) != 2 {
		t.Fatalf("期望 2 个月度点，得到 %d", len(points))
	}
	cur, prev := points[1], points[0]
	if cur.Month != cstMonthStart(time.Now()).Format("2006-01") {
		t.Errorf("当前月标识错误：%s", cur.Month)
	}
	if cur.Income != 5000 || cur.Expense != 1234 {
		t.Errorf("本月聚合错误：income=%d expense=%d（转账 10000 不得计入收支）", cur.Income, cur.Expense)
	}
	if prev.Expense != 777 {
		t.Errorf("上月聚合错误：expense=%d", prev.Expense)
	}
}

func TestReportMonthlyVisibility(t *testing.T) {
	e := newTestEnv(t)
	rs := mkReportSvc(e)
	u2 := &model.User{Username: "member-report", PasswordHash: "x"}
	if _, err := e.engine.Insert(u2); err != nil {
		t.Fatalf("插入成员用户失败：%v", err)
	}
	e.addMemberReport(u2.Id, model.RoleMember)

	start := cstMonthStart(time.Now()).Unix()
	e.createTxReport(int(model.TxTypeExpense), 1000, e.assetID, 0, e.expenseCat, start+3600, "rv-1") // owner 的
	if _, err := e.txs.Create(nil, u2.Id, e.bookID, CreateRequest{
		ClientUuid: "rv-2", TxType: int(model.TxTypeExpense), Amount: 250,
		AccountId: e.assetID, CategoryId: e.expenseCat, TxTime: start + 7200,
	}); err != nil {
		t.Fatalf("member 记账失败：%v", err)
	}

	// 互看开启：member 聚合到 1250
	got, err := rs.Monthly(u2.Id, e.bookID, 1)
	if err != nil {
		t.Fatalf("Monthly(member) 失败：%v", err)
	}
	if got[len(got)-1].Expense != 1250 {
		t.Errorf("互看开启时 member 应聚合全本 1250，得到 %d", got[len(got)-1].Expense)
	}

	// 关闭互看：member 只能看到本人 250
	bs := NewBookService(e.engine, version.NewRepo())
	if _, err := bs.SetVisibility(e.uid, e.bookID, false); err != nil {
		t.Fatalf("SetVisibility 失败：%v", err)
	}
	got, err = rs.Monthly(u2.Id, e.bookID, 1)
	if err != nil {
		t.Fatalf("Monthly(member, 关互看) 失败：%v", err)
	}
	if got[len(got)-1].Expense != 250 {
		t.Errorf("关闭互看后 member 只应聚合本人 250，得到 %d（越权读取！）", got[len(got)-1].Expense)
	}
	// owner 不受开关影响
	gotO, err := rs.Monthly(e.uid, e.bookID, 1)
	if err != nil {
		t.Fatalf("Monthly(owner) 失败：%v", err)
	}
	if gotO[len(gotO)-1].Expense != 1250 {
		t.Errorf("owner 应聚合全本 1250，得到 %d", gotO[len(gotO)-1].Expense)
	}
}

func TestReportCategoryShare(t *testing.T) {
	e := newTestEnv(t)
	rs := mkReportSvc(e)
	cs := NewCategoryService(e.engine, version.NewRepo())
	transport, err := cs.Create(e.uid, e.bookID, CategoryCreateInput{Name: "交通", CatType: model.CategoryTypeExpense})
	if err != nil {
		t.Fatalf("建分类失败：%v", err)
	}
	start := cstMonthStart(time.Now()).Unix()
	e.createTxReport(int(model.TxTypeExpense), 3000, e.assetID, 0, e.expenseCat, start+3600, "rc-1")
	e.createTxReport(int(model.TxTypeExpense), 1200, e.assetID, 0, transport.Id, start+7200, "rc-2")
	// 收入不混入支出占比
	e.createTxReport(int(model.TxTypeIncome), 9000, e.assetID, 0, e.incomeCat, start+9000, "rc-3")

	month := cstMonthStart(time.Now()).Format("2006-01")
	rows, err := rs.CategoryShare(e.uid, e.bookID, month, model.TxTypeExpense)
	if err != nil {
		t.Fatalf("CategoryShare 失败：%v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("期望 2 个分类切片，得到 %d", len(rows))
	}
	if rows[0].Name != "餐饮" || rows[0].Total != 3000 {
		t.Errorf("第一位应为 餐饮/3000（降序），得到 %s/%d", rows[0].Name, rows[0].Total)
	}
	if rows[1].Name != "交通" || rows[1].Total != 1200 {
		t.Errorf("第二位应为 交通/1200，得到 %s/%d", rows[1].Name, rows[1].Total)
	}
	rowsIn, err := rs.CategoryShare(e.uid, e.bookID, month, model.TxTypeIncome)
	if err != nil {
		t.Fatalf("CategoryShare(income) 失败：%v", err)
	}
	if len(rowsIn) != 1 || rowsIn[0].Total != 9000 {
		t.Errorf("收入占比错误：%v", rowsIn)
	}
	// 转账方向 → 拒绝
	if _, err := rs.CategoryShare(e.uid, e.bookID, month, model.TxTypeTransfer); err == nil {
		t.Fatal("转账方向的分类占比应被拒绝")
	}
	// 非法月份 → 拒绝
	if _, err := rs.CategoryShare(e.uid, e.bookID, "2026/13", model.TxTypeExpense); err == nil {
		t.Fatal("非法月份应被拒绝")
	}
}

func TestExportRowsAndTruncation(t *testing.T) {
	e := newTestEnv(t)
	rs := mkReportSvc(e)
	start := cstMonthStart(time.Now()).Unix()
	e.createTxReport(int(model.TxTypeExpense), 1234, e.assetID, 0, e.expenseCat, start+3600, "re-1")
	e.createTxReport(int(model.TxTypeIncome), 5000, e.assetID, 0, e.incomeCat, start+7200, "re-2")
	e.createTxReport(int(model.TxTypeTransfer), 8000, e.assetID, e.toAssetID, 0, start+9000, "re-3")

	rows, truncated, err := rs.ExportRows(e.uid, e.bookID, ExportQuery{})
	if err != nil {
		t.Fatalf("ExportRows 失败：%v", err)
	}
	if truncated {
		t.Fatal("4 行不应截断")
	}
	if len(rows) != 4 { // 转账两条
		t.Fatalf("期望 4 行（转账两条），得到 %d", len(rows))
	}
	rows, _, err = rs.ExportRows(e.uid, e.bookID, ExportQuery{TxType: model.TxTypeExpense})
	if err != nil {
		t.Fatalf("ExportRows(expense) 失败：%v", err)
	}
	if len(rows) != 1 || rows[0].Tx.Amount != 1234 {
		t.Fatalf("支出过滤错误：%v", rows)
	}
	// 时间过滤（[start, start+3600] 闭区间 → 只命中第一笔 re-1；re-2 在 start+7200 被排除）
	rows, _, err = rs.ExportRows(e.uid, e.bookID, ExportQuery{MinTime: start, MaxTime: start + 3600})
	if err != nil {
		t.Fatalf("ExportRows(time) 失败：%v", err)
	}
	if len(rows) != 1 || rows[0].Tx.Amount != 1234 {
		t.Fatalf("时间过滤错误：%v", rows)
	}
	// 名称映射与单元格取值
	if rows[0].Cell("amount") != "12.34" {
		t.Errorf("金额元字符串错误：%s", rows[0].Cell("amount"))
	}
	if rows[0].Cell("account") != "现金" {
		t.Errorf("账户名映射错误：%s", rows[0].Cell("account"))
	}
	if rows[0].Cell("type") != "支出" {
		t.Errorf("类型名错误：%s", rows[0].Cell("type"))
	}
	if rows[0].Cell("category") != "餐饮" {
		t.Errorf("分类名映射错误：%s", rows[0].Cell("category"))
	}
	// 字段白名单
	if _, err := ParseFields("time,evil"); err == nil {
		t.Fatal("未知导出字段应被拒绝")
	}
	keys, err := ParseFields("")
	if err != nil || len(keys) != 9 {
		t.Fatalf("缺省字段应返回全 9 列，得到 %v / err=%v", keys, err)
	}
}

func TestBuildCSVAndSanitize(t *testing.T) {
	e := newTestEnv(t)
	rs := mkReportSvc(e)
	start := cstMonthStart(time.Now()).Unix()
	e.createTxReport(int(model.TxTypeExpense), 1234, e.assetID, 0, e.expenseCat, start+3600, "rb-1")
	rows, _, err := rs.ExportRows(e.uid, e.bookID, ExportQuery{})
	if err != nil {
		t.Fatalf("ExportRows 失败：%v", err)
	}
	fields, _ := ParseFields("")
	csvBytes, err := BuildCSV(rows, fields)
	if err != nil {
		t.Fatalf("BuildCSV 失败：%v", err)
	}
	if !strings.HasPrefix(string(csvBytes), "\uFEFF") {
		t.Fatal("CSV 缺少 UTF-8 BOM（Excel 直开会乱码）")
	}
	if !strings.Contains(string(csvBytes), "时间,类型,金额(元)") {
		t.Errorf("CSV 表头不符：%q", strings.SplitN(string(csvBytes), "\n", 2)[0])
	}
	if !strings.Contains(string(csvBytes), "12.34") {
		t.Error("CSV 金额缺失")
	}
	// 公式注入防护
	if got := sanitizeCSVCell("=cmd|' /C calc'!A0"); !strings.HasPrefix(got, "'=") {
		t.Errorf("公式注入未防护：%q", got)
	}
	for _, s := range []string{"+8613800000000", "-备注", "@import", "a\tb"} {
		if got := sanitizeCSVCell(s); !strings.HasPrefix(got, "'") {
			t.Errorf("%q 应前置单引号，得到 %q", s, got)
		}
	}
	if got := sanitizeCSVCell("普通文本"); got != "普通文本" {
		t.Errorf("普通文本不应改动：%q", got)
	}
	// JSON 导出可序列化
	fieldsSel, _ := ParseFields("type,amount")
	if _, err := BuildExportJSON(rows, fieldsSel); err != nil {
		t.Fatalf("BuildExportJSON 失败：%v", err)
	}
}

func TestCents2Yuan(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0.00"}, {5, "0.05"}, {50, "0.50"}, {1234, "12.34"},
		{123456789, "1,234,567.89"}, {-1250, "-12.50"},
	}
	for _, c := range cases {
		if got := cents2yuan(c.in); got != c.want {
			t.Errorf("cents2yuan(%d) = %q，期望 %q", c.in, got, c.want)
		}
	}
}
