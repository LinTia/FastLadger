// Package migrate 版本化数据库迁移。
//
// 约定（对齐《落地方案》§4.7）：
//  1. schema_migrations 记录已应用的版本；
//  2. 迁移默认【仅 CLI 手动执行】（fastledger db migrate）；服务启动只做版本检查，
//     **绝不静默自动改表**；
//  3. 每个脚本在**事务内**执行，失败即整体回滚；
//  4. 老库升级**不得删除旧回执**（idempotency_records），否则离线重试会产生重复账；
//  5. SQLite 早期不支持 DROP/RENAME COLUMN，需改列时走"建新表→拷数据→改名"重建范式。
package migrate

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"

	"xorm.io/xorm"
)

//go:embed migrations/*.up.sql
var migrationFS embed.FS

const migrationsDir = "migrations"

// versionTableName 版本表名（Check 只读探测、Apply 才建）
const versionTableName = "schema_migrations"

// ErrNotMigrated 数据库尚未迁移：schema_migrations 表不存在。
//
// GPT-R37：Check 是**纯只读**检查，绝不代替 `fastledger db migrate` 建表。
// 原先 Check 会先 CREATE TABLE IF NOT EXISTS 再判版本——服务启动就静默改了表，
// 与本包约定第 2 条（服务启动绝不静默改表）直接冲突；而且"建表 + 版本不匹配退出"
// 会留下一个半初始化的库，让运维误以为"迁移已经跑过一部分"。
var ErrNotMigrated = errors.New("数据库尚未迁移")

// Status 迁移状态：Current 为库当前版本，Expected 为程序期望版本
type Status struct {
	Current  int64
	Expected int64
	OK       bool
}

type migration struct {
	version  int64
	filename string
}

// Check 只检查版本，**纯只读，不建表不改表**。服务启动时调用。
//
// GPT-R37：表不存在直接报 ErrNotMigrated，由 `fastledger db migrate` 负责创建。
func Check(engine *xorm.Engine) (*Status, error) {
	if engine == nil {
		return nil, fmt.Errorf("migrate.Check：engine 缺失")
	}
	return checkDB(engine.DB().DB)
}

// checkDB 版本检查的核心逻辑（便于在测试中用替身驱动验证"只读"这一性质）
func checkDB(db *sql.DB) (*Status, error) {
	if db == nil {
		return nil, fmt.Errorf("migrate.Check：数据库句柄缺失")
	}
	expected, err := latestVersion()
	if err != nil {
		return nil, err
	}
	exists, err := versionTableExists(db)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("%w：缺少 %s 表，请先执行 `fastledger db migrate`（期望 v%d）",
			ErrNotMigrated, versionTableName, expected)
	}
	current, err := currentVersion(db)
	if err != nil {
		return nil, err
	}
	return &Status{Current: current, Expected: expected, OK: current == expected}, nil
}

// versionTableExists 只读探测版本表是否存在（绝不创建）。
//
// 走 sqlite_master 查询而不是"试着 SELECT 一把、报错就当没表"：
// 后者会把权限错误、磁盘错误也误判成"未迁移"。
func versionTableExists(db *sql.DB) (bool, error) {
	var n int
	err := db.QueryRow(
		`SELECT COUNT(1) FROM sqlite_master WHERE type = 'table' AND name = ?`,
		versionTableName).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("读取 sqlite_master 失败：%w", err)
	}
	return n > 0, nil
}

// Apply 应用所有未执行的迁移，返回本次应用的个数。CLI 调用。
func Apply(engine *xorm.Engine) (int, error) {
	db := engine.DB().DB
	if err := ensureVersionTable(db); err != nil {
		return 0, err
	}
	ms, err := listMigrations()
	if err != nil {
		return 0, err
	}
	applied := 0
	for _, m := range ms {
		done, err := isApplied(db, m.version)
		if err != nil {
			return applied, err
		}
		if done {
			continue
		}
		content, err := migrationFS.ReadFile(path.Join(migrationsDir, m.filename))
		if err != nil {
			return applied, err
		}
		if err := execInTx(db, m.version, m.filename, string(content)); err != nil {
			return applied, err
		}
		applied++
	}
	return applied, nil
}

// execInTx 单个迁移在事务内执行，任一语句失败即整体回滚
func execInTx(db *sql.DB, version int64, name, script string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败：%w", err)
	}
	if _, err := tx.Exec(script); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("迁移 %s 执行失败（已回滚）：%w", name, err)
	}
	if _, err := tx.Exec(
		`INSERT INTO schema_migrations(version, name, applied_at) VALUES (?, ?, strftime('%s','now'))`,
		version, name); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("迁移 %s 记录版本失败（已回滚）：%w", name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("迁移 %s 提交失败：%w", name, err)
	}
	return nil
}

func listMigrations() ([]migration, error) {
	entries, err := migrationFS.ReadDir(migrationsDir)
	if err != nil {
		return nil, err
	}
	var ms []migration
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".up.sql") {
			continue
		}
		ver, err := parseVersion(e.Name())
		if err != nil {
			return nil, err
		}
		ms = append(ms, migration{version: ver, filename: e.Name()})
	}
	sort.Slice(ms, func(i, j int) bool { return ms[i].version < ms[j].version })
	return ms, nil
}

// parseVersion 文件名形如 0001_init.up.sql，取前导数字为版本号
func parseVersion(filename string) (int64, error) {
	base := strings.TrimSuffix(filename, ".up.sql")
	parts := strings.SplitN(base, "_", 2)
	n, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("迁移文件名须以版本号开头（如 0001_init.up.sql）：%s", filename)
	}
	return n, nil
}

func latestVersion() (int64, error) {
	ms, err := listMigrations()
	if err != nil {
		return 0, err
	}
	if len(ms) == 0 {
		return 0, nil
	}
	return ms[len(ms)-1].version, nil
}

// ensureVersionTable 建版本表。
//
// GPT-R37：**只允许出现在 Apply 路径**（`fastledger db migrate`）。
// Check 不得调用它——服务启动必须保持纯只读。
func ensureVersionTable(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		name VARCHAR(128) NOT NULL,
		applied_at INTEGER NOT NULL
	)`)
	return err
}

func currentVersion(db *sql.DB) (int64, error) {
	var v int64
	if err := db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&v); err != nil {
		return 0, err
	}
	return v, nil
}

func isApplied(db *sql.DB, version int64) (bool, error) {
	var n int
	if err := db.QueryRow(`SELECT COUNT(1) FROM schema_migrations WHERE version = ?`, version).Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}
