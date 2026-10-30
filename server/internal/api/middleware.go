package api

// JWT 鉴权中间件。
//
// 约束（R09）：限定 HS256、非法 claims 返回 401 而非 panic、校验账号是否删除/禁用。
// token 校验放在 service.AuthService.Verify，密钥来自配置注入（禁止硬编码）。
import (
	"net/http"
	"strings"

	"fastledger/internal/model"
	"fastledger/internal/service"

	"github.com/gin-gonic/gin"
)

// uidKey 上下文中用户 ID 的键
const uidKey = "uid"

// JWTAuth 校验 Authorization: Bearer <token>
func JWTAuth(authSvc *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if h == "" {
			fail(c, http.StatusUnauthorized, "缺少登录凭证")
			c.Abort()
			return
		}
		parts := strings.SplitN(h, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(strings.TrimSpace(parts[0]), "Bearer") {
			fail(c, http.StatusUnauthorized, "凭证格式错误")
			c.Abort()
			return
		}
		u, err := authSvc.Verify(strings.TrimSpace(parts[1]))
		if err != nil {
			fail(c, http.StatusUnauthorized, "登录已失效，请重新登录")
			c.Abort()
			return
		}
		c.Set(uidKey, u.Id)
		c.Next()
	}
}

// currentUID 取出当前用户 ID（中间件已保证存在）
func currentUID(c *gin.Context) int64 {
	v, ok := c.Get(uidKey)
	if !ok {
		return 0
	}
	uid, _ := v.(int64)
	return uid
}

// currentUserDTO 返回脱敏的用户信息
func currentUserDTO(u *model.User) gin.H {
	return gin.H{
		"id":              u.Id,
		"username":        u.Username,
		"email":           u.Email,
		"isInstanceAdmin": u.IsInstanceAdmin,
	}
}
