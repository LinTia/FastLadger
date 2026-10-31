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
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
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
	// INFRA-01：入口只做通用校验（端口/路径/保留期），**不含 JWT**——
	// `instance bump` 发生在"数据库刚被替换、服务还没起来"的恢复时刻，
	// `db migrate` / `db purge` / `user create` 也都不签发令牌，
	// 它们不该被一个与本命令无关的 JWT 校验挡在门外。
	cfg, err := config.LoadBase()
	if err != nil {
		log.Fatalf("配置加载失败：%v", err)
	}

	// GPT-R35：先把进程 umask 收紧到 0077，之后再建的文件/目录默认就是 0600/0700。
	// 放在最前面——umask 是进程全局状态，改晚了对之前创建的文件无效。
	database.ApplyStrictUmask()

	// 数据目录（SQLite 与 instance.meta 都不会自动创建目录）
	// 权限 0700：目录里是财务明细与 instance.meta，同机其他用户不该进得来
	if err := database.EnsureDataDir(cfg.DataDir); err != nil {
		log.Fatalf("准备数据目录失败：%v", err)
	}
	if dir := filepath.Dir(cfg.DBPath); dir != "" && dir != "." {
		if err := database.EnsureDataDir(dir); err != nil {
			log.Fatalf("准备数据库目录失败：%v", err)
		}
	}

	// instance 子命令不依赖数据库，必须能在"替换 db 文件之后、服务启动之前"执行：
	// 停容器 → 替换 db → `fastledger instance bump` → 启动（《落地方案》§8.3 恢复流程）。
	if args := os.Args[1:]; len(args) > 0 && args[0] == "instance" {
		handleInstance(cfg, args[1:])
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

	// INFRA-01：走到这里说明不是子命令，而是真正要启动 HTTP 服务——
	// 此时才要求 JWT 合法（该挡的仍然挡住，只是不再挡子命令）。
	if err := cfg.ValidateRuntime(); err != nil {
		log.Fatalf("配置校验失败：%v", err)
	}

	// 正常启动：只检查版本（GPT-R37：Check 纯只读，绝不建表）
	st, err := migrate.Check(engine)
	if err != nil {
		if errors.Is(err, migrate.ErrNotMigrated) {
			log.Fatalf("%v", err)
		}
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
	// GPT-R27：邀请 TTL 非法（≤0 或超上限）直接启动失败，不带着错误的有效期运行
	members, err := service.NewMemberService(engine, authSvc, cfg.InviteTTLDays)
	if err != nil {
		log.Fatalf("成员服务初始化失败：%v", err)
	}
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
func handleInstance(cfg *config.Config, args []string) {
	if len(args) == 0 || args[0] != "bump" {
		log.Fatalf("用法：instance bump（恢复备份后递增 restore_epoch）")
	}
	dataDir := cfg.DataDir
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
	auditBackupRestore(cfg, before.RestoreEpoch, after.RestoreEpoch)
}

// auditBackupRestore 备份恢复的审计（WBS 4.3）。
//
// 刻意做成 **best-effort**：本子命令必须能在【没有数据库】时执行——
// 《落地方案》§8.3 的恢复流程是"停容器 → 替换 db 文件 → instance bump → 启动"，
// bump 恰恰发生在 db 刚被替换之后，此时强制连库会把恢复流程本身卡死。
//
// 因此审计失败只告警、不阻断；restore_epoch 与 restored_at 已落在 instance.meta
// （不依赖数据库），构成恢复这件事的兜底留痕，不会因审计缺失而完全无据可查。
func auditBackupRestore(cfg *config.Config, beforeEpoch, afterEpoch int64) {
	engine, err := database.Init(cfg.DBPath)
	if err != nil {
		log.Printf("警告：无法打开数据库，备份恢复未写入 audit_logs（%v）；"+
			"restore_epoch %d 已记录在 instance.meta", err, afterEpoch)
		return
	}
	defer func() {
		if cerr := engine.Close(); cerr != nil {
			log.Printf("关闭数据库出错：%v", cerr)
		}
	}()
	// actor_uid 用哨兵 0：这是运维在命令行执行的操作，没有登录用户
	if err := service.AppendAudit(engine, model.AuditBookInstance, model.AuditActorSystem,
		model.AuditActionBackupRestore, model.AuditEntityInstance, afterEpoch,
		map[string]interface{}{"restoreEpoch": beforeEpoch}, "cli"); err != nil {
		log.Printf("警告：备份恢复审计写入失败（%v）；restore_epoch %d 已记录在 instance.meta",
			err, afterEpoch)
	}
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
//
// GPT-R34：参数解析【先于任何 DELETE】完成，且非法输入一律 fail-fast。
// 这里的删除不可逆，用"默认值顶替运维的真实意图"会把一次输入错误变成数据丢失。
func handleDBPurge(engine *xorm.Engine, args []string) {
	recycleTotal, auditTotal, bookCount, opts, err := purgeWithOptions(engine, args)
	if err != nil {
		log.Fatalf("db purge 未执行（参数非法，未触碰数据库）：%v", err)
	}
	log.Printf("清理完成：回收站硬清除 %d 条（TTL=%d 天，账本 %d 个）；审计清除 %d 条（保留 %d 天）",
		recycleTotal, opts.RecycleTTLDays, bookCount, auditTotal, opts.AuditRetentionDays)
}

// purgeOptions db purge 解析后的保留天数（天数均为已校验的正整数）
type purgeOptions struct {
	// RecycleTTLDays 回收站墓碑保留天数
	RecycleTTLDays int
	// AuditRetentionDays 审计日志保留天数
	AuditRetentionDays int
}

// purgeWithOptions 先严格解析参数，解析失败**立即返回错误且不触碰数据库**；
// 解析通过后才执行硬删除。返回（回收站清除数，审计清除数，账本数，生效参数，错误）。
func purgeWithOptions(engine *xorm.Engine, args []string) (int64, int64, int, *purgeOptions, error) {
	opts, err := resolvePurgeOptions(args)
	if err != nil {
		return 0, 0, 0, nil, err
	}
	recycleTotal, auditTotal, bookCount, err := runPurge(engine, opts)
	if err != nil {
		return 0, 0, 0, nil, err
	}
	return recycleTotal, auditTotal, bookCount, opts, nil
}

// runPurge 执行硬删除（调用方必须已通过 resolvePurgeOptions 校验）
func runPurge(engine *xorm.Engine, opts *purgeOptions) (int64, int64, int, error) {
	if engine == nil {
		return 0, 0, 0, errPurgeNoEngine
	}
	now := time.Now().Unix()
	// 回收站：按账本逐个清除（PurgeExpiredTransactions 以账本为界，避免误删他库）
	var bookIDs []int64
	if err := engine.Table("books").Select("id").Find(&bookIDs); err != nil {
		return 0, 0, 0, fmt.Errorf("读取账本列表失败：%w", err)
	}
	txs := service.NewTxService(engine, version.NewRepo())
	var recycleTotal int64
	for _, id := range bookIDs {
		n, err := txs.PurgeExpiredTransactions(id, opts.RecycleTTLDays, now)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("回收站清除失败（账本 %d）：%w", id, err)
		}
		recycleTotal += n
	}
	auditTotal, err := service.PurgeAuditLogs(engine, opts.AuditRetentionDays, now)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("审计保留期清除失败：%w", err)
	}
	return recycleTotal, auditTotal, len(bookIDs), nil
}

// resolvePurgeOptions 决定本次清理的保留天数，优先级：显式 CLI > 环境变量 > 默认值。
//
// GPT-R34 的核心：**区分"未提供"与"提供了但非法"**。
// 只有"未提供"才允许回落（那是运维确实没有表达偏好）；"提供了但非法"一律报错——
// 静默回退会让运维以为指定了较长保留期，实际按默认 30/365 天执行不可恢复的硬删除。
func resolvePurgeOptions(args []string) (*purgeOptions, error) {
	// 这里不走 config.Load()：该 CLI 只关心保留天数，不应被 JWT 校验等无关约束挡住
	recycleTTL, err := envRetentionDays(recycleTTLEnv, config.DefaultRecycleTTLDays)
	if err != nil {
		return nil, err
	}
	auditDays, err := envRetentionDays(auditRetentionEnv, config.DefaultAuditRetentionDays)
	if err != nil {
		return nil, err
	}
	opts := &purgeOptions{RecycleTTLDays: recycleTTL, AuditRetentionDays: auditDays}
	if err := applyPurgeFlags(opts, args); err != nil {
		return nil, err
	}
	return opts, nil
}

const (
	// recycleTTLEnv 回收站保留天数环境变量
	recycleTTLEnv = "EBK_RECYCLE_TTL_DAYS"
	// auditRetentionEnv 审计保留天数环境变量
	auditRetentionEnv = "EBK_AUDIT_RETENTION_DAYS"
	// recycleFlagName / auditFlagName CLI 覆盖参数名
	recycleFlagName = "--recycle-ttl-days"
	auditFlagName   = "--audit-retention-days"
)

// errPurgeNoEngine 数据库句柄缺失（防御性，正常 CLI 路径不会触发）
var errPurgeNoEngine = errors.New("db purge：数据库句柄缺失")

// applyPurgeFlags 严格解析命令行覆盖参数。
//
// 用标准库 flag 而非手写扫描：缺值、未知参数、格式错误都由 flag 明确报错，
// "未提供"与"提供了"通过 retentionFlag.set 区分（不能用 0 当哨兵——
// `--recycle-ttl-days 0` 是运维真实输入的非法值，必须报错而不是回落默认值）。
func applyPurgeFlags(opts *purgeOptions, args []string) error {
	fs := flag.NewFlagSet("db purge", flag.ContinueOnError)
	// 报错由本函数包装后交给调用方（log.Fatalf / 测试断言），不打到 stderr 两次
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	recycle := &retentionFlag{name: recycleFlagName}
	audit := &retentionFlag{name: auditFlagName}
	fs.Var(recycle, strings.TrimPrefix(recycleFlagName, "--"), "回收站保留天数（正整数）")
	fs.Var(audit, strings.TrimPrefix(auditFlagName, "--"), "审计保留天数（正整数）")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("解析 db purge 参数失败：%w", err)
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("db purge 不接受位置参数：%q（可用 %s / %s）",
			fs.Args(), recycleFlagName, auditFlagName)
	}
	if recycle.set {
		opts.RecycleTTLDays = recycle.value
	}
	if audit.set {
		opts.AuditRetentionDays = audit.value
	}
	return nil
}

