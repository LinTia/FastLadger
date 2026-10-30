package service

// 账户（accounts）CRUD。
//
// 约束：
//  1. `book_id` 只来自可信路径参数，并在服务端校验「账户归属本账本」，不信任请求体；
//  2. 账户/分类属于账本级共享配置，写操作需要 owner 或 admin（枚举字典 §6 角色权限）；
//  3. 余额只能由 TxService.Create 驱动（唯一余额写入口），本服务不提供"直接改余额"的接口，
//     期初余额请通过一笔收入交易建立。
import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"fastledger/internal/model"
	"fastledger/internal/version"

	"xorm.io/xorm"
)

var (
	// ErrAccountNotFound 账户不存在
	ErrAccountNotFound = errors.New("账户不存在")
	// ErrAccountInvalid 账户参数非法
	ErrAccountInvalid = errors.New("账户参数非法")
	// ErrAccountInUse 账户仍被未删除的交易引用，不允许删除
	ErrAccountInUse = errors.New("账户仍存在交易记录，无法删除")
)

// AccountService 账户服务
type AccountService struct {
	engine   *xorm.Engine
	versions version.Source
}

// NewAccountService 构造账户服务（版本号经 VersionSource 注入，保证唯一实现）
func NewAccountService(engine *xorm.Engine, versions version.Source) *AccountService {
	return &AccountService{engine: engine, versions: versions}
}

// AccountCreateInput 新建账户入参
type AccountCreateInput struct {
	Name     string
	Category int8
	Currency string
	ParentId int64
}

// AccountUpdateInput 修改账户入参（nil 表示不修改）
type AccountUpdateInput struct {
	Name     *string
	ParentId *int64
}

// List 账户列表（含已删除过滤，按资产在前、id 升序）
func (s *AccountService) List(uid, bookID int64) ([]model.Account, error) {
	if err := requireMember(s.engine, bookID, uid); err != nil {
		return nil, err
	}
	out := make([]model.Account, 0, 8)
	err := s.engine.SQL(
		`SELECT * FROM accounts WHERE book_id = ? AND deleted = 0 ORDER BY category ASC, id ASC`,
		bookID).Find(&out)
	return out, err
}

// Get 账户详情
func (s *AccountService) Get(uid, bookID, accountID int64) (*model.Account, error) {
	if err := requireMember(s.engine, bookID, uid); err != nil {
		return nil, err
	}
	var a model.Account
	has, err := s.engine.Where("id = ? AND book_id = ? AND deleted = 0", accountID, bookID).Get(&a)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrAccountNotFound
	}
	return &a, nil
}

// Create 新建账户（初始余额恒为 0，避免绕过交易入口直接写余额）
func (s *AccountService) Create(uid, bookID int64, in AccountCreateInput) (*model.Account, error) {
	name, err := normalizeName(in.Name, "账户名称")
	if err != nil {
		return nil, err
	}
	if !model.IsValidAccountCategory(in.Category) {
		return nil, ErrAccountInvalid
	}
	currency, err := normalizeCurrency(in.Currency)
	if err != nil {
		return nil, err
	}

	sess := s.engine.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return nil, err
	}
	defer func() { _ = sess.Rollback() }()

	if _, err := requireManagerTx(sess, bookID, uid); err != nil {
		return nil, err
	}
	if in.ParentId != 0 {
		if err := ensureAccountInBook(sess, bookID, in.ParentId); err != nil {
			return nil, err
		}
	}
	ver, err := s.versions.Next(sess, bookID)
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	a := &model.Account{
		BookId:      bookID,
		Name:        name,
		Category:    in.Category,
		Currency:    currency,
		Balance:     0,
		ParentId:    in.ParentId,
		SyncVersion: ver,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if _, err := sess.Insert(a); err != nil {
		return nil, err
	}
	if err := sess.Commit(); err != nil {
		return nil, err
	}
	return a, nil
}

