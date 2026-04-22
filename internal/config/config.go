package config

import "os"

type Config struct {
	HTTPAddr               string
	Environment            string
	ProjectID              string
	StoreBackend           string
	NotifyBackend          string
	NotifyTopic            string
	DefaultBenchmarkSymbol string
}

func FromEnv() Config {
	cfg := Config{
		HTTPAddr:               getenv("HTTP_ADDR", ":8080"),
		Environment:            getenv("ENVIRONMENT", "local"),
		ProjectID:              os.Getenv("FIREBASE_PROJECT_ID"),
		StoreBackend:           getenv("STORE_BACKEND", "memory"),
		NotifyBackend:          getenv("NOTIFICATION_BACKEND", "noop"),
		NotifyTopic:            getenv("FCM_TOPIC", "trade-signal-engine"),
		DefaultBenchmarkSymbol: getenv("MARKET_BENCHMARK_SYMBOL", "IXIC"),
	}
	return cfg
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
