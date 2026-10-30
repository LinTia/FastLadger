package instance

// GPT-R39【P2】instance.meta 原子重命名没有 fsync —— 回归测试。
//
// ⚠️ 验证边界（务必连同报告一起读，不要把它读成"断电已验证"）：
//   - **普通单元测试无法模拟断电**。这里能验证的是【调用序列】与【各步失败的处理】，
//     即"代码确实在 rename 前刷了文件、rename 后刷了目录，并且任何一步失败都会报错"。
//     真正的断电持久化只能在带电源故障注入的环境（或人工拔电）里验证，本仓库不具备。
//   - Windows 上【目录 fsync 不可用】（实测 FlushFileBuffers 对目录句柄返回
//     Access is denied），syncDir 在 Windows 上返回 nil；因此目录持久性这一环
//     在 Windows 上是**未被覆盖**的，只有 Linux（CI）上真实生效。
//   - 文件本体的 fsync 在两个平台上都真实执行。

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// withSyncHooks 临时替换同步钩子（测试结束自动还原）
func withSyncHooks(t *testing.T, fileHook func(*os.File) error, dirHook func(string) error) {
	t.Helper()
	origFile, origDir := syncFileHook, syncDirHook
	syncFileHook, syncDirHook = fileHook, dirHook
	t.Cleanup(func() { syncFileHook, syncDirHook = origFile, origDir })
}

// errInjected 故障注入用的哨兵错误
var errInjected = errors.New("注入的 fsync 故障")

// TestR39BumpSyncsFileBeforeRenameAndDirAfter 校验调用序列：
// 文件 fsync 发生在 rename 之前（此时目标文件还是旧内容），
// 目录 fsync 发生在 rename 之后（此时目标文件已是新内容）。
func TestR39BumpSyncsFileBeforeRenameAndDirAfter(t *testing.T) {
	dir := t.TempDir()
	before, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatalf("初始化失败：%v", err)
	}

	var fileSyncs, dirSyncs int
	var contentAtFileSync, contentAtDirSync string
	readContent := func() string {
		b, rerr := os.ReadFile(Path(dir))
		if rerr != nil {
			return "<read-error:" + rerr.Error() + ">"
		}
		return string(b)
	}

	withSyncHooks(t,
		func(f *os.File) error {
			fileSyncs++
			contentAtFileSync = readContent() // 此刻目标文件仍应是旧内容
			return f.Sync()
		},
		func(d string) error {
			dirSyncs++
			if d != dir {
				t.Errorf("目录 fsync 的对象应是数据目录 %q，实际 %q", dir, d)
			}
			contentAtDirSync = readContent() // 此刻目标文件应已是新内容
			return nil
		})

	after, err := Bump(dir)
	if err != nil {
		t.Fatalf("Bump 失败：%v", err)
	}
	if fileSyncs != 1 {
		t.Fatalf("应恰好调用 1 次文件 fsync，实际 %d 次", fileSyncs)
	}
	if dirSyncs != 1 {
		t.Fatalf("应恰好调用 1 次目录 fsync，实际 %d 次", dirSyncs)
	}
	// 序列证据 1：文件 fsync 时 rename 还没发生 → 目标仍是旧世代
	if got := epochOf(t, contentAtFileSync); got != before.RestoreEpoch {
		t.Fatalf("文件 fsync 应发生在 rename 之前（目标仍是旧世代 %d），实际世代 %d",
			before.RestoreEpoch, got)
	}
	// 序列证据 2：目录 fsync 时 rename 已完成 → 目标已是新世代
	if got := epochOf(t, contentAtDirSync); got != after.RestoreEpoch {
		t.Fatalf("目录 fsync 应发生在 rename 之后（目标已是新世代 %d），实际世代 %d",
			after.RestoreEpoch, got)
	}
}

// epochOf 解析磁盘上的 instance.meta，取出 restore_epoch
func epochOf(t *testing.T, raw string) int64 {
	t.Helper()
	m := &Meta{}
	if err := json.Unmarshal([]byte(raw), m); err != nil {
		t.Fatalf("解析磁盘内容失败：%v（内容=%q）", err, raw)
	}
	return m.RestoreEpoch
}

