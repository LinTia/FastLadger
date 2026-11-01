package api

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"fastledger/internal/config"
	"fastledger/internal/database"
	"fastledger/internal/migrate"
	"fastledger/internal/service"
	"github.com/gin-gonic/gin"
	"xorm.io/xorm"
)

func TestOCRRoutesRegisteredBehindAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(Deps{Config: &config.Config{}, SchemaVersion: 1})
	for _, path := range []string{"/api/v1/ocr/recognize", "/api/v1/ocr/parse"} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", path, nil)
		r.ServeHTTP(w, req)
		if w.Code != 401 {
			t.Fatalf("%s expected 401, got %d", path, w.Code)
		}
	}
}

// ocrE2EEnv 真实 SQLite + 真实路由 + 真实登录（沿用 session_api_test.go 的环境口径），
// 用于补 QA 缺口：有效 JWT 下的 OCR 端到端行为与零账务副作用断言。
type ocrE2EEnv struct {
	t      *testing.T
	r      *gin.Engine
	engine *xorm.Engine
}

func newOCRE2EEnv(t *testing.T, cfg *config.Config) *ocrE2EEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine, err := database.Init(t.TempDir() + "/ocr_e2e.db")
	if err != nil {
		t.Fatalf("database.Init 失败（CGO 是否可用？）：%v", err)
	}
	t.Cleanup(func() { _ = engine.Close() })
	if _, err := migrate.Apply(engine); err != nil {
		t.Fatalf("migrate.Apply 失败：%v", err)
	}
	as := service.NewAuthService(engine, "ocr-e2e-test-secret-0123456789abcdef", time.Hour)
	r := NewRouter(Deps{
		Config:        cfg,
		AuthService:   as,
		OCR:           service.NewOCRService(cfg),
		SchemaVersion: 1,
	})
	return &ocrE2EEnv{t: t, r: r, engine: engine}
}

