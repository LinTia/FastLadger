package api

// QA 独立补充测试（Edward）：HTTP 适配层的可离线验证部分。
//
// 本机无 gcc（CGO 不可用），无法跑真实 SQLite，因此这里只覆盖【不碰数据库】
// 的适配层契约：
//  1. mapSvcErr 的状态码映射（400/403/404/409/500）与"不外露内部错误"；
//  2. 路径/查询参数解析的拒绝行为（pathID、bookID、queryInt64、queryInt）；
//  3. handler 层 clientUuid 必填与请求体格式错误的 400 兜底（不进入 service）；
//  4. 阶段 2 只允许"创建 + 查询"，不得出现 PUT/DELETE 交易入口。

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fastledger/internal/config"
	"fastledger/internal/service"

	"github.com/gin-gonic/gin"
)

// qaCtx 构造一个带响应记录器的 gin 上下文（不注册路由，直接调用被适配函数）
func qaCtx(method, target string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, target, nil)
	return c, w
}

// TestQAMapSvcErrStatusMapping 《转账入口裁决》§2 的状态码契约
func TestQAMapSvcErrStatusMapping(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		// 400：缺键 / 非法命令 / 绑定校验
		{"ErrInvalidCreate", service.ErrInvalidCreate, http.StatusBadRequest},
		{"ErrInvalidArgument", service.ErrInvalidArgument, http.StatusBadRequest},
		{"ErrAccountInvalid", service.ErrAccountInvalid, http.StatusBadRequest},
		{"ErrCategoryInvalid", service.ErrCategoryInvalid, http.StatusBadRequest},
		{"ErrWeakPassword", service.ErrWeakPassword, http.StatusBadRequest},
		{"ErrInviteInvalid", service.ErrInviteInvalid, http.StatusBadRequest},

		// 403：成员资格 / 权限
		{"ErrCreateForbidden", service.ErrCreateForbidden, http.StatusForbidden},
		{"ErrNotMember", service.ErrNotMember, http.StatusForbidden},
		{"ErrForbidden", service.ErrForbidden, http.StatusForbidden},
		{"ErrCannotOperateOwner", service.ErrCannotOperateOwner, http.StatusForbidden},

		// 404：资源不存在
		{"ErrBookNotFound", service.ErrBookNotFound, http.StatusNotFound},
		{"ErrAccountNotFound", service.ErrAccountNotFound, http.StatusNotFound},
		{"ErrCategoryNotFound", service.ErrCategoryNotFound, http.StatusNotFound},
		{"ErrTransactionNotFound", service.ErrTransactionNotFound, http.StatusNotFound},

		// 409：幂等冲突 / 回执不完整 / 并发冲突 / 引用冲突
		{"ErrCreateConflict", service.ErrCreateConflict, http.StatusConflict},
		{"ErrIncompleteReceipt", service.ErrIncompleteReceipt, http.StatusConflict},
		{"ErrConcurrentConflict", service.ErrConcurrentConflict, http.StatusConflict},
		{"ErrAccountInUse", service.ErrAccountInUse, http.StatusConflict},
		{"ErrCategoryInUse", service.ErrCategoryInUse, http.StatusConflict},
		{"ErrCategoryHasChildren", service.ErrCategoryHasChildren, http.StatusConflict},
		{"ErrUserExists", service.ErrUserExists, http.StatusConflict},

		// 429：限流
		{"ErrRateLimited", service.ErrRateLimited, http.StatusTooManyRequests},

		// 5xx：未知错误不外露
		{"未知错误", errors.New("db: sqlite3: no such table: secret_table"), http.StatusInternalServerError},

		// 包装后仍按 errors.Is 判定（服务层大量使用 %w 包装）
		{"包装后的幂等冲突", fmt.Errorf("回写幂等回执失败：%w", service.ErrCreateConflict), http.StatusConflict},
		{"包装后的非法创建", fmt.Errorf("%w：金额必须为正整数分", service.ErrInvalidCreate), http.StatusBadRequest},
		{"包装后的账户不存在", fmt.Errorf("%w：账户 9 不存在", service.ErrAccountNotFound), http.StatusNotFound},
	}

	for _, c := range cases {
		ctx, w := qaCtx(http.MethodPost, "/api/v1/books/1/transactions")
		mapSvcErr(ctx, c.err)

		if w.Code != c.want {
			t.Errorf("%s：HTTP 状态应为 %d，实际 %d（body=%s）", c.name, c.want, w.Code, w.Body.String())
			continue
		}
		var body struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Errorf("%s：响应体不是合法 JSON：%s", c.name, w.Body.String())
			continue
		}
		if body.Code != c.want {
			t.Errorf("%s：响应 code 应与 HTTP 状态一致（%d），实际 %d", c.name, c.want, body.Code)
		}
		// 错误响应不得回显原始错误内容（防泄露实现细节 / SQL）
		if strings.Contains(body.Msg, "secret_table") || strings.Contains(body.Msg, "sqlite3") {
			t.Errorf("%s：错误响应泄露了内部细节：%s", c.name, body.Msg)
		}
	}
}

