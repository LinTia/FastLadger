package config

// GPT-R34 / GPT-R27：环境变量解析不得静默回退，保留期上限须在启动期拦截。
// 纯逻辑测试，不需要数据库。

import (
	"os"
	"strings"
	"testing"
)

// withEnv 临时设置环境变量，测试结束恢复
func withEnv(t *testing.T, kv map[string]string) {
	t.Helper()
	for k, v := range kv {
		old, had := os.LookupEnv(k)
		if err := os.Setenv(k, v); err != nil {
			t.Fatalf("设置 %s 失败：%v", k, err)
		}
		t.Cleanup(func() {
			if had {
				_ = os.Setenv(k, old)
			} else {
				_ = os.Unsetenv(k)
			}
		})
	}
}

func TestEnvIntUsesDefaultWhenUnset(t *testing.T) {
	withEnv(t, map[string]string{"EBK_TEST_TTL": ""})
	got, err := envInt("EBK_TEST_TTL", 30)
	if err != nil {
		t.Fatalf("未设置时应回退默认值且不报错：%v", err)
	}
	if got != 30 {
		t.Fatalf("得到 %d，期望 30", got)
	}
}

// TestEnvIntRejectsInsteadOfSilentFallback 核心：非法值必须报错，
// 而不是静默回退到一个"会触发数据清理"的默认值。
func TestEnvIntRejectsInsteadOfSilentFallback(t *testing.T) {
	for _, bad := range []string{"abc", "30days", "1.5", "-5", "0", "+ 3", "1e3"} {
		withEnv(t, map[string]string{"EBK_TEST_TTL": bad})
		if got, err := envInt("EBK_TEST_TTL", 30); err == nil {
			t.Fatalf("非法值 %q 应报错，实际静默回退为 %d", bad, got)
		}
	}
}

// TestEnvIntTreatsBlankAsUnset 纯空白视为【未设置】→ 回退默认值。
// 与 envOr 的口径保持一致：空白不是"配错了"，而是"确实没配"。
// 只有"配了但配错"（如 abc / -5 / 超大值）才报错，避免把空环境误判为故障。
func TestEnvIntTreatsBlankAsUnset(t *testing.T) {
	for _, blank := range []string{"", "   ", "\t"} {
		withEnv(t, map[string]string{"EBK_TEST_TTL": blank})
		got, err := envInt("EBK_TEST_TTL", 30)
		if err != nil {
			t.Fatalf("空白值 %q 应视为未设置且不报错，实际：%v", blank, err)
		}
		if got != 30 {
			t.Fatalf("空白值 %q 应回退默认 30，实际 %d", blank, got)
		}
	}
}

func TestEnvIntAcceptsValidValues(t *testing.T) {
	for _, c := range []struct {
		in   string
		want int
	}{{"1", 1}, {"30", 30}, {"365", 365}, {"36500", 36500}} {
		withEnv(t, map[string]string{"EBK_TEST_TTL": c.in})
		got, err := envInt("EBK_TEST_TTL", 30)
		if err != nil {
			t.Fatalf("合法值 %q 不应报错：%v", c.in, err)
		}
		if got != c.want {
			t.Fatalf("%q 得到 %d，期望 %d", c.in, got, c.want)
		}
	}
}

// TestLoadRejectsBadRetentionEnv 保留期环境变量非法 → 启动失败（而不是带病运行）。
func TestLoadRejectsBadRetentionEnv(t *testing.T) {
	secret := strings.Repeat("s", 40) // 通过 JWT 校验
	for _, kv := range []map[string]string{
		{"EBK_RECYCLE_TTL_DAYS": "abc"},
		{"EBK_RECYCLE_TTL_DAYS": "0"},
		{"EBK_RECYCLE_TTL_DAYS": "-30"},
		{"EBK_INVITE_TTL_DAYS": "xyz"},
		{"EBK_AUDIT_RETENTION_DAYS": "not-a-number"},
	} {
		withEnv(t, map[string]string{"EBK_JWT_SECRET": secret})
		withEnv(t, kv)
		if _, err := Load(); err == nil {
			t.Fatalf("%v 应导致启动失败，实际通过", kv)
		}
	}
}

func TestLoadRejectsRetentionAboveCap(t *testing.T) {
	secret := strings.Repeat("s", 40)
	for _, kv := range []map[string]string{
		{"EBK_RECYCLE_TTL_DAYS": "36501"},
		{"EBK_AUDIT_RETENTION_DAYS": "999999999"},
	} {
		withEnv(t, map[string]string{"EBK_JWT_SECRET": secret})
		withEnv(t, kv)
		_, err := Load()
		if err == nil {
			t.Fatalf("%v 超过上限应报错，实际通过", kv)
		}
		if !strings.Contains(err.Error(), "超过上限") {
			t.Fatalf("错误信息应说明超过上限，实际：%v", err)
		}
	}
}

func TestLoadDefaultsAreSane(t *testing.T) {
	withEnv(t, map[string]string{"EBK_JWT_SECRET": strings.Repeat("s", 40)})
	for _, k := range []string{"EBK_RECYCLE_TTL_DAYS", "EBK_INVITE_TTL_DAYS", "EBK_AUDIT_RETENTION_DAYS"} {
		withEnv(t, map[string]string{k: ""})
	}
	c, err := Load()
	if err != nil {
		t.Fatalf("默认值配置不应报错：%v", err)
	}
	if c.RecycleTTLDays != DefaultRecycleTTLDays || c.InviteTTLDays != DefaultInviteTTLDays ||
		c.AuditRetentionDays != DefaultAuditRetentionDays {
		t.Fatalf("默认值不符：recycle=%d invite=%d audit=%d",
			c.RecycleTTLDays, c.InviteTTLDays, c.AuditRetentionDays)
	}
}
