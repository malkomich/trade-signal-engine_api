package trading

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"trade-signal-engine-api/internal/alpaca"
	"trade-signal-engine-api/internal/model"
)

func TestRoundStopPrice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value float64
		want  float64
	}{
		{name: "above one", value: 123.4567, want: 123.46},
		{name: "below one", value: 0.123456, want: 0.1235},
		{name: "zero", value: 0, want: 0},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := roundStopPrice(tt.value); got != tt.want {
				t.Fatalf("roundStopPrice(%v) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestStopLossTimeInForce(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		qty  float64
		want string
	}{
		{name: "integer", qty: 10, want: "gtc"},
		{name: "fractional", qty: 2.5, want: "day"},
		{name: "zero", qty: 0, want: "gtc"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := stopLossTimeInForce(tt.qty); got != tt.want {
				t.Fatalf("stopLossTimeInForce(%v) = %q, want %q", tt.qty, got, tt.want)
			}
		})
	}
}

func TestExecuteBuyUsesLimitAndStopOrders(t *testing.T) {
	t.Parallel()

	var buyBody map[string]any
	var stopBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/paper/v2/account":
			_ = json.NewEncoder(w).Encode(alpaca.Account{
				Status:         "ACTIVE",
				BuyingPower:    "5000",
				Cash:           "5000",
				Equity:         "5000",
				PortfolioValue: "5000",
			})
		case req.Method == http.MethodPost && req.URL.Path == "/paper/v2/orders":
			var payload map[string]any
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode order payload: %v", err)
			}
			switch payload["type"] {
			case "market":
				buyBody = payload
				_ = json.NewEncoder(w).Encode(alpaca.Order{
					ID:             "buy-order",
					Status:         "filled",
					Symbol:         "NVDA",
					Side:           "buy",
					Type:           "market",
					Qty:            "4.6545",
					FilledQty:      "4.6545",
					FilledAvgPrice: "215.70",
				})
			case "stop":
				stopBody = payload
				_ = json.NewEncoder(w).Encode(alpaca.Order{
					ID:     "trail-order",
					Status: "new",
				})
			default:
				t.Fatalf("unexpected order type %v", payload["type"])
			}
		case req.Method == http.MethodGet && req.URL.Path == "/paper/v2/orders/buy-order":
			_ = json.NewEncoder(w).Encode(alpaca.Order{
				ID:             "buy-order",
				Status:         "filled",
				Symbol:         "NVDA",
				Side:           "buy",
				Type:           "market",
				Qty:            "4.6545",
				FilledQty:      "4.6545",
				FilledAvgPrice: "215.70",
			})
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	service := NewService(alpaca.NewClient(
		"paper-key",
		"paper-secret",
		"live-key",
		"live-secret",
		server.URL+"/paper",
		server.URL+"/live",
		5*time.Second,
	))

	result, err := service.Execute(context.Background(), model.SessionSummary{
		ID:                 "session-1",
		TradingMode:        "paper",
		TradingAllocations: map[string]float64{"balanced_buy": 1000},
		TradingStopLossPct: 0.2,
	}, model.TradingExecutionRequest{
		SessionID:  "session-1",
		Symbol:     "NVDA",
		Action:     "BUY_ALERT",
		Price:      215.70,
		SignalTier: "balanced_buy",
	})
	if err != nil {
		t.Fatalf("execute buy: %v", err)
	}
	if result.StopLossPrice != 215.27 {
		t.Fatalf("expected stop loss price 215.27, got %v", result.StopLossPrice)
	}
	if got := buyBody["type"]; got != "market" {
		t.Fatalf("expected market buy order, got %#v", got)
	}
	if _, ok := buyBody["limit_price"]; ok {
		t.Fatalf("expected no limit price for market buy order, got %#v", buyBody["limit_price"])
	}
	if got := buyBody["notional"]; got != 1000 {
		t.Fatalf("expected notional 1000, got %#v", got)
	}
	if got := result.Details["order_type"]; got != "market" {
		t.Fatalf("expected market order type detail, got %#v", got)
	}
	if got := result.Details["signal_price"]; got != 215.7 {
		t.Fatalf("expected signal price 215.7, got %#v", got)
	}
	if got := strings.ToLower(strings.TrimSpace(stopBody["type"].(string))); got != "stop" {
		t.Fatalf("expected stop order for protection, got %#v", got)
	}
	if got := stopBody["stop_price"]; got != 215.27 {
		t.Fatalf("expected stop price 215.27, got %#v", got)
	}
	if got := strings.ToLower(strings.TrimSpace(stopBody["time_in_force"].(string))); got != "day" {
		t.Fatalf("expected day stop order for fractional protection, got %#v", got)
	}
	if got := result.Details["stop_loss_percent"]; got != 0.2 {
		t.Fatalf("expected stop loss percent 0.2, got %#v", got)
	}
	if got := result.Details["stop_order_id"]; got != "trail-order" {
		t.Fatalf("expected stop order id trail-order, got %#v", got)
	}
	if _, ok := result.Details["stop_order_error"]; ok {
		t.Fatalf("expected no stop order error details on successful stop protection")
	}
}

