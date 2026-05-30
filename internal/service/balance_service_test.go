package service

import (
	"context"
	"errors"
	"testing"

	"github.com/tradekmv/gophermart.git/internal/model"
	"github.com/tradekmv/gophermart.git/internal/repository/storage"
)

// MockBalanceStorage implements storage.Storage for balance tests
type MockBalanceStorage struct {
	balances      map[string]struct{ current, withdrawn float64 }
	withdrawals   []storage.Withdrawal
	getBalanceErr error
	withdrawErr   error
}

func NewMockBalanceStorage() *MockBalanceStorage {
	return &MockBalanceStorage{
		balances:    make(map[string]struct{ current, withdrawn float64 }),
		withdrawals: make([]storage.Withdrawal, 0),
	}
}

func (m *MockBalanceStorage) CreateUserWithBalance(ctx context.Context, login, passwordHash string) (string, error) {
	return "user-id-" + login, nil
}

func (m *MockBalanceStorage) GetUserByLogin(ctx context.Context, login string) (storage.User, error) {
	return storage.User{}, storage.ErrNotFound
}

func (m *MockBalanceStorage) SaveOrder(ctx context.Context, userID, orderNumber string) (storage.Order, error) {
	return storage.Order{}, nil
}

func (m *MockBalanceStorage) GetOrderByNumber(ctx context.Context, orderNumber string) (storage.Order, error) {
	return storage.Order{}, storage.ErrNotFound
}

func (m *MockBalanceStorage) GetOrdersByUserID(ctx context.Context, userID string) ([]storage.Order, error) {
	return []storage.Order{}, nil
}

func (m *MockBalanceStorage) GetPendingOrders(ctx context.Context) ([]storage.Order, error) {
	return []storage.Order{}, nil
}

func (m *MockBalanceStorage) UpdateOrderByNumber(ctx context.Context, number, status string, accrual *float64) error {
	return nil
}

func (m *MockBalanceStorage) AddAccrual(ctx context.Context, userID, orderNumber string, accrual float64) error {
	return nil
}

func (m *MockBalanceStorage) GetBalance(ctx context.Context, userID string) (float64, float64, error) {
	if m.getBalanceErr != nil {
		return 0, 0, m.getBalanceErr
	}
	if bal, ok := m.balances[userID]; ok {
		return bal.current, bal.withdrawn, nil
	}
	return 0, 0, nil
}

func (m *MockBalanceStorage) Withdraw(ctx context.Context, userID, order string, sum float64) error {
	if m.withdrawErr != nil {
		return m.withdrawErr
	}
	m.withdrawals = append(m.withdrawals, storage.Withdrawal{
		ID:     "withdrawal-id",
		UserID: userID,
		Order:  order,
		Sum:    sum,
	})
	return nil
}

func (m *MockBalanceStorage) GetWithdrawals(ctx context.Context, userID string) ([]storage.Withdrawal, error) {
	return m.withdrawals, nil
}

func (m *MockBalanceStorage) Close() error {
	return nil
}

func TestBalanceService_GetBalance(t *testing.T) {
	tests := []struct {
		name      string
		userID    string
		wantCur   float64
		wantWith  float64
		setupMock func(*MockBalanceStorage)
	}{
		{
			name:      "zero balance",
			userID:    "user-123",
			wantCur:   0,
			wantWith:  0,
			setupMock: func(m *MockBalanceStorage) {},
		},
		{
			name:     "with balance",
			userID:   "user-123",
			wantCur:  500.50,
			wantWith: 100.00,
			setupMock: func(m *MockBalanceStorage) {
				m.balances["user-123"] = struct{ current, withdrawn float64 }{500.50, 100.00}
			},
		},
		{
			name:     "large balance",
			userID:   "user-123",
			wantCur:  999999.99,
			wantWith: 0,
			setupMock: func(m *MockBalanceStorage) {
				m.balances["user-123"] = struct{ current, withdrawn float64 }{999999.99, 0}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockBalanceStorage()
			tt.setupMock(mock)

			svc := NewBalanceService(mock)
			balance, err := svc.GetBalance(context.Background(), tt.userID)

			if err != nil {
				t.Errorf("GetBalance() unexpected error = %v", err)
				return
			}

			if balance.Current != tt.wantCur {
				t.Errorf("GetBalance() current = %v, want %v", balance.Current, tt.wantCur)
			}
			if balance.Withdrawn != tt.wantWith {
				t.Errorf("GetBalance() withdrawn = %v, want %v", balance.Withdrawn, tt.wantWith)
			}
		})
	}
}

