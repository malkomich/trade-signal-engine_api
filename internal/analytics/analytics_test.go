package analytics

import (
	"testing"
	"time"

	"trade-signal-engine-api/internal/model"
)

func TestSnapshotFromDecision(t *testing.T) {
	snapshot := SnapshotFromDecision(
		model.DecisionRecord{
			ID:        "decision-1",
			SessionID: "session-1",
			Symbol:    "AAPL",
			EventType: model.EventTypeDecisionAccepted,
		},
		&model.TradeWindow{ID: "window-1"},
	)

	if snapshot.ID != "session-1:decision-1:decision.accepted:window-1" {
		t.Fatalf("unexpected snapshot id: %s", snapshot.ID)
	}
	if snapshot.WindowID != "window-1" {
		t.Fatalf("unexpected window id: %s", snapshot.WindowID)
	}
}

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

func TestUpdateWindowSummary(t *testing.T) {
	summary := UpdateWindowSummary(
		model.WindowAnalyticsSummary{SessionID: "session-1"},
		model.WindowSnapshot{SessionID: "session-1", Symbol: "AAPL", Phase: "entry", EntryScore: 1.25, ExitScore: 0.5},
		&model.TradeWindow{Symbol: "AAPL", Status: "open"},
		time.Unix(100, 0).UTC(),
	)

	if summary.SessionID != "session-1" {
		t.Fatalf("unexpected session id: %s", summary.SessionID)
	}
	if summary.SnapshotCount != 1 {
		t.Fatalf("unexpected snapshot count: %d", summary.SnapshotCount)
	}
	if summary.OpenWindows != 1 {
		t.Fatalf("unexpected open windows: %d", summary.OpenWindows)
	}
	if summary.AverageEntryScore != 1.25 || summary.AverageExitScore != 0.5 {
		t.Fatalf("unexpected averages: entry=%v exit=%v", summary.AverageEntryScore, summary.AverageExitScore)
	}
	if got := len(summary.Symbols); got != 1 || summary.Symbols[0] != "AAPL" {
		t.Fatalf("unexpected symbols: %#v", summary.Symbols)
	}
}

func TestIndicatorOrderReturnsCopy(t *testing.T) {
	order := IndicatorOrder()
	order[0] = "modified"

	if IndicatorOrder()[0] != "SMA" {
		t.Fatalf("indicator order should be immutable to callers")
	}
}

func TestBuildDailyAnalyticsExport(t *testing.T) {
	snapshots := []model.WindowSnapshot{
		{
			SessionID:  "session-1",
			Symbol:     "MSFT",
			Phase:      "entry",
			EntryScore: 0.8,
			ExitScore:  0.4,
			CapturedAt: time.Date(2026, 4, 21, 15, 0, 0, 0, time.UTC),
		},
		{
			SessionID:  "session-1",
			Symbol:     "AAPL",
			Phase:      "closed",
			EntryScore: 0.5,
			ExitScore:  0.7,
			CapturedAt: time.Date(2026, 4, 20, 16, 0, 0, 0, time.UTC),
		},
		{
			SessionID:  "session-1",
			Symbol:     "AAPL",
			Phase:      "entry",
			EntryScore: 1.0,
			ExitScore:  0.2,
			CapturedAt: time.Date(2026, 4, 20, 15, 30, 0, 0, time.UTC),
		},
	}

	export := BuildDailyAnalyticsExport("session-1", snapshots, time.Unix(100, 0).UTC())

	if export.Version != "daily.analytics.v1" {
		t.Fatalf("unexpected export version: %s", export.Version)
	}
	if export.ExportPath != "/v1/sessions/session-1/analytics/export" {
		t.Fatalf("unexpected export path: %s", export.ExportPath)
	}
	if got := len(export.SymbolSummaries); got != 2 {
		t.Fatalf("unexpected symbol summaries count: %d", got)
	}
	if got := len(export.MarketSummaries); got != 2 {
		t.Fatalf("unexpected market summaries count: %d", got)
	}
	if export.SymbolSummaries[0].Day != "2026-04-20" || export.SymbolSummaries[0].Symbol != "AAPL" {
		t.Fatalf("unexpected first symbol summary: %#v", export.SymbolSummaries[0])
	}
	if export.SymbolSummaries[0].LastPhase != "closed" {
		t.Fatalf("unexpected last phase for first symbol summary: %#v", export.SymbolSummaries[0])
	}
	if export.MarketSummaries[0].SnapshotCount != 2 || export.MarketSummaries[0].SymbolCount != 1 {
		t.Fatalf("unexpected first market summary: %#v", export.MarketSummaries[0])
	}
}
