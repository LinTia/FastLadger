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
	"unicode/utf8"

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
		// INFRA-02：不同形状的版本不一致需要【不同的处置】，
		// 一句"请先执行 db migrate"对降级运行是死路（migrate 救不了它）
		var vme *migrate.VersionMismatchError
		if errors.As(err, &vme) {
			switch {
			case len(vme.Unknown) > 0:
				log.Fatalf("迁移状态检查失败：%v\n"+
					"处置：库比程序新（可能在降级运行）——请升级程序版本，或恢复与程序匹配的数据库备份；"+
					"`db migrate` 无法修复此状态（unknown=%v）", err, vme.Unknown)
			case len(vme.Missing) > 0:
				log.Fatalf("迁移状态检查失败：%v\n"+
					"处置：请先执行 `fastledger db migrate`（missing=%v）。"+
					"若 migrate 仍失败，说明版本记录与真实结构不一致——"+
					"【不要】手工 DELETE schema_migrations 里的版本记录（那会诱发非幂等 DDL 重跑失败），"+
					"应恢复数据库备份或重建", err, vme.Missing)
			default:
				log.Fatalf("迁移状态检查失败：%v\n"+
					"处置：版本号齐但结构缺失/被改动——请恢复数据库备份或重建，"+
					"勿手工增删 schema_migrations 的版本记录", err)
			}
		}
		log.Fatalf("迁移状态检查失败：%v", err)
	}
	// N1：理论上 Check 出错即退出，走到这里 st.OK 必为 true。
	// 这个分支是防御性保留：Check 的实现将来若改为"报告状态但不报错"（例如加 --report 之类），
	// 这里仍是最后一道闸，不能删。
	if !st.OK {
		log.Fatalf("schema 版本不一致（当前 v%d，期望 v%d）：请先执行 `fastledger db migrate`",
			st.Current, st.Expected)
	}
	warnUnbackfilledTransfers(engine) // INFRA-11：0006 只回填了 related_id>0 的历史转账

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
	// N4：成功日志必须打【绝对路径】——运维用相对路径/环境变量时，
	// 只有绝对路径才能让人一眼看出"到底改的是哪个文件"。
	absPath, err := filepath.Abs(instance.Path(dataDir))
	if err != nil {
		absPath = instance.Path(dataDir)
	}
	before, created, err := instance.LoadOrCreateWithStatus(dataDir)
	if err != nil {
		log.Fatalf("读取实例元信息失败：%v", err)
	}
	if created {
		// N4：错误目录下 bump 会静默新建 meta 并递增它，真的实例纹丝不动。
		// 这里必须显著警告，让运维立刻检查 EBK_DATA_DIR 是否指错。
		log.Printf("警告：%s 原本不存在，本次是【新建】而不是读取已有实例！\n"+
			"  当前 EBK_DATA_DIR=%q。若这与预期不符，请立即停止：新建意味着全新的 instance_id，"+
			"客户端会把它视为另一个实例，原实例的恢复世代将不会被递增。", absPath, dataDir)
	}
	after, err := instance.Bump(dataDir)
	if err != nil {
		log.Fatalf("递增 restore_epoch 失败：%v", err)
	}
	log.Printf("restore_epoch：%d → %d（%s，文件 %s）",
		before.RestoreEpoch, after.RestoreEpoch, after.RestoredAt, absPath)
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
	warnUnbackfilledTransfers(engine) // INFRA-11
}

