package migrate_test

// INFRA-02 / INFRA-03 / N2 / N3 的回归测试：版本记录与真实结构的一致性。
//
// 吸收自复审探针 zz_infra_probe_test.go（ProbeA/B/C/D/E）与
// zz_crosscheck_probe_test.go（缺口矩阵），探针文件已删除、证据在此保留。
//
// 实测矩阵（修复前）：缺 1/2/6 可自愈（脚本恰好幂等）；缺 3/4/5 死锁
// （ALTER TABLE ADD COLUMN 重跑必 duplicate column），且 Check 拒绝启动、
// db migrate 又失败，形成死循环；"登记了 1..6 实际只执行了 0001"的库 Check 假 OK。
// 修复后：非幂等 DDL 的版本登记了结构断言，缺口可自愈；Check 增加只读结构核验。

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fastledger/internal/database"
	"fastledger/internal/migrate"
	"xorm.io/xorm"
)

// openReal 打开一个真实 SQLite 库（CGO 不可用时跳过）
func openReal(t *testing.T, name string) (*xorm.Engine, *sql.DB) {
	t.Helper()
	engine, err := database.Init(filepath.Join(t.TempDir(), name))
	if err != nil {
		t.Skipf("真实 SQLite 不可用（需 CGO/gcc）：%v", err)
	}
	t.Cleanup(func() { _ = engine.Close() })
	return engine, engine.DB().DB
}

// scriptOf 读取磁盘上的迁移脚本正文（测试 CWD = 本包目录）
func scriptOf(t *testing.T, file string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("migrations", file))
	if err != nil {
		t.Fatalf("读取迁移脚本 %s 失败：%v", file, err)
	}
	return string(b)
}

// applyManual 手工执行若干迁移脚本并登记版本（模拟"老库停在某个版本"）
func applyManual(t *testing.T, db *sql.DB, files ...string) {
	t.Helper()
	for i, f := range files {
		if _, err := db.Exec(scriptOf(t, f)); err != nil {
			t.Fatalf("手工执行 %s 失败：%v", f, err)
		}
		if err := db.QueryRow(
			`SELECT COUNT(1) FROM schema_migrations WHERE version = ?`, int64(i+1)).Scan(new(int)); err != nil && !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("探测版本表失败：%v", err)
		}
		if _, err := db.Exec(
			`INSERT OR REPLACE INTO schema_migrations(version, name, applied_at) VALUES (?, ?, 0)`,
			int64(i+1), f); err != nil {
			t.Fatalf("登记版本 %d 失败：%v", i+1, err)
		}
	}
}

// versionsOfDB 读出已应用版本
func versionsOfDB(t *testing.T, db *sql.DB) []int64 {
	t.Helper()
	rows, err := db.Query(`SELECT version FROM schema_migrations ORDER BY version`)
	if err != nil {
		t.Fatalf("读取已应用版本失败：%v", err)
	}
	defer rows.Close()
	out := []int64{}
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("解析失败：%v", err)
		}
		out = append(out, v)
	}
	return out
}

// hasColumn 判断某表是否已有某列（PRAGMA table_info）
func hasColumn(t *testing.T, db *sql.DB, table, col string) bool {
	t.Helper()
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		t.Fatalf("读取 %s 表结构失败：%v", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("解析表结构失败：%v", err)
		}
		if name == col {
			return true
		}
	}
	return false
}

// buildV6 从空库一路 Apply 到 v6（缺口场景的公共基线）
func buildV6(t *testing.T, e *xorm.Engine) {
	t.Helper()
	n, err := migrate.Apply(e)
	if err != nil {
		t.Fatalf("基线：空库 Apply 到 v6 失败：%v", err)
	}
	if n != 6 {
		t.Fatalf("基线：空库应应用 6 个迁移，实际 %d", n)
	}
	if st, err := migrate.Check(e); err != nil || !st.OK {
		t.Fatalf("基线：v6 库 Check 应 OK：st=%+v err=%v", st, err)
	}
}

