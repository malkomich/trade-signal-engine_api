package notify

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

type Event struct {
	Key       string
	SessionID string
	Symbol    string
	Type      string
	Title     string
	Body      string
	CreatedAt time.Time
}

type Publisher interface {
	Publish(context.Context, Event) error
}

type NoopPublisher struct{}

func (NoopPublisher) Publish(context.Context, Event) error {
	return nil
}

type CollapsingPublisher struct {
	mu         sync.Mutex
	ttl        time.Duration
	now        func() time.Time
	downstream Publisher
	lastSeen   map[string]time.Time
}

func NewCollapsingPublisher(downstream Publisher, ttl time.Duration) *CollapsingPublisher {
	if downstream == nil {
		downstream = NoopPublisher{}
	}
	return &CollapsingPublisher{
		ttl:        ttl,
		now:        time.Now,
		downstream: downstream,
		lastSeen:   make(map[string]time.Time),
	}
}

func (p *CollapsingPublisher) Publish(ctx context.Context, event Event) error {
	if event.Key == "" {
		event.Key = event.CollapseKey()
	}
	if event.Key == "" {
		return fmt.Errorf("notification event key is required")
	}

	p.mu.Lock()
	last, ok := p.lastSeen[event.Key]
	current := p.now()
	if ok && current.Sub(last) < p.ttl {
		p.mu.Unlock()
		return nil
	}
	p.lastSeen[event.Key] = current
	p.mu.Unlock()

	return p.downstream.Publish(ctx, event)
}

func (e Event) CollapseKey() string {
	parts := []string{e.SessionID, e.Symbol, e.Type}
	return strings.Join(parts, ":")
}
