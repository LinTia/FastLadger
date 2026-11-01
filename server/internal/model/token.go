package model

// AuthToken 长期 API Token（auth_tokens，WBS 6.6 / PRD §187 §296 §370）。
//
// 与 token_sessions（登录会话）互补：会话随登录签发、按设备撤销；
// 本表是"不随会话过期"的长期凭证，供快捷指令/脚本调用 API 使用。
//
// 安全口径：**只存 SHA-256 哈希，不存明文**——PRD §370 的 token 列落地为
// token_hash，明文仅在创建响应中出现一次（同 GitHub PAT 语义）。
// 前缀 `flt_` 是中间件分流标记（与 JWT `eyJ` 开头互斥）。
type AuthToken struct {
	Id         int64  `xorm:"pk autoincr" json:"id"`
	Uid        int64  `xorm:"notnull" json:"uid"`
	TokenHash  string `xorm:"varchar(64) notnull unique" json:"-"`
	Name       string `xorm:"varchar(64) notnull default ''" json:"name"`
	CreatedAt  int64  `xorm:"notnull" json:"createdAt"`
	ExpiresAt  int64  `xorm:"notnull default 0" json:"expiresAt"` // 0 = 永不过期
	RevokedAt  int64  `xorm:"notnull default 0" json:"revokedAt"` // 0 = 未撤销
	LastUsedAt int64  `xorm:"'last_used_at' notnull default 0" json:"lastUsedAt"`
}

func (AuthToken) TableName() string { return "auth_tokens" }
