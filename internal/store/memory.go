package store

import (
	"context"
	"sort"
	"sync"

	"trade-signal-engine-api/internal/model"
)

type MemoryStore struct {
	mu        sync.RWMutex
	decisions map[string][]model.DecisionRecord
	signals   map[string][]model.SignalEvent
	market    map[string][]model.MarketSnapshot
	sessions  map[string]model.SessionSummary
	windows   map[string][]model.TradeWindow
	snapshots map[string][]model.WindowSnapshot
	summaries map[string]model.WindowAnalyticsSummary
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		decisions: make(map[string][]model.DecisionRecord),
		signals:   make(map[string][]model.SignalEvent),
		market:    make(map[string][]model.MarketSnapshot),
		sessions:  make(map[string]model.SessionSummary),
		windows:   make(map[string][]model.TradeWindow),
		snapshots: make(map[string][]model.WindowSnapshot),
		summaries: make(map[string]model.WindowAnalyticsSummary),
	}
}

func (s *MemoryStore) SaveDecision(_ context.Context, record model.DecisionRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.decisions[record.SessionID] = append(s.decisions[record.SessionID], record)
	return nil
}

func (s *MemoryStore) SaveSignalEvent(_ context.Context, event model.SignalEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.signals[event.SessionID] = append(s.signals[event.SessionID], event)
	return nil
}

func (s *MemoryStore) SaveMarketSnapshot(_ context.Context, snapshot model.MarketSnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.market[snapshot.SessionID] = append(s.market[snapshot.SessionID], snapshot)
	return nil
}

func (s *MemoryStore) ListMarketSnapshots(_ context.Context, sessionID string) ([]model.MarketSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := append([]model.MarketSnapshot(nil), s.market[sessionID]...)
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

func (s *MemoryStore) ListDecisions(_ context.Context, sessionID string) ([]model.DecisionRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := append([]model.DecisionRecord(nil), s.decisions[sessionID]...)
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items, nil
}

func (s *MemoryStore) GetSession(_ context.Context, sessionID string) (model.SessionSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[sessionID]
	if !ok {
		return model.SessionSummary{}, ErrNotFound
	}
	return session, nil
}

func (s *MemoryStore) UpsertSession(_ context.Context, session model.SessionSummary) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.ID] = session
	return nil
}

func (s *MemoryStore) SaveWindow(_ context.Context, window model.TradeWindow) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := s.windows[window.SessionID]
	for index, existing := range items {
		if existing.ID == window.ID {
			items[index] = window
			return nil
		}
	}
	s.windows[window.SessionID] = append(items, window)
	return nil
}

func (s *MemoryStore) ListWindows(_ context.Context, sessionID string) ([]model.TradeWindow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := append([]model.TradeWindow(nil), s.windows[sessionID]...)
	sort.Slice(items, func(i, j int) bool { return items[i].OpenedAt.Before(items[j].OpenedAt) })
	return items, nil
}

func (s *MemoryStore) SaveWindowSnapshot(_ context.Context, snapshot model.WindowSnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots[snapshot.SessionID] = append(s.snapshots[snapshot.SessionID], snapshot)
	return nil
}

func (s *MemoryStore) ListWindowSnapshots(_ context.Context, sessionID string) ([]model.WindowSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]model.WindowSnapshot(nil), s.snapshots[sessionID]...), nil
}

func (s *MemoryStore) UpsertWindowSummary(_ context.Context, summary model.WindowAnalyticsSummary) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.summaries[summary.SessionID] = summary
	return nil
}

func (s *MemoryStore) GetWindowSummary(_ context.Context, sessionID string) (model.WindowAnalyticsSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	summary, ok := s.summaries[sessionID]
	if !ok {
		return model.WindowAnalyticsSummary{}, ErrNotFound
	}
	return summary, nil
}
