// Package storage defines the repository interface for data persistence.
package storage

import (
	"context"
	"time"
)

// Order represents an order in the system.
type Order struct {
	ID         string
	UserID     string
	Number     string
	Status     string
	Accrual    *float64
	UploadedAt time.Time
}

// Balance represents a user's balance.
type Balance struct {
	UserID    string
	Current   float64
	Withdrawn float64
}

// Withdrawal represents a withdrawal record.
type Withdrawal struct {
	ID          string
	UserID      string
	Order       string
	Sum         float64
	ProcessedAt time.Time
}

// User represents a user in the system.
type User struct {
	ID           string
	Login        string
	PasswordHash string
	CreatedAt    time.Time
}

// Storage defines the interface for data persistence operations.
type Storage interface {
	// User operations
	CreateUserWithBalance(ctx context.Context, login, passwordHash string) (string, error)
	GetUserByLogin(ctx context.Context, login string) (User, error)

	// Order operations
	SaveOrder(ctx context.Context, userID, orderNumber string) (Order, error)
	GetOrderByNumber(ctx context.Context, orderNumber string) (Order, error)
	GetOrdersByUserID(ctx context.Context, userID string) ([]Order, error)
	GetPendingOrders(ctx context.Context) ([]Order, error)
	UpdateOrderByNumber(ctx context.Context, number, status string, accrual *float64) error

	// Accrual operations
	AddAccrual(ctx context.Context, userID, orderNumber string, accrual float64) error

	// Balance operations
	GetBalance(ctx context.Context, userID string) (float64, float64, error)
	Withdraw(ctx context.Context, userID, order string, sum float64) error
	GetWithdrawals(ctx context.Context, userID string) ([]Withdrawal, error)

	// Lifecycle
	Close() error
}

//go:generate mockgen -source=interface.go -destination=mock/mock.go -package=mock
