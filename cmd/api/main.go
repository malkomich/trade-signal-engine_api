package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"trade-signal-engine-api/internal/config"
	"trade-signal-engine-api/internal/httpapi"
	"trade-signal-engine-api/internal/notify"
	"trade-signal-engine-api/internal/store"
)

func main() {
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
			logger.Warn("fcm backend requested without FIREBASE_PROJECT_ID; notifications disabled")
			break
		}
		fcmPublisher, err := notify.NewFCMPublisher(ctx, cfg.ProjectID, cfg.NotifyTopic)
		if err != nil {
			logger.Error("fcm publisher initialization failed", "error", err)
			os.Exit(1)
		}
		notifier = notify.NewCollapsingPublisher(fcmPublisher, 2*time.Minute)
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpapi.NewRouter(st, notifier, logger),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("api server starting", "addr", cfg.HTTPAddr, "mode", cfg.StoreBackend)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server stopped unexpectedly", "error", err)
		os.Exit(1)
	}
}
