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
