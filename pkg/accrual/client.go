// Package accrual provides a client for the accrual system API.
package accrual

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

var (
	// ErrOrderNotFound is returned when order is not found in accrual system.
	ErrOrderNotFound = errors.New("order not found")
	// ErrRateLimited is returned when rate limited by accrual system.
	ErrRateLimited = errors.New("rate limited")
)

const (
	maxRetries     = 3
	retryDelayBase = 1 * time.Second
)

// AccrualResponse represents response from accrual system.
type AccrualResponse struct {
	Order   string   `json:"order"`
	Status  string   `json:"status"`
	Accrual *float64 `json:"accrual"`
}

// Client provides methods to interact with the accrual system.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new accrual system client.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Fetch retrieves accrual information for an order.
func (c *Client) Fetch(ctx context.Context, orderNumber string) (AccrualResponse, error) {
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			delay := retryDelayBase * time.Duration(1<<uint(attempt-1))
			select {
			case <-ctx.Done():
				return AccrualResponse{}, ctx.Err()
			case <-time.After(delay):
			}
		}

		resp, err := c.httpClient.Get(fmt.Sprintf("%s/api/orders/%s", c.baseURL, orderNumber))
		if err != nil {
			lastErr = err
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		switch resp.StatusCode {
		case http.StatusOK:
			var result AccrualResponse
			if err := json.Unmarshal(body, &result); err != nil {
				lastErr = err
				continue
			}
			return result, nil
		case http.StatusNoContent:
			return AccrualResponse{}, ErrOrderNotFound
		case http.StatusTooManyRequests:
			// Parse Retry-After header
			if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
				if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds > 0 {
					select {
					case <-ctx.Done():
						return AccrualResponse{}, ctx.Err()
					case <-time.After(time.Duration(seconds) * time.Second):
					}
				}
			}
			lastErr = ErrRateLimited
			continue
		default:
			return AccrualResponse{}, fmt.Errorf("unexpected status: %d, body: %s", resp.StatusCode, string(body))
		}
	}

	return AccrualResponse{}, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// FetchInfo retrieves accrual information as AccrualInfo.
func (c *Client) FetchInfo(ctx context.Context, orderNumber string) (AccrualInfo, error) {
	resp, err := c.Fetch(ctx, orderNumber)
	if err != nil {
		return AccrualInfo{}, err
	}
	var accrual float64
	if resp.Accrual != nil {
		accrual = *resp.Accrual
	}
	return AccrualInfo{
		Status:  resp.Status,
		Accrual: accrual,
	}, nil
}

// AccrualInfo is the internal representation of accrual data.
type AccrualInfo struct {
	Status  string
	Accrual float64
}
