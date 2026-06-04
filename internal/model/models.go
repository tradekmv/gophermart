// Package model defines data structures for the gophermart service.
package model

import (
	"time"
)

// OrderStatus represents the status of an order.
type OrderStatus string

const (
	OrderStatusNew        OrderStatus = "NEW"
	OrderStatusProcessing OrderStatus = "PROCESSING"
	OrderStatusInvalid    OrderStatus = "INVALID"
	OrderStatusProcessed  OrderStatus = "PROCESSED"
)

// AccrualStatus represents the status from external accrual system.
type AccrualStatus string

const (
	AccrualStatusRegistered AccrualStatus = "REGISTERED"
	AccrualStatusInvalid    AccrualStatus = "INVALID"
	AccrualStatusProcessing AccrualStatus = "PROCESSING"
	AccrualStatusProcessed  AccrualStatus = "PROCESSED"
)

// User represents a registered user.
type User struct {
	ID           string    `json:"-" db:"id"`
	Login        string    `json:"login" db:"login"`
	PasswordHash string    `json:"-" db:"password_hash"`
	CreatedAt    time.Time `json:"-" db:"created_at"`
}

// Order represents a customer order.
type Order struct {
	ID         string      `json:"-" db:"id"`
	UserID     string      `json:"-" db:"user_id"`
	Number     string      `json:"number" db:"number"`
	Status     OrderStatus `json:"status" db:"status"`
	Accrual    *float64    `json:"accrual,omitempty" db:"accrual"`
	UploadedAt time.Time   `json:"uploaded_at" db:"uploaded_at"`
}

// Balance represents a user's financial balance.
type Balance struct {
	UserID    string  `json:"-" db:"user_id"`
	Current   float64 `json:"current" db:"current"`
	Withdrawn float64 `json:"withdrawn" db:"withdrawn"`
}

// Withdrawal represents a withdrawal transaction.
type Withdrawal struct {
	ID          string    `json:"-" db:"id"`
	UserID      string    `json:"-" db:"user_id"`
	Order       string    `json:"order" db:"order"`
	Sum         float64   `json:"sum" db:"sum"`
	ProcessedAt time.Time `json:"processed_at" db:"processed_at"`
}

// OrderResponse is the DTO for GET /api/user/orders.
type OrderResponse struct {
	Number     string      `json:"number"`
	Status     OrderStatus `json:"status"`
	Accrual    *float64    `json:"accrual,omitempty"`
	UploadedAt time.Time   `json:"uploaded_at"`
}

// WithdrawalResponse is the DTO for GET /api/user/withdrawals.
type WithdrawalResponse struct {
	Order       string    `json:"order"`
	Sum         float64   `json:"sum"`
	ProcessedAt time.Time `json:"processed_at"`
}

// BalanceResponse is the DTO for GET /api/user/balance.
type BalanceResponse struct {
	Current   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}

// AccrualResponse represents response from accrual system.
type AccrualResponse struct {
	Order   string        `json:"order"`
	Status  AccrualStatus `json:"status"`
	Accrual float64       `json:"accrual"`
}

// RegisterRequest represents user registration request.
type RegisterRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// LoginRequest represents user login request.
type LoginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// WithdrawRequest represents withdrawal request.
type WithdrawRequest struct {
	Order string  `json:"order"`
	Sum   float64 `json:"sum"`
}

// ToOrderResponse converts Order to OrderResponse.
func (o *Order) ToOrderResponse() OrderResponse {
	return OrderResponse{
		Number:     o.Number,
		Status:     o.Status,
		Accrual:    o.Accrual,
		UploadedAt: o.UploadedAt,
	}
}

// ToBalanceResponse converts Balance to BalanceResponse.
func (b *Balance) ToBalanceResponse() BalanceResponse {
	return BalanceResponse{
		Current:   b.Current,
		Withdrawn: b.Withdrawn,
	}
}

// ToWithdrawalResponse converts Withdrawal to WithdrawalResponse.
func (w *Withdrawal) ToWithdrawalResponse() WithdrawalResponse {
	return WithdrawalResponse{
		Order:       w.Order,
		Sum:         w.Sum,
		ProcessedAt: w.ProcessedAt,
	}
}
