// Package config 加载运行配置。
//
// 约定（对齐《落地方案》§8.1 与 PRD §12 决策）：
//   - 所有环境变量统一 EBK_ 前缀，禁止出现裸 ENABLE_REGISTER 之类的写法（C-R05）。
//   - JWT 密钥空值 / 占位值 / 过短一律启动即失败（fail-fast，R08）。
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	// DefaultHTTPPort 默认监听端口
	DefaultHTTPPort = "8080"
	// DefaultDBPath 默认 SQLite 路径
	DefaultDBPath = "./data/fastledger.db"
	// DefaultDataDir 默认数据目录（instance.meta / 附件都落在这里，见指南 §26.1）
	DefaultDataDir = "./data"

	// minJWTSecretLen JWT 密钥最短长度，短于此视为不安全
	minJWTSecretLen = 32

	// DefaultAuditRetentionDays 审计日志默认保留天数（PRD §12.1：保留 1 年）
	DefaultAuditRetentionDays = 365
	// DefaultRecycleTTLDays 回收站默认保留天数（PRD §12.1 决策：30 天）
	DefaultRecycleTTLDays = 30
	// DefaultInviteTTLDays 邀请码默认有效期天数（PRD §12.1 决策：7 天）
	DefaultInviteTTLDays = 7
	// DefaultWSMaxConnPerUser 单用户 WebSocket 并发连接数上限默认值（SYNC-01）。
	//
	// 为什么必须设上限：一条 WS 连接 ≈ 2 个 goroutine + 1 个 FD + 256 槽发送缓冲，
	// 而订阅限流若只按连接计，多开连接就能把"单帧鉴权查询量封顶"线性放大。
	// 家庭 NAS 场景单人同时在线的设备数远小于 10。
	DefaultWSMaxConnPerUser = 10
	// DefaultWSMaxConnTotal 全局 WebSocket 并发连接数上限默认值（SYNC-01）。
	// 兜住"注册一堆账号各开 10 条"这类组合放大；家庭场景同时在线设备远小于 1000。
	DefaultWSMaxConnTotal = 1000
	// DefaultLoginMaxAttempts 登录失败限流默认配额（A6，§8.4）。
	DefaultLoginMaxAttempts = 10
	// DefaultLoginWindowMinutes 登录失败限流默认窗口/分钟（A6）。
	// 窗口 = 被打满后的瘫痪时长，故不照抄邀请的 1 小时。
	DefaultLoginWindowMinutes = 15
	// DefaultSyncPushPerMinute 单用户 sync/push 限流默认值/分钟（INFRA 复审：写路径优先）。
	DefaultSyncPushPerMinute = 60
	// DefaultSyncPullPerMinute 单用户 sync/pull 限流默认值/分钟（读路径放宽）。
	DefaultSyncPullPerMinute = 300
	// MaxRetentionDays 保留天数上限（100 年）。与 service.maxRetentionDays 同值：
	// 这里是【配置层】的启动期拦截，那里是【换算层】的运行时拦截，互为纵深防御。
	MaxRetentionDays = 36500
)

// jwtPlaceholders 常见的"没改过"的占位值
var jwtPlaceholders = []string{
	"changeme", "change-me", "secret", "your-secret",
	"请改成随机长字符串", "改成你自己的密钥",
}

