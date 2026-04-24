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

type ConfigField struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Value       any      `json:"value"`
	Description string   `json:"description"`
	Group       string   `json:"group"`
	InputType   string   `json:"input_type"`
	Step        *float64 `json:"step,omitempty"`
	Placeholder string   `json:"placeholder,omitempty"`
}

type ConfigVersion struct {
	ID        string        `json:"id"`
	SessionID string        `json:"session_id"`
	Version   string        `json:"version"`
	Status    string        `json:"status"`
	Summary   string        `json:"summary"`
	Fields    []ConfigField `json:"fields"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

type DecisionRecord struct {
	ID         string    `json:"id"`
	SessionID  string    `json:"session_id"`
	WindowID   string    `json:"window_id,omitempty"`
	Symbol     string    `json:"symbol"`
	Action     string    `json:"action"`
	Reason     string    `json:"reason"`
	EntryScore float64   `json:"entry_score"`
	ExitScore  float64   `json:"exit_score"`
	EventType  string    `json:"event_type"`
	CreatedAt  time.Time `json:"created_at"`
}

type SignalEvent struct {
	ID         string    `json:"id"`
	SessionID  string    `json:"session_id"`
	WindowID   string    `json:"window_id,omitempty"`
	Symbol     string    `json:"symbol"`
	State      string    `json:"state"`
	EntryScore float64   `json:"entry_score"`
	ExitScore  float64   `json:"exit_score"`
	Regime     string    `json:"regime"`
	Reasons    []string  `json:"reasons"`
	Timestamp  time.Time `json:"timestamp"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type MarketSnapshot struct {
	ID              string    `json:"id"`
	SessionID       string    `json:"session_id"`
	WindowID        string    `json:"window_id,omitempty"`
	Symbol          string    `json:"symbol"`
	Timestamp       time.Time `json:"timestamp"`
	Open            float64   `json:"open"`
	High            float64   `json:"high"`
	Low             float64   `json:"low"`
	Close           float64   `json:"close"`
	Volume          float64   `json:"volume"`
	SMAFast         float64   `json:"sma_fast"`
	SMASlow         float64   `json:"sma_slow"`
	EMAFast         float64   `json:"ema_fast"`
	EMASlow         float64   `json:"ema_slow"`
	VWAP            float64   `json:"vwap"`
	RSI             float64   `json:"rsi"`
	ATR             float64   `json:"atr"`
	PlusDI          float64   `json:"plus_di"`
	MinusDI         float64   `json:"minus_di"`
	ADX             float64   `json:"adx"`
	MACD            float64   `json:"macd"`
	MACDSignal      float64   `json:"macd_signal"`
	MACDHistogram   float64   `json:"macd_histogram"`
	StochasticK     float64   `json:"stochastic_k"`
	StochasticD     float64   `json:"stochastic_d"`
	EntryScore      float64   `json:"entry_score"`
	ExitScore       float64   `json:"exit_score"`
	EventType       string    `json:"event_type"`
	SignalAction    string    `json:"signal_action"`
	SignalState     string    `json:"signal_state"`
	SignalRegime    string    `json:"signal_regime"`
	BenchmarkSymbol string    `json:"benchmark_symbol,omitempty"`
	Reasons         []string  `json:"reasons"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type SessionSummary struct {
	ID                  string                     `json:"id"`
	Status              string                     `json:"status"`
	OpenWindows         int                        `json:"open_windows"`
	LastDecisionAt      time.Time                  `json:"last_decision_at"`
	Symbols             []string                   `json:"symbols"`
	ConfigVersion       string                     `json:"config_version"`
	OptimizationSummary *WindowOptimizationSummary `json:"optimization_summary,omitempty"`
	UpdatedAt           time.Time                  `json:"updated_at"`
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

type DailySymbolAnalyticsSummary struct {
	Day               string  `json:"day"`
	Symbol            string  `json:"symbol"`
	SnapshotCount     int     `json:"snapshot_count"`
	AverageEntryScore float64 `json:"average_entry_score"`
	AverageExitScore  float64 `json:"average_exit_score"`
	LastPhase         string  `json:"last_phase"`
}

type DailyMarketAnalyticsSummary struct {
	Day               string   `json:"day"`
	SnapshotCount     int      `json:"snapshot_count"`
	SymbolCount       int      `json:"symbol_count"`
	AverageEntryScore float64  `json:"average_entry_score"`
	AverageExitScore  float64  `json:"average_exit_score"`
	Symbols           []string `json:"symbols"`
}

type DailyAnalyticsExport struct {
	Version         string                        `json:"version"`
	SessionID       string                        `json:"session_id"`
	ExportPath      string                        `json:"export_path"`
	GeneratedAt     time.Time                     `json:"generated_at"`
	SymbolSummaries []DailySymbolAnalyticsSummary `json:"symbol_summaries"`
	MarketSummaries []DailyMarketAnalyticsSummary `json:"market_summaries"`
}

type WindowOptimization struct {
	ID            string         `json:"id"`
	SessionID     string         `json:"session_id"`
	WindowID      string         `json:"window_id"`
	Symbol        string         `json:"symbol"`
	Day           string         `json:"day"`
	EntrySnapshot MarketSnapshot `json:"entry_snapshot"`
	ExitSnapshot  MarketSnapshot `json:"exit_snapshot"`
	EntryScore    float64        `json:"entry_score"`
	ExitScore     float64        `json:"exit_score"`
	ChangePct     float64        `json:"change_pct"`
	Notes         string         `json:"notes"`
	RequestedBy   string         `json:"requested_by,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

type WindowOptimizationSummary struct {
	SessionID             string             `json:"session_id"`
	SampleCount           int                `json:"sample_count"`
	AverageChangePct      float64            `json:"average_change_pct"`
	AverageEntryScore     float64            `json:"average_entry_score"`
	AverageExitScore      float64            `json:"average_exit_score"`
	Symbols               []string           `json:"symbols"`
	EntryProfile          map[string]float64 `json:"entry_profile"`
	ExitProfile           map[string]float64 `json:"exit_profile"`
	OptimizerLearningRate float64            `json:"optimizer_learning_rate"`
	OptimizerBiasCap      float64            `json:"optimizer_bias_cap"`
	UpdatedAt             time.Time          `json:"updated_at"`
}
