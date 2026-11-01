package service

// 长期 API Token（PAT）服务（WBS 6.6 / PRD §187 §296 §370）。
//
// 与会话（token_sessions）的分工：会话解决"多端登录、按设备撤销"；
// PAT 解决"快捷指令/脚本调用 API"，不随会话过期、可单独撤销。
// 两者都是"凭证的事实来源"，故同样挂在 AuthService 上（与会话同构）。
//
// 安全口径：
//   - 明文只出现一次（Create 返回值），库里只有 SHA-256 哈希；
//   - 明文前缀 `flt_` 是鉴权中间件的分流标记，同时让误贴到公开场合的
//     Token 可被扫描器识别（同 GitHub `ghp_` / Stripe `sk_live_` 惯例）；
//   - 校验顺序与 VerifyWithClaims 一致：先哈希查行（撤销/过期 → 401），
//     再确认账号存在且未禁用；
//   - last_used_at 回写按 1 小时粒度节流（与会话 touchLastSeen 同惯用法），
//     避免高频脚本每请求都写库。
//
// 管理接口（生成/列表/撤销）按 uid 过滤，跨用户 404 不泄露存在性。

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"fastledger/internal/model"

	"xorm.io/xorm"
)

// PATPrefix 长期 API Token 明文前缀（中间件按它分流到本表校验）
const PATPrefix = "flt_"

// PAT 业务错误（mapSvcErr 统一映射 HTTP 状态码）
var (
	// ErrPATNotFound Token 不存在（含跨用户访问——不区分，防存在性泄露）
	ErrPATNotFound = errors.New("Token 不存在")
)

// PATTokenNameMax 备注名最大长度（与 0008 列定义一致）
const PATTokenNameMax = 64

// PATService 长期 API Token 服务（复用 AuthService 的 engine）
type PATService struct {
	engine *xorm.Engine
}

// NewPATService 构造 PAT 服务
func NewPATService(engine *xorm.Engine) *PATService {
	return &PATService{engine: engine}
}

// PATItem 列表项（**绝不含哈希**）
type PATItem struct {
	Id         int64  `json:"id"`
	Name       string `json:"name"`
	CreatedAt  int64  `json:"createdAt"`
	ExpiresAt  int64  `json:"expiresAt"`
	RevokedAt  int64  `json:"revokedAt"`
	LastUsedAt int64  `json:"lastUsedAt"`
}

// Create 生成新 Token，返回**仅此一次**的明文。同用户有效 Token 数量上限 20，
// 防止失控循环生成刷表。
func (s *PATService) Create(uid int64, name string) (string, *PATItem, error) {
	if len(name) > PATTokenNameMax {
		return "", nil, fmt.Errorf("%w：备注不超过 %d 字符", ErrInvalidArgument, PATTokenNameMax)
	}
	// 同用户有效 Token 数量上限 20（xorm 的 Count(*int64) 在本版本不可用，
	// 与 report.go 的 SUM 同惯用法：SQL 聚合扫进 struct）
	var cnt struct {
		N int64 `xorm:"n"`
	}
	if _, err := s.engine.SQL(
		"SELECT COUNT(1) AS n FROM auth_tokens WHERE uid = ? AND revoked_at = 0", uid,
	).Get(&cnt); err != nil {
		return "", nil, err
	}
	if cnt.N >= 20 {
		return "", nil, fmt.Errorf("%w：有效 Token 数量已达上限（20），请先撤销不再使用的 Token",
			ErrInvalidArgument)
	}

	// 明文：flt_ + 32 随机字节的 hex（64 字符）；哈希入库
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	plaintext := PATPrefix + hex.EncodeToString(raw)
	sum := sha256.Sum256([]byte(plaintext))

	now := time.Now().Unix()
	row := &model.AuthToken{
		Uid:       uid,
		TokenHash: hex.EncodeToString(sum[:]),
		Name:      name,
		CreatedAt: now,
	}
	if _, err := s.engine.Insert(row); err != nil {
		return "", nil, err
	}
	return plaintext, &PATItem{
		Id: row.Id, Name: row.Name, CreatedAt: row.CreatedAt,
		ExpiresAt: row.ExpiresAt, RevokedAt: row.RevokedAt, LastUsedAt: row.LastUsedAt,
	}, nil
}

