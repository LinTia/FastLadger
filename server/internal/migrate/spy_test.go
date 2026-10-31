package migrate

// 最小 database/sql 驱动替身，用于在没有 CGO/gcc 的机器上验证迁移框架的
// **SQL 层面行为**（发了哪些语句、按返回结果走哪条分支）。
//
// 定位说明（避免误读为"用 mock 顶替真实验证"）：
//   - 它是【补充】而不是【替代】：migrate_r37_integration_test.go / migrate_r38_integration_test.go
//     里针对真实 SQLite 的断言照旧保留，由 Linux CI（自带 gcc）真实执行；
//   - 它验证的是"代码发了什么语句、如何解释返回结果"，这类断言本来就是
//     SQL 层面的，用替身驱动反而比真库更精确（真库没法断言"没有发 CREATE"）；
//   - 它不伪造业务结论：预设返回值模拟的是"空库 / 缺版本 / 多版本"等真实场景。

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
)

// spyDriverName 替身驱动注册名
const spyDriverName = "fastledger-spy"

// row 一行预设结果
type row []driver.Value

// queryResult 一次查询的预设结果
type queryResult struct {
	cols []string
	rows []row
}

// spyDriver 记录所有语句并按 handler 返回预设结果
type spyDriver struct {
	mu      sync.Mutex
	stmts   []string
	handler func(query string, args []driver.Value) (*queryResult, error)
}

// openSpy 打开一个替身数据库，同时返回驱动实例以便检查"发过哪些语句"。
//
// handler 按 SQL 文本返回预设结果；返回 (nil, nil) 表示"该查询无结果集"。
func openSpy(t *testing.T, handler func(query string, args []driver.Value) (*queryResult, error)) (*sql.DB, *spyDriver) {
	t.Helper()
	drv := &spyDriver{handler: handler}
	name := fmt.Sprintf("%s-%p", spyDriverName, drv)
	sql.Register(name, drv)
	db, err := sql.Open(name, "spy")
	if err != nil {
		t.Fatalf("打开替身数据库失败：%v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, drv
}

// statements 返回已执行的 SQL 列表（副本）
func (d *spyDriver) statements() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]string, len(d.stmts))
	copy(out, d.stmts)
	return out
}

// countStatements 统计包含某个片段（不区分大小写）的语句条数
func (d *spyDriver) countStatements(fragment string) int {
	n := 0
	for _, s := range d.statements() {
		if strings.Contains(strings.ToUpper(s), strings.ToUpper(fragment)) {
			n++
		}
	}
	return n
}

// schemaStub 手写的"最新结构"模型：替身库按它回答 PRAGMA / sqlite_master 查询。
//
// 注意：这份模型是**从 migrations/*.up.sql 人工抄录**的，与生产断言表
// （versionAssertions，见 verify.go）互相独立——替身不该"按断言的期望答题"，
// 否则断言测试就成了循环论证。真实结构的权威仍然是真库集成测试。
type schemaStub struct {
	// appliedVersions 已应用版本集合
	appliedVersions []int64
	// tables 存在的表名集合
	tables map[string]bool
	// columns 表名 → 列名集合
	columns map[string]map[string]bool
	// indexSQL 索引名 → 完整定义（不存在则查无此行）
	indexSQL map[string]string
}