func TestExecuteBuySkipsStopOrdersInRebuyMode(t *testing.T) {
	t.Parallel()

	var stopOrderCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/paper/v2/account":
			_ = json.NewEncoder(w).Encode(alpaca.Account{
				Status:         "ACTIVE",
				BuyingPower:    "5000",
				Cash:           "5000",
				Equity:         "5000",
				PortfolioValue: "5000",
			})
		case req.Method == http.MethodPost && req.URL.Path == "/paper/v2/orders":
			var payload map[string]any
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode order payload: %v", err)
			}
			switch payload["type"] {
			case "market":
				_ = json.NewEncoder(w).Encode(alpaca.Order{
					ID:             "buy-order",
					Status:         "filled",
					Symbol:         "TSLA",
					Side:           "buy",
					Type:           "market",
					Qty:            "5",
					FilledQty:      "5",
					FilledAvgPrice: "444.06",
				})
			case "stop":
				stopOrderCalls++
				t.Fatalf("rebuy mode should not submit stop orders")
			default:
				t.Fatalf("unexpected order type %v", payload["type"])
			}
		case req.Method == http.MethodGet && req.URL.Path == "/paper/v2/orders/buy-order":
			_ = json.NewEncoder(w).Encode(alpaca.Order{
				ID:             "buy-order",
				Status:         "filled",
				Symbol:         "TSLA",
				Side:           "buy",
				Type:           "market",
				Qty:            "5",
				FilledQty:      "5",
				FilledAvgPrice: "444.06",
			})
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	service := NewService(alpaca.NewClient(
		"paper-key",
		"paper-secret",
		"live-key",
		"live-secret",
		server.URL+"/paper",
		server.URL+"/live",
		5*time.Second,
	))

	result, err := service.Execute(context.Background(), model.SessionSummary{
		ID:                     "session-1",
		TradingMode:            "paper",
		TradingPositionMode:    "rebuy",
		TradingAllocations:     map[string]float64{"balanced_buy": 1000},
		TradingStopLossPct:     0.2,
		TradingRebuyMinDropPct: 0.8,
		TradingRebuyMaxCount:   2,
	}, model.TradingExecutionRequest{
		SessionID:  "session-1",
		Symbol:     "TSLA",
		Action:     "BUY_ALERT",
		Price:      444.06,
		SignalTier: "balanced_buy",
	})
	if err != nil {
		t.Fatalf("execute rebuy mode buy: %v", err)
	}
	if result.StopLossPrice != 0 {
		t.Fatalf("expected no stop loss price in rebuy mode, got %v", result.StopLossPrice)
	}
	if stopOrderCalls != 0 {
		t.Fatalf("expected no stop order calls in rebuy mode, got %d", stopOrderCalls)
	}
	if got := result.Details["position_mode"]; got != "rebuy" {
		t.Fatalf("expected rebuy position mode detail, got %#v", got)
	}
}

