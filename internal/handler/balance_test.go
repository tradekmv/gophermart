package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/tradekmv/gophermart.git/internal/middleware"
	"github.com/tradekmv/gophermart.git/internal/model"
	"github.com/tradekmv/gophermart.git/internal/service"
)

type MockBalanceService struct {
	balance     model.BalanceResponse
	balanceErr  error
	withdrawErr error
}

func (m *MockBalanceService) GetBalance(ctx context.Context, userID string) (model.BalanceResponse, error) {
	return m.balance, m.balanceErr
}

func (m *MockBalanceService) Withdraw(ctx context.Context, userID, orderNumber string, sum float64) error {
	return m.withdrawErr
}

func (m *MockBalanceService) GetWithdrawals(ctx context.Context, userID string) ([]model.WithdrawalResponse, error) {
	return []model.WithdrawalResponse{}, nil
}

func TestBalanceHandler_GetBalance_Unauthorized(t *testing.T) {
	logger := zerolog.Nop()
	req := httptest.NewRequest(http.MethodGet, "/api/user/balance", nil)
	rr := httptest.NewRecorder()
	h := &BalanceHandler{service: nil, logger: &logger}
	h.GetBalance(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestBalanceHandler_GetBalance_Success(t *testing.T) {
	logger := zerolog.Nop()
	req := httptest.NewRequest(http.MethodGet, "/api/user/balance", nil)
	ctx := context.WithValue(req.Context(), middleware.ContextKey("userID"), "user-123")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	mockService := &MockBalanceService{
		balance: model.BalanceResponse{Current: 500.50, Withdrawn: 100.00},
	}
	h := &BalanceHandler{service: mockService, logger: &logger}
	h.GetBalance(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rr.Code)
	}
	var balance model.BalanceResponse
	json.Unmarshal(rr.Body.Bytes(), &balance)
	if balance.Current != 500.50 {
		t.Errorf("expected 500.50, got %f", balance.Current)
	}
}

func TestBalanceHandler_GetBalance_InternalError(t *testing.T) {
	logger := zerolog.Nop()
	req := httptest.NewRequest(http.MethodGet, "/api/user/balance", nil)
	ctx := context.WithValue(req.Context(), middleware.ContextKey("userID"), "user-123")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	mockService := &MockBalanceService{balanceErr: errors.New("db error")}
	h := &BalanceHandler{service: mockService, logger: &logger}
	h.GetBalance(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected %d, got %d", http.StatusInternalServerError, rr.Code)
	}
}

func TestBalanceHandler_Withdraw_InvalidContentType(t *testing.T) {
	logger := zerolog.Nop()
	reqBody := model.WithdrawRequest{Order: "79927398713", Sum: 100}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "text/plain")
	ctx := context.WithValue(req.Context(), middleware.ContextKey("userID"), "user-123")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	h := &BalanceHandler{service: nil, logger: &logger}
	h.Withdraw(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestBalanceHandler_Withdraw_InvalidJSON(t *testing.T) {
	logger := zerolog.Nop()
	req := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), middleware.ContextKey("userID"), "user-123")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	h := &BalanceHandler{service: nil, logger: &logger}
	h.Withdraw(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestBalanceHandler_Withdraw_Unauthorized(t *testing.T) {
	logger := zerolog.Nop()
	reqBody := model.WithdrawRequest{Order: "79927398713", Sum: 100}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h := &BalanceHandler{service: nil, logger: &logger}
	h.Withdraw(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestBalanceHandler_Withdraw_MissingOrder(t *testing.T) {
	logger := zerolog.Nop()
	reqBody := model.WithdrawRequest{Order: "", Sum: 100}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), middleware.ContextKey("userID"), "user-123")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	h := &BalanceHandler{service: nil, logger: &logger}
	h.Withdraw(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestBalanceHandler_Withdraw_InvalidSum(t *testing.T) {
	logger := zerolog.Nop()
	reqBody := model.WithdrawRequest{Order: "79927398713", Sum: 0}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), middleware.ContextKey("userID"), "user-123")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	h := &BalanceHandler{service: nil, logger: &logger}
	h.Withdraw(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestBalanceHandler_Withdraw_NegativeSum(t *testing.T) {
	logger := zerolog.Nop()
	reqBody := model.WithdrawRequest{Order: "79927398713", Sum: -100}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), middleware.ContextKey("userID"), "user-123")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	h := &BalanceHandler{service: nil, logger: &logger}
	h.Withdraw(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestBalanceHandler_Withdraw_Success(t *testing.T) {
	logger := zerolog.Nop()
	reqBody := model.WithdrawRequest{Order: "79927398713", Sum: 100}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), middleware.ContextKey("userID"), "user-123")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	mockService := &MockBalanceService{withdrawErr: nil}
	h := &BalanceHandler{service: mockService, logger: &logger}
	h.Withdraw(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestBalanceHandler_Withdraw_InvalidOrderNumber(t *testing.T) {
	logger := zerolog.Nop()
	reqBody := model.WithdrawRequest{Order: "1234567890123456", Sum: 100}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), middleware.ContextKey("userID"), "user-123")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	mockService := &MockBalanceService{withdrawErr: service.ErrInvalidOrderNumber}
	h := &BalanceHandler{service: mockService, logger: &logger}
	h.Withdraw(rr, req)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected %d, got %d", http.StatusUnprocessableEntity, rr.Code)
	}
}

func TestBalanceHandler_Withdraw_InsufficientFunds(t *testing.T) {
	logger := zerolog.Nop()
	reqBody := model.WithdrawRequest{Order: "79927398713", Sum: 1000}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), middleware.ContextKey("userID"), "user-123")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	mockService := &MockBalanceService{withdrawErr: service.ErrInsufficientFunds}
	h := &BalanceHandler{service: mockService, logger: &logger}
	h.Withdraw(rr, req)
	if rr.Code != http.StatusPaymentRequired {
		t.Errorf("expected %d, got %d", http.StatusPaymentRequired, rr.Code)
	}
}

func TestBalanceHandler_Withdraw_InternalError(t *testing.T) {
	logger := zerolog.Nop()
	reqBody := model.WithdrawRequest{Order: "79927398713", Sum: 100}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), middleware.ContextKey("userID"), "user-123")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	mockService := &MockBalanceService{withdrawErr: errors.New("db error")}
	h := &BalanceHandler{service: mockService, logger: &logger}
	h.Withdraw(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected %d, got %d", http.StatusInternalServerError, rr.Code)
	}
}
