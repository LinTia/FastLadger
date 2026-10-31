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
	// ErrAccountInUse 账户仍被未删除或仍在回收站的交易引用，不允许删除
	ErrAccountInUse = errors.New("账户仍存在交易记录，无法删除")
	// ErrAccountHasChildren 该账户下仍有未删除的子账户（parent_id 指向它），不允许删除。
	//
	// GPT-R53：直接删父账户会让子账户留下悬空的 parent_id——列表/j树组装时父节点
	// 不存在，前端只能把子账户当孤儿节点展示，父子层级（净资产分组）就此失真。
	ErrAccountHasChildren = errors.New("该账户下仍有子账户，无法删除")
	// ErrAccountCycle 账户层级形成环（GPT-R47）。
	//
	// 旧实现只挡住"parent_id 指向自己"，挡不住"指向自己的后代"（A→B→C→A）。
	// 层级一旦成环，树组装会无限递归，净资产分组的汇总口径也彻底失真——
	// 分组只是 UI 呈现，但它是净资产视图的结构，成环即数据损坏。
	ErrAccountCycle = errors.New("账户层级存在环：上级不能是自己的下级")
)

// maxAccountDepth 层级回溯的最大步数。
//
// 正常账本的分组层级是个位数；这个上限只是防御"历史脏数据里已存在环"时
// 回溯变成死循环——检出环要报错，而不是把请求线程挂死。
const maxAccountDepth = 1000

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
		// GPT-R47：新账户还没有子账户，无人能指向它，因此这里只需确认【父链本身有限】
		// ——历史脏数据里若已存在环（A→B→A），不能再往里挂新账户，否则环只会变大。
		if err := assertNoAccountCycle(sess, bookID, 0, in.ParentId); err != nil {
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
			// 1 节点环：与更长的环同一个哨兵（HTTP 层统一 400）
			return nil, fmt.Errorf("%w：账户的上级不能是自身", ErrAccountCycle)
		}
		if parentID != 0 {
			if err := ensureAccountInBook(sess, bookID, parentID); err != nil {
				return nil, err
			}
		}
		// GPT-R47：必须沿父链一路回溯到根，检测【任意长度】的环。
		// 只看一层（"上级是不是我的下级"）挡不住 A→B→C→A。
		if err := assertNoAccountCycle(sess, bookID, accountID, parentID); err != nil {
			return nil, err
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

// Delete 软删账户（保留历史交易引用）；存在未删除交易时拒绝，避免账目悬空。
//
// 返回本次写入产生的 `sync_version`：HTTP 适配层要用它广播 data_changed
// （账户是账本共享配置，任何在线成员都需要知道"有新版本"）。若返回 error，版本无意义。
func (s *AccountService) Delete(uid, bookID, accountID int64) (int64, error) {
	sess := s.engine.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return 0, err
	}
	defer func() { _ = sess.Rollback() }()

	if _, err := requireManagerTx(sess, bookID, uid); err != nil {
		return 0, err
	}
	if err := ensureAccountInBook(sess, bookID, accountID); err != nil {
		return 0, err
	}
	// GPT-R53：仍有子账户引用本账户的 parent_id 时显式拒绝。
	// 与 R47（parent_id 环路）不同：这里不是"能不能指向"，而是"被指向的一方能不能消失"。
	// 与 R31（交易引用账户）也不同：这里是账户树内部的父子引用。
	//
	// GPT-C06：检查必须覆盖**任何状态**的子账户（含已软删）。旧实现带 `deleted = 0`，
	// 于是"先删子账户、再删父账户"可以成功——软删的子账户日后被恢复时会带着悬空的
	// parent_id，层级组装直接产出孤儿；比活跃子账户更危险（恢复动作让悬空引用"复活"）。
	var children int64
	if _, err := sess.SQL(
		`SELECT COUNT(1) FROM accounts WHERE book_id = ? AND parent_id = ?`,
		bookID, accountID).Get(&children); err != nil {
		return 0, err
	}
	if children > 0 {
		return 0, ErrAccountHasChildren
	}
	// GPT-R31：引用检查必须覆盖【软删除】的交易。
	//
	// 旧实现带 `deleted = 0`，于是"先删交易、再删账户"可以成功：交易还躺在回收站里，
	// 30 天保留期内用户想恢复它，却发现恢复所依赖的账户已经没了——回收站形同虚设。
	// 现在只要还有任何一行（含墓碑）引用该账户，就不允许删除；
	// TTL 到期硬清理（PurgeExpiredTransactions）之后墓碑消失，账户才可正常删除。
	// 转账两侧账户都要计入：转出账户走 account_id、转入账户走 to_account_id。
	var used int64
	if _, err := sess.SQL(
		`SELECT COUNT(1) FROM transactions
		 WHERE book_id = ? AND (account_id = ? OR to_account_id = ?)`,
		bookID, accountID, accountID).Get(&used); err != nil {
		return 0, err
	}
	if used > 0 {
		return 0, ErrAccountInUse
	}
	ver, err := s.versions.Next(sess, bookID)
	if err != nil {
		return 0, err
	}
	now := time.Now().Unix()
	res, err := sess.Exec(
		`UPDATE accounts SET deleted = 1, deleted_at = ?, sync_version = ?, updated_at = ?
		 WHERE id = ? AND book_id = ? AND deleted = 0`, now, ver, now, accountID, bookID)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	if n != 1 {
		return 0, ErrAccountNotFound
	}
	if err := sess.Commit(); err != nil {
		return 0, err
	}
	return ver, nil
}