func TestExecuteBuyUsesGTCForWholeShareStops(t *testing.T) {
	t.Parallel()

	var stopBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/paper/v2/account":
			_ = json.NewEncoder(w).Encode(alpaca.Account{
				Status:         "ACTIVE",
				BuyingPower:    "5000",
				Cash:           "5000",
				Equity:         "5000",
				PortfolioValue: "5000",
			})
		case req.Method == http.MethodPost && req.URL.Path == "/paper/v2/orders":
			var payload map[string]any
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode order payload: %v", err)
			}
			switch payload["type"] {
			case "market":
				_ = json.NewEncoder(w).Encode(alpaca.Order{
					ID:             "buy-order",
					Status:         "filled",
					Symbol:         "TSLA",
					Side:           "buy",
					Type:           "market",
					Qty:            "5",
					FilledQty:      "5",
					FilledAvgPrice: "444.06",
				})
			case "stop":
				stopBody = payload
				_ = json.NewEncoder(w).Encode(alpaca.Order{
					ID:     "stop-order",
					Status: "new",
				})
			default:
				t.Fatalf("unexpected order type %v", payload["type"])
			}
		case req.Method == http.MethodGet && req.URL.Path == "/paper/v2/orders/buy-order":
			_ = json.NewEncoder(w).Encode(alpaca.Order{
				ID:             "buy-order",
				Status:         "filled",
				Symbol:         "TSLA",
				Side:           "buy",
				Type:           "market",
				Qty:            "5",
				FilledQty:      "5",
				FilledAvgPrice: "444.06",
			})
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	service := NewService(alpaca.NewClient(
		"paper-key",
		"paper-secret",
		"live-key",
		"live-secret",
		server.URL+"/paper",
		server.URL+"/live",
		5*time.Second,
	))

	result, err := service.Execute(context.Background(), model.SessionSummary{
		ID:                 "session-1",
		TradingMode:        "paper",
		TradingAllocations: map[string]float64{"balanced_buy": 1000},
		TradingStopLossPct: 0.2,
	}, model.TradingExecutionRequest{
		SessionID:  "session-1",
		Symbol:     "TSLA",
		Action:     "BUY_ALERT",
		Price:      444.06,
		SignalTier: "balanced_buy",
	})
	if err != nil {
		t.Fatalf("execute whole-share buy: %v", err)
	}
	if got := result.Details["stop_order_id"]; got != "stop-order" {
		t.Fatalf("expected stop order id stop-order, got %#v", got)
	}
	if got := stopBody["time_in_force"]; got != "gtc" {
		t.Fatalf("expected gtc stop order for whole shares, got %#v", got)
	}
	if got := stopBody["stop_price"]; got != 443.17 {
		t.Fatalf("expected stop price 443.17, got %#v", got)
	}
}

func TestExecuteBuyReturnsSuccessWhenStopFails(t *testing.T) {
	t.Parallel()

	var cancelCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/paper/v2/account":
			_ = json.NewEncoder(w).Encode(alpaca.Account{
				Status:         "ACTIVE",
				BuyingPower:    "5000",
				Cash:           "5000",
				Equity:         "5000",
				PortfolioValue: "5000",
			})
		case req.Method == http.MethodPost && req.URL.Path == "/paper/v2/orders":
			var payload map[string]any
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode order payload: %v", err)
			}
			if payload["type"] == "market" {
				_ = json.NewEncoder(w).Encode(alpaca.Order{
					ID:             "buy-order",
					Status:         "filled",
					Symbol:         "NVDA",
					Side:           "buy",
					Type:           "market",
					Qty:            "4.6545",
					FilledQty:      "4.6545",
					FilledAvgPrice: "215.70",
				})
				return
			}
			http.Error(w, "stop order failed", http.StatusInternalServerError)
		case req.Method == http.MethodGet && req.URL.Path == "/paper/v2/orders/buy-order":
			_ = json.NewEncoder(w).Encode(alpaca.Order{
				ID:             "buy-order",
				Status:         "filled",
				Symbol:         "NVDA",
				Side:           "buy",
				Type:           "market",
				Qty:            "4.6545",
				FilledQty:      "4.6545",
				FilledAvgPrice: "215.70",
			})
		case req.Method == http.MethodGet && req.URL.Path == "/paper/v2/orders?status=open&symbols=NVDA":
			_ = json.NewEncoder(w).Encode([]alpaca.Order{})
		case req.Method == http.MethodDelete:
			cancelCalls++
			http.NotFound(w, req)
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	service := NewService(alpaca.NewClient(
		"paper-key",
		"paper-secret",
		"live-key",
		"live-secret",
		server.URL+"/paper",
		server.URL+"/live",
		5*time.Second,
	))

	result, err := service.Execute(context.Background(), model.SessionSummary{
		ID:                 "session-1",
		TradingMode:        "paper",
		TradingAllocations: map[string]float64{"balanced_buy": 1000},
		TradingStopLossPct: 0.2,
	}, model.TradingExecutionRequest{
		SessionID:  "session-1",
		Symbol:     "NVDA",
		Action:     "BUY_ALERT",
		Price:      215.70,
		SignalTier: "balanced_buy",
	})
	if err != nil {
		t.Fatalf("execute buy with stop protection failure: %v", err)
	}
	if result.OrderID != "buy-order" {
		t.Fatalf("expected buy order id buy-order, got %q", result.OrderID)
	}
	if got, ok := result.Details["stop_order_error"].(string); !ok || strings.TrimSpace(got) == "" {
		t.Fatalf("expected stop order error details, got %#v", result.Details["stop_order_error"])
	}
	if cancelCalls != 0 {
		t.Fatalf("expected no cancel calls during stop submission failure, got %d", cancelCalls)
	}
}

