package api

// 阶段 3 返工（P1-1 / P2-3 / P3-5 / P3-7 / N1 / N2）的契约测试——仍然**不碰数据库**：
//
//  1. 事件信封（裁定 2 修订版）：五字段固定；信号事件 data_changed（forceFullResync=false，
//     data={}）、四个协调事件（forceFullResync=true）；业务字段只能出现在 data 里；
//  2. 命名规则：事件名枚举一律下划线、JSON 字段一律 camelCase——用一条用例固化；
//  3. 幂等重放不广播（P3-5）：源码结构断言（真实重放需要数据库）；
//  4. P1-1 修订版：`BookService.Update` 只写 name，可见性写入收敛到 SetVisibility，
//     老入口 `PUT /books/{id}` 收到 membersCanViewOthers 必须**显式 400**（绝不静默忽略）；
//  5. 成员上限 → 409（P3-7）；被移除成员被摘出订阅表（N2）。
import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"net/http"
	"strings"
	"testing"
	"time"

	"fastledger/internal/service"
	"fastledger/internal/ws"
)

// stage3CaptureHub 换成测试专用 Hub，并登记一个已订阅账本 1 的连接，用于抓广播载荷。
func stage3CaptureHub(t *testing.T) *ws.Client {
	t.Helper()
	old := ws.DefaultHub
	hub := ws.NewHub(func(uid, bookID int64) (bool, error) { return true, nil })
	client := ws.NewClient()
	hub.Register(client)
	hub.Subscribe(client, []int64{1}, 7)
	ws.DefaultHub = hub
	t.Cleanup(func() { ws.DefaultHub = old })
	return client
}

// stage3ReadEvent 从连接的 Send 里取出下一条事件并解析成 map
func stage3ReadEvent(t *testing.T, c *ws.Client) map[string]interface{} {
	t.Helper()
	select {
	case raw := <-c.Send:
		var m map[string]interface{}
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("事件不是合法 JSON：%s", raw)
		}
		return m
	case <-time.After(time.Second):
		t.Fatal("等待广播超时")
		return nil
	}
}

// stage3AssertEnvelope 校验信封五字段与取值（所有事件共用）
func stage3AssertEnvelope(t *testing.T, m map[string]interface{}, wantEvent string, wantVersion float64, wantForce bool) {
	t.Helper()
	if m["event"] != wantEvent {
		t.Fatalf("event 期望 %q，实际 %v", wantEvent, m["event"])
	}
	if m["bookId"] != float64(1) {
		t.Fatalf("bookId 期望 1，实际 %v", m["bookId"])
	}
	if m["version"] != wantVersion {
		t.Fatalf("version 期望 %v，实际 %v", wantVersion, m["version"])
	}
	// forceFullResync 必须**始终存在**（信号事件为 false），客户端可无条件读取
	force, has := m["forceFullResync"]
	if !has {
		t.Fatalf("信封缺少 forceFullResync（必须始终出现）：%v", m)
	}
	if force != wantForce {
		t.Fatalf("forceFullResync 期望 %v，实际 %v", wantForce, force)
	}
	data, ok := m["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("data 必须是对象（业务字段唯一承载位），实际 %v", m["data"])
	}
	_ = data // 具体内容由调用方按事件类型断言
}

