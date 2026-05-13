package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"trade-signal-engine-api/internal/alpaca"
	"trade-signal-engine-api/internal/model"
	"trade-signal-engine-api/internal/notify"
	"trade-signal-engine-api/internal/store"
	"trade-signal-engine-api/internal/trading"
)

var rtdbSafeKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_:\-]+$`)

func assertRTDBSafeKey(t *testing.T, label, key string) {
	t.Helper()
	if !rtdbSafeKeyPattern.MatchString(key) {
		t.Fatalf("expected RTDB-safe %s, got %q", label, key)
	}
}

type configTestStore struct {
	*store.MemoryStore
	versions []model.ConfigVersion
	session  model.SessionSummary
}

type recordingNotifyPublisher struct {
	event notify.Event
	calls int
}

func (p *recordingNotifyPublisher) Publish(_ context.Context, event notify.Event) error {
	p.calls++
	p.event = event
	return nil
}

func (s *configTestStore) ListConfigVersions(_ context.Context, _ string) ([]model.ConfigVersion, error) {
	return append([]model.ConfigVersion(nil), s.versions...), nil
}

func (s *configTestStore) GetSession(ctx context.Context, sessionID string) (model.SessionSummary, error) {
	if s.session.ID != "" {
		return s.session, nil
	}
	return s.MemoryStore.GetSession(ctx, sessionID)
}

func TestRootEndpointReturnsServiceMetadata(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	NewRouter(nil, nil, nil, slog.Default(), "IXIC", nil).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json body: %v", err)
	}
	if got := payload["service"]; got != "trade-signal-engine-api" {
		t.Fatalf("expected service name trade-signal-engine-api, got %v", got)
	}
	if got := payload["status"]; got != "ok" {
		t.Fatalf("expected status ok, got %v", got)
	}
	routes, ok := payload["routes"].([]any)
	if !ok || len(routes) == 0 {
		t.Fatalf("expected routes array in payload, got %T %#v", payload["routes"], payload["routes"])
	}
	if !containsRoute(routes, "/v1/sessions/{id}/notifications/pushover") {
		t.Fatalf("expected pushover route in payload, got %#v", routes)
	}
}

func containsRoute(routes []any, target string) bool {
	for _, route := range routes {
		if route == target {
			return true
		}
	}
	return false
}

func TestSessionConfigEndpointReturnsSelectedVersion(t *testing.T) {
	st := &configTestStore{
		MemoryStore: store.NewMemoryStore(),
		versions: []model.ConfigVersion{
			{
				ID:        "session-1:v18",
				SessionID: "session-1",
				Version:   "v18",
				Status:    "archived",
				Summary:   "previous config",
				UpdatedAt: time.Date(2026, 4, 20, 15, 45, 0, 0, time.UTC),
			},
			{
				ID:        "session-1:v19",
				SessionID: "session-1",
				Version:   "v19",
				Status:    "active",
				Summary:   "current config",
				UpdatedAt: time.Date(2026, 4, 21, 15, 45, 0, 0, time.UTC),
			},
		},
	}
	if err := st.UpsertSession(context.Background(), model.SessionSummary{ID: "session-1", ConfigVersion: "v19"}); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/session-1/config", nil)
	rr := httptest.NewRecorder()

	NewRouter(st, nil, nil, slog.Default(), "IXIC", []string{"https://admin.example.test"}).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json body: %v", err)
	}
	if got := payload["session_config_version"]; got != "v19" {
		t.Fatalf("expected session config version v19, got %v", got)
	}
	selected, ok := payload["selected_version"].(map[string]any)
	if !ok {
		t.Fatalf("expected selected_version object, got %#v", payload["selected_version"])
	}
	if got := selected["version"]; got != "v19" {
		t.Fatalf("expected selected version v19, got %v", got)
	}
}

func TestSessionPushoverNotificationEndpointPublishesNotification(t *testing.T) {
	t.Setenv("PUSHOVER_NOTIFICATION_TIMEZONE", "UTC")
	st := store.NewMemoryStore()
	pushover := &recordingNotifyPublisher{}
	router := NewRouter(st, nil, pushover, slog.Default(), "IXIC", nil)
	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/sessions/session-1/notifications/pushover",
		strings.NewReader(`{"session_id":"session-1","symbol":"NVDA","action":"BUY_ALERT","reason":"entry-qualified; trend:aligned","price":206.5,"entry_score":0.82,"exit_score":0.18,"signal_tier":"balanced_buy","event_type":"signal.emitted","window_id":"window-1","created_at":"2026-04-24T13:30:00Z"}`),
	)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rr.Code)
	}
	if pushover.calls != 1 {
		t.Fatalf("expected one pushover publish call, got %d", pushover.calls)
	}
	if pushover.event.SessionID != "session-1" {
		t.Fatalf("expected session-1 event, got %#v", pushover.event)
	}
	if pushover.event.Symbol != "NVDA" {
		t.Fatalf("expected NVDA event, got %#v", pushover.event)
	}
	if pushover.event.Title != "BUY (NVDA)" {
		t.Fatalf("expected BUY (NVDA) title, got %#v", pushover.event.Title)
	}
	if pushover.event.Body != "Price: 206.50\nType: Medium Conviction Buy\nConviction: 82%\nNew York Time: 13:30:00" {
		t.Fatalf("expected simplified body, got %#v", pushover.event.Body)
	}
}

func TestSessionPushoverNotificationEndpointPublishesSellNotificationWithPriceFallback(t *testing.T) {
	t.Setenv("PUSHOVER_NOTIFICATION_TIMEZONE", "UTC")
	st := store.NewMemoryStore()
	pushover := &recordingNotifyPublisher{}
	router := NewRouter(st, nil, pushover, slog.Default(), "IXIC", nil)
	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/sessions/session-1/notifications/pushover",
		strings.NewReader(`{"session_id":"session-1","symbol":"","action":"SELL","reason":"exit-qualified","entry_score":0.18,"exit_score":0.87,"signal_tier":"","event_type":"signal.emitted","window_id":"window-1","created_at":"2026-04-24T13:30:00Z"}`),
	)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rr.Code)
	}
	if pushover.calls != 1 {
		t.Fatalf("expected one pushover publish call, got %d", pushover.calls)
	}
	if pushover.event.Title != "SELL" {
		t.Fatalf("expected SELL title, got %#v", pushover.event.Title)
	}
	if pushover.event.Body != "Price: n/a\nConviction: 87%\nNew York Time: 13:30:00" {
		t.Fatalf("expected sell body with price fallback, got %#v", pushover.event.Body)
	}
}

func TestSessionPushoverNotificationEndpointFallsBackToLegacyTitleAndBody(t *testing.T) {
	t.Setenv("PUSHOVER_NOTIFICATION_TIMEZONE", "UTC")
	st := store.NewMemoryStore()
	pushover := &recordingNotifyPublisher{}
	router := NewRouter(st, nil, pushover, slog.Default(), "IXIC", nil)
	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/sessions/session-1/notifications/pushover",
		strings.NewReader(`{"session_id":"session-1","symbol":"NVDA","action":"BUY_ALERT","title":"Legacy Title","body":"Legacy Body","reason":"entry-qualified","entry_score":0,"exit_score":0,"event_type":"signal.emitted","window_id":"window-1","created_at":"2026-04-24T13:30:00Z"}`),
	)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rr.Code)
	}
	if pushover.event.Title != "Legacy Title" {
		t.Fatalf("expected legacy title to be preserved, got %#v", pushover.event.Title)
	}
	if pushover.event.Body != "Legacy Body" {
		t.Fatalf("expected legacy body to be preserved, got %#v", pushover.event.Body)
	}
}

func TestNotificationFormattingCoversEdgeCases(t *testing.T) {
	t.Setenv("PUSHOVER_NOTIFICATION_TIMEZONE", "UTC")

	if got := buildNotificationTitle("", ""); got != "SIGNAL" {
		t.Fatalf("expected SIGNAL fallback title, got %q", got)
	}
	if got := buildNotificationTitle("sell_alert", "nvda"); got != "SELL (NVDA)" {
		t.Fatalf("expected SELL title, got %q", got)
	}

	body := buildNotificationBody(model.PushoverNotificationRequest{
		Action:     "HOLD",
		ExitScore:  0.87,
		CreatedAt:  time.Date(2026, 4, 24, 13, 30, 0, 0, time.UTC),
		Price:      0,
		SignalTier: "",
	})
	if body != "Price: n/a\nConviction: 87%\nNew York Time: 13:30:00" {
		t.Fatalf("expected stable fallback body, got %#v", body)
	}
}

func TestSessionConfigEndpointIncludesDefaultOptimizationSummaryWithoutHistory(t *testing.T) {
	st := &configTestStore{
		MemoryStore: store.NewMemoryStore(),
		session:     model.SessionSummary{ID: "session-1", ConfigVersion: "v19"},
		versions: []model.ConfigVersion{
			{
				ID:        "session-1:v19",
				SessionID: "session-1",
				Version:   "v19",
				Status:    "active",
				Summary:   "current config",
				UpdatedAt: time.Date(2026, 4, 21, 15, 45, 0, 0, time.UTC),
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/session-1/config", nil)
	rr := httptest.NewRecorder()

	NewRouter(st, nil, nil, slog.Default(), "IXIC", nil).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json body: %v", err)
	}
	summary, ok := payload["optimization_summary"].(map[string]any)
	if !ok {
		t.Fatalf("expected optimization_summary object, got %#v", payload["optimization_summary"])
	}
	if got := summary["optimizer_learning_rate"]; got != defaultOptimizerLearningRate {
		t.Fatalf("expected default optimizer learning rate %v, got %v", defaultOptimizerLearningRate, got)
	}
	if got := summary["optimizer_bias_cap"]; got != defaultOptimizerBiasCap {
		t.Fatalf("expected default optimizer bias cap %v, got %v", defaultOptimizerBiasCap, got)
	}
	if got := summary["sample_count"]; got != float64(0) {
		t.Fatalf("expected zero sample count, got %v", got)
	}
}

func TestSelectSessionConfigVersionPrefersMostRecentActiveVersion(t *testing.T) {
	versions := []model.ConfigVersion{
		{ID: "session-1:v18", Version: "v18", Status: "active", UpdatedAt: time.Date(2026, 4, 20, 15, 45, 0, 0, time.UTC)},
		{ID: "session-1:v19", Version: "v19", Status: "archived", UpdatedAt: time.Date(2026, 4, 21, 15, 45, 0, 0, time.UTC)},
		{ID: "session-1:v20", Version: "v20", Status: "active", UpdatedAt: time.Date(2026, 4, 22, 15, 45, 0, 0, time.UTC)},
	}

	selected := selectSessionConfigVersion("", versions)
	if selected == nil {
		t.Fatalf("expected selected version, got nil")
	}
	if selected.Version != "v20" {
		t.Fatalf("expected latest active version v20, got %q", selected.Version)
	}
}

func TestSelectSessionConfigVersionFallsBackToLatestWhenNoActiveVersionExists(t *testing.T) {
	versions := []model.ConfigVersion{
		{ID: "session-1:v18", Version: "v18", Status: "archived", UpdatedAt: time.Date(2026, 4, 20, 15, 45, 0, 0, time.UTC)},
		{ID: "session-1:v19", Version: "v19", Status: "candidate", UpdatedAt: time.Date(2026, 4, 21, 15, 45, 0, 0, time.UTC)},
		{ID: "session-1:v20", Version: "v20", Status: "archived", UpdatedAt: time.Date(2026, 4, 22, 15, 45, 0, 0, time.UTC)},
	}

	selected := selectSessionConfigVersion("", versions)
	if selected == nil {
		t.Fatalf("expected selected version, got nil")
	}
	if selected.Version != "v20" {
		t.Fatalf("expected latest version v20, got %q", selected.Version)
	}
}

func TestSelectSessionConfigVersionReturnsNilForEmptyVersions(t *testing.T) {
	selected := selectSessionConfigVersion("", nil)
	if selected != nil {
		t.Fatalf("expected nil selected version, got %#v", selected)
	}
}

func TestUnknownPathReturnsNotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rr := httptest.NewRecorder()

	NewRouter(nil, nil, nil, slog.Default(), "IXIC", nil).ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rr.Code)
	}
}

func TestAcceptAndExitUpdateSessionStateFromLocalWindowState(t *testing.T) {
	st := store.NewMemoryStore()
	if err := st.UpsertSession(nil, model.SessionSummary{ID: "session-1", Status: "live"}); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	router := NewRouter(st, nil, nil, slog.Default(), "IXIC", nil)

	acceptReq := httptest.NewRequest(http.MethodPost, "/v1/sessions/session-1/accept", strings.NewReader(`{"symbol":"NVDA","entry_score":0.82,"exit_score":0.17}`))
	acceptRR := httptest.NewRecorder()
	router.ServeHTTP(acceptRR, acceptReq)
	if acceptRR.Code != http.StatusCreated {
		t.Fatalf("expected accept status 201, got %d", acceptRR.Code)
	}
	var accepted model.DecisionRecord
	if err := json.Unmarshal(acceptRR.Body.Bytes(), &accepted); err != nil {
		t.Fatalf("expected valid accept json body: %v", err)
	}
	assertRTDBSafeKey(t, "decision id", accepted.ID)

	session, err := st.GetSession(nil, "session-1")
	if err != nil {
		t.Fatalf("load session after accept: %v", err)
	}
	if session.Status != "open" {
		t.Fatalf("expected session status open after accept, got %q", session.Status)
	}
	if session.OpenWindows != 1 {
		t.Fatalf("expected one open window after accept, got %d", session.OpenWindows)
	}
	if session.UpdatedAt.IsZero() {
		t.Fatalf("expected session updated_at to be set after accept")
	}
	if len(session.Symbols) != 1 || session.Symbols[0] != "NVDA" {
		t.Fatalf("expected session symbols to include NVDA, got %#v", session.Symbols)
	}

	exitReq := httptest.NewRequest(http.MethodPost, "/v1/sessions/session-1/exit", strings.NewReader(`{"symbol":"NVDA","entry_score":0.82,"exit_score":0.79}`))
	exitRR := httptest.NewRecorder()
	router.ServeHTTP(exitRR, exitReq)
	if exitRR.Code != http.StatusCreated {
		t.Fatalf("expected exit status 201, got %d", exitRR.Code)
	}

	session, err = st.GetSession(nil, "session-1")
	if err != nil {
		t.Fatalf("load session after exit: %v", err)
	}
	if session.Status != "closed" {
		t.Fatalf("expected session status closed after exit, got %q", session.Status)
	}
	if session.OpenWindows != 0 {
		t.Fatalf("expected zero open windows after exit, got %d", session.OpenWindows)
	}
	if session.UpdatedAt.IsZero() {
		t.Fatalf("expected session updated_at to be set after exit")
	}

	windows, err := st.ListWindows(nil, "session-1")
	if err != nil {
		t.Fatalf("list windows: %v", err)
	}
	if len(windows) != 1 || windows[0].Status != "closed" || windows[0].ClosedAt == nil {
		t.Fatalf("expected closed trade window, got %#v", windows)
	}
	assertRTDBSafeKey(t, "window id", windows[0].ID)
}

func TestAcceptAndMarketSnapshotSanitizeRTDBUnsafeSymbols(t *testing.T) {
	st := store.NewMemoryStore()
	if err := st.UpsertSession(nil, model.SessionSummary{ID: "session-1", Status: "live"}); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	router := NewRouter(st, nil, nil, slog.Default(), "IXIC", nil)

	acceptReq := httptest.NewRequest(http.MethodPost, "/v1/sessions/session-1/accept", strings.NewReader(`{"symbol":"BRK.B","entry_score":0.82,"exit_score":0.17}`))
	acceptRR := httptest.NewRecorder()
	router.ServeHTTP(acceptRR, acceptReq)
	if acceptRR.Code != http.StatusCreated {
		t.Fatalf("expected accept status 201, got %d", acceptRR.Code)
	}
	var accepted model.DecisionRecord
	if err := json.Unmarshal(acceptRR.Body.Bytes(), &accepted); err != nil {
		t.Fatalf("expected valid accept json body: %v", err)
	}
	assertRTDBSafeKey(t, "decision id", accepted.ID)
	assertRTDBSafeKey(t, "window id", accepted.WindowID)

	windows, err := st.ListWindows(nil, "session-1")
	if err != nil {
		t.Fatalf("list windows: %v", err)
	}
	if len(windows) != 1 {
		t.Fatalf("expected one stored window, got %#v", windows)
	}
	assertRTDBSafeKey(t, "persisted window id", windows[0].ID)

	postReq := httptest.NewRequest(http.MethodPost, "/v1/sessions/session-1/market-snapshots", strings.NewReader(`{"symbol":"BRK.B","session_id":"session-1","timeframe":"5m","timestamp":"2024-04-22T13:30:00Z","close":123.45,"event_type":"market.snapshot"}`))
	postRR := httptest.NewRecorder()
	router.ServeHTTP(postRR, postReq)
	if postRR.Code != http.StatusCreated {
		t.Fatalf("expected market snapshot status 201, got %d: %s", postRR.Code, postRR.Body.String())
	}
	var createdSnapshot model.MarketSnapshot
	if err := json.Unmarshal(postRR.Body.Bytes(), &createdSnapshot); err != nil {
		t.Fatalf("expected valid market snapshot json body: %v", err)
	}
	assertRTDBSafeKey(t, "market snapshot id", createdSnapshot.ID)
}

func TestExitWithoutOpenWindowReturnsNotFound(t *testing.T) {
	st := store.NewMemoryStore()
	if err := st.UpsertSession(nil, model.SessionSummary{ID: "session-1", Status: "live"}); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	router := NewRouter(st, nil, nil, slog.Default(), "IXIC", nil)
	exitReq := httptest.NewRequest(http.MethodPost, "/v1/sessions/session-1/exit", strings.NewReader(`{"symbol":"NVDA","entry_score":0.82,"exit_score":0.79}`))
	exitRR := httptest.NewRecorder()
	router.ServeHTTP(exitRR, exitReq)

	if exitRR.Code != http.StatusNotFound {
		t.Fatalf("expected exit status 404 without open window, got %d", exitRR.Code)
	}
	decisions, err := st.ListDecisions(nil, "session-1")
	if err != nil {
		t.Fatalf("list decisions: %v", err)
	}
	if len(decisions) != 0 {
		t.Fatalf("expected no persisted decisions when exit precondition fails, got %#v", decisions)
	}
}

func TestMarketSnapshotsRoundTrip(t *testing.T) {
	st := store.NewMemoryStore()
	router := NewRouter(st, nil, nil, slog.Default(), "QQQ", nil)

	postReq := httptest.NewRequest(http.MethodPost, "/v1/sessions/session-1/market-snapshots", strings.NewReader(`{"symbol":"BRK.B","session_id":"session-1","timestamp":"2024-04-22T13:30:00Z","timeframe":"5m","close":123.45,"event_type":"market.snapshot"}`))
	postRR := httptest.NewRecorder()
	router.ServeHTTP(postRR, postReq)
	if postRR.Code != http.StatusCreated {
		t.Fatalf("expected market snapshot status 201, got %d: %s", postRR.Code, postRR.Body.String())
	}
	var createdSnapshot model.MarketSnapshot
	if err := json.Unmarshal(postRR.Body.Bytes(), &createdSnapshot); err != nil {
		t.Fatalf("expected valid market snapshot json body: %v", err)
	}
	assertRTDBSafeKey(t, "market snapshot id", createdSnapshot.ID)

	getReq := httptest.NewRequest(http.MethodGet, "/v1/sessions/session-1/market-snapshots", nil)
	getRR := httptest.NewRecorder()
	router.ServeHTTP(getRR, getReq)
	if getRR.Code != http.StatusOK {
		t.Fatalf("expected market snapshot status 200, got %d", getRR.Code)
	}

	var snapshots []model.MarketSnapshot
	if err := json.Unmarshal(getRR.Body.Bytes(), &snapshots); err != nil {
		t.Fatalf("expected valid json body: %v", err)
	}
	if len(snapshots) != 1 || snapshots[0].Symbol != "BRK.B" {
		t.Fatalf("expected persisted market snapshot, got %#v", snapshots)
	}
	if snapshots[0].Timeframe != "5m" {
		t.Fatalf("expected timeframe to round-trip, got %#v", snapshots[0].Timeframe)
	}
	if snapshots[0].BenchmarkSymbol != "QQQ" {
		t.Fatalf("expected configured benchmark symbol, got %#v", snapshots[0].BenchmarkSymbol)
	}
	assertRTDBSafeKey(t, "persisted market snapshot id", snapshots[0].ID)
	if snapshots[0].UpdatedAt.IsZero() || !snapshots[0].UpdatedAt.After(snapshots[0].Timestamp) {
		t.Fatalf("expected updated_at to be set from wall clock, got %#v", snapshots[0].UpdatedAt)
	}
}

func TestMarketSnapshotsRoundTripIncludesNewIndicators(t *testing.T) {
	st := store.NewMemoryStore()
	router := NewRouter(st, nil, nil, slog.Default(), "QQQ", nil)

	postReq := httptest.NewRequest(http.MethodPost, "/v1/sessions/session-1/market-snapshots", strings.NewReader(`{"symbol":"AAPL","session_id":"session-1","timestamp":"2024-04-22T13:30:00Z","close":123.45,"bollinger_middle":120.0,"bollinger_upper":124.0,"bollinger_lower":118.0,"obv":23456.0,"relative_volume":1.3,"volume_profile":0.21,"event_type":"market.snapshot"}`))
	postRR := httptest.NewRecorder()
	router.ServeHTTP(postRR, postReq)
	if postRR.Code != http.StatusCreated {
		t.Fatalf("expected market snapshot status 201, got %d: %s", postRR.Code, postRR.Body.String())
	}

	var createdSnapshot model.MarketSnapshot
	if err := json.Unmarshal(postRR.Body.Bytes(), &createdSnapshot); err != nil {
		t.Fatalf("expected valid market snapshot json body: %v", err)
	}
	assertRTDBSafeKey(t, "market snapshot id", createdSnapshot.ID)

	snapshots, err := st.ListMarketSnapshots(nil, "session-1")
	if err != nil {
		t.Fatalf("list snapshots: %v", err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("expected one persisted snapshot, got %#v", snapshots)
	}
	if snapshots[0].BollingerMiddle != 120.0 || snapshots[0].BollingerUpper != 124.0 || snapshots[0].BollingerLower != 118.0 {
		t.Fatalf("expected bollinger bands to round-trip, got %#v", snapshots[0])
	}
	if snapshots[0].OBV != 23456.0 || snapshots[0].RelativeVolume != 1.3 || snapshots[0].VolumeProfile != 0.21 {
		t.Fatalf("expected volume indicators to round-trip, got %#v", snapshots[0])
	}
}

func TestOptimizationSnapshotProfileIncludesNewIndicatorDeltas(t *testing.T) {
	profile := optimizationSnapshotProfile(model.MarketSnapshot{
		Close:              123.45,
		SMAFast:            120.1,
		SMASlow:            119.4,
		EMAFast:            121.2,
		EMASlow:            120.3,
		VWAP:               122.5,
		RSI:                57.2,
		RSIDelta:           1.4,
		ATR:                2.1,
		PlusDI:             28.0,
		MinusDI:            14.0,
		ADX:                26.0,
		MACD:               0.48,
		MACDSignal:         0.31,
		MACDHistogram:      0.17,
		MACDHistogramDelta: 0.04,
		StochasticK:        62.0,
		StochasticD:        55.0,
		StochasticKDelta:   3.2,
		StochasticDDelta:   1.1,
		BollingerMiddle:    121.0,
		BollingerUpper:     124.0,
		BollingerLower:     118.0,
		OBV:                34567.0,
		RelativeVolume:     1.25,
		VolumeProfile:      0.19,
		EntryScore:         0.72,
		ExitScore:          0.41,
	})

	if got := profile["rsi_delta"]; got != 1.4 {
		t.Fatalf("expected rsi_delta to be included, got %v", got)
	}
	if got := profile["macd_histogram_delta"]; got != 0.04 {
		t.Fatalf("expected macd_histogram_delta to be included, got %v", got)
	}
	if got := profile["stochastic_k_delta"]; got != 3.2 {
		t.Fatalf("expected stochastic_k_delta to be included, got %v", got)
	}
	if got := profile["stochastic_d_delta"]; got != 1.1 {
		t.Fatalf("expected stochastic_d_delta to be included, got %v", got)
	}
}

func TestDecisionsEndpointPersistsSignalTier(t *testing.T) {
	st := store.NewMemoryStore()
	router := NewRouter(st, nil, nil, slog.Default(), "QQQ", nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/decisions", strings.NewReader(`{"session_id":"session-1","symbol":"AAPL","action":"BUY_ALERT","reason":"entry-qualified","entry_score":0.72,"exit_score":0.31,"signal_tier":"balanced_buy"}`))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var record model.DecisionRecord
	if err := json.Unmarshal(rr.Body.Bytes(), &record); err != nil {
		t.Fatalf("expected valid json body: %v", err)
	}
	if record.SignalTier != "balanced_buy" {
		t.Fatalf("expected signal tier to persist, got %#v", record.SignalTier)
	}

	decisions, err := st.ListDecisions(nil, "session-1")
	if err != nil {
		t.Fatalf("list decisions: %v", err)
	}
	if len(decisions) != 1 {
		t.Fatalf("expected one stored decision, got %#v", decisions)
	}
	if decisions[0].SignalTier != "balanced_buy" {
		t.Fatalf("expected stored decision tier, got %#v", decisions[0].SignalTier)
	}
}

func TestMarketSnapshotsValidatePayloadAndSessionConsistency(t *testing.T) {
	st := store.NewMemoryStore()
	router := NewRouter(st, nil, nil, slog.Default(), "IXIC", nil)

	mismatchReq := httptest.NewRequest(http.MethodPost, "/v1/sessions/session-1/market-snapshots", strings.NewReader(`{"symbol":"NVDA","session_id":"session-2","timestamp":"2024-04-22T13:30:00Z"}`))
	mismatchRR := httptest.NewRecorder()
	router.ServeHTTP(mismatchRR, mismatchReq)
	if mismatchRR.Code != http.StatusBadRequest {
		t.Fatalf("expected mismatched session id to return 400, got %d", mismatchRR.Code)
	}

	missingSymbolReq := httptest.NewRequest(http.MethodPost, "/v1/sessions/session-1/market-snapshots", strings.NewReader(`{"session_id":"session-1","timestamp":"2024-04-22T13:30:00Z"}`))
	missingSymbolRR := httptest.NewRecorder()
	router.ServeHTTP(missingSymbolRR, missingSymbolReq)
	if missingSymbolRR.Code != http.StatusBadRequest {
		t.Fatalf("expected missing symbol to return 400, got %d", missingSymbolRR.Code)
	}

	missingTimestampReq := httptest.NewRequest(http.MethodPost, "/v1/sessions/session-1/market-snapshots", strings.NewReader(`{"symbol":"NVDA","session_id":"session-1"}`))
	missingTimestampRR := httptest.NewRecorder()
	router.ServeHTTP(missingTimestampRR, missingTimestampReq)
	if missingTimestampRR.Code != http.StatusBadRequest {
		t.Fatalf("expected missing timestamp to return 400, got %d", missingTimestampRR.Code)
	}

	unsupportedMethodReq := httptest.NewRequest(http.MethodPut, "/v1/sessions/session-1/market-snapshots", strings.NewReader(`{"symbol":"NVDA","session_id":"session-1","timestamp":"2024-04-22T13:30:00Z"}`))
	unsupportedMethodRR := httptest.NewRecorder()
	router.ServeHTTP(unsupportedMethodRR, unsupportedMethodReq)
	if unsupportedMethodRR.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected unsupported method to return 405, got %d", unsupportedMethodRR.Code)
	}
}

func TestMarketSnapshotsUpsertByID(t *testing.T) {
	st := store.NewMemoryStore()
	router := NewRouter(st, nil, nil, slog.Default(), "IXIC", nil)

	firstReq := httptest.NewRequest(http.MethodPost, "/v1/sessions/session-1/market-snapshots", strings.NewReader(`{"id":"snapshot-1","symbol":"NVDA","session_id":"session-1","timestamp":"2024-04-22T13:30:00Z","close":123.45}`))
	firstRR := httptest.NewRecorder()
	router.ServeHTTP(firstRR, firstReq)
	if firstRR.Code != http.StatusCreated {
		t.Fatalf("expected first snapshot to persist, got %d", firstRR.Code)
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/v1/sessions/session-1/market-snapshots", strings.NewReader(`{"id":"snapshot-1","symbol":"NVDA","session_id":"session-1","timestamp":"2024-04-22T13:31:00Z","close":125.00}`))
	secondRR := httptest.NewRecorder()
	router.ServeHTTP(secondRR, secondReq)
	if secondRR.Code != http.StatusCreated {
		t.Fatalf("expected second snapshot to persist, got %d", secondRR.Code)
	}

	snapshots, err := st.ListMarketSnapshots(nil, "session-1")
	if err != nil {
		t.Fatalf("list snapshots: %v", err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("expected upsert semantics for market snapshot IDs, got %#v", snapshots)
	}
	if got := snapshots[0].Close; got != 125.00 {
		t.Fatalf("expected latest snapshot to replace previous value, got %v", got)
	}
}

func TestSessionTradingEndpointReturnsDefaultsWithoutTradingService(t *testing.T) {
	st := store.NewMemoryStore()
	if err := st.UpsertSession(context.Background(), model.SessionSummary{ID: "session-1"}); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/session-1/trading", nil)
	rr := httptest.NewRecorder()

	NewRouter(st, nil, nil, slog.Default(), "IXIC", nil).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json body: %v", err)
	}
	if got := payload["trading_mode"]; got != "paper" {
		t.Fatalf("expected default paper mode, got %v", got)
	}
	if got := payload["trading_stop_loss_percent"]; got != 0.2 {
		t.Fatalf("expected default stop loss percent 0.2, got %v", got)
	}
	allocations, ok := payload["trading_allocations"].(map[string]any)
	if !ok {
		t.Fatalf("expected trading_allocations map, got %#v", payload["trading_allocations"])
	}
	if got := allocations["conviction_buy"]; got != float64(1000) {
		t.Fatalf("expected conviction_buy allocation 1000, got %v", got)
	}
	if payload["trading_account"] != nil {
		t.Fatalf("expected nil trading_account without trading service, got %#v", payload["trading_account"])
	}
}

func TestSessionTradingAccountEndpointReturnsSelectedModeSnapshot(t *testing.T) {
	st := store.NewMemoryStore()
	if err := st.UpsertSession(context.Background(), model.SessionSummary{ID: "session-1", TradingMode: "paper"}); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet || req.URL.Path != "/live/v2/account" {
			t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(alpaca.Account{
			Status:         "ACTIVE",
			BuyingPower:    "1234.56",
			Cash:           "987.65",
			Equity:         "2222.22",
			PortfolioValue: "3333.33",
		})
	}))
	t.Cleanup(server.Close)

	service := trading.NewService(alpaca.NewClient(
		"paper-key",
		"paper-secret",
		"live-key",
		"live-secret",
		server.URL+"/paper",
		server.URL+"/live",
		5*time.Second,
	))

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/session-1/trading/account?mode=live", nil)
	rr := httptest.NewRecorder()

	NewRouter(st, nil, nil, slog.Default(), "IXIC", nil, service).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json body: %v", err)
	}
	if got := payload["trading_mode"]; got != "live" {
		t.Fatalf("expected live trading mode, got %v", got)
	}
	account, ok := payload["trading_account"].(map[string]any)
	if !ok {
		t.Fatalf("expected trading_account map, got %#v", payload["trading_account"])
	}
	if got := account["status"]; got != "ACTIVE" {
		t.Fatalf("expected active account, got %v", got)
	}
	if got := account["buying_power"]; got != 1234.56 {
		t.Fatalf("expected buying_power 1234.56, got %v", got)
	}
}

func TestSessionTradingAccountEndpointRejectsInvalidMode(t *testing.T) {
	st := store.NewMemoryStore()
	if err := st.UpsertSession(context.Background(), model.SessionSummary{ID: "session-1", TradingMode: "paper"}); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/session-1/trading/account?mode=invalid", nil)
	rr := httptest.NewRecorder()

	NewRouter(st, nil, nil, slog.Default(), "IXIC", []string{"https://admin.example.test"}).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
}

func TestRouterAddsCORSHeadersForPreflightRequests(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/v1/sessions/session-1/trading", nil)
	req.Header.Set("Origin", "https://admin.example.test")
	req.Header.Set("Access-Control-Request-Method", http.MethodPut)
	rr := httptest.NewRecorder()

	NewRouter(store.NewMemoryStore(), nil, nil, slog.Default(), "IXIC", []string{"https://admin.example.test"}).ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", rr.Code)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://admin.example.test" {
		t.Fatalf("expected allow-origin header, got %q", got)
	}
	if got := rr.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Fatalf("expected allow-methods header")
	}
}

func TestRouterAllowsWildcardCORSOrigins(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/v1/sessions/session-1/trading", nil)
	req.Header.Set("Origin", "https://anything.example.test")
	req.Header.Set("Access-Control-Request-Method", http.MethodPut)
	rr := httptest.NewRecorder()

	NewRouter(store.NewMemoryStore(), nil, nil, slog.Default(), "IXIC", []string{"*"}, nil).ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", rr.Code)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://anything.example.test" {
		t.Fatalf("expected allow-origin header, got %q", got)
	}
}

func TestRouterRejectsDisallowedCORSPreflightRequests(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/v1/sessions/session-1/trading", nil)
	req.Header.Set("Origin", "https://malicious.example.test")
	req.Header.Set("Access-Control-Request-Method", http.MethodPut)
	rr := httptest.NewRecorder()

	NewRouter(store.NewMemoryStore(), nil, nil, slog.Default(), "IXIC", []string{"https://admin.example.test"}, nil).ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", rr.Code)
	}
}

func TestRouterRejectsDisallowedCORSRequestsForNonPreflightMethods(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/session-1/trading", strings.NewReader(`{}`))
	req.Header.Set("Origin", "https://malicious.example.test")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	NewRouter(store.NewMemoryStore(), nil, nil, slog.Default(), "IXIC", []string{"https://admin.example.test"}, nil).ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", rr.Code)
	}
}

func TestSessionTradingEndpointNormalizesAllocationKeys(t *testing.T) {
	st := store.NewMemoryStore()
	if err := st.UpsertSession(context.Background(), model.SessionSummary{ID: "session-1"}); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/v1/sessions/session-1/trading", strings.NewReader(`{"session_id":"session-1","mode":"paper","allocations":{"Conviction Buy":1500," balanced_buy ":1400,"OPPORTUNISTIC_BUY":1300,"speculative_buy":1200},"stop_loss_percent":0.2}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	NewRouter(st, nil, nil, slog.Default(), "IXIC", nil, nil).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json body: %v", err)
	}
	allocations, ok := payload["trading_allocations"].(map[string]any)
	if !ok {
		t.Fatalf("expected trading_allocations map, got %#v", payload["trading_allocations"])
	}
	if got := allocations["conviction_buy"]; got != float64(1500) {
		t.Fatalf("expected normalized conviction_buy allocation 1500, got %v", got)
	}
	if got := allocations["balanced_buy"]; got != float64(1400) {
		t.Fatalf("expected normalized balanced_buy allocation 1400, got %v", got)
	}
	if got := allocations["opportunistic_buy"]; got != float64(1300) {
		t.Fatalf("expected normalized opportunistic_buy allocation 1300, got %v", got)
	}
	if got := allocations["speculative_buy"]; got != float64(1200) {
		t.Fatalf("expected normalized speculative_buy allocation 1200, got %v", got)
	}
}

func TestSessionTradingExecuteRejectsUnsupportedAction(t *testing.T) {
	st := store.NewMemoryStore()
	if err := st.UpsertSession(context.Background(), model.SessionSummary{ID: "session-1"}); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	body := strings.NewReader(`{"session_id":"session-1","symbol":"NVDA","action":"HOLD"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/session-1/trading/execute", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	NewRouter(st, nil, nil, slog.Default(), "IXIC", nil, nil).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
}

func TestSessionTradingExecuteRejectsBuyWithoutPrice(t *testing.T) {
	st := store.NewMemoryStore()
	if err := st.UpsertSession(context.Background(), model.SessionSummary{ID: "session-1"}); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	body := strings.NewReader(`{"session_id":"session-1","symbol":"NVDA","action":"BUY_ALERT","price":0}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/session-1/trading/execute", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	NewRouter(st, nil, nil, slog.Default(), "IXIC", nil, nil).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
}
