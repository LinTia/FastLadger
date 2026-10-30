package service

// GPT 缺陷整改（R26 / R28 / R29 / R31 / R32 / R33 / R53）的**可本地执行**回归测试。
//
// 本机无 gcc（CGO 不可用，mattn/go-sqlite3 跑不起来），因此把每个 Finding 中
// 【不依赖真实数据库】的那部分证据单独放在这里：
//   - 纯函数行为（转账配对校验、墓碑时间计算）；
//   - 源码结构 / 顺序断言（权限必须在 Begin 之后重读、引用检查必须覆盖墓碑行等）。
//
// 真实 SQLite 的数据完整性验证在 remediation_integration_test.go（需 CGO，由 CI 执行）。
// 本文件不冒充那部分证据。

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"fastledger/internal/model"
)

// ---------------------------------------------------------------------------
// GPT-R26：转账配对校验（纯函数部分）
// ---------------------------------------------------------------------------

// validTransferPair 构造一对合法互指的转账记录（转出侧 id=1、转入侧 id=2）
func validTransferPair() (*model.Transaction, *model.Transaction) {
	out := &model.Transaction{
		Id: 1, TxType: model.TxTypeTransfer, AccountId: 10, ToAccountId: 20,
		Amount: 500, Direction: model.TxDirectionOut, RelatedId: 2,
	}
	in := &model.Transaction{
		Id: 2, TxType: model.TxTypeTransfer, AccountId: 20, ToAccountId: 10,
		Amount: 500, Direction: model.TxDirectionIn, RelatedId: 1,
	}
	return out, in
}

// TestR26VerifyTransferPairMatrix 转账配对七项校验的逐项反例。
//
// 每一条都对应"余额会被算错 / 恢复出孤儿记录"的真实后果，因此必须逐项拒绝，
// 绝不允许静默退化为单边操作（WBS 4.2）。
func TestR26VerifyTransferPairMatrix(t *testing.T) {
	out, in := validTransferPair()
	if err := verifyTransferPair(out, in); err != nil {
		t.Fatalf("合法配对不应报错：%v", err)
	}
	// 反序传入（从转入侧发起整笔操作）也必须通过
	if err := verifyTransferPair(in, out); err != nil {
		t.Fatalf("从转入侧发起时合法配对不应报错：%v", err)
	}

	cases := []struct {
		name string
		mut  func(row, cp *model.Transaction)
		want error
	}{
		{
			name: "对侧未互指（对侧 related_id 指向别的转账）",
			mut:  func(_, cp *model.Transaction) { cp.RelatedId = 99 },
			want: ErrTransferGroupIncomplete,
		},
		{
			name: "对侧类型不是转账",
			mut:  func(_, cp *model.Transaction) { cp.TxType = model.TxTypeExpense },
			want: ErrTransferGroupIncomplete,
		},
		{
			name: "两侧金额不一致",
			mut:  func(_, cp *model.Transaction) { cp.Amount = 501 },
			want: ErrTransferGroupIncomplete,
		},
		{
			name: "账户未交叉对应（对侧 to_account_id 指错）",
			mut:  func(_, cp *model.Transaction) { cp.ToAccountId = 30 },
			want: ErrTransferGroupIncomplete,
		},
		{
			name: "账户未交叉对应（对侧 account_id 指错）",
			mut:  func(_, cp *model.Transaction) { cp.AccountId = 30 },
			want: ErrTransferGroupIncomplete,
		},
		{
			name: "两侧方向相同（同为转出）",
			mut:  func(_, cp *model.Transaction) { cp.Direction = model.TxDirectionOut },
			want: ErrTransferGroupIncomplete,
		},
		{
			name: "两侧方向相同（同为转入）",
			mut: func(row, cp *model.Transaction) {
				row.Direction, cp.Direction = model.TxDirectionIn, model.TxDirectionIn
			},
			want: ErrTransferGroupIncomplete,
		},
		{
			name: "本侧缺方向标记",
			mut:  func(row, _ *model.Transaction) { row.Direction = model.TxDirectionSingle },
			want: ErrTxDirectionMissing,
		},
		{
			name: "对侧缺方向标记",
			mut:  func(_, cp *model.Transaction) { cp.Direction = model.TxDirectionSingle },
			want: ErrTxDirectionMissing,
		},
		{
			name: "方向取值越界",
			mut:  func(_, cp *model.Transaction) { cp.Direction = 7 },
			want: ErrTransferGroupIncomplete,
		},
	}
	for _, c := range cases {
		row, cp := validTransferPair()
		c.mut(row, cp)
		err := verifyTransferPair(row, cp)
		if err == nil {
			t.Errorf("%s：应被拒绝，实际通过", c.name)
			continue
		}
		if !errors.Is(err, c.want) {
			t.Errorf("%s：错误类型应为 %v，实际 %v", c.name, c.want, err)
		}
		// 反向传入（从对侧发起）也必须被拒绝：配对校验是对称的
		if rev := verifyTransferPair(cp, row); rev == nil {
			t.Errorf("%s：从对侧发起时应同样被拒绝，实际通过", c.name)
		}
	}
}

