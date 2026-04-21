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
	sessions  map[string]model.SessionSummary
	windows   map[string][]model.TradeWindow
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		decisions: make(map[string][]model.DecisionRecord),
		sessions:  make(map[string]model.SessionSummary),
		windows:   make(map[string][]model.TradeWindow),
	}
}

func (s *MemoryStore) SaveDecision(_ context.Context, record model.DecisionRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.decisions[record.SessionID] = append(s.decisions[record.SessionID], record)
	return nil
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
	s.windows[window.SessionID] = append(s.windows[window.SessionID], window)
	return nil
}

func (s *MemoryStore) ListWindows(_ context.Context, sessionID string) ([]model.TradeWindow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := append([]model.TradeWindow(nil), s.windows[sessionID]...)
	return items, nil
}