// envRetentionDays 读取环境变量保留天数：未设置用默认值，设置了但非法则报错。
func envRetentionDays(key string, def int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def, nil
	}
	return parseRetentionDays(key, raw)
}

// parseRetentionDays 解析保留天数：必须是 (0, config.MaxRetentionDays] 内的整数。
//
// 刻意【不】做 TrimSpace 之类的"善意容错"：这是个不可逆删除的参数，
// " 30 " 这种带空白的输入要么被明确接受要么被明确拒绝，不能靠猜测。
// 这里选择拒绝（strconv.Atoi 对带空白的串会报错），让运维立刻看到问题。
//
// 返回值是 (天数, error) 而不是"失败就回落"的 (天数)：调用方需要据此决定是否终止，
// 而"回落默认值"正是 GPT-R34 要消灭的行为。
func parseRetentionDays(name, raw string) (int, error) {
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s 必须是整数，当前=%q", name, raw)
	}
	if n <= 0 {
		return 0, fmt.Errorf("%s 必须为正整数，当前=%d", name, n)
	}
	if n > config.MaxRetentionDays {
		return 0, fmt.Errorf("%s 超过上限 %d 天，当前=%d", name, config.MaxRetentionDays, n)
	}
	return n, nil
}

// retentionFlag 实现 flag.Value：记录"是否被显式设置"，并对非法值直接报错
type retentionFlag struct {
	name  string
	value int
	set   bool
}

// String 供 flag 打印默认值（未设置时为空串）
func (f *retentionFlag) String() string {
	if f == nil || !f.set {
		return ""
	}
	return strconv.Itoa(f.value)
}

// Set 解析并校验单个参数值；非法一律返回错误，不做任何回落
func (f *retentionFlag) Set(s string) error {
	n, err := parseRetentionDays(f.name, s)
	if err != nil {
		return err
	}
	f.value = n
	f.set = true
	return nil
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
