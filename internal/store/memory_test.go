package store

import (
	"context"
	"testing"
	"time"

	"trade-signal-engine-api/internal/model"
)

func TestMemoryStoreSaveWindowOptimizationPersistsRecords(t *testing.T) {
	st := NewMemoryStore()
	record := model.WindowOptimization{
		ID:        "session-1:window-1:entry:exit",
		SessionID: "session-1",
		WindowID:  "window-1",
		Symbol:    "NVDA",
		Day:       "2026-04-24",
		CreatedAt: time.Date(2026, 4, 24, 13, 30, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 4, 24, 13, 30, 0, 0, time.UTC),
	}

	if err := st.SaveWindowOptimization(context.Background(), record); err != nil {
		t.Fatalf("SaveWindowOptimization() error = %v", err)
	}

	items, err := st.ListWindowOptimizations(context.Background(), "session-1")
	if err != nil {
		t.Fatalf("ListWindowOptimizations() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("ListWindowOptimizations() len = %d, want 1", len(items))
	}
	if items[0].ID != record.ID {
		t.Fatalf("ListWindowOptimizations() id = %q, want %q", items[0].ID, record.ID)
	}
}
