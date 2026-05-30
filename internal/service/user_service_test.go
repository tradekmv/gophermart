package service

import (
	"context"
	"errors"
	"testing"

	"github.com/tradekmv/gophermart.git/internal/repository/storage"
	"golang.org/x/crypto/bcrypt"
)

// MockStorage implements storage.Storage interface for testing
type MockUserStorage struct {
	users         map[string]storage.User
	createErr     error
	getByLoginErr error
}

func NewMockUserStorage() *MockUserStorage {
	return &MockUserStorage{
		users: make(map[string]storage.User),
	}
}

func (m *MockUserStorage) CreateUserWithBalance(ctx context.Context, login, passwordHash string) (string, error) {
	if m.createErr != nil {
		return "", m.createErr
	}
	id := "test-user-id-" + login
	m.users[login] = storage.User{
		ID:           id,
		Login:        login,
		PasswordHash: passwordHash,
	}
	return id, nil
}

func (m *MockUserStorage) GetUserByLogin(ctx context.Context, login string) (storage.User, error) {
	if m.getByLoginErr != nil {
		return storage.User{}, m.getByLoginErr
	}
	if user, ok := m.users[login]; ok {
		return user, nil
	}
	return storage.User{}, storage.ErrNotFound
}

func (m *MockUserStorage) SaveOrder(ctx context.Context, userID, orderNumber string) (storage.Order, error) {
	return storage.Order{}, nil
}

func (m *MockUserStorage) GetOrderByNumber(ctx context.Context, orderNumber string) (storage.Order, error) {
	return storage.Order{}, storage.ErrNotFound
}

func (m *MockUserStorage) GetOrdersByUserID(ctx context.Context, userID string) ([]storage.Order, error) {
	return []storage.Order{}, nil
}

func (m *MockUserStorage) GetPendingOrders(ctx context.Context) ([]storage.Order, error) {
	return []storage.Order{}, nil
}

func (m *MockUserStorage) UpdateOrderByNumber(ctx context.Context, number, status string, accrual *float64) error {
	return nil
}

func (m *MockUserStorage) AddAccrual(ctx context.Context, userID, orderNumber string, accrual float64) error {
	return nil
}

func (m *MockUserStorage) GetBalance(ctx context.Context, userID string) (float64, float64, error) {
	return 0, 0, nil
}

func (m *MockUserStorage) Withdraw(ctx context.Context, userID, order string, sum float64) error {
	return nil
}

func (m *MockUserStorage) GetWithdrawals(ctx context.Context, userID string) ([]storage.Withdrawal, error) {
	return []storage.Withdrawal{}, nil
}

func (m *MockUserStorage) Close() error {
	return nil
}

func TestUserService_Register(t *testing.T) {
	tests := []struct {
		name      string
		login     string
		password  string
		wantErr   error
		setupMock func(*MockUserStorage)
	}{
		{
			name:      "successful registration",
			login:     "testuser",
			password:  "password123",
			wantErr:   nil,
			setupMock: func(m *MockUserStorage) {},
		},
		{
			name:      "empty login",
			login:     "",
			password:  "password123",
			wantErr:   ErrInvalidCredentials,
			setupMock: func(m *MockUserStorage) {},
		},
		{
			name:      "empty password",
			login:     "testuser",
			password:  "",
			wantErr:   ErrInvalidCredentials,
			setupMock: func(m *MockUserStorage) {},
		},
		{
			name:     "user already exists",
			login:    "existinguser",
			password: "password123",
			wantErr:  ErrUserExists,
			setupMock: func(m *MockUserStorage) {
				m.users["existinguser"] = storage.User{ID: "123", Login: "existinguser"}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockUserStorage()
			tt.setupMock(mock)

			svc := NewUserService(mock)
			_, err := svc.Register(context.Background(), tt.login, tt.password)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("Register() error = %v, wantErr %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Errorf("Register() unexpected error = %v", err)
			}
		})
	}
}

func TestUserService_Login(t *testing.T) {
	tests := []struct {
		name      string
		login     string
		password  string
		wantErr   error
		setupMock func(*MockUserStorage)
	}{
		{
			name:     "successful login",
			login:    "testuser",
			password: "password123",
			wantErr:  nil,
			setupMock: func(m *MockUserStorage) {
				// Generate real bcrypt hash for "password123"
				hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
				m.users["testuser"] = storage.User{
					ID:           "user-123",
					Login:        "testuser",
					PasswordHash: string(hash),
				}
			},
		},
		{
			name:      "empty login",
			login:     "",
			password:  "password123",
			wantErr:   ErrInvalidCredentials,
			setupMock: func(m *MockUserStorage) {},
		},
		{
			name:      "empty password",
			login:     "testuser",
			password:  "",
			wantErr:   ErrInvalidCredentials,
			setupMock: func(m *MockUserStorage) {},
		},
		{
			name:      "user not found",
			login:     "nonexistent",
			password:  "password123",
			wantErr:   ErrInvalidCredentials,
			setupMock: func(m *MockUserStorage) {},
		},
		{
			name:     "wrong password",
			login:    "testuser",
			password: "wrongpassword",
			wantErr:  ErrInvalidCredentials,
			setupMock: func(m *MockUserStorage) {
				hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
				m.users["testuser"] = storage.User{
					ID:           "user-123",
					Login:        "testuser",
					PasswordHash: string(hash),
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockUserStorage()
			tt.setupMock(mock)

			svc := NewUserService(mock)
			_, err := svc.Login(context.Background(), tt.login, tt.password)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("Login() error = %v, wantErr %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Errorf("Login() unexpected error = %v", err)
			}
		})
	}
}
