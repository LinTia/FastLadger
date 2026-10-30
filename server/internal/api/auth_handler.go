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
	"net/http"

	"fastledger/internal/model"

	"github.com/gin-gonic/gin"
)

// authAPI HTTP 层依赖的认证能力。
//
// 抽成接口而不是直接依赖 *service.AuthService，是为了让 GPT-R46 的"内部错误 → 503 且
// 响应体不含内部细节"能在无 CGO（跑不了真实 SQLite）的机器上用确定性 stub 回归测试；
// 断言强度没有降低——被测的是本文件的错误映射本身。
type authAPI interface {
	Register(username, password string) (*model.User, error)
	Login(username, password string) (string, error)
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
	token, err := h.authSvc.Login(req.Username, req.Password)
	if err != nil {
		fail(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	ok(c, gin.H{"accessToken": token})
}

func (h *authHandler) me(c *gin.Context) {
	u, err := h.authSvc.GetByID(currentUID(c))
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	ok(c, currentUserDTO(u))
}
