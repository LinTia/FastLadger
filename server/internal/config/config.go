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
}

// Load 从环境变量加载配置并校验。
//
// GPT-R34：涉及保留期的整数变量若非法，在此即【启动失败】，不做静默回退。
func Load() (*Config, error) {
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
	c := &Config{
		HTTPPort:           envOr("EBK_HTTP_PORT", DefaultHTTPPort),
		DBPath:             envOr("EBK_DB_PATH", DefaultDBPath),
		DataDir:            envOr("EBK_DATA_DIR", DefaultDataDir),
		JWTSecret:          os.Getenv("EBK_JWT_SECRET"),
		EnableRegister:     envBool("EBK_ENABLE_REGISTER", false),
		RecycleTTLDays:     recycleTTL,
		InviteTTLDays:      inviteTTL,
		AuditRetentionDays: auditDays,
		WSMaxConnPerUser:   wsPerUser,
		WSMaxConnTotal:     wsTotal,
	}
	if err := c.validate(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Config) validate() error {
	if strings.TrimSpace(c.HTTPPort) == "" {
		return fmt.Errorf("EBK_HTTP_PORT 不能为空")
	}
	if strings.TrimSpace(c.DBPath) == "" {
		return fmt.Errorf("EBK_DB_PATH 不能为空")
	}
	if strings.TrimSpace(c.DataDir) == "" {
		return fmt.Errorf("EBK_DATA_DIR 不能为空")
	}
	if err := validateJWTSecret(c.JWTSecret); err != nil {
		return err
	}
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

func envBool(key string, def bool) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
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
