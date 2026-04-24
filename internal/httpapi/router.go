package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"trade-signal-engine-api/internal/analytics"
	"trade-signal-engine-api/internal/model"
	"trade-signal-engine-api/internal/notify"
	"trade-signal-engine-api/internal/store"
)

const (
	defaultOptimizerLearningRate = 0.12
	defaultOptimizerBiasCap      = 0.08
)

type Router struct {
	store                  store.Store
	notifier               notify.Publisher
	logger                 *slog.Logger
	defaultBenchmarkSymbol string
}

func NewRouter(st store.Store, notifier notify.Publisher, logger *slog.Logger, defaultBenchmarkSymbol string) http.Handler {
	r := &Router{store: st, notifier: notifier, logger: logger, defaultBenchmarkSymbol: defaultBenchmarkSymbol}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", r.root)
	mux.HandleFunc("/healthz", r.healthz)
	mux.HandleFunc("/readyz", r.readyz)
	mux.HandleFunc("/v1/decisions", r.decisions)
	mux.HandleFunc("/v1/sessions/", r.sessions)
	return mux
}

func (r *Router) root(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"service": "trade-signal-engine-api",
		"status":  "ok",
		"routes": []string{
			"/healthz",
			"/readyz",
			"/v1/decisions",
			"/v1/sessions/{id}",
			"/v1/sessions/{id}/windows",
			"/v1/sessions/{id}/config",
			"/v1/sessions/{id}/market-snapshots",
			"/v1/sessions/{id}/analytics",
			"/v1/sessions/{id}/analytics/export",
			"/v1/sessions/{id}/accept",
			"/v1/sessions/{id}/exit",
			"/v1/sessions/{id}/reject",
			"/v1/sessions/{id}/ack",
		},
	})
}

