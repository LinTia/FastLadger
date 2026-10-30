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
type User struct {
	Id              int64  `xorm:"pk autoincr"`
	Username        string `xorm:"varchar(64) notnull unique"`
	PasswordHash    string `xorm:"varchar(255) notnull"`
	Email           string `xorm:"varchar(128) notnull default ''"`
	IsInstanceAdmin bool   `xorm:"notnull default 0"`
	Disabled        bool   `xorm:"notnull default 0"`
	Deleted         bool   `xorm:"notnull default 0"`
	CreatedAt       int64  `xorm:"created"`
	UpdatedAt       int64  `xorm:"updated"`
	DeletedAt       int64
}

func (User) TableName() string { return "users" }

// Book 账本
type Book struct {
	Id                   int64  `xorm:"pk autoincr"`
	Name                 string `xorm:"varchar(64) notnull"`
	OwnerUid             int64  `xorm:"notnull"`
	BookType             int8   `xorm:"notnull default 1"`
	DefaultCurrency      string `xorm:"varchar(8) notnull default 'CNY'"`
	MembersCanViewOthers bool   `xorm:"notnull default 1"`
	Deleted              bool   `xorm:"notnull default 0"`
	CreatedAt            int64  `xorm:"created"`
	UpdatedAt            int64  `xorm:"updated"`
	DeletedAt            int64
}

func (Book) TableName() string { return "books" }

// BookMember 账本成员（退出为软删，保留历史记录）
type BookMember struct {
	Id            int64 `xorm:"pk autoincr"`
	BookId        int64 `xorm:"notnull"`
	Uid           int64 `xorm:"notnull"`
	Role          int8  `xorm:"notnull"`
	CanViewOthers bool  `xorm:"notnull default 1"`
	Deleted       bool  `xorm:"notnull default 0"`
	CreatedAt     int64 `xorm:"created"`
	UpdatedAt     int64 `xorm:"updated"`
	DeletedAt     int64
}

func (BookMember) TableName() string { return "book_members" }

// BookVersion 账本级全局版本号（增量同步的水位）
type BookVersion struct {
	BookId  int64 `xorm:"pk"`
	Version int64 `xorm:"notnull default 0"`
}

func (BookVersion) TableName() string { return "book_versions" }

// Invite 邀请码（关闭公开注册后的唯一入户通道，见 PRD §12.1）
type Invite struct {
	Id            int64  `xorm:"pk autoincr"`
	BookId        int64  `xorm:"notnull"`
	Code          string `xorm:"varchar(64) notnull unique"`
	Role          int8   `xorm:"notnull default 3"`
	CreatedBy     int64  `xorm:"notnull"`
	ExpiresAt     int64  `xorm:"notnull"`
	UsedAt        int64
	UsedBy        int64  `xorm:"default 0"`
	BoundEmail    string `xorm:"varchar(128) notnull default ''"`
	BoundUsername string `xorm:"varchar(64) notnull default ''"`
	Revoked       bool   `xorm:"notnull default 0"`
	CreatedAt     int64  `xorm:"created"`
}

func (Invite) TableName() string { return "invites" }