func (e *ocrE2EEnv) token(name string) string {
	e.t.Helper()
	post := func(path, body string) *httptest.ResponseRecorder {
		e.t.Helper()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		e.r.ServeHTTP(w, req)
		return w
	}
	if w := post("/api/v1/auth/register", `{"username":"`+name+`","password":"password123"}`); w.Code != http.StatusOK {
		e.t.Fatalf("注册失败：%d %s", w.Code, w.Body.String())
	}
	w := post("/api/v1/auth/login", `{"username":"`+name+`","password":"password123"}`)
	if w.Code != http.StatusOK {
		e.t.Fatalf("登录失败：%d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			AccessToken string `json:"accessToken"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || resp.Data.AccessToken == "" {
		e.t.Fatalf("登录响应缺 accessToken：%s", w.Body.String())
	}
	return resp.Data.AccessToken
}

// postJSON 带 JWT 的 JSON POST（parse 用）。
func (e *ocrE2EEnv) postJSON(token, body string) *httptest.ResponseRecorder {
	e.t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ocr/parse", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	return w
}

// postImage 带 JWT 的 multipart 图片 POST（recognize 用）。
func (e *ocrE2EEnv) postImage(token string, data []byte) *httptest.ResponseRecorder {
	e.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("image", "a.png")
	if err != nil {
		e.t.Fatalf("构造 multipart 失败：%v", err)
	}
	if _, err := part.Write(data); err != nil {
		e.t.Fatalf("写入图片失败：%v", err)
	}
	if err := mw.Close(); err != nil {
		e.t.Fatalf("收尾 multipart 失败：%v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ocr/recognize", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	return w
}

// pngMagic 最小 PNG 魔数（服务端嗅探只看前 8 字节；下游是 httptest stub，无需真图）。
var pngMagic = []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0, 0, 0, 0}

// TestOCRE2EWithValidJWT 覆盖 QA 缺口：有效 JWT 下 parse/recognize 完整链路、
// 服务端内容嗅探（不信任客户端 MIME）、ocr:<uid> 共享配额与 429、零账务写入。
func TestOCRE2EWithValidJWT(t *testing.T) {
	// 模拟 Python 兜底：记录收到的 part Content-Type，返回契约响应。
	backendCT := ""
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			w.WriteHeader(400)
			return
		}
		file, hdr, err := r.FormFile("image")
		if err != nil {
			w.WriteHeader(400)
			return
		}
		defer file.Close()
		backendCT = hdr.Header.Get("Content-Type")
		_, _ = io.WriteString(w, `{"engine":"rapidocr_onnxruntime","lines":[{"text":"¥12.30","confidence":0.9}]}`)
	}))
	defer backend.Close()

	e := newOCRE2EEnv(t, &config.Config{
		EnableRegister:   true,
		OCRFallbackURL:   backend.URL,
		OCRRatePerMin:    3, // 故意调低，验证 parse/recognize 共享配额
		OCRTimeoutMS:     2000,
		OCRMaxImageBytes: 10 * 1024 * 1024,
	})
	tok := e.token("ocr-e2e-user")

	// ⓪ 零账务副作用基线：注册/登录本身有审计（A1/A7），以 OCR 调用前的计数为基线，
	// 测试末尾断言 OCR 链路对 transactions / audit_logs 的【增量】为 0。
	countOf := func(table string) int64 {
		t.Helper()
		n, err := e.engine.SQL("SELECT COUNT(*) FROM " + table).Count()
		if err != nil {
			t.Fatalf("查询 %s 失败：%v", table, err)
		}
		return n
	}
	baseline := map[string]int64{
		"transactions": countOf("transactions"),
		"audit_logs":   countOf("audit_logs"),
	}

	// ① parse：有效 JWT + 合法文本 → 200 + 候选
	w := e.postJSON(tok, `{"text":"2024-02-09 支出 微信支付 ¥12.30"}`)
	if w.Code != 200 {
		t.Fatalf("parse 应 200：%d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Candidates []struct {
				AmountCents int64 `json:"amountCents"`
			} `json:"candidates"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse 响应非法：%s", w.Body.String())
	}
	if len(resp.Data.Candidates) != 1 || resp.Data.Candidates[0].AmountCents != 1230 {
		t.Fatalf("parse 结果不符：%s", w.Body.String())
	}

	// ② recognize：服务端必须按内容嗅探转发 image/png（客户端未声明也不能 octet-stream）
	w = e.postImage(tok, pngMagic)
	if w.Code != 200 {
		t.Fatalf("recognize 应 200：%d %s", w.Code, w.Body.String())
	}
	if backendCT != "image/png" {
		t.Fatalf("下游收到 %q，应为嗅探出的 image/png", backendCT)
	}

	// ③ recognize：非图片内容 → 400，且不触达下游
	before := backendCT
	w = e.postImage(tok, []byte("just text"))
	if w.Code != 400 {
		t.Fatalf("非图片应 400：%d %s", w.Code, w.Body.String())
	}
	if backendCT != before {
		t.Fatal("非图片不应触达下游")
	}

	// ④ 配额：已耗 2 次（①②）；第 3 次放行、第 4 次 429（parse/recognize 共享 ocr:<uid> 桶）
	w = e.postJSON(tok, `{"text":"2024-02-09 收入 5.00"}`)
	if w.Code != 200 {
		t.Fatalf("配额内第 3 次应 200：%d %s", w.Code, w.Body.String())
	}
	w = e.postJSON(tok, `{"text":"2024-02-09 支出 6.00"}`)
	if w.Code != 429 {
		t.Fatalf("超配额应 429：%d %s", w.Code, w.Body.String())
	}

	// ⑤ 零账务副作用：对比基线，OCR 全链路增量必须为 0
	for _, table := range []string{"transactions", "audit_logs"} {
		if got := countOf(table); got != baseline[table] {
			t.Fatalf("OCR 链路向 %s 写入了 %d 行（契约禁止）", table, got-baseline[table])
		}
	}
}

// TestOCRE2EUnconfiguredFallback 未配置兜底 URL：recognize 统一 503 友好回退（不裸 5xx 细节）。
func TestOCRE2EUnconfiguredFallback(t *testing.T) {
	e := newOCRE2EEnv(t, &config.Config{EnableRegister: true})
	tok := e.token("ocr-e2e-user2")
	w := e.postImage(tok, pngMagic)
	if w.Code != 503 {
		t.Fatalf("未配置兜底应 503：%d %s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("OCR 服务暂时不可用")) {
		t.Fatalf("503 应带友好回退文案：%s", w.Body.String())
	}
}
