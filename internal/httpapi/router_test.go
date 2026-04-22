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

	NewRouter(nil, nil, slog.Default()).ServeHTTP(rr, req)

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

	NewRouter(nil, nil, slog.Default()).ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rr.Code)
	}
}

func TestAcceptAndExitUpdateSessionStateFromLocalWindowState(t *testing.T) {
	st := store.NewMemoryStore()
	if err := st.UpsertSession(nil, model.SessionSummary{ID: "session-1", Status: "live"}); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	router := NewRouter(st, nil, slog.Default())

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

	windows, err := st.ListWindows(nil, "session-1")
	if err != nil {
		t.Fatalf("list windows: %v", err)
	}
	if len(windows) != 1 || windows[0].Status != "closed" || windows[0].ClosedAt == nil {
		t.Fatalf("expected closed trade window, got %#v", windows)
	}
}