// Config 运行配置
type Config struct {
	HTTPPort string
	DBPath   string

	// DataDir 数据目录（instance.meta 等实例级元信息，备份须排除该文件）
	DataDir string

	JWTSecret      string
	EnableRegister bool

	// RecycleTTLDays 回收站保留天数（PRD §12.1 决策：默认 30）
	RecycleTTLDays int
	// InviteTTLDays 邀请码有效期天数（PRD §12.1 决策：默认 7）
	InviteTTLDays int
	// AuditRetentionDays 审计日志保留天数（PRD §12.1 决策：保留 1 年）
	AuditRetentionDays int

	// WSMaxConnPerUser 单用户 WebSocket 并发连接数上限（SYNC-01；
	// 环境变量 EBK_WS_MAX_CONN_PER_USER，默认 10）
	WSMaxConnPerUser int
	// WSMaxConnTotal 全局 WebSocket 并发连接数上限（SYNC-01；
	// 环境变量 EBK_WS_MAX_CONN_TOTAL，默认 1000）
	WSMaxConnTotal int

	// LoginMaxAttempts 登录失败限流配额（A6；环境变量 EBK_LOGIN_MAX_ATTEMPTS，默认 10）。
	// 只对【失败】计数：成功登录不占配额（多设备正常登录不得误伤）。
	LoginMaxAttempts int
	// LoginWindowMinutes 登录失败限流窗口/分钟（A6；环境变量 EBK_LOGIN_WINDOW_MINUTES，
	// 默认 15）。窗口 = 被打满后的瘫痪时长，勿照抄邀请的 1 小时。
	LoginWindowMinutes int

	// SyncPushPerMinute 单用户 sync/push 限流/分钟（INFRA 复审；EBK_SYNC_PUSH_PER_MINUTE，
	// 默认 60）。push 是写路径，优先收紧。
	SyncPushPerMinute int
	// SyncPullPerMinute 单用户 sync/pull 限流/分钟（EBK_SYNC_PULL_PER_MINUTE，默认 300，
	// 读路径放宽）。与 push 各自独立 key 空间，也与登录/邀请限流完全隔离。
	SyncPullPerMinute int
}

// newConfig 从环境变量读取全部配置项（只读取、不校验）。
//
// 读取顺序即优先级：任何一个 envInt 非法都会在这里先失败，与重构前 Load 的行为一致。
func newConfig() (*Config, error) {
	recycleTTL, err := envInt("EBK_RECYCLE_TTL_DAYS", DefaultRecycleTTLDays)
	if err != nil {
		return nil, err
	}
	inviteTTL, err := envInt("EBK_INVITE_TTL_DAYS", DefaultInviteTTLDays)
	if err != nil {
		return nil, err
	}
	// 审计保留 1 年（PRD §12.1）；到期由 PurgeAuditLogs 硬清除
	auditDays, err := envInt("EBK_AUDIT_RETENTION_DAYS", DefaultAuditRetentionDays)
	if err != nil {
		return nil, err
	}
	// WS 连接数上限（SYNC-01）：envInt 已保证正整数，无需额外 validate 分支
	wsPerUser, err := envInt("EBK_WS_MAX_CONN_PER_USER", DefaultWSMaxConnPerUser)
	if err != nil {
		return nil, err
	}
	wsTotal, err := envInt("EBK_WS_MAX_CONN_TOTAL", DefaultWSMaxConnTotal)
	if err != nil {
		return nil, err
	}
	// EBK_ENABLE_REGISTER（INFRA-05）：非法值在此即失败，不静默回落
	enableRegister, err := envBool("EBK_ENABLE_REGISTER", false)
	if err != nil {
		return nil, err
	}
	// 登录失败限流（A6）：envInt 同款 fail-fast——配错成 0 / 负数等于把闸门拆掉，启动即拦
	loginMax, err := envInt("EBK_LOGIN_MAX_ATTEMPTS", DefaultLoginMaxAttempts)
	if err != nil {
		return nil, err
	}
	loginWindow, err := envInt("EBK_LOGIN_WINDOW_MINUTES", DefaultLoginWindowMinutes)
	if err != nil {
		return nil, err
	}
	// 同步限流（INFRA 复审）：push 写路径收紧、pull 读路径放宽
	syncPush, err := envInt("EBK_SYNC_PUSH_PER_MINUTE", DefaultSyncPushPerMinute)
	if err != nil {
		return nil, err
	}
	syncPull, err := envInt("EBK_SYNC_PULL_PER_MINUTE", DefaultSyncPullPerMinute)
	if err != nil {
		return nil, err
	}
	return &Config{
		HTTPPort:           envOr("EBK_HTTP_PORT", DefaultHTTPPort),
		DBPath:             envOr("EBK_DB_PATH", DefaultDBPath),
		DataDir:            envOr("EBK_DATA_DIR", DefaultDataDir),
		JWTSecret:          os.Getenv("EBK_JWT_SECRET"),
		EnableRegister:     enableRegister,
		RecycleTTLDays:     recycleTTL,
		InviteTTLDays:      inviteTTL,
		AuditRetentionDays: auditDays,
		WSMaxConnPerUser:   wsPerUser,
		WSMaxConnTotal:     wsTotal,
		LoginMaxAttempts:   loginMax,
		LoginWindowMinutes: loginWindow,
		SyncPushPerMinute:  syncPush,
		SyncPullPerMinute:  syncPull,
	}, nil
}

