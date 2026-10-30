package model

// 持仓（holdings）—— 字段严格对齐 `migrate/migrations/0001_init.up.sql` 的 holdings 表。
//
// 说明：
//   - v0.1 只支持手动录入（PRD §12 决策#3：MVP 不做自动行情），故 quantity / current_price
//     用字符串保存（"字符串 + decimal"，避免浮点误差），cost_amount 为整数分。
//   - 阶段 3 的增量同步需要把持仓作为第四类实体一起下发（指南 §22.2），
//     因此本阶段补上该实体（表在 0001 迁移中早已存在，无需新迁移）。
type Holding struct {
	Id           int64  `xorm:"pk autoincr" json:"id"`
	BookId       int64  `xorm:"notnull" json:"bookId"`
	AccountId    int64  `xorm:"notnull" json:"accountId"`
	InstrumentId int64  `xorm:"notnull" json:"instrumentId"`
	Quantity     string `xorm:"varchar(32) notnull default '0'" json:"quantity"`
	CostAmount   int64  `xorm:"notnull default 0" json:"costAmount"`
	CurrentPrice string `xorm:"varchar(32) default '0'" json:"currentPrice"`
	SyncVersion  int64  `xorm:"notnull default 0" json:"syncVersion"`
	Deleted      bool   `xorm:"notnull default 0" json:"deleted"` // 同步墓碑标识：客户端须以 deleted=true 覆盖本地行
	CreatedAt    int64  `xorm:"created" json:"createdAt"`
	UpdatedAt    int64  `xorm:"updated" json:"updatedAt"`
	DeletedAt    int64  `json:"deletedAt,omitempty"` // 墓碑时间（未删除时为 0，不输出）
}

func (Holding) TableName() string { return "holdings" }
