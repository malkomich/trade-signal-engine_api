package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
	_ "time/tzdata"

	"trade-signal-engine-api/internal/analytics"
	"trade-signal-engine-api/internal/model"
	"trade-signal-engine-api/internal/notify"
	"trade-signal-engine-api/internal/rtdb"
	"trade-signal-engine-api/internal/store"
)

const (
	defaultOptimizerLearningRate = 0.12
	defaultOptimizerBiasCap      = 0.08
)

type Router struct {
	store                  store.Store
	notifier               notify.Publisher
	pushoverNotifier       notify.Publisher
	logger                 *slog.Logger
	defaultBenchmarkSymbol string
}

func NewRouter(st store.Store, notifier notify.Publisher, pushoverNotifier notify.Publisher, logger *slog.Logger, defaultBenchmarkSymbol string) http.Handler {
	r := &Router{store: st, notifier: notifier, pushoverNotifier: pushoverNotifier, logger: logger, defaultBenchmarkSymbol: defaultBenchmarkSymbol}
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
			"/v1/sessions/{id}/notifications/pushover",
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
		createdAt := time.Now().UTC()
		record := model.DecisionRecord{
			ID:         buildRTDBRecordID(payload.SessionID, payload.Symbol, payload.Action, createdAt),
			SessionID:  payload.SessionID,
			Symbol:     payload.Symbol,
			Action:     payload.Action,
			Reason:     payload.Reason,
			EntryScore: payload.EntryScore,
			ExitScore:  payload.ExitScore,
			SignalTier: payload.SignalTier,
			EventType:  payload.EventType,
			CreatedAt:  createdAt,
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
	if len(parts) == 3 && parts[1] == "notifications" && parts[2] == "pushover" && req.Method == http.MethodPost {
		r.sessionPushoverNotification(w, req, sessionID)
		return
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
			if payload.Timeframe == "" {
				payload.Timeframe = "1m"
			}
			payload.Timeframe = rtdb.SafeKeyPart(payload.Timeframe)
			createdAt := time.Now().UTC()
			if payload.ID == "" {
				// Retried snapshots keep the same ID so both realtime and memory backends upsert the record.
				payload.ID = buildRTDBMarketSnapshotID(payload.SessionID, payload.Symbol, payload.Timeframe, payload.Timestamp)
			} else {
				payload.ID = rtdb.SafeKeyPart(payload.ID)
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
			payload.UpdatedAt = createdAt
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

	createdAt := time.Now().UTC()
	record := model.DecisionRecord{
		ID:         buildRTDBRecordID(sessionID, payload.Symbol, action, createdAt),
		SessionID:  sessionID,
		Symbol:     payload.Symbol,
		Action:     action,
		Reason:     payload.Reason,
		EntryScore: payload.EntryScore,
		ExitScore:  payload.ExitScore,
		SignalTier: payload.SignalTier,
		CreatedAt:  createdAt,
	}
	switch action {
	case "accept":
		record.EventType = model.EventTypeDecisionAccepted
		record.WindowID = buildRTDBRecordID(sessionID, payload.Symbol, action, record.CreatedAt)
	case "exit":
		record.EventType = model.EventTypeDecisionExited
		if window := findOpenWindow(windows, payload.Symbol); window != nil {
			record.WindowID = rtdb.SafeKeyPart(window.ID)
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
			ID:              buildRTDBRecordID(sessionID, payload.Symbol, action, record.CreatedAt),
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

func (r *Router) sessionPushoverNotification(w http.ResponseWriter, req *http.Request, sessionID string) {
	if r.pushoverNotifier == nil {
		if r.logger != nil {
			r.logger.Warn("pushover notification request rejected", "session_id", sessionID, "reason", "notifier not configured")
		}
		writeError(w, http.StatusServiceUnavailable, "pushover notifier is not configured")
		return
	}
	var payload model.PushoverNotificationRequest
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
	if payload.Symbol == "" || payload.Action == "" {
		writeError(w, http.StatusBadRequest, "symbol and action are required")
		return
	}
	if payload.CreatedAt.IsZero() {
		payload.CreatedAt = time.Now().UTC()
	}
	title := buildNotificationTitle(payload.Action, payload.Symbol)
	if strings.TrimSpace(payload.Title) != "" && strings.TrimSpace(payload.Body) != "" && payload.Price == 0 && payload.EntryScore == 0 && payload.ExitScore == 0 && strings.TrimSpace(payload.SignalTier) == "" {
		title = strings.TrimSpace(payload.Title)
	}
	body := buildNotificationBody(payload)
	if strings.TrimSpace(body) == "" && strings.TrimSpace(payload.Body) != "" {
		// Structured fields take precedence; legacy title/body inputs remain supported for older callers.
		body = strings.TrimSpace(payload.Body)
	}
	event := notify.Event{
		Key:       payload.WindowID,
		SessionID: payload.SessionID,
		Symbol:    payload.Symbol,
		Type:      payload.EventType,
		Title:     title,
		Body:      body,
		CreatedAt: payload.CreatedAt,
	}
	if r.logger != nil {
		r.logger.Info(
			"pushover notification requested",
			"session_id", payload.SessionID,
			"symbol", payload.Symbol,
			"action", payload.Action,
			"price", payload.Price,
			"event_type", payload.EventType,
			"window_id", payload.WindowID,
		)
	}
	if err := r.pushoverNotifier.Publish(req.Context(), event); err != nil {
		if r.logger != nil {
			r.logger.Error(
				"pushover notification delivery failed",
				"session_id", payload.SessionID,
				"symbol", payload.Symbol,
				"action", payload.Action,
				"event_type", payload.EventType,
				"window_id", payload.WindowID,
				"error", err,
			)
		}
		writeError(w, http.StatusBadGateway, "failed to deliver pushover notification")
		return
	}
	if r.logger != nil {
		r.logger.Info(
			"pushover notification delivered",
			"session_id", payload.SessionID,
			"symbol", payload.Symbol,
			"action", payload.Action,
			"price", payload.Price,
			"event_type", payload.EventType,
			"window_id", payload.WindowID,
		)
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"status":     "delivered",
		"session_id": sessionID,
		"symbol":     payload.Symbol,
		"action":     payload.Action,
		"event_type": payload.EventType,
	})
}

func buildNotificationTitle(action string, symbol string) string {
	side := notificationActionSide(action)
	if side == "" {
		side = "SIGNAL"
	}
	normalizedSymbol := strings.ToUpper(strings.TrimSpace(symbol))
	if normalizedSymbol == "" {
		return side
	}
	return fmt.Sprintf("%s (%s)", side, normalizedSymbol)
}

func buildNotificationBody(payload model.PushoverNotificationRequest) string {
	if strings.TrimSpace(payload.Body) != "" && payload.Price == 0 && payload.EntryScore == 0 && payload.ExitScore == 0 && strings.TrimSpace(payload.SignalTier) == "" {
		return strings.TrimSpace(payload.Body)
	}
	lines := []string{formatNotificationPrice(payload.Price)}
	side := notificationActionSide(payload.Action)
	if side == "BUY" {
		tier := formatSignalTier(payload.SignalTier)
		if tier == "" {
			tier = "Buy"
		}
		lines = append(lines, fmt.Sprintf("Type: %s", tier))
		lines = append(lines, fmt.Sprintf("Conviction: %.0f%%", payload.EntryScore*100))
	} else {
		lines = append(lines, fmt.Sprintf("Conviction: %.0f%%", payload.ExitScore*100))
	}
	lines = append(lines, fmt.Sprintf("New York Time: %s", formatNotificationTimestamp(payload.CreatedAt)))
	return strings.Join(lines, "\n")
}

func formatNotificationPrice(price float64) string {
	if price <= 0 {
		return "Price: n/a"
	}
	return fmt.Sprintf("Price: %.2f", price)
}

func notificationActionSide(action string) string {
	normalized := strings.ToUpper(strings.TrimSpace(action))
	switch {
	case strings.HasPrefix(normalized, "BUY"):
		return "BUY"
	case strings.HasPrefix(normalized, "SELL"):
		return "SELL"
	default:
		return normalized
	}
}

func formatSignalTier(tier string) string {
	normalized := strings.TrimSpace(tier)
	if normalized == "" {
		return ""
	}
	switch strings.ToLower(normalized) {
	case "conviction_buy":
		return "High Conviction Buy"
	case "balanced_buy":
		return "Medium Conviction Buy"
	case "opportunistic_buy":
		return "Lower Conviction Buy"
	case "speculative_buy":
		return "Very Low Conviction Buy"
	default:
		parts := strings.Fields(strings.ReplaceAll(strings.ToLower(normalized), "_", " "))
		for index, part := range parts {
			if part == "" {
				continue
			}
			parts[index] = strings.ToUpper(part[:1]) + part[1:]
		}
		return strings.Join(parts, " ")
	}
}

func formatNotificationTimestamp(createdAt time.Time) string {
	location := loadNotificationTimeZone()
	if createdAt.IsZero() {
		return time.Now().In(location).Format("15:04:05")
	}
	return createdAt.In(location).Format("15:04:05")
}

func loadNotificationTimeZone() *time.Location {
	timezoneName := strings.TrimSpace(os.Getenv("PUSHOVER_NOTIFICATION_TIMEZONE"))
	if timezoneName == "" {
		timezoneName = "America/New_York"
	}
	location, err := time.LoadLocation(timezoneName)
	if err != nil {
		return time.UTC
	}
	return location
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

func buildRTDBRecordID(sessionID string, symbol string, action string, timestamp time.Time) string {
	parts := []string{rtdb.SafeKeyPart(sessionID), rtdb.SafeKeyPart(symbol)}
	if trimmedAction := strings.TrimSpace(action); trimmedAction != "" {
		parts = append(parts, rtdb.SafeKeyPart(trimmedAction))
	}
	parts = append(parts, rtdb.SafeTimestampKey(timestamp))
	return strings.Join(parts, ":")
}

func buildRTDBMarketSnapshotID(sessionID string, symbol string, timeframe string, timestamp time.Time) string {
	parts := []string{rtdb.SafeKeyPart(sessionID), rtdb.SafeKeyPart(symbol), rtdb.SafeKeyPart(timeframe)}
	parts = append(parts, rtdb.SafeTimestampKey(timestamp))
	return strings.Join(parts, ":")
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
	if strings.EqualFold(decision.Action, "HOLD") {
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
	if r.logger != nil {
		r.logger.Info(
			"decision notification requested",
			"session_id", decision.SessionID,
			"symbol", decision.Symbol,
			"action", decision.Action,
			"event_type", decision.EventType,
			"window_id", decision.WindowID,
		)
	}
	if err := r.notifier.Publish(ctx, event); err != nil {
		if r.logger != nil {
			r.logger.Error(
				"decision notification delivery failed",
				"session_id", decision.SessionID,
				"symbol", decision.Symbol,
				"action", decision.Action,
				"event_type", decision.EventType,
				"window_id", decision.WindowID,
				"error", err,
			)
		}
		return
	}
	if r.logger != nil {
		r.logger.Info(
			"decision notification delivered",
			"session_id", decision.SessionID,
			"symbol", decision.Symbol,
			"action", decision.Action,
			"event_type", decision.EventType,
			"window_id", decision.WindowID,
		)
	}
}

func (r *Router) persistSignalEvent(ctx context.Context, decision model.DecisionRecord) error {
	if strings.EqualFold(decision.Action, "HOLD") {
		return nil
	}
	event := model.SignalEvent{
		ID:         decision.ID,
		SessionID:  decision.SessionID,
		WindowID:   decision.WindowID,
		Symbol:     decision.Symbol,
		Action:     decision.Action,
		SignalTier: decision.SignalTier,
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
		"close":                snapshot.Close,
		"sma_fast":             snapshot.SMAFast,
		"sma_slow":             snapshot.SMASlow,
		"ema_fast":             snapshot.EMAFast,
		"ema_slow":             snapshot.EMASlow,
		"vwap":                 snapshot.VWAP,
		"rsi":                  snapshot.RSI,
		"rsi_delta":            snapshot.RSIDelta,
		"atr":                  snapshot.ATR,
		"plus_di":              snapshot.PlusDI,
		"minus_di":             snapshot.MinusDI,
		"adx":                  snapshot.ADX,
		"macd":                 snapshot.MACD,
		"macd_signal":          snapshot.MACDSignal,
		"macd_histogram":       snapshot.MACDHistogram,
		"macd_histogram_delta": snapshot.MACDHistogramDelta,
		"stochastic_k":         snapshot.StochasticK,
		"stochastic_d":         snapshot.StochasticD,
		"stochastic_k_delta":   snapshot.StochasticKDelta,
		"stochastic_d_delta":   snapshot.StochasticDDelta,
		"bollinger_middle":     snapshot.BollingerMiddle,
		"bollinger_upper":      snapshot.BollingerUpper,
		"bollinger_lower":      snapshot.BollingerLower,
		"obv":                  snapshot.OBV,
		"relative_volume":      snapshot.RelativeVolume,
		"volume_profile":       snapshot.VolumeProfile,
		"entry_score":          snapshot.EntryScore,
		"exit_score":           snapshot.ExitScore,
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
