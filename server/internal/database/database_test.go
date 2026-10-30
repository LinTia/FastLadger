package database_test

import (
	"path/filepath"
	"testing"

	"fastledger/internal/database"
)

// TestInitOpensRealSQLiteAndConfiguresPragmas 验证 mattn/go-sqlite3 真的打开了数据库，
// 且 database.Init 在启动校验阶段落地的三项 PRAGMA 真的生效。
//
// 该用例需真实 CGO（gcc）：在本地无 gcc 的机器上无法运行，由 .github/workflows/ci.yml
// 在 ubuntu-latest runner 上执行。它直接回应 GPT-R25——让 SQLite 专项冒烟步骤从
// `[no test files]` 变成真正跑起来的集成测试。
func TestInitOpensRealSQLiteAndConfiguresPragmas(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "smoke.db")

	engine, err := database.Init(dbPath)
	if err != nil {
		t.Fatalf("database.Init 失败（CGO/驱动是否可用？）：%v", err)
	}
	defer engine.Close()

	sqlDB := engine.DB().DB
	if sqlDB == nil {
		t.Fatal("engine.DB().DB 为 nil")
	}

	// foreign_keys 必须开启（断外键约束保护）
	var fk int
	if err := sqlDB.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("读取 foreign_keys 失败：%v", err)
	}
	if fk != 1 {
		t.Fatalf("foreign_keys 未生效，期望 1，实际 %d", fk)
	}

	// journal_mode 必须为 WAL（断电恢复与并发读）
	var jm string
	if err := sqlDB.QueryRow("PRAGMA journal_mode").Scan(&jm); err != nil {
		t.Fatalf("读取 journal_mode 失败：%v", err)
	}
	if jm != "wal" && jm != "WAL" {
		t.Fatalf("journal_mode 不是 WAL，实际 %q", jm)
	}

	// synchronous 必须为 FULL（2 / full）：断电不丢已提交事务
	var sync string
	if err := sqlDB.QueryRow("PRAGMA synchronous").Scan(&sync); err != nil {
		t.Fatalf("读取 synchronous 失败：%v", err)
	}
	if sync != "2" && sync != "full" && sync != "FULL" {
		t.Fatalf("synchronous 不是 FULL，实际 %q（断电可能丢失已提交事务）", sync)
	}

	// 真库可写可读：建一张临时表并回读，证明驱动不是 stub
	if _, err := sqlDB.Exec("CREATE TABLE smoke_probe(id INTEGER PRIMARY KEY, v TEXT)"); err != nil {
		t.Fatalf("建临时表失败：%v", err)
	}
	if _, err := sqlDB.Exec("INSERT INTO smoke_probe(id, v) VALUES (1, 'ok')"); err != nil {
		t.Fatalf("插入失败：%v", err)
	}
	var v string
	if err := sqlDB.QueryRow("SELECT v FROM smoke_probe WHERE id = 1").Scan(&v); err != nil {
		t.Fatalf("读取失败：%v", err)
	}
	if v != "ok" {
		t.Fatalf("回读值错误，期望 ok，实际 %q", v)
	}
}
