package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tradekmv/gophermart.git/internal/repository/storage"
)

type MockOrderStorage struct {
	orders       map[string]storage.Order
	saveErr      error
	getOrderErr  error
	getOrdersErr error
}

func NewMockOrderStorage() *MockOrderStorage {
	return &MockOrderStorage{orders: make(map[string]storage.Order)}
}

func (m *MockOrderStorage) CreateUserWithBalance(ctx context.Context, login, passwordHash string) (string, error) {
	return "user-id", nil
}

func (m *MockOrderStorage) GetUserByLogin(ctx context.Context, login string) (storage.User, error) {
	return storage.User{}, storage.ErrNotFound
}

func (m *MockOrderStorage) SaveOrder(ctx context.Context, userID, orderNumber string) (storage.Order, error) {
	if m.saveErr != nil {
		return storage.Order{}, m.saveErr
	}
	order := storage.Order{ID: "order-" + orderNumber, UserID: userID, Number: orderNumber, Status: "NEW", UploadedAt: time.Now()}
	m.orders[orderNumber] = order
	return order, nil
}

func (m *MockOrderStorage) GetOrderByNumber(ctx context.Context, orderNumber string) (storage.Order, error) {
	if m.getOrderErr != nil {
		return storage.Order{}, m.getOrderErr
	}
	if order, ok := m.orders[orderNumber]; ok {
		return order, nil
	}
	return storage.Order{}, storage.ErrNotFound
}

func (m *MockOrderStorage) GetOrdersByUserID(ctx context.Context, userID string) ([]storage.Order, error) {
	if m.getOrdersErr != nil {
		return nil, m.getOrdersErr
	}
	var result []storage.Order
	for _, o := range m.orders {
		if o.UserID == userID {
			result = append(result, o)
		}
	}
	return result, nil
}

func (m *MockOrderStorage) GetPendingOrders(ctx context.Context) ([]storage.Order, error) {
	return []storage.Order{}, nil
}

func (m *MockOrderStorage) UpdateOrderByNumber(ctx context.Context, number, status string, accrual *float64) error {
	return nil
}

func (m *MockOrderStorage) AddAccrual(ctx context.Context, userID, orderNumber string, accrual float64) error {
	return nil
}

func (m *MockOrderStorage) GetBalance(ctx context.Context, userID string) (float64, float64, error) {
	return 0, 0, nil
}

func (m *MockOrderStorage) Withdraw(ctx context.Context, userID, order string, sum float64) error {
	return nil
}

func (m *MockOrderStorage) GetWithdrawals(ctx context.Context, userID string) ([]storage.Withdrawal, error) {
	return []storage.Withdrawal{}, nil
}

func (m *MockOrderStorage) Close() error {
	return nil
}

