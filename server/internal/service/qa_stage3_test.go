package service

// QA 独立验收（阶段 3：可见性 + 同步）—— 不复制工程师的用例，只补「证明代码真的正确」的部分。
//
// 本机无 gcc、CGO 不可用（mattn/go-sqlite3 只能编译不能运行），故本文件分两类：
//   A. 纯逻辑用例：条件片段、字段映射、幂等摘要、限流归一化、状态映射；
//   B. **源码结构用例**（go/ast）：把"条件是否真的拼进了三处 SQL""COUNT 与列表是否同一条件"
//     "是否存在第二个创建入口"变成可执行断言。这类断言比"跑一次看结果"更强——它对未来
//     任何一次改动静默回归都生效，且不依赖数据库。
//
// 命名前缀 QAS3 避免与工程师用例冲突。

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// A. 纯逻辑用例
// ---------------------------------------------------------------------------

// TestQAS3VisibilityCondMatrix 可见性条件矩阵：三种角色组合 + creator_uid=0 反例。
//
// 反例构造思路（指南 §22.2 的归属哨兵）：`creator_uid = 0` 表示"阶段 2 之前写入的未知归属"。
// 若条件写成 `creator_uid IN (?, 0)` / `creator_uid = ? OR creator_uid = 0` / `creator_uid != ?`
// 等放行写法，member 就能读到不属于任何人的历史行。本用例逐个反例机制断言。
func TestQAS3VisibilityCondMatrix(t *testing.T) {
	// 1) owner / admin / 开关打开 → 不加条件
	if cond, args := txVisibilityCond(true, 42); cond != "" || len(args) != 0 {
		t.Fatalf("canViewAll=true 必须无附加条件，实际 cond=%q args=%v", cond, args)
	}

	// 2) member + 关闭互看 → 只本人且排除 0 号哨兵
	cond, args := txVisibilityCond(false, 42)
	if len(args) != 1 {
		t.Fatalf("参数个数应为 1，实际 %v", args)
	}
	if got, ok := args[0].(int64); !ok || got != 42 {
		t.Fatalf("参数应为 int64(42)，实际 %T(%v)", args[0], args[0])
	}
	// 反例清单：任一出现即视为"把 0 当放行"
	for _, bad := range []string{
		"IN (", "IN(", "OR ", "!=", ">=", "creator_uid = 0",
	} {
		if strings.Contains(strings.ToUpper(cond), strings.ToUpper(bad)) {
			t.Fatalf("可见性条件出现放行写法 %q：%q（creator_uid=0 的历史行会对 member 可见）", bad, cond)
		}
	}
	// 必须同时出现等值与排除哨兵两个子句
	if !strings.Contains(cond, "creator_uid = ?") || !strings.Contains(cond, "creator_uid <> 0") {
		t.Fatalf("条件必须为 `creator_uid = ? AND creator_uid <> 0`，实际 %q", cond)
	}
	// 两个子句必须是 AND 关系（不能被 OR 连起来）
	if strings.Contains(strings.ToUpper(cond), " OR ") {
		t.Fatalf("两个子句必须 AND，实际 %q", cond)
	}

	// 3) uid 非法（0 / 负数）→ 条件恒假，fail-closed（绝不退化为"不过滤"）
	for _, uid := range []int64{0, -1} {
		cond, args := txVisibilityCond(false, uid)
		if cond == "" || len(args) != 1 {
			t.Fatalf("uid=%d 仍必须返回过滤条件，实际 cond=%q args=%v", uid, cond, args)
		}
	}
}