// TestR39BumpFailsWhenFileSyncFails 文件 fsync 失败：Bump 必须报错，且旧世代原封不动
func TestR39BumpFailsWhenFileSyncFails(t *testing.T) {
	dir := t.TempDir()
	before, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatalf("初始化失败：%v", err)
	}
	rawBefore, err := os.ReadFile(Path(dir))
	if err != nil {
		t.Fatalf("读取初始文件失败：%v", err)
	}

	var dirSyncCalled bool
	withSyncHooks(t,
		func(*os.File) error { return errInjected },
		func(string) error { dirSyncCalled = true; return nil })

	if _, err := Bump(dir); err == nil {
		t.Fatal("文件 fsync 失败时 Bump 必须报错")
	} else if !errors.Is(err, errInjected) {
		t.Fatalf("错误应包装注入的故障：%v", err)
	}
	// 关键：绝不能"同步失败但把新世代留在盘上"
	rawAfter, err := os.ReadFile(Path(dir))
	if err != nil {
		t.Fatalf("读取文件失败：%v", err)
	}
	if string(rawAfter) != string(rawBefore) {
		t.Fatalf("文件 fsync 失败后不得改变磁盘内容：\n前=%s\n后=%s", rawBefore, rawAfter)
	}
	if _, err := Read(dir); err != nil {
		t.Fatalf("磁盘内容应保持可读的旧世代：%v", err)
	}
	m, err := Read(dir)
	if err != nil || m.RestoreEpoch != before.RestoreEpoch {
		t.Fatalf("世代不得在 fsync 失败后推进：%+v / err=%v", m, err)
	}
	if dirSyncCalled {
		t.Fatal("文件 fsync 失败时不应继续做 rename + 目录 fsync")
	}
	// 临时文件不得残留
	if _, statErr := os.Stat(Path(dir) + tmpSuffix); !os.IsNotExist(statErr) {
		t.Fatalf("失败的写入应清理临时文件，实际仍存在（stat=%v）", statErr)
	}
}

// TestR39BumpFailsWhenDirSyncFails 目录 fsync 失败：Bump 必须报错（不假装成功）
//
// 注意：这一步失败时 rename 已经生效，新世代确实在盘上——测试如实断言这一点，
// 不因为"看起来更好"而把它写成"内容应当回滚"。运维据此知道"世代已变但持久化未确认"。
func TestR39BumpFailsWhenDirSyncFails(t *testing.T) {
	dir := t.TempDir()
	before, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatalf("初始化失败：%v", err)
	}
	withSyncHooks(t,
		func(f *os.File) error { return f.Sync() },
		func(string) error { return errInjected })

	_, err = Bump(dir)
	if err == nil {
		t.Fatal("目录 fsync 失败时 Bump 必须报错（否则调用方以为持久化已完成）")
	}
	if !errors.Is(err, errInjected) {
		t.Fatalf("错误应包装注入的故障：%v", err)
	}
	m, rerr := Read(dir)
	if rerr != nil {
		t.Fatalf("rename 已生效，磁盘应可读：%v", rerr)
	}
	if m.RestoreEpoch != before.RestoreEpoch+1 {
		t.Fatalf("rename 已生效，世代应已推进到 %d，实际 %d", before.RestoreEpoch+1, m.RestoreEpoch)
	}
}

// TestR39BumpFailsWhenTmpCannotBeCreated 临时文件无法创建时报错（不留下半个文件）
func TestR39BumpFailsWhenTmpCannotBeCreated(t *testing.T) {
	dir := t.TempDir()
	before, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatalf("初始化失败：%v", err)
	}
	// 用一个同名目录堵住 instance.meta.tmp，让 OpenFile 必然失败
	tmp := Path(dir) + tmpSuffix
	if err := os.Mkdir(tmp, 0o755); err != nil {
		t.Fatalf("准备阻塞用的目录失败：%v", err)
	}
	if _, err := Bump(dir); err == nil {
		t.Fatal("临时文件无法创建时 Bump 必须报错")
	}
	m, rerr := Read(dir)
	if rerr != nil || m.RestoreEpoch != before.RestoreEpoch {
		t.Fatalf("写入失败不得推进世代：%+v / err=%v", m, rerr)
	}
	if st, serr := os.Stat(tmp); serr != nil || !st.IsDir() {
		// 阻塞用的目录是测试夹具本身，不应被误删
		t.Fatalf("阻塞用目录不应被删除：stat=%v err=%v", st, serr)
	}
}

// TestR39NoTmpFileLeftAfterSuccess 成功后不得残留 .tmp 文件
func TestR39NoTmpFileLeftAfterSuccess(t *testing.T) {
	dir := t.TempDir()
	if _, err := LoadOrCreate(dir); err != nil {
		t.Fatalf("初始化失败：%v", err)
	}
	if _, err := Bump(dir); err != nil {
		t.Fatalf("Bump 失败：%v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读取目录失败：%v", err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == tmpSuffix {
			t.Fatalf("成功后不应残留临时文件：%s", e.Name())
		}
	}
}

// TestR39SyncDirDoesNotError syncDir 在本平台都不应抛错
// （Windows 上它是显式 no-op——目录 fsync 在该平台不可用，见本文件顶部说明）
func TestR39SyncDirDoesNotError(t *testing.T) {
	if err := syncDir(t.TempDir()); err != nil {
		t.Fatalf("syncDir 不应报错：%v", err)
	}
	if runtime.GOOS == "windows" {
		t.Logf("Windows：目录 fsync 不可用（FlushFileBuffers 对目录句柄 Access is denied），" +
			"syncDir 为显式 no-op，目录持久化这一环在本平台未被覆盖")
	}
}
