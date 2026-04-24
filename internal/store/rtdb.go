package store

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	_ "time/tzdata"

	firebase "firebase.google.com/go/v4"
	firebase_db "firebase.google.com/go/v4/db"

	"trade-signal-engine-api/internal/model"
)

type RealtimeDatabaseStore struct {
	client *firebase_db.Client
}

var newYorkLocation = func() *time.Location {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		return time.UTC
	}
	return location
}()

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
	return s.client.NewRef(nestedCollectionPath(model.CollectionDecisionEvents, record.SessionID, record.ID)).Set(ctx, record)
}

func (s *RealtimeDatabaseStore) SaveSignalEvent(ctx context.Context, event model.SignalEvent) error {
	return s.client.NewRef(nestedCollectionPath(model.CollectionSignalEvents, event.SessionID, event.ID)).Set(ctx, event)
}

func (s *RealtimeDatabaseStore) SaveMarketSnapshot(ctx context.Context, snapshot model.MarketSnapshot) error {
	return s.client.NewRef(nestedCollectionPath(
		model.CollectionMarketSnapshots,
		snapshot.SessionID,
		marketDayKeyForTime(snapshot.Timestamp),
		snapshot.ID,
	)).Set(ctx, snapshot)
}

func (s *RealtimeDatabaseStore) ListMarketSnapshots(ctx context.Context, sessionID string) ([]model.MarketSnapshot, error) {
	var raw map[string]map[string]model.MarketSnapshot
	if err := s.client.NewRef(collectionPath(model.CollectionMarketSnapshots, sessionID)).Get(ctx, &raw); err != nil {
		return nil, err
	}
	items := make([]model.MarketSnapshot, 0)
	for _, dayItems := range raw {
		for _, item := range dayItems {
			items = append(items, item)
		}
	}
	if len(items) == 0 {
		return []model.MarketSnapshot{}, nil
	}
	sort.Slice(items, func(i, j int) bool {
		if !items[i].Timestamp.Equal(items[j].Timestamp) {
			return items[i].Timestamp.Before(items[j].Timestamp)
		}
		if items[i].Symbol != items[j].Symbol {
			return items[i].Symbol < items[j].Symbol
		}
		return items[i].ID < items[j].ID
	})
	return items, nil
}

func (s *RealtimeDatabaseStore) ListDecisions(ctx context.Context, sessionID string) ([]model.DecisionRecord, error) {
	items, err := loadCollection[model.DecisionRecord](ctx, s.client.NewRef(collectionPath(model.CollectionDecisionEvents, sessionID)))
	if err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool {
		if !items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].CreatedAt.Before(items[j].CreatedAt)
		}
		return items[i].ID < items[j].ID
	})
	return items, nil
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
	items, err := loadCollection[model.ConfigVersion](ctx, s.client.NewRef(collectionPath(model.CollectionConfigVersions, sessionID)))
	if err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool {
		if !items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].UpdatedAt.Before(items[j].UpdatedAt)
		}
		if items[i].Version != items[j].Version {
			return items[i].Version < items[j].Version
		}
		return items[i].ID < items[j].ID
	})
	return items, nil
}

func (s *RealtimeDatabaseStore) SaveWindow(ctx context.Context, window model.TradeWindow) error {
	window.UpdatedAt = time.Now().UTC()
	return s.client.NewRef(nestedCollectionPath(model.CollectionTradeWindows, window.SessionID, window.ID)).Set(ctx, window)
}

func (s *RealtimeDatabaseStore) ListWindows(ctx context.Context, sessionID string) ([]model.TradeWindow, error) {
	items, err := loadCollection[model.TradeWindow](ctx, s.client.NewRef(collectionPath(model.CollectionTradeWindows, sessionID)))
	if err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool {
		if !items[i].OpenedAt.Equal(items[j].OpenedAt) {
			return items[i].OpenedAt.Before(items[j].OpenedAt)
		}
		return items[i].ID < items[j].ID
	})
	return items, nil
}

func (s *RealtimeDatabaseStore) SaveWindowSnapshot(ctx context.Context, snapshot model.WindowSnapshot) error {
	return s.client.NewRef(nestedCollectionPath(
		model.CollectionWindowSnapshots,
		snapshot.SessionID,
		marketDayKeyForTime(snapshot.CapturedAt),
		snapshot.ID,
	)).Set(ctx, snapshot)
}

func (s *RealtimeDatabaseStore) SaveWindowOptimization(ctx context.Context, optimization model.WindowOptimization) error {
	return s.client.NewRef(nestedCollectionPath(model.CollectionWindowOptimizations, optimization.SessionID, optimization.ID)).Set(ctx, optimization)
}

func (s *RealtimeDatabaseStore) ListWindowSnapshots(ctx context.Context, sessionID string) ([]model.WindowSnapshot, error) {
	items, err := loadNestedCollection[model.WindowSnapshot](ctx, s.client.NewRef(collectionPath(model.CollectionWindowSnapshots, sessionID)))
	if err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool {
		if !items[i].CapturedAt.Equal(items[j].CapturedAt) {
			return items[i].CapturedAt.Before(items[j].CapturedAt)
		}
		if items[i].Symbol != items[j].Symbol {
			return items[i].Symbol < items[j].Symbol
		}
		return items[i].ID < items[j].ID
	})
	return items, nil
}

func (s *RealtimeDatabaseStore) ListWindowOptimizations(ctx context.Context, sessionID string) ([]model.WindowOptimization, error) {
	items, err := loadCollection[model.WindowOptimization](ctx, s.client.NewRef(collectionPath(model.CollectionWindowOptimizations, sessionID)))
	if err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool {
		if !items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].CreatedAt.Before(items[j].CreatedAt)
		}
		if items[i].Symbol != items[j].Symbol {
			return items[i].Symbol < items[j].Symbol
		}
		return items[i].ID < items[j].ID
	})
	return items, nil
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

func nestedCollectionPath(parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.Trim(part, "/"); trimmed != "" {
			filtered = append(filtered, trimmed)
		}
	}
	return strings.Join(filtered, "/")
}

func marketDayKeyForTime(timestamp time.Time) string {
	return timestamp.In(newYorkLocation).Format("2006-01-02")
}

func loadCollection[T any](ctx context.Context, ref *firebase_db.Ref) ([]T, error) {
	var raw map[string]T
	if err := ref.Get(ctx, &raw); err != nil {
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

func loadNestedCollection[T any](ctx context.Context, ref *firebase_db.Ref) ([]T, error) {
	var raw map[string]map[string]T
	if err := ref.Get(ctx, &raw); err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return []T{}, nil
	}
	items := make([]T, 0)
	for _, bucket := range raw {
		for _, value := range bucket {
			items = append(items, value)
		}
	}
	return items, nil
}
