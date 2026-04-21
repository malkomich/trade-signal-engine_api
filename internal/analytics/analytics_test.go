package analytics

import (
	"testing"
	"time"

	"trade-signal-engine-api/internal/model"
)

func TestBuildWindowSummary(t *testing.T) {
	windows := []model.TradeWindow{
		{Symbol: "AAPL", Status: "open", EntryScore: 1.5, ExitScore: 0.4},
		{Symbol: "MSFT", Status: "closed", EntryScore: 2.5, ExitScore: 1.1},
	}
	snapshots := []model.WindowSnapshot{
		{Symbol: "AAPL", Phase: "entry"},
		{Symbol: "MSFT", Phase: "closed"},
	}

	summary := BuildWindowSummary("session-1", windows, snapshots, time.Unix(100, 0).UTC())

	if summary.SessionID != "session-1" {
		t.Fatalf("unexpected session id: %s", summary.SessionID)
	}
	if summary.SnapshotCount != 2 {
		t.Fatalf("unexpected snapshot count: %d", summary.SnapshotCount)
	}
	if summary.OpenWindows != 1 || summary.ClosedWindows != 1 {
		t.Fatalf("unexpected window counts: open=%d closed=%d", summary.OpenWindows, summary.ClosedWindows)
	}
	if got := len(summary.Symbols); got != 2 {
		t.Fatalf("unexpected symbols count: %d", got)
	}
	if summary.LastPhase != "closed" {
		t.Fatalf("unexpected last phase: %s", summary.LastPhase)
	}
	if summary.AverageEntryScore <= 0 || summary.AverageExitScore <= 0 {
		t.Fatalf("expected non-zero averages")
	}
}
