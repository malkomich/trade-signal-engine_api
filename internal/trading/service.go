package trading

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"trade-signal-engine-api/internal/alpaca"
	"trade-signal-engine-api/internal/model"
)

const (
	DefaultTradingMode        = "paper"
	DefaultTradingAllocation  = 1000.0
	DefaultTradingStopLossPct = 0.20
	maxTradingStopLossPct     = 10.0
	orderFillPollInterval     = 750 * time.Millisecond
	orderFillTimeout          = 30 * time.Second
)

type Service struct {
	client *alpaca.Client
}

func NewService(client *alpaca.Client) *Service {
	if client == nil {
		return nil
	}
	return &Service{client: client}
}

func (s *Service) Enabled() bool {
	return s != nil && s.client != nil
}

func (s *Service) CurrentAccount(ctx context.Context, mode string) (model.TradingAccountSnapshot, error) {
	if !s.Enabled() {
		return model.TradingAccountSnapshot{}, errors.New("alpaca trading is not configured")
	}
	mode = normalizeMode(mode)
	if mode == "" {
		mode = DefaultTradingMode
	}
	account, err := s.client.GetAccount(ctx, mode)
	if err != nil {
		return model.TradingAccountSnapshot{}, err
	}
	return model.TradingAccountSnapshot{
		Mode:           mode,
		Status:         account.Status,
		BuyingPower:    parseFloat(account.BuyingPower, 0),
		Cash:           parseFloat(account.Cash, 0),
		Equity:         parseFloat(account.Equity, 0),
		PortfolioValue: parseFloat(account.PortfolioValue, 0),
		UpdatedAt:      time.Now().UTC(),
	}, nil
}

func (s *Service) Execute(ctx context.Context, session model.SessionSummary, request model.TradingExecutionRequest) (model.TradingExecutionResult, error) {
	if !s.Enabled() {
		return model.TradingExecutionResult{}, errors.New("alpaca trading is not configured")
	}
	mode := normalizeMode(session.TradingMode)
	if mode == "" {
		mode = DefaultTradingMode
	}
	settings := normalizeTradingSettings(session)

	switch strings.ToUpper(strings.TrimSpace(request.Action)) {
	case "BUY_ALERT", "BUY":
		account, err := s.CurrentAccount(ctx, mode)
		if err != nil {
			return model.TradingExecutionResult{}, err
		}
		return s.executeBuy(ctx, session.ID, mode, settings, request, account)
	case "SELL_ALERT", "SELL":
		account := model.TradingAccountSnapshot{Mode: mode}
		if currentAccount, err := s.CurrentAccount(ctx, mode); err == nil {
			account = currentAccount
		}
		return s.executeSell(ctx, session.ID, mode, request, account)
	default:
		return model.TradingExecutionResult{}, fmt.Errorf("unsupported trading action %q", request.Action)
	}
}

