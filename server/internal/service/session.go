package service

// 多端会话（token_sessions）的持久化 / 校验 / 查询 / 撤销 / 清理
// （R52，WBS 5.10 / PRD §4.1 §10.10）。
//
// 职责边界：
//   - 签发侧：insertSession —— 与令牌签发【同事务】写会话行（auth.go LoginFromIP）；
//   - 校验侧：checkSessionActive —— VerifyWithClaims 强校验 jti（缺失/撤销/过期即拒绝）；
//   - 查询侧：ListSessions —— 本人会话列表（HTTP 层据此标记 current）；
//   - 撤销侧：RevokeSession / RevokeOtherSessions —— 只能操作本人会话，跨用户 404；
//   - 清理侧：PurgeExpiredSessions —— 过期 + 已撤销超保留期的行（db purge 接线）。
//
// 设计取舍（已在《落地方案》§5.5 明示）：
//   - 每次鉴权一次按 jti 的索引点查（jti 有 UNIQUE 索引，SQLite 单机亚毫秒级）：
//     这是"撤销即时生效"的必要代价，换取单设备撤销能力；
//   - last_seen 按 1 小时粒度回写：让"最近活跃时间"有意义，同时把活跃会话的
//     写放大限制在每小时每会话一次（MaxOpenConns=1，写要省着用）；
//   - 状态未知 fail-closed：DB 故障时宁可 401 让用户重登，也不在"会话状态未知"
//     的情况下放行。

import (
	"errors"
	"fmt"
	"log"

	"fastledger/internal/model"

	"xorm.io/xorm"
)

const (
	// MaxUserAgentLen 会话展示用的 User-Agent 截断长度（按字符计，对齐 schema VARCHAR(255)）
	MaxUserAgentLen = 255
	// SessionRetentionDays 已撤销/已过期会话行的保留天数（db purge 回收窗口，R52 口径）
	SessionRetentionDays = 7
	// sessionLastSeenInterval last_seen 回写的最小间隔（秒）
	sessionLastSeenInterval = 3600
)

// ErrSessionNotFound 指定会话不存在（或不属于当前用户）。
//
// 两种情况共用一个错误、映射同一个 HTTP 404：撤销接口不区分"没有这个会话"
// 与"这是别人的会话"，不泄露会话存在性（与"账号不存在 vs 口令错误"同口径）。
var ErrSessionNotFound = errors.New("会话不存在")

// insertSession 在【调用方的事务】内写一行会话（登录签发令牌的同事务步骤）。
//
// userAgent 为客户端自报的 User-Agent，仅展示用：按字符截断到 MaxUserAgentLen，
// 超长不报错（UA 不可信也不该影响登录成败）。
func insertSession(sess *xorm.Session, uid int64, claims *Claims, userAgent string, now int64) error {
	if sess == nil {
		return fmt.Errorf("%w：会话事务缺失", ErrAuthUnavailable)
	}
	if claims == nil || claims.Jti == "" {
		// jti 是撤销能力的锚点，缺了就不能发令牌（fail-closed）
		return fmt.Errorf("%w：令牌缺少 jti，拒绝签发", ErrAuthUnavailable)
	}
	if claims.Exp <= now {
		return fmt.Errorf("%w：令牌 exp 非法（exp=%d now=%d）", ErrAuthUnavailable, claims.Exp, now)
	}
	row := &model.TokenSession{
		Jti:        claims.Jti,
		Uid:        uid,
		CreatedAt:  now,
		ExpiresAt:  claims.Exp,
		UserAgent:  truncateRunes(userAgent, MaxUserAgentLen),
		LastSeenAt: now,
	}
	if _, err := sess.Insert(row); err != nil {
		return fmt.Errorf("会话写入失败：%w", err)
	}
	return nil
}

// checkSessionActive 强校验 jti 对应的会话当前可用，返回会话行（供 last_seen 回写）。
//
// 拒绝语义（前三种 → ErrTokenRevoked，HTTP 401 + 可区分文案"会话已撤销或已失效"）：
//   - 无会话行：0007 之前签发的存量令牌（升级后失效，预期行为）或伪造 jti；
//   - revoked_at > 0：已被用户主动撤销；
//   - expires_at <= now：会话行自身过期（与 JWT exp 双保险，防表值被改动）。
//
// DB 查询失败 → ErrAuthUnavailable（依赖不可用，与凭证错误严格区分，R46 口径）；
// HTTP 层对二者都返回 401，但错误链保留真实原因供服务端日志排查。
func (s *AuthService) checkSessionActive(claims *Claims, now int64) (*model.TokenSession, error) {
	var row model.TokenSession
	has, err := s.engine.Where("jti = ?", claims.Jti).Get(&row)
	if err != nil {
		return nil, fmt.Errorf("%w：查询会话失败：%w", ErrAuthUnavailable, err)
	}
	if !has {
		return nil, fmt.Errorf("%w：会话不存在（签发于升级前或已被清理）", ErrTokenRevoked)
	}
	if row.Uid != claims.Uid {
		// jti 与 uid 不匹配只可能是数据异常或伪造：一律拒绝，绝不放行
		return nil, fmt.Errorf("%w：会话与用户不匹配", ErrTokenRevoked)
	}
	if row.RevokedAt > 0 {
		return nil, fmt.Errorf("%w：会话已被撤销", ErrTokenRevoked)
	}
	if row.ExpiresAt <= now {
		return nil, fmt.Errorf("%w：会话已过期", ErrTokenRevoked)
	}
	return &row, nil
}