// TestQAMapSvcErrNilWritesNothing 成功路径不写错误响应
func TestQAMapSvcErrNilWritesNothing(t *testing.T) {
	ctx, w := qaCtx(http.MethodPost, "/api/v1/books/1/transactions")
	mapSvcErr(ctx, nil)
	if w.Body.Len() != 0 {
		t.Fatalf("err=nil 不应写响应，实际写入了：%s", w.Body.String())
	}
}

// TestQAPathIDRejectsInvalid 路径 ID 必须是正整数
func TestQAPathIDRejectsInvalid(t *testing.T) {
	for _, raw := range []string{"", "abc", "0", "-1", "1.5", "9223372036854775808"} {
		ctx, w := qaCtx(http.MethodGet, "/api/v1/books/1/accounts/x")
		ctx.Params = gin.Params{{Key: "aid", Value: raw}}
		if id, good := pathID(ctx, "aid", "账户 ID 非法"); good {
			t.Errorf("路径 ID %q 应被拒绝，实际返回 id=%d", raw, id)
		}
		if w.Code != http.StatusBadRequest {
			t.Errorf("路径 ID %q 应返回 400，实际 %d", raw, w.Code)
		}
	}

	ctx, w := qaCtx(http.MethodGet, "/api/v1/books/1/accounts/12")
	ctx.Params = gin.Params{{Key: "aid", Value: "12"}}
	id, good := pathID(ctx, "aid", "账户 ID 非法")
	if !good || id != 12 {
		t.Fatalf("合法路径 ID 应解析为 12，实际 id=%d good=%v（状态 %d）", id, good, w.Code)
	}
}

// TestQABookIDRejectsInvalid 账本 ID 必须是正整数（越权与脏参数的第一道闸）
func TestQABookIDRejectsInvalid(t *testing.T) {
	for _, raw := range []string{"", "abc", "0", "-3"} {
		ctx, w := qaCtx(http.MethodGet, "/api/v1/books/x/accounts")
		ctx.Params = gin.Params{{Key: "id", Value: raw}}
		if id, good := bookID(ctx); good {
			t.Errorf("账本 ID %q 应被拒绝，实际返回 id=%d", raw, id)
		}
		if w.Code != http.StatusBadRequest {
			t.Errorf("账本 ID %q 应返回 400，实际 %d", raw, w.Code)
		}
	}

	ctx, _ := qaCtx(http.MethodGet, "/api/v1/books/7/accounts")
	ctx.Params = gin.Params{{Key: "id", Value: "7"}}
	if id, good := bookID(ctx); !good || id != 7 {
		t.Fatalf("合法账本 ID 应解析为 7，实际 id=%d good=%v", id, good)
	}
}

// TestQAQueryIntParsers 可选整数查询参数：缺省 0、非法 400
func TestQAQueryIntParsers(t *testing.T) {
	ok64 := []struct {
		target string
		want   int64
	}{
		{"/x", 0},
		{"/x?maxTime=", 0},
		{"/x?maxTime=1793260800", 1793260800},
		{"/x?maxTime=-5", -5}, // 当前实现不设下界；服务层用 >0 判定，等价于忽略
	}
	for _, c := range ok64 {
		ctx, w := qaCtx(http.MethodGet, c.target)
		got, good := queryInt64(ctx, "maxTime")
		if !good || got != c.want {
			t.Errorf("%s：期望 (%d,true)，实际 (%d,%v)，状态 %d", c.target, c.want, got, good, w.Code)
		}
	}

	for _, target := range []string{"/x?maxTime=abc", "/x?maxTime=1.5", "/x?maxTime=1e3"} {
		ctx, w := qaCtx(http.MethodGet, target)
		if _, good := queryInt64(ctx, "maxTime"); good {
			t.Errorf("%s：非法整数应被拒绝", target)
		}
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s：非法整数应返回 400，实际 %d", target, w.Code)
		}
	}

	// queryInt 复用同一套校验
	ctx, _ := qaCtx(http.MethodGet, "/x?page=21")
	if v, good := queryInt(ctx, "page"); !good || v != 21 {
		t.Errorf("page=21 应解析为 21，实际 (%d,%v)", v, good)
	}
	ctx, w := qaCtx(http.MethodGet, "/x?page=x")
	if _, good := queryInt(ctx, "page"); good || w.Code != http.StatusBadRequest {
		t.Errorf("page=x 应返回 400，实际 good=%v 状态=%d", good, w.Code)
	}
}

