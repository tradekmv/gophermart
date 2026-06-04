// Package service implements business logic for the gophermart service.
package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/tradekmv/gophermart.git/internal/model"
	"github.com/tradekmv/gophermart.git/internal/repository/storage"
)

var (
	// ErrOrderAlreadyUploaded is returned when order is uploaded by the same user.
	ErrOrderAlreadyUploaded = errors.New("order already uploaded")
	// ErrOrderByAnotherUser is returned when order belongs to another user.
	ErrOrderByAnotherUser = errors.New("order belongs to another user")
	// ErrInvalidOrderNumber is returned when order number fails validation.
	ErrInvalidOrderNumber = errors.New("invalid order number")
)

// OrderService handles order-related business logic.
type OrderService struct {
	storage storage.Storage
}

// NewOrderService creates a new OrderService.
func NewOrderService(storage storage.Storage) *OrderService {
	return &OrderService{storage: storage}
}

// ValidateLuhn validates a number using the Luhn algorithm.
func ValidateLuhn(number string) bool {
	if len(number) == 0 {
		return false
	}

	sum := 0
	isSecond := false

	for i := len(number) - 1; i >= 0; i-- {
		d, err := strconv.Atoi(string(number[i]))
		if err != nil {
			return false
		}

		if isSecond {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}

		sum += d
		isSecond = !isSecond
	}

	return sum%10 == 0
}

// validateOrderNumber validates order number format and Luhn algorithm.
func validateOrderNumber(orderNumber string) error {
	// Check if empty
	if len(orderNumber) == 0 {
		return ErrInvalidOrderNumber
	}

	// Check if contains only digits
	for _, c := range orderNumber {
		if c < '0' || c > '9' {
			return ErrInvalidOrderNumber
		}
	}

	// Validate Luhn algorithm
	if !ValidateLuhn(orderNumber) {
		return ErrInvalidOrderNumber
	}

	return nil
}

// UploadOrder uploads a new order for the user.
func (s *OrderService) UploadOrder(ctx context.Context, userID, orderNumber string) error {
	// Validate order number
	orderNumber = strings.TrimSpace(orderNumber)
	if err := validateOrderNumber(orderNumber); err != nil {
		return err
	}

	// Check if order exists
	existingOrder, err := s.storage.GetOrderByNumber(ctx, orderNumber)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return fmt.Errorf("check order: %w", err)
	}

	// If order exists, check ownership
	if err == nil {
		if existingOrder.UserID == userID {
			return ErrOrderAlreadyUploaded
		}
		return ErrOrderByAnotherUser
	}

	// Save new order
	_, err = s.storage.SaveOrder(ctx, userID, orderNumber)
	if err != nil {
		if errors.Is(err, storage.ErrOrderExists) {
			// Double-check ownership
			existingOrder, getErr := s.storage.GetOrderByNumber(ctx, orderNumber)
			if getErr == nil {
				if existingOrder.UserID == userID {
					return ErrOrderAlreadyUploaded
				}
				return ErrOrderByAnotherUser
			}
			return ErrOrderAlreadyUploaded
		}
		return fmt.Errorf("save order: %w", err)
	}

	return nil
}

// GetOrders returns all orders for a user.
func (s *OrderService) GetOrders(ctx context.Context, userID string) ([]model.OrderResponse, error) {
	orders, err := s.storage.GetOrdersByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get orders: %w", err)
	}

	responses := make([]model.OrderResponse, 0, len(orders))
	for _, o := range orders {
		responses = append(responses, model.OrderResponse{
			Number:     o.Number,
			Status:     model.OrderStatus(o.Status),
			Accrual:    o.Accrual,
			UploadedAt: o.UploadedAt,
		})
	}

	return responses, nil
}
