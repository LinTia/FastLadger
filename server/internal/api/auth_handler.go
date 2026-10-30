package api

import (
	"net/http"

	"fastledger/internal/service"

	"github.com/gin-gonic/gin"
)

type authHandler struct {
	authSvc        *service.AuthService
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
		// 不区分"用户不存在"与"口令错误"，避免账号枚举
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
