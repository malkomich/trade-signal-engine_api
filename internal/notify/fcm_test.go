package notify

import (
	"context"
	"testing"
	"time"

	"firebase.google.com/go/v4/messaging"
)

type fakeSender struct {
	message *messaging.Message
}

func (s *fakeSender) Send(_ context.Context, message *messaging.Message) (string, error) {
	s.message = message
	return "message-id", nil
}

func TestFCMPublisherBuildsTopicMessage(t *testing.T) {
	sender := &fakeSender{}
	publisher := &FCMPublisher{sender: sender, topic: "trade-signal-engine"}

	event := Event{
		SessionID: "session-1",
		Symbol:    "NVDA",
		Type:      "decision.accepted",
		Title:     "BUY signal",
		Body:      "SMA stack aligned",
		CreatedAt: time.Unix(100, 0).UTC(),
	}

	if err := publisher.Publish(context.Background(), event); err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	if sender.message == nil {
		t.Fatalf("expected an fcm message to be sent")
	}
	if sender.message.Topic != "trade-signal-engine" {
		t.Fatalf("expected topic trade-signal-engine, got %q", sender.message.Topic)
	}
	if sender.message.Notification == nil || sender.message.Notification.Title != "BUY signal" {
		t.Fatalf("expected notification title BUY signal, got %#v", sender.message.Notification)
	}
	if got := sender.message.Data["symbol"]; got != "NVDA" {
		t.Fatalf("expected symbol NVDA, got %q", got)
	}
}
