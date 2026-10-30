package api

// 路由注册冒烟测试（不依赖数据库）：确保阶段 2 的记账接口已接线，
// 且【没有】第二个转账创建入口（《转账入口裁决》§5 禁止双入口）。
import (
	"testing"

	"fastledger/internal/config"

	"github.com/gin-gonic/gin"
)

func TestRouterRegistersLedgerRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(Deps{Config: &config.Config{}, SchemaVersion: 1})

	got := make(map[string]int, len(r.Routes()))
	for _, ri := range r.Routes() {
		got[ri.Method+" "+ri.Path]++
	}

	want := []string{
		"POST /api/v1/books/:id/transactions",
		"GET /api/v1/books/:id/transactions",
		"GET /api/v1/books/:id/transactions/:txId",
		"GET /api/v1/books/:id/accounts",
		"POST /api/v1/books/:id/accounts",
		"PUT /api/v1/books/:id/accounts/:aid",
		"DELETE /api/v1/books/:id/accounts/:aid",
		"GET /api/v1/books/:id/categories",
		"POST /api/v1/books/:id/categories",
		"PUT /api/v1/books/:id/categories/:cid",
		"DELETE /api/v1/books/:id/categories/:cid",
	}
	for _, route := range want {
		if got[route] != 1 {
			t.Errorf("路由 %s 应注册且仅注册一次，实际 %d 次", route, got[route])
		}
	}

	// 唯一创建入口：不允许出现独立转账入口
	for _, forbidden := range []string{
		"POST /api/v1/books/:id/transactions/transfer",
		"POST /api/v1/books/:id/transfer",
		"POST /api/v1/books/:id/transactions/batch",
	} {
		if got[forbidden] != 0 {
			t.Errorf("禁止的第二条创建入口已注册：%s", forbidden)
		}
	}
}
