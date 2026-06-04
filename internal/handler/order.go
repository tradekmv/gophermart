// Package handler provides HTTP handlers for the gophermart service.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/rs/zerolog"
	"github.com/tradekmv/gophermart.git/internal/middleware"
	"github.com/tradekmv/gophermart.git/internal/model"
	"github.com/tradekmv/gophermart.git/internal/service"
)

// OrderServiceInterface defines the interface for order operations
type OrderServiceInterface interface {
	UploadOrder(ctx context.Context, userID, orderNumber string) error
	GetOrders(ctx context.Context, userID string) ([]model.OrderResponse, error)
}

// OrderHandler handles order-related HTTP requests.
type OrderHandler struct {
	service OrderServiceInterface
	logger  *zerolog.Logger
}

// NewOrderHandler creates a new OrderHandler.
// Принимает интерфейс OrderServiceInterface для возможности подмены в тестах
// (принцип "accept interfaces, return structures").
func NewOrderHandler(svc OrderServiceInterface, logger *zerolog.Logger) *OrderHandler {
	return &OrderHandler{
		service: svc,
		logger:  logger,
	}
}

// UploadOrder handles order upload (text/plain body).
func (h *OrderHandler) UploadOrder(w http.ResponseWriter, r *http.Request) {
	// Check Content-Type (text/plain)
	ct := r.Header.Get("Content-Type")
	if ct != "" && !strings.HasPrefix(ct, "text/plain") {
		http.Error(w, "Content-Type must be text/plain", http.StatusBadRequest)
		return
	}

	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}
	orderNumber := strings.TrimSpace(string(body))
	if orderNumber == "" {
		http.Error(w, "empty order number", http.StatusBadRequest)
		return
	}

	// Get user ID from context
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Upload order
	err = h.service.UploadOrder(r.Context(), userID, orderNumber)
	if err != nil {
		if errors.Is(err, service.ErrOrderAlreadyUploaded) {
			// 200 - Already uploaded by this user
			w.WriteHeader(http.StatusOK)
			return
		}
		if errors.Is(err, service.ErrOrderByAnotherUser) {
			http.Error(w, "order belongs to another user", http.StatusConflict)
			return
		}
		if errors.Is(err, service.ErrInvalidOrderNumber) {
			http.Error(w, "invalid order number", http.StatusUnprocessableEntity)
			return
		}
		h.logger.Error().Err(err).Msg("upload order")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// 202 - New order accepted
	w.WriteHeader(http.StatusAccepted)
}

// GetOrders returns all orders for the authenticated user.
func (h *OrderHandler) GetOrders(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Get orders
	orders, err := h.service.GetOrders(r.Context(), userID)
	if err != nil {
		h.logger.Error().Err(err).Msg("get orders")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Empty list returns 204
	if len(orders) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Return orders as JSON
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(orders); err != nil {
		h.logger.Error().Err(err).Msg("encode orders")
	}
}
