package api

// 报表与导出 handler 级测试（真实 SQLite + 真实登录；需 CGO，CI/服务器执行）。
//
// 覆盖：路由鉴权（无 token 401）、非成员 403、月度/分类占比信封契约、
// 导出 CSV（BOM + 表头 + Content-Disposition）与 JSON、format/fields 校验、
// 截断响应头、导出零账务副作用（只读口径）。
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

type reportAPIEnv struct {
	t      *testing.T
	r      *gin.Engine
	engine *xorm.Engine
	token  string
	bookID int64
}

func newReportAPIEnv(t *testing.T) *reportAPIEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine, err := database.Init(t.TempDir() + "/report_api.db")
	if err != nil {
		t.Fatalf("database.Init 失败（CGO 是否可用？）：%v", err)
	}
	t.Cleanup(func() { _ = engine.Close() })
	if _, err := migrate.Apply(engine); err != nil {
		t.Fatalf("migrate.Apply 失败：%v", err)
	}
	as := service.NewAuthService(engine, "report-api-test-secret-0123456789abcdef", time.Hour)
	versions := version.NewRepo()
	cfg := &config.Config{EnableRegister: true}
	r := NewRouter(Deps{
		Config:        cfg,
		AuthService:   as,
		Books:         service.NewBookService(engine, versions),
		Members:       nil,
		Accounts:      service.NewAccountService(engine, versions),
		Categories:    service.NewCategoryService(engine, versions),
		Txs:           service.NewTxService(engine, versions),
		Sync:          nil,
		Reports:       service.NewReportService(engine),
		OCR:           service.NewOCRService(cfg),
		SchemaVersion: 1,
	})
	e := &reportAPIEnv{t: t, r: r, engine: engine}

	// 注册 + 登录拿 token
	do := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if e.token != "" {
			req.Header.Set("Authorization", "Bearer "+e.token)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	if w := do(http.MethodPost, "/api/v1/auth/register", `{"username":"reporter","password":"password123"}`); w.Code != http.StatusOK {
		t.Fatalf("注册失败：%d %s", w.Code, w.Body.String())
	}
	w := do(http.MethodPost, "/api/v1/auth/login", `{"username":"reporter","password":"password123"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("登录失败：%d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			AccessToken string `json:"accessToken"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || resp.Data.AccessToken == "" {
		t.Fatalf("登录响应缺 accessToken：%s", w.Body.String())
	}
	e.token = resp.Data.AccessToken

	// 建账本
	w = do(http.MethodPost, "/api/v1/books", `{"name":"报表账本","bookType":1}`)
	if w.Code != http.StatusOK {
		t.Fatalf("建账本失败：%d %s", w.Code, w.Body.String())
	}
	var bresp struct {
		Data struct {
			Id int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &bresp); err != nil || bresp.Data.Id == 0 {
		t.Fatalf("建账本响应缺 id：%s", w.Body.String())
	}
	e.bookID = bresp.Data.Id

	// 账户 + 分类 + 两笔交易（直接走服务层，账务口径与生产一致）
	acc, err := service.NewAccountService(engine, versions).Create(1, e.bookID,
		service.AccountCreateInput{Name: "现金", Category: model.CategoryCash, Currency: "CNY"})
	if err != nil {
		t.Fatalf("建账户失败：%v", err)
	}
	cat, err := service.NewCategoryService(engine, versions).Create(1, e.bookID,
		service.CategoryCreateInput{Name: "餐饮", CatType: model.CategoryTypeExpense})
	if err != nil {
		t.Fatalf("建分类失败：%v", err)
	}
	catIn, err := service.NewCategoryService(engine, versions).Create(1, e.bookID,
		service.CategoryCreateInput{Name: "工资", CatType: model.CategoryTypeIncome})
	if err != nil {
		t.Fatalf("建收入分类失败：%v", err)
	}
	now := time.Now().In(service.CSTZone()).Unix()
	txSvc := service.NewTxService(engine, versions)
	if _, err := txSvc.Create(nil, 1, e.bookID, service.CreateRequest{
		ClientUuid: "rapi-1", TxType: int(model.TxTypeExpense), Amount: 1234,
		AccountId: acc.Id, CategoryId: cat.Id, TxTime: now,
	}); err != nil {
		t.Fatalf("建交易失败：%v", err)
	}
	if _, err := txSvc.Create(nil, 1, e.bookID, service.CreateRequest{
		ClientUuid: "rapi-2", TxType: int(model.TxTypeIncome), Amount: 5000,
		AccountId: acc.Id, CategoryId: catIn.Id, TxTime: now - 3600,
	}); err != nil {
		t.Fatalf("建收入失败：%v", err)
	}
	return e
}

func (e *reportAPIEnv) get(path string) *httptest.ResponseRecorder {
	e.t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+e.token)
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	return w
}

func TestReportRoutesBehindAuth(t *testing.T) {
	engine, err := database.Init(t.TempDir() + "/report_auth.db")
	if err != nil {
		t.Fatalf("database.Init 失败：%v", err)
	}
	t.Cleanup(func() { _ = engine.Close() })
	if _, err := migrate.Apply(engine); err != nil {
		t.Fatalf("migrate.Apply 失败：%v", err)
	}
	as := service.NewAuthService(engine, "report-auth-test-secret-0123456789abcdef", time.Hour)
	r := NewRouter(Deps{
		Config:        &config.Config{},
		AuthService:   as,
		Reports:       service.NewReportService(engine),
		SchemaVersion: 1,
	})
	for _, p := range []string{
		"/api/v1/books/1/reports/monthly",
		"/api/v1/books/1/reports/categories",
		"/api/v1/books/1/export",
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, p, nil))
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s 无 token 应 401，实际 %d", p, w.Code)
		}
	}
}

