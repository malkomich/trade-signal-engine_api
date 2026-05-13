package alpaca

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientUsesModeSpecificCredentials(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		t.Helper()
		switch req.URL.Path {
		case "/paper/v2/account":
			assertHeader(t, req, "APCA-API-KEY-ID", "paper-key")
			assertHeader(t, req, "APCA-API-SECRET-KEY", "paper-secret")
		case "/live/v2/account":
			assertHeader(t, req, "APCA-API-KEY-ID", "live-key")
			assertHeader(t, req, "APCA-API-SECRET-KEY", "live-secret")
		default:
			t.Fatalf("unexpected path %q", req.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(Account{
			Status:         "ACTIVE",
			BuyingPower:    "2500",
			Cash:           "1250",
			Equity:         "3750",
			PortfolioValue: "4000",
		})
	}))
	t.Cleanup(server.Close)

	client := NewClient(
		"paper-key",
		"paper-secret",
		"live-key",
		"live-secret",
		server.URL+"/paper",
		server.URL+"/live",
		5*time.Second,
	)

	paperAccount, err := client.GetAccount(context.Background(), "paper")
	if err != nil {
		t.Fatalf("paper account: %v", err)
	}
	if paperAccount.Status != "ACTIVE" {
		t.Fatalf("unexpected paper account status %q", paperAccount.Status)
	}

	liveAccount, err := client.GetAccount(context.Background(), "live")
	if err != nil {
		t.Fatalf("live account: %v", err)
	}
	if liveAccount.PortfolioValue != "4000" {
		t.Fatalf("unexpected live account portfolio value %q", liveAccount.PortfolioValue)
	}
}

func TestClientSerializesLimitAndTrailingStopOrders(t *testing.T) {
	t.Parallel()

	var orderBodies []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		t.Helper()
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/paper/v2/orders":
			assertHeader(t, req, "APCA-API-KEY-ID", "paper-key")
			assertHeader(t, req, "APCA-API-SECRET-KEY", "paper-secret")
			var payload map[string]any
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			orderBodies = append(orderBodies, payload)
			_ = json.NewEncoder(w).Encode(Order{
				ID:             "order-" + payload["type"].(string),
				Status:         "filled",
				Symbol:         strings.TrimSpace(payload["symbol"].(string)),
				Side:           strings.TrimSpace(payload["side"].(string)),
				Type:           strings.TrimSpace(payload["type"].(string)),
				Qty:            "5",
				FilledQty:      "5",
				FilledAvgPrice: "215.70",
			})
		case req.Method == http.MethodPost && req.URL.Path == "/live/v2/orders":
			assertHeader(t, req, "APCA-API-KEY-ID", "live-key")
			assertHeader(t, req, "APCA-API-SECRET-KEY", "live-secret")
			var payload map[string]any
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			orderBodies = append(orderBodies, payload)
			_ = json.NewEncoder(w).Encode(Order{
				ID:             "order-" + payload["type"].(string),
				Status:         "filled",
				Symbol:         strings.TrimSpace(payload["symbol"].(string)),
				Side:           strings.TrimSpace(payload["side"].(string)),
				Type:           strings.TrimSpace(payload["type"].(string)),
				Qty:            "5",
				FilledQty:      "5",
				FilledAvgPrice: "215.70",
			})
		case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/v2/account"):
			_ = json.NewEncoder(w).Encode(Account{
				Status:         "ACTIVE",
				BuyingPower:    "5000",
				Cash:           "5000",
				Equity:         "5000",
				PortfolioValue: "5000",
			})
		case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/v2/orders/"):
			_ = json.NewEncoder(w).Encode(Order{
				ID:             strings.TrimPrefix(req.URL.Path, "/paper/v2/orders/"),
				Status:         "filled",
				Symbol:         "NVDA",
				Side:           "buy",
				Type:           "limit",
				Qty:            "5",
				FilledQty:      "5",
				FilledAvgPrice: "215.70",
			})
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	client := NewClient(
		"paper-key",
		"paper-secret",
		"live-key",
		"live-secret",
		server.URL+"/paper",
		server.URL+"/live",
		5*time.Second,
	)

	if _, err := client.SubmitOrder(context.Background(), "paper", OrderRequest{
		Symbol:      "NVDA",
		Side:        "buy",
		Type:        "limit",
		TimeInForce: "day",
		Notional:    float64Ptr(1000),
		LimitPrice:  float64Ptr(215.7),
	}); err != nil {
		t.Fatalf("submit limit order: %v", err)
	}
	if _, err := client.SubmitOrder(context.Background(), "live", OrderRequest{
		Symbol:       "NVDA",
		Side:         "sell",
		Type:         "trailing_stop",
		TimeInForce:  "gtc",
		Qty:          float64Ptr(5),
		TrailPercent: float64Ptr(0.2),
	}); err != nil {
		t.Fatalf("submit trailing stop order: %v", err)
	}

	if len(orderBodies) != 2 {
		t.Fatalf("expected two order bodies, got %d", len(orderBodies))
	}
	if got := orderBodies[0]["limit_price"]; got != 215.7 {
		t.Fatalf("expected limit_price 215.7, got %#v", got)
	}
	if got := orderBodies[1]["trail_percent"]; got != 0.2 {
		t.Fatalf("expected trail_percent 0.2, got %#v", got)
	}
}

func TestClosePositionRejectsEmptySymbol(t *testing.T) {
	t.Parallel()

	client := NewClient("paper-key", "paper-secret", "live-key", "live-secret", "https://paper-api.alpaca.markets", "https://api.alpaca.markets", 5*time.Second)
	if _, err := client.ClosePosition(context.Background(), "paper", ""); err == nil {
		t.Fatalf("expected close position to reject empty symbol")
	}
}

func float64Ptr(value float64) *float64 {
	return &value
}

func assertHeader(t *testing.T, req *http.Request, name, want string) {
	t.Helper()
	if got := req.Header.Get(name); got != want {
		t.Fatalf("expected %s=%q, got %q", name, want, got)
	}
}
