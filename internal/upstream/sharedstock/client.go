package sharedstock

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	legacyPrefix    = "/plugin/SharedStock/api"
	responseMaxSize = 2 << 20
)

var (
	ErrRequestUncertain = errors.New("sharedstock request outcome is uncertain")
	ErrUnsupported      = errors.New("sharedstock operation is unsupported")
)

var coreActions = map[string]string{
	"connect":   "/shared/authentication/connect",
	"items":     "/shared/commodity/items",
	"item":      "/shared/commodity/item",
	"inventory": "/shared/commodity/inventory",
	"trade":     "/shared/commodity/trade",
	"query":     "/shared/commodity/query",
}

type ResponseError struct {
	Status  int
	Code    string
	Message string
}

func (e *ResponseError) Error() string {
	return fmt.Sprintf("sharedstock response status=%d code=%s: %s", e.Status, e.Code, e.Message)
}

type Client struct {
	baseURL string
	appID   string
	appKey  string
	http    *http.Client

	mu          sync.RWMutex
	routeFamily string
}

type Option func(*Client)

func WithHTTPClient(client *http.Client) Option {
	return func(target *Client) {
		if client != nil {
			target.http = client
		}
	}
}

func NewClient(baseURL, appID, appKey string, options ...Option) *Client {
	client := &Client{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		appID:   strings.TrimSpace(appID),
		appKey:  appKey,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
	for _, option := range options {
		option(client)
	}
	return client
}

func (c *Client) Connect(ctx context.Context) (*Connection, error) {
	var result Connection
	if err := c.request(ctx, "connect", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) Items(ctx context.Context) ([]Category, error) {
	var result []Category
	if err := c.request(ctx, "items", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) Item(ctx context.Context, code string) (*Commodity, error) {
	var result Commodity
	if err := c.request(ctx, "item", map[string]string{"code": code}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) Inventory(ctx context.Context, code, race string) (*Inventory, error) {
	values := map[string]string{"sharedCode": code}
	if race != "" {
		values["race"] = race
	}
	var result Inventory
	if err := c.request(ctx, "inventory", values, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) Trade(ctx context.Context, input TradeRequest) (*Trade, error) {
	if input.Quantity < 1 || strings.TrimSpace(input.SharedCode) == "" || strings.TrimSpace(input.RequestNo) == "" {
		return nil, fmt.Errorf("sharedstock trade: invalid request")
	}
	values := map[string]string{
		"shared_code": input.SharedCode,
		"num":         strconv.Itoa(input.Quantity),
		"contact":     input.RequestNo,
		"device":      "0",
		"request_no":  input.RequestNo,
	}
	if input.Race != "" {
		values["race"] = input.Race
	}
	var result Trade
	if err := c.request(ctx, "trade", values, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) Query(ctx context.Context, tradeNo string) (*Order, error) {
	var result Order
	if err := c.request(ctx, "query", map[string]string{"tradeNo": tradeNo}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) request(ctx context.Context, action string, values map[string]string, target any) error {
	corePath, ok := coreActions[action]
	if !ok {
		return fmt.Errorf("%w: %s", ErrUnsupported, action)
	}
	paths := c.candidatePaths(corePath, legacyPrefix+"/"+action)
	payload := make(map[string]string, len(values)+2)
	for key, value := range values {
		payload[key] = value
	}
	payload["app_id"] = c.appID
	payload["sign"] = Sign(payload, c.appKey)

	var invalidJSON error
	for index, path := range paths {
		data, status, err := c.post(ctx, path, payload)
		if err != nil {
			return err
		}
		var envelope struct {
			Code ScalarString    `json:"code"`
			Msg  string          `json:"msg"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(data, &envelope); err != nil {
			invalidJSON = fmt.Errorf("sharedstock invalid JSON: %w", err)
			if index < len(paths)-1 {
				continue
			}
			return invalidJSON
		}
		c.rememberRouteFamily(path == corePath)
		if status != http.StatusOK || string(envelope.Code) != "200" {
			if strings.Contains(strings.ToLower(envelope.Msg), "already exists") {
				return ErrRequestUncertain
			}
			return &ResponseError{Status: status, Code: string(envelope.Code), Message: envelope.Msg}
		}
		if target == nil || len(envelope.Data) == 0 || string(envelope.Data) == "null" {
			return nil
		}
		if err := json.Unmarshal(envelope.Data, target); err != nil {
			return fmt.Errorf("sharedstock decode data: %w", err)
		}
		return nil
	}
	return invalidJSON
}

func (c *Client) candidatePaths(core, legacy string) []string {
	c.mu.RLock()
	family := c.routeFamily
	c.mu.RUnlock()
	switch family {
	case "core":
		return []string{core}
	case "legacy":
		return []string{legacy}
	default:
		return []string{core, legacy}
	}
}

func (c *Client) rememberRouteFamily(core bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if core {
		c.routeFamily = "core"
	} else {
		c.routeFamily = "legacy"
	}
}

func (c *Client) post(ctx context.Context, path string, values map[string]string) ([]byte, int, error) {
	form := url.Values{}
	for key, value := range values {
		form.Set(key, value)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, 0, fmt.Errorf("sharedstock request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := c.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("sharedstock send: %w", err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, responseMaxSize+1))
	if err != nil {
		return nil, response.StatusCode, fmt.Errorf("sharedstock read: %w", err)
	}
	if len(data) > responseMaxSize {
		return nil, response.StatusCode, fmt.Errorf("sharedstock response exceeds %d bytes", responseMaxSize)
	}
	return data, response.StatusCode, nil
}
