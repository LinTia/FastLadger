package main

// GPT-R34【P2】CLI 参数非法时静默回退、仍继续执行破坏性清理 —— 回归测试。
//
// 本文件分两层：
//  1. 纯参数解析层（不需要数据库，本机无 gcc 也能跑）：逐条断言"非法输入必须报错"；
//  2. 真实数据库层（需要 CGO/gcc，Linux CI 上跑）：断言非法输入下【行数不变】，
//     并用一次合法清理做阳性对照，证明夹具本身是"可被清理的"，
//     否则"行数不变"会是一个空洞的断言。

import (
	"path/filepath"
	"testing"
	"time"

	"fastledger/internal/database"
	"fastledger/internal/migrate"
	"xorm.io/xorm"
)

// TestR34PurgeArgsRejectsInvalidInput 非法输入一律报错，绝不回落默认值。
//
// 这是本条的核心断言：把"静默回退"换成"显式失败"。
func TestR34PurgeArgsRejectsInvalidInput(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		args []string
	}{
		// —— 命令行覆盖值非法 ——
		{"格式错误-字母", nil, []string{"--recycle-ttl-days", "abc"}},
		{"格式错误-小数", nil, []string{"--recycle-ttl-days", "1.5"}},
		{"格式错误-空串", nil, []string{"--recycle-ttl-days", ""}},
		{"格式错误-带空格", nil, []string{"--recycle-ttl-days", " 30 "}},
		{"零值", nil, []string{"--recycle-ttl-days", "0"}},
		{"负数", nil, []string{"--recycle-ttl-days", "-5"}},
		{"越界-超过上限", nil, []string{"--recycle-ttl-days", "99999999"}},
		{"审计-格式错误", nil, []string{"--audit-retention-days", "abc"}},
		{"审计-零值", nil, []string{"--audit-retention-days", "0"}},
		{"审计-负数", nil, []string{"--audit-retention-days", "-1"}},
		{"等号形式非法", nil, []string{"--recycle-ttl-days=abc"}},
		// —— 缺值 / 未知参数 / 位置参数 ——
		{"缺值-只有flag", nil, []string{"--recycle-ttl-days"}},
		{"缺值-审计", nil, []string{"--audit-retention-days"}},
		{"未知参数", nil, []string{"--unknown", "1"}},
		{"未知参数-拼写错误", nil, []string{"--recycle-ttl-day", "10"}},
		{"位置参数", nil, []string{"30"}},
		// —— 有环境变量兜底时，非法 CLI 值同样不得被忽略 ——
		{"非法值不得被环境变量掩盖",
			map[string]string{"EBK_RECYCLE_TTL_DAYS": "90"},
			[]string{"--recycle-ttl-days", "abc"}},
		{"非法值不得被环境变量掩盖-零值",
			map[string]string{"EBK_RECYCLE_TTL_DAYS": "90"},
			[]string{"--recycle-ttl-days", "0"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// 清空两个环境变量，保证用例之间互不干扰（t.Setenv 在并行时禁止，这里不并行）
			t.Setenv(recycleTTLEnv, "")
			t.Setenv(auditRetentionEnv, "")
			for k, v := range c.env {
				t.Setenv(k, v)
			}
			opts, err := resolvePurgeOptions(c.args)
			if err == nil {
				t.Fatalf("%q：非法输入应报错退出，实际被接受（recycle=%d audit=%d）——"+
					"这就是 GPT-R34 要消灭的静默回退",
					c.args, opts.RecycleTTLDays, opts.AuditRetentionDays)
			}
			if opts != nil {
				t.Fatalf("%q：解析失败时不应返回可用参数，实际 %+v", c.args, opts)
			}
		})
	}
}