func (r *Router) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *Router) readyz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (r *Router) decisions(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodPost:
		var payload model.DecisionRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json payload")
			return
		}
		if payload.SessionID == "" || payload.Symbol == "" || payload.Action == "" {
			writeError(w, http.StatusBadRequest, "session_id, symbol and action are required")
			return
		}
		record := model.DecisionRecord{
			ID:         time.Now().UTC().Format("20060102T150405.000000000Z"),
			SessionID:  payload.SessionID,
			Symbol:     payload.Symbol,
			Action:     payload.Action,
			Reason:     payload.Reason,
			EntryScore: payload.EntryScore,
			ExitScore:  payload.ExitScore,
			EventType:  payload.EventType,
			CreatedAt:  time.Now().UTC(),
		}
		if record.EventType == "" {
			record.EventType = model.EventTypeDecisionCreated
		}
		if err := r.store.SaveDecision(req.Context(), record); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save decision")
			return
		}
		if err := r.persistAnalytics(req.Context(), record, nil); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save analytics snapshot")
			return
		}
		if err := r.persistSignalEvent(req.Context(), record); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save signal event")
			return
		}
		writeJSON(w, http.StatusCreated, record)
	case http.MethodGet:
		sessionID := req.URL.Query().Get("session_id")
		if sessionID == "" {
			writeError(w, http.StatusBadRequest, "session_id query parameter is required")
			return
		}
		items, err := r.store.ListDecisions(req.Context(), sessionID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load decisions")
			return
		}
		writeJSON(w, http.StatusOK, items)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (r *Router) sessions(w http.ResponseWriter, req *http.Request) {
	path := strings.TrimPrefix(req.URL.Path, "/v1/sessions/")
	parts := strings.Split(path, "/")
	sessionID := parts[0]
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session id is required")
		return
	}
	if len(parts) == 2 && req.Method == http.MethodPost {
		switch parts[1] {
		case "accept", "exit", "reject", "ack":
			r.sessionAction(w, req, sessionID, parts[1])
			return
		}
	}
	if len(parts) == 2 && parts[1] == "windows" && req.Method == http.MethodGet {
		windows, err := r.store.ListWindows(req.Context(), sessionID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load windows")
			return
		}
		writeJSON(w, http.StatusOK, windows)
		return
	}
	if len(parts) == 2 && parts[1] == "config" && req.Method == http.MethodGet {
		session, err := r.store.GetSession(req.Context(), sessionID)
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "session not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load session")
			return
		}
		versions, err := r.store.ListConfigVersions(req.Context(), sessionID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load config versions")
			return
		}
		selected := selectSessionConfigVersion(session.ConfigVersion, versions)
		optimizationSummary := session.OptimizationSummary
		if optimizationSummary == nil {
			optimizations, err := r.store.ListWindowOptimizations(req.Context(), sessionID)
			if err != nil {
				slog.Warn("failed to load optimization history", "session", sessionID, "error", err)
			} else {
				optimizationSummary = buildWindowOptimizationSummary(sessionID, optimizations)
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"session_id":             sessionID,
			"session_config_version": session.ConfigVersion,
			"selected_version":       selected,
			"versions":               versions,
			"optimization_summary":   optimizationSummary,
		})
		return
	}
	if len(parts) == 2 && parts[1] == "market-snapshots" {
		switch req.Method {
		case http.MethodGet:
			snapshots, err := r.store.ListMarketSnapshots(req.Context(), sessionID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to load market snapshots")
				return
			}
			writeJSON(w, http.StatusOK, snapshots)
			return
		case http.MethodPost:
			var payload model.MarketSnapshot
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				writeError(w, http.StatusBadRequest, "invalid json payload")
				return
			}
			if payload.SessionID == "" {
				payload.SessionID = sessionID
			}
			if payload.SessionID != sessionID {
				writeError(w, http.StatusBadRequest, "session_id does not match path")
				return
			}
			if payload.Symbol == "" {
				writeError(w, http.StatusBadRequest, "symbol is required")
				return
			}
			if payload.Timestamp.IsZero() {
				writeError(w, http.StatusBadRequest, "timestamp is required")
				return
			}
			if payload.ID == "" {
				// Retried snapshots keep the same ID so both realtime and memory backends upsert the record.
				payload.ID = payload.SessionID + ":" + payload.Symbol + ":" + payload.Timestamp.UTC().Format(time.RFC3339Nano)
			}
			if payload.EventType == "" {
				payload.EventType = "market.snapshot"
			}
			if payload.SignalAction == "" {
				payload.SignalAction = "HOLD"
			}
			if payload.SignalState == "" {
				payload.SignalState = "FLAT"
			}
			if payload.SignalRegime == "" {
				payload.SignalRegime = "Live market session"
			}
			if payload.BenchmarkSymbol == "" {
				payload.BenchmarkSymbol = r.defaultBenchmarkSymbol
			}
			if payload.BenchmarkSymbol == "" {
				payload.BenchmarkSymbol = "IXIC"
			}
			if payload.CreatedAt.IsZero() {
				payload.CreatedAt = payload.Timestamp
			}
			payload.UpdatedAt = time.Now().UTC()
			if err := r.store.SaveMarketSnapshot(req.Context(), payload); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to save market snapshot")
				return
			}
			writeJSON(w, http.StatusCreated, payload)
			return
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
	}
	if len(parts) == 2 && parts[1] == "analytics" && req.Method == http.MethodGet {
		summary, snapshots, err := r.loadAnalytics(req.Context(), sessionID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load analytics")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"summary":   summary,
			"snapshots": snapshots,
		})
		return
	}
	if len(parts) == 3 && parts[1] == "analytics" && parts[2] == "export" && req.Method == http.MethodGet {
		snapshots, err := r.loadSnapshots(req.Context(), sessionID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load analytics export")
			return
		}
		export := analytics.BuildDailyAnalyticsExport(sessionID, snapshots, time.Now().UTC())
		writeJSON(w, http.StatusOK, export)
		return
	}
	switch req.Method {
	case http.MethodGet:
		session, err := r.store.GetSession(req.Context(), sessionID)
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "session not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load session")
			return
		}
		writeJSON(w, http.StatusOK, session)
	case http.MethodPut:
		var payload model.SessionSummary
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json payload")
			return
		}
		payload.ID = sessionID
		if err := r.store.UpsertSession(req.Context(), payload); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save session")
			return
		}
		writeJSON(w, http.StatusOK, payload)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (r *Router) sessionAction(w http.ResponseWriter, req *http.Request, sessionID string, action string) {
	var payload model.DecisionRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json payload")
		return
	}
	if payload.Symbol == "" {
		writeError(w, http.StatusBadRequest, "symbol is required")
		return
	}
	windows, err := r.store.ListWindows(req.Context(), sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load windows")
		return
	}

	record := model.DecisionRecord{
		ID:         time.Now().UTC().Format("20060102T150405.000000000Z"),
		SessionID:  sessionID,
		Symbol:     payload.Symbol,
		Action:     action,
		Reason:     payload.Reason,
		EntryScore: payload.EntryScore,
		ExitScore:  payload.ExitScore,
		CreatedAt:  time.Now().UTC(),
	}
	switch action {
	case "accept":
		record.EventType = model.EventTypeDecisionAccepted
		record.WindowID = sessionID + ":" + payload.Symbol + ":" + record.ID
	case "exit":
		record.EventType = model.EventTypeDecisionExited
		if window := findOpenWindow(windows, payload.Symbol); window != nil {
			record.WindowID = window.ID
		}
	case "reject":
		record.EventType = model.EventTypeDecisionRejected
	case "ack":
		record.EventType = model.EventTypeDecisionAcknowledged
	default:
		writeError(w, http.StatusBadRequest, "unsupported session action")
		return
	}
	if action == "exit" && !hasOpenWindow(windows, payload.Symbol) {
		writeError(w, http.StatusNotFound, "open trade window not found")
		return
	}

	if err := r.store.SaveDecision(req.Context(), record); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save decision")
		return
	}
	session, err := r.store.GetSession(req.Context(), sessionID)
	if err != nil && err != store.ErrNotFound {
		writeError(w, http.StatusInternalServerError, "failed to load session")
		return
	}
	session.ID = sessionID
	session.LastDecisionAt = record.CreatedAt
	session.UpdatedAt = record.CreatedAt
	session.Symbols = appendUniqueSymbol(session.Symbols, payload.Symbol)
	switch action {
	case "accept":
		window := model.TradeWindow{
			ID:              sessionID + ":" + payload.Symbol + ":" + record.ID,
			SessionID:       sessionID,
			Symbol:          payload.Symbol,
			Status:          "open",
			EntryDecisionID: record.ID,
			OpenedAt:        record.CreatedAt,
			EntryScore:      payload.EntryScore,
			ExitScore:       payload.ExitScore,
		}
		if err := r.store.SaveWindow(req.Context(), window); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save trade window")
			return
		}
		record.WindowID = window.ID
		windows = append(windows, window)
		if err := r.persistAnalytics(req.Context(), record, &window); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save analytics snapshot")
			return
		}
	case "exit":
		window, updatedWindows, err := r.closeOpenWindow(req.Context(), windows, sessionID, payload.Symbol, record)
		if err != nil {
			if err == store.ErrNotFound {
				writeError(w, http.StatusNotFound, "open trade window not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to close trade window")
			return
		}
		record.WindowID = window.ID
		windows = updatedWindows
		if err := r.persistAnalytics(req.Context(), record, window); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save analytics snapshot")
			return
		}
	default:
		if err := r.persistAnalytics(req.Context(), record, nil); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save analytics snapshot")
			return
		}
	}
	if err := r.persistSignalEvent(req.Context(), record); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save signal event")
		return
	}
	session.OpenWindows = countOpenWindows(windows)
	switch action {
	case "accept":
		session.Status = "open"
	case "exit":
		if session.OpenWindows > 0 {
			session.Status = "open"
		} else {
			session.Status = "closed"
		}
	case "reject":
		session.Status = "rejected"
	case "ack":
		session.Status = "acknowledged"
	}
	if err := r.store.UpsertSession(req.Context(), session); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update session")
		return
	}
	r.publishNotification(req.Context(), record)
	writeJSON(w, http.StatusCreated, record)
}

