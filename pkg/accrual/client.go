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
)

// RateLimitError is returned when accrual system answers 429 Too Many
// Requests. RetryAfter — пауза, которую сервер попросил подождать (0,
// если сервер не вернул заголовок Retry-After).
//
// Потребитель может сделать errors.As(err, &rle) и получить конкретное
// значение retry-after, чтобы скорректировать собственный backoff.
type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limited, retry after %s", e.RetryAfter)
}

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

// AccrualInfo is the internal representation of accrual data.
//
// Defined in pkg/accrual, not in internal/service, to avoid an import
// cycle between the service layer and the client package.
type AccrualInfo struct {
	Status  string
	Accrual float64
}

// ClientInterface — контракт клиента системы начислений, который
// использует internal/service/AccrualService. Описан в pkg/accrual,
// чтобы адаптер был не нужен: любой, кто реализует Fetch, может
// удовлетворять потребителю.
//
//go:generate mockgen -destination=mock/client.go -package=mock github.com/tradekmv/gophermart.git/pkg/accrual ClientInterface
type ClientInterface interface {
	Fetch(ctx context.Context, orderNumber string) (AccrualInfo, error)
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

// Fetch implements ClientInterface.
func (c *Client) Fetch(ctx context.Context, orderNumber string) (AccrualInfo, error) {
	resp, err := c.FetchRaw(ctx, orderNumber)
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

// FetchRaw retrieves accrual information as AccrualResponse.
// Возвращает типизированный ответ (для unit-тестов), используется внутри
// Client.Fetch. Не входит в ClientInterface.
func (c *Client) FetchRaw(ctx context.Context, orderNumber string) (AccrualResponse, error) {
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff. Используем time.NewTimer вместо time.After,
			// чтобы при отмене контекста таймер не удерживался в heap до срабатывания
			// (см. https://github.com/golang/go/issues/37196).
			delay := retryDelayBase * time.Duration(1<<uint(attempt-1))
			t := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				t.Stop()
				return AccrualResponse{}, ctx.Err()
			case <-t.C:
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet,
			fmt.Sprintf("%s/api/orders/%s", c.baseURL, orderNumber), nil)
		if err != nil {
			return AccrualResponse{}, fmt.Errorf("build request: %w", err)
		}

		resp, err := c.httpClient.Do(req)
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
			// Parse Retry-After header. Если заголовок есть и валидный —
			// возвращаем *RateLimitError с конкретной паузой, чтобы потребитель
			// мог скорректировать backoff через errors.As.
			var retry time.Duration
			if h := resp.Header.Get("Retry-After"); h != "" {
				if seconds, err := strconv.Atoi(h); err == nil && seconds > 0 {
					retry = time.Duration(seconds) * time.Second
				}
			}
			t := time.NewTimer(retry)
			select {
			case <-ctx.Done():
				t.Stop()
				return AccrualResponse{}, ctx.Err()
			case <-t.C:
			}
			return AccrualResponse{}, &RateLimitError{RetryAfter: retry}
		default:
			return AccrualResponse{}, fmt.Errorf("unexpected status: %d, body: %s", resp.StatusCode, string(body))
		}
	}

	return AccrualResponse{}, fmt.Errorf("max retries exceeded: %w", lastErr)
}
