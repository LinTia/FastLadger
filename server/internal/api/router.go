package api

// 路由（对齐《落地方案》§5.1 REST API 契约，统一前缀 /api/v1）
import (
	"time"

	"fastledger/internal/config"
	"fastledger/internal/instance"
	"fastledger/internal/service"
	"fastledger/internal/ws"

	"github.com/gin-gonic/gin"
)

// Deps 路由依赖
type Deps struct {
	Config      *config.Config
	AuthService *service.AuthService
	Books       *service.BookService
	Members     *service.MemberService
	Accounts    *service.AccountService
	Categories  *service.CategoryService
	Txs         *service.TxService
	Sync        *service.SyncService
	Reports     *service.ReportService
	Budgets     *service.BudgetService
	Invest      *service.InvestService
	PATs        *service.PATService
	Instance    *instance.Meta
	// Hub 会话撤销踢连用的 Hub（R52）。nil 时会话 handler 回落 ws.DefaultHub，
	// 与生产接线（main 用 DefaultHub）行为一致；测试可注入独立 Hub 隔离状态。
	Hub           *ws.Hub
	OCR           *service.OCRService
	SchemaVersion int64
}

// NewRouter 构建 gin 引擎
func NewRouter(d Deps) *gin.Engine {
	r := gin.New()
	// 信任代理口径（复审 A5 / INFRA-04 反向，**根因修复**）：
	// gin 的出厂默认是 trustedProxies = 0.0.0.0/0（信任一切代理），c.ClientIP() 会取
	// X-Forwarded-For 最左段——那是请求方完全可控的字符串。而登录/邀请限流的桶键都是
	// "…:" + ClientIP，等于把限流闸门的钥匙交给攻击者：每换一个 X-Forwarded-For 头
	// 就换一个全新配额桶，还能用一次性伪造 IP 灌满限流器容量（见 limiter.go 容量语义）。
	//
	// 自托管 MVP 默认【不信任任何代理】：ClientIP 恒取 TCP 对端（RemoteAddr），
	// 伪造头从此无效。**反代部署必须显式改为受信网段**，例如
	//	r.SetTrustedProxies([]string{"10.0.0.0/8"})
	// 并确保只有反代能直达本进程（否则客户端可直连绕过反代伪造源地址）。
	// nil 入参在 gin 里不会返回错误；一旦未来 gin 语义变化导致失败，
	// 宁可启动失败也不能带病监听（限流键的完整性依赖这一行）。
	if err := r.SetTrustedProxies(nil); err != nil {
		panic("设置受信代理失败（ClientIP 将不可信）：" + err.Error())
	}
	r.Use(gin.Recovery())

	// SYNC-01：WS 连接数上限来自配置（config.Load 已保证两个值恒为正整数；
	// 测试里以零值 Config 构造时为 0 = 不限，与既有用例行为一致）。
	// Hub 是进程级单例（DefaultHub），这里重复调用只是幂等地重设同一个值。
	ws.DefaultHub.SetConnectionLimits(d.Config.WSMaxConnPerUser, d.Config.WSMaxConnTotal)

	// 登录失败限流参数（A6）：零值 Config（直构的单测）为 0 = 沿用 AuthService 默认
	// （10 次 / 15 分钟）；生产走 config.Load，两个值恒为正。
	if d.AuthService != nil && d.Config.LoginMaxAttempts > 0 && d.Config.LoginWindowMinutes > 0 {
		d.AuthService.ConfigureLoginThrottle(d.Config.LoginMaxAttempts,
			time.Duration(d.Config.LoginWindowMinutes)*time.Minute)
	}
	// 同步限流（INFRA 复审）：同理，零值 Config = 不启用（直调服务层的单测不受影响）；
	// 生产经 config.Load 得到默认 60（push）/ 300（pull）次/分钟。
	if d.Sync != nil && d.Config.SyncPushPerMinute > 0 && d.Config.SyncPullPerMinute > 0 {
		d.Sync.EnableThrottle(d.Config.SyncPushPerMinute, d.Config.SyncPullPerMinute)
	}

	ah := &authHandler{authSvc: d.AuthService, enableRegister: d.Config.EnableRegister}
	bh := &bookHandler{books: d.Books, members: d.Members}
	lh := &ledgerHandler{accounts: d.Accounts, categories: d.Categories, txs: d.Txs}
	// WBS 6.1/6.2：报表与导出（只读聚合）。nil 容忍与 Txs 同口径：测试可省略，
	// 路由仍注册（调用时 500 由 nil 检查兜底不如直接不挂——这里选择显式判空挂载）
	var rh *reportHandler
	if d.Reports != nil {
		rh = &reportHandler{svc: d.Reports}
	}
	// WBS 6.3：预算（nil 容忍口径与 Reports 一致）
	var bh2 *budgetHandler
	if d.Budgets != nil {
		bh2 = &budgetHandler{svc: d.Budgets}
	}
	// WBS 6.4/6.5：投资与净资产
	var ih *investHandler
	if d.Invest != nil {
		ih = &investHandler{svc: d.Invest}
	}
	// WBS 6.6：长期 API Token（PAT）。nil 时 flt_ 凭证一律 401（fail-closed）。
	var ph *patHandler
	if d.PATs != nil {
		ph = &patHandler{svc: d.PATs}
	}
	sh := &syncHandler{svc: d.Sync}
	// R52：多端会话（列表 / 撤销）。Hub 为 nil 时回落 DefaultHub（见 session_handler.go）
	seh := newSessionHandler(d.AuthService, d.Hub)
	ocr := &ocrHandler{svc: d.OCR, maxBytes: ocrImageLimit(d.Config.OCRMaxImageBytes)}

	// Web 端静态资源（WBS 5.9）：NoRoute 兜底，显式路由（/healthz、/api/v1、/ws）
	// 永远优先；embed 为空时跳过挂载（服务照常，仅 Web 不可用）。
	if wh := newWebHandler(); wh != nil {
		r.NoRoute(wh)
	}

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
		v1.GET("/ws", wsAuth(d.AuthService, d.PATs), ws.ServeWS)

		auth := v1.Group("", JWTAuth(d.AuthService, d.PATs))
		auth.GET("/users/me", ah.me)
		// WBS 6.6：长期 API Token 管理（生成/列表/撤销；明文仅创建响应出现一次）
		if ph != nil {
			auth.GET("/users/me/tokens", ph.list)
			auth.POST("/users/me/tokens", ph.create)
			auth.DELETE("/users/me/tokens/:tokenId", ph.revoke)
		}
		auth.POST("/ocr/recognize", ocr.recognize)
		auth.POST("/ocr/parse", ocr.parse)
		// WBS 5.8：云端 LLM 文本兜底（默认关闭，需请求体 consent=true）
		auth.POST("/ocr/llm-parse", ocr.llmParse)

		// 多端会话（R52，WBS 5.10 / PRD §10.10）：列表 / 登出其它设备 / 撤销指定设备。
		// DELETE 的两个路由同一静态节点（sessions）分挂 handler 与 :jti 参数子节点，
		// 不构成 gin 路由树冲突（冲突仅发生在"同层静态段 vs 参数段"）。
		auth.GET("/auth/sessions", seh.list)
		auth.DELETE("/auth/sessions", seh.revokeOthers)
		auth.DELETE("/auth/sessions/:jti", seh.revokeOne)

		auth.GET("/books", bh.list)
		auth.POST("/books", bh.create)
		auth.GET("/books/:id", bh.get)
		auth.PUT("/books/:id", bh.update)
		auth.DELETE("/books/:id", bh.remove)

		// 可见性开关（阶段 3.4，PRD §7.3 / §12.1）
		auth.PUT("/books/:id/visibility", bh.setVisibility)

		auth.GET("/books/:id/members", bh.listMembers)
		auth.POST("/books/:id/members/invite", bh.createInvite)
		// GPT-R40：撤销邀请码（只能撤销尚未使用、未撤销、未过期的邀请）。
		//
		// 用 POST + 动作段而不是 DELETE /invite/:inviteId：gin 的路由树不允许在
		// 同一层同时存在通配段与静态段，而 DELETE /books/:id/members/:uid 已占用
		// `:uid` 通配——注册 DELETE /books/:id/members/invite/... 会在启动时直接 panic。
		// 动作段风格在本仓也有先例：`POST /transactions/:txId/restore`。
		auth.POST("/books/:id/members/invite/:inviteId/revoke", bh.revokeInvite)
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
		// WBS 5.9b：更新（全量替换，转账整笔联动；语义见 docs/FastLedger-交易更新语义设计.md）
		auth.PUT("/books/:id/transactions/:txId", lh.updateTransaction)
		// 阶段 4：删除 / 恢复（整笔，含转账对侧）
		auth.DELETE("/books/:id/transactions/:txId", lh.deleteTransaction)
		auth.POST("/books/:id/transactions/:txId/restore", lh.restoreTransaction)
		// 阶段 4.1：回收站视图。路径不与 /transactions/:txId 的通配段冲突
		auth.GET("/books/:id/recycle", lh.listRecycleBin)

		// 报表与导出（WBS 6.1 / 6.2，只读聚合；报表读路径同样受可见性开关约束）
		if rh != nil {
			auth.GET("/books/:id/reports/monthly", rh.monthly)
			auth.GET("/books/:id/reports/categories", rh.categoryShare)
			auth.GET("/books/:id/export", rh.export)
		}

		// 预算（WBS 6.3）：信封式总预算 + 分类月度预算；写操作 owner/admin
		if bh2 != nil {
			auth.GET("/books/:id/budgets", bh2.list)
			auth.POST("/books/:id/budgets", bh2.create)
			auth.PUT("/books/:id/budgets/:budgetId", bh2.update)
			auth.DELETE("/books/:id/budgets/:budgetId", bh2.remove)
		}

		// 投资与净资产（WBS 6.4 / 6.5）：标的库 + 持仓 + 手动行情 + 净资产汇总
		if ih != nil {
			auth.GET("/books/:id/instruments", ih.listInstruments)
			auth.POST("/books/:id/instruments", ih.createInstrument)
			auth.PUT("/books/:id/instruments/:instrumentId", ih.updateInstrument)
			auth.DELETE("/books/:id/instruments/:instrumentId", ih.deleteInstrument)
			auth.POST("/books/:id/instruments/:instrumentId/price", ih.setPrice)
			auth.GET("/books/:id/holdings", ih.listHoldings)
			auth.POST("/books/:id/holdings", ih.createHolding)
			auth.PUT("/books/:id/holdings/:holdingId", ih.updateHolding)
			auth.DELETE("/books/:id/holdings/:holdingId", ih.deleteHolding)
			auth.GET("/books/:id/net-worth", ih.netWorth)
		}

		// 增量同步（阶段 3.1 / 3.2）
		auth.GET("/books/:id/sync/pull", sh.pull)
		auth.POST("/books/:id/sync/push", sh.push)
	}
	return r
}
