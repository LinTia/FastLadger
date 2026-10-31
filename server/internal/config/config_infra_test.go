package config

// INFRA-01 / INFRA-05 / INFRA-06 的回归测试。
//
// INFRA-01：LoadBase 不含 JWT 校验（子命令可用），Load 仍含（服务入口该挡的挡住）。
//   原缺陷：main 一进来就 config.Load()，`instance bump` / `db migrate` /
//   `db purge` / `user create` 在没配 JWT 的环境全被挡死——而恢复流程恰恰
//   发生在"数据库刚被替换、服务还没起来"的时刻。
//
// INFRA-05：envBool 与 envInt 同口径，非法值报错不静默回落。
//   （吸收自复审探针 zz_infra_probe_test.go 的 TestInfraProbeEnvBoolSilentlyFallsBackOnGarbage）
//
// INFRA-06：端口必须是 1-65535 的整数，不能等到 ListenAndServe 才炸。

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// TestLoadBaseSucceedsWithoutJWT 子命令场景：不配 JWT 也必须能拿到配置
func TestLoadBaseSucceedsWithoutJWT(t *testing.T) {
	withEnv(t, map[string]string{"EBK_JWT_SECRET": ""})
	c, err := LoadBase()
	if err != nil {
		t.Fatalf("LoadBase 不应被 JWT 校验挡住：%v", err)
	}
	if c == nil || c.DBPath == "" {
		t.Fatalf("LoadBase 应返回可用配置，实际 %+v", c)
	}
	if c.HTTPPort != DefaultHTTPPort {
		t.Fatalf("默认端口应为 %s，实际 %s", DefaultHTTPPort, c.HTTPPort)
	}
}

// TestLoadStillFailsWithoutJWT 服务入口场景：无 JWT 仍必须失败（不能把挡的也放开）
func TestLoadStillFailsWithoutJWT(t *testing.T) {
	withEnv(t, map[string]string{"EBK_JWT_SECRET": ""})
	if _, err := Load(); err == nil {
		t.Fatal("Load 无 JWT 时应失败（服务启动的 JWT fail-fast 不能丢）")
	}
}

// TestLoadBaseRejectsBadRetention 保留期对子命令同样要拦（db purge 真的会按它硬删）
func TestLoadBaseRejectsBadRetention(t *testing.T) {
	withEnv(t, map[string]string{
		"EBK_JWT_SECRET":       "",
		"EBK_RECYCLE_TTL_DAYS": "abc",
	})
	if _, err := LoadBase(); err == nil {
		t.Fatal("保留期非法时 LoadBase 应失败（它决定 db purge 删多少数据）")
	}
}

// TestValidateRuntime FailsWithoutJWT / PassesWithValid JWT 校验本身的行为
func TestValidateRuntime(t *testing.T) {
	t.Run("无 JWT 应失败", func(t *testing.T) {
		withEnv(t, map[string]string{"EBK_JWT_SECRET": ""})
		c, err := LoadBase()
		if err != nil {
			t.Fatalf("LoadBase 不应失败：%v", err)
		}
		if err := c.ValidateRuntime(); err == nil {
			t.Fatal("无 JWT 时 ValidateRuntime 应失败")
		}
	})
	t.Run("合法 JWT 应通过", func(t *testing.T) {
		withEnv(t, map[string]string{"EBK_JWT_SECRET": strings.Repeat("s", 40)})
		c, err := LoadBase()
		if err != nil {
			t.Fatalf("LoadBase 不应失败：%v", err)
		}
		if err := c.ValidateRuntime(); err != nil {
			t.Fatalf("合法 JWT 应通过：%v", err)
		}
	})
}

