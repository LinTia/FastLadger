package config

// R-7：登录失败限流窗口的上限校验（fail-fast，启动期拒绝"合法但危险"的配置）。
//
// 窗口 = 被打满后的瘫痪时长：EBK_LOGIN_WINDOW_MINUTES 配 1000000 意味着
// "一次打满永久锁死该 IP"。必须在启动期拒绝，不能等运行期才暴露。

import (
	"os"
	"strings"
	"testing"
)

// setEnv 辅助：设置环境变量并在测试结束后还原
func setEnv(t *testing.T, kv map[string]string) {
	t.Helper()
	old := map[string]string{}
	for k, v := range kv {
		old[k] = os.Getenv(k)
		if v == "" {
			_ = os.Unsetenv(k)
		} else {
			if err := os.Setenv(k, v); err != nil {
				t.Fatalf("设置 %s 失败：%v", k, err)
			}
		}
	}
	t.Cleanup(func() {
		for k, v := range old {
			if v == "" {
				_ = os.Unsetenv(k)
			} else {
				_ = os.Setenv(k, v)
			}
		}
	})
}

const r7Secret = "r7-guard-secret-0123456789abcdef-0123456789"

// TestR7LoginWindowUpperBoundAccepted 窗口在上限内（含上限本身）必须放行。
func TestR7LoginWindowUpperBoundAccepted(t *testing.T) {
	for _, w := range []string{"1", "15", "1440"} {
		setEnv(t, map[string]string{
			"EBK_JWT_SECRET":           r7Secret,
			"EBK_LOGIN_WINDOW_MINUTES": w,
		})
		cfg, err := Load()
		if err != nil {
			t.Fatalf("EBK_LOGIN_WINDOW_MINUTES=%s 应被接受，实际失败：%v", w, err)
		}
		if cfg.LoginWindowMinutes == 0 {
			t.Fatalf("EBK_LOGIN_WINDOW_MINUTES=%s 未生效", w)
		}
	}
}

// TestR7LoginWindowUpperBoundRejected 超过上限（> 1440）必须启动失败。
func TestR7LoginWindowUpperBoundRejected(t *testing.T) {
	for _, w := range []string{"1441", "1000000"} {
		setEnv(t, map[string]string{
			"EBK_JWT_SECRET":           r7Secret,
			"EBK_LOGIN_WINDOW_MINUTES": w,
		})
		_, err := Load()
		if err == nil {
			t.Fatalf("EBK_LOGIN_WINDOW_MINUTES=%s 必须启动失败（一次打满将锁死该 IP 超过一天）", w)
		}
		if !strings.Contains(err.Error(), "EBK_LOGIN_WINDOW_MINUTES") {
			t.Fatalf("错误信息应指向 EBK_LOGIN_WINDOW_MINUTES，实际 %v", err)
		}
	}
}

// TestR7LoadBaseAlsoEnforcesWindow 子命令入口（LoadBase）同样要拦（db purge 等子命令
// 也会读取这份配置，带着危险窗口运行没有意义）。
func TestR7LoadBaseAlsoEnforcesWindow(t *testing.T) {
	setEnv(t, map[string]string{
		"EBK_LOGIN_WINDOW_MINUTES": "1000000",
	})
	if _, err := LoadBase(); err == nil {
		t.Fatal("LoadBase 也必须拒绝超过上限的登录限流窗口")
	}
}