// TestR2OldDBOnlyV1UpgradesToLatest 只有 0001 的老库一路升到最新（吸收 ProbeA）
//
// 每条脚本都是【多语句】文本；若驱动只执行首句，0003 的 3 个 ALTER 只会生效 1 个。
// 这类"静默少执行"是老库升级最难发现的损失，逐列断言堵住它。
func TestR2OldDBOnlyV1UpgradesToLatest(t *testing.T) {
	engine, db := openReal(t, "r2a.db")
	applyManual(t, db, "0001_init.up.sql")

	n, err := migrate.Apply(engine)
	if err != nil {
		t.Fatalf("老库（仅 v1）升级失败：%v", err)
	}
	if n != 5 {
		t.Fatalf("期望应用 5 个（0002..0006），实际 %d", n)
	}
	st, err := migrate.Check(engine)
	if err != nil || !st.OK || st.Current != 6 {
		t.Fatalf("升级后 Check 应 OK@v6：st=%+v err=%v", st, err)
	}
	for _, c := range []string{"creator_uid", "source", "timezone_offset", "direction"} {
		if !hasColumn(t, db, "transactions", c) {
			t.Fatalf("老库升级后 transactions 仍缺列 %q（多语句脚本可能只执行了首句）", c)
		}
	}
	if !hasColumn(t, db, "idempotency_records", "op") {
		t.Fatal("老库升级后 idempotency_records 仍缺列 op")
	}
}

// TestR2OldDBV4UpgradesToLatest 停在 v4 的老库升到最新（吸收 ProbeB）
func TestR2OldDBV4UpgradesToLatest(t *testing.T) {
	engine, db := openReal(t, "r2b.db")
	applyManual(t, db, "0001_init.up.sql", "0002_invites_and_member_unique.up.sql",
		"0003_transactions_creator_uid.up.sql", "0004_tx_direction.up.sql")

	n, err := migrate.Apply(engine)
	if err != nil {
		t.Fatalf("v4 老库升级失败：%v", err)
	}
	if n != 2 {
		t.Fatalf("期望应用 2 个（0005/0006），实际 %d", n)
	}
	if st, err := migrate.Check(engine); err != nil || !st.OK || st.Current != 6 {
		t.Fatalf("v4 老库升级后 Check 应 OK@v6：st=%+v err=%v", st, err)
	}
	if !hasColumn(t, db, "idempotency_records", "op") {
		t.Fatal("v4 老库升级后仍缺列 op")
	}
}

// TestR2MissingNonIdempotentVersionSelfHeals 缺失非幂等版本（3/4/5）必须自愈。
//
// 修复前这是死锁：结构已在、版本记录丢失 → Apply 重跑 ALTER 必失败 →
// Check 拒绝启动 → 提示 db migrate → 又失败。修复后按结构断言补登记版本。
func TestR2MissingNonIdempotentVersionSelfHeals(t *testing.T) {
	for _, gap := range []int64{3, 4, 5} {
		t.Run(fmt.Sprintf("缺%d", gap), func(t *testing.T) {
			engine, db := openReal(t, "r2gap.db")
			buildV6(t, engine)
			if _, err := db.Exec(`DELETE FROM schema_migrations WHERE version = ?`, gap); err != nil {
				t.Fatalf("删除版本 %d 记录失败：%v", gap, err)
			}
			// 服务启动视角：Check 必须报错（不能装作没事）
			if _, err := migrate.Check(engine); err == nil {
				t.Fatalf("缺 %d 时 Check 应报错", gap)
			}
			// 运维执行 db migrate：必须自愈（不再死锁）
			if _, err := migrate.Apply(engine); err != nil {
				t.Fatalf("缺 %d 时 Apply 应自愈，实际失败：%v", gap, err)
			}
			st, err := migrate.Check(engine)
			if err != nil || !st.OK || st.Current != 6 {
				t.Fatalf("自愈后 Check 应 OK@v6：st=%+v err=%v", st, err)
			}
			got := versionsOfDB(t, db)
			if fmt.Sprint(got) != "[1 2 3 4 5 6]" {
				t.Fatalf("自愈后版本集合应为 [1 2 3 4 5 6]，实际 %v", got)
			}
		})
	}
}

