package api

// SYNC-01 接线守卫：NewRouter 必须把 config 的 WS 连接数上限注入进程级 Hub。
//
// 为什么要这条用例：连接上限的缺省语义是"NewHub 不限、由 api.NewRouter 从
// config 注入"（保持既有测试行为，生产由 config.Load 保证恒为正）。
// 这意味着**安全性依赖这段接线存在**——若将来重构路由时有人删掉
// `ws.DefaultHub.SetConnectionLimits(...)` 那两行，生产会静默退回
// "连接无上限"（SYNC-01 的放大面重新打开），而普通用例不会失败。
// 本用例把接线本身钉死：删了就红，效果等同于兜底、且不牺牲压测覆盖。

import (
	"testing"

	"fastledger/internal/config"
	"fastledger/internal/ws"
)

// TestNewRouterWiresWSConnectionLimitsFromConfig NewRouter 必须把 config 注入 Hub
func TestNewRouterWiresWSConnectionLimitsFromConfig(t *testing.T) {
	// DefaultHub 是进程级单例：先保存当前值，用例结束恢复，避免污染其它用例
	oldPerUser, oldTotal := ws.DefaultHub.ConnectionLimits()
	t.Cleanup(func() { ws.DefaultHub.SetConnectionLimits(oldPerUser, oldTotal) })

	// 1) config 默认值路径（等价于生产未设置环境变量时 Load() 的产出）
	ws.DefaultHub.SetConnectionLimits(0, 0) // 先归零，排除前序用例残留
	NewRouter(Deps{Config: &config.Config{
		WSMaxConnPerUser: config.DefaultWSMaxConnPerUser,
		WSMaxConnTotal:   config.DefaultWSMaxConnTotal,
	}})
	if perUser, total := ws.DefaultHub.ConnectionLimits(); perUser != config.DefaultWSMaxConnPerUser ||
		total != config.DefaultWSMaxConnTotal {
		t.Fatalf("NewRouter 未把 config 的 WS 连接数上限注入 Hub：期望 (%d, %d)，实际 (%d, %d)",
			config.DefaultWSMaxConnPerUser, config.DefaultWSMaxConnTotal, perUser, total)
	}

	// 2) 自定义值路径：运维通过环境变量调大/调小后必须原样生效
	const wantPerUser, wantTotal = 7, 42
	ws.DefaultHub.SetConnectionLimits(0, 0)
	NewRouter(Deps{Config: &config.Config{
		WSMaxConnPerUser: wantPerUser,
		WSMaxConnTotal:   wantTotal,
	}})
	if perUser, total := ws.DefaultHub.ConnectionLimits(); perUser != wantPerUser || total != wantTotal {
		t.Fatalf("NewRouter 注入的上限与 config 不一致：期望 (%d, %d)，实际 (%d, %d)",
			wantPerUser, wantTotal, perUser, total)
	}
}
