// Package api HTTP 层：路由、中间件与 handler。
//
// 统一响应：{ "code": 0, "msg": "ok", "data": {...} }；
// 错误时 code 与 HTTP 状态码语义一致（401/403/404/409/422/500），且不外露内部错误细节。
package api

import (
	"errors"
	"net/http"

	"fastledger/internal/service"

	"github.com/gin-gonic/gin"
)

// ok 成功响应
func ok(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": data})
}

// fail 失败响应（不回显内部错误）
func fail(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"code": status, "msg": msg})
}

// bindJSON 绑定并显式校验，绝不吞掉错误（D-R04 同类问题）
func bindJSON(c *gin.Context, obj interface{}) bool {
	if err := c.ShouldBindJSON(obj); err != nil {
		fail(c, http.StatusBadRequest, "请求格式错误")
		return false
	}
	return true
}

// mapSvcErr 把服务层错误映射为 HTTP 状态码
func mapSvcErr(c *gin.Context, err error) {
	switch {
	case err == nil:
		return
	case errors.Is(err, service.ErrForbidden), errors.Is(err, service.ErrNotMember):
		fail(c, http.StatusForbidden, "无权限")
	case errors.Is(err, service.ErrCannotOperateOwner):
		fail(c, http.StatusForbidden, "不能对账本所有者执行该操作")
	case errors.Is(err, service.ErrBookNotFound):
		fail(c, http.StatusNotFound, "账本不存在")
	case errors.Is(err, service.ErrUserExists):
		fail(c, http.StatusConflict, "用户名已存在")
	case errors.Is(err, service.ErrInviteInvalid):
		fail(c, http.StatusBadRequest, "邀请码无效或已过期")
	case errors.Is(err, service.ErrInviteBoundMismatch):
		fail(c, http.StatusBadRequest, "邀请码已绑定指定用户，信息不匹配")
	case errors.Is(err, service.ErrRateLimited):
		fail(c, http.StatusTooManyRequests, "操作过于频繁，请稍后再试")
	case errors.Is(err, service.ErrWeakPassword):
		fail(c, http.StatusBadRequest, "密码至少 8 位")
	default:
		// 不回显内部错误，避免泄露实现细节
		fail(c, http.StatusInternalServerError, "服务内部错误")
	}
}
