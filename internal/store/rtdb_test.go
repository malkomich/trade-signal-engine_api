package store

import (
	"testing"
	"time"
)

func TestNestedCollectionPath(t *testing.T) {
	t.Parallel()

	got := nestedCollectionPath("market_snapshots", "session-1", "2026-04-22", "snapshot-1")
	want := "market_snapshots/session-1/2026-04-22/snapshot-1"
	if got != want {
		t.Fatalf("nestedCollectionPath() = %q, want %q", got, want)
	}
}

func TestMarketDayKeyForTimeUsesNewYorkDayBoundary(t *testing.T) {
	t.Parallel()

	timestamp := time.Date(2026, 4, 23, 3, 30, 0, 0, time.UTC)
	got := marketDayKeyForTime(timestamp)
	want := "2026-04-22"
	if got != want {
		t.Fatalf("marketDayKeyForTime() = %q, want %q", got, want)
	}
}