// TestLoadErrorOrderPreservedINFRA01 拆分后 Load 的报错顺序必须与重构前逐字一致。
//
// 历史顺序是【路径 → JWT → 保留期】——运维按报错顺序修配置，顺序一变就会误导。
// 注意两点构造技巧：
//   - 端口用 "abc" 而不是空串：envOr 会把空串当"未设置"回落默认值，
//     "端口为空"这条分支从环境变量根本到不了（重构前也一样）；
//   - 保留期用"超过上限"而不是 0：0 会在 newConfig 的 envInt 阶段就报错，
//     那是读取层的事，与本次拆分的 validate 顺序无关。
func TestLoadErrorOrderPreservedINFRA01(t *testing.T) {
	withEnv(t, map[string]string{
		"EBK_HTTP_PORT":        "abc",
		"EBK_JWT_SECRET":       "",
		"EBK_RECYCLE_TTL_DAYS": "36501",
	})
	_, err := Load()
	if err == nil {
		t.Fatal("多项非法应报错")
	}
	if !strings.Contains(err.Error(), "EBK_HTTP_PORT") {
		t.Fatalf("路径/端口应最先被报出，实际：%v", err)
	}

	withEnv(t, map[string]string{"EBK_HTTP_PORT": DefaultHTTPPort})
	_, err = Load()
	if err == nil {
		t.Fatal("JWT 缺失应报错")
	}
	if !strings.Contains(err.Error(), "EBK_JWT_SECRET") {
		t.Fatalf("端口已修复后应轮到 JWT，实际：%v", err)
	}

	withEnv(t, map[string]string{"EBK_JWT_SECRET": strings.Repeat("s", 40)})
	_, err = Load()
	if err == nil {
		t.Fatal("保留期超上限应报错")
	}
	if !strings.Contains(err.Error(), "超过上限") {
		t.Fatalf("端口与 JWT 已修复后应轮到保留期，实际：%v", err)
	}
}

// TestEnvBoolStrictINFRA05 非法布尔值必须报错（吸收自复审探针）
func TestEnvBoolStrictINFRA05(t *testing.T) {
	for _, raw := range []string{"enable", "2", "Y", "yes please", "真"} {
		withEnv(t, map[string]string{"EBK_TEST_BOOL": raw})
		if got, err := envBool("EBK_TEST_BOOL", false); err == nil {
			t.Fatalf("EBK_TEST_BOOL=%q：非法值应报错，实际静默得到 %v", raw, got)
		}
	}
	for _, c := range []struct {
		raw  string
		want bool
	}{
		{"1", true}, {"true", true}, {"TRUE", true}, {"Yes", true}, {"on", true},
		{"0", false}, {"false", false}, {"No", false}, {"OFF", false},
	} {
		withEnv(t, map[string]string{"EBK_TEST_BOOL": c.raw})
		got, err := envBool("EBK_TEST_BOOL", false)
		if err != nil {
			t.Fatalf("EBK_TEST_BOOL=%q：合法值不应报错：%v", c.raw, err)
		}
		if got != c.want {
			t.Fatalf("EBK_TEST_BOOL=%q 期望 %v，实际 %v", c.raw, c.want, got)
		}
	}
	// 未设置/空白 → 默认值（与 envInt 同口径：空白是"没配"，不是"配错"）
	for _, raw := range []string{"", "   "} {
		withEnv(t, map[string]string{"EBK_TEST_BOOL": raw})
		got, err := envBool("EBK_TEST_BOOL", true)
		if err != nil {
			t.Fatalf("空白值 %q 应视为未设置：%v", raw, err)
		}
		if got != true {
			t.Fatalf("空白值应回默认值 true，实际 %v", got)
		}
	}
}

// TestLoadRejectsBadRegisterEnv Load 全链路上非法布尔值要真的挡住启动
func TestLoadRejectsBadRegisterEnv(t *testing.T) {
	withEnv(t, map[string]string{
		"EBK_JWT_SECRET":      strings.Repeat("s", 40),
		"EBK_ENABLE_REGISTER": "enable",
	})
	if _, err := Load(); err == nil {
		t.Fatal("EBK_ENABLE_REGISTER=enable 应导致启动失败（原先是静默回落 false）")
	}
}

// TestValidatePortINFRA06 端口数值校验（INFRA-06）
func TestValidatePortINFRA06(t *testing.T) {
	for _, bad := range []string{"abc", "1.5", "0", "-1", "65536", "99999", ""} {
		if err := validatePort(bad); err == nil {
			t.Fatalf("端口 %q 应被拒绝", bad)
		}
	}
	for _, good := range []string{"1", "80", "8080", "65535", " 8080 "} {
		if err := validatePort(good); err != nil {
			t.Fatalf("端口 %q 应被接受：%v", good, err)
		}
	}
}

// TestLoadRejectsBadPortINFRA06 Load 全链路上坏端口要真的挡住启动
func TestLoadRejectsBadPortINFRA06(t *testing.T) {
	withEnv(t, map[string]string{
		"EBK_JWT_SECRET": strings.Repeat("s", 40),
		"EBK_HTTP_PORT":  "abc",
	})
	_, err := Load()
	if err == nil {
		t.Fatal("EBK_HTTP_PORT=abc 应导致启动失败（原先是 ListenAndServe 才炸）")
	}
	if !strings.Contains(err.Error(), "EBK_HTTP_PORT") {
		t.Fatalf("错误信息应指明是哪个环境变量，实际：%v", err)
	}
}

