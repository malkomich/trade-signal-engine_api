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

func TestExecuteBuyUsesLimitAndTrailingStopOrders(t *testing.T) {
	t.Parallel()

	var buyBody map[string]any
	var trailBody map[string]any
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
			case "limit":
				buyBody = payload
				_ = json.NewEncoder(w).Encode(alpaca.Order{
					ID:             "buy-order",
					Status:         "filled",
					Symbol:         "NVDA",
					Side:           "buy",
					Type:           "limit",
					Qty:            "4.6545",
					FilledQty:      "4.6545",
					FilledAvgPrice: "215.70",
				})
			case "trailing_stop":
				trailBody = payload
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
				Type:           "limit",
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
	if got := buyBody["type"]; got != "limit" {
		t.Fatalf("expected limit buy order, got %#v", got)
	}
	if got := buyBody["limit_price"]; got != 215.7 {
		t.Fatalf("expected limit price 215.7, got %#v", got)
	}
	if got := buyBody["notional"]; got != 1000 {
		t.Fatalf("expected notional 1000, got %#v", got)
	}
	if got := strings.ToLower(strings.TrimSpace(trailBody["type"].(string))); got != "trailing_stop" {
		t.Fatalf("expected trailing stop order, got %#v", got)
	}
	if got := trailBody["trail_percent"]; got != 0.2 {
		t.Fatalf("expected trail percent 0.2, got %#v", got)
	}
	if got := strings.ToLower(strings.TrimSpace(trailBody["time_in_force"].(string))); got != "gtc" {
		t.Fatalf("expected gtc trailing stop, got %#v", got)
	}
	if _, ok := result.Details["stop_order_error"]; ok {
		t.Fatalf("expected no stop order error details on successful trailing stop")
	}
}

func TestExecuteBuyReturnsSuccessWhenTrailingStopFails(t *testing.T) {
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
			if payload["type"] == "limit" {
				_ = json.NewEncoder(w).Encode(alpaca.Order{
					ID:             "buy-order",
					Status:         "filled",
					Symbol:         "NVDA",
					Side:           "buy",
					Type:           "limit",
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
				Type:           "limit",
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
		t.Fatalf("execute buy with trailing stop failure: %v", err)
	}
	if result.OrderID != "buy-order" {
		t.Fatalf("expected buy order id buy-order, got %q", result.OrderID)
	}
	if got := result.Details["stop_order_error"]; got == "" {
		t.Fatalf("expected stop order error details, got %#v", got)
	}
	if cancelCalls != 0 {
		t.Fatalf("expected no cancel calls during trailing stop submission failure, got %d", cancelCalls)
	}
}
