package migrate

// 版本结构断言（INFRA-02 / INFRA-03）。
//
// 背景（交叉复核实测矩阵）：版本记录与真实结构脱节有两类，原框架都处理不了——
//
//  1. **版本记录缺口**：库里结构已在（迁移实际执行过），但 schema_migrations 的
//     版本记录丢了（误删、手工修库）。此时 Apply 重跑非幂等 DDL 必然失败
//     （ALTER TABLE ADD COLUMN → duplicate column），Check 又因为版本缺失拒绝启动
//     → 提示 db migrate → db migrate 又失败 → **死锁**。实测缺 3/4/5 全部死锁；
//     缺 1/2/6 只是侥幸（脚本恰好幂等）。
//
//  2. **登记了版本但结构缺失**：只按版本号记账、不看结构，导致"登记了 1..6
//     实际只执行了 0001"的库 Check 照样 OK 启动（INFRA-03 的假 OK）。
//
// 解法：给"含非幂等 DDL 的版本"登记**全部产物**（列 + 索引），只读探测：
//   - Apply 执行失败时，若该版本的断言通过 → 判定为"版本记录缺口"，补登记自愈；
//     断言不通过 → 照常报错（绝不能把真实的迁移失败美化成自愈）；
//   - Check 通过版本集合比对后，对已应用版本逐一跑断言，结构缺失即拒绝启动。

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
)

// versionAssert 某个版本应留下的全部结构产物
type versionAssert struct {
	// tables 必须存在的表（按名查询 sqlite_master）
	tables []string
	// columns 表名 → 必须存在的列（按 PRAGMA table_info 探测）
	columns map[string][]string
	// indexes 索引名 → 定义必须包含的片段（大小写不敏感；空串 = 只要求存在）
	//
	// 定义片段是必须的：同一索引名可能被后续版本重建（如 idx_idem_unique 在 0005
	// 被扩成含 op 列），"存在"不代表"是这一版留下的"。
	indexes map[string]string
}

// versionAssertions 版本 → 结构断言。
//
// 覆盖口径（V-1 整改后）：**机械全量**——每个版本创建的表、定义（或最后重定义）
// 的索引、ALTER 增加的列，全部登记，不是抽样。守卫测试
// TestAssertionCoversAllArtifacts 会从内嵌 .sql 重新解析出这份清单并逐项比对：
// 断言表漏登记、或新脚本加了产物忘登记，都会红。
//
// 维护约定：**新增迁移必须同步登记断言**。
var versionAssertions = map[int64]versionAssert{
	1: {
		tables: []string{
			"users", "books", "book_members", "book_versions", "accounts", "categories",
			"transactions", "idempotency_records", "audit_logs",
			"instruments", "instrument_prices", "holdings", "budgets",
			"schema_migrations",
		},
		indexes: map[string]string{
			"idx_users_email":      "",
			"idx_books_owner":      "",
			"idx_member_uid":       "",
			"idx_accounts_book":    "",
			"idx_categories_book":  "",
			"idx_tx_book_time":     "",
			"idx_tx_sync":          "",
			"idx_tx_related":       "",
			"idx_audit_book_at":    "",
			"idx_audit_entity":     "",
			"idx_instr_book":       "",
			"idx_instr_price":      "",
			"idx_holdings_account": "",
			"idx_budget_book":      "",
			// idx_member_unique（v2 重定义为部分索引）与 idx_idem_unique（v5 重定义含 op）
			// 归属【最后定义它们的版本】，见下方 2 / 5。
		},
	},
	2: {
		tables: []string{"invites"},
		indexes: map[string]string{
			// 0002 把 idx_member_unique 改成部分唯一索引——定义里必须带 WHERE
			"idx_member_unique": "where",
			"idx_invite_book":   "",
		},
	},
	3: {
		columns: map[string][]string{
			"transactions": {"creator_uid", "source", "timezone_offset"},
		},
		indexes: map[string]string{"idx_tx_creator": ""},
	},
	4: {
		columns: map[string][]string{
			"transactions": {"direction"},
		},
	},
	5: {
		columns: map[string][]string{
			"idempotency_records": {"op"},
		},
		// 0005 把唯一键扩为 (book_id, uid, op, client_uuid)：
		// 只断言"存在"不够——0001 的旧定义也在，必须确认定义里真的有 op
		indexes: map[string]string{"idx_idem_unique": "op"},
	},
	// 0006 只做 UPDATE 回填（幂等，可安全重跑），无结构产物，不登记。
}

// verifyVersion 只读检查某版本的产物是否真实存在
func verifyVersion(db *sql.DB, version int64) error {
	a, ok := versionAssertions[version]
	if !ok {
		return nil
	}
	for _, table := range a.tables {
		exists, err := tableExists(db, table)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("表 %s 不存在", table)
		}
	}
	// 固定遍历顺序，保证错误信息稳定可读
	tables := make([]string, 0, len(a.columns))
	for t := range a.columns {
		tables = append(tables, t)
	}
	sort.Strings(tables)
	for _, table := range tables {
		have, err := tableColumns(db, table)
		if err != nil {
			return err
		}
		for _, col := range a.columns[table] {
			if !have[col] {
				return fmt.Errorf("表 %s 缺列 %s", table, col)
			}
		}
	}
	names := make([]string, 0, len(a.indexes))
	for n := range a.indexes {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		want := a.indexes[name]
		def, exists, err := indexDefinition(db, name)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("索引 %s 不存在", name)
		}
		if want != "" && !containsFold(def, want) {
			return fmt.Errorf("索引 %s 的定义不含 %q（可能是旧版定义）：%s", name, want, def)
		}
	}
	return nil
}

// verifyAssertions 对全部已应用版本逐一跑断言（只读，Check 在版本集合比对通过后调用）
func verifyAssertions(db *sql.DB, versions []int64) error {
	for _, v := range versions {
		if _, has := versionAssertions[v]; !has {
			continue
		}
		if err := verifyVersion(db, v); err != nil {
			return fmt.Errorf("版本 %d 的版本记录与真实结构不一致：%v", v, err)
		}
	}
	return nil
}

// tableExists 表是否存在
func tableExists(db *sql.DB, table string) (bool, error) {
	var n int
	err := db.QueryRow(
		`SELECT COUNT(1) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("探测表 %s 失败：%w", table, err)
	}
	return n > 0, nil
}

// tableColumns 表的列名集合（PRAGMA 无法参数化，表名全部来自我们自己的断言表，无注入面）
func tableColumns(db *sql.DB, table string) (map[string]bool, error) {
	rows, err := db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return nil, fmt.Errorf("读取表 %s 结构失败：%w", table, err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return nil, fmt.Errorf("解析表 %s 结构失败：%w", table, err)
		}
		out[name] = true
	}
	return out, rows.Err()
}

// indexDefinition 读取索引定义（第二返回值 false = 索引不存在）
func indexDefinition(db *sql.DB, name string) (string, bool, error) {
	var def sql.NullString
	err := db.QueryRow(
		`SELECT sql FROM sqlite_master WHERE type = 'index' AND name = ?`, name).Scan(&def)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("读取索引 %s 定义失败：%w", name, err)
	}
	return def.String, true, nil
}

// containsFold 大小写不敏感的包含判断
func containsFold(s, sub string) bool {
	n := len(sub)
	if n == 0 {
		return true
	}
	for i := 0; i+n <= len(s); i++ {
		if equalFold(s[i:i+n], sub) {
			return true
		}
	}
	return false
}

// equalFold ASCII 大小写不敏感比较（索引定义片段只需 ASCII 判断）
func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if 'A' <= ca && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if 'A' <= cb && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