// TestR2EmptyVersionTableHealsToLatest 版本表存在但【零记录】必须能一路自愈到 v6。
//
// 修复前的行为：Apply 先成功应用 0001/0002，再死在 0003（结构已在），
// 把状态恶化成"半截记录"；修复后逐个版本按断言判定，可全部补登记。
func TestR2EmptyVersionTableHealsToLatest(t *testing.T) {
	engine, db := openReal(t, "r2empty.db")
	buildV6(t, engine)
	if _, err := db.Exec(`DELETE FROM schema_migrations`); err != nil {
		t.Fatalf("清空版本表失败：%v", err)
	}

	if _, err := migrate.Check(engine); err == nil {
		t.Fatal("版本表无记录时 Check 应报错")
	}
	if _, err := migrate.Apply(engine); err != nil {
		t.Fatalf("零记录场景 Apply 应一路自愈：%v", err)
	}
	st, err := migrate.Check(engine)
	if err != nil || !st.OK || st.Current != 6 {
		t.Fatalf("零记录自愈后 Check 应 OK@v6：st=%+v err=%v", st, err)
	}
	if got := versionsOfDB(t, db); fmt.Sprint(got) != "[1 2 3 4 5 6]" {
		t.Fatalf("自愈后版本集合应为 [1 2 3 4 5 6]，实际 %v", got)
	}
}

// TestR2SelfHealMustNotMaskRealFailure 【反向】断言不通过时 Apply 必须仍然失败。
//
// 构造：缺 3 的版本记录，且 0003 的产物之一（idx_tx_creator）被手工删掉——
// 此时 0003 重跑会因 duplicate column 失败，而断言也过不了（索引缺失），
// 这不是"版本记录缺口"，绝不能被自愈逻辑吞掉。
func TestR2SelfHealMustNotMaskRealFailure(t *testing.T) {
	engine, db := openReal(t, "r2real.db")
	buildV6(t, engine)
	if _, err := db.Exec(`DROP INDEX idx_tx_creator`); err != nil {
		t.Fatalf("预置索引缺失失败：%v", err)
	}
	if _, err := db.Exec(`DELETE FROM schema_migrations WHERE version = 3`); err != nil {
		t.Fatalf("删除版本 3 记录失败：%v", err)
	}

	before := versionsOfDB(t, db)
	n, err := migrate.Apply(engine)
	if err == nil {
		t.Fatalf("结构缺失时不得自愈，实际 Apply 成功（n=%d）", n)
	}
	if !strings.Contains(err.Error(), "idx_tx_creator") && !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("错误应指向真实原因，实际：%v", err)
	}
	if after := versionsOfDB(t, db); fmt.Sprint(after) != fmt.Sprint(before) {
		t.Fatalf("失败后版本集合不得被改动：%v → %v", before, after)
	}
	if _, err := migrate.Check(engine); err == nil {
		t.Fatal("未修复状态下 Check 仍应报错")
	}
}