// TestQACreateTransactionRejectsBadBody handler 层必填与格式兜底（不进入 service）
//
// 这三条路径都在 bindJSON 阶段返回，因此可以在 txs=nil 时安全调用，
// 证明"clientUuid 缺失不会走到 service"这一层，与 service 层校验互为双保险。
func TestQACreateTransactionRejectsBadBody(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"缺 clientUuid", `{"txType":2,"accountId":1,"amount":100,"categoryId":11,"txTime":1793260800}`},
		{"clientUuid 为空串", `{"clientUuid":"","txType":2,"accountId":1,"amount":100,"categoryId":11,"txTime":1793260800}`},
		{"非 JSON", `txType=2&amount=100`},
		{"空 body", ``},
		{"类型不匹配", `{"clientUuid":123456,"txType":2,"amount":100}`},
	}
	for _, c := range cases {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/books/1/transactions",
			strings.NewReader(c.body))
		ctx.Request.Header.Set("Content-Type", "application/json")
		ctx.Params = gin.Params{{Key: "id", Value: "1"}}
		ctx.Set(uidKey, int64(7))

		h := &ledgerHandler{} // txs 为 nil：任何进入 service 的路径都会 panic，能被本用例发现
		h.createTransaction(ctx)

		if w.Code != http.StatusBadRequest {
			t.Errorf("%s：应在 handler 层返回 400，实际 %d（body=%s）", c.name, w.Code, w.Body.String())
		}
	}
}

// TestQACreateTransactionRejectsBadBookID 非法账本 ID 在 handler 层即拒绝
func TestQACreateTransactionRejectsBadBookID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/books/abc/transactions",
		strings.NewReader(`{"clientUuid":"k","txType":2,"accountId":1,"amount":100,"categoryId":11,"txTime":1}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = gin.Params{{Key: "id", Value: "abc"}}
	ctx.Set(uidKey, int64(7))

	h := &ledgerHandler{}
	h.createTransaction(ctx)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("非法账本 ID 应返回 400，实际 %d", w.Code)
	}
}

// TestQATransactionRoutesCreateAndQueryOnly 阶段 2 只开放创建与查询
//
// 《转账入口裁决》§5 禁止第二条创建路径；交易的 PUT/DELETE 属阶段 4
// （删除/恢复/审计），本阶段若提前注册却未实现，会变成"能删但余额不回滚"的隐患。
func TestQATransactionRoutesCreateAndQueryOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(Deps{Config: &config.Config{}, SchemaVersion: 1})

	count := map[string]int{}
	for _, ri := range r.Routes() {
		count[ri.Method+" "+ri.Path]++
	}

	for _, allowed := range []string{
		"POST /api/v1/books/:id/transactions",
		"GET /api/v1/books/:id/transactions",
		"GET /api/v1/books/:id/transactions/:txId",
	} {
		if count[allowed] != 1 {
			t.Errorf("路由 %s 应注册且仅注册一次，实际 %d", allowed, count[allowed])
		}
	}
	for _, forbidden := range []string{
		"PUT /api/v1/books/:id/transactions/:txId",
		"DELETE /api/v1/books/:id/transactions/:txId",
		"POST /api/v1/books/:id/transactions/transfer",
		"POST /api/v1/books/:id/transactions/batch",
		"POST /api/v1/books/:id/transfer",
		"POST /api/v1/sync/push",
	} {
		if count[forbidden] != 0 {
			t.Errorf("本阶段不应注册的路由已出现：%s", forbidden)
		}
	}
}
