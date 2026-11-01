package model

// TokenSession 多端会话（token_sessions，WBS 5.10 / PRD §10.10）。
//
// 每个签发过的 JWT（按 jti）对应一行：登录签发时与令牌【同事务】写入，
// 撤销时写 revoked_at，过期/已撤销超保留期的行由 db purge 回收。
// VerifyWithClaims 强校验"该 jti 存在且未撤销、未过期"，因此 0007 之前
// 签发的存量令牌升级后一律失效（用户重新登录即可，属预期）。
type TokenSession struct {
	Id         int64  `xorm:"pk autoincr"`
	Jti        string `xorm:"varchar(64) notnull unique"`
	Uid        int64  `xorm:"notnull"`
	CreatedAt  int64  `xorm:"notnull"`
	ExpiresAt  int64  `xorm:"notnull"`
	RevokedAt  int64  `xorm:"notnull default 0"`
	UserAgent  string `xorm:"varchar(255) notnull default ''"`
	LastSeenAt int64  `xorm:"notnull default 0"`
}

func (TokenSession) TableName() string { return "token_sessions" }
