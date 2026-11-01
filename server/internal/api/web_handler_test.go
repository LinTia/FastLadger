package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fastledger/internal/config"
	"github.com/gin-gonic/gin"
)

// TestWebStaticServed Web 端静态资源经 NoRoute 兜底服务（WBS 5.9）：
// 根路径回 index.html、静态文件按名命中、未知路径回退 index.html。
func TestWebStaticServed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(Deps{Config: &config.Config{}, SchemaVersion: 1})

	t.Run("root returns index.html", func(t *testing.T) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("GET / 应 200，实际 %d", w.Code)
		}
		body := w.Body.String()
		if !strings.Contains(body, "<!DOCTYPE html>") || !strings.Contains(body, "FastLedger") {
			t.Fatalf("GET / 应返回 index.html，实际前 200 字节：%s", body[:min(200, len(body))])
		}
	})

	t.Run("static assets by name", func(t *testing.T) {
		for _, p := range []string{"/app.js", "/style.css"} {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, p, nil))
			if w.Code != http.StatusOK {
				t.Fatalf("GET %s 应 200，实际 %d", p, w.Code)
			}
			if strings.Contains(w.Body.String(), "<!DOCTYPE html>") {
				t.Fatalf("GET %s 不应回退到 index.html", p)
			}
		}
	})

	t.Run("unknown path falls back to index.html", func(t *testing.T) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/some/spa/route", nil))
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "<!DOCTYPE html>") {
			t.Fatalf("未知 GET 路径应回退 index.html，实际 %d", w.Code)
		}
	})

	t.Run("api 404 not swallowed", func(t *testing.T) {
		// 未注册的 API 路径必须保持 404 JSON，绝不能被静态回退吞掉
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/definitely-not-a-route", nil))
		if w.Code != http.StatusNotFound {
			t.Fatalf("未注册 API 路径应 404，实际 %d", w.Code)
		}
		// 未注册的 POST 也保持 404（非 GET 不做静态回退）
		w = httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/whatever", nil))
		if w.Code != http.StatusNotFound {
			t.Fatalf("未知 POST 应 404，实际 %d", w.Code)
		}
	})
}

// TestWebPathTraversalDenied 内嵌 FS 路径语义拒绝目录穿越。
func TestWebPathTraversalDenied(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(Deps{Config: &config.Config{}, SchemaVersion: 1})
	for _, p := range []string{"/../internal/api/router.go", "/..%2f..%2fserver%2fgo.mod", "/%2e%2e/go.mod"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, p, nil))
		body := w.Body.String()
		if strings.Contains(body, "package api") || strings.Contains(body, "module fastledger") {
			t.Fatalf("路径 %q 穿透了内嵌文件系统（泄露源码）", p)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
