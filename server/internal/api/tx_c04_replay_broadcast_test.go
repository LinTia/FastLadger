package api

// GPT-C04 回归守卫（第二轮加固）：deleteTransaction / restoreTransaction 的幂等重放
// 不得广播 data_changed，且守卫必须能抓出【极性绕过】——第一版用
// `strings.Contains(cond, "replayed")` 判定"条件引用了标志"，不校验极性：
// `if replayed || !replayed { notifyDataChanged(...) }`（把 ! 手滑写丢后或上原条件）
// 可编译、标志仍被引用、守卫却放行。现在改为 AST 级极性校验：
//  1. 条件中必须存在对重放标志的**一元取反**（!replayed / !res.Replayed）；
//  2. 条件中不得存在任何未经取反的裸引用（极性翻转必被抓出）。
//
// 另附行为级兜底测试（TestC04DeleteReplayDoesNotBroadcast）：真实 SQLite + gin +
// 测试 Hub，同一交易连做两次 delete，断言第二次（重放）不产生任何新广播事件——
// 守卫再怎么被绕过，行为级断言仍然兜底。
//
// 与 TestReplayedIsNotBroadcast（createTransaction）共用极性判定器。

import (
	"context"
	"encoding/json"
	"go/ast"
	"go/token"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"fastledger/internal/database"
	"fastledger/internal/migrate"
	"fastledger/internal/model"
	"fastledger/internal/service"
	"fastledger/internal/version"
	"fastledger/internal/ws"

	"github.com/gin-gonic/gin"
)

// c04IsReplayedRef 判断表达式是否为"重放标志"引用：
// 局部变量 replayed（delete/restore）或结果字段 res.Replayed（create）。
func c04IsReplayedRef(n ast.Node) bool {
	switch e := n.(type) {
	case *ast.Ident:
		return e.Name == "replayed"
	case *ast.SelectorExpr:
		return e.Sel.Name == "replayed" || e.Sel.Name == "Replayed"
	}
	return false
}

// c04FlagPolarity 统计条件表达式中重放标志的引用极性：
// 返回（被一元取反 ! 覆盖的引用数，未经取反的裸引用数）。
func c04FlagPolarity(cond ast.Expr) (negated, bare int) {
	total := 0
	ast.Inspect(cond, func(n ast.Node) bool {
		if c04IsReplayedRef(n) {
			total++
		}
		return true
	})
	ast.Inspect(cond, func(n ast.Node) bool {
		if u, ok := n.(*ast.UnaryExpr); ok && u.Op == token.NOT {
			// 穿透括号：!(replayed) 同样视为取反引用
			x := ast.Node(u.X)
			for {
				if p, isParen := x.(*ast.ParenExpr); isParen {
					x = p.X
					continue
				}
				break
			}
			if c04IsReplayedRef(x) {
				negated++
				return false // 取反消费掉该引用，不再向下重复计数
			}
		}
		return true
	})
	return negated, total - negated
}

// c04CondGatesOnNotReplayed 条件是否把广播正确地守在"非重放"分支内：
// 必须存在 !replayed / !res.Replayed，且无裸引用。
func c04CondGatesOnNotReplayed(cond ast.Expr) bool {
	negated, bare := c04FlagPolarity(cond)
	return negated >= 1 && bare == 0
}

// assertBroadcastGatedByNotReplayed AST 级守卫：
// fnName 中 notifyDataChanged 恰好出现一次，且位于以一元取反重放标志为前提的分支内。
func assertBroadcastGatedByNotReplayed(t *testing.T, path, fnName string) {
	t.Helper()
	fn := stage3FuncDecl(t, path, fnName)
	if n := stage3CallCount(fn, "notifyDataChanged"); n != 1 {
		t.Fatalf("%s.%s 应恰好广播一次（且在 !replayed 分支内），实际 %d 次", path, fnName, n)
	}
	gated := false
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		ifs, ok := node.(*ast.IfStmt)
		if !ok || ifs.Body == nil {
			return true
		}
		hasCall := false
		ast.Inspect(ifs.Body, func(b ast.Node) bool {
			if call, isCall := b.(*ast.CallExpr); isCall {
				switch f := call.Fun.(type) {
				case *ast.Ident:
					if f.Name == "notifyDataChanged" {
						hasCall = true
					}
				case *ast.SelectorExpr:
					if f.Sel.Name == "notifyDataChanged" {
						hasCall = true
					}
				}
			}
			return true
		})
		if !hasCall {
			return true
		}
		gated = c04CondGatesOnNotReplayed(ifs.Cond)
		return true
	})
	if !gated {
		t.Fatalf("%s.%s 的广播必须由对重放标志的一元取反（!replayed / !res.Replayed）把守，"+
			"且条件中不得存在未取反的裸引用（极性校验，GPT-C04）", path, fnName)
	}
}

