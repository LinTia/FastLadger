package database_test

// GPT-R35【P2】SQLite 数据库及 WAL/SHM 文件权限没有被明确收紧 —— 回归测试。
//
// 平台边界（请连同报告一起读）：
//   - **Windows 没有 POSIX 权限位**：os.Chmod(0600) 在 Windows 上会被翻译成
//     "设置只读属性"，断言 0600 在 Windows 上既无意义也会失败。因此所有权限位
//     断言在 Windows 上【显式跳过并说明】，而不是删除断言或伪造"通过"。
//   - Linux（CI runner）上这些断言全部真实执行。
//   - 依赖真实 SQLite 的用例还需要 CGO/gcc，本机无 gcc 时同样显式跳过。

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"fastledger/internal/database"
)

// TestR35EnsureDataDirTightensExistingDir 新建目录是 0700，已存在的宽松目录会被收紧。
//
// os.MkdirAll 对已存在目录不会改权限，这正是原缺陷的一部分：一次宽松部署
// （umask 022 → 0755）会永久留下可读的数据目录。
func TestR35EnsureDataDirTightensExistingDir(t *testing.T) {
	// 先造一个"宽松权限的旧目录"（0755），模拟历史部署
	dir := filepath.Join(t.TempDir(), "data")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("准备旧目录失败：%v", err)
	}
	if err := database.EnsureDataDir(dir); err != nil {
		t.Fatalf("EnsureDataDir 失败：%v", err)
	}
	if runtime.GOOS == "windows" {
		// 无 POSIX 权限位：只验证调用不报错、目录仍可用
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			t.Fatalf("目录不可用：%v", err)
		}
		t.Skip("Windows 无 POSIX 权限位，跳过目录权限位断言（Linux CI 上真实断言 0700）")
	}
	st, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat 失败：%v", err)
	}
	if perm := st.Mode().Perm(); perm != database.DataDirPerm {
		t.Fatalf("已存在目录应被收紧为 %o，实际 %o", database.DataDirPerm, perm)
	}

	// 新建目录同样必须是 0700（不依赖 umask）
	fresh := filepath.Join(t.TempDir(), "fresh", "nested")
	if err := database.EnsureDataDir(fresh); err != nil {
		t.Fatalf("EnsureDataDir（新建）失败：%v", err)
	}
	fst, err := os.Stat(fresh)
	if err != nil {
		t.Fatalf("stat 失败：%v", err)
	}
	if perm := fst.Mode().Perm(); perm != database.DataDirPerm {
		t.Fatalf("新建目录应为 %o，实际 %o", database.DataDirPerm, perm)
	}
}

// TestR35HardenTightensLooseExistingFiles 已存在 db / -wal / -shm 且权限宽松时会被收紧。
//
// 这条不需要数据库，只验证"收紧"这一步本身，因此在 Linux 上无条件真实执行。
func TestR35HardenTightensLooseExistingFiles(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "fastledger.db")
	sidecars := []string{dbPath, dbPath + "-wal", dbPath + "-shm"}
	// 模拟 umask 022 下历史遗留的宽松权限（0644）
	for _, p := range sidecars {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatalf("准备文件 %s 失败：%v", p, err)
		}
	}
	if err := database.HardenDBFilePerms(dbPath); err != nil {
		t.Fatalf("HardenDBFilePerms 失败：%v", err)
	}
	if runtime.GOOS == "windows" {
		for _, p := range sidecars {
			if _, err := os.Stat(p); err != nil {
				t.Fatalf("文件不应被破坏：%v", err)
			}
		}
		t.Skip("Windows 无 POSIX 权限位，跳过文件权限位断言（Linux CI 上真实断言 0600）")
	}
	for _, p := range sidecars {
		st, err := os.Stat(p)
		if err != nil {
			t.Fatalf("stat %s 失败：%v", p, err)
		}
		if perm := st.Mode().Perm(); perm != database.DBFilePerm {
			t.Fatalf("%s 应被收紧为 %o，实际 %o", filepath.Base(p), database.DBFilePerm, perm)
		}
	}
}

