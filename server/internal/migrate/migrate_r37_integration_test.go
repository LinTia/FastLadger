package migrate_test

// GPT-R37【P3】的真实 SQLite 端到端断言。
//
// 需要 CGO/gcc（mattn/go-sqlite3）：本机无 gcc 时【显式跳过并说明】，
// 断言本身保持不变，由 .github/workflows/ci.yml 在 ubuntu-latest 上真实执行。

import (
	"errors"
	"path/filepath"
	"testing"

	"fastledger/internal/database"
	"fastledger/internal/migrate"
)

// TestR37CheckDoesNotCreateTableOnEmptyDB 空库上 Check 不得创建 schema_migrations。
//
// 原缺陷：Check 先 CREATE TABLE IF NOT EXISTS 再判版本——服务启动就静默改了表，
// 与本包"服务启动只检查、绝不静默改表"的约定冲突，且会留下半初始化的库。
func TestR37CheckDoesNotCreateTableOnEmptyDB(t *testing.T) {
	engine, err := database.Init(filepath.Join(t.TempDir(), "r37.db"))
	if err != nil {
		t.Skipf("真实 SQLite 不可用（mattn/go-sqlite3 需 CGO/gcc，本机无 gcc），"+
			"跳过端到端断言：%v", err)
	}
	defer engine.Close()

	sqlDB := engine.DB().DB
	// 前置：确认这是一个真正的空库（否则"没有建表"的断言没有意义）
	var before int
	if err := sqlDB.QueryRow("SELECT COUNT(1) FROM sqlite_master").Scan(&before); err != nil {
		t.Fatalf("读取 sqlite_master 失败：%v", err)
	}
	if before != 0 {
		t.Fatalf("前置条件失败：新库不应有任何对象，实际 %d 个", before)
	}

	st, err := migrate.Check(engine)
	if err == nil {
		t.Fatalf("空库上 Check 应报错，实际返回 %+v", st)
	}
	if !errors.Is(err, migrate.ErrNotMigrated) {
		t.Fatalf("应返回 ErrNotMigrated，实际 %v", err)
	}

	// 核心断言：Check 之后仍然【一个表都没有】
	var after int
	if err := sqlDB.QueryRow("SELECT COUNT(1) FROM sqlite_master").Scan(&after); err != nil {
		t.Fatalf("读取 sqlite_master 失败：%v", err)
	}
	if after != 0 {
		t.Fatalf("Check 必须是纯只读：空库上调用后出现了 %d 个数据库对象", after)
	}
	var named int
	if err := sqlDB.QueryRow(
		"SELECT COUNT(1) FROM sqlite_master WHERE name = 'schema_migrations'").Scan(&named); err != nil {
		t.Fatalf("读取 sqlite_master 失败：%v", err)
	}
	if named != 0 {
		t.Fatal("Check 不得创建 schema_migrations 表")
	}
}

// TestR37ApplyThenCheckIsOK 迁移之后 Check 必须可用（不能把只读改造做成"永远报错"）
func TestR37ApplyThenCheckIsOK(t *testing.T) {
	engine, err := database.Init(filepath.Join(t.TempDir(), "r37ok.db"))
	if err != nil {
		t.Skipf("真实 SQLite 不可用（需 CGO/gcc），跳过端到端断言：%v", err)
	}
	defer engine.Close()

	if _, err := migrate.Apply(engine); err != nil {
		t.Fatalf("migrate.Apply 失败：%v", err)
	}
	st, err := migrate.Check(engine)
	if err != nil {
		t.Fatalf("迁移后 Check 不应报错：%v", err)
	}
	if !st.OK {
		t.Fatalf("迁移后版本应一致，实际 %+v", st)
	}
	if st.Current != st.Expected || st.Current <= 0 {
		t.Fatalf("版本状态异常：%+v", st)
	}
}
