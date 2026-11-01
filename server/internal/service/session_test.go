package service

// R52 多端会话的服务层回归测试（真实 SQLite，需 CGO；由 CI 与服务器 run_tests.sh 执行）。
//
// 覆盖 WBS 5.10 的四条硬约束：
//   ① 签发即落库：Login 写 token_sessions，且"会话写失败 ⇒ 不签发令牌"；
//   ② Verify 强校验：无会话行 / 已撤销 / 会话行过期 → ErrTokenRevoked（HTTP 401）；
//   ③ 撤销边界：跨用户 404（ErrSessionNotFound）、幂等、revoke-others 保留当前会话；
//   ④ 清理：过期 + 已撤销超保留期的行被回收，有效会话不受影响。
//
// 复用 tx_integration_test.go 的 newTestEngine 与 audit_integration_test.go 的 testJWTSecret。

import (
	"errors"
	"testing"
	"time"

	"fastledger/internal/model"

	"xorm.io/xorm"
)

// newSessionAuth 一个带认证服务的干净环境（全量迁移已应用）
func newSessionAuth(t *testing.T) (*sessionTestEnv, *AuthService) {
	t.Helper()
	engine := newTestEngine(t)
	as := NewAuthService(engine, testJWTSecret, time.Hour)
	return &sessionTestEnv{t: t, engine: engine}, as
}

// sessionTestEnv 轻包装：避免与既有 testEnv（账本种子数据）耦合，这里只需要引擎。
type sessionTestEnv struct {
	t      *testing.T
	engine *xorm.Engine
}

func (e *sessionTestEnv) register(t *testing.T, as *AuthService, name string) {
	t.Helper()
	if _, err := as.Register(name, "password123"); err != nil {
		t.Fatalf("注册 %s 失败：%v", name, err)
	}
}

func (e *sessionTestEnv) login(t *testing.T, as *AuthService, name string) (string, *Claims) {
	t.Helper()
	tok, err := as.Login(name, "password123")
	if err != nil {
		t.Fatalf("登录 %s 失败：%v", name, err)
	}
	claims, err := ParseJWT(testJWTSecret, tok)
	if err != nil {
		t.Fatalf("解析令牌失败：%v", err)
	}
	return tok, claims
}

func (e *sessionTestEnv) sessionRow(t *testing.T, jti string) model.TokenSession {
	t.Helper()
	var row model.TokenSession
	has, err := e.engine.Where("jti = ?", jti).Get(&row)
	if err != nil {
		t.Fatalf("读会话行失败：%v", err)
	}
	if !has {
		t.Fatalf("会话行不存在 jti=%s", jti)
	}
	return row
}

// TestLoginWritesSessionRow ①登录必须落一行会话（jti/uid/exp 与令牌一致，UA 透传）。
func TestLoginWritesSessionRow(t *testing.T) {
	e, as := newSessionAuth(t)
	e.register(t, as, "sess-login-user")

	// 直接走 LoginFromIP：同时验证 UA 透传（HTTP 层从请求头原样带来）
	tok, err := as.LoginFromIP("sess-login-user", "password123", "203.0.113.10", "FastLedger-Android/1.0")
	if err != nil {
		t.Fatalf("登录失败：%v", err)
	}
	claims, err := ParseJWT(testJWTSecret, tok)
	if err != nil {
		t.Fatalf("解析令牌失败：%v", err)
	}
	row := e.sessionRow(t, claims.Jti)
	if row.Uid != claims.Uid {
		t.Fatalf("会话 uid 不符：期望 %d，实际 %d", claims.Uid, row.Uid)
	}
	if row.ExpiresAt != claims.Exp {
		t.Fatalf("会话 exp 不符：期望 %d，实际 %d", claims.Exp, row.ExpiresAt)
	}
	if row.RevokedAt != 0 {
		t.Fatalf("新会话不应已被撤销：revoked_at=%d", row.RevokedAt)
	}
	if row.UserAgent != "FastLedger-Android/1.0" {
		t.Fatalf("User-Agent 未透传：%q", row.UserAgent)
	}
	if row.LastSeenAt <= 0 {
		t.Fatal("新会话的 last_seen_at 应在签发时初始化")
	}
}

// TestLoginSessionFailureMeansNoToken ①同失败语义：会话落库失败 ⇒ 不返回令牌（503 语义）。
//
// 用"表不存在"模拟存储故障——无论故障形态如何，契约都是"拿不到令牌 + ErrAuthUnavailable"。
func TestLoginSessionFailureMeansNoToken(t *testing.T) {
	e, as := newSessionAuth(t)
	e.register(t, as, "sess-broken-store")
	if _, err := e.engine.Exec("DROP TABLE token_sessions"); err != nil {
		t.Fatalf("构造故障失败：%v", err)
	}
	tok, err := as.Login("sess-broken-store", "password123")
	if err == nil {
		t.Fatal("会话落库失败时必须拒绝签发令牌")
	}
	if tok != "" {
		t.Fatalf("不得返回半成品令牌：%q", tok)
	}
	if !errors.Is(err, ErrAuthUnavailable) {
		t.Fatalf("应归类为 ErrAuthUnavailable（503），实际：%v", err)
	}
}

