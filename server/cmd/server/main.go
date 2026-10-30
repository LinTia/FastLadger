// FastLedger 服务端入口（阶段 0 脚手架 + 阶段 1 认证/账本/成员）。
//
// 用法：
//
//	fastledger                                   # 启动 HTTP 服务
//	fastledger db migrate                        # 手动执行数据库迁移
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
	"syscall"
	"time"

	"fastledger/internal/api"
	"fastledger/internal/config"
	"fastledger/internal/database"
	"fastledger/internal/migrate"
	"fastledger/internal/model"
	"fastledger/internal/service"
	"fastledger/internal/version"

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

	// SQLite 不会自动创建目录
	if dir := filepath.Dir(cfg.DBPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatalf("创建数据目录失败：%v", err)
		}
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
			log.Fatalf("未知子命令 %q（可用：db migrate / user create）", args[0])
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

	versions := version.NewRepo() // 版本唯一实现（VersionRepo.Next）
	authSvc := service.NewAuthService(engine, cfg.JWTSecret, tokenTTL)
	books := service.NewBookService(engine, versions)
	members := service.NewMemberService(engine, authSvc, cfg.InviteTTLDays)

	r := api.NewRouter(api.Deps{
		Config:        cfg,
		AuthService:   authSvc,
		Books:         books,
		Members:       members,
		SchemaVersion: st.Current,
	})

	srv := &http.Server{Addr: ":" + cfg.HTTPPort, Handler: r}
	go func() {
		log.Printf("FastLedger 启动：http://localhost:%s （schema v%d，公开注册=%v）",
			cfg.HTTPPort, st.Current, cfg.EnableRegister)
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

// handleDB 迁移子命令
func handleDB(engine *xorm.Engine, args []string) {
	if len(args) == 0 || args[0] != "migrate" {
		log.Fatalf("用法：db migrate")
	}
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
