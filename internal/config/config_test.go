package config

import (
	"testing"
)

func TestFromEnvReadsAlpacaKeys(t *testing.T) {
	t.Setenv("ALPACA_LIVE_API_KEY", "live-key")
	t.Setenv("ALPACA_LIVE_SECRET", "live-secret")
	t.Setenv("ALPACA_PAPER_API_KEY", "paper-key")
	t.Setenv("ALPACA_PAPER_SECRET", "paper-secret")

	cfg := FromEnv()

	if cfg.AlpacaLiveAPIKey != "live-key" {
		t.Fatalf("expected live api key, got %q", cfg.AlpacaLiveAPIKey)
	}
	if cfg.AlpacaLiveSecret != "live-secret" {
		t.Fatalf("expected live secret, got %q", cfg.AlpacaLiveSecret)
	}
	if cfg.AlpacaPaperAPIKey != "paper-key" {
		t.Fatalf("expected paper api key, got %q", cfg.AlpacaPaperAPIKey)
	}
	if cfg.AlpacaPaperSecret != "paper-secret" {
		t.Fatalf("expected paper secret, got %q", cfg.AlpacaPaperSecret)
	}
}