// TestVerifyRejectsMissingSessionRow ②存量令牌（无会话行）升级后必须 401。
func TestVerifyRejectsMissingSessionRow(t *testing.T) {
	e, as := newSessionAuth(t)
	e.register(t, as, "sess-legacy-user")
	tok, claims := e.login(t, as, "sess-legacy-user")

	if _, err := e.engine.Exec("DELETE FROM token_sessions WHERE jti = ?", claims.Jti); err != nil {
		t.Fatalf("删除会话行失败：%v", err)
	}
	if _, err := as.Verify(tok); !errors.Is(err, ErrTokenRevoked) {
		t.Fatalf("无会话行的令牌应被拒绝（ErrTokenRevoked），实际：%v", err)
	}
}

// TestVerifyRejectsRevokedSession ②已撤销会话必须 401（单设备撤销的核心语义）。
func TestVerifyRejectsRevokedSession(t *testing.T) {
	e, as := newSessionAuth(t)
	e.register(t, as, "sess-revoke-user")
	tok, claims := e.login(t, as, "sess-revoke-user")

	if _, err := as.RevokeSession(claims.Uid, claims.Jti, time.Now().Unix()); err != nil {
		t.Fatalf("撤销失败：%v", err)
	}
	if _, err := as.Verify(tok); !errors.Is(err, ErrTokenRevoked) {
		t.Fatalf("已撤销会话的令牌应被拒绝（ErrTokenRevoked），实际：%v", err)
	}
}

// TestVerifyRejectsExpiredSessionRow ②会话行自身过期必须 401（与 JWT exp 双保险）。
func TestVerifyRejectsExpiredSessionRow(t *testing.T) {
	e, as := newSessionAuth(t)
	e.register(t, as, "sess-expired-row")
	tok, claims := e.login(t, as, "sess-expired-row")

	// JWT 本身未过期（ttl=1h），只把表值改过去——必须仍被拒
	if _, err := e.engine.Exec(
		"UPDATE token_sessions SET expires_at = ? WHERE jti = ?",
		time.Now().Unix()-10, claims.Jti); err != nil {
		t.Fatalf("改写会话过期时间失败：%v", err)
	}
	if _, err := as.Verify(tok); !errors.Is(err, ErrTokenRevoked) {
		t.Fatalf("会话行过期应被拒绝（ErrTokenRevoked），实际：%v", err)
	}
}

// TestRevokeSessionCrossUserIsNotFound ③跨用户撤销 → ErrSessionNotFound（HTTP 404），
// 且被撤销目标原封不动。
func TestRevokeSessionCrossUserIsNotFound(t *testing.T) {
	e, as := newSessionAuth(t)
	e.register(t, as, "sess-owner-a")
	e.register(t, as, "sess-owner-b")

	_, claimsA := e.login(t, as, "sess-owner-a")
	tokB, _ := e.login(t, as, "sess-owner-b")

	if _, err := as.RevokeSession(claimsA.Uid+1000000, claimsA.Jti, time.Now().Unix()); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("不存在的 uid 应返回 ErrSessionNotFound，实际：%v", err)
	}
	// B 撤销 A 的会话：按 B 的 uid 查不到 → 404，A 的会话不受影响
	uidB := mustUID(t, e, "sess-owner-b")
	if _, err := as.RevokeSession(uidB, claimsA.Jti, time.Now().Unix()); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("跨用户撤销应返回 ErrSessionNotFound，实际：%v", err)
	}
	if _, _, err := as.VerifyWithClaims(tokB); err != nil {
		t.Fatalf("对照路径不应受影响：%v", err)
	}
	row := e.sessionRow(t, claimsA.Jti)
	if row.RevokedAt != 0 {
		t.Fatalf("跨用户撤销不得改动目标会话：revoked_at=%d", row.RevokedAt)
	}
}

// mustUID 按用户名查 uid（测试辅助）
func mustUID(t *testing.T, e *sessionTestEnv, name string) int64 {
	t.Helper()
	var u model.User
	has, err := e.engine.Where("username = ?", name).Get(&u)
	if err != nil || !has {
		t.Fatalf("读用户 %s 失败：err=%v has=%v", name, err, has)
	}
	return u.Id
}

// TestRevokeSessionIdempotent ③重复撤销幂等：不报错、撤销时刻不被刷新。
func TestRevokeSessionIdempotent(t *testing.T) {
	e, as := newSessionAuth(t)
	e.register(t, as, "sess-idem-user")
	_, claims := e.login(t, as, "sess-idem-user")

	now := time.Now().Unix()
	r1, err := as.RevokeSession(claims.Uid, claims.Jti, now)
	if err != nil {
		t.Fatalf("首次撤销失败：%v", err)
	}
	r2, err := as.RevokeSession(claims.Uid, claims.Jti, now+999)
	if err != nil {
		t.Fatalf("重复撤销应幂等成功，实际：%v", err)
	}
	if r2.RevokedAt != r1.RevokedAt {
		t.Fatalf("重复撤销不得刷新撤销时刻：first=%d second=%d", r1.RevokedAt, r2.RevokedAt)
	}
}