// warnUnbackfilledTransfers INFRA-11：0006 只回填了 related_id > 0 的历史转账
// （不能改已发布的 0006），related_id=0 的转账行 direction 会一直是 0，
// 而这类行的删除/恢复会被服务端拒绝。这里做只读统计，>0 时给运维一条告警。
//
// 刻意不做修复、也不阻断：怎么处理属于数据治理决策（重建记录 / 保留历史），
// 框架不该替运维做。查询失败（如表尚不存在）静默返回——这只是提示，不是门禁。
func warnUnbackfilledTransfers(engine *xorm.Engine) {
	if engine == nil {
		return
	}
	var n int64
	if err := engine.DB().DB.QueryRow(
		`SELECT COUNT(1) FROM transactions WHERE type = 3 AND direction = 0 AND related_id = 0`,
	).Scan(&n); err != nil {
		return
	}
	if n > 0 {
		log.Printf("警告：存在 %d 条历史转账未回填方向（type=3 且 direction=0 且 related_id=0，"+
			"0006 只回填 related_id>0 的行），对它们的删除/恢复会被拒绝；"+
			"如需修复请依据《转账入口裁决》重建对应记录", n)
	}
}

// handleDBPurge 回收站与审计的到期硬清除（WBS 4.1 / 4.3，PRD §12.1）。
//
// 与迁移同一口径：**仅 CLI 手动执行**，服务启动绝不静默删数据。
// 保留天数默认取环境变量（EBK_RECYCLE_TTL_DAYS / EBK_AUDIT_RETENTION_DAYS），
// 可用 --recycle-ttl-days / --audit-retention-days 覆盖（便于演练与应急）。
//
// GPT-R34：参数解析【先于任何 DELETE】完成，且非法输入一律 fail-fast。
// GPT-R38/INFRA-08：--dry-run 只扫描统计、绝不 DELETE；正式执行改为
// 【先全量扫描、再执行】两段式，中途失败会报告已完成/未完成的账本清单。
func handleDBPurge(engine *xorm.Engine, args []string) {
	res, opts, err := purgeWithOptions(engine, args)
	if err != nil {
		if res != nil && res.DryRun {
			// dry-run 的失败同样是"未触碰数据库"，但语义要讲清楚
			log.Fatalf("db purge --dry-run 未执行（参数/扫描失败，未删除任何数据）：%v", err)
		}
		log.Fatalf("db purge 未执行（参数非法，未触碰数据库）：%v", err)
	}
	if opts.DryRun {
		log.Printf("[dry-run] 预览：回收站将硬清除 %d 条（TTL=%d 天，明细：%s）；"+
			"审计将清除 %d 条（保留 %d 天）。本次【未删除任何数据】，去掉 --dry-run 后执行。",
			res.Recycled, opts.RecycleTTLDays, res.bookDetail(), res.Audited, opts.AuditRetentionDays)
		return
	}
	log.Printf("清理完成：回收站硬清除 %d 条（TTL=%d 天，账本 %d 个）；审计清除 %d 条（保留 %d 天）",
		res.Recycled, opts.RecycleTTLDays, len(res.Books), res.Audited, opts.AuditRetentionDays)
}

// purgeOptions db purge 解析后的参数（天数均为已校验的正整数）
type purgeOptions struct {
	// RecycleTTLDays 回收站墓碑保留天数
	RecycleTTLDays int
	// AuditRetentionDays 审计日志保留天数
	AuditRetentionDays int
	// DryRun 只扫描统计，不执行任何 DELETE（INFRA-08）
	DryRun bool
}

// purgeBookPlan 单个账本的清除计划/结果
type purgeBookPlan struct {
	BookID  int64
	Pending int64 // 扫描阶段：将删除的条数；执行阶段：实际删除的条数
}

// purgeResult 一次 purge 的统计结果
type purgeResult struct {
	// DryRun true = 本次只做了扫描
	DryRun bool
	// Books 每个账本的计划/实际条数
	Books []purgeBookPlan
	// Recycled / Audited 回收站与审计的条数合计
	Recycled int64
	Audited  int64
}

// bookDetail 人读的按账本明细（"1:3条 2:0条"）
func (r *purgeResult) bookDetail() string {
	parts := make([]string, 0, len(r.Books))
	for _, b := range r.Books {
		parts = append(parts, fmt.Sprintf("账本%d:%d条", b.BookID, b.Pending))
	}
	if len(parts) == 0 {
		return "（无账本）"
	}
	return strings.Join(parts, " ")
}

