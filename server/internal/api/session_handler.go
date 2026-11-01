package api

// 多端会话 HTTP 适配层（R52，WBS 5.10 / PRD §4.1 §10.10）。
//
// 契约（《落地方案》§5.1 追加，全部挂在 JWTAuth 组内）：
//
//	GET    /api/v1/auth/sessions        列出本人会话（默认排除已撤销/已过期；标记 current）
//	DELETE /api/v1/auth/sessions        撤销本人除当前会话外的全部有效会话（"登出其它设备"）
//	DELETE /api/v1/auth/sessions/:jti   撤销本人指定会话（跨用户/不存在 → 404）
//
// 越权口径：service 层所有读写都先按 uid 过滤——跨用户撤销返回 404 且不区分
// "不存在"，不泄露他人会话存在性。撤销成功后立即 KickJti 主动断开对应 WS 连接
// （关闭码 4003）；HTTP 侧无需断连，下一个请求经 JWTAuth 的会话强校验自然 401。
//
// User-Agent 仅作展示（登录时客户端自报，截断到 255），不用于任何鉴权决策。

import (
	"errors"
	"log"
	"net/http"
	"time"

	"fastledger/internal/service"
	"fastledger/internal/ws"

	"github.com/gin-gonic/gin"
)

// sessionHandler 多端会话 handler
type sessionHandler struct {
	auth *service.AuthService
	hub  *ws.Hub
}

// newSessionHandler 构造 handler。hub 为 nil 时回落 DefaultHub：
// 生产 main 经 Deps.Hub 注入 DefaultHub；直构 Deps 的调用方/测试不传时行为一致。
func newSessionHandler(auth *service.AuthService, hub *ws.Hub) *sessionHandler {
	if hub == nil {
		hub = ws.DefaultHub
	}
	return &sessionHandler{auth: auth, hub: hub}
}

// sessionDTO 会话列表/撤销响应条目。
//
// jti 返回全量：客户端撤销指定设备必须携带它；jti 本身不是凭证
// （不能用它登录，只能用于"本人撤销本人会话"，跨用户会被 404 挡住）。
type sessionDTO struct {
	Id         int64  `json:"id"`
	Jti        string `json:"jti"`
	CreatedAt  int64  `json:"createdAt"`
	ExpiresAt  int64  `json:"expiresAt"`
	RevokedAt  int64  `json:"revokedAt"` // 0 = 未撤销
	LastSeenAt int64  `json:"lastSeenAt"`
	UserAgent  string `json:"userAgent"`
	Current    bool   `json:"current"`
}

// toSessionDTO 组装响应条目（current 按"是否为发起本次请求的会话"标记）。
func toSessionDTO(it service.SessionItem, currentJti string) sessionDTO {
	return sessionDTO{
		Id: it.Id, Jti: it.Jti, CreatedAt: it.CreatedAt, ExpiresAt: it.ExpiresAt,
		RevokedAt: it.RevokedAt, LastSeenAt: it.LastSeenAt, UserAgent: it.UserAgent,
		Current: it.Jti == currentJti,
	}
}

// list GET /api/v1/auth/sessions?includeRevoked=1
//
// 默认只返回有效会话（未撤销且未过期）——列表语义是"当前在哪些设备上登录着"；
// includeRevoked=1 供核对撤销时刻/排障，包含全部行。
func (h *sessionHandler) list(c *gin.Context) {
	uid := currentUID(c)
	now := time.Now().Unix()
	items, err := h.auth.ListSessions(uid, c.Query("includeRevoked") == "1", now)
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	current := currentJTI(c)
	dtos := make([]sessionDTO, 0, len(items))
	for _, it := range items {
		dtos = append(dtos, toSessionDTO(it, current))
	}
	ok(c, gin.H{"sessions": dtos})
}

// revokeOne DELETE /api/v1/auth/sessions/:jti
//
// 撤销本人指定会话；允许撤销当前会话（等价于本机登出）。响应带 current 标记，
// 客户端据此知道"撤销的是不是自己正在用的这台"。
func (h *sessionHandler) revokeOne(c *gin.Context) {
	uid := currentUID(c)
	jti := c.Param("jti")
	if jti == "" {
		fail(c, http.StatusBadRequest, "请求参数非法")
		return
	}
	row, err := h.auth.RevokeSession(uid, jti, time.Now().Unix())
	if err != nil {
		if errors.Is(err, service.ErrSessionNotFound) {
			// 不存在与"别人的会话"统一 404（不泄露会话存在性）
			fail(c, http.StatusNotFound, "会话不存在")
			return
		}
		mapSvcErr(c, err)
		return
	}
	h.kickAndLog(uid, []string{row.Jti})
	ok(c, gin.H{
		"revoked":   true,
		"jti":       row.Jti,
		"revokedAt": row.RevokedAt,
		"current":   row.Jti == currentJTI(c),
	})
}

// revokeOthers DELETE /api/v1/auth/sessions
//
// 撤销本人除当前会话外的全部有效会话（"登出其它所有设备"）；
// 当前会话不受影响。返回撤销数量与被断开的 WS 连接数。
func (h *sessionHandler) revokeOthers(c *gin.Context) {
	uid := currentUID(c)
	keep := currentJTI(c)
	jtis, err := h.auth.RevokeOtherSessions(uid, keep, time.Now().Unix())
	if err != nil {
		// 部分撤销成功后失败：已撤销的部分已经生效（如实上报数量），
		// 错误照常映射（500），客户端重试会幂等地继续撤销剩余部分
		mapSvcErr(c, err)
		return
	}
	h.kickAndLog(uid, jtis)
	ok(c, gin.H{"revoked": len(jtis), "jtiList": jtis})
}

// kickAndLog 撤销后的收尾：主动断开这些会话的 WS 连接（4003）并记一条服务端日志。
//
// 刻意放在"撤销已落库"之后：即使这里失败（进程重启/Hub 为空），撤销依然生效——
// WS 踢出是尽力而为的即时反馈，不是撤销语义的一部分。
func (h *sessionHandler) kickAndLog(uid int64, jtis []string) {
	closed := 0
	for _, j := range jtis {
		closed += h.hub.KickJti(j)
	}
	if len(jtis) > 0 {
		log.Printf("[auth] 会话已撤销 uid=%d count=%d wsClosed=%d", uid, len(jtis), closed)
	}
}
