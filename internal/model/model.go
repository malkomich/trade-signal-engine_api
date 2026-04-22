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
	ID         string    `json:"id" firestore:"id"`
	SessionID  string    `json:"session_id" firestore:"session_id"`
	Symbol     string    `json:"symbol" firestore:"symbol"`
	Action     string    `json:"action" firestore:"action"`
	Reason     string    `json:"reason" firestore:"reason"`
	EntryScore float64   `json:"entry_score" firestore:"entry_score"`
	ExitScore  float64   `json:"exit_score" firestore:"exit_score"`
	EventType  string    `json:"event_type" firestore:"event_type"`
	CreatedAt  time.Time `json:"created_at" firestore:"created_at"`
}

type SignalEvent struct {
	ID         string    `json:"id" firestore:"id"`
	SessionID  string    `json:"session_id" firestore:"session_id"`
	Symbol     string    `json:"symbol" firestore:"symbol"`
	State      string    `json:"state" firestore:"state"`
	EntryScore float64   `json:"entry_score" firestore:"entry_score"`
	ExitScore  float64   `json:"exit_score" firestore:"exit_score"`
	Regime     string    `json:"regime" firestore:"regime"`
	Reasons    []string  `json:"reasons" firestore:"reasons"`
	Timestamp  time.Time `json:"timestamp" firestore:"timestamp"`
	UpdatedAt  time.Time `json:"updated_at" firestore:"updated_at"`
}

type MarketSnapshot struct {
	ID              string    `json:"id" firestore:"id"`
	SessionID       string    `json:"session_id" firestore:"session_id"`
	Symbol          string    `json:"symbol" firestore:"symbol"`
	Timestamp       time.Time `json:"timestamp" firestore:"timestamp"`
	Open            float64   `json:"open" firestore:"open"`
	High            float64   `json:"high" firestore:"high"`
	Low             float64   `json:"low" firestore:"low"`
	Close           float64   `json:"close" firestore:"close"`
	Volume          float64   `json:"volume" firestore:"volume"`
	SMAFast         float64   `json:"sma_fast" firestore:"sma_fast"`
	SMASlow         float64   `json:"sma_slow" firestore:"sma_slow"`
	EMAFast         float64   `json:"ema_fast" firestore:"ema_fast"`
	EMASlow         float64   `json:"ema_slow" firestore:"ema_slow"`
	VWAP            float64   `json:"vwap" firestore:"vwap"`
	RSI             float64   `json:"rsi" firestore:"rsi"`
	ATR             float64   `json:"atr" firestore:"atr"`
	PlusDI          float64   `json:"plus_di" firestore:"plus_di"`
	MinusDI         float64   `json:"minus_di" firestore:"minus_di"`
	ADX             float64   `json:"adx" firestore:"adx"`
	MACD            float64   `json:"macd" firestore:"macd"`
	MACDSignal      float64   `json:"macd_signal" firestore:"macd_signal"`
	MACDHistogram   float64   `json:"macd_histogram" firestore:"macd_histogram"`
	StochasticK     float64   `json:"stochastic_k" firestore:"stochastic_k"`
	StochasticD     float64   `json:"stochastic_d" firestore:"stochastic_d"`
	EntryScore      float64   `json:"entry_score" firestore:"entry_score"`
	ExitScore       float64   `json:"exit_score" firestore:"exit_score"`
	EventType       string    `json:"event_type" firestore:"event_type"`
	SignalAction    string    `json:"signal_action" firestore:"signal_action"`
	SignalState     string    `json:"signal_state" firestore:"signal_state"`
	SignalRegime    string    `json:"signal_regime" firestore:"signal_regime"`
	BenchmarkSymbol string    `json:"benchmark_symbol,omitempty" firestore:"benchmark_symbol,omitempty"`
	Reasons         []string  `json:"reasons" firestore:"reasons"`
	CreatedAt       time.Time `json:"created_at" firestore:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" firestore:"updated_at"`
}

type SessionSummary struct {
	ID             string    `json:"id" firestore:"id"`
	Status         string    `json:"status" firestore:"status"`
	OpenWindows    int       `json:"open_windows" firestore:"open_windows"`
	LastDecisionAt time.Time `json:"last_decision_at" firestore:"last_decision_at"`
	Symbols        []string  `json:"symbols" firestore:"symbols"`
	UpdatedAt      time.Time `json:"updated_at" firestore:"updated_at"`
}

type TradeWindow struct {
	ID              string     `json:"id" firestore:"id"`
	SessionID       string     `json:"session_id" firestore:"session_id"`
	Symbol          string     `json:"symbol" firestore:"symbol"`
	Status          string     `json:"status" firestore:"status"`
	EntryDecisionID string     `json:"entry_decision_id,omitempty" firestore:"entry_decision_id,omitempty"`
	ExitDecisionID  string     `json:"exit_decision_id,omitempty" firestore:"exit_decision_id,omitempty"`
	OpenedAt        time.Time  `json:"opened_at" firestore:"opened_at"`
	ClosedAt        *time.Time `json:"closed_at,omitempty" firestore:"closed_at,omitempty"`
	EntryScore      float64    `json:"entry_score" firestore:"entry_score"`
	ExitScore       float64    `json:"exit_score" firestore:"exit_score"`
	UpdatedAt       time.Time  `json:"updated_at" firestore:"updated_at"`
}

type WindowSnapshot struct {
	ID             string    `json:"id" firestore:"id"`
	SessionID      string    `json:"session_id" firestore:"session_id"`
	WindowID       string    `json:"window_id" firestore:"window_id"`
	Symbol         string    `json:"symbol" firestore:"symbol"`
	Phase          string    `json:"phase" firestore:"phase"`
	EntryScore     float64   `json:"entry_score" firestore:"entry_score"`
	ExitScore      float64   `json:"exit_score" firestore:"exit_score"`
	IndicatorOrder []string  `json:"indicator_order" firestore:"indicator_order"`
	CapturedAt     time.Time `json:"captured_at" firestore:"captured_at"`
}

type WindowAnalyticsSummary struct {
	SessionID         string    `json:"session_id" firestore:"session_id"`
	SnapshotCount     int       `json:"snapshot_count" firestore:"snapshot_count"`
	OpenWindows       int       `json:"open_windows" firestore:"open_windows"`
	ClosedWindows     int       `json:"closed_windows" firestore:"closed_windows"`
	LastPhase         string    `json:"last_phase" firestore:"last_phase"`
	IndicatorOrder    []string  `json:"indicator_order" firestore:"indicator_order"`
	AverageEntryScore float64   `json:"average_entry_score" firestore:"average_entry_score"`
	AverageExitScore  float64   `json:"average_exit_score" firestore:"average_exit_score"`
	Symbols           []string  `json:"symbols" firestore:"symbols"`
	UpdatedAt         time.Time `json:"updated_at" firestore:"updated_at"`
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
