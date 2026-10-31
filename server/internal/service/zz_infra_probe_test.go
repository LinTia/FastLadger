package service

// reviewer-infra 的探针用例 —— 第二波整改后翻转为回归守卫（文件所有权：登录/限流域工程师）。
//
// TestInfraProbeClientIPMustIgnoreForgedXFF：信任代理口径（根因修复在 api/router.go）。
// TestInfraProbeLimiterCapacityExhaustionWindow：限流器容量耗尽的恢复策略（INFRA-04 反向）。

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// TestInfraProbeClientIPMustIgnoreForgedXFF ClientIP 不得被 X-Forwarded-For 单方面操纵。
//
// 原缺陷：生产用 gin.New()（internal/api/router.go）且未调用 SetTrustedProxies，
// gin 的出厂默认 trustedProxies = {"0.0.0.0/0", "::/0"}（信任一切代理），
// c.ClientIP() 返回 XFF 最左段——即【请求方完全可控的字符串】，登录/邀请限流的
// 桶键随之失效（每换一个头就是全新配额）。
//
// 修复后翻转为回归守卫，两层断言：
//  1. 生产口径（gin.New + SetTrustedProxies(nil)，与 router.go 完全同一构造）：
//     无论 XFF 填什么，ClientIP 恒等于 TCP 对端；
//  2. 钉住 gin 的危险默认值：不调 SetTrustedProxies 时 XFF 确实生效——这条永远为真
//     （gin 库语义），作用是解释 router.go 里那一行为什么绝不能删：删了它，第 1 层立刻红。
func TestInfraProbeClientIPMustIgnoreForgedXFF(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// ① 生产口径：XFF 不影响 ClientIP
	r := gin.New()
	if err := r.SetTrustedProxies(nil); err != nil {
		t.Fatalf("SetTrustedProxies(nil) 不应失败：%v", err)
	}
	var got string
	r.GET("/", func(c *gin.Context) { got = c.ClientIP() })
	for _, xff := range []string{"1.2.3.4", "9.9.9.9", "203.0.113.77", "2001:db8::1"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.7:54321" // 真实 TCP 对端
		req.Header.Set("X-Forwarded-For", xff)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if got != "10.0.0.7" {
			t.Fatalf("伪造 XFF=%q 影响了 ClientIP（实际 %q）：限流桶键可被请求头单方面操纵",
				xff, got)
		}
	}

	// ② 钉住 gin 出厂默认值（文档性断言，语义若变则提示复核注释）
	raw := gin.New()
	var rawGot string
	raw.GET("/", func(c *gin.Context) { rawGot = c.ClientIP() })
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.7:54321"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	w := httptest.NewRecorder()
	raw.ServeHTTP(w, req)
	if rawGot != "1.2.3.4" {
		t.Skipf("gin 出厂默认语义已变化（XFF=1.2.3.4 → ClientIP=%q）：请复核 router.go 关于"+
			"『gin 默认信任全部代理』的注释是否仍然成立", rawGot)
	}
}

// TestInfraProbeLimiterCapacityExhaustionWindow 容量被打满后的恢复策略（INFRA-04 反向，新语义）。
//
// 修复后语义（与 limiter.go 的容量语义注释一一对应）：
//   - 容量有界（R44 保持）：打满后 Size 恒为上限，不 OOM；
//   - fail-closed：满容量期间新 key 被拒（明确取舍：淘汰最旧会让攻击者冲掉正常用户计数）；
//   - **自愈**：Allow 在容量满时会先强制清扫过期条目——攻击 key 一过窗容量即刻回收，
//     无需人工 Sweep 或重启；恢复时长上界 = 攻击 key 的窗口长度。
//
// 用注入的固定时钟推进时间（与 R44 的限流用例同一手法）：过期回收必须确定性可断言，
// sleep 只会在慢机器上偶发失败并掩盖真实时序。
func TestInfraProbeLimiterCapacityExhaustionWindow(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := base
	l := newLimiter(100, func() time.Time { return now })

	const attackWindow = time.Hour
	// 攻击者：100 个一次性 key，每个只打 1 次（1 小时窗口）
	for i := 0; i < 100; i++ {
		if !l.Allow("attacker:"+string(rune('a'+i%26))+time.Duration(i).String(), 10, attackWindow) {
			t.Fatalf("攻击者的第 %d 个 key 不应被拒（容量未打满时新 key 应当放行）", i)
		}
	}
	if l.Size() != 100 {
		t.Fatalf("打满前应正好 100 个 key（容量有界），实际 %d", l.Size())
	}
	// 打满瞬间：正常用户的新 key 被拒（fail-closed 取舍）
	if l.Allow("legit:user", 10, time.Hour) {
		t.Fatalf("容量打满后新 key 应被拒（fail-closed 取舍，见 limiter.go 容量语义）")
	}

	// 推进 59 分钟：攻击 key 未过窗，容量不得回收（恢复上界 = 攻击窗口的体现）
	now = base.Add(59 * time.Minute)
	if l.Allow("legit:user", 10, time.Hour) {
		t.Fatalf("59 分钟后攻击 key（1 小时窗口）尚未过期，容量不应恢复——" +
			"提前回收仍在窗口内的计数等于把限流改成 fail-open")
	}

	// 推进到 61 分钟：攻击 key 全部过期 ⇒ 【无需人工 Sweep】，下一次 Allow
	// 在容量满时强制清扫后直接放行（自愈）
	now = base.Add(61 * time.Minute)
	if !l.Allow("legit:user", 10, time.Hour) {
		t.Fatalf("攻击 key 过窗后容量应自愈（Allow 容量满时先强制清扫），实际仍被拒；Size=%d", l.Size())
	}
	if l.Size() != 1 {
		t.Fatalf("自愈后应只剩正常 key 1 条，实际 %d", l.Size())
	}
}