// TestR2UnknownVersionIsActionable 降级运行（库里有本地没有的版本）必须给出可行动的错误。
//
// 修复前：Apply 对 unknown 版本不闻不问，返回 n=0 且无错误，运维以为迁移"成功"了，
// 服务却依旧起不来。
func TestR2UnknownVersionIsActionable(t *testing.T) {
	engine, db := openReal(t, "r2unknown.db")
	buildV6(t, engine)
	if _, err := db.Exec(
		`INSERT OR REPLACE INTO schema_migrations(version, name, applied_at) VALUES (7, '0007_future.up.sql', 0)`); err != nil {
		t.Fatalf("插入未知版本失败：%v", err)
	}

	// Check：报版本不一致
	_, errCheck := migrate.Check(engine)
	if errCheck == nil {
		t.Fatal("存在未知版本时 Check 应报错")
	}
	// Apply：必须明确拒绝，并指出这是"库比程序新"，而不是默默返回成功
	n, errApply := migrate.Apply(engine)
	if errApply == nil {
		t.Fatalf("降级运行时 Apply 应报错，实际 n=%d 无错误", n)
	}
	if !errors.Is(errApply, migrate.ErrVersionMismatch) {
		t.Fatalf("错误应包装 ErrVersionMismatch，实际：%v", errApply)
	}
	for _, want := range []string{"降级", "备份", "db migrate"} {
		if !strings.Contains(errApply.Error(), want) {
			t.Fatalf("错误信息应包含可行动指引 %q，实际：%v", want, errApply)
		}
	}
	if got := versionsOfDB(t, db); len(got) != 7 {
		t.Fatalf("Apply 不得擅动版本集合，实际 %v", got)
	}
}

// TestR2CheckCatchesStructureWithoutVersion INFRA-03：登记了版本但结构缺失，Check 不得假 OK。
//
// 吸收 ProbeD：构造"登记 1..6 但实际只执行了 0001"——修复前 Check 返回 OK=true，
// Apply 静默跳过全部脚本，transactions 根本没有 direction 列。
func TestR2CheckCatchesStructureWithoutVersion(t *testing.T) {
	engine, db := openReal(t, "r2d.db")
	applyManual(t, db, "0001_init.up.sql")
	for v := int64(2); v <= 6; v++ {
		if _, err := db.Exec(
			`INSERT OR REPLACE INTO schema_migrations(version, name, applied_at) VALUES (?, ?, 0)`,
			v, fmt.Sprintf("%04d_x.up.sql", v)); err != nil {
			t.Fatalf("登记版本 %d 失败：%v", v, err)
		}
	}

	// 核心：Check 必须指出"版本记录与真实结构不一致"，而不是 OK
	st, err := migrate.Check(engine)
	if err == nil {
		t.Fatalf("版本记录与结构不一致时 Check 应报错，实际返回 %+v（假 OK）", st)
	}
	if !errors.Is(err, migrate.ErrVersionMismatch) {
		t.Fatalf("错误应包装 ErrVersionMismatch，实际：%v", err)
	}
	if !strings.Contains(err.Error(), "结构不一致") {
		t.Fatalf("错误信息应说明是版本记录与结构不一致，实际：%v", err)
	}
	if hasColumn(t, db, "transactions", "direction") {
		t.Fatal("前置确认：该库确实没有 direction 列")
	}

	// 这个方向（版本记录在、结构缺失）与缺口相反：Apply 视角下这些版本"已应用"，
	// 它无法替运维决定是重跑脚本还是恢复备份——但绝不许静默通过。
	// `db migrate` 收尾会再跑一次 Check，把这个状态原样报给运维（见 main.go）。
	n, errApply := migrate.Apply(engine)
	if errApply != nil {
		t.Fatalf("Apply 不应在此场景报错（它只是无事可做）：%v", errApply)
	}
	if n != 0 {
		t.Fatalf("Apply 不应推进任何版本（版本记录都在），实际 %d", n)
	}
	if _, err := migrate.Check(engine); err == nil {
		t.Fatal("未修复前 Check 必须持续拒绝启动（不许假 OK）")
	}
	if got := versionsOfDB(t, db); fmt.Sprint(got) != "[1 2 3 4 5 6]" {
		t.Fatalf("Apply 不得擅动版本集合，实际 %v", got)
	}
	if !hasColumn(t, db, "transactions", "direction") {
		// 结构确实没被悄悄改动：这种状态需要运维恢复备份或重建，而不是被框架美化
		t.Log("结构仍缺失：需要恢复备份或人工重建，Check 会一直拒绝启动直到修复")
	}
}

