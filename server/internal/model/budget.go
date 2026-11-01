package model

// 预算（budgets）——表结构来自迁移 0001（已含全部所需列，本批零迁移）。
// 枚举口径：《枚举字典》§7 / §8。
//
// 口径约束：
//   - budget_type：1=分类预算（category_id 必填）、2=总预算（category_id=0）；
//   - period_type：MVP 固定自然月（PRD §12 决策 #1），2~4 为预留，服务层拒绝；
//   - rollover：仅存储与展示，MVP 不实现结转（PRD §4.7）；
//   - 金额一律整数分。
const (
	BudgetTypeCategory int8 = 1 // 分类预算
	BudgetTypeTotal    int8 = 2 // 总预算

	PeriodTypeMonthly int8 = 1 // 月（MVP 唯一支持）
)

// AuditEntityBudget 预算审计实体（《枚举字典》§14.2，账本级共享配置）。
const AuditEntityBudget = "budget"

// Budget 信封式预算（每月 1 日重置，无月份维度——预算额是配置而非按月流水）。
// 注意：0001 的 budgets 表【没有 sync_version 列】——预算不在增量同步契约内
// （客户端经 REST 读取），服务层因此不做版本递增。
type Budget struct {
	Id         int64  `xorm:"pk autoincr" json:"id"`
	BookId     int64  `xorm:"notnull" json:"bookId"`
	Name       string `xorm:"varchar(64) notnull" json:"name"`
	BudgetType int8   `xorm:"'budget_type' notnull" json:"budgetType"`
	CategoryId int64  `xorm:"default 0" json:"categoryId"`
	PeriodType int8   `xorm:"'period_type' default 1" json:"periodType"`
	Amount     int64  `xorm:"notnull" json:"amount"` // 整数分，恒为正
	Rollover   bool   `xorm:"default 0" json:"rollover"`
	Enabled    bool   `xorm:"default 1" json:"enabled"`
	Deleted    bool   `xorm:"notnull default 0" json:"deleted"`
	CreatedAt  int64  `xorm:"created" json:"createdAt"`
	UpdatedAt  int64  `xorm:"updated" json:"updatedAt"`
	DeletedAt  int64  `json:"deletedAt,omitempty"`
}

func (Budget) TableName() string { return "budgets" }
