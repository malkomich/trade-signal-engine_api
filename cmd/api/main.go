package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"time"

	"trade-signal-engine-api/internal/alpaca"
	"trade-signal-engine-api/internal/config"
	"trade-signal-engine-api/internal/httpapi"
	"trade-signal-engine-api/internal/notify"
	"trade-signal-engine-api/internal/store"
	"trade-signal-engine-api/internal/trading"
)

func main() {
	healthcheck := flag.Bool("healthcheck", false, "probe the local API health endpoint and exit")
	flag.Parse()
	if *healthcheck {
		os.Exit(runHealthcheck())
	}

	ctx := context.Background()
	cfg := config.FromEnv()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	st, err := store.New(ctx, cfg, logger)
	if err != nil {
		logger.Error("store initialization failed", "error", err)
		os.Exit(1)
	}
	var notifier notify.Publisher = notify.NoopPublisher{}
	switch cfg.NotifyBackend {
	case "collapse":
		notifier = notify.NewCollapsingPublisher(notify.NoopPublisher{}, 2*time.Minute)
	case "fcm":
		if cfg.ProjectID == "" {
			logger.Error("fcm backend requested without FIREBASE_PROJECT_ID")
			os.Exit(1)
		}
		fcmPublisher, err := notify.NewFCMPublisher(ctx, cfg.ProjectID, cfg.NotifyTopic)
		if err != nil {
			logger.Error("fcm publisher initialization failed", "error", err)
			os.Exit(1)
		}
		notifier = notify.NewCollapsingPublisher(fcmPublisher, 2*time.Minute)
	}
	var pushoverNotifier notify.Publisher
	if cfg.PushoverUserKey != "" && cfg.PushoverAPIToken != "" {
		pushoverPublisher, err := notify.NewPushoverPublisher(
			cfg.PushoverUserKey,
			cfg.PushoverAPIToken,
			cfg.PushoverSound,
		)
		if err != nil {
			logger.Error("pushover publisher initialization failed", "error", err)
			os.Exit(1)
		}
		pushoverNotifier = pushoverPublisher
		logger.Info("pushover notifications enabled", "sound_configured", cfg.PushoverSound != "")
	} else {
		logger.Info("pushover notifications disabled", "reason", "missing user key or api token")
	}
	var tradingService *trading.Service
	if cfg.AlpacaAPIKeyID != "" && cfg.AlpacaSecretKey != "" {
		alpacaClient := alpaca.NewClient(
			cfg.AlpacaAPIKeyID,
			cfg.AlpacaSecretKey,
			cfg.AlpacaPaperTradingURL,
			cfg.AlpacaLiveTradingURL,
			10*time.Second,
		)
		tradingService = trading.NewService(alpacaClient)
		logger.Info("alpaca trading enabled", "paper_url", cfg.AlpacaPaperTradingURL, "live_url", cfg.AlpacaLiveTradingURL)
	} else {
		logger.Info("alpaca trading disabled", "reason", "missing api key or secret")
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpapi.NewRouter(st, notifier, pushoverNotifier, logger, cfg.DefaultBenchmarkSymbol, tradingService),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info(
		"api server starting",
		"addr", cfg.HTTPAddr,
		"mode", cfg.StoreBackend,
		"pushover_enabled", pushoverNotifier != nil,
		"alpaca_enabled", tradingService != nil,
	)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server stopped unexpectedly", "error", err)
		os.Exit(1)
	}
}

func runHealthcheck() int {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	if addr[0] == ':' {
		addr = "127.0.0.1" + addr
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://" + addr + "/healthz")
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}
