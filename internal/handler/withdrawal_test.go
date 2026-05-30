package handler

import (
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
)

type MockBalanceServiceForWithdrawals struct {
	withdrawals []model.WithdrawalResponse
	getWErr     error
}

func (m *MockBalanceServiceForWithdrawals) GetBalance(ctx context.Context, userID string) (model.BalanceResponse, error) {
	return model.BalanceResponse{}, nil
}

func (m *MockBalanceServiceForWithdrawals) Withdraw(ctx context.Context, userID, orderNumber string, sum float64) error {
	return nil
}

func (m *MockBalanceServiceForWithdrawals) GetWithdrawals(ctx context.Context, userID string) ([]model.WithdrawalResponse, error) {
	return m.withdrawals, m.getWErr
}

func TestWithdrawalHandler_GetWithdrawals_Unauthorized(t *testing.T) {
	logger := zerolog.Nop()
	req := httptest.NewRequest(http.MethodGet, "/api/user/withdrawals", nil)
	rr := httptest.NewRecorder()
	h := &WithdrawalHandler{service: nil, logger: &logger}
	h.GetWithdrawals(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestWithdrawalHandler_GetWithdrawals_Success(t *testing.T) {
	logger := zerolog.Nop()
	req := httptest.NewRequest(http.MethodGet, "/api/user/withdrawals", nil)
	ctx := context.WithValue(req.Context(), middleware.ContextKey("userID"), "user-123")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	now := time.Now()
	mockService := &MockBalanceServiceForWithdrawals{
		withdrawals: []model.WithdrawalResponse{
			{Order: "79927398713", Sum: 100.50, ProcessedAt: now},
			{Order: "79927398714", Sum: 200.00, ProcessedAt: now},
		},
	}
	h := &WithdrawalHandler{service: mockService, logger: &logger}
	h.GetWithdrawals(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rr.Code)
	}
	var withdrawals []model.WithdrawalResponse
	json.Unmarshal(rr.Body.Bytes(), &withdrawals)
	if len(withdrawals) != 2 {
		t.Errorf("expected 2, got %d", len(withdrawals))
	}
}

func TestWithdrawalHandler_GetWithdrawals_Empty(t *testing.T) {
	logger := zerolog.Nop()
	req := httptest.NewRequest(http.MethodGet, "/api/user/withdrawals", nil)
	ctx := context.WithValue(req.Context(), middleware.ContextKey("userID"), "user-123")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	mockService := &MockBalanceServiceForWithdrawals{withdrawals: []model.WithdrawalResponse{}}
	h := &WithdrawalHandler{service: mockService, logger: &logger}
	h.GetWithdrawals(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Errorf("expected %d, got %d", http.StatusNoContent, rr.Code)
	}
}

func TestWithdrawalHandler_GetWithdrawals_InternalError(t *testing.T) {
	logger := zerolog.Nop()
	req := httptest.NewRequest(http.MethodGet, "/api/user/withdrawals", nil)
	ctx := context.WithValue(req.Context(), middleware.ContextKey("userID"), "user-123")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	mockService := &MockBalanceServiceForWithdrawals{getWErr: errors.New("db error")}
	h := &WithdrawalHandler{service: mockService, logger: &logger}
	h.GetWithdrawals(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected %d, got %d", http.StatusInternalServerError, rr.Code)
	}
}
