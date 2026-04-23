package store

import (
	"context"
	"sort"
	"time"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"trade-signal-engine-api/internal/model"
)

type FirestoreStore struct {
	client *firestore.Client
}

func NewFirestoreStore(ctx context.Context, projectID string) (*FirestoreStore, error) {
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: projectID})
	if err != nil {
		return nil, err
	}
	client, err := app.Firestore(ctx)
	if err != nil {
		return nil, err
	}
	return &FirestoreStore{client: client}, nil
}

func (s *FirestoreStore) SaveDecision(ctx context.Context, record model.DecisionRecord) error {
	_, err := s.client.Collection(model.CollectionDecisionEvents).Doc(record.ID).Set(ctx, record)
	return err
}

func (s *FirestoreStore) SaveSignalEvent(ctx context.Context, event model.SignalEvent) error {
	_, err := s.client.Collection(model.CollectionSignalEvents).Doc(event.ID).Set(ctx, event)
	return err
}

func (s *FirestoreStore) SaveMarketSnapshot(ctx context.Context, snapshot model.MarketSnapshot) error {
	_, err := s.client.Collection(model.CollectionMarketSnapshots).Doc(snapshot.ID).Set(ctx, snapshot)
	return err
}

func (s *FirestoreStore) ListMarketSnapshots(ctx context.Context, sessionID string) ([]model.MarketSnapshot, error) {
	docs, err := s.client.Collection(model.CollectionMarketSnapshots).Where("session_id", "==", sessionID).Documents(ctx).GetAll()
	if err != nil {
		return nil, err
	}
	items := make([]model.MarketSnapshot, 0, len(docs))
	for _, doc := range docs {
		var snapshot model.MarketSnapshot
		if err := doc.DataTo(&snapshot); err != nil {
			return nil, err
		}
		items = append(items, snapshot)
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

func (s *FirestoreStore) ListDecisions(ctx context.Context, sessionID string) ([]model.DecisionRecord, error) {
	docs, err := s.client.Collection(model.CollectionDecisionEvents).Where("session_id", "==", sessionID).Documents(ctx).GetAll()
	if err != nil {
		return nil, err
	}
	items := make([]model.DecisionRecord, 0, len(docs))
	for _, doc := range docs {
		var record model.DecisionRecord
		if err := doc.DataTo(&record); err != nil {
			return nil, err
		}
		items = append(items, record)
	}
	sort.Slice(items, func(i, j int) bool {
		if !items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].CreatedAt.Before(items[j].CreatedAt)
		}
		return items[i].ID < items[j].ID
	})
	return items, nil
}

func (s *FirestoreStore) GetSession(ctx context.Context, sessionID string) (model.SessionSummary, error) {
	doc, err := s.client.Collection(model.CollectionMarketSessions).Doc(sessionID).Get(ctx)
	if err != nil {
		return model.SessionSummary{}, mapFirestoreError(err)
	}
	var session model.SessionSummary
	if err := doc.DataTo(&session); err != nil {
		return model.SessionSummary{}, err
	}
	return session, nil
}

func (s *FirestoreStore) UpsertSession(ctx context.Context, session model.SessionSummary) error {
	session.UpdatedAt = time.Now().UTC()
	_, err := s.client.Collection(model.CollectionMarketSessions).Doc(session.ID).Set(ctx, session)
	return err
}

func (s *FirestoreStore) SaveWindow(ctx context.Context, window model.TradeWindow) error {
	window.UpdatedAt = time.Now().UTC()
	_, err := s.client.Collection(model.CollectionTradeWindows).Doc(window.ID).Set(ctx, window)
	return err
}

func (s *FirestoreStore) ListWindows(ctx context.Context, sessionID string) ([]model.TradeWindow, error) {
	docs, err := s.client.Collection(model.CollectionTradeWindows).Where("session_id", "==", sessionID).Documents(ctx).GetAll()
	if err != nil {
		return nil, err
	}
	items := make([]model.TradeWindow, 0, len(docs))
	for _, doc := range docs {
		var window model.TradeWindow
		if err := doc.DataTo(&window); err != nil {
			return nil, err
		}
		items = append(items, window)
	}
	sort.Slice(items, func(i, j int) bool {
		if !items[i].OpenedAt.Equal(items[j].OpenedAt) {
			return items[i].OpenedAt.Before(items[j].OpenedAt)
		}
		return items[i].ID < items[j].ID
	})
	return items, nil
}

func (s *FirestoreStore) SaveWindowSnapshot(ctx context.Context, snapshot model.WindowSnapshot) error {
	_, err := s.client.Collection(model.CollectionWindowSnapshots).Doc(snapshot.ID).Set(ctx, snapshot)
	return err
}

func (s *FirestoreStore) ListWindowSnapshots(ctx context.Context, sessionID string) ([]model.WindowSnapshot, error) {
	docs, err := s.client.Collection(model.CollectionWindowSnapshots).Where("session_id", "==", sessionID).Documents(ctx).GetAll()
	if err != nil {
		return nil, err
	}
	items := make([]model.WindowSnapshot, 0, len(docs))
	for _, doc := range docs {
		var snapshot model.WindowSnapshot
		if err := doc.DataTo(&snapshot); err != nil {
			return nil, err
		}
		items = append(items, snapshot)
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

func (s *FirestoreStore) UpsertWindowSummary(ctx context.Context, summary model.WindowAnalyticsSummary) error {
	summary.UpdatedAt = time.Now().UTC()
	_, err := s.client.Collection(model.CollectionWindowSummaries).Doc(summary.SessionID).Set(ctx, summary)
	return err
}

func (s *FirestoreStore) GetWindowSummary(ctx context.Context, sessionID string) (model.WindowAnalyticsSummary, error) {
	doc, err := s.client.Collection(model.CollectionWindowSummaries).Doc(sessionID).Get(ctx)
	if err != nil {
		return model.WindowAnalyticsSummary{}, mapFirestoreError(err)
	}
	var summary model.WindowAnalyticsSummary
	if err := doc.DataTo(&summary); err != nil {
		return model.WindowAnalyticsSummary{}, err
	}
	return summary, nil
}

func mapFirestoreError(err error) error {
	if status.Code(err) == codes.NotFound {
		return ErrNotFound
	}
	return err
}
