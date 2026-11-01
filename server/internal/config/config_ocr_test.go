package config

import "testing"

func TestOCRConfigDefaults(t *testing.T) {
	t.Setenv("EBK_OCR_FALLBACK_URL", "")
	cfg, err := newConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OCRFallbackURL != "" || cfg.OCRTimeoutMS != 8000 || cfg.OCRMaxImageBytes != 10*1024*1024 || cfg.OCRRatePerMin != 10 {
		t.Fatalf("unexpected OCR defaults: %+v", cfg)
	}
}
