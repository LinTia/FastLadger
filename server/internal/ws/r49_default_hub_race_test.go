package ws

// CI `-race` 抓到的真实数据竞争（run 41073315674 / 41074321097）的定向回归。
//
// 报错形态（两次运行分别记在不同用例名下，因为"谁在读"随调度漂移）：
//
//	WARNING: DATA RACE
//	Read  ... internal/ws.ServeWS.func?()  serve.go:<DefaultHub.Unregister / Subscribe>
//	Write ... r49_subscribe_limits_test.go:268  t.Cleanup(func(){ DefaultHub = old })
//
// 根因（**不是**"测试改写了订阅上限常量"——那几个都是 const，Go 里无法运行时赋值）：
//   ServeWS 在**连接生命周期的任意时刻**反复读包级全局 DefaultHub
//   （每来一帧 subscribe 读一次 + 收尾 defer 里 Unregister 读一次）；
//   测试为了注入假 checker 会临时替换 DefaultHub，并在 t.Cleanup 里写回原值；
//   WebSocket 是 hijacked 连接，**httptest.Server.Close() 不等待被劫持的连接**，
//   所以 ServeWS 的 goroutine 完全可能活过 t.Cleanup ⇒ 一读一写，无任何同步。
//
// 修复（两层）：
//   1. 结构性：ServeWS 在 Upgrade 之后**一次性**捕获 Hub 引用
//      （`hub := DefaultHub`），此后不再触碰全局变量 —— 竞争窗口收敛为零；
//   2. 纵深防御：用到真实连接的用例在返回前把连接排干（关闭客户端连接并等
//      Hub 在线数归 0），确保没有 goroutine 活到下一个用例。
//
// 本文件用源码级断言把第 1 条机械地锁住：ServeWS 内对 DefaultHub 的引用
// 必须**恰好一次**，且不允许出现在任何闭包/goroutine 里。这样即使将来有人
// 图省事又写回 `DefaultHub.Subscribe(...)`，也会被这条用例拦下。

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// countDefaultHubIdents 统计一棵 AST 子树里标识符 DefaultHub 的出现次数
func countDefaultHubIdents(root ast.Node) int {
	n := 0
	ast.Inspect(root, func(node ast.Node) bool {
		if id, ok := node.(*ast.Ident); ok && id.Name == "DefaultHub" {
			n++
		}
		return true
	})
	return n
}

// TestServeWSReadsDefaultHubExactlyOnce ServeWS 只能在"取 Hub 引用"时读一次全局
func TestServeWSReadsDefaultHubExactlyOnce(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "serve.go", nil, 0)
	if err != nil {
		t.Fatalf("解析 serve.go 失败：%v", err)
	}
	var fn *ast.FuncDecl
	for _, d := range f.Decls {
		if v, ok := d.(*ast.FuncDecl); ok && v.Name != nil && v.Name.Name == "ServeWS" && v.Body != nil {
			fn = v
			break
		}
	}
	if fn == nil {
		t.Fatal("未找到 ServeWS")
	}
	if got := countDefaultHubIdents(fn.Body); got != 1 {
		t.Fatalf("ServeWS 内对 DefaultHub 的引用必须恰好 1 次（一次性捕获 Hub 引用），实际 %d 次。"+
			"多处读取会与测试中 t.Cleanup 写回 DefaultHub 形成数据竞争", got)
	}
}

// TestServeWSClosuresNeverReadDefaultHub 闭包（defer / 读循环 goroutine）不得读全局 Hub
func TestServeWSClosuresNeverReadDefaultHub(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "serve.go", nil, 0)
	if err != nil {
		t.Fatalf("解析 serve.go 失败：%v", err)
	}
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Name == nil || fn.Name.Name != "ServeWS" || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			lit, ok := node.(*ast.FuncLit)
			if !ok {
				return true
			}
			if n := countDefaultHubIdents(lit); n != 0 {
				t.Fatalf("ServeWS 的闭包（defer / 读循环 goroutine）里出现了 %d 次 DefaultHub；"+
					"这些代码运行在连接收尾或收到帧的时刻，会与测试 t.Cleanup 的写回竞争。"+
					"请改用连接建立时捕获的 hub 局部变量", n)
			}
			return true
		})
		return
	}
	t.Fatal("未找到 ServeWS")
}

// rCloseWSAndDrain 关闭客户端连接并等服务端把该连接彻底注销（在线数归 0）。
//
// 为什么测试收尾必须显式排干：httptest.Server.Close() **不等待 hijacked 的
// WebSocket 连接**。若用例结束时 ServeWS 的 goroutine 还活着，后续任何
// 替换 DefaultHub 的动作都可能与它竞争。等在线数归 0 即代表该 goroutine 已经
// 走过 Unregister（之后它不再访问 Hub 之外的全局量）。
func rCloseWSAndDrain(t *testing.T, conn *websocket.Conn, hub *Hub) {
	t.Helper()
	if conn != nil {
		_ = conn.Close()
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if hub.Count() == 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("连接未排干：Hub 在线数仍为 %d（ServeWS 的 goroutine 可能仍在读全局变量）", hub.Count())
}
