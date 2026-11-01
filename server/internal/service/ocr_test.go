package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fastledger/internal/config"
)

func TestParseOCRTextRules(t *testing.T) {
	tests := []struct {
		name, text      string
		amount          int64
		direction, date string
		want            int
	}{
		{"fullwidth and grouped amount", "2024-02-29 收入 ￥１,２３４.５０", 123450, "income", "2024-02-29", 1},
		{"chinese date", "2024年2月9日 支出 微信支付 ¥12.30", 1230, "expense", "2024-02-09", 1},
		{"slash date", "2024/02/09 支出 12.30", 1230, "expense", "2024-02-09", 1},
		{"compact date", "20240209 收款 12.30", 1230, "income", "2024-02-09", 1},
		{"wechat income phrase", "2024-02-09 微信支付收入 100.00", 10000, "income", "2024-02-09", 1},
		{"alipay income phrase", "2024-02-09 支付宝收入 88.00", 8800, "income", "2024-02-09", 1},
		{"wechat expense phrase", "2024-02-09 微信支付付款 15.50", 1550, "expense", "2024-02-09", 1},
		{"bad comma group", "2024-01-01 收入 ¥10,00", 0, "", "", 0},
		{"bad comma prefix", "2024-01-01 支出 1,23", 0, "", "", 0},
		{"multiple amounts", "2024-01-01 收入 10.00 20.00", 0, "", "", 0},
		{"multiple dates", "2024-01-01 2024-01-02 收入 10.00", 0, "", "", 0},
		{"invalid date", "2023-02-29 收入 10.00", 0, "", "", 0},
		{"conflicting directions", "2024-01-01 收入 支出 10.00", 0, "", "", 0},
		// QA Round2：负号在 token 外，regex 只捕数字部分——必须整窗拒绝，绝不取绝对值。
		{"negative amount rejected", "2024-01-01 支出 -1.00", 0, "", "", 0},
		{"signed amount ambiguous", "2024-01-01 收入 +123.00", 0, "", "", 0},
		{"scientific notation rejected", "2024-01-01 支出 1e5.00", 0, "", "", 0},
		{"noise only", "hello world nothing here", 0, "", "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseOCRText(tt.text)
			if len(got) != tt.want {
				t.Fatalf("want %d candidates, got %#v", tt.want, got)
			}
			if tt.want == 1 && (got[0].AmountCents != tt.amount || got[0].Direction != tt.direction || got[0].OccurredDate != tt.date) {
				t.Fatalf("unexpected candidate: %#v", got[0])
			}
		})
	}
}