func (s *Service) executeBuy(
	ctx context.Context,
	sessionID string,
	mode string,
	settings model.SessionSummary,
	request model.TradingExecutionRequest,
	account model.TradingAccountSnapshot,
) (model.TradingExecutionResult, error) {
	allocation := allocationForTier(settings.TradingAllocations, request.SignalTier)
	if allocation <= 0 {
		allocation = DefaultTradingAllocation
	}
	if account.BuyingPower > 0 {
		allocation = math.Min(allocation, account.BuyingPower)
	}
	limitPrice := roundStopPrice(request.Price)
	if limitPrice <= 0 {
		return model.TradingExecutionResult{}, fmt.Errorf("alpaca buy order %s requires a valid limit price", strings.ToUpper(strings.TrimSpace(request.Symbol)))
	}
	order, err := s.client.SubmitOrder(ctx, mode, alpaca.OrderRequest{
		Symbol:      strings.ToUpper(strings.TrimSpace(request.Symbol)),
		Side:        "buy",
		Type:        "limit",
		TimeInForce: "day",
		Notional:    float64Ptr(allocation),
		LimitPrice:  float64Ptr(limitPrice),
	})
	if err != nil {
		return model.TradingExecutionResult{}, err
	}

	filledOrder, err := s.waitForFilledOrder(ctx, mode, order.ID)
	if err != nil {
		return model.TradingExecutionResult{}, err
	}
	filledQty := parseFloat(filledOrder.FilledQty, 0)
	if filledQty <= 0 {
		filledQty = parseFloat(filledOrder.Qty, 0)
	}
	filledPrice := parseFloat(filledOrder.FilledAvgPrice, 0)
	if filledQty <= 0 || filledPrice <= 0 {
		return model.TradingExecutionResult{}, fmt.Errorf("alpaca buy order %s did not return a filled quantity and price", order.ID)
	}
	stopLossPct := normalizeStopLossPercent(settings.TradingStopLossPct)
	stopLossPrice := 0.0
	if filledPrice > 0 && stopLossPct > 0 {
		stopLossPrice = roundStopPrice(filledPrice * (1.0 - (stopLossPct / 100.0)))
	}
	trailingStopOrder := alpaca.Order{}
	trailingStopError := ""
	if filledQty > 0 && stopLossPrice > 0 {
		trailingStopOrder, err = s.client.SubmitOrder(ctx, mode, alpaca.OrderRequest{
			Symbol:       strings.ToUpper(strings.TrimSpace(request.Symbol)),
			Side:         "sell",
			Type:         "trailing_stop",
			TimeInForce:  "gtc",
			Qty:          float64Ptr(filledQty),
			TrailPercent: float64Ptr(stopLossPct),
		})
		if err != nil {
			trailingStopOrder = alpaca.Order{}
			trailingStopError = err.Error()
		}
	}
	updatedAccount, err := s.CurrentAccount(ctx, mode)
	if err == nil {
		account = updatedAccount
	}

	return model.TradingExecutionResult{
		Status:        "submitted",
		SessionID:     sessionID,
		Symbol:        strings.ToUpper(strings.TrimSpace(request.Symbol)),
		Action:        strings.ToUpper(strings.TrimSpace(request.Action)),
		Mode:          mode,
		OrderID:       order.ID,
		Side:          order.Side,
		Quantity:      filledQty,
		Notional:      allocation,
		StopLossPrice: stopLossPrice,
		Account:       &account,
		SubmittedAt:   time.Now().UTC(),
		Details: func() map[string]any {
			details := map[string]any{
				"filled_order_status": filledOrder.Status,
				"trail_order_id":      trailingStopOrder.ID,
				"limit_price":         limitPrice,
				"trail_percent":       stopLossPct,
			}
			if trailingStopError != "" {
				details["stop_order_error"] = trailingStopError
			}
			return details
		}(),
	}, nil
}

func (s *Service) executeSell(
	ctx context.Context,
	sessionID string,
	mode string,
	request model.TradingExecutionRequest,
	account model.TradingAccountSnapshot,
) (model.TradingExecutionResult, error) {
	symbol := strings.ToUpper(strings.TrimSpace(request.Symbol))
	if err := s.cancelOpenOrdersForSymbol(ctx, mode, symbol); err != nil {
		return model.TradingExecutionResult{}, err
	}
	order, err := s.client.ClosePosition(ctx, mode, symbol)
	if err != nil {
		return model.TradingExecutionResult{}, err
	}
	return model.TradingExecutionResult{
		Status:      "submitted",
		SessionID:   sessionID,
		Symbol:      symbol,
		Action:      strings.ToUpper(strings.TrimSpace(request.Action)),
		Mode:        mode,
		OrderID:     order.ID,
		Side:        order.Side,
		Account:     &account,
		SubmittedAt: time.Now().UTC(),
	}, nil
}

