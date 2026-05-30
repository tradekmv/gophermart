// Package service implements business logic for the gophermart service.
package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/tradekmv/gophermart.git/internal/model"
	"github.com/tradekmv/gophermart.git/internal/repository/storage"
)

var (
	// ErrInsufficientFunds is returned when user has insufficient balance.
	ErrInsufficientFunds = errors.New("insufficient funds")
)

// BalanceService handles balance-related business logic.
type BalanceService struct {
	storage storage.Storage
}

// NewBalanceService creates a new BalanceService.
func NewBalanceService(storage storage.Storage) *BalanceService {
	return &BalanceService{storage: storage}
}

// GetBalance returns user's current balance and withdrawn amount.
func (s *BalanceService) GetBalance(ctx context.Context, userID string) (model.BalanceResponse, error) {
	current, withdrawn, err := s.storage.GetBalance(ctx, userID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			// Return zero balance if user has no balance record
			return model.BalanceResponse{Current: 0, Withdrawn: 0}, nil
		}
		return model.BalanceResponse{}, fmt.Errorf("get balance: %w", err)
	}

	return model.BalanceResponse{
		Current:   current,
		Withdrawn: withdrawn,
	}, nil
}

// Withdraw processes a withdrawal request.
func (s *BalanceService) Withdraw(ctx context.Context, userID, orderNumber string, sum float64) error {
	// Validate order number using Luhn algorithm
	if err := validateOrderNumber(orderNumber); err != nil {
		return err
	}

	// Validate sum
	if sum <= 0 {
		return ErrInvalidOrderNumber
	}

	// Process withdrawal
	err := s.storage.Withdraw(ctx, userID, orderNumber, sum)
	if err != nil {
		if errors.Is(err, storage.ErrInsufficientFunds) {
			return ErrInsufficientFunds
		}
		if errors.Is(err, storage.ErrNotFound) {
			return ErrInsufficientFunds
		}
		return fmt.Errorf("withdraw: %w", err)
	}

	return nil
}

// GetWithdrawals returns all withdrawals for a user.
func (s *BalanceService) GetWithdrawals(ctx context.Context, userID string) ([]model.WithdrawalResponse, error) {
	withdrawals, err := s.storage.GetWithdrawals(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get withdrawals: %w", err)
	}

	responses := make([]model.WithdrawalResponse, 0, len(withdrawals))
	for _, w := range withdrawals {
		responses = append(responses, model.WithdrawalResponse{
			Order:       w.Order,
			Sum:         w.Sum,
			ProcessedAt: w.ProcessedAt,
		})
	}

	return responses, nil
}
