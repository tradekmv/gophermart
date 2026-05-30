package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/tradekmv/gophermart.git/internal/middleware"
	"github.com/tradekmv/gophermart.git/internal/model"
	"github.com/tradekmv/gophermart.git/internal/service"
)

type MockOrderService struct {
	uploadErr error
	orders    []model.OrderResponse
	getErr    error
}

func (m *MockOrderService) UploadOrder(ctx context.Context, userID, orderNumber string) error {
	return m.uploadErr
}

func (m *MockOrderService) GetOrders(ctx context.Context, userID string) ([]model.OrderResponse, error) {
	return m.orders, m.getErr
}

func TestOrderHandler_UploadOrder_EmptyBody(t *testing.T) {
	logger := zerolog.Nop()
	req := httptest.NewRequest(http.MethodPost, "/api/user/orders", bytes.NewBufferString(""))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()
	h := &OrderHandler{service: nil, logger: &logger}
	h.UploadOrder(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestOrderHandler_UploadOrder_InvalidContentType(t *testing.T) {
	logger := zerolog.Nop()
	req := httptest.NewRequest(http.MethodPost, "/api/user/orders", bytes.NewBufferString("79927398713"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h := &OrderHandler{service: nil, logger: &logger}
	h.UploadOrder(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestOrderHandler_UploadOrder_Unauthorized(t *testing.T) {
	logger := zerolog.Nop()
	req := httptest.NewRequest(http.MethodPost, "/api/user/orders", bytes.NewBufferString("79927398713"))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()
	h := &OrderHandler{service: nil, logger: &logger}
	h.UploadOrder(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestOrderHandler_UploadOrder_Success(t *testing.T) {
	logger := zerolog.Nop()
	req := httptest.NewRequest(http.MethodPost, "/api/user/orders", bytes.NewBufferString("79927398713"))
	req.Header.Set("Content-Type", "text/plain")
	ctx := context.WithValue(req.Context(), middleware.ContextKey("userID"), "user-123")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	mockService := &MockOrderService{uploadErr: nil}
	h := &OrderHandler{service: mockService, logger: &logger}
	h.UploadOrder(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Errorf("expected %d, got %d", http.StatusAccepted, rr.Code)
	}
}

func TestOrderHandler_UploadOrder_AlreadyUploaded(t *testing.T) {
	logger := zerolog.Nop()
	req := httptest.NewRequest(http.MethodPost, "/api/user/orders", bytes.NewBufferString("79927398713"))
	req.Header.Set("Content-Type", "text/plain")
	ctx := context.WithValue(req.Context(), middleware.ContextKey("userID"), "user-123")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	mockService := &MockOrderService{uploadErr: service.ErrOrderAlreadyUploaded}
	h := &OrderHandler{service: mockService, logger: &logger}
	h.UploadOrder(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestOrderHandler_UploadOrder_AnotherUser(t *testing.T) {
	logger := zerolog.Nop()
	req := httptest.NewRequest(http.MethodPost, "/api/user/orders", bytes.NewBufferString("79927398713"))
	req.Header.Set("Content-Type", "text/plain")
	ctx := context.WithValue(req.Context(), middleware.ContextKey("userID"), "user-123")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	mockService := &MockOrderService{uploadErr: service.ErrOrderByAnotherUser}
	h := &OrderHandler{service: mockService, logger: &logger}
	h.UploadOrder(rr, req)
	if rr.Code != http.StatusConflict {
		t.Errorf("expected %d, got %d", http.StatusConflict, rr.Code)
	}
}

func TestOrderHandler_UploadOrder_InvalidNumber(t *testing.T) {
	logger := zerolog.Nop()
	req := httptest.NewRequest(http.MethodPost, "/api/user/orders", bytes.NewBufferString("1234567890123456"))
	req.Header.Set("Content-Type", "text/plain")
	ctx := context.WithValue(req.Context(), middleware.ContextKey("userID"), "user-123")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	mockService := &MockOrderService{uploadErr: service.ErrInvalidOrderNumber}
	h := &OrderHandler{service: mockService, logger: &logger}
	h.UploadOrder(rr, req)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected %d, got %d", http.StatusUnprocessableEntity, rr.Code)
	}
}

func TestOrderHandler_UploadOrder_InternalError(t *testing.T) {
	logger := zerolog.Nop()
	req := httptest.NewRequest(http.MethodPost, "/api/user/orders", bytes.NewBufferString("79927398713"))
	req.Header.Set("Content-Type", "text/plain")
	ctx := context.WithValue(req.Context(), middleware.ContextKey("userID"), "user-123")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	mockService := &MockOrderService{uploadErr: errors.New("internal error")}
	h := &OrderHandler{service: mockService, logger: &logger}
	h.UploadOrder(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected %d, got %d", http.StatusInternalServerError, rr.Code)
	}
}

func TestOrderHandler_GetOrders_Unauthorized(t *testing.T) {
	logger := zerolog.Nop()
	req := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)
	rr := httptest.NewRecorder()
	h := &OrderHandler{service: nil, logger: &logger}
	h.GetOrders(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestOrderHandler_GetOrders_Success(t *testing.T) {
	logger := zerolog.Nop()
	req := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)
	ctx := context.WithValue(req.Context(), middleware.ContextKey("userID"), "user-123")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	mockService := &MockOrderService{
		orders: []model.OrderResponse{
			{Number: "79927398713", Status: "NEW", UploadedAt: time.Now()},
		},
	}
	h := &OrderHandler{service: mockService, logger: &logger}
	h.GetOrders(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rr.Code)
	}
	var orders []model.OrderResponse
	json.Unmarshal(rr.Body.Bytes(), &orders)
	if len(orders) != 1 {
		t.Errorf("expected 1 order, got %d", len(orders))
	}
}

func TestOrderHandler_GetOrders_Empty(t *testing.T) {
	logger := zerolog.Nop()
	req := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)
	ctx := context.WithValue(req.Context(), middleware.ContextKey("userID"), "user-123")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	mockService := &MockOrderService{orders: []model.OrderResponse{}}
	h := &OrderHandler{service: mockService, logger: &logger}
	h.GetOrders(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Errorf("expected %d, got %d", http.StatusNoContent, rr.Code)
	}
}

func TestOrderHandler_GetOrders_InternalError(t *testing.T) {
	logger := zerolog.Nop()
	req := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)
	ctx := context.WithValue(req.Context(), middleware.ContextKey("userID"), "user-123")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	mockService := &MockOrderService{getErr: errors.New("db error")}
	h := &OrderHandler{service: mockService, logger: &logger}
	h.GetOrders(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected %d, got %d", http.StatusInternalServerError, rr.Code)
	}
}
