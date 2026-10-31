package api

// 认证 HTTP 适配层：注册 / 登录 / 当前用户。
//
// GPT-R45：注册的输入边界（用户名长度、bcrypt 72 字节上限）由 service 层在【开事务之前】
// 校验完毕，本层只负责把 ErrUsernameTooLong / ErrPasswordTooLong 映射成明确的 400，
// 绝不让它落到 default 分支变成 500。
//
// GPT-R46：登录**不能**把所有错误一律映射成 401。旧实现为了"不泄露账号是否存在"，
// 把数据库故障、登录审计写入失败、令牌签发失败也一并吞成 401——结果真正的服务端故障
// 被伪装成"用户名或密码错误"，客户端只会让用户反复重试，运维也看不到线索。
// 现在用 errors.Is 分岔：凭证类统一 401（防枚举的语义不变），其余归 500/503 + 安全日志。

import (
	"errors"
	"log"
	"net/http"

	"fastledger/internal/model"
	"fastledger/internal/service"

	"github.com/gin-gonic/gin"
)

// authAPI HTTP 层依赖的认证能力。
//
// 抽成接口而不是直接依赖 *service.AuthService，是为了让 GPT-R46 的"内部错误 → 503 且
// 响应体不含内部细节"能在无 CGO（跑不了真实 SQLite）的机器上用确定性 stub 回归测试；
// 断言强度没有降低——被测的是本文件的错误映射本身。
type authAPI interface {
	Register(username, password string) (*model.User, error)
	// LoginFromIP 登录：clientIP 取 gin 的 ClientIP()（router.go 已 SetTrustedProxies(nil)，
	// 恒等于 TCP 对端）。登录失败限流按它分桶，绝不能用请求头拼出来的值。
	LoginFromIP(username, password, clientIP string) (string, error)
	GetByID(uid int64) (*model.User, error)
}

type authHandler struct {
	authSvc        authAPI
	enableRegister bool
}

func (h *authHandler) register(c *gin.Context) {
	// 关闭公开注册后，唯一入户通道是邀请码（PRD §12.1 决策）
	if !h.enableRegister {
		fail(c, http.StatusForbidden, "已关闭公开注册，请使用邀请码加入")
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !bindJSON(c, &req) {
		return
	}
	u, err := h.authSvc.Register(req.Username, req.Password)
	if err != nil {
		// 用户名长度 / 口令超长 / 口令过短 都是 400（见 common.go 的 mapSvcErr），
		// 且 service 在开事务之前就返回，不会有任何一次落库。
		mapSvcErr(c, err)
		return
	}
	ok(c, currentUserDTO(u))
}

func (h *authHandler) login(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !bindJSON(c, &req) {
		return
	}
	// ClientIP 必须传真实对端地址：登录失败限流按它分桶（A6）。
	// router.go 已 SetTrustedProxies(nil)，因此它不受 X-Forwarded-For 操纵。
	token, err := h.authSvc.LoginFromIP(req.Username, req.Password, c.ClientIP())
	if err != nil {
		mapLoginErr(c, err)
		return
	}
	ok(c, gin.H{"accessToken": token})
}

// mapLoginErr 登录错误的唯一映射入口。
//
// 四类分支：
//  1. 凭证类（口令不匹配 / 账号禁用 / 账号不存在）→ 统一 401 + 同一句文案。
//     账号禁用也用同一句，避免据此枚举"哪些账号被封了"。
//  2. 失败限流（A6）→ 429：它【不是】凭证错误（不能混进 401 的统一文案——那会让
//     客户端以为是密码错而反复重试），也【不是】内部错误（不该触发告警）；客户端应退避。
//  3. 依赖不可用（DB 故障 / 成功审计写入失败 / 令牌签发失败）→ 503 + 安全日志。
//  4. 其余未预期错误 → 500 + 安全日志。
//
// 3 与 4 都必须先记日志：日志留在服务端供排查，响应体只给一句通用文案，
// 绝不回显 err（会把表名、驱动报错、内部路径泄露给客户端）。
func mapLoginErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrBadCredentials), errors.Is(err, service.ErrAccountDisabled):
		// 不区分"用户不存在"与"口令错误"，避免账号枚举
		fail(c, http.StatusUnauthorized, "用户名或密码错误")
	case errors.Is(err, service.ErrRateLimited):
		// A6：与 R46 口径并存——429 不是凭证错误，也不是内部错误
		fail(c, http.StatusTooManyRequests, "登录尝试过于频繁，请稍后再试")
	case errors.Is(err, service.ErrAuthUnavailable):
		logLoginInternalError(c, err)
		fail(c, http.StatusServiceUnavailable, "服务暂时不可用，请稍后重试")
	default:
		logLoginInternalError(c, err)
		fail(c, http.StatusInternalServerError, "服务内部错误，请稍后重试")
	}
}

// logLoginInternalError 登录内部错误的**安全日志**。
//
// 记录：客户端 IP、方法、路径、错误本身（服务端排查所需）。
// 刻意【不记录】用户名、口令、令牌——它们是凭据/PII，一旦落日志就等于把凭证泄露到
// 日志采集与备份里；排查登录故障靠 IP + 时间 + 错误链已经足够。
func logLoginInternalError(c *gin.Context, err error) {
	ip, method, path := "-", "-", "-"
	if c != nil {
		ip = c.ClientIP()
		if c.Request != nil {
			method = c.Request.Method
			path = c.Request.URL.Path
		}
	}
	log.Printf("[auth] 登录内部错误 ip=%s method=%s path=%s err=%v", ip, method, path, err)
}

func (h *authHandler) me(c *gin.Context) {
	u, err := h.authSvc.GetByID(currentUID(c))
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	ok(c, currentUserDTO(u))
}