// TestEventNamesAreSnakeCaseAndFieldsCamelCase 命名规则固化：
// 事件名（枚举值）一律下划线，JSON 字段名一律 camelCase。
func TestEventNamesAreSnakeCaseAndFieldsCamelCase(t *testing.T) {
	events := []string{
		eventDataChanged, eventVisibilityChanged, eventMemberRemoved,
		eventMemberRoleChanged, eventBookDeleted,
	}
	if len(events) != 5 {
		t.Fatalf("事件枚举应为 5 个，实际 %d", len(events))
	}
	for _, e := range events {
		if strings.Contains(e, ".") {
			t.Errorf("事件名 %q 含点号：事件名一律下划线（字段名才用 camelCase）", e)
		}
		if strings.ToLower(e) != e {
			t.Errorf("事件名 %q 不是全小写", e)
		}
	}
	if eventDataChanged != "data_changed" {
		t.Errorf("信号事件名应为 data_changed，实际 %q", eventDataChanged)
	}
	// 信封字段必须是 camelCase（用 JSON 序列化结果验证，而不是靠肉眼看结构体）
	raw, err := json.Marshal(bookEventEnvelope{Event: eventDataChanged, BookId: 1, Version: 2})
	if err != nil {
		t.Fatalf("序列化失败：%v", err)
	}
	for _, field := range []string{`"event"`, `"bookId"`, `"version"`, `"forceFullResync"`, `"data"`} {
		if !strings.Contains(string(raw), field) {
			t.Fatalf("信封缺少字段 %s：%s", field, raw)
		}
	}
	// 事件名只有下划线，不允许出现点号（历史上 point 混用是 QA 抓到的 P2-3）
	if strings.Contains(string(raw), `"event":"data.changed"`) {
		t.Fatalf("事件名不得使用点号：%s", raw)
	}
}

// TestEventEnvelopeDataChanged 信号事件：data_changed + forceFullResync=false + data={}
func TestEventEnvelopeDataChanged(t *testing.T) {
	client := stage3CaptureHub(t)
	notifyDataChanged(1, 88)
	m := stage3ReadEvent(t, client)
	stage3AssertEnvelope(t, m, eventDataChanged, 88, false)

	// 信号事件的 data **刻意为空**：不含金额/账户/备注，明细走授权 pull。
	// 一旦有人往这里塞明细，关闭互看的 member 也能收到，等于绕过可见性过滤。
	data, _ := m["data"].(map[string]interface{})
	if len(data) != 0 {
		t.Fatalf("data_changed 的 data 必须为空（明细只能走授权 pull），实际 %v", data)
	}
}

// TestEventEnvelopeCoordinationEvents 四个协调事件的信封契约
func TestEventEnvelopeCoordinationEvents(t *testing.T) {
	cases := []struct {
		name     string
		fire     func()
		event    string
		wantData map[string]interface{}
	}{
		{
			name:  "visibility_changed",
			fire:  func() { notifyVisibilityChanged(1, 37) },
			event: eventVisibilityChanged,
		},
		{
			name:     "member_removed",
			fire:     func() { notifyMemberRemoved(1, 37, 9) },
			event:    eventMemberRemoved,
			wantData: map[string]interface{}{"targetUid": float64(9)},
		},
		{
			name:     "member_role_changed",
			fire:     func() { notifyMemberRoleChanged(1, 37, 9, 2) },
			event:    eventMemberRoleChanged,
			wantData: map[string]interface{}{"targetUid": float64(9), "role": float64(2)},
		},
		{
			name:  "book_deleted",
			fire:  func() { notifyBookDeleted(1, 37) },
			event: eventBookDeleted,
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			client := stage3CaptureHub(t)
			c.fire()
			m := stage3ReadEvent(t, client)
			stage3AssertEnvelope(t, m, c.event, 37, true)

			// 业务字段只能在 data 里（不得平铺在顶层）
			for _, leak := range []string{"targetUid", "role"} {
				if _, has := m[leak]; has {
					t.Fatalf("业务字段 %q 不得平铺在事件顶层：%v", leak, m)
				}
			}
			data, _ := m["data"].(map[string]interface{})
			if len(data) != len(c.wantData) {
				t.Fatalf("data 字段数期望 %d，实际 %v", len(c.wantData), data)
			}
			for k, want := range c.wantData {
				if data[k] != want {
					t.Fatalf("data.%s 期望 %v，实际 %v", k, want, data[k])
				}
			}
		})
	}
}

