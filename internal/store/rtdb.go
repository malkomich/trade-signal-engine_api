package store

import (
	"context"
	"errors"
	"sort"
	"time"

	firebase "firebase.google.com/go/v4"
	firebase_db "firebase.google.com/go/v4/db"

	"trade-signal-engine-api/internal/model"
)

type RealtimeDatabaseStore struct {
	client *firebase_db.Client
}

func NewRealtimeDatabaseStore(ctx context.Context, projectID string, databaseURL string) (*RealtimeDatabaseStore, error) {
	if projectID == "" {
		return nil, errors.New("project id is required")
	}
	app, err := firebase.NewApp(ctx, &firebase.Config{
		ProjectID:   projectID,
		DatabaseURL: databaseURL,
	})
	if err != nil {
		return nil, err
	}
	client, err := app.Database(ctx)
	if err != nil {
		return nil, err
	}
	return &RealtimeDatabaseStore{client: client}, nil
}

func (s *RealtimeDatabaseStore) SaveDecision(ctx context.Context, record model.DecisionRecord) error {
	return s.client.NewRef(collectionPath(model.CollectionDecisionEvents, record.ID)).Set(ctx, record)
}

func (s *RealtimeDatabaseStore) SaveSignalEvent(ctx context.Context, event model.SignalEvent) error {
	return s.client.NewRef(collectionPath(model.CollectionSignalEvents, event.ID)).Set(ctx, event)
}

func (s *RealtimeDatabaseStore) SaveMarketSnapshot(ctx context.Context, snapshot model.MarketSnapshot) error {
	return s.client.NewRef(collectionPath(model.CollectionMarketSnapshots, snapshot.ID)).Set(ctx, snapshot)
}

func (s *RealtimeDatabaseStore) ListMarketSnapshots(ctx context.Context, sessionID string) ([]model.MarketSnapshot, error) {
	items, err := loadCollection[model.MarketSnapshot](ctx, s.client, model.CollectionMarketSnapshots)
	if err != nil {
		return nil, err
	}
	filtered := filterBySession(items, sessionID)
	sort.Slice(filtered, func(i, j int) bool {
		if !filtered[i].Timestamp.Equal(filtered[j].Timestamp) {
			return filtered[i].Timestamp.Before(filtered[j].Timestamp)
		}
		if filtered[i].Symbol != filtered[j].Symbol {
			return filtered[i].Symbol < filtered[j].Symbol
		}
		return filtered[i].ID < filtered[j].ID
	})
	return filtered, nil
}

func (s *RealtimeDatabaseStore) ListDecisions(ctx context.Context, sessionID string) ([]model.DecisionRecord, error) {
	items, err := loadCollection[model.DecisionRecord](ctx, s.client, model.CollectionDecisionEvents)
	if err != nil {
		return nil, err
	}
	filtered := filterBySession(items, sessionID)
	sort.Slice(filtered, func(i, j int) bool {
		if !filtered[i].CreatedAt.Equal(filtered[j].CreatedAt) {
			return filtered[i].CreatedAt.Before(filtered[j].CreatedAt)
		}
		return filtered[i].ID < filtered[j].ID
	})
	return filtered, nil
}

func (s *RealtimeDatabaseStore) GetSession(ctx context.Context, sessionID string) (model.SessionSummary, error) {
	var session model.SessionSummary
	if err := s.client.NewRef(collectionPath(model.CollectionMarketSessions, sessionID)).Get(ctx, &session); err != nil {
		return model.SessionSummary{}, err
	}
	if session.ID == "" {
		return model.SessionSummary{}, ErrNotFound
	}
	return session, nil
}

func (s *RealtimeDatabaseStore) UpsertSession(ctx context.Context, session model.SessionSummary) error {
	session.UpdatedAt = time.Now().UTC()
	return s.client.NewRef(collectionPath(model.CollectionMarketSessions, session.ID)).Set(ctx, session)
}

func (s *RealtimeDatabaseStore) ListConfigVersions(ctx context.Context, sessionID string) ([]model.ConfigVersion, error) {
	items, err := loadCollection[model.ConfigVersion](ctx, s.client, model.CollectionConfigVersions)
	if err != nil {
		return nil, err
	}
	filtered := filterBySession(items, sessionID)
	sort.Slice(filtered, func(i, j int) bool {
		if !filtered[i].UpdatedAt.Equal(filtered[j].UpdatedAt) {
			return filtered[i].UpdatedAt.Before(filtered[j].UpdatedAt)
		}
		if filtered[i].Version != filtered[j].Version {
			return filtered[i].Version < filtered[j].Version
		}
		return filtered[i].ID < filtered[j].ID
	})
	return filtered, nil
}

func (s *RealtimeDatabaseStore) SaveWindow(ctx context.Context, window model.TradeWindow) error {
	window.UpdatedAt = time.Now().UTC()
	return s.client.NewRef(collectionPath(model.CollectionTradeWindows, window.ID)).Set(ctx, window)
}

