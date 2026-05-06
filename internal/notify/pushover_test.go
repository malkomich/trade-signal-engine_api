package notify

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestPushoverPublisherPostsFormEncodedNotification(t *testing.T) {
	var received url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST request, got %s", r.Method)
		}
		if got := r.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
			t.Fatalf("expected form encoded content type, got %q", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		received, err = url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("parse body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	publisher := &PushoverPublisher{
		sender:          server.Client(),
		endpointURL:     server.URL,
		userKey:         "user-key",
		apiToken:        "api-token",
		device:          "malkphone",
		sound:           "trade-sound",
		applicationName: "trade-signal-engine",
	}

	event := Event{
		SessionID: "session-1",
		Symbol:    "NVDA",
		Type:      "signal.emitted",
		Title:     "BUY signal",
		Body:      "NVDA buy at 15:22",
		CreatedAt: time.Unix(100, 0).UTC(),
	}

	if err := publisher.Publish(context.Background(), event); err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	if got := received.Get("token"); got != "api-token" {
		t.Fatalf("expected token api-token, got %q", got)
	}
	if got := received.Get("user"); got != "user-key" {
		t.Fatalf("expected user user-key, got %q", got)
	}
	if got := received.Get("device"); got != "malkphone" {
		t.Fatalf("expected device malkphone, got %q", got)
	}
	if got := received.Get("sound"); got != "trade-sound" {
		t.Fatalf("expected sound trade-sound, got %q", got)
	}
	if got := received.Get("title"); got != "trade-signal-engine - BUY signal" {
		t.Fatalf("expected title to be prefixed with application name, got %q", got)
	}
	if got := received.Get("message"); got != "NVDA buy at 15:22" {
		t.Fatalf("expected message body forwarded, got %q", got)
	}
	if got := received.Get("timestamp"); got != "100" {
		t.Fatalf("expected timestamp 100, got %q", got)
	}
}

func TestNewPushoverPublisherRejectsMissingCredentials(t *testing.T) {
	if _, err := NewPushoverPublisher("", "token", "", "", "trade-signal-engine"); err == nil {
		t.Fatalf("expected missing user key to fail")
	}
	if _, err := NewPushoverPublisher("user", "", "", "", "trade-signal-engine"); err == nil {
		t.Fatalf("expected missing api token to fail")
	}
}
