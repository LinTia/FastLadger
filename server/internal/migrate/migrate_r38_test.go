package migrate

// GPT-R38【P2】迁移框架不拒绝重复版本，也不验证已应用版本集合完整性 —— 回归测试（内部包）。
//
// 四类场景全部在【纯函数层】覆盖（本机无需 CGO 即可跑）：
//  1. 重复版本号      → collectMigrations 报错（TestR38DuplicateVersionsRejected）
//  2. 版本缺口        → diffVersions 检出缺失的中间版本（TestR38VersionGapDetected）
//  3. 未知版本        → diffVersions 检出库里有、本地没有的版本（TestR38UnknownVersionDetected）
//  4. 乱序文件名      → 按版本升序重排（TestR38OutOfOrderFilenamesSorted）
//
// 另外用替身驱动在 checkDB 层面验证"不得只依赖 MAX(version)"：
// MAX 与期望值相同、但中间缺一个版本时，checkDB 必须报错（TestR38CheckRejectsGapEvenWhenMaxMatches）。
//
// 需要真实 SQLite 的端到端版本见 migrate_r38_integration_test.go（Linux CI 执行）。

import (
	"database/sql/driver"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// TestR38DuplicateVersionsRejected 重复版本号必须报错，而不是"第一个生效、其余静默跳过"
func TestR38DuplicateVersionsRejected(t *testing.T) {
	cases := []struct {
		name  string
		files []string
	}{
		{"同名不同版本描述", []string{"0001_init.up.sql", "0001_init_v2.up.sql"}},
		{"两位补零写法不同", []string{"0001_a.up.sql", "001_b.up.sql"}},
		{"三个文件两个重复", []string{"0001_a.up.sql", "0002_b.up.sql", "0002_c.up.sql"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := collectMigrations(c.files)
			if err == nil {
				t.Fatalf("%v：重复版本号应报错（否则后一个脚本会被静默跳过）", c.files)
			}
			if !errors.Is(err, ErrDuplicateVersion) {
				t.Fatalf("错误应包装 ErrDuplicateVersion，实际 %v", err)
			}
		})
	}
}

// TestR38UniqueVersionsAccepted 正常（不重复）的文件列表必须照常通过
func TestR38UniqueVersionsAccepted(t *testing.T) {
	ms, err := collectMigrations([]string{"0002_b.up.sql", "0001_a.up.sql", "0010_c.up.sql"})
	if err != nil {
		t.Fatalf("合法文件列表不应报错：%v", err)
	}
	want := []int64{1, 2, 10}
	if got := versionsOf(ms); !reflect.DeepEqual(got, want) {
		t.Fatalf("版本解析错误：期望 %v，实际 %v", want, got)
	}
}

// TestR38OutOfOrderFilenamesSorted 乱序文件名要按版本升序重排（不按文件名字典序）
func TestR38OutOfOrderFilenamesSorted(t *testing.T) {
	// 字典序是 0003 < 0001? 不是：这里刻意用字典序与版本号不一致的组合，
	// 证明排序依据是【解析出的版本号】而不是文件名字符串
	ms, err := collectMigrations([]string{"0010_j.up.sql", "0002_b.up.sql", "0001_a.up.sql", "0003_c.up.sql"})
	if err != nil {
		t.Fatalf("乱序文件不应报错：%v", err)
	}
	wantVers := []int64{1, 2, 3, 10}
	if got := versionsOf(ms); !reflect.DeepEqual(got, wantVers) {
		t.Fatalf("应按版本升序：期望 %v，实际 %v", wantVers, got)
	}
	wantFiles := []string{"0001_a.up.sql", "0002_b.up.sql", "0003_c.up.sql", "0010_j.up.sql"}
	gotFiles := make([]string, 0, len(ms))
	for _, m := range ms {
		gotFiles = append(gotFiles, m.filename)
	}
	if !reflect.DeepEqual(gotFiles, wantFiles) {
		t.Fatalf("文件顺序错误：期望 %v，实际 %v", wantFiles, gotFiles)
	}
	if latestOf(ms) != 10 {
		t.Fatalf("latestOf 应为 10，实际 %d", latestOf(ms))
	}
}

// TestR38VersionGapDetected 缺失中间版本必须被检出（MAX 判断法发现不了）
func TestR38VersionGapDetected(t *testing.T) {
	cases := []struct {
		name     string
		expected []int64
		applied  []int64
		missing  []int64
		unknown  []int64
	}{
		{"缺中间版本", []int64{1, 2, 3}, []int64{1, 3}, []int64{2}, nil},
		{"缺首个版本", []int64{1, 2, 3}, []int64{2, 3}, []int64{1}, nil},
		{"缺末位版本", []int64{1, 2, 3}, []int64{1, 2}, []int64{3}, nil},
		{"缺多个", []int64{1, 2, 3, 4, 5}, []int64{1, 5}, []int64{2, 3, 4}, nil},
		{"未知版本", []int64{1, 2}, []int64{1, 2, 9}, nil, []int64{9}},
		{"同时缺口与未知", []int64{1, 2, 3}, []int64{1, 3, 7}, []int64{2}, []int64{7}},
		{"完全一致", []int64{1, 2, 3}, []int64{1, 2, 3}, nil, nil},
		{"库里为空", []int64{1, 2}, nil, []int64{1, 2}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			missing, unknown := diffVersions(c.expected, c.applied)
			if !reflect.DeepEqual(missing, c.missing) {
				t.Fatalf("%s：missing 期望 %v，实际 %v", c.name, c.missing, missing)
			}
			if !reflect.DeepEqual(unknown, c.unknown) {
				t.Fatalf("%s：unknown 期望 %v，实际 %v", c.name, c.unknown, unknown)
			}
		})
	}
}

