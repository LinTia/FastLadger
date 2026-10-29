// FastLedger 服务端入口（阶段 0 脚手架）。
//
// 用法：
//
//	fastledger                # 启动 HTTP 服务（默认 :8080，当前仅 /healthz）
//	fastledger db migrate     # 手动执行数据库迁移
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

	"fastledger/internal/config"
	"fastledger/internal/database"
	"fastledger/internal/migrate"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("配置加载失败：%v", err)
	}

	// SQLite 不会自动创建目录，需先确保数据目录存在
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

	// 子命令：db migrate（默认仅手动执行）
	if len(os.Args) > 1 && os.Args[1] == "db" {
		if len(os.Args) > 2 && os.Args[2] == "migrate" {
			n, err := migrate.Apply(engine)
			if err != nil {
				log.Fatalf("迁移失败：%v", err)
			}
			st, err := migrate.Check(engine)
			if err != nil {
				log.Fatalf("迁移后校验失败：%v", err)
			}
			log.Printf("迁移完成：本次应用 %d 个，当前 schema v%d", n, st.Current)
			return
		}
		log.Fatalf("未知子命令 %v（可用：db migrate）", os.Args[1:])
	}

	// 正常启动：只检查版本，不匹配即拒绝启动
	st, err := migrate.Check(engine)
	if err != nil {
		log.Fatalf("迁移状态检查失败：%v", err)
	}
	if !st.OK {
		log.Fatalf("schema 版本不一致（当前 v%d，期望 v%d）：请先执行 `fastledger db migrate`",
			st.Current, st.Expected)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": gin.H{"schemaVersion": st.Current}})
	})

	srv := &http.Server{Addr: ":" + cfg.HTTPPort, Handler: r}
	go func() {
		log.Printf("FastLedger 启动：http://localhost:%s （schema v%d）", cfg.HTTPPort, st.Current)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("服务异常：%v", err)
		}
	}()

	// 优雅关机：给数据库关闭留出时间（部署需 stop_grace_period ≥ 30s，见方案 §8.1）
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