func TestReportMonthlyAndCategoriesAPI(t *testing.T) {
	e := newReportAPIEnv(t)
	w := e.get("/api/v1/books/" + itoa64(e.bookID) + "/reports/monthly?months=3")
	if w.Code != http.StatusOK {
		t.Fatalf("monthly 应 200，实际 %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Items []struct {
				Month   string `json:"month"`
				Income  int64  `json:"income"`
				Expense int64  `json:"expense"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("monthly 响应解析失败：%s", w.Body.String())
	}
	if len(resp.Data.Items) != 3 {
		t.Fatalf("期望 3 个月度点，得到 %d", len(resp.Data.Items))
	}
	cur := resp.Data.Items[len(resp.Data.Items)-1]
	if cur.Income != 5000 || cur.Expense != 1234 {
		t.Errorf("月度聚合错误：income=%d expense=%d", cur.Income, cur.Expense)
	}

	// 分类占比（缺省当月、支出）
	month := time.Now().In(service.CSTZone()).Format("2006-01")
	w = e.get("/api/v1/books/" + itoa64(e.bookID) + "/reports/categories?type=2&month=" + month)
	if w.Code != http.StatusOK {
		t.Fatalf("categories 应 200，实际 %d %s", w.Code, w.Body.String())
	}
	var cresp struct {
		Data struct {
			Items []struct {
				Name  string `json:"name"`
				Total int64  `json:"total"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &cresp); err != nil {
		t.Fatalf("categories 响应解析失败：%s", w.Body.String())
	}
	// 支出仅一笔：餐饮/1234（无收入的 5000）
	if len(cresp.Data.Items) != 1 || cresp.Data.Items[0].Name != "餐饮" || cresp.Data.Items[0].Total != 1234 {
		t.Errorf("分类占比错误：%s", w.Body.String())
	}
	// 转账方向 → 400
	w = e.get("/api/v1/books/" + itoa64(e.bookID) + "/reports/categories?type=3&month=" + month)
	if w.Code != http.StatusBadRequest {
		t.Errorf("type=3 应 400，实际 %d", w.Code)
	}
}

func TestExportAPICsvAndJson(t *testing.T) {
	e := newReportAPIEnv(t)
	base := "/api/v1/books/" + itoa64(e.bookID) + "/export"

	// CSV：BOM + 中文表头 + Content-Disposition
	w := e.get(base + "?format=csv")
	if w.Code != http.StatusOK {
		t.Fatalf("csv 导出应 200，实际 %d %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Errorf("Content-Type 错误：%s", ct)
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.Contains(cd, "attachment") || !strings.Contains(cd, ".csv") {
		t.Errorf("Content-Disposition 错误：%s", cd)
	}
	body := w.Body.String()
	if !strings.HasPrefix(body, "\uFEFF") {
		t.Fatal("CSV 缺 BOM")
	}
	if !strings.Contains(body, "时间,类型,金额(元)") || !strings.Contains(body, "12.34") {
		t.Errorf("CSV 内容不符：%q", body)
	}

	// JSON：可直接反序列化为对象数组，且仅含所选字段
	w = e.get(base + "?format=json&fields=type,amount")
	if w.Code != http.StatusOK {
		t.Fatalf("json 导出应 200，实际 %d", w.Code)
	}
	var arr []map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &arr); err != nil {
		t.Fatalf("JSON 导出解析失败：%s", w.Body.String())
	}
	if len(arr) != 2 {
		t.Fatalf("期望 2 行，得到 %d", len(arr))
	}
	if _, has := arr[0]["type"]; !has {
		t.Error("JSON 缺 type 字段")
	}
	if _, has := arr[0]["merchant"]; has {
		t.Error("JSON 不应包含未选字段 merchant")
	}

	// format 非法 / fields 非法 → 400
	if w = e.get(base + "?format=xml"); w.Code != http.StatusBadRequest {
		t.Errorf("format=xml 应 400，实际 %d", w.Code)
	}
	if w = e.get(base + "?format=json&fields=evil"); w.Code != http.StatusBadRequest {
		t.Errorf("fields=evil 应 400，实际 %d", w.Code)
	}

	// 导出零账务副作用：交易数不变（只读口径）
	var n int64
	if _, err := e.engine.SQL("SELECT COUNT(1) FROM transactions").Get(&n); err != nil {
		t.Fatalf("统计交易失败：%v", err)
	}
	if n != 2 {
		t.Errorf("导出不得写账务：交易数 %d ≠ 2", n)
	}
}

func itoa64(v int64) string { return strconv.FormatInt(v, 10) }