// TestQAS3ToCreateRequestCoversEveryCreateRequestField 用反射做**字段级全覆盖**：
// CreateRequest 的每个字段都必须能从 PushItem 取到同名字段且值无损。
//
// 为什么比手工比对强：以后有人给 CreateRequest 加字段（例如阶段 4 的 relatedId）却忘了
// 给 PushItem 加，本用例立刻红——否则离线补传会静默丢字段（最危险的是丢字段导致
// 幂等摘要不变 → 重试命中上一笔回执，用户看到"补传成功"但没有这笔账）。
func TestQAS3ToCreateRequestCoversEveryCreateRequestField(t *testing.T) {
	crType := reflect.TypeOf(CreateRequest{})
	piType := reflect.TypeOf(PushItem{})

	// 构造一个"每个字段都不为零且互不相同"的 PushItem
	item := PushItem{}
	itemVal := reflect.ValueOf(&item).Elem()
	for i := 0; i < piType.NumField(); i++ {
		f := itemVal.Field(i)
		switch f.Kind() {
		case reflect.String:
			f.SetString("qa-" + piType.Field(i).Name)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			f.SetInt(int64(1000 + i))
		case reflect.Bool:
			f.SetBool(true)
		default:
			t.Fatalf("PushItem 出现未覆盖的字段类型 %s（请同步扩展本用例）", f.Kind())
		}
	}

	got := item.toCreateRequest()
	gotVal := reflect.ValueOf(got)

	missing := make([]string, 0)
	for i := 0; i < crType.NumField(); i++ {
		name := crType.Field(i).Name
		pf, ok := piType.FieldByName(name)
		if !ok {
			missing = append(missing, name)
			continue
		}
		// 类型必须相同（否则 JSON 反序列化会静默截断）
		if crType.Field(i).Type != pf.Type {
			t.Errorf("字段 %s 类型不一致：CreateRequest=%s / PushItem=%s",
				name, crType.Field(i).Type, pf.Type)
		}
		want := itemVal.FieldByName(name).Interface()
		if have := gotVal.FieldByName(name).Interface(); !reflect.DeepEqual(have, want) {
			t.Errorf("字段 %s 在 toCreateRequest 后失真：期望 %v，实际 %v", name, want, have)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("PushItem 缺少 CreateRequest 的字段 %v：离线补传会静默丢字段", missing)
	}

	// 反向：PushItem 不得有 CreateRequest 里没有的多余字段（会渲染成"客户端传了却被丢弃"）
	extra := make([]string, 0)
	for i := 0; i < piType.NumField(); i++ {
		if _, ok := crType.FieldByName(piType.Field(i).Name); !ok {
			extra = append(extra, piType.Field(i).Name)
		}
	}
	if len(extra) > 0 {
		t.Fatalf("PushItem 有 CreateRequest 不认识的字段 %v（会被静默丢弃）", extra)
	}
}

// TestQAS3IdempotencyHashIdenticalForPushAndHTTP 幂等摘要必须与"直接构造 CreateRequest"完全一致。
//
// 这是"重试能否命中同一回执"的充分条件：push 与 HTTP 两条路径算出同一个 hash，
// 唯一的幂等键是 (book_id, uid, client_uuid) + request_hash 核对。
func TestQAS3IdempotencyHashIdenticalForPushAndHTTP(t *testing.T) {
	item := PushItem{
		ClientUuid:     "  uuid-identical  ", // 前后空白也要被归一化掉
		TxType:         2,
		AccountId:      7,
		ToAccountId:    0,
		Amount:         12345,
		CategoryId:     33,
		PaymentChannel: 2,
		TxTime:         1793260800,
		TimezoneOffset: 480,
		Comment:        "  午饭  ",
		Source:         1,
	}
	direct := CreateRequest{
		ClientUuid:     item.ClientUuid,
		TxType:         item.TxType,
		AccountId:      item.AccountId,
		ToAccountId:    item.ToAccountId,
		Amount:         item.Amount,
		CategoryId:     item.CategoryId,
		PaymentChannel: item.PaymentChannel,
		TxTime:         item.TxTime,
		TimezoneOffset: item.TimezoneOffset,
		Comment:        item.Comment,
		Source:         item.Source,
	}

	hPush, err := createRequestHash(normalizeCreate(item.toCreateRequest()))
	if err != nil {
		t.Fatalf("push 路径摘要失败：%v", err)
	}
	hHTTP, err := createRequestHash(normalizeCreate(direct))
	if err != nil {
		t.Fatalf("HTTP 路径摘要失败：%v", err)
	}
	if hPush != hHTTP {
		t.Fatalf("两条入口的幂等摘要必须一致：push=%s http=%s", hPush, hHTTP)
	}
	if len(hPush) != 64 {
		t.Fatalf("摘要应为 64 位十六进制，实际 %q", hPush)
	}

	// 摘要必须覆盖每个业务字段：任一字段单独变化都要换摘要，否则"改了内容仍被判为重放"
	mutators := map[string]func(*CreateRequest){
		"txType":         func(r *CreateRequest) { r.TxType = 3; r.ToAccountId = 8; r.CategoryId = 0 },
		"accountId":      func(r *CreateRequest) { r.AccountId = 99 },
		"toAccountId":    func(r *CreateRequest) { r.TxType = 3; r.ToAccountId = 8; r.CategoryId = 0 },
		"amount":         func(r *CreateRequest) { r.Amount = 12346 },
		"categoryId":     func(r *CreateRequest) { r.CategoryId = 34 },
		"paymentChannel": func(r *CreateRequest) { r.PaymentChannel = 3 },
		"txTime":         func(r *CreateRequest) { r.TxTime = 1793260801 },
		"timezoneOffset": func(r *CreateRequest) { r.TimezoneOffset = 0 },
		"comment":        func(r *CreateRequest) { r.Comment = "晚饭" },
		"source":         func(r *CreateRequest) { r.Source = 0 },
	}
	names := make([]string, 0, len(mutators))
	for k := range mutators {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, name := range names {
		r := direct
		mutators[name](&r)
		h, err := createRequestHash(normalizeCreate(r))
		if err != nil {
			t.Fatalf("字段 %s 变体摘要失败：%v", name, err)
		}
		if h == hHTTP {
			t.Errorf("字段 %s 变化后摘要未变：该字段不参与幂等判定，改内容仍会被当成重放", name)
		}
	}

	// 操作键本身不参与摘要（它只用于定位回执）
	other := direct
	other.ClientUuid = "another-uuid"
	hOther, _ := createRequestHash(normalizeCreate(other))
	if hOther != hHTTP {
		t.Fatalf("clientUuid 不应参与业务摘要：%s != %s", hOther, hHTTP)
	}
}

// TestQAS3PushStatusContract push 逐条状态映射：五种状态可达 + 内部错误必须报 INTERNAL_ERROR。
//
// 注意 status 的取值故意放宽：本 QA 认为"未预期错误"不应伪装成业务冲突（见报告 P2-1），
// 若工程师改为独立的 `error` 状态，本用例仍然通过（只锁 ErrorCode，不锁 status）。
func TestQAS3PushStatusContract(t *testing.T) {
	allowed := map[string]bool{
		PushStatusCreated: true, PushStatusReplayed: true, PushStatusConflict: true,
		PushStatusInvalid: true, PushStatusForbidden: true,
		"error": true, // 建议新增的可重试状态（非业务冲突）
	}
	cases := []struct {
		name   string
		err    error
		status string
		code   string
	}{
		{"非成员", ErrCreateForbidden, PushStatusForbidden, ErrCodeForbidden},
		{"幂等内容冲突", ErrCreateConflict, PushStatusConflict, ErrCodeClientUUIDConflict},
		{"回执不完整", ErrIncompleteReceipt, PushStatusConflict, ErrCodeIncompleteReceipt},
		{"并发冲突", ErrConcurrentConflict, PushStatusConflict, ErrCodeConcurrentConflict},
		{"账户不存在", ErrAccountNotFound, PushStatusInvalid, ErrCodeAccountNotFound},
		{"分类不存在", ErrCategoryNotFound, PushStatusInvalid, ErrCodeCategoryNotFound},
		{"请求非法", fmt.Errorf("%w：缺 clientUuid", ErrInvalidCreate), PushStatusInvalid, ErrCodeInvalidRequest},
	}
	for _, c := range cases {
		status, code, msg := pushStatusOf(c.err)
		if status != c.status || code != c.code {
			t.Errorf("%s：得到 (%s,%s)，期望 (%s,%s)", c.name, status, code, c.status, c.code)
		}
		if !allowed[status] {
			t.Errorf("%s：status=%q 不在契约枚举内", c.name, status)
		}
		if msg == "" {
			t.Errorf("%s：message 不得为空（客户端至少要能给用户一句人话）", c.name)
		}
	}

	// 未预期错误：ErrorCode 必须是 INTERNAL_ERROR（可重试语义），message 不得泄露实现细节
	for _, err := range []error{
		errors.New("database is locked"),
		fmt.Errorf("写入幂等回执占位失败：%w", errors.New("disk I/O error")),
	} {
		status, code, msg := pushStatusOf(err)
		if code != ErrCodeInternal {
			t.Errorf("未预期错误应映射 %s，实际 %s（err=%v）", ErrCodeInternal, code, err)
		}
		if !allowed[status] {
			t.Errorf("未预期错误映射出非法 status=%q", status)
		}
		for _, leak := range []string{"database", "disk", "I/O", "sql", "SQL"} {
			if strings.Contains(msg, leak) {
				t.Errorf("message 泄露实现细节 %q：%q", leak, msg)
			}
		}
	}

	// 单笔成功/重放：Id 必须回带到第一个交易 ID（转账为转出记录）
	created := pushResultOf("u1", &CreateResult{TransactionIds: []int64{11}, Version: 5}, nil)
	if created.Status != PushStatusCreated || created.Id != 11 || created.Version != 5 {
		t.Fatalf("created 结果不符：%+v", created)
	}
	replayed := pushResultOf("u2", &CreateResult{TransactionIds: []int64{11, 12}, Version: 5, Replayed: true}, nil)
	if replayed.Status != PushStatusReplayed || replayed.Id != 11 || len(replayed.TransactionIds) != 2 {
		t.Fatalf("replayed 结果不符：%+v", replayed)
	}
	// 回带的 TransactionIds 必须是副本（调用方改它不得影响 CreateResult）
	res := &CreateResult{TransactionIds: []int64{1, 2}}
	out := pushResultOf("u3", res, nil)
	out.TransactionIds[0] = 999
	if res.TransactionIds[0] != 1 {
		t.Fatalf("TransactionIds 必须是拷贝，实际被别名共享：%v", res.TransactionIds)
	}
}

// TestQAS3NormalizeSyncLimitBoundaries limit 归一化边界（0/负数/1/500/501/极大/极小）
func TestQAS3NormalizeSyncLimitBoundaries(t *testing.T) {
	cases := []struct{ in, want int }{
		{0, MaxSyncBatch},
		{-1, MaxSyncBatch},
		{-1 << 30, MaxSyncBatch},
		{1, 1},
		{MaxSyncBatch - 1, MaxSyncBatch - 1},
		{MaxSyncBatch, MaxSyncBatch},
		{MaxSyncBatch + 1, MaxSyncBatch},
		{1 << 30, MaxSyncBatch},
	}
	for _, c := range cases {
		if got := normalizeSyncLimit(c.in); got != c.want {
			t.Errorf("normalizeSyncLimit(%d)=%d，期望 %d", c.in, got, c.want)
		}
	}
}

// TestQAS3PushPerItemIsolation 单项非法不得中断整批（硬验收 §3.2）。
//
// 这里用"引擎为空 + 全部项在 validateCreate 阶段就失败"的方式验证**顺序与隔离**：
// 每项都必须有独立结果、顺序保持、ClientUuid 回显。任何"提前 return"都会被本用例抓到。
func TestQAS3PushPerItemIsolation(t *testing.T) {
	s := &SyncService{txs: &TxService{}} // engine=nil：合法项会 panic，因此只放非法项
	items := []PushItem{
		{ClientUuid: "", Amount: 100, AccountId: 1, TxTime: 1, TxType: 2, CategoryId: 1},   // 缺 clientUuid
		{ClientUuid: "b", Amount: 0, AccountId: 1, TxTime: 1, TxType: 2, CategoryId: 1},    // 金额非法
		{ClientUuid: "c", Amount: 100, AccountId: 0, TxTime: 1, TxType: 2, CategoryId: 1},  // 账户缺失
		{ClientUuid: "d", Amount: 100, AccountId: 1, TxTime: 0, TxType: 2, CategoryId: 1},  // 时间缺失
		{ClientUuid: "e", Amount: 100, AccountId: 1, TxTime: 1, TxType: 9, CategoryId: 1},  // 类型越界
		{ClientUuid: "f", Amount: 100, AccountId: 1, TxTime: 1, TxType: 2, CategoryId: 0},  // 分类缺失
		{ClientUuid: "g", Amount: -5, AccountId: 1, TxTime: 1, TxType: 1, CategoryId: 1},   // 负金额
		{ClientUuid: "h", Amount: 100, AccountId: 1, TxTime: 1, TxType: 3, ToAccountId: 0}, // 转账缺对手户
	}
	results, err := s.Push(context.Background(), 1, 2, items)
	if err != nil {
		t.Fatalf("逐条失败不能升级为整批错误，实际 %v", err)
	}
	if len(results) != len(items) {
		t.Fatalf("必须返回与输入等长的结果：期望 %d，实际 %d", len(items), len(results))
	}
	for i, r := range results {
		if r.ClientUuid != items[i].ClientUuid {
			t.Errorf("第 %d 项 ClientUuid 回显错位：期望 %q，实际 %q", i, items[i].ClientUuid, r.ClientUuid)
		}
		if r.Status != PushStatusInvalid || r.ErrorCode != ErrCodeInvalidRequest {
			t.Errorf("第 %d 项应为 invalid/INVALID_REQUEST，实际 %s/%s", i, r.Status, r.ErrorCode)
		}
	}
}

// ---------------------------------------------------------------------------
// B. 源码结构用例（不依赖数据库）
// ---------------------------------------------------------------------------

// qas3Parse 解析本包内的源文件（测试工作目录 = 包目录）
func qas3Parse(t *testing.T, name string) *ast.File {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), filepath.Base(name), nil, 0)
	if err != nil {
		t.Fatalf("解析 %s 失败：%v", name, err)
	}
	return f
}