// TestCurrentBookVersionNilSafe 未注入 BookService 时回落到 0，不 panic
func TestCurrentBookVersionNilSafe(t *testing.T) {
	if v := currentBookVersion(nil, 1); v != 0 {
		t.Fatalf("nil BookService 应回落 0，实际 %d", v)
	}
}

// stage3FuncBody 解析 Go 源文件并返回指定函数的方法体源码（不含注释）。
func stage3FuncBody(t *testing.T, path, fnName string) string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("解析 %s 失败：%v", path, err)
	}
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Body == nil || fn.Name.Name != fnName {
			continue
		}
		var buf bytes.Buffer
		if perr := printer.Fprint(&buf, fset, fn.Body); perr != nil {
			t.Fatalf("打印 %s.%s 失败：%v", path, fnName, perr)
		}
		return buf.String()
	}
	t.Fatalf("%s 中找不到函数 %s", path, fnName)
	return ""
}

// TestReplayedIsNotBroadcast 幂等重放不得广播（P3-5）——源码级断言。
//
// 真实重放需要数据库（本机 CGO 不可用），但"是否在重放时广播"是一个结构性决策，
// 用源码断言可以对未来任何改动静默回归生效。
//
// GPT-C04 第二轮加固：改用 AST 极性校验（assertBroadcastGatedByNotReplayed）——
// 旧版 strings.Contains("!res.Replayed") 不校验极性，放过了
// `if res.Replayed || !res.Replayed` 这类可编译的极性翻转样本。
func TestReplayedIsNotBroadcast(t *testing.T) {
	assertBroadcastGatedByNotReplayed(t, "tx_handler.go", "createTransaction")
	body := stage3FuncBody(t, "tx_handler.go", "createTransaction")
	if !strings.Contains(body, "notifyDataChanged") {
		t.Fatalf("createTransaction 未调用 notifyDataChanged：%s", body)
	}
}

// TestUpdateDelegatesVisibilityToSingleImpl P1 不变量（服务层）：
// 两个入口共用**一套**可见性写实现 —— Update 不得自己拼 members_can_view_others 的 UPDATE，
// 必须委托 SetVisibility；名称分支仍要校验 RowsAffected（P3-2）。
func TestUpdateDelegatesVisibilityToSingleImpl(t *testing.T) {
	body := stage3FuncBody(t, "../service/book.go", "Update")

	if strings.Contains(body, "members_can_view_others") {
		t.Fatalf("BookService.Update 自己拼了 members_can_view_others 的 UPDATE（两套写实现）：%s", body)
	}
	if !strings.Contains(body, "SetVisibility") {
		t.Fatalf("BookService.Update 未委托 SetVisibility（唯一实现）：%s", body)
	}
	if !strings.Contains(body, "Cols(\"name\", \"updated_at\")") {
		t.Fatalf("BookService.Update 的 Cols 必须只含 name/updated_at：%s", body)
	}
	if !strings.Contains(body, "n != 1") {
		t.Fatalf("BookService.Update 缺少 RowsAffected != 1 校验（P3-2）：%s", body)
	}
	// changed 结果必须原样回传（handler 靠它决定是否广播）
	if !strings.Contains(body, "VisibilityChanged") {
		t.Fatalf("BookService.Update 未把 SetVisibility 的 changed 结果回传到 BookUpdateResult：%s", body)
	}
}

// stage3FuncDecl 解析源文件并返回指定函数声明（找不到即失败）。
func stage3FuncDecl(t *testing.T, path, fnName string) *ast.FuncDecl {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("解析 %s 失败：%v", path, err)
	}
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Body == nil || fn.Name.Name != fnName {
			continue
		}
		return fn
	}
	t.Fatalf("%s 中找不到函数 %s", path, fnName)
	return nil
}

// stage3CallCount 统计函数体内对 callee 的调用次数（选择器取 Sel 名）
func stage3CallCount(fn *ast.FuncDecl, callee string) int {
	n := 0
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch f := call.Fun.(type) {
		case *ast.SelectorExpr:
			if f.Sel.Name == callee {
				n++
			}
		case *ast.Ident:
			if f.Name == callee {
				n++
			}
		}
		return true
	})
	return n
}