// purgeProgress 中途失败时给运维的进度清单
type purgeProgress struct {
	// Completed 已完成硬清除的账本
	Completed []int64
	// Pending 尚未处理的账本（重跑会继续处理它们）
	Pending []int64
}

// purgeWithOptions 先严格解析参数，解析失败**立即返回错误且不触碰数据库**；
// 随后扫描（只读），--dry-run 到此为止，正式执行才 DELETE。
func purgeWithOptions(engine *xorm.Engine, args []string) (*purgeResult, *purgeOptions, error) {
	opts, err := resolvePurgeOptions(args)
	if err != nil {
		return nil, nil, err
	}
	plan, err := scanPurge(engine, opts)
	if err != nil {
		return nil, nil, err
	}
	if opts.DryRun {
		res := &purgeResult{DryRun: true, Books: plan.Books}
		for _, b := range plan.Books {
			res.Recycled += b.Pending
		}
		res.Audited = plan.AuditPending
		return res, opts, nil
	}
	res, err := executePurge(engine, plan, opts)
	if err != nil {
		return res, opts, err
	}
	return res, opts, nil
}

// scanPurge 只读扫描：统计各账本到期墓碑与到期审计的条数（INFRA-08 第一步）
func scanPurge(engine *xorm.Engine, opts *purgeOptions) (*purgePlan, error) {
	if engine == nil {
		return nil, errPurgeNoEngine
	}
	now := time.Now().Unix()
	// cutoff 与 service.retentionCutoff 同口径（保留 N 天 = now - N*86400）
	cutoff := now - int64(opts.RecycleTTLDays)*secondsPerDay
	plan := &purgePlan{Cutoff: cutoff, Now: now}
	var bookIDs []int64
	if err := engine.Table("books").Select("id").Find(&bookIDs); err != nil {
		return nil, fmt.Errorf("读取账本列表失败：%w", err)
	}
	for _, id := range bookIDs {
		var n int64
		// 与 PurgeExpiredTransactions 的 WHERE 逐字一致，统计才不是空头支票
		if err := engine.DB().DB.QueryRow(
			`SELECT COUNT(1) FROM transactions WHERE book_id = ? AND deleted = 1 AND deleted_at > 0 AND deleted_at <= ?`,
			id, cutoff).Scan(&n); err != nil {
			return nil, fmt.Errorf("统计账本 %d 的到期墓碑失败：%w", id, err)
		}
		plan.Books = append(plan.Books, purgeBookPlan{BookID: id, Pending: n})
	}
	var audit int64
	if err := engine.DB().DB.QueryRow(
		`SELECT COUNT(1) FROM audit_logs WHERE at <= ?`,
		now-int64(opts.AuditRetentionDays)*secondsPerDay).Scan(&audit); err != nil {
		return nil, fmt.Errorf("统计到期审计失败：%w", err)
	}
	plan.AuditPending = audit
	return plan, nil
}

// executePurge 按计划执行硬删除（INFRA-08 第二步；失败时报告已完成/未完成账本）
func executePurge(engine *xorm.Engine, plan *purgePlan, opts *purgeOptions) (*purgeResult, error) {
	res := &purgeResult{Books: make([]purgeBookPlan, len(plan.Books))}
	txs := service.NewTxService(engine, version.NewRepo())
	var progress purgeProgress
	for i, b := range plan.Books {
		n, err := purgeBookFn(txs, b.BookID, opts.RecycleTTLDays, plan.Now)
		if err != nil {
			for _, bb := range plan.Books[i:] {
				progress.Pending = append(progress.Pending, bb.BookID)
			}
			return res, fmt.Errorf("回收站清除失败（账本 %d）：%w（已完成：%v；未完成：%v；审计未动）",
				b.BookID, err, progress.Completed, progress.Pending)
		}
		progress.Completed = append(progress.Completed, b.BookID)
		res.Books[i] = purgeBookPlan{BookID: b.BookID, Pending: n}
		res.Recycled += n
	}
	audited, err := purgeAuditFn(engine, opts.AuditRetentionDays, plan.Now)
	if err != nil {
		return res, fmt.Errorf("审计保留期清除失败：%w（回收站已完成：%v）", err, progress.Completed)
	}
	res.Audited = audited
	return res, nil
}

