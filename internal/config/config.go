package config

import (
	"fmt"
	"os"
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
	DefaultBenchmarkSymbol string
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
		DefaultBenchmarkSymbol: getenv("MARKET_BENCHMARK_SYMBOL", "IXIC"),
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