// touchLastSeen 按 1 小时粒度回写 last_seen_at（best-effort：失败只记服务端日志，
// 绝不影响本次鉴权结果——活跃时间展示属于体验增强，不值得为它引入 5xx）。
//
// UPDATE 带 last_seen_at <= ? 条件：并发请求下只有持旧值的那一次写会生效，
// 避免同一秒内 N 个请求造成 N 次无谓的行写（MaxOpenConns=1，写是全局排队资源）。
func (s *AuthService) touchLastSeen(claims *Claims, row *model.TokenSession, now int64) {
	if row == nil || now-row.LastSeenAt < sessionLastSeenInterval {
		return
	}
	if _, err := s.engine.Exec(
		"UPDATE token_sessions SET last_seen_at = ? WHERE jti = ? AND last_seen_at <= ?",
		now, claims.Jti, row.LastSeenAt); err != nil {
		log.Printf("[auth] 会话活跃时间回写失败（不影响本次鉴权）jti=%s err=%v", claims.Jti, err)
	}
}

// SessionItem 会话列表条目（HTTP 层 DTO 的数据源，不含任何凭据材料）。
type SessionItem struct {
	Id         int64  // 行 ID
	Jti        string // 会话标识（撤销接口的路径参数；jti 本身不是凭证）
	CreatedAt  int64  // 签发时刻
	ExpiresAt  int64  // 过期时刻
	RevokedAt  int64  // 撤销时刻（0 = 未撤销）
	LastSeenAt int64  // 最近活跃时刻
	UserAgent  string // 登录时客户端自报的 User-Agent
}

// ListSessions 列出某用户的会话。
//
// includeRevoked=false（默认口径）：排除已撤销与已过期的行——列表语义是
// "当前有效的登录凭证"；历史行交给 db purge 按保留期回收。
// includeRevoked=true：保留全部行（排障/核对撤销时刻用）。
// 排序：最近活跃优先，其次按签发时间倒序，末位以 id DESC 收口。
//
// 为什么必须有 id 这个 tie-breaker（race 模式实测暴露的真缺陷）：created_at /
// last_seen_at 都是【秒级】时间戳，同一用户在 1 秒内登录两台设备时两个键都会
// 并列，SQL 此时返回顺序未定义（取决于走哪个索引与扫描方向）——表现为"同一份
// 数据两次查询顺序不同"（race 构建更容易触发）、客户端刷新列表时同秒会话来回
// 跳动。id 是自增主键、全局唯一且与签发顺序一致，作为末位排序键即可把顺序
// 完全钉死。
func (s *AuthService) ListSessions(uid int64, includeRevoked bool, now int64) ([]SessionItem, error) {
	if uid <= 0 {
		return nil, fmt.Errorf("%w：uid 非法（uid=%d）", ErrInvalidArgument, uid)
	}
	if now <= 0 {
		return nil, fmt.Errorf("%w：当前时间非法（now=%d）", ErrInvalidArgument, now)
	}
	q := s.engine.Where("uid = ?", uid)
	if !includeRevoked {
		q = q.And("revoked_at = 0").And("expires_at > ?", now)
	}
	var rows []model.TokenSession
	if err := q.Desc("last_seen_at").Desc("created_at").Desc("id").Find(&rows); err != nil {
		return nil, fmt.Errorf("查询会话列表失败：%w", err)
	}
	items := make([]SessionItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, SessionItem{
			Id: r.Id, Jti: r.Jti, CreatedAt: r.CreatedAt, ExpiresAt: r.ExpiresAt,
			RevokedAt: r.RevokedAt, LastSeenAt: r.LastSeenAt, UserAgent: r.UserAgent,
		})
	}
	return items, nil
}

