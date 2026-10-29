// Package database 数据库初始化。
//
// 关键约束（来自《新手开发完整指南》§22.4，经 gpt/v1.4 真实运行验证）：
//  1. PRAGMA 必须走 DSN 逐连接设置，不能用 db.Exec("PRAGMA ...")——那只作用于当时
//     拿到的某一个连接，池中其他连接仍是默认值（R16）。
//  2. synchronous=FULL：断电不丢已提交事务（PRD §12 决策#5 与 §12.1），
//     对齐"无数据丢失"质量目标。不要退回 NORMAL。
//  3. 启动必须校验 PRAGMA 真的生效，而不是"执行了没报错"。
package database

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	_ "github.com/mattn/go-sqlite3" // SQLite 驱动（需 CGO；正式构建环境请准备 gcc）
	"xorm.io/xorm"
)

// Init 建立 xorm 引擎并校验 PRAGMA 生效。
// 写引擎连接数固定为 1：SQLite 同时只允许一个写连接，家庭负载极小，串行最稳妥。
func Init(dbPath string) (*xorm.Engine, error) {
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
	log.Printf("数据库已初始化：%s（WAL / foreign_keys=on / synchronous=FULL / MaxOpenConns=1）", dbPath)
	return engine, nil
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
