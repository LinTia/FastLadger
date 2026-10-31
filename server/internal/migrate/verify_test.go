package migrate

// INFRA-02 的防漏登记守卫：含非幂等 DDL 的迁移版本必须登记结构断言。
//
// 内部包测试（package migrate），可直接访问 versionAssertions / listMigrations，
// 避免为了一条测试守卫导出生产 API。以后新增迁移若忘了登记断言，这条会红。

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
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

// hasDDLKeyword 判断脚本是否含结构类 DDL。
//
// 先剥掉 `--` 注释行再匹配：注释掉的 DDL 不该要求登记断言
// （否则会要求一个永远不会被创建的表存在，verifyVersion 反而被假线索打红）。
func hasDDLKeyword(script string) bool {
	var b strings.Builder
	for _, line := range strings.Split(script, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	s := strings.ToUpper(b.String())
	for _, kw := range []string{
		"CREATE TABLE", "ALTER TABLE", "CREATE INDEX", "CREATE UNIQUE INDEX", "DROP INDEX",
		// V-2：重建范式的收尾与对象类 DDL 同样非幂等，漏了会让 INFRA-02 死锁
		// 以新形态复发（未来 0007 走"建新表→拷数据→改名→DROP TABLE t_old"）
		"DROP TABLE", "CREATE TRIGGER", "CREATE VIEW", "DROP TRIGGER",
	} {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

// TestGuardKeywordsCoverFutureDDL V-2：守卫关键字必须覆盖典型"未来 DDL"样本
func TestGuardKeywordsCoverFutureDDL(t *testing.T) {
	cases := []struct {
		name   string
		script string
		want   bool
	}{
		{"ALTER ADD COLUMN", "ALTER TABLE t ADD COLUMN c INTEGER;", true},
		{"CREATE TABLE（重建范式第一步）", "CREATE TABLE t_new(a INT);", true},
		{"DROP INDEX + CREATE UNIQUE INDEX", "DROP INDEX IF EXISTS i; CREATE UNIQUE INDEX IF NOT EXISTS i ON t(a);", true},
		{"CREATE INDEX", "CREATE INDEX IF NOT EXISTS i ON t(a);", true},
		{"DROP TABLE（重建范式收尾）", "DROP TABLE t_old;", true},
		{"CREATE TRIGGER", "CREATE TRIGGER trg AFTER INSERT ON t BEGIN SELECT 1; END;", true},
		{"CREATE VIEW", "CREATE VIEW v AS SELECT 1;", true},
		{"DROP TRIGGER", "DROP TRIGGER IF EXISTS trg;", true},
		{"纯 DML（0006 同款，不要求登记）", "UPDATE t SET a = 1 WHERE b = 2;", false},
		{"注释里的 CREATE（不算）", "-- CREATE TABLE xx", false},
	}
	for _, c := range cases {
		if got := hasDDLKeyword(c.script); got != c.want {
			t.Fatalf("hasDDLKeyword(%s)=%v，期望 %v", c.name, got, c.want)
		}
	}
}

// TestAssertionCoversAllArtifacts V-1：【机械全量覆盖守卫】断言表必须覆盖
// 内嵌脚本里的每一个结构产物。
//
// 方法（吸收自 crosscheck-infra 探针）：从 .sql 解析出每个版本创建的表、
// ALTER 增加的列、定义（或最后重定义）的索引，与 versionAssertions 逐项比对。
// 断言表漏登记、或将来新脚本加了产物忘登记，这条都会红——
// 避免"启动结构核验退化为抽样式"再次发生。
func TestAssertionCoversAllArtifacts(t *testing.T) {
	ms, err := listMigrations()
	if err != nil {
		t.Fatalf("读取内嵌迁移列表失败：%v", err)
	}
	if len(ms) == 0 {
		t.Fatal("内嵌迁移列表不应为空")
	}
	type artifacts struct {
		tables  []string
		columns map[string][]string // 表 → 列
		indexes []string            // 该版本定义（含重定义）的索引
	}
	perVersion := map[int64]*artifacts{}
	indexLastDef := map[string]int64{} // 索引名 → 最后定义它的版本

	for _, m := range ms {
		script := readEmbeddedScript(t, m.filename)
		a := &artifacts{columns: map[string][]string{}}
		perVersion[m.version] = a
		for _, sm := range createTableRe.FindAllStringSubmatch(script, -1) {
			a.tables = append(a.tables, strings.ToLower(sm[1]))
		}
		for _, sm := range createIndexRe.FindAllStringSubmatch(script, -1) {
			name := strings.ToLower(sm[1])
			a.indexes = append(a.indexes, name)
			indexLastDef[name] = m.version
		}
		for _, sm := range addColumnRe.FindAllStringSubmatch(script, -1) {
			table, col := strings.ToLower(sm[1]), strings.ToLower(sm[2])
			a.columns[table] = append(a.columns[table], col)
		}
	}

	var missing []string
	for _, m := range ms {
		a := perVersion[m.version]
		art, registered := versionAssertions[m.version]
		for _, tb := range a.tables {
			if !registered || !containsStr(art.tables, tb) {
				missing = append(missing, fmt.Sprintf("版本 %d 创建的表 %q 未登记断言", m.version, tb))
			}
		}
		for table, cols := range a.columns {
			have := art.columns[table]
			for _, c := range cols {
				if !registered || !containsStr(have, c) {
					missing = append(missing, fmt.Sprintf("版本 %d 给 %s 增加的列 %q 未登记断言", m.version, table, c))
				}
			}
		}
	}
	for name, v := range indexLastDef {
		art, registered := versionAssertions[v]
		if !registered {
			missing = append(missing, fmt.Sprintf("索引 %q 最后定义于版本 %d，但该版本未登记断言", name, v))
			continue
		}
		if _, ok := art.indexes[name]; !ok {
			missing = append(missing, fmt.Sprintf("索引 %q（最后定义于版本 %d）未登记断言", name, v))
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("versionAssertions 覆盖缺口 %d 处（启动结构核验会漏掉这些产物）：\n  %s",
			len(missing), strings.Join(missing, "\n  "))
	}
}

var (
	createTableRe = regexp.MustCompile(`(?i)CREATE\s+TABLE\s+IF\s+NOT\s+EXISTS\s+([A-Za-z_]\w*)`)
	createIndexRe = regexp.MustCompile(`(?i)CREATE\s+(?:UNIQUE\s+)?INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?([A-Za-z_]\w*)`)
	addColumnRe   = regexp.MustCompile(`(?i)ALTER\s+TABLE\s+([A-Za-z_]\w*)\s+ADD\s+COLUMN\s+([A-Za-z_]\w*)`)
)

// containsStr 线性查找（清单都很短，不值得引入 set 结构）
func containsStr(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

// goldenScriptSHA256 已发布迁移脚本的内容指纹（V-1 附带建议：堵住"已发布脚本被改"盲区）。
//
// 规则：
//   - 表里每个文件的内容必须与指纹一致（改一个字符都会红）；
//   - **发布新迁移时必须在此登记它的指纹**——这是强制而非可选：
//     migrations/ 下存在但指纹表里没有的文件同样会让本测试红（反向守卫）。
func TestPublishedMigrationsMatchGoldenHash(t *testing.T) {
	// 反向守卫：migrations/ 下存在、但指纹表里没有的文件 = 发布时忘了登记，
	// 该文件不受任何保护。登记是强制的，不是可选动作（crosscheck-infra 复核建议采纳）。
	entries, err := migrationFS.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("读取内嵌迁移目录失败：%v", err)
	}
	registered := make(map[string]bool, len(goldenScripts))
	for _, f := range goldenScripts {
		registered[f.name] = true
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".up.sql") {
			continue
		}
		if !registered[e.Name()] {
			t.Fatalf("迁移脚本 %s 未登记 golden 指纹（发布新迁移必须在 goldenScripts 里登记，"+
				"否则它不受\"内容不可变\"守卫保护）", e.Name())
		}
	}

	for _, f := range goldenScripts {
		b, err := migrationFS.ReadFile(migrationsDir + "/" + f.name)
		if err != nil {
			t.Fatalf("已发布脚本 %s 丢失：%v", f.name, err)
		}
		// 指纹对换行符归一化（CRLF→LF）：git 在 Windows 上可能以 CRLF 检出，
		// 指纹只应反映"内容被改"，不应被检出平台的换行差异打红
		norm := strings.ReplaceAll(string(b), "\r\n", "\n")
		sum := sha256.Sum256([]byte(norm))
		got := hex.EncodeToString(sum[:])
		if got != f.sha256 {
			t.Fatalf("已发布迁移脚本 %s 的内容被改动！\n  期望 sha256=%s\n  实际 sha256=%s\n"+
				"已发布的迁移不可修改（改了对已应用的环境不会重跑）；确需变更请新增版本。",
				f.name, f.sha256, got)
		}
	}
}

type goldenScript struct {
	name   string
	sha256 string
}

// goldenScripts 当前全部已发布迁移（新增迁移时在此追加一条指纹）
var goldenScripts = []goldenScript{
	{"0001_init.up.sql", "7ddf22ea50a7d4be37de17452bccd2035f4fec1b3f0e1a931f6c0cf7457a90ef"},
	{"0002_invites_and_member_unique.up.sql", "c0cd85e7432285c3b741107a60fb9e8378e9e802183e93be558eab21f138d02a"},
	{"0003_transactions_creator_uid.up.sql", "213a614d0dda82c9175d89dcd196f725b69884b62608c5c9bd5fd720db80e45d"},
	{"0004_tx_direction.up.sql", "f8772ded12d5ffa73af290c9270c0afc6b6fbfac2841632828fa118c04267251"},
	{"0005_idem_op.up.sql", "c5f9ce7df2f421e5156bd78c5034d67a06c442a3fa6b0b69e2a8e5d7870a66ab"},
	{"0006_tx_direction_backfill.up.sql", "dc2b5a4ffd2f32788500eb8dd0efe027ecb1be8db3ef7bb26795114f2f0ea30a"},
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