// TestC04MutateReplayIsNotBroadcast delete/restore：广播必须位于 !replayed 分支内
// 且恰好一次；必须走带重放标记的 Receipt 入口。
func TestC04MutateReplayIsNotBroadcast(t *testing.T) {
	for _, c := range []struct{ path, fn string }{
		{"tx_handler.go", "deleteTransaction"},
		{"tx_handler.go", "restoreTransaction"},
	} {
		assertBroadcastGatedByNotReplayed(t, c.path, c.fn)
		body := stage3FuncBody(t, c.path, c.fn)
		if !strings.Contains(body, "Receipt(") {
			t.Fatalf("%s.%s 必须调用 DeleteReceipt/RestoreReceipt（拿到 replayed 标记）：%s", c.path, c.fn, body)
		}
	}
}

// TestC04GuardPolarityDiscriminates 判别力自证（对抗验证抓到的缺口回归钉）：
// 同一判定器必须放行真实源码、抓住极性绕过样本。
func TestC04GuardPolarityDiscriminates(t *testing.T) {
	// 真实源码（含 create 的 selector 形态 !res.Replayed）必须放行
	for _, c := range []struct{ path, fn string }{
		{"tx_handler.go", "deleteTransaction"},
		{"tx_handler.go", "restoreTransaction"},
		{"tx_handler.go", "createTransaction"},
	} {
		fn := stage3FuncDecl(t, c.path, c.fn)
		found := false
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			ifs, ok := node.(*ast.IfStmt)
			if !ok || ifs.Cond == nil {
				return true
			}
			neg, bare := c04FlagPolarity(ifs.Cond)
			if neg+bare == 0 {
				return true // 与重放标志无关的条件
			}
			found = true
			if !c04CondGatesOnNotReplayed(ifs.Cond) {
				t.Fatalf("真实源码 %s.%s 未通过极性判定（守卫误报）", c.path, c.fn)
			}
			return true
		})
		if !found {
			t.Fatalf("%s.%s 中找不到带重放标志条件的 if", c.path, c.fn)
		}
	}

	// 变体 B（对抗验证实测的绕过样本）：replayed || !replayed —— 必须报警
	variantB := &ast.BinaryExpr{
		X:  &ast.Ident{Name: "replayed"},
		Op: token.OR,
		Y:  &ast.UnaryExpr{Op: token.NOT, X: &ast.Ident{Name: "replayed"}},
	}
	if negated, bare := c04FlagPolarity(variantB); negated != 1 || bare != 1 {
		t.Fatalf("变体 B 极性统计不符：negated=%d bare=%d（期望 1/1）", negated, bare)
	} else if c04CondGatesOnNotReplayed(variantB) {
		t.Fatal("判别器失效：极性绕过样本（replayed || !replayed）被放行")
	}
	// 无条件引用样本：裸 replayed —— 必须报警
	if negated, bare := c04FlagPolarity(&ast.Ident{Name: "replayed"}); negated != 0 || bare != 1 {
		t.Fatalf("无条件引用极性统计不符：negated=%d bare=%d（期望 0/1）", negated, bare)
	}
	// 正确形态：!replayed —— 必须放行
	if negated, bare := c04FlagPolarity(&ast.UnaryExpr{Op: token.NOT, X: &ast.Ident{Name: "replayed"}}); negated != 1 || bare != 0 {
		t.Fatalf("正确形态极性统计不符：negated=%d bare=%d（期望 1/0）", negated, bare)
	}
	// selector 形态：!res.Replayed —— 必须放行
	if negated, bare := c04FlagPolarity(&ast.UnaryExpr{Op: token.NOT,
		X: &ast.SelectorExpr{X: &ast.Ident{Name: "res"}, Sel: &ast.Ident{Name: "Replayed"}}}); negated != 1 || bare != 0 {
		t.Fatalf("selector 形态极性统计不符：negated=%d bare=%d（期望 1/0）", negated, bare)
	}
	// 括号形态：!(replayed) —— 必须放行
	if negated, bare := c04FlagPolarity(&ast.UnaryExpr{Op: token.NOT,
		X: &ast.ParenExpr{X: &ast.Ident{Name: "replayed"}}}); negated != 1 || bare != 0 {
		t.Fatalf("括号形态极性统计不符：negated=%d bare=%d（期望 1/0）", negated, bare)
	}
}