func appendUniqueSymbol(symbols []string, symbol string) []string {
	if symbol == "" {
		return symbols
	}
	for _, existing := range symbols {
		if strings.EqualFold(existing, symbol) {
			return symbols
		}
	}
	return append(symbols, symbol)
}

func selectSessionConfigVersion(sessionVersion string, versions []model.ConfigVersion) *model.ConfigVersion {
	if len(versions) == 0 {
		return nil
	}
	if sessionVersion != "" {
		for index := range versions {
			if versions[index].Version == sessionVersion || versions[index].ID == sessionVersion {
				return &versions[index]
			}
		}
	}
	for index := len(versions) - 1; index >= 0; index-- {
		if versions[index].Status == "active" {
			return &versions[index]
		}
	}
	return &versions[len(versions)-1]
}

func findOpenWindow(windows []model.TradeWindow, symbol string) *model.TradeWindow {
	for index := range windows {
		if windows[index].Symbol == symbol && windows[index].Status == "open" {
			return &windows[index]
		}
	}
	return nil
}

func (r *Router) closeOpenWindow(ctx context.Context, windows []model.TradeWindow, sessionID string, symbol string, record model.DecisionRecord) (*model.TradeWindow, []model.TradeWindow, error) {
	for index := range windows {
		if windows[index].Symbol != symbol || windows[index].Status != "open" {
			continue
		}
		windowToClose := windows[index]
		closedAt := record.CreatedAt
		windowToClose.Status = "closed"
		windowToClose.ExitDecisionID = record.ID
		windowToClose.ClosedAt = &closedAt
		windowToClose.ExitScore = record.ExitScore
		windowToClose.UpdatedAt = record.CreatedAt
		if err := r.store.SaveWindow(ctx, windowToClose); err != nil {
			return nil, nil, err
		}
		windows[index] = windowToClose
		return &windows[index], windows, nil
	}
	return nil, windows, store.ErrNotFound
}

