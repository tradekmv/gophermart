// Package handler provides HTTP handlers for the gophermart service.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/rs/zerolog"
	"github.com/tradekmv/gophermart.git/internal/middleware"
	"github.com/tradekmv/gophermart.git/internal/model"
	"github.com/tradekmv/gophermart.git/internal/service"
)

// BalanceServiceInterface defines the interface for balance operations
type BalanceServiceInterface interface {
	GetBalance(ctx context.Context, userID string) (model.BalanceResponse, error)
	Withdraw(ctx context.Context, userID, orderNumber string, sum float64) error
	GetWithdrawals(ctx context.Context, userID string) ([]model.WithdrawalResponse, error)
}

// BalanceHandler handles balance-related HTTP requests.
type BalanceHandler struct {
	service BalanceServiceInterface
	logger  *zerolog.Logger
}

// NewBalanceHandler creates a new BalanceHandler.
// Принимает интерфейс BalanceServiceInterface для возможности подмены в тестах
// (принцип "accept interfaces, return structures").
func NewBalanceHandler(svc BalanceServiceInterface, logger *zerolog.Logger) *BalanceHandler {
	return &BalanceHandler{
		service: svc,
		logger:  logger,
	}
}

// GetBalance returns current balance for the authenticated user.
func (h *BalanceHandler) GetBalance(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Get balance
	balance, err := h.service.GetBalance(r.Context(), userID)
	if err != nil {
		h.logger.Error().Err(err).Msg("get balance")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Return balance as JSON
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(balance); err != nil {
		h.logger.Error().Err(err).Msg("encode balance")
	}
}

// Withdraw processes a withdrawal request.
func (h *BalanceHandler) Withdraw(w http.ResponseWriter, r *http.Request) {
	// Check Content-Type
	ct := r.Header.Get("Content-Type")
	if ct != "" && !strings.HasPrefix(ct, "application/json") {
		http.Error(w, "Content-Type must be application/json", http.StatusBadRequest)
		return
	}

	// Get user ID from context
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Decode request
	var req model.WithdrawRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Order == "" {
		http.Error(w, "order number required", http.StatusBadRequest)
		return
	}
	if req.Sum <= 0 {
		http.Error(w, "sum must be positive", http.StatusBadRequest)
		return
	}

	// Process withdrawal
	err := h.service.Withdraw(r.Context(), userID, req.Order, req.Sum)
	if err != nil {
		if errors.Is(err, service.ErrInvalidOrderNumber) {
			http.Error(w, "invalid order number", http.StatusUnprocessableEntity)
			return
		}
		if errors.Is(err, service.ErrInvalidAmount) {
			http.Error(w, "sum must be positive", http.StatusBadRequest)
			return
		}
		if errors.Is(err, service.ErrInsufficientFunds) {
			http.Error(w, "insufficient funds", http.StatusPaymentRequired)
			return
		}
		h.logger.Error().Err(err).Msg("withdraw")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
