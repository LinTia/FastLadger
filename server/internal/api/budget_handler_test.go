package api

// 预算 handler 级测试（真实 SQLite + 真实登录；需 CGO）。
//
// 覆盖：路由鉴权 401、CRUD 信封契约、列表含进度字段、重复预算 409、
// 非法 payload 400（member 写权限拒绝在 service 层测试覆盖）。
import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"fastledger/internal/config"
	"fastledger/internal/database"
	"fastledger/internal/migrate"
	"fastledger/internal/model"
	"fastledger/internal/service"
	"fastledger/internal/version"

	"github.com/gin-gonic/gin"
	"xorm.io/xorm"
)

type budgetAPIEnv struct {
	t      *testing.T
	r      *gin.Engine
	engine *xorm.Engine
	token  string
	bookID int64
	catID  int64
	do     func(method, path, body, token string) *httptest.ResponseRecorder
}

func newBudgetAPIEnv(t *testing.T) *budgetAPIEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine, err := database.Init(t.TempDir() + "/budget_api.db")
	if err != nil {
		t.Fatalf("database.Init 失败：%v", err)
	}
	t.Cleanup(func() { _ = engine.Close() })
	if _, err := migrate.Apply(engine); err != nil {
		t.Fatalf("migrate.Apply 失败：%v", err)
	}
	as := service.NewAuthService(engine, "budget-api-test-secret-0123456789abcdef", time.Hour)
	versions := version.NewRepo()
	cfg := &config.Config{EnableRegister: true}
	r := NewRouter(Deps{
		Config:        cfg,
		AuthService:   as,
		Books:         service.NewBookService(engine, versions),
		Accounts:      service.NewAccountService(engine, versions),
		Categories:    service.NewCategoryService(engine, versions),
		Txs:           service.NewTxService(engine, versions),
		Budgets:       service.NewBudgetService(engine),
		OCR:           service.NewOCRService(cfg),
		SchemaVersion: 1,
	})
	e := &budgetAPIEnv{t: t, r: r, engine: engine}

	do := func(method, path, body, token string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	e.do = do

	if w := do(http.MethodPost, "/api/v1/auth/register", `{"username":"budgeter","password":"password123"}`, ""); w.Code != http.StatusOK {
		t.Fatalf("注册失败：%d %s", w.Code, w.Body.String())
	}
	w := do(http.MethodPost, "/api/v1/auth/login", `{"username":"budgeter","password":"password123"}`, "")
	var resp struct {
		Data struct {
			AccessToken string `json:"accessToken"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || resp.Data.AccessToken == "" {
		t.Fatalf("登录失败：%s", w.Body.String())
	}
	e.token = resp.Data.AccessToken

	w = do(http.MethodPost, "/api/v1/books", `{"name":"预算账本","bookType":1}`, e.token)
	var bresp struct {
		Data struct {
			Id int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &bresp); err != nil || bresp.Data.Id == 0 {
		t.Fatalf("建账本失败：%s", w.Body.String())
	}
	e.bookID = bresp.Data.Id

	cat, err := service.NewCategoryService(engine, versions).Create(1, e.bookID,
		service.CategoryCreateInput{Name: "餐饮", CatType: model.CategoryTypeExpense})
	if err != nil {
		t.Fatalf("建分类失败：%v", err)
	}
	e.catID = cat.Id
	return e
}

func TestBudgetRoutesBehindAuth(t *testing.T) {
	r := NewRouter(Deps{Config: &config.Config{}, Budgets: service.NewBudgetService(nil), SchemaVersion: 1})
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/books/1/budgets"},
		{http.MethodPost, "/api/v1/books/1/budgets"},
		{http.MethodPut, "/api/v1/books/1/budgets/1"},
		{http.MethodDelete, "/api/v1/books/1/budgets/1"},
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s 无 token 应 401，实际 %d", tc.method, tc.path, w.Code)
		}
	}
}

func TestBudgetCRUDFlow(t *testing.T) {
	e := newBudgetAPIEnv(t)
	base := "/api/v1/books/" + strconv.FormatInt(e.bookID, 10) + "/budgets"

	// 建分类预算
	w := e.do(http.MethodPost, base,
		`{"name":"餐饮预算","budgetType":1,"categoryId":`+strconv.FormatInt(e.catID, 10)+`,"amount":150000}`, e.token)
	if w.Code != http.StatusOK {
		t.Fatalf("建预算失败：%d %s", w.Code, w.Body.String())
	}
	var cresp struct {
		Data struct {
			Id int64 `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &cresp)
	if cresp.Data.Id == 0 {
		t.Fatal("响应缺预算 id")
	}

	// 重复同分类 → 409
	w = e.do(http.MethodPost, base,
		`{"budgetType":1,"categoryId":`+strconv.FormatInt(e.catID, 10)+`,"amount":100000}`, e.token)
	if w.Code != http.StatusConflict {
		t.Fatalf("重复预算应 409，实际 %d %s", w.Code, w.Body.String())
	}

	// 更新金额
	w = e.do(http.MethodPut, base+"/"+strconv.FormatInt(cresp.Data.Id, 10),
		`{"amount":200000,"name":"餐饮预算"}`, e.token)
	if w.Code != http.StatusOK {
		t.Fatalf("更新预算失败：%d %s", w.Code, w.Body.String())
	}

	// 列表：含进度字段
	w = e.do(http.MethodGet, base, "", e.token)
	if w.Code != http.StatusOK {
		t.Fatalf("列表失败：%d %s", w.Code, w.Body.String())
	}
	var lresp struct {
		Data struct {
			Items []struct {
				Amount   int64  `json:"amount"`
				Spent    int64  `json:"spent"`
				Percent  int    `json:"percent"`
				Status   string `json:"status"`
				Category string `json:"categoryName"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &lresp); err != nil {
		t.Fatalf("列表解析失败：%s", w.Body.String())
	}
	if len(lresp.Data.Items) != 1 {
		t.Fatalf("期望 1 条预算，得到 %d", len(lresp.Data.Items))
	}
	it := lresp.Data.Items[0]
	if it.Amount != 200000 || it.Status != "normal" {
		t.Errorf("列表数据错误：amount=%d status=%s", it.Amount, it.Status)
	}
	if it.Category != "餐饮" {
		t.Errorf("分类名映射错误：%q", it.Category)
	}

	// 非法 payload：周期=季 → 400
	w = e.do(http.MethodPost, base, `{"budgetType":2,"amount":100000,"periodType":2}`, e.token)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("period=2 应 400，实际 %d", w.Code)
	}

	// 删除 → 列表为空
	w = e.do(http.MethodDelete, base+"/"+strconv.FormatInt(cresp.Data.Id, 10), "", e.token)
	if w.Code != http.StatusOK {
		t.Fatalf("删除失败：%d", w.Code)
	}
	w = e.do(http.MethodGet, base, "", e.token)
	if !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Errorf("删除后列表应为空：%s", w.Body.String())
	}
	// 删除不存在 → 404
	w = e.do(http.MethodDelete, base+"/999999", "", e.token)
	if w.Code != http.StatusNotFound {
		t.Fatalf("删除不存在预算应 404，实际 %d", w.Code)
	}
}
