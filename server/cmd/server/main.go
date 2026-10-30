// FastLedger 服务端入口（阶段 0 脚手架 + 阶段 1 认证/账本/成员 + 阶段 3 同步/协同）。
//
// 用法：
//
//	fastledger                                   # 启动 HTTP 服务
//	fastledger db migrate                        # 手动执行数据库迁移
//	fastledger db purge                          # 回收站与审计的到期硬清除（可用
//	                                             #   --recycle-ttl-days / --audit-retention-days 覆盖）
//	fastledger instance bump                     # 恢复后递增 restore_epoch（不随备份恢复）
//	fastledger user create --username u --password p [--owner]
//	                                             # --owner 仅在【无任何用户】时可用（首个 owner 初始化）
//
// 约束（《落地方案》§4.7）：服务启动【只检查】schema 版本，绝不静默自动改表。
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"fastledger/internal/api"
	"fastledger/internal/config"
	"fastledger/internal/database"
	"fastledger/internal/instance"
	"fastledger/internal/migrate"
	"fastledger/internal/model"
	"fastledger/internal/service"
	"fastledger/internal/version"
	"fastledger/internal/ws"

	"golang.org/x/crypto/bcrypt"
	"xorm.io/xorm"
)

// tokenTTL 访问令牌有效期
const tokenTTL = 24 * time.Hour

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("配置加载失败：%v", err)
	}

	// 数据目录（SQLite 与 instance.meta 都不会自动创建目录）
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		log.Fatalf("创建数据目录失败：%v", err)
	}
	if dir := filepath.Dir(cfg.DBPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatalf("创建数据目录失败：%v", err)
		}
	}

	// instance 子命令不依赖数据库，必须能在"替换 db 文件之后、服务启动之前"执行：
	// 停容器 → 替换 db → `fastledger instance bump` → 启动（《落地方案》§8.3 恢复流程）。
	if args := os.Args[1:]; len(args) > 0 && args[0] == "instance" {
		handleInstance(cfg.DataDir, args[1:])
		return
	}

	engine, err := database.Init(cfg.DBPath)
	if err != nil {
		log.Fatalf("数据库初始化失败：%v", err)
	}
	defer func() {
		if cerr := engine.Close(); cerr != nil {
			log.Printf("关闭数据库出错：%v", cerr)
		}
	}()

	// 子命令
	if args := os.Args[1:]; len(args) > 0 {
		switch args[0] {
		case "db":
			handleDB(engine, args[1:])
			return
		case "user":
			handleUser(engine, args[1:])
			return
		default:
			log.Fatalf("未知子命令 %q（可用：db migrate / instance bump / user create）", args[0])
		}
	}

	// 正常启动：只检查版本
	st, err := migrate.Check(engine)
	if err != nil {
		log.Fatalf("迁移状态检查失败：%v", err)
	}
	if !st.OK {
		log.Fatalf("schema 版本不一致（当前 v%d，期望 v%d）：请先执行 `fastledger db migrate`",
			st.Current, st.Expected)
	}

	// 实例元信息（restore_epoch）：不存在则生成并落盘 0600（指南 §26.1）
	meta, err := instance.LoadOrCreate(cfg.DataDir)
	if err != nil {
		log.Fatalf("实例元信息加载失败：%v", err)
	}

	versions := version.NewRepo() // 版本唯一实现（VersionRepo.Next）
	authSvc := service.NewAuthService(engine, cfg.JWTSecret, tokenTTL)
	books := service.NewBookService(engine, versions)
	members := service.NewMemberService(engine, authSvc, cfg.InviteTTLDays)
	accounts := service.NewAccountService(engine, versions)
	categories := service.NewCategoryService(engine, versions)
	txs := service.NewTxService(engine, versions)
	syncSvc := service.NewSyncService(engine, txs, meta)

	// WS 订阅资格 + 可见性校验：由 service 提供（ws 不得在 Hub 锁内查库，
	// 只在 Subscribe / RefreshVisibility 的锁外调用本函数并缓存结果）
	ws.DefaultHub.SetChecker(func(uid, bookID int64) (bool, error) {
		return service.CanViewDetail(engine, uid, bookID)
	})

	r := api.NewRouter(api.Deps{
		Config:        cfg,
		AuthService:   authSvc,
		Books:         books,
		Members:       members,
		Accounts:      accounts,
		Categories:    categories,
		Txs:           txs,
		Sync:          syncSvc,
		Instance:      meta,
		SchemaVersion: st.Current,
	})

	srv := &http.Server{Addr: ":" + cfg.HTTPPort, Handler: r}
	go func() {
		log.Printf("FastLedger 启动：http://localhost:%s （schema v%d，公开注册=%v，restore_epoch=%d）",
			cfg.HTTPPort, st.Current, cfg.EnableRegister, meta.RestoreEpoch)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("服务异常：%v", err)
		}
	}()

	// 优雅关机（部署需 stop_grace_period ≥ 30s）
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("收到退出信号，优雅关闭中…")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("HTTP 关闭出错：%v", err)
	}
	log.Println("已退出")
}

// handleInstance 实例元信息子命令（恢复流程使用）
func handleInstance(dataDir string, args []string) {
	if len(args) == 0 || args[0] != "bump" {
		log.Fatalf("用法：instance bump（恢复备份后递增 restore_epoch）")
	}
	before, err := instance.LoadOrCreate(dataDir)
	if err != nil {
		log.Fatalf("读取实例元信息失败：%v", err)
	}
	after, err := instance.Bump(dataDir)
	if err != nil {
		log.Fatalf("递增 restore_epoch 失败：%v", err)
	}
	log.Printf("restore_epoch：%d → %d（%s，文件 %s）",
		before.RestoreEpoch, after.RestoreEpoch, after.RestoredAt, instance.Path(dataDir))
}

