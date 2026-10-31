package migrate

// INFRA-02 的防漏登记守卫：含非幂等 DDL 的迁移版本必须登记结构断言。
//
// 内部包测试（package migrate），可直接访问 versionAssertions / listMigrations，
// 避免为了一条测试守卫导出生产 API。以后新增迁移若忘了登记断言，这条会红。

import (
	"strings"
	"testing"
)

// TestNonIdempotentDDLHaveAssertions 遍历内嵌迁移脚本，逐一核对断言登记
func TestNonIdempotentDDLHaveAssertions(t *testing.T) {
	ms, err := listMigrations()
	if err != nil {
		t.Fatalf("读取内嵌迁移列表失败：%v", err)
	}
	if len(ms) == 0 {
		t.Fatal("内嵌迁移列表不应为空")
	}
	for _, m := range ms {
		script := readEmbeddedScript(t, m.filename)
		if !hasDDLKeyword(script) {
			continue // 纯 DML（如 0006 的 UPDATE）无结构产物，不要求登记
		}
		if _, ok := versionAssertions[m.version]; !ok {
			t.Errorf("版本 %d（%s）含结构类 DDL，但 versionAssertions 未登记断言，"+
				"Apply 自愈与启动结构核验都会失去覆盖。脚本：\n%s", m.version, m.filename, script)
		}
	}
}

// TestAssertionsReferenceRealTables 断言表自身的健全性：表/列/索引名不得为空串
func TestAssertionsReferenceRealTables(t *testing.T) {
	for v, a := range versionAssertions {
		for _, tb := range a.tables {
			if strings.TrimSpace(tb) == "" {
				t.Errorf("版本 %d：tables 里有空表名", v)
			}
		}
		for table, cols := range a.columns {
			if strings.TrimSpace(table) == "" || len(cols) == 0 {
				t.Errorf("版本 %d：columns[%q] 非法", v, table)
			}
			for _, c := range cols {
				if strings.TrimSpace(c) == "" {
					t.Errorf("版本 %d：columns[%s] 里有空列名", v, table)
				}
			}
		}
		for idx, contains := range a.indexes {
			if strings.TrimSpace(idx) == "" {
				t.Errorf("版本 %d：indexes 里有空索引名", v)
			}
			_ = contains // 允许为空串（只要求存在）
		}
	}
}

// readEmbeddedScript 读内嵌脚本正文
func readEmbeddedScript(t *testing.T, filename string) string {
	t.Helper()
	b, err := migrationFS.ReadFile(migrationsDir + "/" + filename)
	if err != nil {
		t.Fatalf("读取内嵌脚本 %s 失败：%v", filename, err)
	}
	return string(b)
}

// hasDDLKeyword 判断脚本是否含结构类 DDL
func hasDDLKeyword(script string) bool {
	s := strings.ToUpper(script)
	for _, kw := range []string{
		"CREATE TABLE", "ALTER TABLE", "CREATE INDEX", "CREATE UNIQUE INDEX", "DROP INDEX",
	} {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

// TestContainsFold 纯函数自检（断言的索引定义匹配用它）
func TestContainsFold(t *testing.T) {
	cases := []struct {
		s, sub string
		want   bool
	}{
		{"CREATE UNIQUE INDEX ... WHERE deleted = 0", "where", true},
		{"CREATE UNIQUE INDEX ... WHERE deleted = 0", "WHERE", true},
		{"CREATE INDEX idx ON t(a)", "where", false},
		{"ON idempotency_records(book_id, uid, op, client_uuid)", "op", true},
		{"", "x", false},
		{"abc", "", true},
	}
	for _, c := range cases {
		if got := containsFold(c.s, c.sub); got != c.want {
			t.Fatalf("containsFold(%q,%q)=%v，期望 %v", c.s, c.sub, got, c.want)
		}
	}
}
