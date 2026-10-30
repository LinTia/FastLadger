package ws

// QA 定向回归（N2）：锁顺序的**机械**守卫。
//
// 约定（hub.go 包注释）：锁顺序恒为 Hub.mu → Client.mu，任何路径都不得在持有
// Client.mu 时去取 Hub.mu，否则 Subscribe/Broadcast 与 RefreshVisibility 之间会死锁。
//
// N2 改动让 RefreshVisibility 在持有 c.mu 时 `delete(c.BookIds, ...)`。这是最容易
// 顺手写成"顺便去 Hub 里把它删掉"的地方（那样就在 c.mu 内取 h.mu → 反向锁序）。
// 因此这里用 AST 扫一遍：**任何 `c.mu.Lock()` 之后的代码都不得再出现 `h.*` 访问**。
//
// 运行时只能"跑一会儿看会不会卡住"，而这种死锁往往要特定交错才触发；
// 源码级顺序断言能在静态阶段把这类错误挡下来（配合 TestQAS3ConcurrentFullLifecycle 的并发压测）。

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// qaLockRecvIs 判断表达式是否为 "c" 或 "x.c"（覆盖 c 与 t.c 两种写法）
func qaLockRecvIs(expr ast.Expr, name string) bool {
	switch v := expr.(type) {
	case *ast.Ident:
		return v.Name == name
	case *ast.SelectorExpr:
		return v.Sel.Name == name
	}
	return false
}

// TestQAS3NoHubLockWhileHoldingClientLock 持 Client.mu 时不得触碰 Hub。
func TestQAS3NoHubLockWhileHoldingClientLock(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "hub.go", nil, 0)
	if err != nil {
		t.Fatalf("解析 hub.go 失败：%v", err)
	}

	depth := 0
	violations := make([]string, 0, 4)
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		depth = 0
		// ast.Inspect 是前序遍历 = 源码顺序，足以发现"先锁 c.mu 再访问 h.*"
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, isCall := n.(*ast.CallExpr)
			if !isCall {
				return true
			}
			sel, isSel := call.Fun.(*ast.SelectorExpr)
			if !isSel {
				return true
			}
			// c.mu.Lock() / c.mu.Unlock() / c.mu.RLock() / c.mu.RUnlock()
			if muSel, isMu := sel.X.(*ast.SelectorExpr); isMu && muSel.Sel.Name == "mu" {
				if !qaLockRecvIs(muSel.X, "c") {
					return true // h.mu / 其它 mutex，不参与本判定
				}
				switch sel.Sel.Name {
				case "Lock", "RLock":
					depth++
				case "Unlock", "RUnlock":
					if depth > 0 {
						depth--
					}
				}
				return true
			}
			// 持着 c.mu 时的 h.xxx 访问 → 反向锁序嫌疑
			if id, isIdent := sel.X.(*ast.Ident); isIdent && id.Name == "h" && depth > 0 {
				violations = append(violations, fn.Name.Name+"."+sel.Sel.Name)
			}
			return true
		})
	}

	if len(violations) > 0 {
		t.Fatalf("在持有 Client.mu 的情况下访问 Hub（反向锁序，会与 Subscribe/Broadcast 死锁）：%v", violations)
	}
}

// TestQAS3ClientMutexNeverNests 同一函数内不得连续两层加 c.mu（自死锁）。
//
// 典型错误写法：RefreshVisibility 已在 c.mu 内，又调用 c.CanView()（内部再 Lock）。
func TestQAS3ClientMutexNeverNests(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "hub.go", nil, 0)
	if err != nil {
		t.Fatalf("解析 hub.go 失败：%v", err)
	}
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		depth := 0
		maxDepth := 0
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, isCall := n.(*ast.CallExpr)
			if !isCall {
				return true
			}
			sel, isSel := call.Fun.(*ast.SelectorExpr)
			if !isSel {
				return true
			}
			if muSel, isMu := sel.X.(*ast.SelectorExpr); isMu && muSel.Sel.Name == "mu" &&
				qaLockRecvIs(muSel.X, "c") {
				switch sel.Sel.Name {
				case "Lock", "RLock":
					depth++
					if depth > maxDepth {
						maxDepth = depth
					}
				case "Unlock", "RUnlock":
					if depth > 0 {
						depth--
					}
				}
			}
			return true
		})
		if maxDepth > 1 {
			t.Fatalf("%s 内对同一把 Client.mu 嵌套加锁（自死锁）", fn.Name.Name)
		}
	}
}