// TestR34PurgeEnvRejectsInvalidInput 环境变量非法同样 fail-fast（不回落默认值）
func TestR34PurgeEnvRejectsInvalidInput(t *testing.T) {
	cases := []struct {
		name       string
		recycleEnv string
		auditEnv   string
	}{
		{"回收站-字母", "abc", ""},
		{"回收站-零", "0", ""},
		{"回收站-负数", "-5", ""},
		{"回收站-越界", "99999999", ""},
		{"审计-字母", "", "abc"},
		{"审计-零", "", "0"},
		{"审计-负数", "", "-1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(recycleTTLEnv, c.recycleEnv)
			t.Setenv(auditRetentionEnv, c.auditEnv)
			if _, err := resolvePurgeOptions(nil); err == nil {
				t.Fatalf("环境变量非法应报错：recycle=%q audit=%q", c.recycleEnv, c.auditEnv)
			}
		})
	}
}

// TestR34PurgeArgsAcceptsValidInput 合法输入必须按预期生效（不能矫枉过正把所有输入都拒了）
func TestR34PurgeArgsAcceptsValidInput(t *testing.T) {
	cases := []struct {
		name       string
		recycleEnv string
		auditEnv   string
		args       []string
		wantRecyc  int
		wantAudit  int
	}{
		{"全部默认", "", "", nil, 30, 365},
		{"环境变量生效", "90", "180", nil, 90, 180},
		{"CLI 覆盖", "", "", []string{"--recycle-ttl-days", "7"}, 7, 365},
		{"CLI 覆盖两个", "", "", []string{"--recycle-ttl-days", "7", "--audit-retention-days", "14"}, 7, 14},
		{"CLI 优先于环境变量", "90", "180", []string{"--recycle-ttl-days", "1"}, 1, 180},
		{"等号形式合法", "", "", []string{"--recycle-ttl-days=3"}, 3, 365},
		{"单横线形式合法", "", "", []string{"-recycle-ttl-days", "3"}, 3, 365},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(recycleTTLEnv, c.recycleEnv)
			t.Setenv(auditRetentionEnv, c.auditEnv)
			opts, err := resolvePurgeOptions(c.args)
			if err != nil {
				t.Fatalf("%q：合法输入不应报错：%v", c.args, err)
			}
			if opts.RecycleTTLDays != c.wantRecyc {
				t.Fatalf("%q：recycle 期望 %d，实际 %d", c.args, c.wantRecyc, opts.RecycleTTLDays)
			}
			if opts.AuditRetentionDays != c.wantAudit {
				t.Fatalf("%q：audit 期望 %d，实际 %d", c.args, c.wantAudit, opts.AuditRetentionDays)
			}
		})
	}
}

// TestR34PurgeRejectsBadFlagLeavesRows 非法参数下【数据库行数不变】。
//
// 这是 GPT-R34 最关键的验证：硬删除不可恢复，因此"参数没解析对"必须等价于
// "一条都没删"。末尾用一次合法清理做阳性对照——若夹具本身不会被清理，
// "行数不变"就是空洞断言，阳性对照专门堵这个漏洞。
func TestR34PurgeRejectsBadFlagLeavesRows(t *testing.T) {
	engine := openRealSQLiteWithSchema(t)
	now := time.Now().Unix()
	seedPurgeFixtures(t, engine, now)

	badArgs := [][]string{
		{"--recycle-ttl-days", "abc"},
		{"--recycle-ttl-days", "0"},
		{"--recycle-ttl-days", "-5"},
		{"--recycle-ttl-days"},
		{"--audit-retention-days", "abc"},
		{"--unknown", "1"},
		{"30"},
	}
	for _, args := range badArgs {
		t.Setenv(recycleTTLEnv, "")
		t.Setenv(auditRetentionEnv, "")
		if _, _, _, _, err := purgeWithOptions(engine, args); err == nil {
			t.Fatalf("%q：非法参数应报错，实际被接受", args)
		}
		if got := countRows(t, engine, "transactions"); got != seedTxRows {
			t.Fatalf("%q：非法参数不得删除任何数据，transactions 行数 %d → %d",
				args, seedTxRows, got)
		}
		if got := countRows(t, engine, "audit_logs"); got != seedAuditRows {
			t.Fatalf("%q：非法参数不得删除任何数据，audit_logs 行数 %d → %d",
				args, seedAuditRows, got)
		}
	}

	// 阳性对照：合法参数确实会清掉到期数据，证明上面的夹具不是"本来就删不动"的
	t.Setenv(recycleTTLEnv, "")
	t.Setenv(auditRetentionEnv, "")
	recycled, audited, _, _, err := purgeWithOptions(engine,
		[]string{"--recycle-ttl-days", "1", "--audit-retention-days", "1"})
	if err != nil {
		t.Fatalf("合法参数不应报错：%v", err)
	}
	if recycled != int64(seedTxRows) {
		t.Fatalf("阳性对照失败：期望清掉 %d 条墓碑，实际 %d 条", seedTxRows, recycled)
	}
	if audited != int64(seedAuditRows) {
		t.Fatalf("阳性对照失败：期望清掉 %d 条审计，实际 %d 条", seedAuditRows, audited)
	}
	if got := countRows(t, engine, "transactions"); got != 0 {
		t.Fatalf("合法清理后 transactions 应清空，实际剩 %d 条", got)
	}
}

