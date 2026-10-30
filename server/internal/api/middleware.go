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
	"fastledger/internal/ws"

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

// wsAuth WebSocket 专用鉴权中间件：**优先读 Authorization: Bearer，回退读 ?token=**。
//
// 为什么需要单独一套：浏览器的 WebSocket API 不允许设置自定义请求头
// （new WebSocket(url, protocols) 只能设子协议），因此前端**无法**像普通 fetch 那样
// 带 Authorization 头，只能把 token 放 query。
//
// 安全提示（必须知道）：token 出现在 URL 上会进入 access log / 反向代理日志 /
// 浏览器历史 / Referer。生产部署须二选一：
//  1. 由反代对日志做脱敏（例如 Caddy `log { format filter { ... query { delete token } } }`）；
//  2. 改用 Sec-WebSocket-Protocol 子协议承载 token（客户端库支持度不一致，MVP 未采用）。
//
// 注意：本中间件**不能**挂在已经套了 JWTAuth 的路由组下面——那样无 Authorization 头的
// 请求会先被 JWTAuth 以 401 拦掉，?token= 回退永远不会生效。路由注册见 router.go。
func wsAuth(authSvc *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := bearerToken(c.GetHeader("Authorization"))
		if raw == "" {
			raw = strings.TrimSpace(c.Query("token"))
		}
		if raw == "" {
			fail(c, http.StatusUnauthorized, "缺少登录凭证")
			c.Abort()
			return
		}
		if authSvc == nil {
			fail(c, http.StatusUnauthorized, "登录已失效，请重新登录")
			c.Abort()
			return
		}
		claims, u, err := authSvc.VerifyWithClaims(raw)
		if err != nil {
			fail(c, http.StatusUnauthorized, "登录已失效，请重新登录")
			c.Abort()
			return
		}
		c.Set(uidKey, u.Id)
		// GPT-R51：把 JWT 的 exp 交下去作为这条长连接的生命周期上限。
		// WS 握手之后不再经过任何中间件，ServeWS 只能凭这个 exp 在令牌到期时
		// 主动关闭连接并清理订阅；缺失它连接就能永远活着。
		c.Set(ws.TokenExpContextKey, claims.Exp)
		c.Next()
	}
}

// bearerToken 从 "Bearer <token>" 中取出 token；格式不符返回空串（交由调用方回退）
func bearerToken(header string) string {
	parts := strings.SplitN(strings.TrimSpace(header), " ", 2)
	if len(parts) != 2 || !strings.EqualFold(strings.TrimSpace(parts[0]), "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
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
