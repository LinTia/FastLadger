// Package authzreview —— 登录限流 / 登录失败审计 的复审证据用例（A6，reviewer-authz）。
//
// 本文件由 team-lead 从 authz_review_test.go 拆出，用于并行整改时的文件所有权隔离：
// 登录域（A6/A7 加固）归一位工程师，其余发现（A1~A5/A8）归另一位。
// 判定约定与共享 helper（engine/newEnv/countAudit 等）见 authz_review_test.go。
//
// 2026-xx 整改后翻转说明：A6/A7 已修复（登录失败限流 + login_failed 审计），
// 本文件的两条用例从"红灯证据"翻转为"回归守卫"，断言口径全部按新语义写死。
package authzreview

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"fastledger/internal/api"
	"fastledger/internal/config"
	"fastledger/internal/model"
	"fastledger/internal/service"
	"fastledger/internal/version"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"xorm.io/xorm"
)

// authzLoginRouter 构造带真实用户的路由（登录用例用）。
func authzLoginRouter(t *testing.T, e *xorm.Engine) (*gin.Engine, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	as := service.NewAuthService(e, "authz-review-secret-0123456789abcdef", time.Hour)
	// 直接落一个 bcrypt 用户，避免走 Register（公开注册开关与本用例无关）
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-horse-battery"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("生成口令哈希失败：%v", err)
	}
	if _, err := e.Exec("INSERT INTO users (username, password_hash, created_at) VALUES (?, ?, ?)",
		"authz_login_user", string(hash), time.Now().Unix()); err != nil {
		t.Fatalf("插入登录用户失败：%v", err)
	}
	r := api.NewRouter(api.Deps{
		Config:        &config.Config{EnableRegister: true},
		AuthService:   as,
		Books:         service.NewBookService(e, version.NewRepo()),
		SchemaVersion: 1,
	})
	return r, "authz_login_user"
}

func authzPostLogin(r *gin.Engine, user, pass string) int {
	return authzPostLoginFrom(r, user, pass, "203.0.113.9:5555", "")
}

