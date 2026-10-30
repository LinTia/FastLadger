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
}

// Load 从环境变量加载配置并校验
func Load() (*Config, error) {
	c := &Config{
		HTTPPort:       envOr("EBK_HTTP_PORT", DefaultHTTPPort),
		DBPath:         envOr("EBK_DB_PATH", DefaultDBPath),
		DataDir:        envOr("EBK_DATA_DIR", DefaultDataDir),
		JWTSecret:      os.Getenv("EBK_JWT_SECRET"),
		EnableRegister: envBool("EBK_ENABLE_REGISTER", false),
		RecycleTTLDays: envInt("EBK_RECYCLE_TTL_DAYS", DefaultRecycleTTLDays),
		InviteTTLDays:  envInt("EBK_INVITE_TTL_DAYS", 7),
		// 审计保留 1 年（PRD §12.1）；到期由 PurgeAuditLogs 硬清除
		AuditRetentionDays: envInt("EBK_AUDIT_RETENTION_DAYS", DefaultAuditRetentionDays),
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

func envInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return n
}
