package model

const (
	CollectionDecisionEvents     = "decision_events"
	CollectionMarketSessions     = "market_sessions"
	CollectionSignalEvents       = "signal_events"
	CollectionTradeWindows       = "trade_windows"
	CollectionDailySymbolSummary = "daily_symbol_summaries"
	CollectionDailyMarketSummary = "daily_market_summaries"
	CollectionConfigVersions     = "config_versions"
	CollectionNotificationEvents = "notification_events"

	EventTypeDecisionCreated      = "decision.created"
	EventTypeDecisionAccepted     = "decision.accepted"
	EventTypeDecisionRejected     = "decision.rejected"
	EventTypeDecisionAcknowledged = "decision.acknowledged"
	EventTypeSignalEmitted        = "signal.emitted"
	EventTypeSessionStarted       = "session.started"
	EventTypeSessionClosed        = "session.closed"
)
