package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
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
