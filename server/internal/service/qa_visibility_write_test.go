package service

// QA 定向回归（P1-1 闭环）：**全仓**唯一可见性写实现的机械证明。
//
// 上一轮 P1 的根因是"同一个业务变更有两套写实现，只修了其中一条的副作用"。
// 工程师这轮把可见性写入收敛到 BookService.SetVisibility，并用 AST 断言锁住
// `Update` 不自己拼 UPDATE。但那条断言只覆盖 `Update` 一个函数——若将来有**第三个**
// 函数（新服务、新 handler service、运维脚本入口）直接写这一列，它照样绿。
//
// 因此这里补一条**全仓扫描**：任何"引用了 MembersCanViewOthers 又调用 Update/Exec/Cols"
// 的函数，只允许是 SetVisibility 一个。

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// qawFindVisibilityWriters 扫描 root 下所有**生产** .go 文件，返回"可能写这一列"的函数名 → 文件列表
func qawFindVisibilityWriters(t *testing.T, root string) map[string][]string {
	t.Helper()
	writers := make(map[string][]string)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// 跳过 testdata 与隐藏目录（.git 等）；**不能跳过根节点**（".." 本身以点开头）
			if path != root {
				base := info.Name()
				if base == "testdata" || strings.HasPrefix(base, ".") {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if perr != nil {
			return perr
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			refsField, writesDB := false, false
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				switch v := n.(type) {
				case *ast.SelectorExpr:
					if v.Sel.Name == "MembersCanViewOthers" {
						refsField = true
					}
					// 只认**数据库写**标记：Cols（xorm 列白名单）与 Exec（原生 SQL）。
					// 不能用 "Update"：handler 里的 `h.books.Update(...)` 也是 Update，
					// 会与请求结构里的同名字段一起误报。
					if v.Sel.Name == "Cols" || v.Sel.Name == "Exec" {
						writesDB = true
					}
				case *ast.Ident:
					if v.Name == "MembersCanViewOthers" {
						refsField = true
					}
				}
				return true
			})
			if refsField && writesDB {
				writers[fn.Name.Name] = append(writers[fn.Name.Name], path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("扫描 %s 失败：%v", root, err)
	}
	return writers
}

// TestQAS3SingleVisibilityWritePathRepoWide 全仓只有 SetVisibility 一个写实现。
//
// 失败含义：新增了第二个写这一列的函数 → 极可能又出现「改了开关却不广播」的旁路。
func TestQAS3SingleVisibilityWritePathRepoWide(t *testing.T) {
	writers := qawFindVisibilityWriters(t, "..")
	if _, ok := writers["SetVisibility"]; !ok {
		t.Fatalf("未找到 SetVisibility 作为唯一写实现（扫描失效或实现已改名）：%v", writers)
	}
	for name, files := range writers {
		if name == "SetVisibility" {
			continue
		}
		t.Fatalf("发现第二个可见性写实现 %q（%v）：只能经由 BookService.SetVisibility 写这一列，"+
			"否则该入口不会刷新 Hub 缓存、不会广播 visibility_changed", name, files)
	}
}

// TestQAS3SetVisibilityIdempotentGuardPrecedesWrite 「值没变 → 不写库」必须由早返回保证，
// 且必须出现在 UPDATE 之前——《落地方案》§5.2 第 3 条硬约束的「不写库」那一半，
// 现有测试只锁住了「不广播」。
func TestQAS3SetVisibilityIdempotentGuardPrecedesWrite(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "book.go", nil, 0)
	if err != nil {
		t.Fatalf("解析 book.go 失败：%v", err)
	}
	fn := qas3Func(f, "SetVisibility")
	if fn == nil {
		t.Fatal("book.go 中找不到 SetVisibility")
	}

	var guardPos, writePos token.Pos
	guardReturnsFalse := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		ifs, ok := n.(*ast.IfStmt)
		if ok && guardPos == token.NoPos {
			condRefs := false
			ast.Inspect(ifs.Cond, func(c ast.Node) bool {
				if id, isIdent := c.(*ast.Ident); isIdent && id.Name == "MembersCanViewOthers" {
					condRefs = true
				}
				return true
			})
			if condRefs {
				guardPos = ifs.Pos()
				ast.Inspect(ifs.Body, func(b ast.Node) bool {
					if ret, isRet := b.(*ast.ReturnStmt); isRet && len(ret.Results) == 2 {
						if id, isIdent := ret.Results[0].(*ast.Ident); isIdent && id.Name == "false" {
							guardReturnsFalse = true
						}
					}
					return true
				})
			}
		}
		call, ok := n.(*ast.CallExpr)
		if ok && writePos == token.NoPos {
			if sel, isSel := call.Fun.(*ast.SelectorExpr); isSel && sel.Sel.Name == "Update" {
				writePos = call.Pos()
			}
		}
		return true
	})

	if guardPos == token.NoPos || !guardReturnsFalse {
		t.Fatal("SetVisibility 缺少「当前值 == canView 就 return false, nil」的幂等早返回：" +
			"值没变时仍会写库并抖动 updated_at")
	}
	if writePos == token.NoPos {
		t.Fatal("SetVisibility 中找不到 UPDATE 调用（实现已变化，请更新用例）")
	}
	if fset.Position(guardPos).Line >= fset.Position(writePos).Line {
		t.Fatalf("幂等早返回（L%d）必须在 UPDATE（L%d）之前，否则值没变也会落库",
			fset.Position(guardPos).Line, fset.Position(writePos).Line)
	}
}

// TestQAS3SetVisibilityHasOnlyTwoCallers 唯一写实现的**调用链**也必须收敛。
//
// 全仓扫描只能保证"没有第二套写实现"；还要保证没有别的入口拿它去写、却不广播。
// 《落地方案》§5.2 定稿为"两个入口、单一写实现"，合法调用链只有两条：
//
//	bookHandler.update        → BookService.Update  → SetVisibility   （handler 按 res.VisibilityChanged 广播）
//	bookHandler.setVisibility →                        SetVisibility   （handler 按 changed 广播）
//
// 因此直接调用方只允许 `Update`（service）与 `setVisibility`（handler）两个。
// 新增第三个（例如未来的管理员端点/运维子命令）时本用例立刻红——提醒补广播。
func TestQAS3SetVisibilityHasOnlyTwoCallers(t *testing.T) {
	callers := make(map[string][]string)
	err := filepath.Walk("..", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if path != ".." {
				base := info.Name()
				if base == "testdata" || strings.HasPrefix(base, ".") {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if perr != nil {
			return perr
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, isCall := n.(*ast.CallExpr)
				if !isCall {
					return true
				}
				if sel, isSel := call.Fun.(*ast.SelectorExpr); isSel && sel.Sel.Name == "SetVisibility" {
					callers[fn.Name.Name] = append(callers[fn.Name.Name], path)
				}
				return true
			})
		}
		return nil
	})
	if err != nil {
		t.Fatalf("扫描失败：%v", err)
	}
	want := map[string]bool{"Update": true, "setVisibility": true}
	for name, files := range callers {
		if !want[name] {
			t.Fatalf("SetVisibility 出现第三个调用方 %q（%v）：该入口改了开关但不会广播 visibility_changed，"+
				"客户端不会定向重拉", name, files)
		}
	}
	for name := range want {
		if _, ok := callers[name]; !ok {
			t.Fatalf("未找到预期调用方 %q（实现已变化，请更新用例）", name)
		}
	}
}
