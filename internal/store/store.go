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
	ListDecisions(context.Context, string) ([]model.DecisionRecord, error)
	GetSession(context.Context, string) (model.SessionSummary, error)
	UpsertSession(context.Context, model.SessionSummary) error
	SaveWindow(context.Context, model.TradeWindow) error
	ListWindows(context.Context, string) ([]model.TradeWindow, error)
}

func New(ctx context.Context, cfg config.Config, logger *slog.Logger) (Store, error) {
	if cfg.StoreBackend == "firestore" {
		st, err := NewFirestoreStore(ctx, cfg.ProjectID)
		if err == nil {
			logger.Info("using firestore store", "project_id", cfg.ProjectID)
			return st, nil
		}
		logger.Warn("falling back to memory store", "error", err)
	}
	return NewMemoryStore(), nil
}