func TestOrderService_UploadOrder_Success(t *testing.T) {
	mock := NewMockOrderStorage()
	svc := NewOrderService(mock)
	err := svc.UploadOrder(context.Background(), "user-123", "79927398713")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestOrderService_UploadOrder_InvalidNumber(t *testing.T) {
	mock := NewMockOrderStorage()
	svc := NewOrderService(mock)
	err := svc.UploadOrder(context.Background(), "user-123", "1234567890123456")
	if !errors.Is(err, ErrInvalidOrderNumber) {
		t.Errorf("expected ErrInvalidOrderNumber, got %v", err)
	}
}

func TestOrderService_UploadOrder_EmptyNumber(t *testing.T) {
	mock := NewMockOrderStorage()
	svc := NewOrderService(mock)
	err := svc.UploadOrder(context.Background(), "user-123", "")
	if !errors.Is(err, ErrInvalidOrderNumber) {
		t.Errorf("expected ErrInvalidOrderNumber, got %v", err)
	}
}

func TestOrderService_UploadOrder_AlreadyUploaded(t *testing.T) {
	mock := NewMockOrderStorage()
	mock.orders["79927398713"] = storage.Order{ID: "order-1", UserID: "user-123", Number: "79927398713"}
	svc := NewOrderService(mock)
	err := svc.UploadOrder(context.Background(), "user-123", "79927398713")
	if !errors.Is(err, ErrOrderAlreadyUploaded) {
		t.Errorf("expected ErrOrderAlreadyUploaded, got %v", err)
	}
}

func TestOrderService_UploadOrder_AnotherUser(t *testing.T) {
	mock := NewMockOrderStorage()
	mock.orders["79927398713"] = storage.Order{ID: "order-1", UserID: "other-user", Number: "79927398713"}
	svc := NewOrderService(mock)
	err := svc.UploadOrder(context.Background(), "user-123", "79927398713")
	if !errors.Is(err, ErrOrderByAnotherUser) {
		t.Errorf("expected ErrOrderByAnotherUser, got %v", err)
	}
}

func TestOrderService_UploadOrder_StorageError(t *testing.T) {
	mock := NewMockOrderStorage()
	mock.saveErr = errors.New("database error")
	svc := NewOrderService(mock)
	err := svc.UploadOrder(context.Background(), "user-123", "79927398713")
	if err == nil {
		t.Error("expected error")
	}
}

func TestOrderService_UploadOrder_NonDigits(t *testing.T) {
	mock := NewMockOrderStorage()
	svc := NewOrderService(mock)
	err := svc.UploadOrder(context.Background(), "user-123", "7992739871a")
	if !errors.Is(err, ErrInvalidOrderNumber) {
		t.Errorf("expected ErrInvalidOrderNumber, got %v", err)
	}
}

func TestOrderService_UploadOrder_Whitespace(t *testing.T) {
	mock := NewMockOrderStorage()
	svc := NewOrderService(mock)
	err := svc.UploadOrder(context.Background(), "user-123", "  79927398713  ")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestOrderService_GetOrders_Success(t *testing.T) {
	mock := NewMockOrderStorage()
	mock.orders["79927398713"] = storage.Order{ID: "order-1", UserID: "user-123", Number: "79927398713", Status: "NEW", UploadedAt: time.Now()}
	mock.orders["4539578763621486"] = storage.Order{ID: "order-2", UserID: "user-123", Number: "4539578763621486", Status: "PROCESSED", UploadedAt: time.Now()}
	svc := NewOrderService(mock)
	orders, err := svc.GetOrders(context.Background(), "user-123")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(orders) != 2 {
		t.Errorf("expected 2 orders, got %d", len(orders))
	}
}

func TestOrderService_GetOrders_Empty(t *testing.T) {
	mock := NewMockOrderStorage()
	svc := NewOrderService(mock)
	orders, err := svc.GetOrders(context.Background(), "user-123")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(orders) != 0 {
		t.Errorf("expected 0 orders, got %d", len(orders))
	}
}

func TestOrderService_GetOrders_OtherUserOrders(t *testing.T) {
	mock := NewMockOrderStorage()
	mock.orders["79927398713"] = storage.Order{ID: "order-1", UserID: "other-user", Number: "79927398713", Status: "NEW", UploadedAt: time.Now()}
	svc := NewOrderService(mock)
	orders, err := svc.GetOrders(context.Background(), "user-123")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(orders) != 0 {
		t.Errorf("expected 0 orders, got %d", len(orders))
	}
}

func TestOrderService_GetOrders_StorageError(t *testing.T) {
	mock := NewMockOrderStorage()
	mock.getOrdersErr = errors.New("database error")
	svc := NewOrderService(mock)
	_, err := svc.GetOrders(context.Background(), "user-123")
	if err == nil {
		t.Error("expected error")
	}
}

func TestValidateLuhn_Valid(t *testing.T) {
	for _, num := range []string{"79927398713", "4539578763621486", "4532015112830366", "5425233430109903", "371449635398431", "0"} {
		if !ValidateLuhn(num) {
			t.Errorf("ValidateLuhn(%s) = false, want true", num)
		}
	}
}

func TestValidateLuhn_Invalid(t *testing.T) {
	for _, num := range []string{"79927398710", "1234567890123456", "1111111111111111", "", "abc", "5"} {
		if ValidateLuhn(num) {
			t.Errorf("ValidateLuhn(%s) = true, want false", num)
		}
	}
}