// stage3FlagPolarity 条件里对目标标志的"期望极性"。
type stage3FlagPolarity int

const (
	// stage3PolarityConsistent 同一条件内必须极性一致（全裸引用或全取反），两种都能过。
	// 这是默认口径：守卫要防的是"把极性抹平"，而不是规定某个业务必须写成哪种。
	stage3PolarityConsistent stage3FlagPolarity = iota
	// stage3PolarityPositive 必须存在裸引用（取反深度为偶），且不得有取反引用。
	stage3PolarityPositive
	// stage3PolarityNegated 必须存在**一元取反**的引用，且不得有裸引用。
	stage3PolarityNegated
)

// stage3FlagRefs 统计条件表达式里对目标标志的引用次数，按"一元取反的奇偶深度"分类。
//
// 为什么要按奇偶而不是"出现过 ! 就算取反"：
//   - `!!replayed` 与 `replayed` 语义等价，若按"出现 ! 即取反"判定，作者可以用双重否定
//     把裸引用伪装成取反（旧守卫只能靠行为级测试兜住）；
//   - 把 `!replayed` 手写成 `replayed || !replayed` 之后，条件恒为真，等于"无条件广播"，
//     而旧守卫只检查"条件里出现过这个标识符"，照样通过。
//
// 选择器（res.VisibilityChanged）与裸标识符（changed）都要计入，且**不**重复计数：
// 前者只匹配 Sel，后者由 Ident 分支处理。
func stage3FlagRefs(expr ast.Expr, condIdent string) (positive, negated int) {
	count := func(depth int) {
		if depth%2 == 1 {
			negated++
		} else {
			positive++
		}
	}
	var walk func(e ast.Expr, depth int)
	walk = func(e ast.Expr, depth int) {
		switch n := e.(type) {
		case nil:
			return
		case *ast.ParenExpr:
			walk(n.X, depth)
		case *ast.UnaryExpr:
			if n.Op == token.NOT {
				walk(n.X, depth+1) // 取反深度 +1：!!x 回到偶数（等价裸引用）
				return
			}
			walk(n.X, depth)
		case *ast.StarExpr:
			walk(n.X, depth)
		case *ast.BinaryExpr:
			walk(n.X, depth)
			walk(n.Y, depth)
		case *ast.SelectorExpr:
			if strings.Contains(n.Sel.Name, condIdent) {
				count(depth)
			}
			walk(n.X, depth)
		case *ast.CallExpr:
			walk(n.Fun, depth)
			for _, a := range n.Args {
				walk(a, depth)
			}
		case *ast.IndexExpr:
			walk(n.X, depth)
			walk(n.Index, depth)
		case *ast.Ident:
			if strings.Contains(n.Name, condIdent) {
				count(depth)
			}
		}
	}
	walk(expr, 0)
	return positive, negated
}

// stage3CallInsideIfReferencing 断言 callee 的调用位于"条件里引用了 condIdent 的 if 分支"内。
//
// 加固（审计域微项）：旧实现只做 `strings.Contains(id.Name, condIdent)` 的"出现过吗"检查，
// 有两个可编译的绕过口子：
//
//	if replayed || !replayed { notify(...) }  // 极性被抹平 → 条件恒真 = 无条件广播
//	if !!replayed { notify(...) }             // 双重否定 → 语义是裸引用，却伪装成"取反"
//
// 现在要求：条件里对该标志的引用**极性一致**（不能既裸又取反），取反按一元 `!` 的
// **奇偶深度**判定。极性混杂直接判失败——它只可能来自手写笔误或刻意绕过。
//
// 需要更强口径（必须正向 / 必须取反）的调用方用 stage3CallInsideIfPolarity。
func stage3CallInsideIfReferencing(fn *ast.FuncDecl, condIdent, callee string) bool {
	return stage3CallInsideIfPolarity(fn, condIdent, callee, stage3PolarityConsistent)
}

