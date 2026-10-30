package api

// GPT-R40（邀请码撤销）的 **HTTP 适配层**回归测试。
//
// 本机无 gcc（CGO 不可用），这里只覆盖【不碰数据库】的契约：路由注册位置、
// 路径参数校验、以及邀请失败错误的状态码映射与"不泄露可枚举信息"。
// service 不能反向 import api，故 HTTP 层断言放这侧；数据行为断言在
// internal/service/remediation_invite_integration_test.go。

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fastledger/internal/config"
	"fastledger/internal/service"

	"github.com/gin-gonic/gin"
)

// inviteRevokeRoute 撤销邀请的路由（见 router.go 注释：动作段而非 DELETE）
const inviteRevokeRoute = "POST /api/v1/books/:id/members/invite/:inviteId/revoke"

// TestR40RevokeInviteRouteRegistered 撤销入口必须存在、只注册一次、且在 JWT 保护组内。
//
// NewRouter 在本机可以构造（不连库），因此这条路由契约可以被真实构造出来验证。
func TestR40RevokeInviteRouteRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(Deps{Config: &config.Config{}, SchemaVersion: 1})

	count := map[string]int{}
	for _, ri := range r.Routes() {
		count[ri.Method+" "+ri.Path]++
	}
	if count[inviteRevokeRoute] != 1 {
		t.Fatalf("撤销路由 %s 应注册且仅注册一次，实际 %d", inviteRevokeRoute, count[inviteRevokeRoute])
	}
	// 撤销是写操作：挂在 JWTAuth 之下的 auth 组里，不能像 join 那样对匿名开放
	src := readInviteAPIFile(t, "router.go")
	authGroup := strings.Index(src, "auth := v1.Group")
	revoke := strings.Index(src, "bh.revokeInvite")
	if authGroup < 0 || revoke < 0 {
		t.Fatal("router.go 缺少 auth 组或 revokeInvite 接线")
	}
	if revoke < authGroup {
		t.Fatalf("撤销路由注册在 auth 组之前（L%d < L%d）：匿名即可废止邀请", revoke, authGroup)
	}
}

// TestR40RevokeInviteRejectsInvalidID 非法 inviteId 必须在 handler 层兜住，不进 service。
func TestR40RevokeInviteRejectsInvalidID(t *testing.T) {
	for _, raw := range []string{"", "abc", "0", "-1", "1.5", "9223372036854775808"} {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost,
			"/api/v1/books/1/members/invite/"+raw+"/revoke", nil)
		c.Params = gin.Params{{Key: "id", Value: "1"}, {Key: "inviteId", Value: raw}}

		// members / books 均为 nil：一旦放行非法 ID 进入 service 即 panic，能被本用例发现
		h := &bookHandler{}
		h.revokeInvite(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("inviteId=%q 应返回 400，实际 %d（body=%s）", raw, w.Code, w.Body.String())
		}
	}
}

// TestR40RevokeInviteRejectsInvalidBookID 账本 ID 非法时同样在 handler 层拒绝。
func TestR40RevokeInviteRejectsInvalidBookID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/books/x/members/invite/9/revoke", nil)
	c.Params = gin.Params{{Key: "id", Value: "x"}, {Key: "inviteId", Value: "9"}}

	h := &bookHandler{}
	h.revokeInvite(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("非法账本 ID 应返回 400，实际 %d", w.Code)
	}
}

// TestMapSvcErrInviteFailuresDoNotLeak 邀请失败必须映射为既有状态码，
// 且响应文案不得回显"邮箱已被占用""已绑定给某人""目标属于另一个账本"这类可枚举信息。
func TestMapSvcErrInviteFailuresDoNotLeak(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		// GPT-R41：邮箱占用刻意并入"信息不匹配"，避免邮箱枚举
		{"邮箱被占用", service.ErrInviteEmailUnavailable, http.StatusBadRequest},
		{"绑定信息不匹配", service.ErrInviteBoundMismatch, http.StatusBadRequest},
		{"邀请不存在", service.ErrInviteNotFound, http.StatusBadRequest},
		{"邀请已使用", service.ErrInviteAlreadyUsed, http.StatusBadRequest},
		{"邀请已撤销", service.ErrInviteAlreadyRevoked, http.StatusBadRequest},
		{"邀请过期", service.ErrInviteExpired, http.StatusBadRequest},
		{"绑定维度冲突", service.ErrInviteBindingConflict, http.StatusBadRequest},
	}
	for _, c := range cases {
		ctx, w := qaCtx(http.MethodPost, "/api/v1/books/1/members/join")
		mapSvcErr(ctx, c.err)
		if w.Code != c.want {
			t.Errorf("%s：应为 %d，实际 %d", c.name, c.want, w.Code)
			continue
		}
		body := w.Body.String()
		for _, leak := range []string{"占用", "已绑定给", "sqlite", "invites", "books."} {
			if strings.Contains(body, leak) {
				t.Errorf("%s：响应泄露内部信息 %q：%s", c.name, leak, body)
			}
		}
	}
}

// TestInviteErrorsWrapInviteInvalid 新增的邀请失败错误必须用 %w 继承既有错误树，
// 而不是另开一套：否则既有 switch 分支会漏掉它们，退化成 500。
func TestInviteErrorsWrapInviteInvalid(t *testing.T) {
	for _, c := range []struct {
		err  error
		base error
	}{
		{service.ErrInviteNotFound, service.ErrInviteInvalid},
		{service.ErrInviteAlreadyUsed, service.ErrInviteInvalid},
		{service.ErrInviteAlreadyRevoked, service.ErrInviteInvalid},
		{service.ErrInviteExpired, service.ErrInviteInvalid},
		{service.ErrInviteBookUnavailable, service.ErrInviteInvalid},
		{service.ErrInviteEmailUnavailable, service.ErrInviteBoundMismatch},
		{service.ErrInviteBindingConflict, service.ErrInvalidArgument},
	} {
		if !errors.Is(c.err, c.base) {
			t.Errorf("%v 应继承 %v（用 %%w 包装），实际 errors.Is=false", c.err, c.base)
		}
	}
}