// Load 从环境变量加载配置并做【全量】校验（含 JWT）。服务进程启动入口使用。
//
// GPT-R34：涉及保留期的整数变量若非法，在此即【启动失败】，不做静默回退。
//
// INFRA-01：`instance bump` / `db migrate` / `db purge` / `user create` 这些子命令
// 不签发令牌、也不该被 JWT 校验挡在门外（恢复流程发生在"数据库刚被替换、
// 服务还没起来"的时刻）。因此入口改用 LoadBase，只有真正启动 HTTP 服务才走本函数。
func Load() (*Config, error) {
	c, err := newConfig()
	if err != nil {
		return nil, err
	}
	if err := c.validate(); err != nil {
		return nil, err
	}
	return c, nil
}

// LoadBase 只做【通用】校验（端口 / 路径 / 保留期），不含 JWT。CLI 子命令使用。
//
// 为什么子命令也要校验保留期：`db purge` 真的会按 EBK_RECYCLE_TTL_DAYS 硬删除，
// 这些值对子命令同样是"会触发不可逆操作"的配置，必须在动手前拦住。
func LoadBase() (*Config, error) {
	c, err := newConfig()
	if err != nil {
		return nil, err
	}
	if err := c.validateCommon(); err != nil {
		return nil, err
	}
	return c, nil
}

// validate 全量校验：通用校验 + JWT。
//
// INFRA-01：**顺序即契约**——保持与重构前逐字一致（端口/路径 → JWT → 保留期），
// 不得因为拆分而改变"两个配置同时非法时报哪一条"。
func (c *Config) validate() error {
	if err := c.validatePaths(); err != nil {
		return err
	}
	if err := c.ValidateRuntime(); err != nil {
		return err
	}
	return c.validateRetention()
}

// validateCommon 通用校验：路径 + 保留期（不含 JWT，子命令可用）
func (c *Config) validateCommon() error {
	if err := c.validatePaths(); err != nil {
		return err
	}
	return c.validateRetention()
}

// validatePaths 端口与路径必填项（INFRA-06 的端口数值校验也在这里）
func (c *Config) validatePaths() error {
	if strings.TrimSpace(c.HTTPPort) == "" {
		return fmt.Errorf("EBK_HTTP_PORT 不能为空")
	}
	if err := validatePort(c.HTTPPort); err != nil {
		return err
	}
	if strings.TrimSpace(c.DBPath) == "" {
		return fmt.Errorf("EBK_DB_PATH 不能为空")
	}
	if strings.TrimSpace(c.DataDir) == "" {
		return fmt.Errorf("EBK_DATA_DIR 不能为空")
	}
	return nil
}

