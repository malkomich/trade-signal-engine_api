package notify

import (
	"context"
	"testing"
	"time"
)

type recordingPublisher struct {
	events []Event
}

func (p *recordingPublisher) Publish(_ context.Context, event Event) error {
	p.events = append(p.events, event)
	return nil
}

func TestCollapsingPublisherSuppressesDuplicatesWithinTTL(t *testing.T) {
	downstream := &recordingPublisher{}
	publisher := NewCollapsingPublisher(downstream, time.Minute)
	publisher.now = func() time.Time { return time.Unix(100, 0) }

	event := Event{SessionID: "s1", Symbol: "AAPL", Type: "signal", CreatedAt: time.Unix(100, 0)}
	if err := publisher.Publish(context.Background(), event); err != nil {
		t.Fatalf("first publish failed: %v", err)
	}
	if err := publisher.Publish(context.Background(), event); err != nil {
		t.Fatalf("duplicate publish failed: %v", err)
	}

	if got := len(downstream.events); got != 1 {
		t.Fatalf("expected 1 published event, got %d", got)
	}
}

func TestCollapsingPublisherAllowsEventsAfterTTL(t *testing.T) {
	downstream := &recordingPublisher{}
	publisher := NewCollapsingPublisher(downstream, time.Minute)
	publisher.now = func() time.Time { return time.Unix(100, 0) }

	event := Event{SessionID: "s1", Symbol: "AAPL", Type: "signal"}
	if err := publisher.Publish(context.Background(), event); err != nil {
		t.Fatalf("first publish failed: %v", err)
	}
	publisher.now = func() time.Time { return time.Unix(200, 0) }
	if err := publisher.Publish(context.Background(), event); err != nil {
		t.Fatalf("second publish failed: %v", err)
	}

	if got := len(downstream.events); got != 2 {
		t.Fatalf("expected 2 published events, got %d", got)
	}
}