// purgePlan 扫描阶段的计划
type purgePlan struct {
	Books        []purgeBookPlan
	AuditPending int64
	Cutoff       int64
	Now          int64
}

// purgeBookFn / purgeAuditFn 生产路径指向真实清除；测试可注入故障（INFRA-08）
var (
	purgeBookFn = func(txs *service.TxService, bookID int64, ttlDays int, now int64) (int64, error) {
		return txs.PurgeExpiredTransactions(bookID, ttlDays, now)
	}
	purgeAuditFn = func(engine *xorm.Engine, retentionDays int, now int64) (int64, error) {
		return service.PurgeAuditLogs(engine, retentionDays, now)
	}
)

// secondsPerDay 与 service.secondsPerDay 同值（保留天数换算口径一致）
const secondsPerDay = 86400

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
	dryRun := false
	fs.BoolVar(&dryRun, "dry-run", false, "只扫描统计将删除多少条，不执行任何 DELETE（INFRA-08）")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("解析 db purge 参数失败：%w", err)
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("db purge 不接受位置参数：%q（可用 %s / %s / --dry-run）",
			fs.Args(), recycleFlagName, auditFlagName)
	}
	if recycle.set {
		opts.RecycleTTLDays = recycle.value
	}
	if audit.set {
		opts.AuditRetentionDays = audit.value
	}
	opts.DryRun = dryRun
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

// cliUsernameRules / cliPasswordMaxBytes 与 service 侧（GPT-R45）同一套口径。
//
// 口径来源：internal/service/auth.go 的 validateUsername / maxPasswordBytes。
// 这里不直接调用它，是因为它连同一大串服务依赖一起初始化，CLI 只需要规则本身；
// 两侧规则必须保持一致——改动任意一侧都要同步另一侧（见各自注释）。
const (
	// cliMaxUsernameRunes 用户名主口径：1~64 个 Unicode 码点（对齐 VARCHAR(64)）
	cliMaxUsernameRunes = 64
	// cliMaxUsernameBytes 用户名兜底口径：UTF-8 编码不超过 255 字节
	cliMaxUsernameBytes = 255
	// maxPasswordBytes bcrypt 的 72 字节上限（超出部分会被静默丢弃）
	maxPasswordBytes = 72
)

// validateCLIUsername 复用 API 侧同套用户名边界（INFRA-07）
func validateCLIUsername(username string) error {
	if strings.TrimSpace(username) == "" {
		return fmt.Errorf("用户名不能为空")
	}
	if n := utf8.RuneCountInString(username); n > cliMaxUsernameRunes {
		return fmt.Errorf("用户名长度超限：1~%d 个字符，实际 %d 个", cliMaxUsernameRunes, n)
	}
	if b := len(username); b > cliMaxUsernameBytes {
		return fmt.Errorf("用户名过长：UTF-8 编码后最多 %d 字节，实际 %d 字节", cliMaxUsernameBytes, b)
	}
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
	// INFRA-07：CLI 必须复用 API 侧同套输入边界（GPT-R45）。
	// 原实现只查"非空 + 密码 ≥8 位"：超长用户名会撞 schema 约束直接 500 文案、
	// 超 72 字节的密码会被 bcrypt 静默截断（用户实际口令与他输入的不一致）。
	if err := validateCLIUsername(username); err != nil {
		log.Fatalf("%v", err)
	}
	if len(password) < 8 {
		log.Fatalf("密码至少 8 位")
	}
	if len(password) > maxPasswordBytes {
		log.Fatalf("密码过长：最多 %d 字节（超出部分 bcrypt 会丢弃）", maxPasswordBytes)
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