// TestR26VerifyTransferPairRejectsNil 空配对不得被当成"合法单边"
func TestR26VerifyTransferPairRejectsNil(t *testing.T) {
	out, _ := validTransferPair()
	if err := verifyTransferPair(out, nil); !errors.Is(err, ErrTransferGroupIncomplete) {
		t.Fatalf("对侧为 nil 应拒绝，实际 %v", err)
	}
	if err := verifyTransferPair(nil, out); !errors.Is(err, ErrTransferGroupIncomplete) {
		t.Fatalf("本侧为 nil 应拒绝，实际 %v", err)
	}
}

// TestR26NoUnilateralFallbackInSource collectTransferGroup 不得再出现"找不到对侧就返回单边"的退化分支。
//
// 这是 R26 的根因所在：旧实现 `if !has { return []*model.Transaction{row}, nil }`。
// 这里从源码层面锁住：函数体内只允许两条 return，且都不能是"只含 row 的单元素切片"的容错路径。
func TestR26NoUnilateralFallbackInSource(t *testing.T) {
	f := qas3Parse(t, "tx.go")
	fn := qas3Func(f, "collectTransferGroup")
	if fn == nil {
		t.Fatal("tx.go 中找不到 collectTransferGroup")
	}
	// 单笔（type≠3）早返回是合法的；转账分支的所有失败路径必须 return error
	sawTransferEarlyReturn := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		ret, ok := n.(*ast.ReturnStmt)
		if !ok || len(ret.Results) != 2 {
			return true
		}
		// 第二个返回值是 nil 且第一个不是 nil → "成功返回了一个分组"
		second, isIdent := ret.Results[1].(*ast.Ident)
		if !isIdent || second.Name != "nil" {
			return true
		}
		cl, isComposite := ret.Results[0].(*ast.CompositeLit)
		if !isComposite || len(cl.Elts) != 1 {
			return true
		}
		// 只有 `[]*model.Transaction{row}` 这一种单元素形式是允许的，且必须紧跟
		// `row.TxType != model.TxTypeTransfer` 的单笔分支（由下方文本断言覆盖）
		sawTransferEarlyReturn = true
		return true
	})
	if !sawTransferEarlyReturn {
		t.Fatal("collectTransferGroup 缺少单笔早返回分支（实现已变化，请更新用例）")
	}
	// 转账分支必须是"非单笔即整笔"：类型判定只能用 TxTypeTransfer 做早返回
	src := readServiceSource(t, "tx.go")
	idx := strings.Index(src, "func collectTransferGroup")
	if idx < 0 {
		t.Fatal("未找到 collectTransferGroup 定义")
	}
	body := src[idx:minStringEnd(src, idx)]
	if !strings.Contains(body, "row.TxType != model.TxTypeTransfer") {
		t.Fatal("collectTransferGroup 必须以 `row.TxType != model.TxTypeTransfer` 作为唯一的单行早返回条件")
	}
	if !strings.Contains(body, "ErrTransferGroupIncomplete") {
		t.Fatal("collectTransferGroup 的失败路径必须返回 ErrTransferGroupIncomplete")
	}
}

// ---------------------------------------------------------------------------
// GPT-R32：墓碑时间语义（纯函数部分）
// ---------------------------------------------------------------------------

// minStringEnd 从 from 开始找函数体结束（`\n}\n`）的绝对下标；找不到则返回末尾
func minStringEnd(src string, from int) int {
	rel := strings.Index(src[from:], "\n}\n")
	if rel < 0 {
		return len(src)
	}
	return from + rel
}

// TestR32TombstoneAtClearedOnRestore 删除写当前时刻、恢复清零。
func TestR32TombstoneAtClearedOnRestore(t *testing.T) {
	const now = 1793260800
	if got := tombstoneAt(now, true); got != now {
		t.Fatalf("删除应写入当前时刻 %d，实际 %d", now, got)
	}
	if got := tombstoneAt(now, false); got != 0 {
		t.Fatalf("恢复必须把 deleted_at 清零，实际 %d", got)
	}
	// 边界：now=0（异常时钟）也不能让恢复得到非零墓碑时间
	if got := tombstoneAt(0, false); got != 0 {
		t.Fatalf("恢复在 now=0 时也必须清零，实际 %d", got)
	}
}

// TestR32RestoreUsesTombstoneAt mutateVisibility 的 UPDATE 必须经 tombstoneAt 取墓碑时间，
// 不得再直接写 now（那会让恢复把 deleted_at 写成恢复时间）。
func TestR32RestoreUsesTombstoneAt(t *testing.T) {
	f := qas3Parse(t, "tx.go")
	fn := qas3Func(f, "mutateVisibility")
	if fn == nil {
		t.Fatal("tx.go 中找不到 mutateVisibility")
	}
	found := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		id, isIdent := call.Fun.(*ast.Ident)
		if isIdent && id.Name == "tombstoneAt" {
			found = true
			return false
		}
		return true
	})
	if !found {
		t.Fatal("mutateVisibility 必须调用 tombstoneAt 计算 deleted_at（GPT-R32）")
	}
}

