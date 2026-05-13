package config

import (
	"fmt"
	"os"
	"strings"
)

const defaultDatabaseURLTemplate = "https://%s-default-rtdb.firebaseio.com/"

type Config struct {
	HTTPAddr               string
	Environment            string
	ProjectID              string
	DatabaseURL            string
	StoreBackend           string
	NotifyBackend          string
	NotifyTopic            string
	PushoverUserKey        string
	PushoverAPIToken       string
	PushoverSound          string
	AlpacaLiveAPIKey       string
	AlpacaLiveSecret       string
	AlpacaPaperAPIKey      string
	AlpacaPaperSecret      string
	AlpacaPaperTradingURL  string
	AlpacaLiveTradingURL   string
	DefaultBenchmarkSymbol string
	CORSAllowedOrigins     []string
}

func FromEnv() Config {
	cfg := Config{
		HTTPAddr:               getenv("HTTP_ADDR", ":8080"),
		Environment:            getenv("ENVIRONMENT", "local"),
		ProjectID:              os.Getenv("FIREBASE_PROJECT_ID"),
		DatabaseURL:            os.Getenv("FIREBASE_DATABASE_URL"),
		StoreBackend:           getenv("STORE_BACKEND", "memory"),
		NotifyBackend:          getenv("NOTIFICATION_BACKEND", "noop"),
		NotifyTopic:            getenv("FCM_TOPIC", "trade-signal-engine"),
		PushoverUserKey:        os.Getenv("PUSHOVER_USER_KEY"),
		PushoverAPIToken:       os.Getenv("PUSHOVER_API_TOKEN"),
		PushoverSound:          os.Getenv("PUSHOVER_SOUND"),
		AlpacaLiveAPIKey:       getenvAny("ALPACA_LIVE_API_KEY", "ALPACA_API_KEY_ID"),
		AlpacaLiveSecret:       getenvAny("ALPACA_LIVE_SECRET", "ALPACA_API_SECRET_KEY"),
		AlpacaPaperAPIKey:      getenvAny("ALPACA_PAPER_API_KEY", "ALPACA_API_KEY_ID"),
		AlpacaPaperSecret:      getenvAny("ALPACA_PAPER_SECRET", "ALPACA_API_SECRET_KEY"),
		AlpacaPaperTradingURL:  getenv("ALPACA_PAPER_TRADING_URL", "https://paper-api.alpaca.markets"),
		AlpacaLiveTradingURL:   getenv("ALPACA_LIVE_TRADING_URL", "https://api.alpaca.markets"),
		DefaultBenchmarkSymbol: getenv("MARKET_BENCHMARK_SYMBOL", "IXIC"),
		CORSAllowedOrigins:     splitList(getenv("CORS_ALLOWED_ORIGINS", "")),
	}
	if cfg.DatabaseURL == "" && cfg.ProjectID != "" {
		cfg.DatabaseURL = defaultDatabaseURL(cfg.ProjectID)
	}
	return cfg
}

func defaultDatabaseURL(projectID string) string {
	return fmt.Sprintf(defaultDatabaseURLTemplate, projectID)
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getenvAny(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}

func splitList(value string) []string {
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}