// qas3Func 按函数名（忽略接收者）查找
func qas3Func(f *ast.File, name string) *ast.FuncDecl {
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == name && fn.Body != nil {
			return fn
		}
	}
	return nil
}

// qas3Callees 收集函数体内所有被调用的名字（选择器取 Sel，标识符取本身）
func qas3Callees(fn *ast.FuncDecl) map[string]bool {
	out := make(map[string]bool)
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch f := call.Fun.(type) {
		case *ast.SelectorExpr:
			out[f.Sel.Name] = true
		case *ast.Ident:
			out[f.Name] = true
		}
		return true
	})
	return out
}

// TestQAS3VisibilityCondWiredIntoAllReadPaths 可见性条件必须真的拼进每一处读路径。
//
// 验收要点：List / Get / Pull 三个入口都必须调用 txVisibilityCond。
// 只"算出来不用"是本阶段最危险的实现错误（List 会变成绕过开关的后门）。
//
// 阶段 4 追加第四处：ListDeleted（回收站视图）。回收站同样受可见性开关约束——
// 否则"关闭互看"后，成员仍能从回收站看到别人删掉的账目明细。
func TestQAS3VisibilityCondWiredIntoAllReadPaths(t *testing.T) {
	txFile := qas3Parse(t, "tx.go")
	syncFile := qas3Parse(t, "sync.go")
	visFile := qas3Parse(t, "visibility.go")

	if qas3Func(visFile, "txVisibilityCond") == nil {
		t.Fatal("visibility.go 中找不到 txVisibilityCond")
	}

	for _, c := range []struct {
		file string
		ast  *ast.File
		fn   string
	}{
		{"tx.go", txFile, "List"},
		{"tx.go", txFile, "Get"},
		{"sync.go", syncFile, "Pull"},
		// 阶段 4：回收站视图也是读路径，必须同样按可见性过滤
		{"recycle.go", qas3Parse(t, "recycle.go"), "ListDeleted"},
	} {
		fn := qas3Func(c.ast, c.fn)
		if fn == nil {
			t.Fatalf("%s 中找不到 %s", c.file, c.fn)
		}
		if !qas3Callees(fn)["txVisibilityCond"] {
			t.Errorf("%s.%s 未调用 txVisibilityCond：该读路径不会按可见性过滤（越权读取他人明细）", c.file, c.fn)
		}
	}

	// txVisibilityCond 必须在 visibility.go 里定义，且**只能**由这些读路径使用（防止散落实现）
	users := qas3AllCallers(t, "txVisibilityCond")
	want := map[string]bool{"List": true, "Get": true, "Pull": true, "ListDeleted": true}
	for name := range users {
		if !want[name] {
			t.Errorf("txVisibilityCond 被非读路径 %q 调用：请确认没有第四处自定义过滤", name)
		}
	}
	for name := range want {
		if !users[name] {
			t.Errorf("txVisibilityCond 未被 %q 调用", name)
		}
	}
}

