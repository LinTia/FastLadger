package api

// 投资/净资产 handler 级测试（真实 SQLite + 真实登录；需 CGO）。
//
// 覆盖：路由鉴权 401、标的/持仓 CRUD 流、录价、净资产信封、非法 payload 400。
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

type investAPIEnv struct {
	t      *testing.T
	r      *gin.Engine
	engine *xorm.Engine
	token  string
	bookID int64
	accID  int64
	do     func(method, path, body, token string) *httptest.ResponseRecorder
}

func newInvestAPIEnv(t *testing.T) *investAPIEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine, err := database.Init(t.TempDir() + "/invest_api.db")
	if err != nil {
		t.Fatalf("database.Init 失败：%v", err)
	}
	t.Cleanup(func() { _ = engine.Close() })
	if _, err := migrate.Apply(engine); err != nil {
		t.Fatalf("migrate.Apply 失败：%v", err)
	}
	as := service.NewAuthService(engine, "invest-api-test-secret-0123456789abcdef", time.Hour)
	versions := version.NewRepo()
	cfg := &config.Config{EnableRegister: true}
	r := NewRouter(Deps{
		Config:        cfg,
		AuthService:   as,
		Books:         service.NewBookService(engine, versions),
		Accounts:      service.NewAccountService(engine, versions),
		Categories:    service.NewCategoryService(engine, versions),
		Txs:           service.NewTxService(engine, versions),
		Invest:        service.NewInvestService(engine, versions),
		OCR:           service.NewOCRService(cfg),
		SchemaVersion: 1,
	})
	e := &investAPIEnv{t: t, r: r, engine: engine}
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

	if w := do(http.MethodPost, "/api/v1/auth/register", `{"username":"investor","password":"password123"}`, ""); w.Code != http.StatusOK {
		t.Fatalf("注册失败：%d %s", w.Code, w.Body.String())
	}
	w := do(http.MethodPost, "/api/v1/auth/login", `{"username":"investor","password":"password123"}`, "")
	var resp struct {
		Data struct {
			AccessToken string `json:"accessToken"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || resp.Data.AccessToken == "" {
		t.Fatalf("登录失败：%s", w.Body.String())
	}
	e.token = resp.Data.AccessToken

	w = do(http.MethodPost, "/api/v1/books", `{"name":"投资账本","bookType":1}`, e.token)
	var bresp struct {
		Data struct {
			Id int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &bresp); err != nil || bresp.Data.Id == 0 {
		t.Fatalf("建账本失败：%s", w.Body.String())
	}
	e.bookID = bresp.Data.Id

	acc, err := service.NewAccountService(engine, versions).Create(1, e.bookID,
		service.AccountCreateInput{Name: "券商账户", Category: model.CategoryInvestment, Currency: "CNY"})
	if err != nil {
		t.Fatalf("建账户失败：%v", err)
	}
	e.accID = acc.Id
	return e
}

func TestInvestRoutesBehindAuth(t *testing.T) {
	r := NewRouter(Deps{Config: &config.Config{}, Invest: service.NewInvestService(nil, nil), SchemaVersion: 1})
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/books/1/instruments"},
		{http.MethodPost, "/api/v1/books/1/instruments"},
		{http.MethodPost, "/api/v1/books/1/instruments/1/price"},
		{http.MethodGet, "/api/v1/books/1/holdings"},
		{http.MethodPost, "/api/v1/books/1/holdings"},
		{http.MethodDelete, "/api/v1/books/1/holdings/1"},
		{http.MethodGet, "/api/v1/books/1/net-worth"},
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s 无 token 应 401，实际 %d", tc.method, tc.path, w.Code)
		}
	}
}

func TestInvestCRUDAndNetWorthFlow(t *testing.T) {
	e := newInvestAPIEnv(t)
	book := strconv.FormatInt(e.bookID, 10)

	// 建标的
	w := e.do(http.MethodPost, "/api/v1/books/"+book+"/instruments",
		`{"name":"测试基金","type":2,"market":"","code":"510300"}`, e.token)
	if w.Code != http.StatusOK {
		t.Fatalf("建标的失败：%d %s", w.Code, w.Body.String())
	}
	var iresp struct {
		Data struct {
			Id int64 `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &iresp)

	// 建持仓（十进制字符串）
	w = e.do(http.MethodPost, "/api/v1/books/"+book+"/holdings",
		`{"accountId":`+strconv.FormatInt(e.accID, 10)+`,"instrumentId":`+strconv.FormatInt(iresp.Data.Id, 10)+
			`,"quantity":"100","costAmount":100000,"currentPrice":"12.34"}`, e.token)
	if w.Code != http.StatusOK {
		t.Fatalf("建持仓失败：%d %s", w.Code, w.Body.String())
	}
	var hresp struct {
		Data struct {
			Id int64 `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &hresp)

	// 非法数量 → 400
	w = e.do(http.MethodPost, "/api/v1/books/"+book+"/holdings",
		`{"accountId":`+strconv.FormatInt(e.accID, 10)+`,"instrumentId":`+strconv.FormatInt(iresp.Data.Id, 10)+`,"quantity":"abc"}`, e.token)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("数量 abc 应 400，实际 %d", w.Code)
	}

	// 录价 → 持仓现价刷新
	w = e.do(http.MethodPost, "/api/v1/books/"+book+"/instruments/"+strconv.FormatInt(iresp.Data.Id, 10)+"/price",
		`{"price":"13.50"}`, e.token)
	if w.Code != http.StatusOK {
		t.Fatalf("录价失败：%d %s", w.Code, w.Body.String())
	}

	// 持仓列表：市值 = 100 × 13.50 × 100 = 135000 分
	w = e.do(http.MethodGet, "/api/v1/books/"+book+"/holdings", "", e.token)
	var hl struct {
		Data struct {
			Items []struct {
				MarketValue  int64  `json:"marketValue"`
				ProfitLoss   int64  `json:"profitLoss"`
				CurrentPrice string `json:"currentPrice"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &hl); err != nil || len(hl.Data.Items) != 1 {
		t.Fatalf("持仓列表错误：%s", w.Body.String())
	}
	if hl.Data.Items[0].MarketValue != 135000 || hl.Data.Items[0].CurrentPrice != "13.5" {
		t.Errorf("市值/现价错误：%+v", hl.Data.Items[0])
	}

	// 净资产信封
	w = e.do(http.MethodGet, "/api/v1/books/"+book+"/net-worth", "", e.token)
	if w.Code != http.StatusOK {
		t.Fatalf("净资产失败：%d %s", w.Code, w.Body.String())
	}
	var nw struct {
		Data struct {
			TotalAssets      int64 `json:"totalAssets"`
			TotalLiabilities int64 `json:"totalLiabilities"`
			NetWorth         int64 `json:"netWorth"`
			HoldingsValue    int64 `json:"holdingsValue"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &nw); err != nil {
		t.Fatalf("净资产解析失败：%s", w.Body.String())
	}
	if nw.Data.HoldingsValue != 135000 || nw.Data.TotalAssets != 135000 || nw.Data.NetWorth != 135000 {
		t.Errorf("净资产数据错误：%+v", nw.Data)
	}

	// 删除持仓 → 404 后续删除
	w = e.do(http.MethodDelete, "/api/v1/books/"+book+"/holdings/"+strconv.FormatInt(hresp.Data.Id, 10), "", e.token)
	if w.Code != http.StatusOK {
		t.Fatalf("删持仓失败：%d", w.Code)
	}
	w = e.do(http.MethodDelete, "/api/v1/books/"+book+"/holdings/"+strconv.FormatInt(hresp.Data.Id, 10), "", e.token)
	if w.Code != http.StatusNotFound {
		t.Fatalf("重复删除应 404，实际 %d", w.Code)
	}
}