// TestLoadBaseRejectsBadPortINFRA06 LoadBase 同样要拦坏端口（子命令也要监听口径一致）
func TestLoadBaseRejectsBadPortINFRA06(t *testing.T) {
	withEnv(t, map[string]string{"EBK_HTTP_PORT": "70000"})
	if _, err := LoadBase(); err == nil {
		t.Fatal("LoadBase 也应拒绝越界端口")
	}
}

// ---------- 差分守卫（吸收自 crosscheck-infra 探针 zz_ccv_config_test.go） ----------
//
// legacyLoad 是【重构前】Load 的 1:1 抄写（5c779db^ 版本），用于差分矩阵：
// 旧代码就校验的条件，新 Load 的错误文本与优先级必须逐字一致。
// 以后若再动 config 的校验顺序/文案，这条矩阵会立刻指出哪里偏离了历史契约。

func legacyEnvBool(key string, def bool) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def // 旧口径：无法识别 → 静默回落
	}
}

// legacyLoad 重构前的 Load 实现（端口无数值校验；envBool 静默回落；顺序 路径→JWT→保留期）
func legacyLoad() (*Config, error) {
	recycleTTL, err := envInt("EBK_RECYCLE_TTL_DAYS", DefaultRecycleTTLDays)
	if err != nil {
		return nil, err
	}
	inviteTTL, err := envInt("EBK_INVITE_TTL_DAYS", DefaultInviteTTLDays)
	if err != nil {
		return nil, err
	}
	auditDays, err := envInt("EBK_AUDIT_RETENTION_DAYS", DefaultAuditRetentionDays)
	if err != nil {
		return nil, err
	}
	c := &Config{
		HTTPPort:           envOr("EBK_HTTP_PORT", DefaultHTTPPort),
		DBPath:             envOr("EBK_DB_PATH", DefaultDBPath),
		DataDir:            envOr("EBK_DATA_DIR", DefaultDataDir),
		JWTSecret:          os.Getenv("EBK_JWT_SECRET"),
		EnableRegister:     legacyEnvBool("EBK_ENABLE_REGISTER", false),
		RecycleTTLDays:     recycleTTL,
		InviteTTLDays:      inviteTTL,
		AuditRetentionDays: auditDays,
	}
	if strings.TrimSpace(c.HTTPPort) == "" {
		return nil, fmt.Errorf("EBK_HTTP_PORT 不能为空")
	}
	if strings.TrimSpace(c.DBPath) == "" {
		return nil, fmt.Errorf("EBK_DB_PATH 不能为空")
	}
	if strings.TrimSpace(c.DataDir) == "" {
		return nil, fmt.Errorf("EBK_DATA_DIR 不能为空")
	}
	if err := validateJWTSecret(c.JWTSecret); err != nil {
		return nil, err
	}
	if c.RecycleTTLDays <= 0 {
		return nil, fmt.Errorf("EBK_RECYCLE_TTL_DAYS 必须为正整数，当前=%d", c.RecycleTTLDays)
	}
	if c.InviteTTLDays <= 0 {
		return nil, fmt.Errorf("EBK_INVITE_TTL_DAYS 必须为正整数，当前=%d", c.InviteTTLDays)
	}
	if c.AuditRetentionDays <= 0 {
		return nil, fmt.Errorf("EBK_AUDIT_RETENTION_DAYS 必须为正整数，当前=%d", c.AuditRetentionDays)
	}
	for _, v := range []struct {
		name string
		n    int
	}{
		{"EBK_RECYCLE_TTL_DAYS", c.RecycleTTLDays},
		{"EBK_INVITE_TTL_DAYS", c.InviteTTLDays},
		{"EBK_AUDIT_RETENTION_DAYS", c.AuditRetentionDays},
	} {
		if v.n > MaxRetentionDays {
			return nil, fmt.Errorf("%s 超过上限 %d 天，当前=%d", v.name, MaxRetentionDays, v.n)
		}
	}
	return c, nil
}

