// Package handler provides HTTP handlers for the gophermart service.
package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog"
	"github.com/tradekmv/gophermart.git/internal/middleware"
	"github.com/tradekmv/gophermart.git/internal/model"
)

// WithdrawalServiceInterface defines the minimal interface for withdrawal operations.
type WithdrawalServiceInterface interface {
	GetWithdrawals(ctx context.Context, userID string) ([]model.WithdrawalResponse, error)
}

// WithdrawalHandler handles withdrawal-related HTTP requests.
type WithdrawalHandler struct {
	service WithdrawalServiceInterface
	logger  *zerolog.Logger
}

// NewWithdrawalHandler creates a new WithdrawalHandler.
// Принимает интерфейс WithdrawalServiceInterface для возможности подмены в тестах
// (принцип "accept interfaces, return structures").
func NewWithdrawalHandler(svc WithdrawalServiceInterface, logger *zerolog.Logger) *WithdrawalHandler {
	return &WithdrawalHandler{
		service: svc,
		logger:  logger,
	}
}

// GetWithdrawals returns all withdrawals for the authenticated user.
func (h *WithdrawalHandler) GetWithdrawals(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	withdrawals, err := h.service.GetWithdrawals(r.Context(), userID)
	if err != nil {
		h.logger.Error().Err(err).Msg("get withdrawals")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if len(withdrawals) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(withdrawals); err != nil {
		h.logger.Error().Err(err).Msg("encode withdrawals")
	}
}