// newLatestSchemaStub 构造一个"已迁移到 v6 最新结构"的库模型（依据 migrations/*.sql）
func newLatestSchemaStub(applied []int64) *schemaStub {
	txs := []string{
		"id", "book_id", "type", "account_id", "to_account_id", "amount", "category_id",
		"payment_channel", "merchant", "note", "tx_time", "related_id", "client_uuid",
		"sync_version", "deleted", "created_at", "updated_at", "deleted_at",
		// 0003 / 0004 增列
		"creator_uid", "source", "timezone_offset", "direction",
	}
	idem := []string{"id", "book_id", "uid", "client_uuid", "request_hash",
		"result_ids", "result_type", "result_version", "created_at",
		// 0005 增列
		"op",
	}
	s := &schemaStub{
		appliedVersions: applied,
		tables: map[string]bool{
			"users": true, "books": true, "book_members": true, "book_versions": true,
			"accounts": true, "categories": true, "transactions": true,
			"idempotency_records": true, "audit_logs": true, "invites": true,
			"instruments": true, "instrument_prices": true, "holdings": true,
			"budgets": true, "schema_migrations": true,
		},
		columns: map[string]map[string]bool{
			"transactions":        setOf(txs),
			"idempotency_records": setOf(idem),
		},
		indexSQL: map[string]string{
			// 0002：部分唯一索引（定义里必须带 WHERE）
			"idx_member_unique": "CREATE UNIQUE INDEX idx_member_unique ON book_members(book_id, uid) WHERE deleted = 0",
			"idx_invite_book":   "CREATE INDEX idx_invite_book ON invites(book_id, revoked)",
			// 0003
			"idx_tx_creator": "CREATE INDEX idx_tx_creator ON transactions(book_id, creator_uid, deleted, tx_time)",
			// 0005：扩为含 op 列
			"idx_idem_unique": "CREATE UNIQUE INDEX idx_idem_unique ON idempotency_records(book_id, uid, op, client_uuid)",
			// 0001
			"idx_users_email":      "CREATE UNIQUE INDEX idx_users_email ON users(email) WHERE email <> ''",
			"idx_books_owner":      "CREATE INDEX idx_books_owner ON books(owner_uid, deleted)",
			"idx_member_uid":       "CREATE INDEX idx_member_uid ON book_members(uid, deleted)",
			"idx_accounts_book":    "CREATE INDEX idx_accounts_book ON accounts(book_id, deleted)",
			"idx_categories_book":  "CREATE INDEX idx_categories_book ON categories(book_id, deleted)",
			"idx_tx_book_time":     "CREATE INDEX idx_tx_book_time ON transactions(book_id, tx_time)",
			"idx_tx_sync":          "CREATE INDEX idx_tx_sync ON transactions(book_id, sync_version)",
			"idx_tx_related":       "CREATE INDEX idx_tx_related ON transactions(book_id, related_id)",
			"idx_audit_book_at":    "CREATE INDEX idx_audit_book_at ON audit_logs(book_id, at)",
			"idx_audit_entity":     "CREATE INDEX idx_audit_entity ON audit_logs(entity_type, entity_id)",
			"idx_instr_book":       "CREATE INDEX idx_instr_book ON instruments(book_id, deleted)",
			"idx_instr_price":      "CREATE INDEX idx_instr_price ON instrument_prices(instrument_id, price_time)",
			"idx_holdings_account": "CREATE INDEX idx_holdings_account ON holdings(book_id, account_id, deleted)",
			"idx_budget_book":      "CREATE INDEX idx_budget_book ON budgets(book_id, deleted)",
		},
	}
	return s
}

func setOf(xs []string) map[string]bool {
	m := make(map[string]bool, len(xs))
	for _, x := range xs {
		m[x] = true
	}
	return m
}

