package model

// 投资与持仓（WBS 6.4）——表结构来自迁移 0001（instruments / instrument_prices /
// holdings 已预建，本批零迁移）。
//
// 口径约束：
//   - 标的类型 type：《枚举字典》§4（1 股票 ~ 6 其他）；market：《枚举字典》§5（自由串）。
//   - 数量/价格一律【字符串存储】，运算走 shopspring/decimal，禁 float（WBS 6.4 硬约束）。
//   - holdings.cost_amount 是整数分（买入总成本）；current_price 是"元/份"字符串。
//   - instrument_prices 为 append-only 行情历史（无 deleted 列）。
//   - instruments / holdings 含 sync_version 列 → 写路径递增账本版本（WBS 6.4：
//     AddHolding 带 versions VersionSource）。
const (
	InstrumentTypeStock         int8 = 1 // 股票
	InstrumentTypeFund          int8 = 2 // 基金
	InstrumentTypeBond          int8 = 3 // 债券
	InstrumentTypePreciousMetal int8 = 4 // 贵金属
	InstrumentTypeForex         int8 = 5 // 外汇
	InstrumentTypeOther         int8 = 6 // 其他
)

// IsValidInstrumentType 标的类型合法（1~6，《枚举字典》§4）。
func IsValidInstrumentType(t int8) bool {
	return t >= InstrumentTypeStock && t <= InstrumentTypeOther
}

// AuditEntityInstrument / AuditEntityHolding 投资审计实体（《枚举字典》§14.2）。
const (
	AuditEntityInstrument = "instrument"
	AuditEntityHolding    = "holding"
)

// Instrument 标的（账本内共享库；名称+类型+市场+代码）。
// 注：Holding 结构体已在 holding.go 声明（阶段 3 同步下发用），此处不再重复。
type Instrument struct {
	Id          int64  `xorm:"pk autoincr" json:"id"`
	BookId      int64  `xorm:"notnull" json:"bookId"`
	Name        string `xorm:"varchar(64) notnull" json:"name"`
	Type        int8   `xorm:"'type' notnull" json:"type"`
	Market      string `xorm:"varchar(16) default ''" json:"market"`
	Code        string `xorm:"varchar(32) default ''" json:"code"`
	SyncVersion int64  `xorm:"'sync_version' notnull default 0" json:"syncVersion"`
	Deleted     bool   `xorm:"notnull default 0" json:"deleted"`
	CreatedAt   int64  `xorm:"created" json:"createdAt"`
	UpdatedAt   int64  `xorm:"updated" json:"updatedAt"`
	DeletedAt   int64  `json:"deletedAt,omitempty"`
}

func (Instrument) TableName() string { return "instruments" }

// InstrumentPrice 行情历史（append-only；price 为"元/份"字符串）。
type InstrumentPrice struct {
	Id           int64  `xorm:"pk autoincr" json:"id"`
	InstrumentId int64  `xorm:"'instrument_id' notnull" json:"instrumentId"`
	Price        string `xorm:"varchar(32) notnull" json:"price"`
	PriceTime    int64  `xorm:"'price_time' notnull" json:"priceTime"`
}

func (InstrumentPrice) TableName() string { return "instrument_prices" }
