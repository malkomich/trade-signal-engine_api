package notify

import (
	"context"
	"fmt"
	"strconv"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
)

type fcmSender interface {
	Send(context.Context, *messaging.Message) (string, error)
}

type FCMPublisher struct {
	sender fcmSender
	topic  string
}

func NewFCMPublisher(ctx context.Context, projectID string, topic string) (*FCMPublisher, error) {
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: projectID})
	if err != nil {
		return nil, err
	}
	client, err := app.Messaging(ctx)
	if err != nil {
		return nil, err
	}
	if topic == "" {
		topic = "trade-signal-engine"
	}
	return &FCMPublisher{sender: client, topic: topic}, nil
}

func (p *FCMPublisher) Publish(ctx context.Context, event Event) error {
	if event.Key == "" {
		event.Key = event.CollapseKey()
	}
	if event.Key == "" {
		return fmt.Errorf("notification event key is required")
	}
	message := &messaging.Message{
		Topic: p.topic,
		Notification: &messaging.Notification{
			Title: event.Title,
			Body:  event.Body,
		},
		Data: map[string]string{
			"event_key":    event.Key,
			"session_id":   event.SessionID,
			"symbol":       event.Symbol,
			"type":         event.Type,
			"title":        event.Title,
			"body":         event.Body,
			"created_at":   event.CreatedAt.UTC().Format(time.RFC3339Nano),
			"created_unix": strconv.FormatInt(event.CreatedAt.Unix(), 10),
			"created_msec": strconv.FormatInt(event.CreatedAt.UnixMilli(), 10),
		},
	}
	_, err := p.sender.Send(ctx, message)
	return err
}
