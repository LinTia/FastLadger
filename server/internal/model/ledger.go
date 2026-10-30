package model

// 记账核心实体：账户 / 分类 / 交易 / 幂等回执。
//
// 字段严格对齐 `migrate/migrations/0001_init.up.sql`（表结构是唯一权威），
// 枚举取值以《FastLedger-枚举字典》为唯一权威，金额一律整数分。
//
// 负债余额符号（枚举字典 §2）：负债账户（10~15）的 balance 存储为【正=欠款】，
// 故刷卡消费使 balance 增加、还款使 balance 减少；净资产 = Σ(1~9) − Σ(10~15)。

// 交易类型 TxType（transactions.type）——《枚举字典》§1
const (
	TxTypeIncome    int8 = 1 // 收入
	TxTypeExpense   int8 = 2 // 支出
	TxTypeTransfer  int8 = 3 // 转账（成对，related_id 配对）
	TxTypeTradeBuy  int8 = 4 // 买入（v0.3 启用，本阶段不开放）
	TxTypeTradeSell int8 = 5 // 卖出（v0.3 启用，本阶段不开放）
	TxTypeDividend  int8 = 6 // 分红（v0.3 启用，本阶段不开放）
	TxTypeInterest  int8 = 7 // 利息（v0.3 启用，本阶段不开放）
)

// TxTypeName 交易类型展示名（仅日志/错误提示用）
func TxTypeName(t int8) string {
	switch t {
	case TxTypeIncome:
		return "收入"
	case TxTypeExpense:
		return "支出"
	case TxTypeTransfer:
		return "转账"
	case TxTypeTradeBuy:
		return "买入"
	case TxTypeTradeSell:
		return "卖出"
	case TxTypeDividend:
		return "分红"
	case TxTypeInterest:
		return "利息"
	default:
		return "未知类型"
	}
}

// 账户分类 AccountCategory（accounts.category）——《枚举字典》§2
const (
	CategoryCash          int8 = 1  // 现金
	CategoryDemandDeposit int8 = 2  // 活期储蓄
	CategoryTimeDeposit   int8 = 3  // 定期存款
	CategoryEWallet       int8 = 4  // 电子钱包
	CategoryInvestment    int8 = 5  // 投资账户
	CategoryPhysicalAsset int8 = 6  // 实物资产
	CategoryInsurance     int8 = 7  // 保险/年金
	CategoryOtherAsset    int8 = 8  // 其他资产
	CategoryReceivable    int8 = 9  // 应收款
	CategoryCreditCard    int8 = 10 // 信用卡
	CategoryConsumerLoan  int8 = 11 // 信用消费贷
	CategoryMortgage      int8 = 12 // 房贷
	CategoryCarLoan       int8 = 13 // 车贷
	CategoryOtherDebt     int8 = 14 // 其他借款
	CategoryPayable       int8 = 15 // 应付款
)

// IsValidAccountCategory 账户分类合法（1~15）
func IsValidAccountCategory(c int8) bool {
	return c >= CategoryCash && c <= CategoryPayable
}

// IsLiabilityCategory 10~15 为负债账户；净资产公式依赖它（枚举字典 §2）
func IsLiabilityCategory(c int8) bool {
	return c >= CategoryCreditCard && c <= CategoryPayable
}

// 支付渠道 PaymentChannel（transactions.payment_channel）——《枚举字典》§3
const (
	PaymentChannelDirect   int8 = 0 // 直接支付（默认）
	PaymentChannelWeChat   int8 = 1 // 微信
	PaymentChannelAlipay   int8 = 2 // 支付宝
	PaymentChannelUnionPay int8 = 3 // 云闪付
	PaymentChannelPOS      int8 = 4 // POS 刷卡
	PaymentChannelOther    int8 = 5 // 其他（预留）
)

// 分类类型 CategoryType（categories.cat_type）——1=收入 2=支出
const (
	CategoryTypeIncome  int8 = 1
	CategoryTypeExpense int8 = 2
)

// 数据来源 Source（transactions 无该列，仅参与命令校验与幂等摘要）——《枚举字典》§10
const (
	SourceManual      int8 = 0
	SourceRecognition int8 = 1
	SourceImport      int8 = 2 // 预留
)

// 幂等结果类型 IdempotencyResultType（idempotency_records.result_type）——《枚举字典》§12
const (
	IdempotencyResultSingle   int8 = 1 // 收入或支出，result_ids 含 1 个交易 ID
	IdempotencyResultTransfer int8 = 2 // 转账，result_ids 按 [转出,转入] 含 2 个交易 ID
)

