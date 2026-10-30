// Package version 账本版本号的唯一实现。
//
// 规范：版本递增的**唯一**可调用实现是 VersionRepo.Next，其余模块通过 VersionSource
// 接口使用；禁止再手写 nextBookVersion（C-R22 / GPT-R22）。
package version

import (
	"fmt"

	"xorm.io/xorm"
)

// Source 版本来源接口：创建、持仓、同步等统一通过它取版本号
type Source interface {
	Next(sess *xorm.Session, bookID int64) (int64, error)
}

// Repo 版本仓库
type Repo struct{}

// NewRepo 构造版本仓库
func NewRepo() *Repo { return &Repo{} }

// Next 在【调用方已有的写事务内】原子 +1 并返回新版本号。
// 必须与业务写入处于同一事务，否则会出现"版本号涨了但数据没写"的不一致。
func (Repo) Next(sess *xorm.Session, bookID int64) (int64, error) {
	res, err := sess.Exec("UPDATE book_versions SET version = version + 1 WHERE book_id = ?", bookID)
	if err != nil {
		return 0, fmt.Errorf("递增账本版本失败：%w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("读取版本影响行数失败：%w", err)
	}
	if n == 0 {
		return 0, fmt.Errorf("账本版本记录不存在（book_id=%d），请先创建账本", bookID)
	}
	var v int64
	if _, err := sess.SQL("SELECT version FROM book_versions WHERE book_id = ?", bookID).Get(&v); err != nil {
		return 0, fmt.Errorf("回读账本版本失败：%w", err)
	}
	return v, nil
}

// 编译期确保 Repo 实现 Source
var _ Source = (*Repo)(nil)