// authzPostLoginFrom 可指定来源与伪造 XFF（用于验证限流桶键取自真实对端而非请求头）。
func authzPostLoginFrom(r *gin.Engine, user, pass, remoteAddr, xff string) int {
	body := `{"username":"` + user + `","password":"` + pass + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = remoteAddr
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

// TestAuthzLoginMustBeRateLimited 登录入口必须有失败尝试限流（A6，已修复→回归守卫）。
//
// 断言口径（默认配额 = service.DefaultLoginMaxAttempts 次 / 15 分钟，只对失败计数）：
//  1. 前 N 次（配额内）失败仍返回 401——配额内的真实用户不能被误伤；
//  2. 第 N+1 次起全部 429；
//  3. **换伪造 X-Forwarded-For 不能重置换取配额**：限流桶键取自 TCP 对端
//     （router.go 已 SetTrustedProxies(nil)），请求头拼不出新桶。
func TestAuthzLoginMustBeRateLimited(t *testing.T) {
	e := engine(t)
	r, user := authzLoginRouter(t, e)

	const attempts = 25
	if service.DefaultLoginMaxAttempts >= attempts {
		t.Skipf("默认配额 %d ≥ 用例尝试次数 %d，请调大用例次数", service.DefaultLoginMaxAttempts, attempts)
	}

	codes := make([]int, 0, attempts)
	for i := 0; i < attempts; i++ {
		codes = append(codes, authzPostLogin(r, user, "wrong-password-"+strconv.Itoa(i)))
	}
	limited, unauthorized := 0, 0
	for _, c := range codes {
		switch c {
		case http.StatusTooManyRequests:
			limited++
		case http.StatusUnauthorized:
			unauthorized++
		}
	}
	if unauthorized != service.DefaultLoginMaxAttempts {
		t.Fatalf("配额内（前 %d 次）的失败登录应仍返回 401，实际 401=%d（全部 25 次响应：%v）——"+
			"限流不得误伤配额内的真实用户", service.DefaultLoginMaxAttempts, unauthorized, codes)
	}
	if limited != attempts-service.DefaultLoginMaxAttempts {
		t.Fatalf("超出配额后应全部 429：期望 %d 次，实际 %d 次（响应：%v）",
			attempts-service.DefaultLoginMaxAttempts, limited, codes)
	}

	// 换伪造 XFF：同一 TCP 对端（203.0.113.9）配额已满，伪造头必须换不来新桶
	for i := 0; i < 15; i++ {
		xff := "198.51.100." + strconv.Itoa(i+1)
		if c := authzPostLoginFrom(r, user, "wrong-password-xff-"+strconv.Itoa(i),
			"203.0.113.9:5555", xff); c != http.StatusTooManyRequests {
			t.Fatalf("换伪造 X-Forwarded-For（%s）不应重置登录限流配额，实际返回 %d", xff, c)
		}
	}
}

// TestAuthzFailedLoginMustLeaveAudit 登录失败必须留痕（A7 加固，已修复→回归守卫）。
//
// 断言口径：
//  1. 10 次失败（5 次口令错 + 5 次账号不存在）= 10 行 AuditActionLoginFailed；
//  2. reason 码区分 bad_password / no_such_user（"有人在枚举账号"必须可见）；
//  3. 账号不存在时 entity_id = 0（没有可指认的用户）；
//  4. before_json 里只有 attempted 用户名与 reason，**绝无口令**；
//  5. 对照：成功登录仍写 AuditActionLogin（成功口径不得倒退），且从【另一个来源】
//     成功登录不受前一个来源已打满的失败配额影响。
func TestAuthzFailedLoginMustLeaveAudit(t *testing.T) {
	e := engine(t)
	r, user := authzLoginRouter(t, e)

	for i := 0; i < 5; i++ {
		if c := authzPostLogin(r, user, "wrong-password-"+strconv.Itoa(i)); c != http.StatusUnauthorized {
			t.Fatalf("配额内的失败登录应仍返回 401（第 %d 次），实际 %d", i+1, c)
		}
	}
	// 账号不存在的失败也算
	for i := 0; i < 5; i++ {
		authzPostLogin(r, "authz_no_such_user", "whatever-password")
	}

	const want = 10
	n := countAudit(t, e, "action = ?", model.AuditActionLoginFailed)
	if n != want {
		t.Fatalf("10 次登录失败（5 次口令错 + 5 次账号不存在）应写 %d 行 login_failed 审计，实际 %d 行", want, n)
	}
	if bad := countAudit(t, e, "action = ? AND before_json LIKE ?",
		model.AuditActionLoginFailed, `%"reason":"bad_password"%`); bad != 5 {
		t.Fatalf("口令错误的失败审计应恰 5 行（reason=bad_password），实际 %d", bad)
	}
	if nosu := countAudit(t, e, "action = ? AND before_json LIKE ?",
		model.AuditActionLoginFailed, `%"reason":"no_such_user"%`); nosu != 5 {
		t.Fatalf("账号不存在的失败审计应恰 5 行（reason=no_such_user），实际 %d", nosu)
	}
	// 账号不存在：没有可指认的用户，actor_uid / entity_id 都必须是 0
	if zero := countAudit(t, e, "action = ? AND entity_id = 0 AND actor_uid = 0",
		model.AuditActionLoginFailed); zero != 5 {
		t.Fatalf("账号不存在的失败审计应恰 5 行 entity_id=0 且 actor_uid=0，实际 %d", zero)
	}
	// 绝不记口令
	if leaked := countAudit(t, e, "action = ? AND before_json LIKE ?",
		model.AuditActionLoginFailed, "%wrong-password%"); leaked != 0 {
		t.Fatalf("登录失败审计泄露了口令（%d 行 before_json 含 attempted 口令）", leaked)
	}

	// 对照：成功登录仍写 AuditActionLogin（WBS 4.3 口径不得倒退）。
	// 用另一个来源：上一个 IP 的失败配额（10 次）已打满，这正是"失败才计数"的证明面。
	if c := authzPostLoginFrom(r, user, "correct-horse-battery", "198.51.100.55:1234", ""); c != http.StatusOK {
		t.Fatalf("另一个来源的成功登录应返回 200，实际 %d", c)
	}
	if m := countAudit(t, e, "action = ?", model.AuditActionLogin); m == 0 {
		t.Fatal("成功登录应写 AuditActionLogin 审计（成功口径不得倒退），实际 0 行")
	}
	// 成功不得额外写 login_failed（成功不占失败配额，也不该留下失败痕迹）
	after := countAudit(t, e, "action = ?", model.AuditActionLoginFailed)
	if after != want {
		t.Fatalf("成功登录不得再写 login_failed 审计：期望仍为 %d 行，实际 %d 行", want, after)
	}
}
