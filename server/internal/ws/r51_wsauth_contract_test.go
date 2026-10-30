package ws

// GPT-R51 的**另一半**契约守卫：api.wsAuth 必须把 JWT 的 exp 写进上下文。
//
// 为什么做成源码级断言而不是运行期调用：
//   - ws **不能**导入 api（api 已导入 ws，会形成循环依赖），ServeWS 拿到的
//     只是 gin 上下文里的两个键，因此"谁写进去的"只能靠契约约束；
//   - api 的 wsAuth 又依赖 service.AuthService.Verify（要连数据库），
//     本机没有 gcc（CGO 不可用），跑不了真实 SQLite，无法在单测里端到端调用它。
//
// 于是这里用 AST 机械地把契约固定下来：wsAuth 内必须存在
// `c.Set(ws.TokenExpContextKey, claims.Exp)`。若有人改了键名、漏传 exp、
// 或把 exp 换成了别的值，本用例立刻失败——
// 那条链一旦断掉，ServeWS 就会 401 拒绝所有连接（fail-closed 的另一种表现），
// 或者更糟：有人为了"让它跑起来"把 exp 改成常量，连接就重新变成永生。

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// TestR51WSAuthMustPropagateTokenExpiry wsAuth 必须把 claims.Exp 写入 ws.TokenExpContextKey
func TestR51WSAuthMustPropagateTokenExpiry(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "../api/middleware.go", nil, 0)
	if err != nil {
		t.Fatalf("解析 ../api/middleware.go 失败：%v", err)
	}

	var wsAuth *ast.FuncDecl
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name != nil && fn.Name.Name == "wsAuth" {
			wsAuth = fn
			break
		}
	}
	if wsAuth == nil || wsAuth.Body == nil {
		t.Fatal("未在 internal/api/middleware.go 找到 wsAuth（WS 鉴权中间件被改名/删除？）")
	}

	// 找 c.Set(<sel>, <sel2>) 调用
	found := false
	var gotKey, gotVal string
	ast.Inspect(wsAuth.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel == nil || sel.Sel.Name != "Set" {
			return true
		}
		if id, isID := sel.X.(*ast.Ident); !isID || id.Name != "c" {
			return true
		}
		if len(call.Args) != 2 {
			return true
		}
		key, isKeySel := call.Args[0].(*ast.SelectorExpr)
		val, isValSel := call.Args[1].(*ast.SelectorExpr)
		if !isKeySel || !isValSel {
			return true
		}
		gotKey = exprString(key)
		gotVal = exprString(val)
		// 键必须是 ws 包导出的契约常量本身（ws.TokenExpContextKey），
		// 而不是有人手抄的字面量——后者改了 ws 这边也不会同步。
		if pkgID, isID := key.X.(*ast.Ident); isID && pkgID.Name == "ws" &&
			key.Sel != nil && key.Sel.Name == "TokenExpContextKey" &&
			gotVal == "claims.Exp" {
			found = true
		}
		return true
	})

	if !found {
		t.Fatalf("wsAuth 必须执行 c.Set(ws.%s, claims.Exp)：实际找到的是 c.Set(%s, %s)。"+
			"缺少它 → ServeWS 无法给连接设生命周期上限（GPT-R51）",
			TokenExpContextKey, gotKey, gotVal)
	}
}

// TestR51WSAuthUsesClaimsFromVerify wsAuth 必须从 VerifyWithClaims 取 claims（而非自己再解一遍 /
// 用当前时间现算一个 exp）—— 否则"令牌过期"与"连接上限"会对不上。
func TestR51WSAuthUsesClaimsFromVerify(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "../api/middleware.go", nil, 0)
	if err != nil {
		t.Fatalf("解析 ../api/middleware.go 失败：%v", err)
	}
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Name == nil || fn.Name.Name != "wsAuth" || fn.Body == nil {
			continue
		}
		used := false
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if sel, isSel := call.Fun.(*ast.SelectorExpr); isSel && sel.Sel != nil &&
				sel.Sel.Name == "VerifyWithClaims" {
				used = true
			}
			return true
		})
		if !used {
			t.Fatal("wsAuth 必须调用 authSvc.VerifyWithClaims 取到 claims（连接生命周期上限必须来自令牌本身）")
		}
		return
	}
	t.Fatal("未找到 wsAuth")
}

// exprString 把 a.b 这样的选择器表达式还原成源码文本（仅用于断言报错信息）
func exprString(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.SelectorExpr:
		if x, ok := v.X.(*ast.Ident); ok {
			return x.Name + "." + v.Sel.Name
		}
	case *ast.Ident:
		return v.Name
	}
	return ""
}
