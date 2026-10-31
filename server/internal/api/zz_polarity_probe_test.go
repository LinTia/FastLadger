package api

// zz_polarity_probe_test.go —— 对 stage3 极性守卫加固（c3ff644）的**对抗性验证**。
//
// 我的立场：守卫是"元断言"，它一旦退化成"条件里出现过标识符就通过"，
// 所有被它保护的用例会集体失效且没有任何红灯。因此这里把已知绕过写法与**新的**绕过写法
// 一起喂给它，逐条断言"必须拒绝"。
//
// 前四组（ZZP-1~4）是守卫应当已覆盖的形态：必须通过。
// 后两组（ZZP-5~6）是我这轮新造的同义绕过：**断言必须拒绝**——若失败即为残留缺口证据。

import (
	"testing"
)

// TestZZP1MixedPolarityRejected 我上轮报的两种姿势（及选择器变体）必须被拒（回归守卫）。
func TestZZP1MixedPolarityRejected(t *testing.T) {
	cases := []struct{ name, body, ident string }{
		{"裸引用或取反（恒真）", "\tif flag || !flag {\n\t\tnotify()\n\t}", "flag"},
		{"选择器形态的极性混杂", "\tif res.VisibilityChanged || !res.VisibilityChanged {\n\t\tnotify()\n\t}", "VisibilityChanged"},
	}
	for _, c := range cases {
		fn := stage3ParseSnippet(t, c.body)
		if stage3CallInsideIfReferencing(fn, c.ident, "notify") {
			t.Fatalf("[%s] 守卫被「%s」骗过：极性混杂意味着条件恒真 = 无条件执行", c.name, c.body)
		}
	}
}

// TestZZP2DoubleNegationIsPositive 双重否定按奇偶深度归正向，不得冒充取反。
func TestZZP2DoubleNegationIsPositive(t *testing.T) {
	fn := stage3ParseSnippet(t, "\tif !!flag {\n\t\tnotify()\n\t}")
	if stage3CallInsideIfPolarity(fn, "flag", "notify", stage3PolarityNegated) {
		t.Fatal("`if !!flag` 被冒充成「取反」：双重否定语义是裸引用")
	}
	if !stage3CallInsideIfPolarity(fn, "flag", "notify", stage3PolarityPositive) {
		t.Fatal("`if !!flag` 未被判定为正向引用（奇偶深度统计失效）")
	}
	// 三重否定 = 取反
	triple := stage3ParseSnippet(t, "\tif !!!flag {\n\t\tnotify()\n\t}")
	if !stage3CallInsideIfPolarity(triple, "flag", "notify", stage3PolarityNegated) {
		t.Fatal("`if !!!flag` 应按奇偶深度判定为取反")
	}
}

// TestZZP3ParenthesizedAndNestedNegation 加括号 / 复合表达式里的取反必须仍被正确识别。
func TestZZP3ParenthesizedAndNestedNegation(t *testing.T) {
	// !(flag) 等价 !flag
	paren := stage3ParseSnippet(t, "\tif !(flag) {\n\t\tnotify()\n\t}")
	if !stage3CallInsideIfPolarity(paren, "flag", "notify", stage3PolarityNegated) {
		t.Fatal("`if !(flag)` 应被识别为取反引用（括号不应改变极性）")
	}
	if stage3CallInsideIfPolarity(paren, "flag", "notify", stage3PolarityPositive) {
		t.Fatal("`if !(flag)` 被判成正向引用：括号穿透失效")
	}
	// !(true && flag) 等价 !flag
	nested := stage3ParseSnippet(t, "\tif !(true && flag) {\n\t\tnotify()\n\t}")
	if !stage3CallInsideIfPolarity(nested, "flag", "notify", stage3PolarityNegated) {
		t.Fatal("`if !(true && flag)` 应被识别为取反引用")
	}
	// 括号里的混杂极性同样要拒
	mixedParen := stage3ParseSnippet(t, "\tif (flag) || (!flag) {\n\t\tnotify()\n\t}")
	if stage3CallInsideIfReferencing(mixedParen, "flag", "notify") {
		t.Fatal("`if (flag) || (!flag)` 被守卫放行：括号是绕过极性检查的口子")
	}
}

// TestZZP4CallMustBeInsideIfBody 调用不在条件分支内 / 在 else 分支里，一律不通过。
func TestZZP4CallMustBeInsideIfBody(t *testing.T) {
	outside := stage3ParseSnippet(t, "\tif flag {\n\t\tother()\n\t}\n\tnotify()")
	if stage3CallInsideIfReferencing(outside, "flag", "notify") {
		t.Fatal("守卫把「条件分支之外」的调用误判为分支内调用")
	}
	elseBranch := stage3ParseSnippet(t, "\tif flag {\n\t\tother()\n\t} else {\n\t\tnotify()\n\t}")
	if stage3CallInsideIfReferencing(elseBranch, "flag", "notify") {
		t.Fatal("守卫把 else 分支里的调用当成「受条件保护」：语义相反")
	}
}

