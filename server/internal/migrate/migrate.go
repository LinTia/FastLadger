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

// ErrDuplicateVersion 迁移脚本版本号重复（GPT-R38）。
//
// 两个脚本抢同一个版本号时，后一个会被"已应用集合"判定为已处理而静默跳过——
// 它的 SQL 永远不执行，而 schema_migrations 看起来一切正常。这类故障只在
// 业务上真正用到那张表时才爆发，必须在启动期拒。
var ErrDuplicateVersion = errors.New("迁移脚本版本号重复")

// ErrVersionMismatch 已应用版本集合与本地预期集合不一致（GPT-R38）。
//
// 只比对 MAX(version) 是不够的：
//   - 缺失中间版本（缺口）时 MAX 可能仍然等于期望值，看起来"已迁移"；
//   - 库里存在本地脚本没有的版本（降级运行、脚本被误删）时 MAX 也看不出来。
var ErrVersionMismatch = errors.New("已应用版本集合与本地迁移脚本不一致")

// VersionMismatchError 版本集合不一致的【结构化】错误（INFRA-02）。
//
// 带上缺失/未知明细，是为了让调用方（CLI/启动）能给出**可行动**的处置指引，
// 而不是让运维对着一串版本号猜。errors.Is(err, ErrVersionMismatch) 仍然成立。
type VersionMismatchError struct {
	// Missing 本地脚本有、库里没有（版本缺口）
	Missing []int64
	// Unknown 库里有、本地脚本没有（降级运行 / 脚本被删）
	Unknown []int64
	// Local / Applied 两边的完整版本集合（诊断用）
	Local   []int64
	Applied []int64
}

// Error 实现 error
func (e *VersionMismatchError) Error() string {
	return fmt.Sprintf("%s：缺失版本=%v，未知版本=%v（本地脚本=%v，已应用=%v）",
		ErrVersionMismatch, e.Missing, e.Unknown, e.Local, e.Applied)
}

// Is 使 errors.Is(err, ErrVersionMismatch) 对本类型成立
func (e *VersionMismatchError) Is(target error) bool { return target == ErrVersionMismatch }

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
	ms, err := listMigrations()
	if err != nil {
		return nil, err
	}
	expected := latestOf(ms)
	exists, err := versionTableExists(db)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("%w：缺少 %s 表，请先执行 `fastledger db migrate`（期望 v%d）",
			ErrNotMigrated, versionTableName, expected)
	}
	applied, err := appliedVersions(db)
	if err != nil {
		return nil, err
	}
	// GPT-R38：对照【整个版本集合】，而不是只看 MAX(version)
	missing, unknown := diffVersions(versionsOf(ms), applied)
	if len(missing) > 0 || len(unknown) > 0 {
		return nil, &VersionMismatchError{
			Missing: missing, Unknown: unknown,
			Local: versionsOf(ms), Applied: applied,
		}
	}
	// INFRA-03：版本号对上了还不够——"登记了版本但结构缺失"（只登记未执行、
	// 结构被手工改动等）的库同样会假 OK 启动。对登记了断言的版本逐一只读核验。
	if err := verifyAssertions(db, applied); err != nil {
		return nil, fmt.Errorf("%w：%v（请恢复数据库备份或重建，勿手工增删 %s 的版本记录）",
			ErrVersionMismatch, err, versionTableName)
	}
	current := int64(0)
	for _, v := range applied {
		if v > current {
			current = v
		}
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

// insertVersionSQL 登记版本（正常路径与"版本记录缺口自愈"共用同一句 SQL）
const insertVersionSQL = `INSERT INTO ` + versionTableName +
	`(version, name, applied_at) VALUES (?, ?, strftime('%s','now'))`

// Apply 应用所有未执行的迁移，返回本次推进的版本个数。CLI 调用。
//
// INFRA-02：对"版本记录缺口"自愈——某版本执行失败、但其全部结构产物经只读断言
// 确认已在（即迁移实际执行过、只是版本记录丢了），则补登记后继续推进；
// 断言不通过则原样报错（绝不把真实的迁移失败美化成自愈）。
func Apply(engine *xorm.Engine) (int, error) {
	if engine == nil {
		return 0, fmt.Errorf("migrate.Apply：engine 缺失")
	}
	db := engine.DB().DB
	if err := ensureVersionTable(db); err != nil {
		return 0, err
	}
	ms, err := listMigrations()
	if err != nil {
		return 0, err
	}
	applied, err := appliedVersions(db)
	if err != nil {
		return 0, err
	}
	// INFRA-02：库里有本地没有的版本 = 降级运行。此时缺的版本补不进去
	// （Check 永远不会 OK），静默返回 n=0 只会让运维误以为迁移"成功"了。
	if _, unknown := diffVersions(versionsOf(ms), applied); len(unknown) > 0 {
		return 0, fmt.Errorf("%w：库中存在本地没有的版本 %v——数据库可能比程序新（降级运行）。"+
			"`db migrate` 无法修复此状态：请升级程序版本，或恢复与程序匹配的数据库备份",
			ErrVersionMismatch, unknown)
	}
	done := make(map[int64]bool, len(applied))
	for _, v := range applied {
		done[v] = true
	}
	count := 0
	for _, m := range ms {
		if done[m.version] {
			continue
		}
		content, err := migrationFS.ReadFile(path.Join(migrationsDir, m.filename))
		if err != nil {
			return count, err
		}
		if err := execInTx(db, m.version, m.filename, string(content)); err != nil {
			// INFRA-02：区分"版本记录缺口"与"真实的迁移失败"。
			// 只有断言确认产物齐全才允许自愈；否则缺口与坏脚本就会变得无法区分。
			if _, has := versionAssertions[m.version]; has {
				if verr := verifyVersion(db, m.version); verr == nil {
					if rerr := recordVersion(db, m.version, m.filename); rerr != nil {
						return count, fmt.Errorf("迁移 %s 疑似版本记录缺口（结构已存在），但补登记失败：%w",
							m.filename, rerr)
					}
					// 结构早已在库、这里只是把账补上，计入"本次推进的版本数"
					count++
					continue
				}
			}
			return count, err
		}
		count++
	}
	return count, nil
}

// recordVersion 把版本写入 schema_migrations（不含迁移脚本本身——那只用于缺口自愈）
func recordVersion(db *sql.DB, version int64, name string) error {
	if _, err := db.Exec(insertVersionSQL, version, name); err != nil {
		return fmt.Errorf("登记版本 %d（%s）失败：%w", version, name, err)
	}
	return nil
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
	if _, err := tx.Exec(insertVersionSQL, version, name); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("迁移 %s 记录版本失败（已回滚）：%w", name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("迁移 %s 提交失败：%w", name, err)
	}
	return nil
}

// listMigrations 读取内嵌目录里的迁移脚本（按版本升序）
func listMigrations() ([]migration, error) {
	entries, err := migrationFS.ReadDir(migrationsDir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".up.sql") {
			continue
		}
		names = append(names, e.Name())
	}
	return collectMigrations(names)
}