func (s *RealtimeDatabaseStore) ListWindows(ctx context.Context, sessionID string) ([]model.TradeWindow, error) {
	items, err := loadCollection[model.TradeWindow](ctx, s.client, model.CollectionTradeWindows)
	if err != nil {
		return nil, err
	}
	filtered := filterBySession(items, sessionID)
	sort.Slice(filtered, func(i, j int) bool {
		if !filtered[i].OpenedAt.Equal(filtered[j].OpenedAt) {
			return filtered[i].OpenedAt.Before(filtered[j].OpenedAt)
		}
		return filtered[i].ID < filtered[j].ID
	})
	return filtered, nil
}

func (s *RealtimeDatabaseStore) SaveWindowSnapshot(ctx context.Context, snapshot model.WindowSnapshot) error {
	return s.client.NewRef(collectionPath(model.CollectionWindowSnapshots, snapshot.ID)).Set(ctx, snapshot)
}

func (s *RealtimeDatabaseStore) SaveWindowOptimization(ctx context.Context, optimization model.WindowOptimization) error {
	return s.client.NewRef(collectionPath(model.CollectionWindowOptimizations, optimization.ID)).Set(ctx, optimization)
}

func (s *RealtimeDatabaseStore) ListWindowSnapshots(ctx context.Context, sessionID string) ([]model.WindowSnapshot, error) {
	items, err := loadCollection[model.WindowSnapshot](ctx, s.client, model.CollectionWindowSnapshots)
	if err != nil {
		return nil, err
	}
	filtered := filterBySession(items, sessionID)
	sort.Slice(filtered, func(i, j int) bool {
		if !filtered[i].CapturedAt.Equal(filtered[j].CapturedAt) {
			return filtered[i].CapturedAt.Before(filtered[j].CapturedAt)
		}
		if filtered[i].Symbol != filtered[j].Symbol {
			return filtered[i].Symbol < filtered[j].Symbol
		}
		return filtered[i].ID < filtered[j].ID
	})
	return filtered, nil
}

func (s *RealtimeDatabaseStore) ListWindowOptimizations(ctx context.Context, sessionID string) ([]model.WindowOptimization, error) {
	items, err := loadCollection[model.WindowOptimization](ctx, s.client, model.CollectionWindowOptimizations)
	if err != nil {
		return nil, err
	}
	filtered := filterBySession(items, sessionID)
	sort.Slice(filtered, func(i, j int) bool {
		if !filtered[i].CreatedAt.Equal(filtered[j].CreatedAt) {
			return filtered[i].CreatedAt.Before(filtered[j].CreatedAt)
		}
		if filtered[i].Symbol != filtered[j].Symbol {
			return filtered[i].Symbol < filtered[j].Symbol
		}
		return filtered[i].ID < filtered[j].ID
	})
	return filtered, nil
}

func (s *RealtimeDatabaseStore) UpsertWindowSummary(ctx context.Context, summary model.WindowAnalyticsSummary) error {
	summary.UpdatedAt = time.Now().UTC()
	return s.client.NewRef(collectionPath(model.CollectionWindowSummaries, summary.SessionID)).Set(ctx, summary)
}

func (s *RealtimeDatabaseStore) GetWindowSummary(ctx context.Context, sessionID string) (model.WindowAnalyticsSummary, error) {
	var summary model.WindowAnalyticsSummary
	if err := s.client.NewRef(collectionPath(model.CollectionWindowSummaries, sessionID)).Get(ctx, &summary); err != nil {
		return model.WindowAnalyticsSummary{}, err
	}
	if summary.SessionID == "" {
		return model.WindowAnalyticsSummary{}, ErrNotFound
	}
	return summary, nil
}

func collectionPath(collectionName, id string) string {
	if id == "" {
		return collectionName
	}
	return collectionName + "/" + id
}

type sessionScoped interface {
	GetSessionID() string
}

func filterBySession[T any](items []T, sessionID string) []T {
	filtered := make([]T, 0, len(items))
	for _, item := range items {
		if sessionScopedItemSessionID(item) == sessionID {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func sessionScopedItemSessionID[T any](item T) string {
	switch value := any(item).(type) {
	case model.DecisionRecord:
		return value.SessionID
	case model.SignalEvent:
		return value.SessionID
	case model.MarketSnapshot:
		return value.SessionID
	case model.ConfigVersion:
		return value.SessionID
	case model.TradeWindow:
		return value.SessionID
	case model.WindowSnapshot:
		return value.SessionID
	case model.WindowOptimization:
		return value.SessionID
	default:
		return ""
	}
}

func loadCollection[T any](ctx context.Context, client *firebase_db.Client, collectionName string) ([]T, error) {
	var raw map[string]T
	if err := client.NewRef(collectionName).Get(ctx, &raw); err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return []T{}, nil
	}
	items := make([]T, 0, len(raw))
	for _, value := range raw {
		items = append(items, value)
	}
	return items, nil
}
