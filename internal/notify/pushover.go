package notify

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultPushoverEndpoint = "https://api.pushover.net/1/messages.json"

type pushoverSender interface {
	Do(*http.Request) (*http.Response, error)
}

type PushoverPublisher struct {
	sender          pushoverSender
	endpointURL     string
	userKey         string
	apiToken        string
	device          string
	sound           string
	applicationName string
}

func NewPushoverPublisher(userKey, apiToken, device, sound, applicationName string) (*PushoverPublisher, error) {
	if strings.TrimSpace(userKey) == "" {
		return nil, fmt.Errorf("pushover user key is required")
	}
	if strings.TrimSpace(apiToken) == "" {
		return nil, fmt.Errorf("pushover api token is required")
	}
	if strings.TrimSpace(applicationName) == "" {
		applicationName = "trade-signal-engine"
	}
	return &PushoverPublisher{
		sender:          &http.Client{Timeout: 10 * time.Second},
		endpointURL:     defaultPushoverEndpoint,
		userKey:         strings.TrimSpace(userKey),
		apiToken:        strings.TrimSpace(apiToken),
		device:          strings.TrimSpace(device),
		sound:           strings.TrimSpace(sound),
		applicationName: strings.TrimSpace(applicationName),
	}, nil
}

func (p *PushoverPublisher) Publish(ctx context.Context, event Event) error {
	if p == nil {
		return fmt.Errorf("pushover publisher is not configured")
	}
	if event.Key == "" {
		event.Key = event.CollapseKey()
	}
	if event.Key == "" {
		return fmt.Errorf("notification event key is required")
	}
	body := strings.TrimSpace(event.Body)
	if body == "" {
		return fmt.Errorf("pushover notification body is required")
	}
	title := strings.TrimSpace(event.Title)
	if title == "" {
		title = p.applicationName
	} else if p.applicationName != "" && !strings.Contains(strings.ToLower(title), strings.ToLower(p.applicationName)) {
		title = p.applicationName + " - " + title
	}
	values := url.Values{}
	values.Set("token", p.apiToken)
	values.Set("user", p.userKey)
	values.Set("message", body)
	if title != "" {
		values.Set("title", title)
	}
	if p.device != "" {
		values.Set("device", p.device)
	}
	if p.sound != "" {
		values.Set("sound", p.sound)
	}
	if !event.CreatedAt.IsZero() {
		values.Set("timestamp", strconv.FormatInt(event.CreatedAt.Unix(), 10))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpointURL, strings.NewReader(values.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := p.sender.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("pushover publish failed with status %d", resp.StatusCode)
	}
	return nil
}
