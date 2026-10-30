package migrate_test

import (
	"path/filepath"
	"testing"

	"fastledger/internal/database"
	"fastledger/internal/migrate"
)

// TestApplyMigrationsOnFreshDB 验证真实 SQLite 上迁移可执行、版本正确、且重复执行幂等。
//
// 同样需要真实 CGO，由 .github/workflows/ci.yml 在 ubuntu-latest 上运行，旨在让
// GPT-R25 指出的 SQLite 专项冒烟步骤产生真实测试证据（而非 [no test files]）。
func TestApplyMigrationsOnFreshDB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "migrate.db")

	engine, err := database.Init(dbPath)
	if err != nil {
		t.Fatalf("database.Init 失败：%v", err)
	}
	defer engine.Close()

	// 首次应用：应应用全部迁移（当前 0001/0002/0003 共 3 条）
	n1, err := migrate.Apply(engine)
	if err != nil {
		t.Fatalf("migrate.Apply 首次失败：%v", err)
	}
	if n1 < 1 {
		t.Fatalf("首次 Apply 应用数应为正，实际 %d", n1)
	}

	// 重复执行：应幂等，应用数为 0
	n2, err := migrate.Apply(engine)
	if err != nil {
		t.Fatalf("migrate.Apply 重复执行失败：%v", err)
	}
	if n2 != 0 {
		t.Fatalf("Apply 未幂等：重复执行又应用了 %d 条", n2)
	}

	// schema_migrations 版本与条目数一致
	sqlDB := engine.DB().DB
	var version int64
	if err := sqlDB.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version); err != nil {
		t.Fatalf("读取当前版本失败：%v", err)
	}
	var count int
	if err := sqlDB.QueryRow("SELECT COUNT(1) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatalf("统计迁移条数失败：%v", err)
	}
	if int64(count) != version {
		t.Fatalf("schema_migrations 条数(%d) 与最大版本(%d) 不一致", count, version)
	}
	if version != int64(n1) {
		t.Fatalf("最大版本(%d) 与首次应用数(%d) 不一致", version, n1)
	}

	// 0001 建立的实体表真实存在且可查，证明迁移脚本确实改了库
	if _, err := sqlDB.Exec("SELECT COUNT(1) FROM accounts"); err != nil {
		t.Fatalf("迁移后 accounts 表不存在或不可读：%v", err)
	}
	if _, err := sqlDB.Exec("SELECT COUNT(1) FROM users"); err != nil {
		t.Fatalf("迁移后 users 表不存在或不可读：%v", err)
	}
}
