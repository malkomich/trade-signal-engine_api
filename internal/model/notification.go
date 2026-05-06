package model

import "time"

type PushoverNotificationRequest struct {
	SessionID   string    `json:"session_id"`
	Symbol      string    `json:"symbol"`
	Action      string    `json:"action"`
	Reason      string    `json:"reason"`
	Title       string    `json:"title,omitempty"`
	Body        string    `json:"body,omitempty"`
	EntryScore  float64   `json:"entry_score"`
	ExitScore   float64   `json:"exit_score"`
	SignalTier  string    `json:"signal_tier,omitempty"`
	EventType   string    `json:"event_type,omitempty"`
	WindowID    string    `json:"window_id,omitempty"`
	RequestedBy string    `json:"requested_by,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
}