// RevokeSession 撤销某用户名下的指定会话，返回撤销后的会话行。
//
// 语义（PRD §10.10）：
//   - 会话不存在 / 不属于该 uid → ErrSessionNotFound（HTTP 404，两者不区分，防探测）；
//   - 已撤销 → 幂等成功（原样返回行，不改动撤销时刻）；
//   - 允许撤销"当前正在使用"的会话（等价于本机登出，撤销后当前请求的下一个
//     请求即 401）。
//
// 调用方（HTTP 层）在撤销成功后负责 KickJti 主动断开该会话的 WS 连接。
func (s *AuthService) RevokeSession(uid int64, jti string, now int64) (*model.TokenSession, error) {
	if uid <= 0 {
		return nil, fmt.Errorf("%w：uid 非法（uid=%d）", ErrInvalidArgument, uid)
	}
	if jti == "" {
		return nil, fmt.Errorf("%w：jti 不能为空", ErrInvalidArgument)
	}
	if now <= 0 {
		return nil, fmt.Errorf("%w：当前时间非法（now=%d）", ErrInvalidArgument, now)
	}
	var row model.TokenSession
	has, err := s.engine.Where("jti = ? AND uid = ?", jti, uid).Get(&row)
	if err != nil {
		return nil, fmt.Errorf("查询会话失败：%w", err)
	}
	if !has {
		return nil, ErrSessionNotFound
	}
	if row.RevokedAt > 0 {
		return &row, nil // 幂等：已撤销的会话保持原撤销时刻
	}
	if _, err := s.engine.Exec(
		"UPDATE token_sessions SET revoked_at = ? WHERE jti = ? AND uid = ? AND revoked_at = 0",
		now, jti, uid); err != nil {
		return nil, fmt.Errorf("撤销会话失败：%w", err)
	}
	row.RevokedAt = now
	return &row, nil
}

// RevokeOtherSessions 撤销某用户除 keepJti（当前会话）之外的全部【有效】会话，
// 返回本次实际撤销的 jti 清单（调用方据此逐个 KickJti 踢 WS 连接）。
//
// "有效"= 未撤销且未过期：已撤销/已过期的行没有活跃凭证，重复处理只会
// 抬高撤销时刻、污染列表口径。keepJti 为空串时撤销该用户全部有效会话
// （当前无会话上下文的调用方，如 CLI，可用此形态）。
func (s *AuthService) RevokeOtherSessions(uid int64, keepJti string, now int64) ([]string, error) {
	if uid <= 0 {
		return nil, fmt.Errorf("%w：uid 非法（uid=%d）", ErrInvalidArgument, uid)
	}
	if now <= 0 {
		return nil, fmt.Errorf("%w：当前时间非法（now=%d）", ErrInvalidArgument, now)
	}
	q := s.engine.Where("uid = ?", uid).And("revoked_at = 0").And("expires_at > ?", now)
	if keepJti != "" {
		q = q.And("jti <> ?", keepJti)
	}
	var rows []model.TokenSession
	if err := q.Find(&rows); err != nil {
		return nil, fmt.Errorf("查询待撤销会话失败：%w", err)
	}
	jtis := make([]string, 0, len(rows))
	for _, r := range rows {
		// 逐条撤销（会话数很小：单人设备数个量级）；条件里带 revoked_at = 0，
		// 并发撤销下重复执行也不会改变已撤销行的撤销时刻
		if _, err := s.engine.Exec(
			"UPDATE token_sessions SET revoked_at = ? WHERE jti = ? AND uid = ? AND revoked_at = 0",
			now, r.Jti, uid); err != nil {
			return jtis, fmt.Errorf("撤销会话 %s 失败：%w", r.Jti, err)
		}
		jtis = append(jtis, r.Jti)
	}
	return jtis, nil
}

// PurgeExpiredSessions 回收会话行（db purge 接线，R52）。
//
// 清理对象（cutoff = now - SessionRetentionDays 天，与审计/回收站同一套
// retentionCutoff 溢出防护口径）：
//   - 已过期：expires_at <= cutoff；
//   - 已撤销且撤销时刻超过保留期：revoked_at > 0 且 revoked_at <= cutoff。
//
// 未过期的有效会话不受影响（WHERE 条件保证）；非法入参不产生任何 DELETE。
// 返回删除的行数。
func PurgeExpiredSessions(engine *xorm.Engine, now int64) (int64, error) {
	if engine == nil {
		return 0, fmt.Errorf("%w：engine 缺失", ErrInvalidArgument)
	}
	if now <= 0 {
		return 0, fmt.Errorf("%w：当前时间非法（now=%d）", ErrInvalidArgument, now)
	}
	// SessionRetentionDays 是固定常量（7），不开放配置：会话行的价值远低于审计，
	// 配置面越少越好；daysToSeconds 在此上限下不可能溢出
	span, err := daysToSeconds(SessionRetentionDays)
	if err != nil {
		return 0, err
	}
	cutoff := now - span
	res, err := engine.Exec(
		"DELETE FROM token_sessions WHERE expires_at <= ? OR (revoked_at > 0 AND revoked_at <= ?)",
		cutoff, cutoff)
	if err != nil {
		return 0, fmt.Errorf("会话清理失败：%w", err)
	}
	return res.RowsAffected()
}
