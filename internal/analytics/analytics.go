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

func BuildDailyAnalyticsExport(sessionID string, snapshots []model.WindowSnapshot, now time.Time) model.DailyAnalyticsExport {
	export := model.DailyAnalyticsExport{
		Version:     "daily.analytics.v1",
		SessionID:   sessionID,
		ExportPath:  fmt.Sprintf("/v1/sessions/%s/analytics/export", sessionID),
		GeneratedAt: now,
	}

	orderedSnapshots := append([]model.WindowSnapshot(nil), snapshots...)
	sort.SliceStable(orderedSnapshots, func(i, j int) bool {
		left := orderedSnapshots[i]
		right := orderedSnapshots[j]
		if !left.CapturedAt.Equal(right.CapturedAt) {
			return left.CapturedAt.Before(right.CapturedAt)
		}
		if left.Symbol != right.Symbol {
			return left.Symbol < right.Symbol
		}
		return left.ID < right.ID
	})

	type symbolAggregate struct {
		snapshotCount int
		entryTotal    float64
		exitTotal     float64
		lastPhase     string
	}

	type dayAggregate struct {
		symbols       map[string]*symbolAggregate
		snapshotCount int
		entryTotal    float64
		exitTotal     float64
	}

	days := make(map[string]*dayAggregate)
	for _, snapshot := range orderedSnapshots {
		day := snapshot.CapturedAt
		if day.IsZero() {
			day = now
		}
		dayKey := day.UTC().Format("2006-01-02")
		if _, ok := days[dayKey]; !ok {
			days[dayKey] = &dayAggregate{symbols: make(map[string]*symbolAggregate)}
		}
		daySummary := days[dayKey]
		daySummary.snapshotCount++
		daySummary.entryTotal += snapshot.EntryScore
		daySummary.exitTotal += snapshot.ExitScore

		symbolKey := snapshot.Symbol
		if symbolKey == "" {
			continue
		}
		symbolSummary, ok := daySummary.symbols[symbolKey]
		if !ok {
			symbolSummary = &symbolAggregate{}
			daySummary.symbols[symbolKey] = symbolSummary
		}
		symbolSummary.snapshotCount++
		symbolSummary.entryTotal += snapshot.EntryScore
		symbolSummary.exitTotal += snapshot.ExitScore
		symbolSummary.lastPhase = snapshot.Phase
	}

	dayKeys := make([]string, 0, len(days))
	for dayKey := range days {
		dayKeys = append(dayKeys, dayKey)
	}
	sort.Strings(dayKeys)
	for _, dayKey := range dayKeys {
		daySummary := days[dayKey]
		symbolKeys := make([]string, 0, len(daySummary.symbols))
		for symbolKey := range daySummary.symbols {
			symbolKeys = append(symbolKeys, symbolKey)
		}
		sort.Strings(symbolKeys)

		for _, symbolKey := range symbolKeys {
			summary := daySummary.symbols[symbolKey]
			export.SymbolSummaries = append(export.SymbolSummaries, model.DailySymbolAnalyticsSummary{
				Day:               dayKey,
				Symbol:            symbolKey,
				SnapshotCount:     summary.snapshotCount,
				AverageEntryScore: safeAverage(summary.entryTotal, summary.snapshotCount),
				AverageExitScore:  safeAverage(summary.exitTotal, summary.snapshotCount),
				LastPhase:         summary.lastPhase,
			})
		}

		export.MarketSummaries = append(export.MarketSummaries, model.DailyMarketAnalyticsSummary{
			Day:               dayKey,
			SnapshotCount:     daySummary.snapshotCount,
			SymbolCount:       len(daySummary.symbols),
			AverageEntryScore: safeAverage(daySummary.entryTotal, daySummary.snapshotCount),
			AverageExitScore:  safeAverage(daySummary.exitTotal, daySummary.snapshotCount),
			Symbols:           symbolKeys,
		})
	}

	return export
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

func safeAverage(total float64, count int) float64 {
	if count <= 0 {
		return 0
	}
	return total / float64(count)
}
