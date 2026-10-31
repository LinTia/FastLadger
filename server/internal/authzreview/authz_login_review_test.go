// Package authzreview —— 登录限流 / 登录失败审计 的复审证据用例（A6，reviewer-authz）。
//
// 本文件由 team-lead 从 authz_review_test.go 拆出，用于并行整改时的文件所有权隔离：
// 登录域（A6/A7 加固）归一位工程师，其余发现（A1~A5/A8）归另一位。
// 判定约定与共享 helper（engine/newEnv/countAudit 等）见 authz_review_test.go。
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

// ---------- 发现 A6：登录入口既无限流、失败也不留审计 ----------

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
	body := `{"username":"` + user + `","password":"` + pass + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.9:5555"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

// TestAuthzLoginMustBeRateLimited 登录入口必须有失败尝试限流。
//
// 期望：连续错误口令超过配额后返回 429（或至少被节流）。
// 实际：全仓的 Limiter 只用在两处——邀请签发（member.go:329）与邀请兑换
//
//	（member.go:734）；login 路径（auth_handler.go:64 / auth.go:192）没有任何限流，
//	25 次错误口令全部原样返回 401，一次 429 都没有。
//
// 影响：自托管实例常常直接暴露在内网/反代后，登录是唯一入口；口令爆破与撞库
//
//	没有服务端闸门，只能靠 bcrypt 的计算成本被动拖慢。
func TestAuthzLoginMustBeRateLimited(t *testing.T) {
	e := engine(t)
	r, user := authzLoginRouter(t, e)

	limited := 0
	for i := 0; i < 25; i++ {
		if c := authzPostLogin(r, user, "wrong-password-"+strconv.Itoa(i)); c == http.StatusTooManyRequests {
			limited++
		}
	}
	if limited == 0 {
		t.Fatalf("连续 25 次错误口令登录，没有一次触发限流（429=0）：" +
			"登录入口完全没有失败尝试闸门（对比：邀请兑换 10 次/小时/IP）")
	}
}

// TestAuthzFailedLoginMustLeaveAudit 登录失败必须留痕。
//
// 期望：audit_logs 至少出现一行能说明"有人在对这个账号做失败尝试"的记录。
// 实际：AuthService.Login 只在【口令校验通过之后】才 AppendAudit（auth.go:219），
//
//	所有失败分支（账号不存在 / 口令错误 / 账号禁用）直接 return，审计表 0 行。
//
// 影响：与 A6 叠加后，"有人正在爆破某个账号"这件事在服务端**完全不可见**——
//
//	既没有速率信号，也没有审计信号；事后无法回答"这个账号是否被尝试过、
//	什么时候、从哪个 IP"。登录成功反而有审计（WBS 4.3），口径同样是反的：
//	正常行为有痕、异常行为无痕。
func TestAuthzFailedLoginMustLeaveAudit(t *testing.T) {
	e := engine(t)
	r, user := authzLoginRouter(t, e)

	for i := 0; i < 5; i++ {
		authzPostLogin(r, user, "wrong-password-"+strconv.Itoa(i))
	}
	// 账号不存在的失败也算
	for i := 0; i < 5; i++ {
		authzPostLogin(r, "authz_no_such_user", "whatever-password")
	}
	n := countAudit(t, e, "action = ?", model.AuditActionLogin)
	if n == 0 {
		t.Fatalf("10 次登录失败（5 次口令错 + 5 次账号不存在）后 audit_logs 中 login 事件为 0 行；" +
			"期望 ≥1 行（失败尝试可追溯：who/when/from-where）")
	}
}
