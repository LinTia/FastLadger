package migrate_test

// GPT-R38【P2】的真实 SQLite 端到端断言（版本集合完整性）。
//
// 需要 CGO/gcc（mattn/go-sqlite3）：本机无 gcc 时显式跳过，Linux CI 上真实执行。

import (
	"errors"
	"path/filepath"
	"testing"

	"fastledger/internal/database"
	"fastledger/internal/migrate"
	"xorm.io/xorm"
)

// openMigratedDB 应用全部迁移后返回引擎（CGO 不可用时跳过）
func openMigratedDB(t *testing.T, name string) *xorm.Engine {
	t.Helper()
	engine, err := database.Init(filepath.Join(t.TempDir(), name))
	if err != nil {
		t.Skipf("真实 SQLite 不可用（mattn/go-sqlite3 需 CGO/gcc，本机无 gcc），"+
			"跳过端到端断言：%v", err)
	}
	t.Cleanup(func() { _ = engine.Close() })
	if _, err := migrate.Apply(engine); err != nil {
		t.Fatalf("migrate.Apply 失败：%v", err)
	}
	return engine
}

// TestR38CheckDetectsMissingMiddleVersion 真实库上删掉一个中间版本记录，Check 必须报错。
//
// 只比对 MAX(version) 的老实现在这里会给出 OK=true——删掉的是中间的 v（不是最大值）。
func TestR38CheckDetectsMissingMiddleVersion(t *testing.T) {
	engine := openMigratedDB(t, "r38gap.db")
	sqlDB := engine.DB().DB

	var maxV, minV int64
	if err := sqlDB.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&maxV); err != nil {
		t.Fatalf("读取最大版本失败：%v", err)
	}
	if err := sqlDB.QueryRow("SELECT MIN(version) FROM schema_migrations").Scan(&minV); err != nil {
		t.Fatalf("读取最小版本失败：%v", err)
	}
	if maxV-minV < 2 {
		t.Skipf("迁移脚本不足 3 个（min=%d max=%d），构造不出中间版本缺口", minV, maxV)
	}
	// 挖掉一个中间版本（既不是最小也不是最大）
	if _, err := sqlDB.Exec("DELETE FROM schema_migrations WHERE version = ?", minV+1); err != nil {
		t.Fatalf("删除中间版本记录失败：%v", err)
	}

	st, err := migrate.Check(engine)
	if err == nil {
		t.Fatalf("缺失中间版本时 Check 应报错，实际返回 %+v（MAX 判断法发现不了缺口）", st)
	}
	if !errors.Is(err, migrate.ErrVersionMismatch) {
		t.Fatalf("错误应包装 ErrVersionMismatch，实际 %v", err)
	}
}

// TestR38CheckDetectsUnknownVersion 真实库上塞入一个本地没有的版本，Check 必须报错
func TestR38CheckDetectsUnknownVersion(t *testing.T) {
	engine := openMigratedDB(t, "r38unknown.db")
	sqlDB := engine.DB().DB

	if _, err := sqlDB.Exec(
		`INSERT INTO schema_migrations(version, name, applied_at) VALUES (9999, 'ghost.up.sql', 0)`); err != nil {
		t.Fatalf("插入未知版本记录失败：%v", err)
	}
	_, err := migrate.Check(engine)
	if err == nil {
		t.Fatal("存在本地不存在的已应用版本时 Check 应报错")
	}
	if !errors.Is(err, migrate.ErrVersionMismatch) {
		t.Fatalf("错误应包装 ErrVersionMismatch，实际 %v", err)
	}
}

// TestR38ApplyThenCheckIsConsistent 正常迁移后 Check 必须 OK（不能矫枉过正）
func TestR38ApplyThenCheckIsConsistent(t *testing.T) {
	engine := openMigratedDB(t, "r38ok.db")
	st, err := migrate.Check(engine)
	if err != nil {
		t.Fatalf("完整迁移后 Check 不应报错：%v", err)
	}
	if !st.OK {
		t.Fatalf("完整迁移后 OK 应为 true，实际 %+v", st)
	}
	// 重复 Apply 幂等，且 Check 依然 OK
	n, err := migrate.Apply(engine)
	if err != nil || n != 0 {
		t.Fatalf("重复 Apply 应幂等（n=0），实际 n=%d err=%v", n, err)
	}
	if st, err := migrate.Check(engine); err != nil || !st.OK {
		t.Fatalf("重复 Apply 后 Check 应 OK，实际 %+v / err=%v", st, err)
	}
}
