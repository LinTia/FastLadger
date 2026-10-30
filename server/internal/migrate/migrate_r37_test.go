package migrate

// GPT-R37【P3】migrate.Check 声称只读，实际会创建 schema_migrations —— 回归测试（内部包）。
//
// 两层验证：
//  1. 【本机可跑，无需 CGO】用替身驱动断言 Check 在"空库"上一条 DDL 都不发，
//     并且返回明确的 ErrNotMigrated；
//  2. 【需真实 SQLite，由 Linux CI 跑】空库上调用 Check 后 sqlite_master 中
//     不出现 schema_migrations（见 migrate_r37_integration_test.go）。

import (
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
)

// masterHandler 预设 sqlite_master 探测结果（表存在与否）
func masterHandler(exists bool) func(string, []driver.Value) (*queryResult, error) {
	var n int64
	if exists {
		n = 1
	}
	return func(query string, _ []driver.Value) (*queryResult, error) {
		if strings.Contains(strings.ToUpper(query), "SQLITE_MASTER") {
			return &queryResult{cols: []string{"n"}, rows: []row{{n}}}, nil
		}
		return nil, nil
	}
}

// TestR37CheckIssuesNoDDLOnEmptyDB 空库上 Check 必须一条 DDL 都不发（纯只读）
func TestR37CheckIssuesNoDDLOnEmptyDB(t *testing.T) {
	db, drv := openSpy(t, masterHandler(false))

	st, err := checkDB(db)
	if err == nil {
		t.Fatalf("空库上 Check 应报错，实际返回状态 %+v", st)
	}
	if !errors.Is(err, ErrNotMigrated) {
		t.Fatalf("应返回 ErrNotMigrated（明确的「未迁移」错误），实际 %v", err)
	}
	if st != nil {
		t.Fatalf("报错时不应同时返回状态，实际 %+v", st)
	}
	// 核心断言：任何写语句都不许出现（原实现正是在这里 CREATE TABLE 建表）
	for _, ddl := range []string{"CREATE", "ALTER", "DROP", "INSERT", "UPDATE", "DELETE"} {
		if n := drv.countStatements(ddl); n != 0 {
			t.Fatalf("Check 必须是纯只读，实际发出了 %d 条含 %s 的语句：%v",
				n, ddl, drv.statements())
		}
	}
	// 错误信息必须指引运维去执行 db migrate，而不是含糊地"检查失败"
	if !strings.Contains(err.Error(), "db migrate") {
		t.Fatalf("错误信息应指引运维执行 db migrate，实际 %q", err.Error())
	}
}

// TestR37CheckReadsVersionWhenTableExists 表已存在时 Check 正常读出版本（不能矫枉过正）
func TestR37CheckReadsVersionWhenTableExists(t *testing.T) {
	// 已应用版本集合取"与本地脚本完全一致"——GPT-R38 之后 Check 比对的是整个集合
	ms, err := listMigrations()
	if err != nil {
		t.Fatalf("读取内嵌迁移脚本失败：%v", err)
	}
	want := latestOf(ms)
	db, drv := openSpy(t, appliedVersionsHandler(versionsOf(ms)))
	st, err := checkDB(db)
	if err != nil {
		t.Fatalf("表存在且版本一致时 Check 不应报错：%v", err)
	}
	if !st.OK {
		t.Fatalf("版本一致时 OK 应为 true，实际 %+v", st)
	}
	if st.Current != want || st.Expected != want {
		t.Fatalf("版本读取错误：%+v（期望当前/均为 %d）", st, want)
	}
	if n := drv.countStatements("CREATE"); n != 0 {
		t.Fatalf("Check 不得建表，实际 %v", drv.statements())
	}
}

// TestR37VersionTableNameIsStable 版本表名不得被拼错（Check 探测与 Apply 建表必须同一张表）
func TestR37VersionTableNameIsStable(t *testing.T) {
	if versionTableName != "schema_migrations" {
		t.Fatalf("版本表名必须是 schema_migrations，实际 %q", versionTableName)
	}
}

// TestR37CheckRejectsNilHandle nil 句柄不得 panic
func TestR37CheckRejectsNilHandle(t *testing.T) {
	if _, err := Check(nil); err == nil {
		t.Fatal("engine 为 nil 时应报错而不是 panic")
	}
	if _, err := checkDB(nil); err == nil {
		t.Fatal("db 为 nil 时应报错而不是 panic")
	}
}
