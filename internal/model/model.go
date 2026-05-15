package model

import "time"

type DecisionRequest struct {
	SessionID   string  `json:"session_id"`
	Symbol      string  `json:"symbol"`
	Action      string  `json:"action"`
	Reason      string  `json:"reason"`
	EntryScore  float64 `json:"entry_score"`
	ExitScore   float64 `json:"exit_score"`
	SignalTier  string  `json:"signal_tier,omitempty"`
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
	SignalTier string    `json:"signal_tier,omitempty"`
	EventType  string    `json:"event_type"`
	CreatedAt  time.Time `json:"created_at"`
}

type SignalEvent struct {
	ID         string    `json:"id"`
	SessionID  string    `json:"session_id"`
	WindowID   string    `json:"window_id,omitempty"`
	Symbol     string    `json:"symbol"`
	Action     string    `json:"action,omitempty"`
	SignalTier string    `json:"signal_tier,omitempty"`
	State      string    `json:"state"`
	EntryScore float64   `json:"entry_score"`
	ExitScore  float64   `json:"exit_score"`
	Regime     string    `json:"regime"`
	Reasons    []string  `json:"reasons"`
	Timestamp  time.Time `json:"timestamp"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type MarketSnapshot struct {
	ID                 string    `json:"id"`
	SessionID          string    `json:"session_id"`
	WindowID           string    `json:"window_id,omitempty"`
	Symbol             string    `json:"symbol"`
	Timeframe          string    `json:"timeframe,omitempty"`
	Timestamp          time.Time `json:"timestamp"`
	Open               float64   `json:"open"`
	High               float64   `json:"high"`
	Low                float64   `json:"low"`
	Close              float64   `json:"close"`
	Volume             float64   `json:"volume"`
	SMAFast            float64   `json:"sma_fast"`
	SMASlow            float64   `json:"sma_slow"`
	EMAFast            float64   `json:"ema_fast"`
	EMASlow            float64   `json:"ema_slow"`
	VWAP               float64   `json:"vwap"`
	RSI                float64   `json:"rsi"`
	RSIDelta           float64   `json:"rsi_delta"`
	ATR                float64   `json:"atr"`
	PlusDI             float64   `json:"plus_di"`
	MinusDI            float64   `json:"minus_di"`
	ADX                float64   `json:"adx"`
	MACD               float64   `json:"macd"`
	MACDSignal         float64   `json:"macd_signal"`
	MACDHistogram      float64   `json:"macd_histogram"`
	MACDHistogramDelta float64   `json:"macd_histogram_delta"`
	StochasticK        float64   `json:"stochastic_k"`
	StochasticD        float64   `json:"stochastic_d"`
	StochasticKDelta   float64   `json:"stochastic_k_delta"`
	StochasticDDelta   float64   `json:"stochastic_d_delta"`
	BollingerMiddle    float64   `json:"bollinger_middle"`
	BollingerUpper     float64   `json:"bollinger_upper"`
	BollingerLower     float64   `json:"bollinger_lower"`
	OBV                float64   `json:"obv"`
	RelativeVolume     float64   `json:"relative_volume"`
	VolumeProfile      float64   `json:"volume_profile"`
	EntryScore         float64   `json:"entry_score"`
	ExitScore          float64   `json:"exit_score"`
	EventType          string    `json:"event_type"`
	SignalAction       string    `json:"signal_action"`
	SignalTier         string    `json:"signal_tier,omitempty"`
	SignalState        string    `json:"signal_state"`
	SignalRegime       string    `json:"signal_regime"`
	BenchmarkSymbol    string    `json:"benchmark_symbol,omitempty"`
	Reasons            []string  `json:"reasons"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type SessionSummary struct {
	ID                  string                     `json:"id"`
	Status              string                     `json:"status"`
	OpenWindows         int                        `json:"open_windows"`
	LastDecisionAt      time.Time                  `json:"last_decision_at"`
	Symbols             []string                   `json:"symbols"`
	ConfigVersion       string                     `json:"config_version"`
	TradingMode         string                     `json:"trading_mode,omitempty"`
	TradingPositionMode string                     `json:"trading_position_mode,omitempty"`
	TradingAllocations  map[string]float64         `json:"trading_allocations,omitempty"`
	TradingStopLossPct  float64                    `json:"trading_stop_loss_percent,omitempty"`
	TradingRebuyMinDropPct float64                 `json:"trading_rebuy_min_drop_percent,omitempty"`
	TradingRebuyMaxCount   int                     `json:"trading_rebuy_max_rebuys,omitempty"`
	TradingAccount      *TradingAccountSnapshot    `json:"trading_account,omitempty"`
	TradingUpdatedAt    *time.Time                 `json:"trading_updated_at,omitempty"`
	OptimizationSummary *WindowOptimizationSummary `json:"optimization_summary,omitempty"`
	UpdatedAt           time.Time                  `json:"updated_at"`
}

type TradingAccountSnapshot struct {
	Mode           string    `json:"mode"`
	Status         string    `json:"status"`
	BuyingPower    float64   `json:"buying_power"`
	Cash           float64   `json:"cash"`
	Equity         float64   `json:"equity"`
	PortfolioValue float64   `json:"portfolio_value"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type TradingSettingsRequest struct {
	SessionID       string             `json:"session_id"`
	Mode            string             `json:"mode"`
	PositionMode    string             `json:"position_management_mode"`
	Allocations     map[string]float64 `json:"allocations"`
	StopLossPercent float64            `json:"stop_loss_percent"`
	RebuyMinDropPct float64            `json:"rebuy_min_drop_percent"`
	RebuyMaxCount   int                `json:"rebuy_max_rebuys"`
}

type TradingExecutionRequest struct {
	SessionID  string    `json:"session_id"`
	Symbol     string    `json:"symbol"`
	Action     string    `json:"action"`
	Price      float64   `json:"price"`
	LimitPrice float64   `json:"limit_price,omitempty"`
	SignalTier string    `json:"signal_tier,omitempty"`
	EntryScore float64   `json:"entry_score"`
	ExitScore  float64   `json:"exit_score"`
	WindowID   string    `json:"window_id,omitempty"`
	Reason     string    `json:"reason,omitempty"`
	CreatedAt  time.Time `json:"created_at,omitempty"`
}

type TradingExecutionResult struct {
	Status        string                  `json:"status"`
	SessionID     string                  `json:"session_id"`
	Symbol        string                  `json:"symbol"`
	Action        string                  `json:"action"`
	Mode          string                  `json:"mode"`
	OrderID       string                  `json:"order_id,omitempty"`
	Side          string                  `json:"side,omitempty"`
	Quantity      float64                 `json:"quantity,omitempty"`
	Notional      float64                 `json:"notional,omitempty"`
	FilledAvgPrice float64                `json:"filled_avg_price,omitempty"`
	StopLossPrice float64                 `json:"stop_loss_price,omitempty"`
	Account       *TradingAccountSnapshot `json:"account,omitempty"`
	SubmittedAt   time.Time               `json:"submitted_at,omitempty"`
	Details       map[string]any          `json:"details,omitempty"`
}

type TradeWindow struct {
	ID                   string     `json:"id"`
	SessionID            string     `json:"session_id"`
	Symbol               string     `json:"symbol"`
	Status               string     `json:"status"`
	PositionMode         string     `json:"position_management_mode,omitempty"`
	EntryDecisionID      string     `json:"entry_decision_id,omitempty"`
	LastEntryDecisionID  string     `json:"last_entry_decision_id,omitempty"`
	ExitDecisionID       string     `json:"exit_decision_id,omitempty"`
	BuySignalCount       int        `json:"buy_signal_count,omitempty"`
	BuyExecutionCount    int        `json:"buy_execution_count,omitempty"`
	RebuyCount           int        `json:"rebuy_count,omitempty"`
	OpenedAt             time.Time  `json:"opened_at"`
	ClosedAt             *time.Time `json:"closed_at,omitempty"`
	EntryScore           float64    `json:"entry_score"`
	ExitScore            float64    `json:"exit_score"`
	EntryQuantity        float64    `json:"entry_quantity,omitempty"`
	EntryNotional        float64    `json:"entry_notional,omitempty"`
	AverageEntryPrice    float64    `json:"average_entry_price,omitempty"`
	LastEntryPrice       float64    `json:"last_entry_price,omitempty"`
	LastEntryAt          *time.Time `json:"last_entry_at,omitempty"`
	UpdatedAt            time.Time  `json:"updated_at"`
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