// seedTxRows / seedAuditRows 夹具规模（均为"早已到期"的数据）
const (
	seedTxRows    = 3
	seedAuditRows = 2
)

// seedPurgeFixtures 插入到期墓碑与到期审计（deleted_at / at 远早于当前时间）
func seedPurgeFixtures(t *testing.T, engine *xorm.Engine, now int64) {
	t.Helper()
	old := now - 400*24*3600 // 400 天前：任何合法 TTL 都覆盖得到
	sqlDB := engine.DB().DB
	if _, err := sqlDB.Exec(
		`INSERT INTO books(id, name, owner_uid, created_at, updated_at) VALUES (1, 'r34', 1, ?, ?)`,
		now, now); err != nil {
		t.Fatalf("插入账本失败：%v", err)
	}
	for i := 1; i <= seedTxRows; i++ {
		if _, err := sqlDB.Exec(
			`INSERT INTO transactions(book_id, type, account_id, amount, tx_time, deleted, created_at, updated_at, deleted_at)
			 VALUES (1, 2, 1, ?, ?, 1, ?, ?, ?)`,
			100*int64(i), old, old, old, old); err != nil {
			t.Fatalf("插入墓碑交易失败：%v", err)
		}
	}
	for i := 1; i <= seedAuditRows; i++ {
		if _, err := sqlDB.Exec(
			`INSERT INTO audit_logs(book_id, actor_uid, action, entity_type, entity_id, at, source)
			 VALUES (1, 0, 'delete', 'transaction', ?, ?, 'cli')`,
			i, old); err != nil {
			t.Fatalf("插入审计失败：%v", err)
		}
	}
}

// countRows 统计表行数
func countRows(t *testing.T, engine *xorm.Engine, table string) int64 {
	t.Helper()
	var n int64
	if err := engine.DB().DB.QueryRow("SELECT COUNT(1) FROM " + table).Scan(&n); err != nil {
		t.Fatalf("统计 %s 行数失败：%v", table, err)
	}
	return n
}

// openRealSQLiteWithSchema 打开真实 SQLite 并应用全部迁移。
//
// 本机（无 gcc / CGO_ENABLED=0）驱动不可用，此时【显式跳过并说明原因】：
// 这不是为了让结果变绿而放宽断言，而是平台能力缺失——断言本身保持不变，
// Linux（CI runner 自带 gcc）上会真实执行。
func openRealSQLiteWithSchema(t *testing.T) *xorm.Engine {
	t.Helper()
	engine, err := database.Init(filepath.Join(t.TempDir(), "r34.db"))
	if err != nil {
		t.Skipf("真实 SQLite 不可用（mattn/go-sqlite3 需 CGO/gcc，本机无 gcc），"+
			"跳过依赖数据库的断言：%v", err)
	}
	t.Cleanup(func() { _ = engine.Close() })
	if _, err := migrate.Apply(engine); err != nil {
		t.Fatalf("迁移失败：%v", err)
	}
	return engine
}
