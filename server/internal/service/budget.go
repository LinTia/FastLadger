package service

// 预算（WBS 6.3，P2）——信封式：总预算 + 分类月度预算。
//
// 口径约束（逐条对应文档，不得自创）：
//  1. 【表结构】budgets 表由迁移 0001 提供（零迁移）；无 sync_version 列，
//     预算不在增量同步契约内，服务层不做版本递增（写路径仅审计留痕）。
//  2. 【周期】MVP 固定自然月（PRD §12 决策 #1）；period_type 只接受 1，
//     2~4（季/年/周）为枚举预留，传入即 400。
//  3. 【唯一性】同一分类同时只允许一条生效预算；总预算（type=2）全本最多一条
//     ——0001 未建唯一约束，由服务层在同一事务内复核（并发写靠 SQLite 单写者
//     + 事务内二次校验收敛，最坏情况是后到者拿到 409）。
//  4. 【权限】预算属账本级共享配置：写操作 owner/admin（requireManagerTx，
//     口径同账户/分类）；读操作全体成员。
//  5. 【进度可见性】"已用"聚合的是交易表——这是读路径，必须挂
//     txVisibilityCond（关闭互看的 member 只统计本人支出；红线同报表）。
//  6. 【聚合】分类预算挂大类时，已用 = 大类自身 + 全部直接子类（WBS 6.3
//     "大类+全部小类"）；区间为半开 [本月 1 日 0 点, 下月 1 日 0 点)（CST）。
//  7. 【状态】percent = 已用/总额：≥100% 超支(over)、≥80% 接近(near)、其余正常
//     （PRD §4.7 阈值口径）。
import (
	"errors"
	"fmt"
	"strings"
	"time"

	"fastledger/internal/model"

	"xorm.io/xorm"
)

var (
	// ErrBudgetNotFound 预算不存在
	ErrBudgetNotFound = errors.New("预算不存在")
	// ErrBudgetConflict 同分类/总预算已存在生效预算
	ErrBudgetConflict = errors.New("该分类已存在生效预算（或总预算已设置）")
)

// BudgetService 预算服务
type BudgetService struct {
	engine *xorm.Engine
}

func NewBudgetService(engine *xorm.Engine) *BudgetService {
	return &BudgetService{engine: engine}
}

// BudgetInput 新建/修改预算入参
type BudgetInput struct {
	Name       string
	BudgetType int8
	CategoryId int64
	PeriodType int8
	Amount     int64
	Rollover   bool
	Enabled    *bool // nil = 沿用（新建默认 true）
}

// BudgetView 预算视图：配置 + 本月进度
type BudgetView struct {
	model.Budget
	CategoryName string `json:"categoryName"`
	Spent        int64  `json:"spent"`   // 本月已用（整数分）
	Percent      int    `json:"percent"` // 0~100+（超支可超 100）
	Status       string `json:"status"`  // normal | near | over
}

func (s *BudgetService) validate(in *BudgetInput) error {
	if in.Amount <= 0 {
		return fmt.Errorf("%w：预算金额必须为正整数分", ErrInvalidArgument)
	}
	if in.BudgetType != model.BudgetTypeCategory && in.BudgetType != model.BudgetTypeTotal {
		return fmt.Errorf("%w：budget_type 仅支持 1（分类预算）或 2（总预算）", ErrInvalidArgument)
	}
	if in.PeriodType == 0 {
		in.PeriodType = model.PeriodTypeMonthly
	}
	if in.PeriodType != model.PeriodTypeMonthly {
		// PRD §12 决策 #1：MVP 固定自然月，自定义周期为 P2
		return fmt.Errorf("%w：MVP 预算周期固定为自然月", ErrInvalidArgument)
	}
	if in.BudgetType == model.BudgetTypeTotal && in.CategoryId != 0 {
		return fmt.Errorf("%w：总预算不能挂分类", ErrInvalidArgument)
	}
	if len([]rune(in.Name)) > 64 {
		return fmt.Errorf("%w：预算名称不超过 64 字符", ErrInvalidArgument)
	}
	return nil
}

