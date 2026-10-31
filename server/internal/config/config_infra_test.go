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
