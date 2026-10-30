package api

// 路由（对齐《落地方案》§5.1 REST API 契约，统一前缀 /api/v1）
import (
	"fastledger/internal/config"
	"fastledger/internal/service"

	"github.com/gin-gonic/gin"
)

// Deps 路由依赖
type Deps struct {
	Config        *config.Config
	AuthService   *service.AuthService
	Books         *service.BookService
	Members       *service.MemberService
	SchemaVersion int64
}

// NewRouter 构建 gin 引擎
func NewRouter(d Deps) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	ah := &authHandler{authSvc: d.AuthService, enableRegister: d.Config.EnableRegister}
	bh := &bookHandler{books: d.Books, members: d.Members}

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 0, "msg": "ok", "data": gin.H{
			"status":        "up",
			"schemaVersion": d.SchemaVersion,
		}})
	})

	v1 := r.Group("/api/v1")
	{
		v1.POST("/auth/register", ah.register)
		v1.POST("/auth/login", ah.login)

		// 邀请码入户：不受 EBK_ENABLE_REGISTER 限制（PRD §12.1）
		v1.POST("/books/:id/members/join", bh.join)

		auth := v1.Group("", JWTAuth(d.AuthService))
		auth.GET("/users/me", ah.me)

		auth.GET("/books", bh.list)
		auth.POST("/books", bh.create)
		auth.GET("/books/:id", bh.get)
		auth.PUT("/books/:id", bh.update)
		auth.DELETE("/books/:id", bh.remove)

		auth.GET("/books/:id/members", bh.listMembers)
		auth.POST("/books/:id/members/invite", bh.createInvite)
		auth.PUT("/books/:id/members/:uid", bh.changeRole)
		auth.DELETE("/books/:id/members/:uid", bh.removeMember)
	}
	return r
}