func hasOpenWindow(windows []model.TradeWindow, symbol string) bool {
	for _, window := range windows {
		if window.Symbol == symbol && window.Status == "open" {
			return true
		}
	}
	return false
}

func countOpenWindows(windows []model.TradeWindow) int {
	open := 0
	for _, window := range windows {
		if window.Status == "open" {
			open++
		}
	}
	return open
}

func (r *Router) persistAnalytics(ctx context.Context, decision model.DecisionRecord, window *model.TradeWindow) error {
	snapshot := analytics.SnapshotFromDecision(decision, window)
	if err := r.store.SaveWindowSnapshot(ctx, snapshot); err != nil {
		return err
	}
	return r.updateAnalyticsSummary(ctx, decision.SessionID, snapshot, window)
}

func (r *Router) loadAnalytics(ctx context.Context, sessionID string) (model.WindowAnalyticsSummary, []model.WindowSnapshot, error) {
	snapshots, err := r.loadSnapshots(ctx, sessionID)
	if err != nil {
		return model.WindowAnalyticsSummary{}, nil, err
	}
	summary, err := r.store.GetWindowSummary(ctx, sessionID)
	if err == store.ErrNotFound {
		windows, windowsErr := r.store.ListWindows(ctx, sessionID)
		if windowsErr != nil {
			return model.WindowAnalyticsSummary{}, nil, windowsErr
		}
		summary = analytics.BuildWindowSummary(sessionID, windows, snapshots, time.Now().UTC())
		return summary, snapshots, nil
	}
	if err != nil {
		return model.WindowAnalyticsSummary{}, nil, err
	}
	return summary, snapshots, nil
}

func (r *Router) loadSnapshots(ctx context.Context, sessionID string) ([]model.WindowSnapshot, error) {
	return r.store.ListWindowSnapshots(ctx, sessionID)
}

func (r *Router) updateAnalyticsSummary(ctx context.Context, sessionID string, snapshot model.WindowSnapshot, window *model.TradeWindow) error {
	summary, err := r.store.GetWindowSummary(ctx, sessionID)
	if err != nil && err != store.ErrNotFound {
		return err
	}
	summary = analytics.UpdateWindowSummary(summary, snapshot, window, time.Now().UTC())
	return r.store.UpsertWindowSummary(ctx, summary)
}

func (r *Router) publishNotification(ctx context.Context, decision model.DecisionRecord) {
	if r.notifier == nil {
		return
	}
	event := notify.Event{
		SessionID: decision.SessionID,
		Symbol:    decision.Symbol,
		Type:      decision.EventType,
		Title:     strings.ToUpper(decision.Action) + " signal",
		Body:      decision.Reason,
		CreatedAt: decision.CreatedAt,
	}
	if decision.WindowID != "" {
		event.Key = decision.WindowID
	}
	_ = r.notifier.Publish(ctx, event)
}

func (r *Router) persistSignalEvent(ctx context.Context, decision model.DecisionRecord) error {
	event := model.SignalEvent{
		ID:         decision.ID,
		SessionID:  decision.SessionID,
		WindowID:   decision.WindowID,
		Symbol:     decision.Symbol,
		State:      signalStateForDecision(decision),
		EntryScore: decision.EntryScore,
		ExitScore:  decision.ExitScore,
		Regime:     signalRegimeForDecision(decision),
		Reasons:    signalReasonsForDecision(decision),
		Timestamp:  decision.CreatedAt,
		UpdatedAt:  decision.CreatedAt,
	}
	return r.store.SaveSignalEvent(ctx, event)
}

func signalStateForDecision(decision model.DecisionRecord) string {
	switch decision.EventType {
	case model.EventTypeDecisionAccepted:
		return "ACCEPTED_OPEN"
	case model.EventTypeDecisionExited:
		return "EXIT_SIGNALLED"
	case model.EventTypeDecisionRejected:
		return "REJECTED"
	case model.EventTypeDecisionAcknowledged:
		return "CLOSED"
	case model.EventTypeDecisionCreated:
		switch {
		case strings.EqualFold(decision.Action, "buy_alert"):
			return "ENTRY_SIGNALLED"
		case strings.EqualFold(decision.Action, "sell_alert"):
			return "EXIT_SIGNALLED"
		default:
			return "FLAT"
		}
	default:
		if strings.EqualFold(decision.Action, "buy_alert") {
			return "ENTRY_SIGNALLED"
		}
		return "FLAT"
	}
}

