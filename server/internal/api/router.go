package api

// 路由（对齐《落地方案》§5.1 REST API 契约，统一前缀 /api/v1）
import (
	"fastledger/internal/config"
	"fastledger/internal/instance"
	"fastledger/internal/service"
	"fastledger/internal/ws"

	"github.com/gin-gonic/gin"
)

// Deps 路由依赖
type Deps struct {
	Config        *config.Config
	AuthService   *service.AuthService
	Books         *service.BookService
	Members       *service.MemberService
	Accounts      *service.AccountService
	Categories    *service.CategoryService
	Txs           *service.TxService
	Sync          *service.SyncService
	Instance      *instance.Meta
	SchemaVersion int64
}

// NewRouter 构建 gin 引擎
func NewRouter(d Deps) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	ah := &authHandler{authSvc: d.AuthService, enableRegister: d.Config.EnableRegister}
	bh := &bookHandler{books: d.Books, members: d.Members}
	lh := &ledgerHandler{accounts: d.Accounts, categories: d.Categories, txs: d.Txs}
	sh := &syncHandler{svc: d.Sync}

	r.GET("/healthz", func(c *gin.Context) {
		data := gin.H{
			"status":        "up",
			"schemaVersion": d.SchemaVersion,
		}
		// 恢复世代（指南 §26.1）：客户端据此判断"服务端是否从备份回退过"
		if d.Instance != nil {
			data["instanceId"] = d.Instance.InstanceId
			data["restoreEpoch"] = d.Instance.RestoreEpoch
		}
		c.JSON(200, gin.H{"code": 0, "msg": "ok", "data": data})
	})

	v1 := r.Group("/api/v1")
	{
		v1.POST("/auth/register", ah.register)
		v1.POST("/auth/login", ah.login)

		// 邀请码入户：不受 EBK_ENABLE_REGISTER 限制（PRD §12.1）
		v1.POST("/books/:id/members/join", bh.join)

		// WebSocket：**不挂在 JWTAuth 组下**。
		// 浏览器 WS 无法设置 Authorization 头，wsAuth 需要在本路由内自行
		// "先读 Header、再回退 ?token="；若被 JWTAuth 先拦，回退分支永远不会生效。
		v1.GET("/ws", wsAuth(d.AuthService), ws.ServeWS)

		auth := v1.Group("", JWTAuth(d.AuthService))
		auth.GET("/users/me", ah.me)

		auth.GET("/books", bh.list)
		auth.POST("/books", bh.create)
		auth.GET("/books/:id", bh.get)
		auth.PUT("/books/:id", bh.update)
		auth.DELETE("/books/:id", bh.remove)

		// 可见性开关（阶段 3.4，PRD §7.3 / §12.1）
		auth.PUT("/books/:id/visibility", bh.setVisibility)

		auth.GET("/books/:id/members", bh.listMembers)
		auth.POST("/books/:id/members/invite", bh.createInvite)
		auth.PUT("/books/:id/members/:uid", bh.changeRole)
		auth.DELETE("/books/:id/members/:uid", bh.removeMember)

		// 账户（阶段 2.1）
		auth.GET("/books/:id/accounts", lh.listAccounts)
		auth.POST("/books/:id/accounts", lh.createAccount)
		auth.GET("/books/:id/accounts/:aid", lh.getAccount)
		auth.PUT("/books/:id/accounts/:aid", lh.updateAccount)
		auth.DELETE("/books/:id/accounts/:aid", lh.deleteAccount)

		// 分类（阶段 2.1，两级；?tree=1 返回树）
		auth.GET("/books/:id/categories", lh.listCategories)
		auth.POST("/books/:id/categories", lh.createCategory)
		auth.PUT("/books/:id/categories/:cid", lh.updateCategory)
		auth.DELETE("/books/:id/categories/:cid", lh.deleteCategory)

		// 交易（阶段 2.2~2.6）：三种类型共用同一创建入口
		auth.GET("/books/:id/transactions", lh.listTransactions)
		auth.GET("/books/:id/transactions/:txId", lh.getTransaction)
		auth.POST("/books/:id/transactions", lh.createTransaction)

		// 增量同步（阶段 3.1 / 3.2）
		auth.GET("/books/:id/sync/pull", sh.pull)
		auth.POST("/books/:id/sync/push", sh.push)
	}
	return r
}
