package service

// 可见性相关的纯逻辑测试（不依赖数据库）。
//
// 关键验收点：**可见性变化不递增 sync_version**（PRD §12.1 决策 4）。
// 本机无法跑 SQLite，因此用【源码结构断言】证明 SetVisibility 既不调用
// 版本唯一实现 versions.Next，也不触碰 sync_version 列 —— 这比"跑一次看结果"
// 更强：它同时对未来任何一次改动生效。
import (
	"bytes"
	"errors"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"strings"
	"testing"
)

// TestSetVisibilityDoesNotBumpVersion 结构断言：SetVisibility 不得递增版本
func TestSetVisibilityDoesNotBumpVersion(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "book.go", nil, 0)
	if err != nil {
		t.Fatalf("解析 book.go 失败：%v", err)
	}

	var fn *ast.FuncDecl
	for _, decl := range file.Decls {
		if d, ok := decl.(*ast.FuncDecl); ok && d.Name.Name == "SetVisibility" && d.Recv != nil {
			fn = d
			break
		}
	}
	if fn == nil || fn.Body == nil {
		t.Fatal("book.go 中找不到 (*BookService).SetVisibility")
	}

	// 1) 不得调用任何名为 Next 的方法（版本唯一实现是 versions.Next）
	var calls []string
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch f := call.Fun.(type) {
		case *ast.SelectorExpr:
			calls = append(calls, f.Sel.Name)
		case *ast.Ident:
			calls = append(calls, f.Name)
		}
		return true
	})
	for _, c := range calls {
		if c == "Next" {
			t.Fatalf("SetVisibility 调用了 Next（版本递增）：可见性变化不得递增 sync_version。调用列表：%v", calls)
		}
	}

	// 2) 函数体不得出现 sync_version / versions 字段引用
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, fn.Body); err != nil {
		t.Fatalf("打印函数体失败：%v", err)
	}
	body := buf.String()
	for _, forbidden := range []string{"sync_version", "SyncVersion", "versions"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("SetVisibility 函数体出现 %q：可见性变化不是数据变更，不得触碰版本语义", forbidden)
		}
	}

	// 3) 必须更新 members_can_view_others（否则开关点了没效果）
	if !strings.Contains(body, "members_can_view_others") {
		t.Fatalf("SetVisibility 未更新 members_can_view_others 列：%s", body)
	}
}

// TestCanViewDetailFailsClosed 无引擎时 fail-closed（绝不因依赖缺失而放行）
func TestCanViewDetailFailsClosed(t *testing.T) {
	canView, err := CanViewDetail(nil, 1, 1)
	if canView {
		t.Fatal("无引擎时不得判定为可见")
	}
	if !errors.Is(err, ErrNotMember) {
		t.Fatalf("无引擎应返回 ErrNotMember（不泄露账本是否存在），实际 %v", err)
	}
}

// TestSyncServiceGenerationNilMeta meta 缺失时世代回零，不 panic
func TestSyncServiceGenerationNilMeta(t *testing.T) {
	s := NewSyncService(nil, nil, nil)
	epoch, id := s.generation()
	if epoch != 0 || id != "" {
		t.Fatalf("nil meta 应回落零值，实际 (%d, %q)", epoch, id)
	}
	// Pull 在服务未初始化时必须明确报错，而不是返回空批次让客户端"以为已同步"
	if _, err := s.Pull(1, 1, 0, 0); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("未初始化引擎的 Pull 应返回 ErrInvalidArgument，实际 %v", err)
	}
	if _, err := s.Pull(0, 1, 0, 0); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("uid 缺失的 Pull 应返回 ErrInvalidArgument，实际 %v", err)
	}
}