// List 列出本人全部 Token（含已撤销——列表语义是"管理"，撤销状态需要可见）
func (s *PATService) List(uid int64) ([]PATItem, error) {
	var rows []model.AuthToken
	if err := s.engine.Where("uid = ?", uid).OrderBy("id DESC").Find(&rows); err != nil {
		return nil, err
	}
	out := make([]PATItem, 0, len(rows))
	for _, r := range rows {
		out = append(out, PATItem{
			Id: r.Id, Name: r.Name, CreatedAt: r.CreatedAt,
			ExpiresAt: r.ExpiresAt, RevokedAt: r.RevokedAt, LastUsedAt: r.LastUsedAt,
		})
	}
	return out, nil
}

// Revoke 撤销本人指定 Token（跨用户/不存在 → ErrPATNotFound，404）
func (s *PATService) Revoke(uid, id int64) error {
	res, err := s.engine.Where("id = ? AND uid = ? AND revoked_at = 0", id, uid).
		Cols("revoked_at").Update(&model.AuthToken{RevokedAt: time.Now().Unix()})
	if err != nil {
		return err
	}
	if res == 0 {
		// 可能是不存在、跨用户、或已撤销——统一按不存在处理（幂等且不泄露状态）
		return ErrPATNotFound
	}
	return nil
}

// VerifyPAT 用明文 Token 鉴权（WS 场景用 VerifyPATWithExpiry 取连接上限）。
// 撤销/过期 → ErrTokenRevoked（复用会话的 401 文案口径"已撤销或已失效"）；
// 账号删除/禁用 → 与会话同错误。命中后节流回写 last_used_at。
func (s *PATService) VerifyPAT(plaintext string) (*model.User, error) {
	_, u, err := s.VerifyPATWithExpiry(plaintext)
	return u, err
}

// VerifyPATWithExpiry 同 VerifyPAT，额外返回 Token 的过期时刻（expires_at；
// 0 = 永不过期）。WS 握手用它作连接生命周期上限——与 JWT exp 同一契约
// （GPT-R51：连接不得比凭证活得久）。
func (s *PATService) VerifyPATWithExpiry(plaintext string) (int64, *model.User, error) {
	sum := sha256.Sum256([]byte(plaintext))
	hash := hex.EncodeToString(sum[:])

	var row model.AuthToken
	has, err := s.engine.Where("token_hash = ?", hash).Get(&row)
	if err != nil {
		return 0, nil, err
	}
	if !has {
		return 0, nil, ErrBadCredentials
	}
	now := time.Now().Unix()
	if row.RevokedAt > 0 {
		return 0, nil, ErrTokenRevoked
	}
	if row.ExpiresAt > 0 && row.ExpiresAt <= now {
		return 0, nil, ErrTokenRevoked
	}

	var u model.User
	uHas, err := s.engine.ID(row.Uid).Where("deleted = 0").Get(&u)
	if err != nil {
		return 0, nil, err
	}
	if !uHas {
		return 0, nil, ErrBadCredentials
	}
	if u.Disabled {
		return 0, nil, ErrAccountDisabled
	}

	// 1 小时粒度回写最近使用（best-effort，失败不影响鉴权）
	if row.LastUsedAt == 0 || now-row.LastUsedAt >= 3600 {
		_, _ = s.engine.ID(row.Id).Cols("last_used_at").Update(&model.AuthToken{LastUsedAt: now})
	}
	return row.ExpiresAt, &u, nil
}

// IsPAT 判断明文是否为 PAT 格式（鉴权中间件分流用；JWT 以 "eyJ" 开头，天然互斥）
func IsPAT(token string) bool {
	return len(token) > len(PATPrefix) && token[:len(PATPrefix)] == PATPrefix
}