// TestC04DeleteReplayDoesNotBroadcast 行为级兜底（真实 SQLite + gin + 测试 Hub）：
// 同一交易连做两次 delete（同 clientUuid），第二次是幂等重放——
// 断言重放不产生任何新广播事件。
func TestC04DeleteReplayDoesNotBroadcast(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 真实 SQLite 环境
	engine, err := database.Init(filepath.Join(t.TempDir(), "c04_behavioral.db"))
	if err != nil {
		t.Fatalf("database.Init 失败（CGO 是否可用？）：%v", err)
	}
	t.Cleanup(func() { _ = engine.Close() })
	if _, err := migrate.Apply(engine); err != nil {
		t.Fatalf("migrate.Apply 失败：%v", err)
	}
	u := &model.User{Username: "c04owner", PasswordHash: "x"}
	if _, err := engine.Insert(u); err != nil {
		t.Fatal(err)
	}
	bs := service.NewBookService(engine, version.NewRepo())
	b, err := bs.Create(u.Id, "C04账本", model.BookTypePersonal)
	if err != nil {
		t.Fatal(err)
	}
	as := service.NewAccountService(engine, version.NewRepo())
	acct, err := as.Create(u.Id, b.Id, service.AccountCreateInput{Name: "现金", Category: model.CategoryCash, Currency: "CNY"})
	if err != nil {
		t.Fatal(err)
	}
	cs := service.NewCategoryService(engine, version.NewRepo())
	cat, err := cs.Create(u.Id, b.Id, service.CategoryCreateInput{Name: "餐饮", CatType: model.CategoryTypeExpense})
	if err != nil {
		t.Fatal(err)
	}
	txSvc := service.NewTxService(engine, version.NewRepo())
	res, err := txSvc.Create(context.Background(), u.Id, b.Id, service.CreateRequest{
		ClientUuid: "c04b-create", TxType: int(model.TxTypeExpense),
		AccountId: acct.Id, CategoryId: cat.Id, Amount: 2500,
		PaymentChannel: 0, TxTime: time.Now().Unix(), Source: 0,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 换上测试专用 Hub 并订阅本账本
	old := ws.DefaultHub
	hub := ws.NewHub(func(uid, bookID int64) (bool, error) { return true, nil })
	client := ws.NewClient()
	hub.Register(client)
	hub.Subscribe(client, []int64{b.Id}, u.Id)
	ws.DefaultHub = hub
	t.Cleanup(func() { ws.DefaultHub = old })

	h := &ledgerHandler{txs: txSvc}
	txID := res.TransactionIds[0]
	idStr := strconv.FormatInt(txID, 10)
	bookStr := strconv.FormatInt(b.Id, 10)
	params := gin.Params{{Key: "id", Value: bookStr}, {Key: "txId", Value: idStr}}
	target := "/api/v1/books/" + bookStr + "/transactions/" + idStr + "?clientUuid=c04b-del"
	do := func() *httptest.ResponseRecorder {
		c, w := stage3Ctx(t, http.MethodDelete, target, "", params, u.Id)
		h.deleteTransaction(c)
		return w
	}

	// 第一次：真实删除 → 200 + 恰好一条 data_changed
	w1 := do()
	if w1.Code != http.StatusOK {
		t.Fatalf("首次删除应 200，实际 %d（body=%s）", w1.Code, w1.Body.String())
	}
	m1 := stage3ReadEvent(t, client)
	if m1["event"] != eventDataChanged {
		t.Fatalf("首次删除应广播 data_changed，实际 %v", m1["event"])
	}

	// 第二次：同键幂等重放 → 200 + 同版本，但不得产生任何新广播
	w2 := do()
	if w2.Code != http.StatusOK {
		t.Fatalf("重放删除应 200，实际 %d（body=%s）", w2.Code, w2.Body.String())
	}
	var v1, v2 struct {
		Data struct {
			Version int64 `json:"version"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w1.Body.Bytes(), &v1)
	_ = json.Unmarshal(w2.Body.Bytes(), &v2)
	if w2.Code == http.StatusOK && v1.Data.Version != v2.Data.Version {
		t.Errorf("重放应返回原版本 %d，实际 %d", v1.Data.Version, v2.Data.Version)
	}
	select {
	case raw := <-client.Send:
		t.Fatalf("GPT-C04：幂等重放不得广播 data_changed，实际收到 %s", raw)
	case <-time.After(300 * time.Millisecond):
	}
}