// TestSniffImageMIME 魔数嗅探：客户端声明的 MIME 不可信，内容才是事实。
func TestSniffImageMIME(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"png", []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0, 0, 0, 0}, "image/png"},
		{"jpeg", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0, 0}, "image/jpeg"},
		{"webp", append([]byte("RIFF____WEBP"), 0, 0), "image/webp"},
		{"bmp", []byte{'B', 'M', 0, 0}, "image/bmp"},
		{"tiff ii", []byte{'I', 'I', 0x2A, 0x00}, "image/tiff"},
		{"garbage", []byte("not an image at all"), ""},
		{"empty", nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sniffImageMIME(tc.data); got != tc.want {
				t.Fatalf("sniff(%s) = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

// ocrFakeBackend 模拟 Python 兜底服务：记录收到的 part Content-Type，返回契约响应。
type ocrFakeBackend struct {
	gotContentType string
	response       string
	status         int
}

func TestRecognizeDownstreamContract(t *testing.T) {
	png := append([]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, make([]byte, 32)...)

	t.Run("happy path forwards sniffed mime and accepts engine field", func(t *testing.T) {
		backend := &ocrFakeBackend{
			response: `{"engine":"rapidocr_onnxruntime","lines":[{"text":"¥12.30","confidence":0.97}]}`,
			status:   200,
		}
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseMultipartForm(10 << 20); err != nil {
				t.Errorf("backend parse multipart: %v", err)
				w.WriteHeader(400)
				return
			}
			file, hdr, err := r.FormFile("image")
			if err != nil {
				t.Errorf("backend FormFile: %v", err)
				w.WriteHeader(400)
				return
			}
			defer file.Close()
			backend.gotContentType = hdr.Header.Get("Content-Type")
			w.WriteHeader(backend.status)
			_, _ = io.WriteString(w, backend.response)
		}))
		defer srv.Close()

		svc := NewOCRService(&config.Config{OCRFallbackURL: srv.URL})
		res, err := svc.Recognize(context.Background(), 42, png, "text/html; whatever client lied")
		if err != nil {
			t.Fatalf("Recognize: %v", err)
		}
		// 转发给下游的 part MIME 必须是嗅探出的 image/png，而不是客户端伪头或 octet-stream。
		if backend.gotContentType != "image/png" {
			t.Fatalf("downstream part Content-Type = %q, want image/png", backend.gotContentType)
		}
		if res.Engine != "server" || len(res.Lines) != 1 || res.Lines[0].Text != "¥12.30" {
			t.Fatalf("unexpected result: %#v", res)
		}
	})

	t.Run("non-image payload rejected before downstream", func(t *testing.T) {
		called := false
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(200)
		}))
		defer srv.Close()
		svc := NewOCRService(&config.Config{OCRFallbackURL: srv.URL})
		if _, err := svc.Recognize(context.Background(), 42, []byte("plain text, not an image"), ""); err != ErrOCRInvalid {
			t.Fatalf("want ErrOCRInvalid, got %v", err)
		}
		if called {
			t.Fatal("非图片内容必须在校验阶段拒绝，不该打到下游")
		}
	})

	t.Run("wrong engine identifier rejected", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `{"engine":"something-else","lines":[]}`)
		}))
		defer srv.Close()
		svc := NewOCRService(&config.Config{OCRFallbackURL: srv.URL})
		if _, err := svc.Recognize(context.Background(), 42, png, ""); err != ErrOCRUnavailable {
			t.Fatalf("want ErrOCRUnavailable for wrong engine, got %v", err)
		}
	})

	t.Run("unknown fields in downstream response rejected", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `{"engine":"rapidocr_onnxruntime","lines":[],"surprise":true}`)
		}))
		defer srv.Close()
		svc := NewOCRService(&config.Config{OCRFallbackURL: srv.URL})
		if _, err := svc.Recognize(context.Background(), 42, png, ""); err != ErrOCRUnavailable {
			t.Fatalf("want ErrOCRUnavailable for unknown field, got %v", err)
		}
	})
}

// TestOCRRateLimitPerUID 同 uid 打满配额后 429 语义，异 uid 互不影响。
func TestOCRRateLimitPerUID(t *testing.T) {
	svc := NewOCRService(&config.Config{OCRRatePerMin: 2})
	for i := 0; i < 2; i++ {
		if _, err := svc.Parse(context.Background(), 7, "2024-01-01 支出 10.00"); err != nil {
			t.Fatalf("第 %d 次应放行: %v", i+1, err)
		}
	}
	if _, err := svc.Parse(context.Background(), 7, "2024-01-01 支出 10.00"); err != ErrOCRRateLimited {
		t.Fatalf("第 3 次应限流, got %v", err)
	}
	// 异 uid 独立配额
	if _, err := svc.Parse(context.Background(), 8, "2024-01-01 支出 10.00"); err != nil {
		t.Fatalf("异 uid 不应被牵连: %v", err)
	}
	// 无有效 uid 直接拒绝
	if _, err := svc.Parse(context.Background(), 0, "text"); err != ErrOCRInvalid {
		t.Fatalf("uid=0 应 ErrOCRInvalid, got %v", err)
	}
}

// TestParseResponseShape 响应 JSON 字段契约（客户端依赖的字段名）。
func TestParseResponseShape(t *testing.T) {
	svc := NewOCRService(&config.Config{})
	res, err := svc.Parse(context.Background(), 1, "2024-02-09 支出 微信支付 ¥12.30")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(res.Candidates) != 1 {
		t.Fatalf("want 1 candidate: %#v", res.Candidates)
	}
	b, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"amountCents", "direction", "occurredDate", "description", "confidence"} {
		if !strings.Contains(string(b), key) {
			t.Fatalf("响应缺少契约字段 %s: %s", key, b)
		}
	}
	if !strings.Contains(string(b), `"amountCents":1230`) {
		t.Fatalf("金额应为整数分 1230: %s", b)
	}
}