// handler 返回替身查询处理器
func (s *schemaStub) handler() func(query string, args []driver.Value) (*queryResult, error) {
	return func(query string, args []driver.Value) (*queryResult, error) {
		q := strings.ToUpper(strings.TrimSpace(query))
		switch {
		case strings.Contains(q, "SQLITE_MASTER") && strings.Contains(q, "COUNT(1)"):
			// versionTableExists / tableExists
			name := ""
			if len(args) > 0 {
				name, _ = args[0].(string)
			}
			var n int64
			if s.tables[name] {
				n = 1
			}
			return &queryResult{cols: []string{"n"}, rows: []row{{n}}}, nil

		case strings.Contains(q, "SQLITE_MASTER") && strings.Contains(q, "SELECT SQL"):
			// indexDefinition：查无此行 = 索引不存在
			name := ""
			if len(args) > 0 {
				name, _ = args[0].(string)
			}
			def, ok := s.indexSQL[name]
			if !ok {
				return &queryResult{cols: []string{"sql"}}, nil // 空结果集 → sql.ErrNoRows
			}
			return &queryResult{cols: []string{"sql"}, rows: []row{{def}}}, nil

		case strings.HasPrefix(q, "PRAGMA TABLE_INFO("):
			// 表名要从【原始】查询里截取：替身的列名模型是小写，
			// 用大写化的 q 截出来会查不到（q 只用于前缀匹配）
			table := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(query), "PRAGMA table_info("), ")")
			cols := s.columns[table]
			if cols == nil {
				return &queryResult{cols: pragmaTableInfoCols}, nil
			}
			rows := make([]row, 0, len(cols))
			i := 0
			for name := range cols {
				rows = append(rows, row{i, name, "TEXT", 0, nil, 0})
				i++
			}
			return &queryResult{cols: pragmaTableInfoCols, rows: rows}, nil

		case strings.Contains(q, "SELECT VERSION FROM SCHEMA_MIGRATIONS"):
			rows := make([]row, 0, len(s.appliedVersions))
			for _, v := range s.appliedVersions {
				rows = append(rows, row{v})
			}
			return &queryResult{cols: []string{"version"}, rows: rows}, nil
		}
		return nil, nil
	}
}

// pragmaTableInfoCols PRAGMA table_info 的 6 列
var pragmaTableInfoCols = []string{"cid", "name", "type", "notnull", "dflt_value", "pk"}

func (d *spyDriver) Open(string) (driver.Conn, error) { return &spyConn{drv: d}, nil }

type spyConn struct{ drv *spyDriver }

func (c *spyConn) Prepare(query string) (driver.Stmt, error) {
	c.drv.mu.Lock()
	c.drv.stmts = append(c.drv.stmts, query)
	c.drv.mu.Unlock()
	return &spyStmt{conn: c, query: query}, nil
}
func (c *spyConn) Close() error              { return nil }
func (c *spyConn) Begin() (driver.Tx, error) { return nil, errors.New("替身驱动不支持事务") }

type spyStmt struct {
	conn  *spyConn
	query string
}

// NumInput 返回 -1：由替身自己决定参数个数检查，避免 database/sql 拒绝参数
func (s *spyStmt) NumInput() int { return -1 }
func (s *spyStmt) Close() error  { return nil }

func (s *spyStmt) Exec(args []driver.Value) (driver.Result, error) {
	if s.conn.drv.handler != nil {
		if _, err := s.conn.drv.handler(s.query, args); err != nil {
			return nil, err
		}
	}
	return driver.RowsAffected(0), nil
}

func (s *spyStmt) Query(args []driver.Value) (driver.Rows, error) {
	if s.conn.drv.handler == nil {
		return &spyRows{}, nil
	}
	res, err := s.conn.drv.handler(s.query, args)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return &spyRows{}, nil
	}
	return &spyRows{cols: res.cols, data: res.rows}, nil
}

type spyRows struct {
	cols []string
	data []row
	idx  int
}

func (r *spyRows) Columns() []string { return r.cols }
func (r *spyRows) Close() error      { return nil }

func (r *spyRows) Next(dest []driver.Value) error {
	if r.idx >= len(r.data) {
		return io.EOF
	}
	src := r.data[r.idx]
	r.idx++
	for i := range dest {
		if i < len(src) {
			dest[i] = src[i]
		}
	}
	return nil
}

// 确保编译期满足接口（防止替身实现漂移）
var (
	_ driver.Driver = (*spyDriver)(nil)
	_ driver.Conn   = (*spyConn)(nil)
	_ driver.Stmt   = (*spyStmt)(nil)
	_ driver.Rows   = (*spyRows)(nil)
)
