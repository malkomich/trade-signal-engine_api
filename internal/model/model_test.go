package model

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestSessionSummaryOmitsZeroTradingUpdatedAt(t *testing.T) {
	summary := SessionSummary{ID: "session-1"}
	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("marshal session summary: %v", err)
	}
	if strings.Contains(string(data), "trading_updated_at") {
		t.Fatalf("expected trading_updated_at to be omitted, got %s", data)
	}
}

func TestSessionSummaryIncludesTradingUpdatedAtWhenPresent(t *testing.T) {
	ts := time.Date(2026, 5, 12, 14, 30, 0, 0, time.UTC)
	summary := SessionSummary{ID: "session-1", TradingUpdatedAt: &ts}
	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("marshal session summary: %v", err)
	}
	if !strings.Contains(string(data), `"trading_updated_at":"2026-05-12T14:30:00Z"`) {
		t.Fatalf("expected trading_updated_at to be present, got %s", data)
	}
}