func TestExecuteBuyPreservesPartialFillAfterContextCancellation(t *testing.T) {
	t.Parallel()

	var orderChecks int
	firstPoll := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/paper/v2/account":
			_ = json.NewEncoder(w).Encode(alpaca.Account{
				Status:         "ACTIVE",
				BuyingPower:    "5000",
				Cash:           "5000",
				Equity:         "5000",
				PortfolioValue: "5000",
			})
		case req.Method == http.MethodPost && req.URL.Path == "/paper/v2/orders":
			var payload map[string]any
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode order payload: %v", err)
			}
			switch payload["type"] {
			case "market":
				_ = json.NewEncoder(w).Encode(alpaca.Order{
					ID:             "buy-order",
					Status:         "new",
					Symbol:         "NVDA",
					Side:           "buy",
					Type:           "market",
					Qty:            "4.6545",
					FilledQty:      "0",
					FilledAvgPrice: "0",
				})
			case "stop":
				_ = json.NewEncoder(w).Encode(alpaca.Order{
					ID:     "trail-order",
					Status: "new",
				})
			default:
				t.Fatalf("unexpected order type %v", payload["type"])
			}
		case req.Method == http.MethodGet && req.URL.Path == "/paper/v2/orders/buy-order":
			orderChecks++
			if orderChecks == 1 {
				close(firstPoll)
			}
			_ = json.NewEncoder(w).Encode(alpaca.Order{
				ID:             "buy-order",
				Status:         "partially_filled",
				Symbol:         "NVDA",
				Side:           "buy",
				Type:           "market",
				Qty:            "4.6545",
				FilledQty:      "2.0000",
				FilledAvgPrice: "215.70",
			})
		case req.Method == http.MethodDelete && req.URL.Path == "/paper/v2/orders/buy-order":
			_ = json.NewEncoder(w).Encode(alpaca.Order{})
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	go func() {
		<-firstPoll
		cancel()
	}()

	service := NewService(alpaca.NewClient(
		"paper-key",
		"paper-secret",
		"live-key",
		"live-secret",
		server.URL+"/paper",
		server.URL+"/live",
		5*time.Second,
	))

	result, err := service.Execute(ctx, model.SessionSummary{
		ID:                 "session-1",
		TradingMode:        "paper",
		TradingAllocations: map[string]float64{"balanced_buy": 1000},
		TradingStopLossPct: 0.2,
	}, model.TradingExecutionRequest{
		SessionID:  "session-1",
		Symbol:     "NVDA",
		Action:     "BUY_ALERT",
		Price:      215.70,
		SignalTier: "balanced_buy",
	})
	if err != nil {
		t.Fatalf("execute buy with partial fill and cancellation: %v", err)
	}
	if result.OrderID != "buy-order" {
		t.Fatalf("expected buy order id buy-order, got %q", result.OrderID)
	}
	if got, ok := result.Details["buy_order_warning"].(string); !ok || strings.TrimSpace(got) == "" {
		t.Fatalf("expected buy order warning details, got %#v", result.Details["buy_order_warning"])
	}
	if got, ok := result.Details["stop_order_error"]; ok {
		t.Fatalf("expected stop order submission to succeed, got %#v", got)
	}
}
