package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"trade-signal-engine-api/internal/model"
	"trade-signal-engine-api/internal/store"
)

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

	postReq := httptest.NewRequest(http.MethodPost, "/v1/sessions/session-1/market-snapshots", strings.NewReader(`{"symbol":"NVDA","session_id":"session-1","timestamp":"2024-04-22T13:30:00Z","close":123.45,"event_type":"market.snapshot"}`))
	postRR := httptest.NewRecorder()
	router.ServeHTTP(postRR, postReq)
	if postRR.Code != http.StatusCreated {
		t.Fatalf("expected market snapshot status 201, got %d: %s", postRR.Code, postRR.Body.String())
	}

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
	if len(snapshots) != 1 || snapshots[0].Symbol != "NVDA" {
		t.Fatalf("expected persisted market snapshot, got %#v", snapshots)
	}
	if snapshots[0].BenchmarkSymbol != "QQQ" {
		t.Fatalf("expected configured benchmark symbol, got %#v", snapshots[0].BenchmarkSymbol)
	}
	if snapshots[0].UpdatedAt.IsZero() || !snapshots[0].UpdatedAt.After(snapshots[0].Timestamp) {
		t.Fatalf("expected updated_at to be set from wall clock, got %#v", snapshots[0].UpdatedAt)
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
