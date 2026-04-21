package analytics

import (
	"sort"
	"time"

	"trade-signal-engine-api/internal/model"
)

var IndicatorOrder = []string{"SMA", "EMA", "VWAP", "RSI", "ATR", "DM", "MACD", "STOCH"}

func SnapshotFromDecision(decision model.DecisionRecord, window *model.TradeWindow) model.WindowSnapshot {
	snapshot := model.WindowSnapshot{
		ID:             decision.SessionID + ":" + decision.ID,
		SessionID:      decision.SessionID,
		Symbol:         decision.Symbol,
		Phase:          decision.EventType,
		EntryScore:     decision.EntryScore,
		ExitScore:      decision.ExitScore,
		IndicatorOrder: append([]string(nil), IndicatorOrder...),
		CapturedAt:     decision.CreatedAt,
	}
	if window != nil {
		snapshot.WindowID = window.ID
		if window.Status != "" {
			snapshot.Phase = window.Status
		}
	}
	return snapshot
}

func BuildWindowSummary(sessionID string, windows []model.TradeWindow, snapshots []model.WindowSnapshot, now time.Time) model.WindowAnalyticsSummary {
	summary := model.WindowAnalyticsSummary{
		SessionID:      sessionID,
		SnapshotCount:  len(snapshots),
		IndicatorOrder: append([]string(nil), IndicatorOrder...),
		UpdatedAt:      now,
	}

	if len(snapshots) > 0 {
		summary.LastPhase = snapshots[len(snapshots)-1].Phase
	}

	symbolSet := make(map[string]struct{})
	for _, window := range windows {
		if window.Status == "open" {
			summary.OpenWindows++
		}
		if window.Status == "closed" {
			summary.ClosedWindows++
		}
		if window.Symbol != "" {
			symbolSet[window.Symbol] = struct{}{}
		}
		summary.AverageEntryScore += window.EntryScore
		summary.AverageExitScore += window.ExitScore
	}

	if len(windows) > 0 {
		summary.AverageEntryScore /= float64(len(windows))
		summary.AverageExitScore /= float64(len(windows))
	}

	for _, snapshot := range snapshots {
		if snapshot.Symbol != "" {
			symbolSet[snapshot.Symbol] = struct{}{}
		}
	}
	for symbol := range symbolSet {
		summary.Symbols = append(summary.Symbols, symbol)
	}
	sort.Strings(summary.Symbols)
	return summary
}
