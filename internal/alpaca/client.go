package alpaca

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	apiKeyID   string
	secretKey  string
	paperURL   string
	liveURL    string
	httpClient *http.Client
}

type Account struct {
	Status         string `json:"status"`
	BuyingPower    string `json:"buying_power"`
	Cash           string `json:"cash"`
	Equity         string `json:"equity"`
	PortfolioValue string `json:"portfolio_value"`
}

type OrderRequest struct {
	Symbol      string   `json:"symbol"`
	Side        string   `json:"side"`
	Type        string   `json:"type"`
	TimeInForce string   `json:"time_in_force"`
	Notional    *float64 `json:"notional,omitempty"`
	Qty         *float64 `json:"qty,omitempty"`
	StopPrice   *float64 `json:"stop_price,omitempty"`
}

type Order struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	Symbol         string `json:"symbol"`
	Side           string `json:"side"`
	Type           string `json:"type"`
	Qty            string `json:"qty"`
	FilledQty      string `json:"filled_qty"`
	FilledAvgPrice string `json:"filled_avg_price"`
	Notional       string `json:"notional"`
	StopPrice      string `json:"stop_price"`
}

func NewClient(apiKeyID, secretKey, paperURL, liveURL string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Client{
		apiKeyID:   strings.TrimSpace(apiKeyID),
		secretKey:  strings.TrimSpace(secretKey),
		paperURL:   strings.TrimRight(strings.TrimSpace(paperURL), "/"),
		liveURL:    strings.TrimRight(strings.TrimSpace(liveURL), "/"),
		httpClient: &http.Client{Timeout: timeout},
	}
}

func (c *Client) configured() bool {
	return c != nil && c.apiKeyID != "" && c.secretKey != "" && (c.paperURL != "" || c.liveURL != "")
}

func (c *Client) GetAccount(ctx context.Context, mode string) (Account, error) {
	if !c.configured() {
		return Account{}, errors.New("alpaca client not configured")
	}
	body, err := c.do(ctx, http.MethodGet, mode, "/v2/account", nil)
	if err != nil {
		return Account{}, err
	}
	var account Account
	if err := json.Unmarshal(body, &account); err != nil {
		return Account{}, fmt.Errorf("decode alpaca account: %w", err)
	}
	return account, nil
}

func (c *Client) SubmitOrder(ctx context.Context, mode string, req OrderRequest) (Order, error) {
	if !c.configured() {
		return Order{}, errors.New("alpaca client not configured")
	}
	body, err := c.do(ctx, http.MethodPost, mode, "/v2/orders", req)
	if err != nil {
		return Order{}, err
	}
	var order Order
	if err := json.Unmarshal(body, &order); err != nil {
		return Order{}, fmt.Errorf("decode alpaca order: %w", err)
	}
	return order, nil
}

func (c *Client) ListOpenOrders(ctx context.Context, mode, symbol string) ([]Order, error) {
	if !c.configured() {
		return nil, errors.New("alpaca client not configured")
	}
	escapedSymbol := url.QueryEscape(strings.TrimSpace(symbol))
	body, err := c.do(ctx, http.MethodGet, mode, "/v2/orders?status=open&symbols="+escapedSymbol, nil)
	if err != nil {
		return nil, err
	}
	var orders []Order
	if err := json.Unmarshal(body, &orders); err != nil {
		return nil, fmt.Errorf("decode alpaca open orders: %w", err)
	}
	return orders, nil
}

func (c *Client) CancelOrder(ctx context.Context, mode, orderID string) error {
	if !c.configured() {
		return errors.New("alpaca client not configured")
	}
	escaped := url.PathEscape(strings.TrimSpace(orderID))
	_, err := c.do(ctx, http.MethodDelete, mode, "/v2/orders/"+escaped, nil)
	return err
}

func (c *Client) GetOrder(ctx context.Context, mode, orderID string) (Order, error) {
	if !c.configured() {
		return Order{}, errors.New("alpaca client not configured")
	}
	escaped := url.PathEscape(strings.TrimSpace(orderID))
	body, err := c.do(ctx, http.MethodGet, mode, "/v2/orders/"+escaped, nil)
	if err != nil {
		return Order{}, err
	}
	var order Order
	if err := json.Unmarshal(body, &order); err != nil {
		return Order{}, fmt.Errorf("decode alpaca order: %w", err)
	}
	return order, nil
}

func (c *Client) ClosePosition(ctx context.Context, mode, symbol string) (Order, error) {
	if !c.configured() {
		return Order{}, errors.New("alpaca client not configured")
	}
	escaped := url.PathEscape(strings.TrimSpace(symbol))
	body, err := c.do(ctx, http.MethodDelete, mode, "/v2/positions/"+escaped, nil)
	if err != nil {
		return Order{}, err
	}
	var order Order
	if err := json.Unmarshal(body, &order); err != nil {
		return Order{}, fmt.Errorf("decode alpaca close position response: %w", err)
	}
	return order, nil
}

func (c *Client) do(ctx context.Context, method, mode, path string, payload any) ([]byte, error) {
	baseURL, err := c.baseURL(mode)
	if err != nil {
		return nil, err
	}
	var reader io.Reader
	if payload != nil {
		buf, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("encode alpaca payload: %w", err)
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("APCA-API-KEY-ID", c.apiKeyID)
	req.Header.Set("APCA-API-SECRET-KEY", c.secretKey)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = resp.Status
		}
		return nil, fmt.Errorf("alpaca %s %s failed: %s", method, path, message)
	}
	return body, nil
}

func (c *Client) baseURL(mode string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(mode))
	switch normalized {
	case "paper", "":
		if c.paperURL == "" {
			return "", errors.New("alpaca paper trading url not configured")
		}
		return c.paperURL, nil
	case "live":
		if c.liveURL == "" {
			return "", errors.New("alpaca live trading url not configured")
		}
		return c.liveURL, nil
	default:
		return "", fmt.Errorf("unsupported alpaca mode %q", mode)
	}
}