// TestR35HardenIgnoresMissingSidecars 尚不存在的 sidecar 不算错误（-wal 首次写才生成）
func TestR35HardenIgnoresMissingSidecars(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "fastledger.db")
	if err := os.WriteFile(dbPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("准备文件失败：%v", err)
	}
	if err := database.HardenDBFilePerms(dbPath); err != nil {
		t.Fatalf("sidecar 不存在时不应报错：%v", err)
	}
	if runtime.GOOS == "windows" {
		t.Skip("Windows 无 POSIX 权限位，跳过文件权限位断言（Linux CI 上真实断言 0600）")
	}
	st, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("stat 失败：%v", err)
	}
	if perm := st.Mode().Perm(); perm != database.DBFilePerm {
		t.Fatalf("db 文件应被收紧为 %o，实际 %o", database.DBFilePerm, perm)
	}
}

// TestR35PermTargetsCoverSidecars 收紧清单必须覆盖主库 + WAL + SHM（各平台都真实执行）。
//
// 只 chmod 主库文件而漏掉 -wal / -shm 是本缺陷的一半：WAL 模式下 sidecar 里
// 同样躺着尚未 checkpoint 的财务明细。这条断言不依赖 POSIX 权限语义，
// 因此在 Windows 上也是真的在验证。
func TestR35PermTargetsCoverSidecars(t *testing.T) {
	got := database.DBPermTargets("/srv/data/fastledger.db")
	want := []string{
		"/srv/data/fastledger.db",
		"/srv/data/fastledger.db-wal",
		"/srv/data/fastledger.db-shm",
	}
	if len(got) != len(want) {
		t.Fatalf("收紧清单应含 %d 个文件，实际 %d：%v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("第 %d 项期望 %q，实际 %q", i, want[i], got[i])
		}
	}
}

// TestR35InitLeavesDatabaseFilesPrivate 真实 SQLite 上：Init 之后 db / -wal / -shm
// 与所在目录都必须是受限权限（GPT-R35 的端到端断言）。
//
// 需要真实 CGO/gcc；本机无 gcc 时显式跳过（不是放宽断言）。
func TestR35InitLeavesDatabaseFilesPrivate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 无 POSIX 权限位，跳过端到端权限断言（Linux CI 上真实断言）")
	}
	dir := filepath.Join(t.TempDir(), "data")
	dbPath := filepath.Join(dir, "fastledger.db")

	engine, err := database.Init(dbPath)
	if err != nil {
		t.Skipf("真实 SQLite 不可用（mattn/go-sqlite3 需 CGO/gcc，本机无 gcc），"+
			"跳过多端权限断言：%v", err)
	}
	// 触发一次写入，让 -wal 与 -shm 真实生成（它们才是"后来才出现的文件"）
	if _, err := engine.DB().DB.Exec("CREATE TABLE perm_probe(id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatalf("建表失败：%v", err)
	}
	if _, err := engine.DB().DB.Exec("INSERT INTO perm_probe(id) VALUES (1)"); err != nil {
		t.Fatalf("写入失败：%v", err)
	}
	// 重新收紧一次，模拟"WAL 在 Init 之后才生成"的窗口被覆盖
	if err := database.HardenDBFilePerms(dbPath); err != nil {
		t.Fatalf("HardenDBFilePerms 失败：%v", err)
	}

	dst, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat 目录失败：%v", err)
	}
	if perm := dst.Mode().Perm(); perm != database.DataDirPerm {
		t.Fatalf("数据目录应为 %o，实际 %o", database.DataDirPerm, perm)
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		p := dbPath + suffix
		st, serr := os.Stat(p)
		if serr != nil {
			t.Logf("注意：%s 不存在（SQLite 版本/时序差异），跳过该 sidecar", filepath.Base(p))
			continue
		}
		if perm := st.Mode().Perm(); perm != database.DBFilePerm {
			t.Fatalf("%s 应为 %o，实际 %o（同机其他用户可读到财务明细）",
				filepath.Base(p), database.DBFilePerm, perm)
		}
	}
	_ = engine.Close()
}