// handleDB 数据库子命令：migrate（应用迁移）/ purge（回收站与审计到期硬清除）
func handleDB(engine *xorm.Engine, args []string) {
	if len(args) == 0 {
		log.Fatalf("用法：db migrate | db purge")
	}
	switch args[0] {
	case "migrate":
		handleDBMigrate(engine)
	case "purge":
		handleDBPurge(engine, args[1:])
	default:
		log.Fatalf("用法：db migrate | db purge")
	}
}

// handleDBMigrate 应用未执行的迁移
func handleDBMigrate(engine *xorm.Engine) {
	n, err := migrate.Apply(engine)
	if err != nil {
		log.Fatalf("迁移失败：%v", err)
	}
	st, err := migrate.Check(engine)
	if err != nil {
		log.Fatalf("迁移后校验失败：%v", err)
	}
	log.Printf("迁移完成：本次应用 %d 个，当前 schema v%d", n, st.Current)
}

// handleDBPurge 回收站与审计的到期硬清除（WBS 4.1 / 4.3，PRD §12.1）。
//
// 与迁移同一口径：**仅 CLI 手动执行**，服务启动绝不静默删数据。
// 保留天数默认取环境变量（EBK_RECYCLE_TTL_DAYS / EBK_AUDIT_RETENTION_DAYS），
// 可用 --recycle-ttl-days / --audit-retention-days 覆盖（便于演练与应急）。
func handleDBPurge(engine *xorm.Engine, args []string) {
	recycleTTL := envIntOr("EBK_RECYCLE_TTL_DAYS", config.DefaultRecycleTTLDays)
	auditDays := envIntOr("EBK_AUDIT_RETENTION_DAYS", config.DefaultAuditRetentionDays)
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--recycle-ttl-days" && i+1 < len(args):
			recycleTTL = parsePositiveInt(args[i+1], recycleTTL)
			i++
		case args[i] == "--audit-retention-days" && i+1 < len(args):
			auditDays = parsePositiveInt(args[i+1], auditDays)
			i++
		}
	}
	if recycleTTL <= 0 || auditDays <= 0 {
		log.Fatalf("保留天数必须为正整数：recycle=%d audit=%d", recycleTTL, auditDays)
	}

	now := time.Now().Unix()
	// 回收站：按账本逐个清除（PurgeExpiredTransactions 以账本为界，避免误删他库）
	var bookIDs []int64
	if err := engine.Table("books").Select("id").Find(&bookIDs); err != nil {
		log.Fatalf("读取账本列表失败：%v", err)
	}
	txs := service.NewTxService(engine, version.NewRepo())
	var recycleTotal int64
	for _, id := range bookIDs {
		n, err := txs.PurgeExpiredTransactions(id, recycleTTL, now)
		if err != nil {
			log.Fatalf("回收站清除失败（账本 %d）：%v", id, err)
		}
		recycleTotal += n
	}
	auditTotal, err := service.PurgeAuditLogs(engine, auditDays, now)
	if err != nil {
		log.Fatalf("审计保留期清除失败：%v", err)
	}
	log.Printf("清理完成：回收站硬清除 %d 条（TTL=%d 天，账本 %d 个）；审计清除 %d 条（保留 %d 天）",
		recycleTotal, recycleTTL, len(bookIDs), auditTotal, auditDays)
}

// envIntOr 读取环境变量并转为正整数，缺失/非法时回落到 def。
// 这里不走 config.Load()：该 CLI 只关心保留天数，不应被 JWT 校验等无关约束挡住。
func envIntOr(key string, def int) int {
	return parsePositiveInt(os.Getenv(key), def)
}

// parsePositiveInt 解析正整数（命令行覆盖值），非法时回落到 def。
func parsePositiveInt(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil && n > 0 {
		return n
	}
	return def
}

// handleUser 用户创建子命令
func handleUser(engine *xorm.Engine, args []string) {
	if len(args) == 0 || args[0] != "create" {
		log.Fatalf("用法：user create --username <name> --password <pw> [--owner]")
	}
	var username, password string
	owner := false
	for i := 1; i < len(args); i++ {
		switch {
		case args[i] == "--username" && i+1 < len(args):
			username = args[i+1]
			i++
		case args[i] == "--password" && i+1 < len(args):
			password = args[i+1]
			i++
		case args[i] == "--owner":
			owner = true
		}
	}
	if username == "" || password == "" {
		log.Fatalf("必须提供 --username 与 --password")
	}
	if len(password) < 8 {
		log.Fatalf("密码至少 8 位")
	}

	// 首个 owner 初始化：仅在无任何用户时可用，建好后自动失效（方案 §6.1 / PRD §12.1）
	var total int64
	if _, err := engine.SQL("SELECT COUNT(1) FROM users").Get(&total); err != nil {
		log.Fatalf("检查用户数失败：%v", err)
	}
	if owner && total > 0 {
		log.Fatalf("已存在用户：owner 初始化入口已失效（请改用邀请码或公开注册入户）")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("口令哈希失败：%v", err)
	}
	u := &model.User{
		Username:        username,
		PasswordHash:    string(hash),
		IsInstanceAdmin: owner, // 首个 owner 默认成为实例管理员（PRD §12.1）
		CreatedAt:       time.Now().Unix(),
		UpdatedAt:       time.Now().Unix(),
	}
	if _, err := engine.Insert(u); err != nil {
		log.Fatalf("创建用户失败：%v", err)
	}
	log.Printf("已创建用户 %q（id=%d，实例管理员=%v）", u.Username, u.Id, u.IsInstanceAdmin)
}
