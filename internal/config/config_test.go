package config

import (
	"testing"
)

func TestFromEnvFallsBackToLegacyAlpacaKeys(t *testing.T) {
	t.Setenv("ALPACA_API_KEY_ID", "legacy-live-key")
	t.Setenv("ALPACA_API_SECRET_KEY", "legacy-live-secret")
	t.Setenv("ALPACA_LIVE_API_KEY", "")
	t.Setenv("ALPACA_LIVE_SECRET", "")
	t.Setenv("ALPACA_PAPER_API_KEY", "")
	t.Setenv("ALPACA_PAPER_SECRET", "")

	cfg := FromEnv()

	if cfg.AlpacaLiveAPIKey != "legacy-live-key" {
		t.Fatalf("expected live api key fallback, got %q", cfg.AlpacaLiveAPIKey)
	}
	if cfg.AlpacaLiveSecret != "legacy-live-secret" {
		t.Fatalf("expected live secret fallback, got %q", cfg.AlpacaLiveSecret)
	}
	if cfg.AlpacaPaperAPIKey != "legacy-live-key" {
		t.Fatalf("expected paper api key fallback, got %q", cfg.AlpacaPaperAPIKey)
	}
	if cfg.AlpacaPaperSecret != "legacy-live-secret" {
		t.Fatalf("expected paper secret fallback, got %q", cfg.AlpacaPaperSecret)
	}
}