// Update 修改账户（仅名称与 UI 分组 parent_id）。
//
// 刻意不支持修改 category：资产/负债语义会翻转余额含义（负债 balance 正=欠款），
// 直接改分类会让历史交易与净资产口径失真。需要换类型请新建账户。
func (s *AccountService) Update(uid, bookID, accountID int64, in AccountUpdateInput) (*model.Account, error) {
	sess := s.engine.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return nil, err
	}
	defer func() { _ = sess.Rollback() }()

	if _, err := requireManagerTx(sess, bookID, uid); err != nil {
		return nil, err
	}
	var a model.Account
	has, err := sess.Where("id = ? AND book_id = ? AND deleted = 0", accountID, bookID).Get(&a)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrAccountNotFound
	}
	if in.Name != nil {
		name, nerr := normalizeName(*in.Name, "账户名称")
		if nerr != nil {
			return nil, nerr
		}
		a.Name = name
	}
	if in.ParentId != nil {
		parentID := *in.ParentId
		if parentID == accountID {
			return nil, fmt.Errorf("%w：账户的上级不能是自身", ErrAccountInvalid)
		}
		if parentID != 0 {
			if err := ensureAccountInBook(sess, bookID, parentID); err != nil {
				return nil, err
			}
		}
		a.ParentId = parentID
	}
	ver, err := s.versions.Next(sess, bookID)
	if err != nil {
		return nil, err
	}
	a.SyncVersion = ver
	a.UpdatedAt = time.Now().Unix()
	n, err := sess.Where("id = ? AND book_id = ? AND deleted = 0", accountID, bookID).
		Cols("name", "parent_id", "sync_version", "updated_at").
		Update(&model.Account{Name: a.Name, ParentId: a.ParentId, SyncVersion: ver, UpdatedAt: a.UpdatedAt})
	if err != nil {
		return nil, err
	}
	if n != 1 {
		return nil, ErrConcurrentConflict
	}
	if err := sess.Commit(); err != nil {
		return nil, err
	}
	return &a, nil
}

// Delete 软删账户（保留历史交易引用）；存在未删除交易时拒绝，避免账目悬空
func (s *AccountService) Delete(uid, bookID, accountID int64) error {
	sess := s.engine.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	defer func() { _ = sess.Rollback() }()

	if _, err := requireManagerTx(sess, bookID, uid); err != nil {
		return err
	}
	if err := ensureAccountInBook(sess, bookID, accountID); err != nil {
		return err
	}
	var used int64
	if _, err := sess.SQL(
		`SELECT COUNT(1) FROM transactions
		 WHERE book_id = ? AND deleted = 0 AND (account_id = ? OR to_account_id = ?)`,
		bookID, accountID, accountID).Get(&used); err != nil {
		return err
	}
	if used > 0 {
		return ErrAccountInUse
	}
	ver, err := s.versions.Next(sess, bookID)
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	res, err := sess.Exec(
		`UPDATE accounts SET deleted = 1, deleted_at = ?, sync_version = ?, updated_at = ?
		 WHERE id = ? AND book_id = ? AND deleted = 0`, now, ver, now, accountID, bookID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrAccountNotFound
	}
	return sess.Commit()
}

// ensureAccountInBook 校验账户归属本账本且未删除（防越权引用他人账本账户）
func ensureAccountInBook(sess *xorm.Session, bookID, accountID int64) error {
	var id int64
	has, err := sess.SQL(
		`SELECT id FROM accounts WHERE id = ? AND book_id = ? AND deleted = 0`, accountID, bookID).Get(&id)
	if err != nil {
		return err
	}
	if !has {
		return ErrAccountNotFound
	}
	return nil
}

// normalizeName 去空白并校验非空、长度不超过 64 字符
func normalizeName(raw, field string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" || utf8.RuneCountInString(name) > 64 {
		return "", fmt.Errorf("%w：%s必须为 1~64 个字符", ErrInvalidArgument, field)
	}
	return name, nil
}

// normalizeCurrency MVP 仅支持 CNY（0001 DDL 注释：default_currency 仅 CNY）
func normalizeCurrency(raw string) (string, error) {
	cur := strings.ToUpper(strings.TrimSpace(raw))
	if cur == "" {
		cur = "CNY"
	}
	if cur != "CNY" {
		return "", fmt.Errorf("%w：MVP 仅支持 CNY 账户", ErrInvalidArgument)
	}
	return cur, nil
}
