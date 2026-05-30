package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/tradekmv/gophermart.git/internal/repository/storage"
)

// Order is an alias for storage.Order
type Order = storage.Order

// MockAccrualStorage implements storage.Storage for accrual tests
type MockAccrualStorage struct {
	orders        []Order
	updateErr     error
	addAccrualErr error
}

func NewMockAccrualStorage() *MockAccrualStorage {
	return &MockAccrualStorage{
		orders: make([]Order, 0),
	}
}

func (m *MockAccrualStorage) CreateUserWithBalance(ctx context.Context, login, passwordHash string) (string, error) {
	return "user-id", nil
}

func (m *MockAccrualStorage) GetUserByLogin(ctx context.Context, login string) (storage.User, error) {
	return storage.User{}, storage.ErrNotFound
}

func (m *MockAccrualStorage) SaveOrder(ctx context.Context, userID, orderNumber string) (Order, error) {
	return Order{}, nil
}

func (m *MockAccrualStorage) GetOrderByNumber(ctx context.Context, orderNumber string) (Order, error) {
	return Order{}, storage.ErrNotFound
}

func (m *MockAccrualStorage) GetOrdersByUserID(ctx context.Context, userID string) ([]Order, error) {
	return m.orders, nil
}

func (m *MockAccrualStorage) GetPendingOrders(ctx context.Context) ([]Order, error) {
	return m.orders, nil
}

func (m *MockAccrualStorage) UpdateOrderByNumber(ctx context.Context, number, status string, accrual *float64) error {
	return m.updateErr
}

func (m *MockAccrualStorage) AddAccrual(ctx context.Context, userID, orderNumber string, accrual float64) error {
	return m.addAccrualErr
}

func (m *MockAccrualStorage) GetBalance(ctx context.Context, userID string) (float64, float64, error) {
	return 100, 0, nil
}

func (m *MockAccrualStorage) Withdraw(ctx context.Context, userID, order string, sum float64) error {
	return nil
}

func (m *MockAccrualStorage) GetWithdrawals(ctx context.Context, userID string) ([]storage.Withdrawal, error) {
	return []storage.Withdrawal{}, nil
}

func (m *MockAccrualStorage) Close() error {
	return nil
}

// MockAccrualClient implements AccrualClient for testing
type MockAccrualClient struct {
	responses map[string]AccrualInfo
	errors    map[string]error
}

func NewMockAccrualClient() *MockAccrualClient {
	return &MockAccrualClient{
		responses: make(map[string]AccrualInfo),
		errors:    make(map[string]error),
	}
}

func (m *MockAccrualClient) Fetch(ctx context.Context, orderNumber string) (AccrualInfo, error) {
	if err, ok := m.errors[orderNumber]; ok {
		return AccrualInfo{}, err
	}
	if resp, ok := m.responses[orderNumber]; ok {
		return resp, nil
	}
	return AccrualInfo{Status: "NEW", Accrual: 0}, nil
}

func TestAccrualService_New(t *testing.T) {
	storage := NewMockAccrualStorage()
	client := NewMockAccrualClient()

	svc := NewAccrualService(storage, client, 1, nil)
	if svc == nil {
		t.Error("NewAccrualService() returned nil")
	}
	if svc.storage != storage {
		t.Error("storage not set correctly")
	}
	if svc.client != client {
		t.Error("client not set correctly")
	}
}

func TestAccrualService_StartWithoutClient(t *testing.T) {
	storage := NewMockAccrualStorage()

	svc := NewAccrualService(storage, nil, 1, nil)

	ctx := context.Background()
	svc.Start(ctx)

	// Should not panic and return immediately
}

func TestAccrualService_Sync(t *testing.T) {
	storage := NewMockAccrualStorage()
	client := NewMockAccrualClient()

	// Add pending order
	storage.orders = append(storage.orders, Order{
		ID:     "order-1",
		UserID: "user-1",
		Number: "79927398713",
		Status: "NEW",
	})

	// Set expected response
	client.responses["79927398713"] = AccrualInfo{
		Status:  "PROCESSED",
		Accrual: 150.50,
	}

	svc := NewAccrualService(storage, client, 1, nil)
	svc.sync(context.Background())
}

func TestAccrualService_SyncWithErrors(t *testing.T) {
	storage := NewMockAccrualStorage()
	client := NewMockAccrualClient()

	// Add pending order
	storage.orders = append(storage.orders, Order{
		ID:     "order-1",
		UserID: "user-1",
		Number: "79927398713",
		Status: "NEW",
	})

	// Set error response
	client.errors["79927398713"] = context.DeadlineExceeded

	svc := NewAccrualService(storage, client, 1, nil)
	svc.sync(context.Background())

	// Should not panic
}

func TestAccrualService_SyncWithRegisteredStatus(t *testing.T) {
	storage := NewMockAccrualStorage()
	client := NewMockAccrualClient()

	// Add pending order
	storage.orders = append(storage.orders, Order{
		ID:     "order-1",
		UserID: "user-1",
		Number: "79927398713",
		Status: "NEW",
	})

	// Set REGISTERED status (should map to PROCESSING)
	client.responses["79927398713"] = AccrualInfo{
		Status:  "REGISTERED",
		Accrual: 0,
	}

	svc := NewAccrualService(storage, client, 1, nil)
	svc.sync(context.Background())
}

func TestAccrualService_Stop(t *testing.T) {
	storage := NewMockAccrualStorage()
	client := NewMockAccrualClient()

	svc := NewAccrualService(storage, client, 1, nil)

	// Start with very long interval
	svc.interval = 100 * time.Hour
	svc.Start(context.Background())

	// Stop should complete quickly
	done := make(chan bool)
	go func() {
		svc.Stop()
		done <- true
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Error("Stop() timed out")
	}
}

func TestAccrualService_SyncConcurrently(t *testing.T) {
	storage := NewMockAccrualStorage()
	client := NewMockAccrualClient()

	// Add pending order
	storage.orders = append(storage.orders, Order{
		ID:     "order-1",
		UserID: "user-1",
		Number: "79927398713",
		Status: "NEW",
	})

	client.responses["79927398713"] = AccrualInfo{
		Status:  "PROCESSED",
		Accrual: 100,
	}

	svc := NewAccrualService(storage, client, 1, nil)

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			svc.sync(context.Background())
		}()
	}
	wg.Wait()
}

func TestMapAccrualStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"REGISTERED", "PROCESSING"},
		{"PROCESSING", "PROCESSING"},
		{"INVALID", "INVALID"},
		{"PROCESSED", "PROCESSED"},
		{"UNKNOWN", "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := mapAccrualStatus(tt.input)
			if result != tt.expected {
				t.Errorf("mapAccrualStatus(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
