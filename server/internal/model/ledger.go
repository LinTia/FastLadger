package model

// 记账核心实体：账户 / 分类 / 交易 / 幂等回执。
//
// 字段严格对齐 `migrate/migrations/0001_init.up.sql`（表结构是唯一权威），
// 枚举取值以《FastLedger-枚举字典》为唯一权威，金额一律整数分。
//
// 软删除字段的 JSON 契约（阶段 3 同步墓碑，**不要改回 `json:"-"`**）：
//
// 增量同步（sync/pull）故意下发**墓碑行**（不过滤 deleted），客户端必须能识别
// `deleted=true` 并覆盖本地对应行；若该字段被隐藏，客户端会把墓碑当活行处理
// （表现为"删掉的数据又回来了"）。REST 读路径本就只查 `deleted=0`，
// 因此对外暴露该字段没有副作用。`deletedAt` 为墓碑时间（0 值不输出）。
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
	Deleted     bool   `xorm:"notnull default 0" json:"deleted"` // 同步墓碑标识：客户端须以 deleted=true 覆盖本地行
	CreatedAt   int64  `xorm:"created" json:"createdAt"`
	UpdatedAt   int64  `xorm:"updated" json:"updatedAt"`
	DeletedAt   int64  `json:"deletedAt,omitempty"` // 墓碑时间（未删除时为 0，不输出）
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
	Deleted     bool   `xorm:"notnull default 0" json:"deleted"` // 同步墓碑标识：客户端须以 deleted=true 覆盖本地行
	CreatedAt   int64  `xorm:"created" json:"createdAt"`
	UpdatedAt   int64  `xorm:"updated" json:"updatedAt"`
	DeletedAt   int64  `json:"deletedAt,omitempty"` // 墓碑时间（未删除时为 0，不输出）
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
	Direction      int8   `xorm:"notnull default 0" json:"direction"` // 0=单笔 1=转出侧 2=转入侧（仅转账）；删除/恢复时据其精确反向余额
	ClientUuid     string `xorm:"varchar(64) default ''" json:"clientUuid"`
	SyncVersion    int64  `xorm:"notnull default 0" json:"syncVersion"`
	Deleted        bool   `xorm:"notnull default 0" json:"deleted"` // 同步墓碑标识：客户端须以 deleted=true 覆盖本地行
	CreatedAt      int64  `xorm:"created" json:"createdAt"`
	UpdatedAt      int64  `xorm:"updated" json:"updatedAt"`
	DeletedAt      int64  `json:"deletedAt,omitempty"` // 墓碑时间（未删除时为 0，不输出）
}

// 转账方向（transactions.direction）——《枚举字典》§13
const (
	TxDirectionSingle int8 = 0 // 单笔（收入/支出）
	TxDirectionOut    int8 = 1 // 转账转出侧（资金离场，余额减少）
	TxDirectionIn     int8 = 2 // 转账转入侧（资金入场，余额增加）
)

// AuditLog 审计日志（append-only；PRD §12.1 决策，保留 1 年）。
//
// 软删/恢复/创建/权限变更等均在此留痕；仓库层只暴露 Insert（禁止 UPDATE/DELETE）。
// before_json 记录变更前状态——软删本身回答不了"删前是什么"，故必须落原文。
type AuditLog struct {
	Id         int64  `xorm:"pk autoincr" json:"id"`
	BookId     int64  `xorm:"notnull" json:"bookId"`
	ActorUid   int64  `xorm:"notnull" json:"actorUid"`
	Action     string `xorm:"varchar(32) notnull" json:"action"`
	EntityType string `xorm:"varchar(32) notnull" json:"entityType"`
	EntityId   int64  `xorm:"notnull default 0" json:"entityId"`
	BeforeJson string `xorm:"text" json:"beforeJson"`
	At         int64  `xorm:"notnull" json:"at"`
	Source     string `xorm:"varchar(16) default ''" json:"source"`
}

func (AuditLog) TableName() string { return "audit_logs" }

// 审计动作（audit_logs.action）——《枚举字典》§14
//
// 前四项是实体增删与交易恢复；后四项为 WBS 4.3 要求的非交易写入点。
// 备份恢复刻意用 backup_restore 而非 restore：restore 已被"恢复一笔交易"占用，
// 混用会让"这笔交易被恢复过"与"整个实例从备份恢复过"无法区分。
const (
	AuditActionCreate  = "create"
	AuditActionUpdate  = "update"
	AuditActionDelete  = "delete"
	AuditActionRestore = "restore"

	// 以下五类不隶属任何账本，落库时 book_id 取 AuditBookInstance（0）
	AuditActionLogin            = "login"             // 登录（成功事件——枚举字典 §14.1 明确口径）
	AuditActionLoginFailed      = "login_failed"      // 登录失败（加固项，与登录失败限流捆绑；见枚举字典 §14.1）
	AuditActionPermissionChange = "permission_change" // 权限变更（改角色 / 移除成员）
	AuditActionConfigChange     = "config_change"     // 配置变更（可见性开关等）
	AuditActionBackupRestore    = "backup_restore"    // 备份恢复（restore_epoch 递增）
)

// 审计实体类型（audit_logs.entity_type）——《枚举字典》§14
const (
	AuditEntityTransaction = "transaction"
	AuditEntityUser        = "user"
	AuditEntityMember      = "book_member"
	AuditEntityBook        = "book"
	AuditEntityInstance    = "instance"
	AuditEntityAccount     = "account"  // 账户（共享配置，PRD §10.9"任一写操作"口径）
	AuditEntityCategory    = "category" // 分类（同上）
)

// AuditActorSystem 审计 actor_uid 的哨兵值：系统 / 运维操作（无登录用户）。
//
// 例如 CLI 执行的备份恢复（`fastledger instance bump`）。actor_uid 是 NOT NULL，
// 没有"空"语义，故显式用 0 表示"不是某个真实用户发起的"。
const AuditActorSystem int64 = 0

// AuditBookInstance 实例级事件的 book_id 哨兵值。
//
// 登录与备份恢复发生在"账本之外"（登录时还不知道要进哪个账本，恢复影响整个实例），
// 而 audit_logs.book_id 是 NOT NULL 且没有"空"语义，故显式用 0 表示"不隶属任何账本"。
// 查询时按需过滤：book_id=0 即实例级事件，不要把它当成某个真实账本。
const AuditBookInstance int64 = 0

func (Transaction) TableName() string { return "transactions" }

// IdempotencyRecord 幂等回执（唯一键 book_id + uid + op + client_uuid）
//
// op 由迁移 0005 补入，用于区分 create / delete / restore：没有它时同一个
// clientUuid 先删后恢复会互相命中回执（WBS 4.2 / 4.4）。
//
// 老库升级【不得删除旧回执】，否则离线重试会产生重复账（0001 注释）。
type IdempotencyRecord struct {
	Id            int64  `xorm:"pk autoincr"`
	BookId        int64  `xorm:"notnull"`
	Uid           int64  `xorm:"notnull"`
	ClientUuid    string `xorm:"varchar(64) notnull"`
	Op            string `xorm:"varchar(16) notnull default 'create'"`
	RequestHash   string `xorm:"varchar(128) notnull"`
	ResultIds     string `xorm:"text"`
	ResultType    int8   `xorm:"default 0"`
	ResultVersion int64  `xorm:"default 0"`
	CreatedAt     int64
}

// 幂等操作类型（idempotency_records.op）——由迁移 0005 引入
const (
	IdemOpCreate  = "create"
	IdemOpDelete  = "delete"
	IdemOpRestore = "restore"
)

func (IdempotencyRecord) TableName() string { return "idempotency_records" }
