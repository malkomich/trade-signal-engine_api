package model

import "time"

type DecisionRequest struct {
	SessionID   string  `json:"session_id"`
	Symbol      string  `json:"symbol"`
	Action      string  `json:"action"`
	Reason      string  `json:"reason"`
	EntryScore  float64 `json:"entry_score"`
	ExitScore   float64 `json:"exit_score"`
	EventType   string  `json:"event_type,omitempty"`
	RequestedBy string  `json:"requested_by,omitempty"`
}

type DecisionRecord struct {
	ID         string    `json:"id"`
	SessionID  string    `json:"session_id"`
	Symbol     string    `json:"symbol"`
	Action     string    `json:"action"`
	Reason     string    `json:"reason"`
	EntryScore float64   `json:"entry_score"`
	ExitScore  float64   `json:"exit_score"`
	EventType  string    `json:"event_type"`
	CreatedAt  time.Time `json:"created_at"`
}

type SessionSummary struct {
	ID             string    `json:"id"`
	Status         string    `json:"status"`
	OpenWindows    int       `json:"open_windows"`
	LastDecisionAt time.Time `json:"last_decision_at"`
	Symbols        []string  `json:"symbols"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type TradeWindow struct {
	ID              string     `json:"id"`
	SessionID       string     `json:"session_id"`
	Symbol          string     `json:"symbol"`
	Status          string     `json:"status"`
	EntryDecisionID string     `json:"entry_decision_id,omitempty"`
	ExitDecisionID  string     `json:"exit_decision_id,omitempty"`
	OpenedAt        time.Time  `json:"opened_at"`
	ClosedAt        *time.Time `json:"closed_at,omitempty"`
	EntryScore      float64    `json:"entry_score"`
	ExitScore       float64    `json:"exit_score"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type WindowSnapshot struct {
	ID             string    `json:"id"`
	SessionID      string    `json:"session_id"`
	WindowID       string    `json:"window_id"`
	Symbol         string    `json:"symbol"`
	Phase          string    `json:"phase"`
	EntryScore     float64   `json:"entry_score"`
	ExitScore      float64   `json:"exit_score"`
	IndicatorOrder []string  `json:"indicator_order"`
	CapturedAt     time.Time `json:"captured_at"`
}

type WindowAnalyticsSummary struct {
	SessionID         string    `json:"session_id"`
	SnapshotCount     int       `json:"snapshot_count"`
	OpenWindows       int       `json:"open_windows"`
	ClosedWindows     int       `json:"closed_windows"`
	LastPhase         string    `json:"last_phase"`
	IndicatorOrder    []string  `json:"indicator_order"`
	AverageEntryScore float64   `json:"average_entry_score"`
	AverageExitScore  float64   `json:"average_exit_score"`
	Symbols           []string  `json:"symbols"`
	UpdatedAt         time.Time `json:"updated_at"`
}
