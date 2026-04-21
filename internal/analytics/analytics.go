package analytics

import (
	"fmt"
	"sort"
	"time"

	"trade-signal-engine-api/internal/model"
)

var indicatorOrder = []string{"SMA", "EMA", "VWAP", "RSI", "ATR", "DM", "MACD", "STOCH"}

func IndicatorOrder() []string {
	return append([]string(nil), indicatorOrder...)
}

func SnapshotFromDecision(decision model.DecisionRecord, window *model.TradeWindow) model.WindowSnapshot {
	snapshotID := fmt.Sprintf("%s:%s:%s", decision.SessionID, decision.ID, decision.EventType)
	if window != nil && window.ID != "" {
		snapshotID = fmt.Sprintf("%s:%s", snapshotID, window.ID)
	}
	snapshot := model.WindowSnapshot{
		ID:             snapshotID,
		SessionID:      decision.SessionID,
		Symbol:         decision.Symbol,
		Phase:          decision.EventType,
		EntryScore:     decision.EntryScore,
		ExitScore:      decision.ExitScore,
		IndicatorOrder: IndicatorOrder(),
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
		IndicatorOrder: IndicatorOrder(),
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

func UpdateWindowSummary(summary model.WindowAnalyticsSummary, snapshot model.WindowSnapshot, window *model.TradeWindow, now time.Time) model.WindowAnalyticsSummary {
	if summary.SessionID == "" {
		summary.SessionID = snapshot.SessionID
	}
	if len(summary.IndicatorOrder) == 0 {
		summary.IndicatorOrder = IndicatorOrder()
	}
	summary.SnapshotCount++
	summary.LastPhase = snapshot.Phase
	summary.AverageEntryScore = weightedAverage(summary.AverageEntryScore, summary.SnapshotCount-1, snapshot.EntryScore)
	summary.AverageExitScore = weightedAverage(summary.AverageExitScore, summary.SnapshotCount-1, snapshot.ExitScore)
	summary.UpdatedAt = now
	summary.Symbols = addSymbol(summary.Symbols, snapshot.Symbol)
	if window == nil {
		return summary
	}
	summary.Symbols = addSymbol(summary.Symbols, window.Symbol)
	switch window.Status {
	case "open":
		summary.OpenWindows++
	case "closed":
		summary.ClosedWindows++
	}
	return summary
}

func weightedAverage(previous float64, previousCount int, next float64) float64 {
	if previousCount <= 0 {
		return next
	}
	return ((previous * float64(previousCount)) + next) / float64(previousCount+1)
}

func addSymbol(symbols []string, symbol string) []string {
	if symbol == "" {
		return symbols
	}
	for _, existing := range symbols {
		if existing == symbol {
			return symbols
		}
	}
	symbols = append(symbols, symbol)
	sort.Strings(symbols)
	return symbols
}