// TestZZP5TautologyViaConstantRejected 新变体：用常量把条件写成恒真，极性检查看不到。
//
// `if flag || true` 与 `if flag || !flag` 语义完全等价（恒真 = 无条件执行），
// 但它只有一处**正向**引用，极性一致，因此能穿过"极性一致"这道检查。
// 期望：守卫必须拒绝（或至少不把它当作"受条件保护"）。若通过即为残留缺口。
func TestZZP5TautologyViaConstantRejected(t *testing.T) {
	tautologies := []struct{ name, body string }{
		{"正向引用或常量真", "\tif flag || true {\n\t\tnotify()\n\t}"},
		{"常量真或正向引用", "\tif true || flag {\n\t\tnotify()\n\t}"},
		{"取反或常量真", "\tif true || !flag {\n\t\tnotify()\n\t}"},
		{"正向引用与常量假", "\tif flag && false {\n\t\tnotify()\n\t}"},
	}
	for _, c := range tautologies {
		fn := stage3ParseSnippet(t, c.body)
		if stage3CallInsideIfReferencing(fn, "flag", "notify") {
			t.Errorf("[残留缺口] 守卫被「%s」骗过：条件恒真（或恒假），调用实际上不受 flag 保护，"+
				"而守卫判定为「位于条件分支内」。极性奇偶只能识别 !x 形式，识别不了常量恒等", c.name)
		}
	}
}

// TestZZP6SubstringIdentMatching 新变体：靠"子串匹配"蹭到条件里。
//
// stage3FlagRefs 用 strings.Contains(n.Name, condIdent) 判定，
// 因此 `if notChanged { notify() }` 会被算作对 `changed` 的**正向**引用（极性一致 → 放行），
// 而它的语义与 `changed` 无关。这是我上轮没提到的第二个绕过面：条件用的标识符名字里
// 恰好包含目标标志名。
func TestZZP6SubstringIdentMatching(t *testing.T) {
	// 注意大小写：真实调用点用的小写标识符是 "changed"，
	// 因此 `unchanged` 是 strings.Contains 命中的实际形态。
	fn := stage3ParseSnippet(t, "\tif unchanged {\n\t\tnotify()\n\t}")
	if stage3CallInsideIfReferencing(fn, "changed", "notify") {
		t.Error("[残留缺口] 守卫把 `if unchanged` 当成对 `changed` 的引用并判定通过：" +
			"strings.Contains 的子串匹配让「名字里含目标标志」的无关变量也能骗过守卫；" +
			"建议改为精确匹配（Ident 全等 / Selector 的 Sel 全等）")
	}
}

// ---------- 第三轮（守卫再硬化之后）：接受侧是否被过严误伤 ----------

// TestZZP7LegitConditionsStillAccepted 硬化的方向必须是"宁可误拒也不放过"，
// 但**误拒**同样有代价：守卫一旦把正常写法也判失败，下一个人就会把它放宽。
// 本用例钉住一批真实可能出现的正常形态，确保它们仍被认作"由标志支配"。
func TestZZP7LegitConditionsStillAccepted(t *testing.T) {
	cases := []struct{ name, body, ident string }{
		{"裸引用", "\tif flag {\n\t\tnotify()\n\t}", "flag"},
		{"选择器裸引用", "\tif res.VisibilityChanged {\n\t\tnotify()\n\t}", "VisibilityChanged"},
		{"与另一个非布尔量相与", "\tif res.VisibilityChanged && needPush {\n\t\tnotify()\n\t}", "VisibilityChanged"},
		{"与常量 true 相与（不构成恒真）", "\tif flag && true {\n\t\tnotify()\n\t}", "flag"},
		{"== true 形态", "\tif flag == true {\n\t\tnotify()\n\t}", "flag"},
		{"!= false 形态", "\tif flag != false {\n\t\tnotify()\n\t}", "flag"},
		{"括号包裹", "\tif (flag) {\n\t\tnotify()\n\t}", "flag"},
		{"双重否定", "\tif !!flag {\n\t\tnotify()\n\t}", "flag"},
	}
	for _, c := range cases {
		fn := stage3ParseSnippet(t, c.body)
		if !stage3CallInsideIfReferencing(fn, c.ident, "notify") {
			t.Errorf("[%s] 正常写法被守卫误拒：「%s」——过度收紧会让后来者把守卫放宽", c.name, c.body)
		}
	}
}

// TestZZP8NewBypassAttemptsRejected 第三轮新造的绕过变体，全部必须被拒。
//
// 与 ZZP5 的区别：这里不再依赖布尔字面量，而是用"同一标志互或/自比较/取反包或"
// 等恒等式形态，检验支配关系分析是否比"常量字面量特判"更普适。
func TestZZP8NewBypassAttemptsRejected(t *testing.T) {
	cases := []struct{ name, body, ident string }{
		{"同一标志自或（恒真）", "\tif flag || flag {\n\t\tnotify()\n\t}", "flag"},
		{"标志与自身比较（恒真）", "\tif flag == flag {\n\t\tnotify()\n\t}", "flag"},
		{"取反后包一层或", "\tif !(flag || false) {\n\t\tnotify()\n\t}", "flag"},
		{"不等式或常量真", "\tif flag != false || true {\n\t\tnotify()\n\t}", "flag"},
		{"括号化的或恒真", "\tif (flag) || (true) {\n\t\tnotify()\n\t}", "flag"},
		{"选择器同名近似变量", "\tif res.NotChanged {\n\t\tnotify()\n\t}", "Changed"},
		{"常量真与或", "\tif true && (flag || true) {\n\t\tnotify()\n\t}", "flag"},
	}
	for _, c := range cases {
		fn := stage3ParseSnippet(t, c.body)
		if stage3CallInsideIfReferencing(fn, c.ident, "notify") {
			t.Errorf("[%s] 守卫仍被绕过：「%s」——条件并不由 %s 支配", c.name, c.body, c.ident)
		}
	}
}