// TestQAS3ListCountUsesSameCondition COUNT 与列表查询必须用同一个 cond 变量。
//
// 若 COUNT 不含可见性条件，`total` 就会泄露"还有多少条你看不到"——
// 条数本身就是信息（可知对方记账频率）。这里机械断言两个 SQL 拼接的是同一个标识符。
func TestQAS3ListCountUsesSameCondition(t *testing.T) {
	fn := qas3Func(qas3Parse(t, "tx.go"), "List")
	if fn == nil {
		t.Fatal("tx.go 中找不到 List")
	}

	// 收集 "SELECT ... FROM transactions WHERE " + <ident> 的右侧标识符
	conds := make([]string, 0, 2)
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		bin, ok := n.(*ast.BinaryExpr)
		if !ok || bin.Op != token.ADD {
			return true
		}
		lit, ok := bin.X.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		if !strings.Contains(lit.Value, "FROM transactions WHERE ") {
			return true
		}
		if id, ok := bin.Y.(*ast.Ident); ok {
			conds = append(conds, id.Name)
		}
		return true
	})

	if len(conds) != 2 {
		t.Fatalf("应恰好有 COUNT 与列表两个查询，实际找到 %d 个（%v）", len(conds), conds)
	}
	if conds[0] != conds[1] {
		t.Fatalf("COUNT 与列表查询的条件不一致：%q vs %q（total 会泄露不可见条数）", conds[0], conds[1])
	}
}