// stage3CallInsideIfPolarity 同 stage3CallInsideIfReferencing，但额外约束极性与取反形式。
func stage3CallInsideIfPolarity(fn *ast.FuncDecl, condIdent, callee string, want stage3FlagPolarity) bool {
	if fn == nil || fn.Body == nil {
		return false
	}
	found := false
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		ifs, ok := node.(*ast.IfStmt)
		if !ok || ifs.Body == nil {
			return true
		}
		positive, negated := stage3FlagRefs(ifs.Cond, condIdent)
		if positive+negated == 0 {
			return true // 本 if 与目标标志无关
		}
		switch want {
		case stage3PolarityConsistent:
			// 极性混杂（既裸又取反）= 条件被写成恒真/恒假：拒绝
			if positive > 0 && negated > 0 {
				return true
			}
		case stage3PolarityPositive:
			if positive == 0 || negated > 0 {
				return true
			}
		case stage3PolarityNegated:
			if negated == 0 || positive > 0 {
				return true
			}
		}
		ast.Inspect(ifs.Body, func(b ast.Node) bool {
			call, isCall := b.(*ast.CallExpr)
			if !isCall {
				return true
			}
			switch f := call.Fun.(type) {
			case *ast.SelectorExpr:
				if f.Sel.Name == callee {
					found = true
				}
			case *ast.Ident:
				if f.Name == callee {
					found = true
				}
			}
			return true
		})
		return true
	})
	return found
}

// stage3ParseSnippet 把一段源码片段解析成函数声明，供守卫自身的"突变测试"使用。
//
// 为什么要给守卫写突变测试：守卫本身是"元断言"，它一旦退化成只要"出现过标识符"就
// 判通过，所有被它保护的用例都会集体失效而且**没有任何红灯**。因此这里把已知的
// 绕过写法（混杂极性 / 双重否定）当作输入，断言守卫必须拒绝它们。
func stage3ParseSnippet(t *testing.T, body string) *ast.FuncDecl {
	t.Helper()
	src := "package x\n\nfunc hero(flag bool) {\n" + body + "\n}\n"
	f, err := parser.ParseFile(token.NewFileSet(), "snippet.go", src, 0)
	if err != nil {
		t.Fatalf("解析片段失败：%v\n源码：%s", err, src)
	}
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok {
			return fn
		}
	}
	t.Fatalf("片段里没有函数声明：%s", src)
	return nil
}

// TestStage3GuardRejectsMixedPolarity 守卫自身的突变测试：三种绕过写法必须被拒绝。
func TestStage3GuardRejectsMixedPolarity(t *testing.T) {
	// ① 基准：正常写法必须通过
	ok := stage3ParseSnippet(t, "\tif flag {\n\t\tnotify()\n\t}")
	if !stage3CallInsideIfReferencing(ok, "flag", "notify") {
		t.Fatal("基准失败：`if flag { notify() }` 应被判定为「调用位于条件分支内」（守卫过严）")
	}

	// ② 手写笔误抹平极性：flag || !flag 恒为真，等于无条件广播
	mixed := stage3ParseSnippet(t, "\tif flag || !flag {\n\t\tnotify()\n\t}")
	if stage3CallInsideIfReferencing(mixed, "flag", "notify") {
		t.Fatal("守卫被 `if flag || !flag` 骗过：条件恒真等于无条件执行，必须判失败")
	}

	// ③ 双重否定：语义等于裸引用，按奇偶深度归入正向
	double := stage3ParseSnippet(t, "\tif !!flag {\n\t\tnotify()\n\t}")
	if stage3CallInsideIfPolarity(double, "flag", "notify", stage3PolarityNegated) {
		t.Fatal("`if !!flag` 不得被当成「取反」：双重否定的取反深度为偶数，语义是裸引用")
	}
	if !stage3CallInsideIfPolarity(double, "flag", "notify", stage3PolarityPositive) {
		t.Fatal("`if !!flag` 应按奇偶深度判定为正向引用")
	}

	// ④ 真正的取反：只有一元取反能满足 Negated 口径
	neg := stage3ParseSnippet(t, "\tif !flag {\n\t\tnotify()\n\t}")
	if !stage3CallInsideIfPolarity(neg, "flag", "notify", stage3PolarityNegated) {
		t.Fatal("`if !flag` 应被判定为取反引用")
	}
	if stage3CallInsideIfPolarity(neg, "flag", "notify", stage3PolarityPositive) {
		t.Fatal("`if !flag` 不得被判定为正向引用（极性判定失效）")
	}

	// ⑤ 调用不在条件分支内：一律不通过（否则守卫形同虚设）
	outside := stage3ParseSnippet(t, "\tif flag {\n\t\tother()\n\t}\n\tnotify()")
	if stage3CallInsideIfReferencing(outside, "flag", "notify") {
		t.Fatal("守卫把「条件分支之外」的调用误判为分支内调用")
	}
}