func (s *Service) waitForFilledOrder(ctx context.Context, mode, orderID string) (alpaca.Order, error) {
	deadline := time.NewTimer(orderFillTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(orderFillPollInterval)
	defer ticker.Stop()
	for {
		order, err := s.client.GetOrder(ctx, mode, orderID)
		if err != nil {
			return alpaca.Order{}, err
		}
		status := strings.ToLower(strings.TrimSpace(order.Status))
		if status == "filled" {
			return order, nil
		}
		if status == "partially_filled" {
			// Keep polling until the order is fully filled, canceled, or times out.
		}
		if status == "canceled" || status == "expired" || status == "rejected" || status == "done_for_day" {
			return alpaca.Order{}, fmt.Errorf("alpaca order %s ended with status %q", orderID, status)
		}
		select {
		case <-ctx.Done():
			cancelCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			_ = s.client.CancelOrder(cancelCtx, mode, orderID)
			cancel()
			return alpaca.Order{}, ctx.Err()
		case <-deadline.C:
			cancelCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			_ = s.client.CancelOrder(cancelCtx, mode, orderID)
			cancel()
			return alpaca.Order{}, fmt.Errorf("alpaca order %s did not fill within %s", orderID, orderFillTimeout)
		case <-ticker.C:
		}
	}
}

func (s *Service) cancelOpenOrdersForSymbol(ctx context.Context, mode, symbol string) error {
	orders, err := s.client.ListOpenOrders(ctx, mode, symbol)
	if err != nil {
		return err
	}
	var cancelErrors []string
	for _, order := range orders {
		if strings.ToUpper(strings.TrimSpace(order.Symbol)) != symbol {
			continue
		}
		if err := s.client.CancelOrder(ctx, mode, order.ID); err != nil {
			cancelErrors = append(cancelErrors, fmt.Sprintf("%s:%s", order.ID, err))
		}
	}
	if len(cancelErrors) > 0 {
		return fmt.Errorf("cancel alpaca open orders: %s", strings.Join(cancelErrors, "; "))
	}
	return nil
}

func normalizeTradingSettings(session model.SessionSummary) model.SessionSummary {
	if normalizeMode(session.TradingMode) == "" {
		session.TradingMode = DefaultTradingMode
	}
	if len(session.TradingAllocations) == 0 {
		session.TradingAllocations = DefaultTradingAllocations()
	}
	if session.TradingStopLossPct <= 0 {
		session.TradingStopLossPct = DefaultTradingStopLossPct
	}
	return session
}

func normalizeMode(mode string) string {
	normalized := strings.ToLower(strings.TrimSpace(mode))
	switch normalized {
	case "paper", "live":
		return normalized
	default:
		return ""
	}
}

func normalizeStopLossPercent(value float64) float64 {
	if value <= 0 {
		return DefaultTradingStopLossPct
	}
	if value > maxTradingStopLossPct {
		return maxTradingStopLossPct
	}
	return value
}

func allocationForTier(allocations map[string]float64, tier string) float64 {
	if len(allocations) == 0 {
		return 0
	}
	normalizedTier := strings.ToLower(strings.TrimSpace(tier))
	if normalizedTier == "" {
		return 0
	}
	if value, ok := allocations[normalizedTier]; ok {
		return value
	}
	return 0
}

func DefaultTradingAllocations() map[string]float64 {
	return map[string]float64{
		"conviction_buy":    DefaultTradingAllocation,
		"balanced_buy":      DefaultTradingAllocation,
		"opportunistic_buy": DefaultTradingAllocation,
		"speculative_buy":   DefaultTradingAllocation,
	}
}

func float64Ptr(value float64) *float64 {
	if value <= 0 {
		return nil
	}
	return &value
}

func roundStopPrice(value float64) float64 {
	if value <= 0 {
		return 0
	}
	precision := 100.0
	if value < 1.0 {
		precision = 10000.0
	}
	return math.Round(value*precision) / precision
}

func stopLossTimeInForce(qty float64) string {
	if qty <= 0 {
		return "gtc"
	}
	if math.Abs(qty-math.Round(qty)) > 1e-9 {
		return "day"
	}
	return "gtc"
}

func parseFloat(value string, fallback float64) float64 {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return fallback
	}
	return parsed
}