// validateRetention 保留期取值（正整数 + 上限），见 GPT-R27 / GPT-R34
func (c *Config) validateRetention() error {
	if c.RecycleTTLDays <= 0 {
		return fmt.Errorf("EBK_RECYCLE_TTL_DAYS 必须为正整数，当前=%d", c.RecycleTTLDays)
	}
	if c.InviteTTLDays <= 0 {
		return fmt.Errorf("EBK_INVITE_TTL_DAYS 必须为正整数，当前=%d", c.InviteTTLDays)
	}
	if c.AuditRetentionDays <= 0 {
		return fmt.Errorf("EBK_AUDIT_RETENTION_DAYS 必须为正整数，当前=%d", c.AuditRetentionDays)
	}
	// GPT-R27 / R34：保留期上限在【启动期】拦截，避免配错成超大值后
	// 由清理任务在运行期溢出（那会造成不可逆的批量清除）
	for _, v := range []struct {
		name string
		n    int
	}{
		{"EBK_RECYCLE_TTL_DAYS", c.RecycleTTLDays},
		{"EBK_INVITE_TTL_DAYS", c.InviteTTLDays},
		{"EBK_AUDIT_RETENTION_DAYS", c.AuditRetentionDays},
	} {
		if v.n > MaxRetentionDays {
			return fmt.Errorf("%s 超过上限 %d 天，当前=%d", v.name, MaxRetentionDays, v.n)
		}
	}
	return nil
}

// ValidateRuntime 校验【仅服务运行期】需要的配置（JWT）。
//
// 导出而非私有：main.go 在 LoadBase 通过后、真正启动 HTTP 服务之前调用它，
// 保证"该挡的仍然挡住"，只是不再挡子命令。
func (c *Config) ValidateRuntime() error {
	return validateJWTSecret(c.JWTSecret)
}

// validatePort 端口必须是 1-65535 的整数（INFRA-06）。
//
// 原实现只查"非空"：EBK_HTTP_PORT=abc 会在 ListenAndServe 时才炸，
// 而且报错是 net 包的原始信息，运维看不出是哪个环境变量配错了。
func validatePort(raw string) error {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("EBK_HTTP_PORT 必须是 1-65535 之间的整数，当前=%q", raw)
	}
	if n < 1 || n > 65535 {
		return fmt.Errorf("EBK_HTTP_PORT 必须是 1-65535 之间的整数，当前=%d", n)
	}
	return nil
}

// validateJWTSecret 空值 / 占位值 / 过短 一律启动失败
func validateJWTSecret(secret string) error {
	s := strings.TrimSpace(secret)
	if s == "" {
		return fmt.Errorf("EBK_JWT_SECRET 未设置：禁止使用默认密钥，请配置至少 %d 位随机字符串", minJWTSecretLen)
	}
	for _, p := range jwtPlaceholders {
		if strings.EqualFold(s, p) {
			return fmt.Errorf("EBK_JWT_SECRET 仍为占位值 %q，请改为随机长字符串", p)
		}
	}
	if len(s) < minJWTSecretLen {
		return fmt.Errorf("EBK_JWT_SECRET 过短（%d < %d）", len(s), minJWTSecretLen)
	}
	return nil
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

// envBool 解析布尔型环境变量（INFRA-05：与 envInt 同口径——非法值报错，不静默回落）。
//
// 原实现对 "enable" / "2" / "Y" 等一律返回默认值：运维写 EBK_ENABLE_REGISTER=enable
// 会看到服务正常启动、日志里"公开注册=false"与预期相反，却没有任何提示指向配置拼写。
// 这与 envInt 在 GPT-R34 之前的缺陷同构，一并消灭。
func envBool(key string, def bool) (bool, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def, nil // 未设置/空白 = 用户没表达偏好，用默认值
	}
	switch strings.ToLower(raw) {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf(
			"%s 必须是 1/true/yes/on 或 0/false/no/off（不区分大小写），当前=%q", key, os.Getenv(key))
	}
}

// envInt 解析正整数环境变量。
//
// GPT-R34：非法值【绝不静默回退默认值】。原先 `abc` / `-5` 会静默退回默认值，
// 而这些变量正是回收站保留天数、审计保留天数——静默回退等于用一个用户从未
// 同意过的保留期去执行【不可逆的清理】。故非法输入一律返回错误（启动失败）。
//
// 仅在【变量未设置】时使用默认值——那是用户确实没有表达偏好，与"配错了"不同。
func envInt(key string, def int) (int, error) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s 必须是整数，当前=%q", key, v)
	}
	if n <= 0 {
		return 0, fmt.Errorf("%s 必须为正整数，当前=%d", key, n)
	}
	return n, nil
}