// assertNoAccountCycle 在【调用方事务内】沿 parent_id 向上回溯到根，确认
// 「把 accountID 挂到 parentID 下」不会形成环（GPT-R47）。
//
// 为什么必须一路回溯而不是只查一两层：环可以是任意长度（A→B→C→…→A），
// 只查一层挡不住 B→C→A 这种"隔代"回指。
//
// 并发保护（与写入同一事务）：
// 本函数只在 Create / Update 已开启的写事务内被调用，检查与随后的 INSERT/UPDATE
// 共享同一个事务与同一份快照；SQLite 是单写者（本库 MaxOpenConns=1），
// 别的写事务不可能插进"检查通过之后、写入之前"的窗口里插入一条指向本账户的
// parent_id——要么它先提交（本事务随后读到的是它的结果，照样检出环），
// 要么它后提交（会被写锁挡住，或本事务先拿到写锁、它拿到 SQLITE_BUSY）。
// 因此不存在"检查通过 → 并发插入环 → 写入落库"的 TOCTOU。
//
// accountID == 0 表示【新建】账户（尚无 ID，不可能有人指向它）：此时只校验父链本身
// 是有限的（不含历史脏数据留下的既存环）。
func assertNoAccountCycle(sess *xorm.Session, bookID, accountID, parentID int64) error {
	if parentID == 0 {
		return nil // 0 = 顶层账户，无上级，不可能成环
	}
	who := fmt.Sprintf("账户 %d", accountID)
	if accountID == 0 {
		who = "新账户"
	}
	if parentID == accountID {
		return fmt.Errorf("%w：%s的上级不能是自身", ErrAccountCycle, who)
	}

	seen := make(map[int64]struct{}, 8)
	cur := parentID
	for step := 0; step < maxAccountDepth; step++ {
		if cur == accountID {
			return fmt.Errorf("%w：%s的上级链路回到了自身（长度 ≥%d 的环）",
				ErrAccountCycle, who, step+1)
		}
		if _, dup := seen[cur]; dup {
			// 父链里存在既存环（历史脏数据）：不能再往里挂账户，否则环只会变大
			return fmt.Errorf("%w：%s的上级链路存在既存环（账户 %d 被重复访问）",
				ErrAccountCycle, who, cur)
		}
		seen[cur] = struct{}{}

		var next int64
		has, err := sess.SQL(
			`SELECT parent_id FROM accounts WHERE id = ? AND book_id = ? AND deleted = 0`,
			cur, bookID).Get(&next)
		if err != nil {
			return err
		}
		if !has {
			// 上级链路上出现"已删除 / 不属于本账本"的账户：这是悬空引用（GPT-R53 的
			// 对偶面）。挂上去只会让层级更乱，直接拒绝。
			return fmt.Errorf("%w：%s的上级 %d 已删除或不属于本账本", ErrAccountInvalid, who, cur)
		}
		if next == 0 {
			return nil // 回溯到顶层：链是有限的，无环
		}
		cur = next
	}
	return fmt.Errorf("%w：%s的上级链路超过 %d 层（疑似数据损坏）",
		ErrAccountCycle, who, maxAccountDepth)
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