// clearEnv 清空相关 EBK_ 变量后按 kv 设置，测试结束【恢复原值】。
//
// 必须恢复：否则本矩阵设置的 "x" 之类会泄漏到同包后续测试
// （探针原实现没做恢复，单独跑没事、合进全量就互相污染）。
func clearEnv(t *testing.T, kv map[string]string) {
	t.Helper()
	tracked := []string{
		"EBK_HTTP_PORT", "EBK_DB_PATH", "EBK_DATA_DIR", "EBK_JWT_SECRET",
		"EBK_ENABLE_REGISTER", "EBK_RECYCLE_TTL_DAYS", "EBK_INVITE_TTL_DAYS",
		"EBK_AUDIT_RETENTION_DAYS", "EBK_WS_MAX_CONN_PER_USER", "EBK_WS_MAX_CONN_TOTAL",
	}
	type saved struct {
		k   string
		v   string
		had bool
	}
	var prev []saved
	for _, k := range tracked {
		old, had := os.LookupEnv(k)
		prev = append(prev, saved{k, old, had})
		if err := os.Unsetenv(k); err != nil {
			t.Fatalf("清理 %s 失败：%v", k, err)
		}
	}
	t.Cleanup(func() {
		for _, s := range prev {
			if s.had {
				_ = os.Setenv(s.k, s.v)
			} else {
				_ = os.Unsetenv(s.k)
			}
		}
	})
	for k, v := range kv {
		if err := os.Setenv(k, v); err != nil {
			t.Fatalf("设置 %s 失败：%v", k, err)
		}
	}
}

// TestLoadMatchesLegacyForPreexistingConditions 旧校验条件下的错误必须与重构前逐字一致
func TestLoadMatchesLegacyForPreexistingConditions(t *testing.T) {
	validJWT := strings.Repeat("s", 40)
	same := []struct {
		name string
		env  map[string]string
	}{
		{"全部默认_应成功", nil},
		{"端口为空+JWT为空", map[string]string{"EBK_HTTP_PORT": ""}},
		{"端口为空+DBPath为空", map[string]string{"EBK_HTTP_PORT": "", "EBK_DB_PATH": " "}},
		{"DBPath空+JWT占位", map[string]string{"EBK_DB_PATH": "", "EBK_JWT_SECRET": "secret"}},
		{"DataDir空+JWT过短", map[string]string{"EBK_DATA_DIR": "", "EBK_JWT_SECRET": "short"}},
		{"JWT空+保留期非法", map[string]string{"EBK_RECYCLE_TTL_DAYS": "abc"}},
		{"JWT占位+保留期零", map[string]string{"EBK_JWT_SECRET": "changeme", "EBK_RECYCLE_TTL_DAYS": "0"}},
		{"保留期负数+JWT合法", map[string]string{"EBK_JWT_SECRET": validJWT, "EBK_AUDIT_RETENTION_DAYS": "-1"}},
		{"保留期超上限", map[string]string{"EBK_JWT_SECRET": validJWT, "EBK_INVITE_TTL_DAYS": "36501"}},
		{"多重非法_端口空_保留期非法_保留期超限",
			map[string]string{"EBK_HTTP_PORT": "", "EBK_INVITE_TTL_DAYS": "xyz", "EBK_AUDIT_RETENTION_DAYS": "999999999"}},
	}
	for _, c := range same {
		t.Run("一致_"+c.name, func(t *testing.T) {
			clearEnv(t, c.env)
			_, errNew := Load()
			_, errOld := legacyLoad()
			newStr, oldStr := errString(errNew), errString(errOld)
			if newStr != oldStr {
				t.Errorf("错误与重构前不一致：\n  新 Load=%q\n  旧实现=%q", newStr, oldStr)
			}
		})
	}

	// 有意的行为变化（INFRA-05/06）：只要求"新代码确实拦住"，不比对文本
	changed := []struct {
		name string
		env  map[string]string
	}{
		{"端口数值非法_新代码现在拦截", map[string]string{"EBK_HTTP_PORT": "abc"}},
		{"端口超范围_0", map[string]string{"EBK_HTTP_PORT": "0", "EBK_JWT_SECRET": validJWT}},
		{"端口超范围_65536", map[string]string{"EBK_HTTP_PORT": "65536", "EBK_JWT_SECRET": validJWT}},
		{"envBool非法值_新代码报错_旧代码静默回落",
			map[string]string{"EBK_JWT_SECRET": validJWT, "EBK_ENABLE_REGISTER": "enable"}},
		{"WS上限非法_新变量新校验", map[string]string{"EBK_JWT_SECRET": validJWT, "EBK_WS_MAX_CONN_PER_USER": "x"}},
	}
	for _, c := range changed {
		t.Run("变化_"+c.name, func(t *testing.T) {
			clearEnv(t, c.env)
			if _, err := Load(); err == nil {
				t.Fatalf("%s：新 Load 应拦截，实际放行", c.name)
			}
		})
	}
}

func errString(err error) string {
	if err == nil {
		return "<nil>"
	}
	return err.Error()
}