func TestBalanceService_Withdraw(t *testing.T) {
	tests := []struct {
		name      string
		userID    string
		orderNum  string
		sum       float64
		wantErr   error
		setupMock func(*MockBalanceStorage)
	}{
		{
			name:      "successful withdrawal",
			userID:    "user-123",
			orderNum:  "79927398713",
			sum:       100,
			wantErr:   nil,
			setupMock: func(m *MockBalanceStorage) {},
		},
		{
			name:      "invalid order number - luhn",
			userID:    "user-123",
			orderNum:  "1234567890123456",
			sum:       100,
			wantErr:   ErrInvalidOrderNumber,
			setupMock: func(m *MockBalanceStorage) {},
		},
		{
			name:      "zero sum",
			userID:    "user-123",
			orderNum:  "79927398713",
			sum:       0,
			wantErr:   ErrInvalidOrderNumber,
			setupMock: func(m *MockBalanceStorage) {},
		},
		{
			name:      "negative sum",
			userID:    "user-123",
			orderNum:  "79927398713",
			sum:       -50,
			wantErr:   ErrInvalidOrderNumber,
			setupMock: func(m *MockBalanceStorage) {},
		},
		{
			name:     "insufficient funds",
			userID:   "user-123",
			orderNum: "79927398713",
			sum:      100,
			wantErr:  ErrInsufficientFunds,
			setupMock: func(m *MockBalanceStorage) {
				m.withdrawErr = storage.ErrInsufficientFunds
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockBalanceStorage()
			tt.setupMock(mock)

			svc := NewBalanceService(mock)
			err := svc.Withdraw(context.Background(), tt.userID, tt.orderNum, tt.sum)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("Withdraw() error = %v, wantErr %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Errorf("Withdraw() unexpected error = %v", err)
			}
		})
	}
}

func TestBalanceService_GetWithdrawals(t *testing.T) {
	tests := []struct {
		name      string
		userID    string
		wantLen   int
		setupMock func(*MockBalanceStorage)
	}{
		{
			name:      "no withdrawals",
			userID:    "user-123",
			wantLen:   0,
			setupMock: func(m *MockBalanceStorage) {},
		},
		{
			name:    "one withdrawal",
			userID:  "user-123",
			wantLen: 1,
			setupMock: func(m *MockBalanceStorage) {
				m.withdrawals = append(m.withdrawals, storage.Withdrawal{
					ID:     "w1",
					UserID: "user-123",
					Order:  "79927398713",
					Sum:    100,
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockBalanceStorage()
			tt.setupMock(mock)

			svc := NewBalanceService(mock)
			withdrawals, err := svc.GetWithdrawals(context.Background(), tt.userID)

			if err != nil {
				t.Errorf("GetWithdrawals() unexpected error = %v", err)
				return
			}

			if len(withdrawals) != tt.wantLen {
				t.Errorf("GetWithdrawals() got %d withdrawals, want %d", len(withdrawals), tt.wantLen)
			}
		})
	}
}

func TestBalanceResponse_Structure(t *testing.T) {
	resp := model.BalanceResponse{
		Current:   500.50,
		Withdrawn: 100.25,
	}

	if resp.Current != 500.50 {
		t.Error("BalanceResponse.Current mismatch")
	}
	if resp.Withdrawn != 100.25 {
		t.Error("BalanceResponse.Withdrawn mismatch")
	}
}

func TestWithdrawalResponse_Structure(t *testing.T) {
	resp := model.WithdrawalResponse{
		Order: "79927398713",
		Sum:   150.75,
	}

	if resp.Order != "79927398713" {
		t.Error("WithdrawalResponse.Order mismatch")
	}
	if resp.Sum != 150.75 {
		t.Error("WithdrawalResponse.Sum mismatch")
	}
}