// TestR2CheckCatchesDroppedIndex Check 的只读结构核验必须能发现结构漂移
func TestR2CheckCatchesDroppedIndex(t *testing.T) {
	engine, db := openReal(t, "r2idx.db")
	buildV6(t, engine)
	if _, err := db.Exec(`DROP INDEX idx_idem_unique`); err != nil {
		t.Fatalf("删除索引失败：%v", err)
	}
	_, err := migrate.Check(engine)
	if err == nil {
		t.Fatal("结构被破坏后 Check 应报错（版本号比对发现不了这种漂移）")
	}
	if !strings.Contains(err.Error(), "idx_idem_unique") {
		t.Fatalf("错误信息应指明缺失的产物，实际：%v", err)
	}
}

// TestR2NonIdempotentDDLHaveAssertions 移至 verify_test.go（内部包测试，
// 便于直接访问 versionAssertions，不必为守卫导出生产 API）

// containsNonIdempotentDDL 判断脚本是否含"重跑会失败"的 DDL
// （CREATE TABLE/INDEX 幂等靠 IF NOT EXISTS，但结构核验同样需要它——
// 登记了版本而表没建的库正是靠这些断言拦下的）
func containsNonIdempotentDDL(t *testing.T, filename string) bool {
	t.Helper()
	s := strings.ToUpper(scriptOf(t, filename))
	for _, kw := range []string{
		"CREATE TABLE", "ALTER TABLE", "CREATE INDEX", "CREATE UNIQUE INDEX", "DROP INDEX",
	} {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

// TestR2_0006BackfillDirection 0006 对历史转账的方向回填（吸收 ProbeE）
func TestR2_0006BackfillDirection(t *testing.T) {
	engine, db := openReal(t, "r2e.db")
	applyManual(t, db, "0001_init.up.sql", "0002_invites_and_member_unique.up.sql",
		"0003_transactions_creator_uid.up.sql", "0004_tx_direction.up.sql", "0005_idem_op.up.sql")

	now := int64(1700000000)
	if _, err := db.Exec(
		`INSERT INTO transactions(id, book_id, type, account_id, to_account_id, amount, tx_time, related_id, created_at, updated_at)
		 VALUES (1, 1, 3, 1, 2, 5000, ?, 2, ?, ?)`, now, now, now); err != nil {
		t.Fatalf("插入转出侧失败：%v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO transactions(id, book_id, type, account_id, to_account_id, amount, tx_time, related_id, created_at, updated_at)
		 VALUES (2, 1, 3, 2, 1, 5000, ?, 1, ?, ?)`, now, now, now); err != nil {
		t.Fatalf("插入转入侧失败：%v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO transactions(id, book_id, type, account_id, amount, tx_time, created_at, updated_at)
		 VALUES (3, 1, 2, 1, 100, ?, ?, ?)`, now, now, now); err != nil {
		t.Fatalf("插入单笔失败：%v", err)
	}

	n, err := migrate.Apply(engine)
	if err != nil {
		t.Fatalf("应用 0006 失败：%v", err)
	}
	if n != 1 {
		t.Fatalf("期望只应用 0006，实际 %d", n)
	}
	got := map[int64]int64{}
	rows, err := db.Query(`SELECT id, direction FROM transactions ORDER BY id`)
	if err != nil {
		t.Fatalf("查询 direction 失败：%v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, d int64
		if err := rows.Scan(&id, &d); err != nil {
			t.Fatalf("解析失败：%v", err)
		}
		got[id] = d
	}
	if got[1] != 1 || got[2] != 2 || got[3] != 0 {
		t.Fatalf("0006 回填结果错误：%v（期望 转出=1 转入=2 单笔=0）", got)
	}
}
