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

	"trade-signal-engine-api/internal/model"
	"trade-signal-engine-api/internal/store"
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

	NewRouter(nil, nil, slog.Default(), "IXIC").ServeHTTP(rr, req)

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

	NewRouter(st, nil, slog.Default(), "IXIC").ServeHTTP(rr, req)

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

	NewRouter(st, nil, slog.Default(), "IXIC").ServeHTTP(rr, req)

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

	NewRouter(nil, nil, slog.Default(), "IXIC").ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rr.Code)
	}
}

func TestAcceptAndExitUpdateSessionStateFromLocalWindowState(t *testing.T) {
	st := store.NewMemoryStore()
	if err := st.UpsertSession(nil, model.SessionSummary{ID: "session-1", Status: "live"}); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	router := NewRouter(st, nil, slog.Default(), "IXIC")

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

	router := NewRouter(st, nil, slog.Default(), "IXIC")

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

	router := NewRouter(st, nil, slog.Default(), "IXIC")
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
	router := NewRouter(st, nil, slog.Default(), "QQQ")

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
	router := NewRouter(st, nil, slog.Default(), "QQQ")

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

func TestMarketSnapshotsValidatePayloadAndSessionConsistency(t *testing.T) {
	st := store.NewMemoryStore()
	router := NewRouter(st, nil, slog.Default(), "IXIC")

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
	router := NewRouter(st, nil, slog.Default(), "IXIC")

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