// collectMigrations 由文件名列表构建【按版本升序】的迁移列表。
//
// GPT-R38：版本号重复一律报错（见 ErrDuplicateVersion）。
// 拆成以文件名为输入的纯函数，是为了让"重复版本 / 乱序文件名"这两类场景
// 能在单元测试里直接构造——内嵌 FS 的内容在测试里是无法临时改写的。
func collectMigrations(names []string) ([]migration, error) {
	ms := make([]migration, 0, len(names))
	seen := make(map[int64]string, len(names))
	for _, name := range names {
		ver, err := parseVersion(name)
		if err != nil {
			return nil, err
		}
		if prev, dup := seen[ver]; dup {
			return nil, fmt.Errorf("%w：版本 %d 同时被 %s 与 %s 占用（脚本只有一个会生效）",
				ErrDuplicateVersion, ver, prev, name)
		}
		seen[ver] = name
		ms = append(ms, migration{version: ver, filename: name})
	}
	sort.Slice(ms, func(i, j int) bool { return ms[i].version < ms[j].version })
	return ms, nil
}

// versionsOf 取迁移列表的版本号切片（升序）
func versionsOf(ms []migration) []int64 {
	out := make([]int64, 0, len(ms))
	for _, m := range ms {
		out = append(out, m.version)
	}
	return out
}

// appliedVersions 读出【全部】已应用版本（升序），而不是只取 MAX(version)
func appliedVersions(db *sql.DB) ([]int64, error) {
	rows, err := db.Query(`SELECT version FROM ` + versionTableName + ` ORDER BY version`)
	if err != nil {
		return nil, fmt.Errorf("读取已应用版本失败：%w", err)
	}
	defer rows.Close()
	out := make([]int64, 0)
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("解析已应用版本失败：%w", err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历已应用版本失败：%w", err)
	}
	return out, nil
}

// diffVersions 对照预期集合与已应用集合（GPT-R38 的核心判定）。
//
//   - missing：本地脚本有、库里没有 —— 版本缺口（含"缺失中间版本"）；
//   - unknown：库里有、本地脚本没有 —— 降级运行、脚本被误删或库被串了。
//
// 纯函数：不碰数据库，因此可以在单元测试里直接覆盖四类场景。
func diffVersions(expected, applied []int64) (missing, unknown []int64) {
	expSet := make(map[int64]bool, len(expected))
	for _, v := range expected {
		expSet[v] = true
	}
	appSet := make(map[int64]bool, len(applied))
	for _, v := range applied {
		appSet[v] = true
	}
	for _, v := range expected {
		if !appSet[v] {
			missing = append(missing, v)
		}
	}
	for _, v := range applied {
		if !expSet[v] {
			unknown = append(unknown, v)
		}
	}
	sortInt64(missing)
	sortInt64(unknown)
	return missing, unknown
}

// sortInt64 升序排序（避免为主流程引入泛型/额外依赖）
func sortInt64(xs []int64) {
	sort.Slice(xs, func(i, j int) bool { return xs[i] < xs[j] })
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

// latestOf 取迁移列表中的最大版本（列表必须已按版本升序；空列表为 0）
func latestOf(ms []migration) int64 {
	if len(ms) == 0 {
		return 0
	}
	return ms[len(ms)-1].version
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
