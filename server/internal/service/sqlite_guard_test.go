package service

// 真实 SQLite（CGO）可用性守卫。
//
// 本机没有 gcc，CGO 不可用，mattn/go-sqlite3 会被编译成"开库即报错"的 stub
// （Binary was compiled with 'CGO_ENABLED=0'...）。依赖真实库的用例因此在本地跑不了，
// 由 .github/workflows/ci.yml 在 ubuntu-latest 上执行。
//
// 为什么显式 Skipf 而不是让它失败：
//   - 失败会被误读成"断言不成立"，而真实原因是"环境缺驱动"；
//   - Skip 会打印原因，任何人跑 `go test -v` 都能一眼看到哪些结论**尚未在本机验证过**。
// 断言一条都不会因为 Skip 而删除或放宽。

import (
	"path/filepath"
	"sync"
	"testing"

	"fastledger/internal/database"
)

var (
	sqliteProbeOnce sync.Once
	sqliteProbeErr  error
)

// requireSQLite 真实 SQLite 不可用则跳过当前用例并打印原因。
//
// 探测只做一次（进程内缓存结果），避免每个用例都去建一次临时库。
func requireSQLite(t *testing.T) {
	t.Helper()
	sqliteProbeOnce.Do(func() {
		dir := t.TempDir()
		engine, err := database.Init(filepath.Join(dir, "probe.db"))
		sqliteProbeErr = err
		if engine != nil {
			_ = engine.Close()
		}
	})
	if sqliteProbeErr != nil {
		t.Skipf("跳过：本机 CGO 不可用，无法运行真实 SQLite 用例（%v）。"+
			"该结论待 CI（ubuntu-latest，含 gcc）验证", sqliteProbeErr)
	}
}