// Account 资金账户
type Account struct {
	Id          int64  `xorm:"pk autoincr" json:"id"`
	BookId      int64  `xorm:"notnull" json:"bookId"`
	Name        string `xorm:"varchar(64) notnull" json:"name"`
	Category    int8   `xorm:"notnull" json:"category"` // 1~9 资产 / 10~15 负债
	Currency    string `xorm:"varchar(8) default 'CNY'" json:"currency"`
	Balance     int64  `xorm:"notnull default 0" json:"balance"` // 整数分；负债账户正=欠款
	ParentId    int64  `xorm:"default 0" json:"parentId"`        // 仅 UI 分组，不参与余额计算
	SyncVersion int64  `xorm:"notnull default 0" json:"syncVersion"`
	Deleted     bool   `xorm:"notnull default 0" json:"-"`
	CreatedAt   int64  `xorm:"created" json:"createdAt"`
	UpdatedAt   int64  `xorm:"updated" json:"updatedAt"`
	DeletedAt   int64  `json:"-"`
}

func (Account) TableName() string { return "accounts" }

// Category 收支分类（两级：parent_id=0 为大类，否则为小类）
type Category struct {
	Id          int64  `xorm:"pk autoincr" json:"id"`
	BookId      int64  `xorm:"notnull" json:"bookId"`
	Name        string `xorm:"varchar(64) notnull" json:"name"`
	ParentId    int64  `xorm:"default 0" json:"parentId"`
	CatType     int8   `xorm:"notnull" json:"catType"` // 1=收入 2=支出
	SyncVersion int64  `xorm:"notnull default 0" json:"syncVersion"`
	Deleted     bool   `xorm:"notnull default 0" json:"-"`
	CreatedAt   int64  `xorm:"created" json:"createdAt"`
	UpdatedAt   int64  `xorm:"updated" json:"updatedAt"`
	DeletedAt   int64  `json:"-"`
}

func (Category) TableName() string { return "categories" }

// Transaction 交易记录
//
// 转账一次写两条：转出记录 account_id=from、to_account_id=to；转入记录反之。
// 两条记录的 related_id 互为对端主键，构成"整笔"分组键（阶段 4 删除/恢复依赖）。
//
// creator_uid / source / timezone_offset 由迁移 0003 补入（0001 未含）：
// creator_uid 是 PRD §7.3 可见性与阶段 4「member 仅能改删本人交易」的判定依据，
// 因此创建时【必须】落库，不能留 0。
type Transaction struct {
	Id             int64  `xorm:"pk autoincr" json:"id"`
	BookId         int64  `xorm:"notnull" json:"bookId"`
	CreatorUid     int64  `xorm:"notnull default 0" json:"creatorUid"`
	TxType         int8   `xorm:"'type' notnull" json:"txType"` // 列名就是 type，必须显式引用
	AccountId      int64  `xorm:"notnull" json:"accountId"`
	ToAccountId    int64  `xorm:"default 0" json:"toAccountId"`
	Amount         int64  `xorm:"notnull" json:"amount"` // 整数分，恒为正
	CategoryId     int64  `xorm:"default 0" json:"categoryId"`
	PaymentChannel int8   `xorm:"default 0" json:"paymentChannel"`
	Merchant       string `xorm:"varchar(128) default ''" json:"merchant"`
	Note           string `xorm:"varchar(255) default ''" json:"note"`
	TxTime         int64  `xorm:"notnull" json:"txTime"`
	TimezoneOffset int    `xorm:"notnull default 0" json:"timezoneOffset"`
	Source         int8   `xorm:"notnull default 0" json:"source"`
	RelatedId      int64  `xorm:"default 0" json:"relatedId"`
	ClientUuid     string `xorm:"varchar(64) default ''" json:"clientUuid"`
	SyncVersion    int64  `xorm:"notnull default 0" json:"syncVersion"`
	Deleted        bool   `xorm:"notnull default 0" json:"-"`
	CreatedAt      int64  `xorm:"created" json:"createdAt"`
	UpdatedAt      int64  `xorm:"updated" json:"updatedAt"`
	DeletedAt      int64  `json:"-"`
}

func (Transaction) TableName() string { return "transactions" }

// IdempotencyRecord 幂等回执（唯一键 book_id + uid + client_uuid）
//
// 老库升级【不得删除旧回执】，否则离线重试会产生重复账（0001 注释）。
type IdempotencyRecord struct {
	Id            int64  `xorm:"pk autoincr"`
	BookId        int64  `xorm:"notnull"`
	Uid           int64  `xorm:"notnull"`
	ClientUuid    string `xorm:"varchar(64) notnull"`
	RequestHash   string `xorm:"varchar(128) notnull"`
	ResultIds     string `xorm:"text"`
	ResultType    int8   `xorm:"default 0"`
	ResultVersion int64  `xorm:"default 0"`
	CreatedAt     int64
}

func (IdempotencyRecord) TableName() string { return "idempotency_records" }