// TestQAS3SingleCreateEntry 交易创建只能有一个入口。
//
// 断言"调用图闭环"：Create → {createSingleInTx, createTransferInTx} → {createChangeBalance, insertTxRow}，
// 且 createChangeBalance / insertTxRow 不得被闭环以外的函数调用。
// 任何新增的"第二个创建入口"都必然破坏这个闭环（要么绕过余额原子更新，要么绕过幂等回执）。
//
// 阶段 4 的澄清：Delete/Restore（经 mutateVisibility）是【变更】路径而非【创建】路径，
// 它不新增交易行、不走幂等创建回执，但需要反向余额——因此被显式授权调用
// createChangeBalance，并在此白名单内登记。它依旧不得调用 insertTxRow。
func TestQAS3SingleCreateEntry(t *testing.T) {
	// 各底层原语的【授权调用方】白名单（阶段 4 起分化）：
	//   - createChangeBalance（余额原子改动）：除两个创建 helper 外，额外允许阶段 4 的
	//     mutateVisibility——删除/恢复做的是"精确反向"，必须复用同一条原子 UPDATE，
	//     绝不许另起"先读余额再写回"的路径。
	//   - insertTxRow（写交易行）：仍然【只】允许两个创建 helper。
	//     删除/恢复不新增行，只改 deleted 标志，因此它出现在别处即为异常。
	allowedCallers := map[string]map[string]bool{
		"createChangeBalance": {"createSingleInTx": true, "createTransferInTx": true, "mutateVisibility": true},
		"insertTxRow":         {"createSingleInTx": true, "createTransferInTx": true},
	}
	for _, callee := range []string{"createChangeBalance", "insertTxRow"} {
		callers := qas3AllCallers(t, callee)
		want := allowedCallers[callee]
		for c := range callers {
			if !want[c] {
				t.Errorf("%s 被 %q 调用：出现了绕过 createSingleInTx/createTransferInTx 的写路径", callee, c)
			}
		}
		for c := range want {
			if !callers[c] {
				t.Errorf("%s 未被 %q 调用（用例与实现已脱节，请更新）", callee, c)
			}
		}
	}
	// 两个 helper 只能被唯一公开入口调用（注意还有测试文件，故只看 *_InTx 的生产调用方）
	for _, callee := range []string{"createSingleInTx", "createTransferInTx"} {
		callers := qas3AllCallers(t, callee)
		for c := range callers {
			if c != "Create" {
				t.Errorf("%s 被 %q 调用：只允许由 TxService.Create 调用", callee, c)
			}
		}
		if !callers["Create"] {
			t.Errorf("%s 未被 Create 调用（实现结构已变化，请更新用例）", callee)
		}
	}

	// sync.go 不得自建事务/写入：push 只能是"批量适配层"
	syncFile := qas3Parse(t, "sync.go")
	push := qas3Func(syncFile, "Push")
	if push == nil {
		t.Fatal("sync.go 中找不到 Push")
	}
	if !qas3Callees(push)["Create"] {
		t.Fatal("Push 未调用 Create：唯一创建入口被绕过或改名")
	}
	for _, forbidden := range []string{"Begin", "Exec", "Insert", "InsertOne", "Commit"} {
		if qas3Callees(push)[forbidden] {
			t.Fatalf("Push 调用了 %s：sync/push 不得自建事务或直写数据库", forbidden)
		}
	}
}

