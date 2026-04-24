package store

import (
	"context"
	"errors"
	"log/slog"

	"trade-signal-engine-api/internal/config"
	"trade-signal-engine-api/internal/model"
)

var ErrNotFound = errors.New("not found")

type Store interface {
	SaveDecision(context.Context, model.DecisionRecord) error
	SaveSignalEvent(context.Context, model.SignalEvent) error
	SaveMarketSnapshot(context.Context, model.MarketSnapshot) error
	ListMarketSnapshots(context.Context, string) ([]model.MarketSnapshot, error)
	ListDecisions(context.Context, string) ([]model.DecisionRecord, error)
	GetSession(context.Context, string) (model.SessionSummary, error)
	UpsertSession(context.Context, model.SessionSummary) error
	ListConfigVersions(context.Context, string) ([]model.ConfigVersion, error)
	SaveWindow(context.Context, model.TradeWindow) error
	ListWindows(context.Context, string) ([]model.TradeWindow, error)
	SaveWindowSnapshot(context.Context, model.WindowSnapshot) error
	ListWindowSnapshots(context.Context, string) ([]model.WindowSnapshot, error)
	SaveWindowOptimization(context.Context, model.WindowOptimization) error
	ListWindowOptimizations(context.Context, string) ([]model.WindowOptimization, error)
	UpsertWindowSummary(context.Context, model.WindowAnalyticsSummary) error
	GetWindowSummary(context.Context, string) (model.WindowAnalyticsSummary, error)
}

func New(ctx context.Context, cfg config.Config, logger *slog.Logger) (Store, error) {
	switch cfg.StoreBackend {
	case "memory":
		return NewMemoryStore(), nil
	case "rtdb":
		st, err := NewRealtimeDatabaseStore(ctx, cfg.ProjectID, cfg.DatabaseURL)
		if err == nil {
			logger.Info("using realtime database store", "project_id", cfg.ProjectID, "database_url", cfg.DatabaseURL)
			return st, nil
		}
		return nil, err
	default:
		logger.Warn("unknown store backend requested, using memory store", "store_backend", cfg.StoreBackend)
		return NewMemoryStore(), nil
	}
}
