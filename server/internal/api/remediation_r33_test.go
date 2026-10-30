package api

// GPT-R33（HTTP 适配层一侧）：WS 广播只能在账本更新事务【成功提交之后】执行。
//
// 服务侧把"名称 + 可见性"收敛进同一个事务（见 service.TestR33BookUpdateSingleTransaction），
// 但事务边界在 service 里，handler 能做的正确性保证只有一件事：
// **先拿到 Update 的成功返回，再广播**。否则客户端会收到 visibility_changed
// 并按 forceFullResync 去拉，却读到一个还没提交（甚至已回滚）的开关值。
//
// 本机无 CGO，无法用真实 SQLite 验证时序，因此用源码顺序断言把它固化下来。

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// TestR33VisibilityBroadcastOnlyAfterCommit 广播必须晚于服务调用，且只在"真变了"的分支里。
func TestR33VisibilityBroadcastOnlyAfterCommit(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "book_handler.go", nil, 0)
	if err != nil {
		t.Fatalf("解析 book_handler.go 失败：%v", err)
	}
	fn := (*ast.FuncDecl)(nil)
	for _, d := range f.Decls {
		if d2, ok := d.(*ast.FuncDecl); ok && d2.Name.Name == "update" && d2.Body != nil {
			fn = d2
		}
	}
	if fn == nil {
		t.Fatal("book_handler.go 中找不到 update")
	}

	updateLine := firstCallLineAt(fn, fset, "Update")
	broadcastLine := firstCallLineAt(fn, fset, "notifyVisibilityChanged")
	if updateLine == 0 {
		t.Fatal("update 未调用 books.Update（实现已变化，请更新用例）")
	}
	if broadcastLine == 0 {
		t.Fatal("update 未广播 visibility_changed（实现已变化，请更新用例）")
	}
	if broadcastLine <= updateLine {
		t.Fatalf("广播（L%d）必须晚于服务调用（L%d）：只有事务提交成功后才允许通知在线端",
			broadcastLine, updateLine)
	}
	// 广播必须在"真的发生变化"的条件分支内：值没变不得惊动客户端全量重拉
	if !stage3CallInsideIfReferencing(fn, "VisibilityChanged", "notifyVisibilityChanged") {
		t.Fatal("notifyVisibilityChanged 必须位于 `if res.VisibilityChanged` 分支内：" +
			"未发生实际变化时广播会让所有在线端白做一次 afterVersion=0 全量重拉")
	}
}

// TestR33UpdateUsesSingleServiceCall handler 不得绕过 BookService.Update 直接写开关
// （那样就绕开了"名称 + 可见性同事务"以及广播判定）。
func TestR33UpdateUsesSingleServiceCall(t *testing.T) {
	fn := stage3FuncDecl(t, "book_handler.go", "update")
	if n := stage3CallCount(fn, "SetVisibility"); n != 0 {
		t.Fatalf("bookHandler.update 直接调用了 SetVisibility %d 次："+
			"可见性写入必须经 BookService.Update 委托，否则名称与开关不在同一事务", n)
	}
	if n := stage3CallCount(fn, "Update"); n != 1 {
		t.Fatalf("bookHandler.update 应只调用一次 books.Update，实际 %d 次", n)
	}
}

// firstCallLineAt 返回函数体内首次调用 callee（选择器名或函数名）的行号；0 表示未出现
func firstCallLineAt(fn *ast.FuncDecl, fset *token.FileSet, callee string) int {
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
			if fun.Sel.Name == callee {
				line = fset.Position(call.Pos()).Line
				return false
			}
		case *ast.Ident:
			if fun.Name == callee {
				line = fset.Position(call.Pos()).Line
				return false
			}
		}
		return true
	})
	return line
}