// TestRevokeOtherSessionsKeepsCurrent ③"登出其它设备"保留当前会话。
func TestRevokeOtherSessionsKeepsCurrent(t *testing.T) {
	e, as := newSessionAuth(t)
	e.register(t, as, "sess-multi-user")
	tok1, claims1 := e.login(t, as, "sess-multi-user")
	tok2, claims2 := e.login(t, as, "sess-multi-user")

	revoked, err := as.RevokeOtherSessions(claims1.Uid, claims1.Jti, time.Now().Unix())
	if err != nil {
		t.Fatalf("批量撤销失败：%v", err)
	}
	if len(revoked) != 1 || revoked[0] != claims2.Jti {
		t.Fatalf("应只撤销设备 2（%s），实际：%v", claims2.Jti, revoked)
	}
	if _, _, err := as.VerifyWithClaims(tok1); err != nil {
		t.Fatalf("当前会话不应受影响：%v", err)
	}
	if _, _, err := as.VerifyWithClaims(tok2); !errors.Is(err, ErrTokenRevoked) {
		t.Fatalf("其它设备的会话应被撤销（ErrTokenRevoked），实际：%v", err)
	}
}

// TestPurgeExpiredSessions ④清理：过期与撤销超保留期的行被删，有效会话保留。
func TestPurgeExpiredSessions(t *testing.T) {
	e, as := newSessionAuth(t)
	e.register(t, as, "sess-purge-user")
	_, claims := e.login(t, as, "sess-purge-user")
	now := time.Now().Unix()

	// 三类行：有效（登录产生的那行）、早已过期、早已撤销
	seed := []model.TokenSession{
		{Jti: "purge-expired", Uid: claims.Uid, CreatedAt: now - 100000,
			ExpiresAt: now - 100 * 86400, RevokedAt: 0, LastSeenAt: now - 100000},
		{Jti: "purge-revoked-long-ago", Uid: claims.Uid, CreatedAt: now - 100000,
			ExpiresAt: now + 86400, RevokedAt: now - 100 * 86400, LastSeenAt: now - 100000},
	}
	for i := range seed {
		if _, err := e.engine.Insert(&seed[i]); err != nil {
			t.Fatalf("构造数据失败：%v", err)
		}
	}
	n, err := PurgeExpiredSessions(e.engine, now)
	if err != nil {
		t.Fatalf("清理失败：%v", err)
	}
	if n != 2 {
		t.Fatalf("应恰好清理 2 行（过期 1 + 撤销超期 1），实际 %d", n)
	}
	e.sessionRow(t, claims.Jti) // 有效会话必须还在
	var cnt int64
	if _, err := e.engine.SQL("SELECT COUNT(1) FROM token_sessions WHERE jti LIKE 'purge-%'").Get(&cnt); err != nil {
		t.Fatalf("统计失败：%v", err)
	}
	if cnt != 0 {
		t.Fatalf("到期会话应被清空，实际剩 %d 行", cnt)
	}
	// 边界：刚过期/刚撤销（保留期内）的行必须保留
	recent := model.TokenSession{Jti: "purge-recent", Uid: claims.Uid, CreatedAt: now,
		ExpiresAt: now - 1, RevokedAt: now, LastSeenAt: now}
	if _, err := e.engine.Insert(&recent); err != nil {
		t.Fatalf("构造数据失败：%v", err)
	}
	if n, err := PurgeExpiredSessions(e.engine, now); err != nil || n != 0 {
		t.Fatalf("保留期内的行不得被清理：n=%d err=%v", n, err)
	}
}

// TestTouchLastSeenThrottled last_seen 回写按 1 小时粒度：旧值才写，新值不写。
func TestTouchLastSeenThrottled(t *testing.T) {
	e, as := newSessionAuth(t)
	e.register(t, as, "sess-seen-user")
	tok, claims := e.login(t, as, "sess-seen-user")
	now := time.Now().Unix()

	// 把 last_seen 拨回 2 小时前 → Verify 命中后应被推进
	old := now - 7200
	if _, err := e.engine.Exec(
		"UPDATE token_sessions SET last_seen_at = ? WHERE jti = ?", old, claims.Jti); err != nil {
		t.Fatalf("构造旧活跃时间失败：%v", err)
	}
	if _, _, err := as.VerifyWithClaims(tok); err != nil {
		t.Fatalf("Verify 不应失败：%v", err)
	}
	if got := e.sessionRow(t, claims.Jti).LastSeenAt; got <= old {
		t.Fatalf("last_seen 应被推进：old=%d got=%d", old, got)
	}
	// 刚刚回写过（间隔 < 1h）→ 再 Verify 不应再写（值保持不变）
	seen := e.sessionRow(t, claims.Jti).LastSeenAt
	if _, _, err := as.VerifyWithClaims(tok); err != nil {
		t.Fatalf("第二次 Verify 不应失败：%v", err)
	}
	if got := e.sessionRow(t, claims.Jti).LastSeenAt; got != seen {
		t.Fatalf("1 小时内不得重复回写：seen=%d got=%d", seen, got)
	}
}