// TestR38UnknownVersionDetected 已应用但本地不存在的版本必须被检出
func TestR38UnknownVersionDetected(t *testing.T) {
	missing, unknown := diffVersions([]int64{1, 2, 3}, []int64{1, 2, 3, 42})
	if len(missing) != 0 {
		t.Fatalf("不应有缺失版本，实际 %v", missing)
	}
	if !reflect.DeepEqual(unknown, []int64{42}) {
		t.Fatalf("应检出未知版本 [42]，实际 %v", unknown)
	}
}

// TestR38ParseVersionRejectsBadFilename 非法文件名不得被当成版本 0 蒙混过关
func TestR38ParseVersionRejectsBadFilename(t *testing.T) {
	for _, name := range []string{"init.up.sql", "v1_init.up.sql", "_init.up.sql", ".up.sql"} {
		if _, err := parseVersion(name); err == nil {
			t.Fatalf("%q 应被拒绝", name)
		}
	}
	if v, err := parseVersion("0007_x.up.sql"); err != nil || v != 7 {
		t.Fatalf("0007_x.up.sql 应解析为 7，实际 %d / err=%v", v, err)
	}
}

// appliedVersionsHandler 预设：表存在 + 已应用版本集合
func appliedVersionsHandler(applied []int64) func(string, []driver.Value) (*queryResult, error) {
	return func(query string, _ []driver.Value) (*queryResult, error) {
		q := strings.ToUpper(query)
		switch {
		case strings.Contains(q, "SQLITE_MASTER"):
			return &queryResult{cols: []string{"n"}, rows: []row{{int64(1)}}}, nil
		case strings.Contains(q, "SELECT VERSION FROM SCHEMA_MIGRATIONS"):
			rows := make([]row, 0, len(applied))
			for _, v := range applied {
				rows = append(rows, row{v})
			}
			return &queryResult{cols: []string{"version"}, rows: rows}, nil
		}
		return nil, nil
	}
}

// TestR38CheckRejectsGapEvenWhenMaxMatches 【关键】MAX 与期望值相同、中间缺版本时仍须报错。
//
// 这是"不得只依赖 MAX(version)"的直接证据：老实现算 current=MAX(version)=3，
// expected=3，于是 OK=true，而 v2 的 DDL 从来没执行过。
func TestR38CheckRejectsGapEvenWhenMaxMatches(t *testing.T) {
	db, drv := openSpy(t, appliedVersionsHandler([]int64{1, 3}))

	st, err := checkDB(db)
	if err == nil {
		t.Fatalf("中间缺版本时 Check 应报错，实际返回 %+v（MAX 判断法的盲区）", st)
	}
	if !errors.Is(err, ErrVersionMismatch) {
		t.Fatalf("错误应包装 ErrVersionMismatch，实际 %v", err)
	}
	if !strings.Contains(err.Error(), "缺失版本") {
		t.Fatalf("错误信息应指明缺失版本，实际 %q", err.Error())
	}
	// 顺带确认没有偷偷建表（Check 仍必须是只读的）
	if n := drv.countStatements("CREATE"); n != 0 {
		t.Fatalf("Check 不得建表，实际 %v", drv.statements())
	}
}

// TestR38CheckRejectsUnknownVersion 库里存在本地没有的版本时报错（降级运行/脚本被删）
func TestR38CheckRejectsUnknownVersion(t *testing.T) {
	db, _ := openSpy(t, appliedVersionsHandler([]int64{1, 2, 3, 4, 5, 6, 99}))
	_, err := checkDB(db)
	if err == nil {
		t.Fatal("存在未知版本时应报错")
	}
	if !errors.Is(err, ErrVersionMismatch) {
		t.Fatalf("错误应包装 ErrVersionMismatch，实际 %v", err)
	}
	if !strings.Contains(err.Error(), "未知版本") {
		t.Fatalf("错误信息应指明未知版本，实际 %q", err.Error())
	}
}

// TestR38CheckAcceptsCompleteSet 完整集合（与本地脚本一致）时必须通过
func TestR38CheckAcceptsCompleteSet(t *testing.T) {
	// 真实嵌入的迁移脚本版本集合：1..N 连续
	ms, err := listMigrations()
	if err != nil {
		t.Fatalf("读取内嵌迁移脚本失败：%v", err)
	}
	want := versionsOf(ms)
	if len(want) == 0 {
		t.Fatal("内嵌迁移脚本不应为空")
	}
	db, drv := openSpy(t, appliedVersionsHandler(want))
	st, err := checkDB(db)
	if err != nil {
		t.Fatalf("完整集合不应报错：%v", err)
	}
	if !st.OK || st.Current != latestOf(ms) || st.Expected != latestOf(ms) {
		t.Fatalf("状态异常：%+v", st)
	}
	if n := drv.countStatements("CREATE"); n != 0 {
		t.Fatalf("Check 不得建表，实际 %v", drv.statements())
	}
}

// TestR38EmbeddedMigrationsHaveNoDuplicateVersions 真实内嵌脚本本身不得有重复版本
func TestR38EmbeddedMigrationsHaveNoDuplicateVersions(t *testing.T) {
	ms, err := listMigrations()
	if err != nil {
		t.Fatalf("内嵌迁移脚本存在重复版本或非法的文件名：%v", err)
	}
	if len(ms) == 0 {
		t.Fatal("内嵌迁移脚本不应为空")
	}
	seen := make(map[int64]bool, len(ms))
	for _, m := range ms {
		if seen[m.version] {
			t.Fatalf("版本 %d 重复（%s）", m.version, m.filename)
		}
		seen[m.version] = true
	}
}
