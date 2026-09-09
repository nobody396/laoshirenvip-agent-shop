package gmshop

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

type Client struct {
	endpoint, token string
	httpClient      *http.Client
}

func New(endpoint, token string) *Client {
	return &Client{endpoint: strings.TrimSpace(endpoint), token: strings.TrimSpace(token), httpClient: &http.Client{Timeout: 10 * time.Second}}
}

func (c *Client) Lookup(ctx context.Context, orderNo, email string) (money.Amount, error) {
	parsed, err := url.Parse(c.endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || c.token == "" {
		return money.Amount{}, errors.New("gmshop invoice lookup unavailable")
	}
	payload, _ := json.Marshal(map[string]string{"orderNumber": strings.TrimSpace(orderNo), "email": strings.ToLower(strings.TrimSpace(email))})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(payload))
	if err != nil {
		return money.Amount{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return money.Amount{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return money.Amount{}, errors.New("gmshop order not eligible")
	}
	var result struct {
		AmountMinor, Currency string
		CurrencyDecimals      int
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return money.Amount{}, err
	}
	minor, err := decimal.NewFromString(result.AmountMinor)
	if err != nil || result.Currency != "CNY" || result.CurrencyDecimals != 2 || !minor.IsPositive() {
		return money.Amount{}, errors.New("gmshop order amount invalid")
	}
	return money.FromDecimal(minor.Shift(-2)), nil
}