// Create 新建预算（owner/admin）。
func (s *BudgetService) Create(uid, bookID int64, in BudgetInput) (*model.Budget, error) {
	if err := s.validate(&in); err != nil {
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
	if err := s.ensureUnique(sess, bookID, in.BudgetType, in.CategoryId, 0); err != nil {
		return nil, err
	}
	if in.BudgetType == model.BudgetTypeCategory {
		if err := ensureExpenseCategory(sess, bookID, in.CategoryId); err != nil {
			return nil, err
		}
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	now := time.Now().Unix()
	b := &model.Budget{
		BookId:     bookID,
		Name:       in.Name,
		BudgetType: in.BudgetType,
		CategoryId: in.CategoryId,
		PeriodType: in.PeriodType,
		Amount:     in.Amount,
		Rollover:   in.Rollover,
		Enabled:    enabled,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if _, err := sess.Insert(b); err != nil {
		return nil, err
	}
	// 预算同为账本级共享配置：写路径必须留痕（口径同账户/分类，PRD §10.9）
	if err := insertAudit(sess, bookID, uid, model.AuditActionCreate,
		model.AuditEntityBudget, b.Id, map[string]interface{}{
			"name": b.Name, "budgetType": b.BudgetType,
			"categoryId": b.CategoryId, "amount": b.Amount,
		}, "api"); err != nil {
		return nil, err
	}
	if err := sess.Commit(); err != nil {
		return nil, err
	}
	return b, nil
}

// Update 修改预算金额/名称/开关（owner/admin）。类型与挂靠分类不可改——
// 改挂靠等于删了重建（审计语义清晰：delete+create 各留一条）。
func (s *BudgetService) Update(uid, bookID, budgetID int64, in BudgetInput) (*model.Budget, error) {
	if in.Amount <= 0 {
		return nil, fmt.Errorf("%w：预算金额必须为正整数分", ErrInvalidArgument)
	}
	if len([]rune(in.Name)) > 64 {
		return nil, fmt.Errorf("%w：预算名称不超过 64 字符", ErrInvalidArgument)
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
	var before model.Budget
	has, err := sess.Where("id = ? AND book_id = ? AND deleted = 0", budgetID, bookID).Get(&before)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrBudgetNotFound
	}
	beforeJson := map[string]interface{}{"amount": before.Amount, "name": before.Name, "enabled": before.Enabled}
	before.Amount = in.Amount
	if in.Name != "" {
		before.Name = in.Name
	}
	if in.Enabled != nil {
		before.Enabled = *in.Enabled
	}
	before.Rollover = in.Rollover
	before.UpdatedAt = time.Now().Unix()
	if _, err := sess.ID(before.Id).AllCols().Update(&before); err != nil {
		return nil, err
	}
	if err := insertAudit(sess, bookID, uid, model.AuditActionUpdate,
		model.AuditEntityBudget, before.Id, map[string]interface{}{
			"before": beforeJson, "after": map[string]interface{}{
				"amount": before.Amount, "name": before.Name, "enabled": before.Enabled,
			},
		}, "api"); err != nil {
		return nil, err
	}
	if err := sess.Commit(); err != nil {
		return nil, err
	}
	return &before, nil
}

// Delete 删除预算（软删，owner/admin）。
func (s *BudgetService) Delete(uid, bookID, budgetID int64) error {
	sess := s.engine.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	defer func() { _ = sess.Rollback() }()

	if _, err := requireManagerTx(sess, bookID, uid); err != nil {
		return err
	}
	var before model.Budget
	has, err := sess.Where("id = ? AND book_id = ? AND deleted = 0", budgetID, bookID).Get(&before)
	if err != nil {
		return err
	}
	if !has {
		return ErrBudgetNotFound
	}
	now := time.Now().Unix()
	before.Deleted = true
	before.DeletedAt = now
	before.UpdatedAt = now
	if _, err := sess.ID(before.Id).Cols("deleted", "deleted_at", "updated_at").Update(&before); err != nil {
		return err
	}
	if err := insertAudit(sess, bookID, uid, model.AuditActionDelete,
		model.AuditEntityBudget, before.Id, map[string]interface{}{
			"name": before.Name, "budgetType": before.BudgetType,
			"categoryId": before.CategoryId, "amount": before.Amount,
		}, "api"); err != nil {
		return err
	}
	return sess.Commit()
}

// List 预算列表 + 本月进度（全体成员可读；"已用"按可见性过滤）。
func (s *BudgetService) List(uid, bookID int64) ([]BudgetView, error) {
	if err := requireMember(s.engine, bookID, uid); err != nil {
		return nil, err
	}
	bs := make([]model.Budget, 0, 8)
	if err := s.engine.SQL(
		`SELECT * FROM budgets WHERE book_id = ? AND deleted = 0 ORDER BY budget_type DESC, id ASC`,
		bookID).Find(&bs); err != nil {
		return nil, err
	}

	canViewAll, err := canViewAllForTx(s.engine, uid, bookID)
	if err != nil {
		return nil, err
	}
	start, end := currentMonthRangeCST()
	// 名称映射（含已删分类——历史预算仍应显示原名）
	catNames, err := s.categoryNames(bookID)
	if err != nil {
		return nil, err
	}
	out := make([]BudgetView, 0, len(bs))
	for _, b := range bs {
		v := BudgetView{Budget: b}
		if b.CategoryId > 0 {
			v.CategoryName = catNames[b.CategoryId]
		}
		if b.Enabled {
			spent, err := s.spent(uid, bookID, b, canViewAll, start, end)
			if err != nil {
				return nil, err
			}
			v.Spent = spent
		}
		if b.Amount > 0 {
			v.Percent = int(v.Spent * 100 / b.Amount)
		}
		switch {
		case v.Percent >= 100:
			v.Status = "over"
		case v.Percent >= 80:
			v.Status = "near"
		default:
			v.Status = "normal"
		}
		out = append(out, v)
	}
	return out, nil
}

// spent 本月已用（半开区间 [start, end)；关闭互看的 member 只统计本人支出）。
// 分类预算挂大类：聚合范围 = 大类自身 + 全部直接子类（WBS 6.3）。
func (s *BudgetService) spent(uid, bookID int64, b model.Budget, canViewAll bool, start, end int64) (int64, error) {
	where := []string{"book_id = ?", "deleted = 0", `"type" = ?`, "tx_time >= ?", "tx_time < ?"}
	args := []interface{}{bookID, model.TxTypeExpense, start, end}
	if b.BudgetType == model.BudgetTypeCategory {
		childIDs, err := s.childCategoryIds(bookID, b.CategoryId)
		if err != nil {
			return 0, err
		}
		ids := make([]interface{}, 0, len(childIDs)+1)
		ids = append(ids, b.CategoryId)
		for _, c := range childIDs {
			ids = append(ids, c)
		}
		ph := strings.Repeat("?,", len(ids))
		where = append(where, "category_id IN ("+ph[:len(ph)-1]+")")
		args = append(args, ids...)
	}
	if vcond, vargs := txVisibilityCond(canViewAll, uid); vcond != "" {
		where = append(where, vcond)
		args = append(args, vargs...)
	}
	// SumInt 口径：SQL SUM 聚合到 int64，禁 float
	var row struct {
		S int64 `xorm:"s"`
	}
	_, err := s.engine.SQL(
		"SELECT COALESCE(SUM(amount), 0) AS s FROM transactions WHERE "+strings.Join(where, " AND "),
		args...).Get(&row)
	return row.S, err
}

func (s *BudgetService) childCategoryIds(bookID, parentID int64) ([]int64, error) {
	var ids []int64
	err := s.engine.SQL(
		`SELECT id FROM categories WHERE book_id = ? AND parent_id = ? AND deleted = 0`,
		bookID, parentID).Find(&ids)
	return ids, err
}

func (s *BudgetService) categoryNames(bookID int64) (map[int64]string, error) {
	var rows []struct {
		Id   int64  `xorm:"id"`
		Name string `xorm:"name"`
	}
	if err := s.engine.SQL(
		`SELECT id, name FROM categories WHERE book_id = ?`, bookID).Find(&rows); err != nil {
		return nil, err
	}
	m := make(map[int64]string, len(rows))
	for _, r := range rows {
		m[r.Id] = r.Name
	}
	return m, nil
}

// ensureUnique 同一事务内复核唯一性：分类预算同分类唯一；总预算全本唯一。
// excludeID 供 Update 场景排除自身（当前 Update 不改挂靠，保留以备扩展）。
func (s *BudgetService) ensureUnique(sess *xorm.Session, bookID int64, bType int8, categoryID, excludeID int64) error {
	var n int64
	cond := "book_id = ? AND deleted = 0 AND budget_type = ?"
	args := []interface{}{bookID, bType}
	if bType == model.BudgetTypeCategory {
		cond += " AND category_id = ?"
		args = append(args, categoryID)
	}
	if excludeID > 0 {
		cond += " AND id <> ?"
		args = append(args, excludeID)
	}
	if _, err := sess.SQL("SELECT COUNT(1) FROM budgets WHERE "+cond, args...).Get(&n); err != nil {
		return err
	}
	if n > 0 {
		return ErrBudgetConflict
	}
	return nil
}

// ensureExpenseCategory 分类预算只允许挂在支出分类上（信封式管支出；PRD §4.7）。
func ensureExpenseCategory(sess *xorm.Session, bookID, categoryID int64) error {
	var c model.Category
	has, err := sess.Where("id = ? AND book_id = ? AND deleted = 0", categoryID, bookID).Get(&c)
	if err != nil {
		return err
	}
	if !has {
		return fmt.Errorf("%w：分类不存在", ErrInvalidArgument)
	}
	if c.CatType != model.CategoryTypeExpense {
		return fmt.Errorf("%w：预算只能挂在支出分类上", ErrInvalidArgument)
	}
	return nil
}

// currentMonthRangeCST 本自然月的 [start, end) unix 秒（CST，口径同报表）。
func currentMonthRangeCST() (int64, int64) {
	now := time.Now().In(cstZone)
	first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, cstZone)
	return first.Unix(), first.AddDate(0, 1, 0).Unix()
}