func signalRegimeForDecision(decision model.DecisionRecord) string {
	if decision.Reason != "" {
		return decision.Reason
	}
	return strings.ToUpper(strings.ReplaceAll(decision.Action, "_", " "))
}

func signalReasonsForDecision(decision model.DecisionRecord) []string {
	reason := strings.TrimSpace(decision.Reason)
	if reason == "" {
		return nil
	}
	parts := strings.Split(reason, ";")
	reasons := make([]string, 0, len(parts))
	for _, part := range parts {
		candidate := strings.TrimSpace(part)
		if candidate != "" {
			reasons = append(reasons, candidate)
		}
	}
	if len(reasons) == 0 {
		return nil
	}
	return reasons
}

func buildWindowOptimizationSummary(sessionID string, optimizations []model.WindowOptimization) *model.WindowOptimizationSummary {
	summary := model.WindowOptimizationSummary{
		SessionID:             sessionID,
		EntryProfile:          make(map[string]float64),
		ExitProfile:           make(map[string]float64),
		OptimizerLearningRate: defaultOptimizerLearningRate,
		OptimizerBiasCap:      defaultOptimizerBiasCap,
	}
	if len(optimizations) == 0 {
		return &summary
	}

	entryTotals := make(map[string]float64)
	entryCounts := make(map[string]int)
	exitTotals := make(map[string]float64)
	exitCounts := make(map[string]int)
	symbols := make(map[string]struct{})
	var totalChange float64
	var totalEntry float64
	var totalExit float64
	for _, optimization := range optimizations {
		summary.SampleCount++
		totalChange += optimization.ChangePct
		totalEntry += optimization.EntryScore
		totalExit += optimization.ExitScore
		summary.UpdatedAt = optimization.UpdatedAt
		if optimization.Symbol != "" {
			symbols[optimization.Symbol] = struct{}{}
		}
		accumulateOptimizationProfile(entryTotals, entryCounts, optimization.EntrySnapshot)
		accumulateOptimizationProfile(exitTotals, exitCounts, optimization.ExitSnapshot)
	}
	summary.AverageChangePct = totalChange / float64(summary.SampleCount)
	summary.AverageEntryScore = totalEntry / float64(summary.SampleCount)
	summary.AverageExitScore = totalExit / float64(summary.SampleCount)
	summary.EntryProfile = finalizeOptimizationProfile(entryTotals, entryCounts)
	summary.ExitProfile = finalizeOptimizationProfile(exitTotals, exitCounts)
	for symbol := range symbols {
		summary.Symbols = append(summary.Symbols, symbol)
	}
	sort.Strings(summary.Symbols)
	return &summary
}

func accumulateOptimizationProfile(totals map[string]float64, counts map[string]int, snapshot model.MarketSnapshot) {
	profile := optimizationSnapshotProfile(snapshot)
	for key, value := range profile {
		totals[key] += value
		counts[key]++
	}
}

func finalizeOptimizationProfile(totals map[string]float64, counts map[string]int) map[string]float64 {
	profile := make(map[string]float64, len(totals))
	for key, total := range totals {
		if count := counts[key]; count > 0 {
			profile[key] = total / float64(count)
		}
	}
	return profile
}

func optimizationSnapshotProfile(snapshot model.MarketSnapshot) map[string]float64 {
	return map[string]float64{
		"close":          snapshot.Close,
		"sma_fast":       snapshot.SMAFast,
		"sma_slow":       snapshot.SMASlow,
		"ema_fast":       snapshot.EMAFast,
		"ema_slow":       snapshot.EMASlow,
		"vwap":           snapshot.VWAP,
		"rsi":            snapshot.RSI,
		"atr":            snapshot.ATR,
		"plus_di":        snapshot.PlusDI,
		"minus_di":       snapshot.MinusDI,
		"adx":            snapshot.ADX,
		"macd":           snapshot.MACD,
		"macd_signal":    snapshot.MACDSignal,
		"macd_histogram": snapshot.MACDHistogram,
		"stochastic_k":   snapshot.StochasticK,
		"stochastic_d":   snapshot.StochasticD,
		"entry_score":    snapshot.EntryScore,
		"exit_score":     snapshot.ExitScore,
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