// TestBothVisibilityEntriesBroadcast 《落地方案》§5.2 不变量：
// 两个可见性入口都必须广播 visibility_changed，且"值没变"不得广播（广播必须在条件分支内）。
//
// 为什么要用源码断言：这是"只改一处"最容易漏的地方——将来有人只在
// PUT /books/{id} 或只在 PUT /books/{id}/visibility 里加/减广播，本用例立刻红。
func TestBothVisibilityEntriesBroadcast(t *testing.T) {
	// 1) 两个入口都必须广播
	for _, c := range []struct{ path, fn string }{
		{"book_handler.go", "update"},
		{"visibility_handler.go", "setVisibility"},
	} {
		fn := stage3FuncDecl(t, c.path, c.fn)
		if stage3CallCount(fn, "notifyVisibilityChanged") == 0 {
			t.Fatalf("%s.%s 未广播 visibility_changed：该入口改了开关但客户端不会重建", c.path, c.fn)
		}
	}

	// 2) PUT /books/:id：广播必须**恰好一次**且在 res.VisibilityChanged 分支内
	upd := stage3FuncDecl(t, "book_handler.go", "update")
	if n := stage3CallCount(upd, "notifyVisibilityChanged"); n != 1 {
		t.Fatalf("bookHandler.update 应恰好广播一次，实际 %d 次（无条件广播会让\"值没变\"也触发全量重拉）", n)
	}
	if !stage3CallInsideIfReferencing(upd, "VisibilityChanged", "notifyVisibilityChanged") {
		t.Fatal("bookHandler.update 的广播必须位于 res.VisibilityChanged 条件分支内（值没变不得广播）")
	}

	// 3) PUT /books/:id/visibility：同样必须在 changed 分支内
	vis := stage3FuncDecl(t, "visibility_handler.go", "setVisibility")
	if !stage3CallInsideIfReferencing(vis, "changed", "notifyVisibilityChanged") {
		t.Fatal("setVisibility 的广播必须位于 changed 条件分支内（幂等提交不得广播）")
	}
	// 4) 可见性写实现仍然只有一个
	if stage3CallCount(vis, "SetVisibility") != 1 {
		t.Fatalf("setVisibility 必须恰好调用一次 SetVisibility")
	}
}

// TestBookFullMapsTo409 成员上限 → 409（PRD §12 决策 6）
func TestBookFullMapsTo409(t *testing.T) {
	c, w := stage3Ctx(t, http.MethodPost, "/api/v1/books/1/members/join", "", nil, 0)
	mapSvcErr(c, service.ErrBookFull)
	if w.Code != http.StatusConflict {
		t.Fatalf("ErrBookFull 应映射 409，实际 %d（body=%s）", w.Code, w.Body.String())
	}
}
