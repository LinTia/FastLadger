package api

// GPT-C04 回归守卫：deleteTransaction / restoreTransaction 的幂等重放不得广播 data_changed。
//
// 与 TestReplayedIsNotBroadcast（createTransaction）同口径的源码结构断言：
// 真实重放需要数据库环境，但"是否在重放时广播"是结构性决策，用源码断言可以
// 对未来任何改动静默回归生效。广播必须位于 `!replayed` 条件分支内。

import "testing"

func TestC04MutateReplayIsNotBroadcast(t *testing.T) {
	for _, c := range []struct{ path, fn string }{
		{"tx_handler.go", "deleteTransaction"},
		{"tx_handler.go", "restoreTransaction"},
	} {
		fn := stage3FuncDecl(t, c.path, c.fn)
		body := stage3FuncBody(t, c.path, c.fn)

		if !stage3CallInsideIfReferencing(fn, "replayed", "notifyDataChanged") {
			t.Fatalf("%s.%s 的 notifyDataChanged 必须位于 replayed 条件分支内"+
				"（幂等重放不得广播 data_changed，GPT-C04）：%s", c.path, c.fn, body)
		}
		if n := stage3CallCount(fn, "notifyDataChanged"); n != 1 {
			t.Fatalf("%s.%s 应恰好广播一次（且在 !replayed 分支内），实际 %d 次：%s", c.path, c.fn, n, body)
		}
		// 必须走带重放标记的 Receipt 入口，而不是丢弃标记的 Delete/Restore
		want := "Receipt"
		if !containsStr(body, want) {
			t.Fatalf("%s.%s 必须调用 DeleteReceipt/RestoreReceipt（拿到 replayed 标记）：%s", c.path, c.fn, body)
		}
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