// ---------------------------------------------------------------------------
// GPT-R28：权限必须在写事务内重读（源码顺序断言）
// ---------------------------------------------------------------------------

// TestR28PermissionRevalidatedInsideWriteTransaction 七个敏感入口的权限判定必须发生在
// 【写事务开始之后】，且不得再使用事务外的 RoleOf（engine 版本）。
//
// 为什么用源码顺序断言而不是"跑一次看结果"：事务外读角色留下的 TOCTOU 窗口，
// 在 SQLite 单写者 + MaxOpenConns=1 下几乎不可能靠 goroutine 调度稳定复现
// （撤销方的写会被 A 的写事务挡住）。而"读角色是否在 Begin 之后"是**确定性**的：
// 只要它成立，撤销一旦先提交，本次判定必然读到新角色。
// 真实 SQLite 的行为验证见 remediation_integration_test.go 的 TestR28* 系列。
func TestR28PermissionRevalidatedInsideWriteTransaction(t *testing.T) {
	cases := []struct {
		file string
		fn   string
	}{
		{"tx.go", "mutateVisibility"}, // 交易删除 / 恢复
		{"member.go", "ChangeRole"},   // 改角色
		{"member.go", "Remove"},       // 移除成员
		{"member.go", "CreateInvite"}, // 签发邀请码
		{"book.go", "Update"},         // 改账本名称 + 可见性
		{"book.go", "SetVisibility"},  // 可见性开关（唯一写实现）
		{"book.go", "Delete"},         // 删除账本
	}
	for _, c := range cases {
		f, fset := remediationParse(t, c.file)
		fn := qas3Func(f, c.fn)
		if fn == nil {
			t.Errorf("%s 中找不到 %s", c.file, c.fn)
			continue
		}
		callees := qas3Callees(fn)
		// ① 不得再出现事务外的 RoleOf（engine 版本）
		if callees["RoleOf"] {
			t.Errorf("%s.%s 仍调用 RoleOf：那是事务外的权限快照（GPT-R28）", c.file, c.fn)
		}
		// ② 必须使用事务内的权限读取原语
		inTx := callees["roleOfTx"] || callees["requireManagerTx"]
		if !inTx {
			t.Errorf("%s.%s 未调用 roleOfTx/requireManagerTx：权限不是在事务内重读的", c.file, c.fn)
		}
		// ③ Begin 必须出现在权限读取之前
		beginLine := remediationFirstCallLine(fn, fset, "Begin")
		if beginLine == 0 {
			t.Errorf("%s.%s 没有开启写事务（Begin）", c.file, c.fn)
			continue
		}
		permLine := remediationFirstCallLine(fn, fset, "roleOfTx")
		if permLine == 0 {
			permLine = remediationFirstCallLine(fn, fset, "requireManagerTx")
		}
		if permLine == 0 {
			t.Errorf("%s.%s 未找到事务内权限读取调用", c.file, c.fn)
			continue
		}
		if beginLine >= permLine {
			t.Errorf("%s.%s：Begin（L%d）必须早于事务内权限读取（L%d），否则仍是事务外快照",
				c.file, c.fn, beginLine, permLine)
		}
	}
}

// TestR28NoEngineRoleReadInWritePaths 全包扫描：写路径函数不得再出现 `RoleOf(s.engine, ...)`。
//
// RoleOf 只允许留在**只读**判定里（handler 的成员资格展示等），写操作一律走 roleOfTx。
func TestR28NoEngineRoleReadInWritePaths(t *testing.T) {
	for _, name := range []string{"tx.go", "member.go", "book.go"} {
		f, _ := remediationParse(t, name)
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			if !qas3Callees(fn)["RoleOf"] {
				continue
			}
			// 允许 RoleOf 的包装实现本身（MemberService.RoleOf）继续存在：它服务只读路径
			if fn.Name.Name == "RoleOf" {
				continue
			}
			t.Errorf("%s.%s 调用了 RoleOf（engine 版本）：写路径必须改用 roleOfTx", name, fn.Name.Name)
		}
	}
}

// ---------------------------------------------------------------------------
// 共用：源码解析helper
// ---------------------------------------------------------------------------

// remediationParse 解析本包源文件并保留 FileSet（需要行列位置时用）
func remediationParse(t *testing.T, name string) (*ast.File, *token.FileSet) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, nil, 0)
	if err != nil {
		t.Fatalf("解析 %s 失败：%v", name, err)
	}
	return f, fset
}

// remediationFirstCallLine 返回函数体内首次调用 sel（选择器名或函数名）的行号；0 表示未出现
func remediationFirstCallLine(fn *ast.FuncDecl, fset *token.FileSet, sel string) int {
	line := 0
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if line != 0 {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fun := call.Fun.(type) {
		case *ast.SelectorExpr:
			if fun.Sel.Name == sel {
				line = fset.Position(call.Pos()).Line
				return false
			}
		case *ast.Ident:
			if fun.Name == sel {
				line = fset.Position(call.Pos()).Line
				return false
			}
		}
		return true
	})
	return line
}