// qas3AllCallers 扫描本包所有生产源文件（排除 _test.go），返回 callee 的调用方集合
func qas3AllCallers(t *testing.T, callee string) map[string]bool {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("列目录失败：%v", err)
	}
	callers := make(map[string]bool)
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatalf("解析 %s 失败：%v", name, err)
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				switch f2 := call.Fun.(type) {
				case *ast.SelectorExpr:
					if f2.Sel.Name == callee {
						callers[fn.Name.Name] = true
					}
				case *ast.Ident:
					if f2.Name == callee {
						callers[fn.Name.Name] = true
					}
				}
				return true
			})
		}
	}
	return callers
}

// TestQAS3VisibilityNeverUsesMemberLevelOverride book_members.can_view_others 不得参与可见性判定。
//
// MVP 只启用账本级开关（PRD §7.3 粒度条款）。注释里提到该字段是允许的（说明"不启用"），
// 因此这里只看**代码**：AST 里的字符串字面量与标识符，不看注释文本。
func TestQAS3VisibilityNeverUsesMemberLevelOverride(t *testing.T) {
	f := qas3Parse(t, "visibility.go")

	sqlText := ""
	idents := make([]string, 0, 32)
	ast.Inspect(f, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.BasicLit:
			if v.Kind == token.STRING {
				sqlText += v.Value
			}
		case *ast.SelectorExpr:
			idents = append(idents, v.Sel.Name)
		case *ast.Ident:
			idents = append(idents, v.Name)
		}
		return true
	})

	for _, forbidden := range []string{"can_view_others", "CanViewOthers"} {
		for _, id := range idents {
			if id == forbidden {
				t.Fatalf("visibility.go 代码中出现成员级覆盖标识符 %q：MVP 只启用账本级开关", id)
			}
		}
	}
	// SQL 字面量里只能出现账本级列；出现 m.can_view_others 即为启用成员级覆盖
	if strings.Contains(sqlText, "m.can_view_others") || strings.Contains(sqlText, "m.CanViewOthers") {
		t.Fatalf("可见性 SQL 读取了成员级覆盖：%s", sqlText)
	}
	if !strings.Contains(sqlText, "b.members_can_view_others") {
		t.Fatalf("可见性 SQL 未读取账本级开关 books.members_can_view_others：%s", sqlText)
	}
	if !strings.Contains(sqlText, "creator_uid <> 0") {
		t.Fatalf("可见性 SQL 缺少 creator_uid <> 0（0 号哨兵会被当成放行）：%s", sqlText)
	}
}

// qas3ReadFile 读取本包源文件
func qas3ReadFile(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("读取 %s 失败：%v", name, err)
	}
	return string(b)
}
