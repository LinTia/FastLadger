// Package database 数据库初始化。
//
// 关键约束（来自《新手开发完整指南》§22.4，经 gpt/v1.4 真实运行验证）：
//  1. PRAGMA 必须走 DSN 逐连接设置，不能用 db.Exec("PRAGMA ...")——那只作用于当时
//     拿到的某一个连接，池中其他连接仍是默认值（R16）。
//  2. synchronous=FULL：断电不丢已提交事务（PRD §12 决策#5 与 §12.1），
//     对齐"无数据丢失"质量目标。不要退回 NORMAL。
//  3. 启动必须校验 PRAGMA 真的生效，而不是"执行了没报错"。
//  4. 数据库文件与目录权限必须【显式收紧】（数据目录 0700，db/WAL/SHM 0600），
//     不能依赖部署机的 umask——同机其他用户一旦能读到库文件，就等于绕过 HTTP
//     鉴权直接看到财务明细与口令哈希（GPT-R35）。
package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	_ "github.com/mattn/go-sqlite3" // SQLite 驱动（需 CGO；正式构建环境请准备 gcc）
	"xorm.io/xorm"
)

const (
	// DataDirPerm 数据目录权限：仅属主可进入（GPT-R35）
	DataDirPerm os.FileMode = 0o700
	// DBFilePerm 数据库文件与 WAL/SHM sidecar 权限：仅属主可读写（GPT-R35）
	DBFilePerm os.FileMode = 0o600

	// walSuffix / shmSuffix WAL 模式下的两个 sidecar 文件后缀（同样含财务明细）
	walSuffix = "-wal"
	shmSuffix = "-shm"
)

// Init 建立 xorm 引擎并校验 PRAGMA 生效。
// 写引擎连接数固定为 1：SQLite 同时只允许一个写连接，家庭负载极小，串行最稳妥。
//
// 打开前先创建并收紧数据目录权限，打开后立即收紧 db/WAL/SHM 文件权限（GPT-R35）。
func Init(dbPath string) (*xorm.Engine, error) {
	// 数据库所在目录：不存在则创建，已存在则收紧（不依赖 umask）
	if dir := filepath.Dir(dbPath); dir != "" && dir != "." {
		if err := EnsureDataDir(dir); err != nil {
			return nil, err
		}
	}

	dsn := fmt.Sprintf(
		"file:%s?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on&_synchronous=FULL",
		dbPath,
	)

	engine, err := xorm.NewEngine("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("创建数据库引擎失败：%w", err)
	}
	engine.SetMaxOpenConns(1)
	engine.SetMaxIdleConns(1)

	// engine.DB() 返回 *core.DB（内嵌 *sql.DB），取底层句柄才能传给标准库函数
	sqlDB := engine.DB().DB
	sqlDB.SetMaxOpenConns(1)

	if err := checkPragmas(sqlDB); err != nil {
		_ = engine.Close()
		return nil, err
	}
	// 库文件与 sidecar 在此刻已存在（-shm 在首次读时创建，-wal 在首次写时创建），
	// 收紧权限；后续新生成的 sidecar 由 ApplyStrictUmask 兜住
	if err := HardenDBFilePerms(dbPath); err != nil {
		_ = engine.Close()
		return nil, err
	}
	log.Printf("数据库已初始化：%s（WAL / foreign_keys=on / synchronous=FULL / MaxOpenConns=1）", dbPath)
	return engine, nil
}

// EnsureDataDir 创建数据目录并【收紧】权限为 0700。
//
// os.MkdirAll 对已存在的目录不会改权限，所以对已存在的旧目录要显式 Chmod——
// 否则一次宽松部署（umask 0022 → 0755）会永久留下可读的数据目录。
func EnsureDataDir(dir string) error {
	if dir == "" || dir == "." {
		return fmt.Errorf("数据目录不能为空：%q", dir)
	}
	if err := os.MkdirAll(dir, DataDirPerm); err != nil {
		return fmt.Errorf("创建数据目录 %s 失败：%w", dir, err)
	}
	if !filePermsEnforced() {
		return nil
	}
	if err := os.Chmod(dir, DataDirPerm); err != nil {
		return fmt.Errorf("收紧数据目录 %s 权限失败：%w", dir, err)
	}
	return nil
}

// DBPermTargets 需要收紧权限的文件清单：数据库本体 + WAL/SHM 两个 sidecar。
//
// 单独暴露出来是因为"漏掉 sidecar"正是本缺陷的一半——WAL 模式下 -wal / -shm
// 里同样有尚未 checkpoint 的财务明细，只 chmod 主库文件等于没收紧。
func DBPermTargets(dbPath string) []string {
	return []string{dbPath, dbPath + walSuffix, dbPath + shmSuffix}
}

// HardenDBFilePerms 收紧数据库文件与 WAL/SHM sidecar 权限为 0600。
//
// 三个文件都不存在时（例如 :memory: 或未来得及落盘）不算错误，逐个"存在即收紧"；
// 这是为了让【老实例升级】也能被收紧，而不只是新装实例。
func HardenDBFilePerms(dbPath string) error {
	if dbPath == "" {
		return fmt.Errorf("数据库路径不能为空")
	}
	if !filePermsEnforced() {
		return nil
	}
	for _, p := range DBPermTargets(dbPath) {
		st, err := os.Stat(p)
		if err != nil {
			if os.IsNotExist(err) {
				continue // sidecar 尚未生成，Init 之后由 umask 兜住
			}
			return fmt.Errorf("检查数据库文件 %s 失败：%w", p, err)
		}
		if st.IsDir() {
			continue
		}
		if err := os.Chmod(p, DBFilePerm); err != nil {
			return fmt.Errorf("收紧数据库文件 %s 权限失败：%w", p, err)
		}
	}
	return nil
}

// filePermsEnforced 本平台是否执行权限收紧。
//
// Windows 没有 POSIX 权限位：Go 的 os.Chmod 只在 mode&0222==0 时才设置只读属性，
// 0600/0700 这类"属主可写"的模式在 Windows 上是【空操作】——st.Mode().Perm()
// 恒为 0666/0777，权限位断言必然失败。因此非 POSIX 平台不执行 Chmod，
// 权限保证由部署形态（容器/单用户目录/ACL）承担。
//
// 保留这个开关而不是无条件 Chmod，还有一个原因：将来若有人把模式误改成 0400，
// Windows 上会真的把库文件设成只读、直接搞坏 SQLite；开关把这类误伤挡在外面。
func filePermsEnforced() bool {
	return runtime.GOOS != "windows"
}

// checkPragmas 启动校验：确认 PRAGMA 真的生效
func checkPragmas(db *sql.DB) error {
	var fk int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		return fmt.Errorf("读取 foreign_keys 失败：%w", err)
	}
	if fk != 1 {
		return fmt.Errorf("foreign_keys 未生效（fk=%d）", fk)
	}

	var jm string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&jm); err != nil {
		return fmt.Errorf("读取 journal_mode 失败：%w", err)
	}
	if !strings.EqualFold(jm, "wal") {
		return fmt.Errorf("journal_mode 不是 WAL（=%s）", jm)
	}

	var sync string
	if err := db.QueryRow("PRAGMA synchronous").Scan(&sync); err != nil {
		return fmt.Errorf("读取 synchronous 失败：%w", err)
	}
	// FULL=2 / NORMAL=1 / OFF=0
	if sync != "2" && !strings.EqualFold(sync, "full") {
		return fmt.Errorf("synchronous 不是 FULL（=%s）：断电可能丢失已提交事务", sync)
	}
	return nil
}
