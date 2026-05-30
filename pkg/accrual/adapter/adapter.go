// Package adapter provides an adapter for the accrual client to implement service.AccrualClient.
package adapter

import (
	"context"

	"github.com/tradekmv/gophermart.git/internal/service"
	"github.com/tradekmv/gophermart.git/pkg/accrual"
)

// Client wraps accrual.Client to implement service.AccrualClient.
type Client struct {
	client *accrual.Client
}

// NewClient creates a new adapter client.
func NewClient(client *accrual.Client) *Client {
	return &Client{client: client}
}

// Fetch retrieves accrual information for an order.
func (c *Client) Fetch(ctx context.Context, orderNumber string) (service.AccrualInfo, error) {
	resp, err := c.client.Fetch(ctx, orderNumber)
	if err != nil {
		return service.AccrualInfo{}, err
	}
	var accrual float64
	if resp.Accrual != nil {
		accrual = *resp.Accrual
	}
	return service.AccrualInfo{
		Status:  resp.Status,
		Accrual: accrual,
	}, nil
}
