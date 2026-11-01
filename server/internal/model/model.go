// Package model 数据实体。
//
// 枚举取值以《FastLedger-枚举字典》为唯一权威；金额一律整数分。
package model

// 账本内角色（books / book_members.role）
const (
	RoleOwner  int8 = 1
	RoleAdmin  int8 = 2
	RoleMember int8 = 3
)

// 账本类型（books.book_type）
const (
	BookTypePersonal int8 = 1
	BookTypeFamily   int8 = 2
)

// User 用户
//
// JSON 契约与 ledger.go 一致采用 lowerCamel；PasswordHash 永不序列化（json:"-"），
// 防止未来任何 handler 直接返回 User 时泄露凭据材料。
type User struct {
	Id              int64  `xorm:"pk autoincr" json:"id"`
	Username        string `xorm:"varchar(64) notnull unique" json:"username"`
	PasswordHash    string `xorm:"varchar(255) notnull" json:"-"`
	Email           string `xorm:"varchar(128) notnull default ''" json:"email"`
	IsInstanceAdmin bool   `xorm:"notnull default 0" json:"isInstanceAdmin"`
	Disabled        bool   `xorm:"notnull default 0" json:"disabled"`
	Deleted         bool   `xorm:"notnull default 0" json:"deleted"`
	CreatedAt       int64  `xorm:"created" json:"createdAt"`
	UpdatedAt       int64  `xorm:"updated" json:"updatedAt"`
	DeletedAt       int64  `json:"deletedAt,omitempty"`
}

func (User) TableName() string { return "users" }

// Book 账本
//
// JSON 键为 lowerCamel（与 Account/Category/Transaction 一致）。
// 此前无 tag 时 API 输出 PascalCase（Id/Name/BookType），Web 端按
// bk.id/bk.name 取值全部落空——修复于 2026-11-01（首次生产部署前）。
type Book struct {
	Id                   int64  `xorm:"pk autoincr" json:"id"`
	Name                 string `xorm:"varchar(64) notnull" json:"name"`
	OwnerUid             int64  `xorm:"notnull" json:"ownerUid"`
	BookType             int8   `xorm:"notnull default 1" json:"bookType"`
	DefaultCurrency      string `xorm:"varchar(8) notnull default 'CNY'" json:"defaultCurrency"`
	MembersCanViewOthers bool   `xorm:"notnull default 1" json:"membersCanViewOthers"`
	Deleted              bool   `xorm:"notnull default 0" json:"deleted"`
	CreatedAt            int64  `xorm:"created" json:"createdAt"`
	UpdatedAt            int64  `xorm:"updated" json:"updatedAt"`
	DeletedAt            int64  `json:"deletedAt,omitempty"`
}

func (Book) TableName() string { return "books" }

// BookMember 账本成员（退出为软删，保留历史记录）
type BookMember struct {
	Id            int64 `xorm:"pk autoincr" json:"id"`
	BookId        int64 `xorm:"notnull" json:"bookId"`
	Uid           int64 `xorm:"notnull" json:"uid"`
	Role          int8  `xorm:"notnull" json:"role"`
	CanViewOthers bool  `xorm:"notnull default 1" json:"canViewOthers"`
	Deleted       bool  `xorm:"notnull default 0" json:"deleted"`
	CreatedAt     int64 `xorm:"created" json:"createdAt"`
	UpdatedAt     int64 `xorm:"updated" json:"updatedAt"`
	DeletedAt     int64 `json:"deletedAt,omitempty"`
}

func (BookMember) TableName() string { return "book_members" }

// BookVersion 账本级全局版本号（增量同步的水位）
type BookVersion struct {
	BookId  int64 `xorm:"pk" json:"bookId"`
	Version int64 `xorm:"notnull default 0" json:"version"`
}

func (BookVersion) TableName() string { return "book_versions" }

// Invite 邀请码（关闭公开注册后的唯一入户通道，见 PRD §12.1）
type Invite struct {
	Id            int64  `xorm:"pk autoincr" json:"id"`
	BookId        int64  `xorm:"notnull" json:"bookId"`
	Code          string `xorm:"varchar(64) notnull unique" json:"code"`
	Role          int8   `xorm:"notnull default 3" json:"role"`
	CreatedBy     int64  `xorm:"notnull" json:"createdBy"`
	ExpiresAt     int64  `xorm:"notnull" json:"expiresAt"`
	UsedAt        int64  `json:"usedAt,omitempty"`
	UsedBy        int64  `xorm:"default 0" json:"usedBy"`
	BoundEmail    string `xorm:"varchar(128) notnull default ''" json:"boundEmail"`
	BoundUsername string `xorm:"varchar(64) notnull default ''" json:"boundUsername"`
	Revoked       bool   `xorm:"notnull default 0" json:"revoked"`
	CreatedAt     int64  `xorm:"created" json:"createdAt"`
}

func (Invite) TableName() string { return "invites" }
