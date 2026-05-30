package model

import (
	"testing"
	"time"
)

func TestOrder_ToOrderResponse(t *testing.T) {
	accrual := 150.50
	order := Order{
		ID:         "order-123",
		UserID:     "user-456",
		Number:     "79927398713",
		Status:     OrderStatusNew,
		Accrual:    &accrual,
		UploadedAt: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
	}

	resp := order.ToOrderResponse()

	if resp.Number != "79927398713" {
		t.Errorf("Number = %q, want %q", resp.Number, "79927398713")
	}
	if resp.Status != OrderStatusNew {
		t.Errorf("Status = %v, want %v", resp.Status, OrderStatusNew)
	}
	if resp.Accrual == nil || *resp.Accrual != 150.50 {
		t.Errorf("Accrual mismatch")
	}
}

func TestOrder_ToOrderResponse_NilAccrual(t *testing.T) {
	order := Order{
		Number:  "79927398713",
		Status:  OrderStatusNew,
		Accrual: nil,
	}

	resp := order.ToOrderResponse()

	if resp.Accrual != nil {
		t.Errorf("Accrual should be nil, got %v", resp.Accrual)
	}
}

func TestBalance_ToBalanceResponse(t *testing.T) {
	balance := Balance{
		UserID:    "user-123",
		Current:   500.50,
		Withdrawn: 100.25,
	}

	resp := balance.ToBalanceResponse()

	if resp.Current != 500.50 {
		t.Errorf("Current = %v, want %v", resp.Current, 500.50)
	}
	if resp.Withdrawn != 100.25 {
		t.Errorf("Withdrawn = %v, want %v", resp.Withdrawn, 100.25)
	}
}

func TestWithdrawal_ToWithdrawalResponse(t *testing.T) {
	time := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	withdrawal := Withdrawal{
		ID:          "w-123",
		UserID:      "user-456",
		Order:       "79927398713",
		Sum:         200.00,
		ProcessedAt: time,
	}

	resp := withdrawal.ToWithdrawalResponse()

	if resp.Order != "79927398713" {
		t.Errorf("Order = %q, want %q", resp.Order, "79927398713")
	}
	if resp.Sum != 200.00 {
		t.Errorf("Sum = %v, want %v", resp.Sum, 200.00)
	}
	if !resp.ProcessedAt.Equal(time) {
		t.Errorf("ProcessedAt mismatch")
	}
}

func TestOrderStatus_Constants(t *testing.T) {
	if OrderStatusNew != "NEW" {
		t.Error("OrderStatusNew should be 'NEW'")
	}
	if OrderStatusProcessing != "PROCESSING" {
		t.Error("OrderStatusProcessing should be 'PROCESSING'")
	}
	if OrderStatusInvalid != "INVALID" {
		t.Error("OrderStatusInvalid should be 'INVALID'")
	}
	if OrderStatusProcessed != "PROCESSED" {
		t.Error("OrderStatusProcessed should be 'PROCESSED'")
	}
}

func TestAccrualStatus_Constants(t *testing.T) {
	if AccrualStatusRegistered != "REGISTERED" {
		t.Error("AccrualStatusRegistered should be 'REGISTERED'")
	}
	if AccrualStatusInvalid != "INVALID" {
		t.Error("AccrualStatusInvalid should be 'INVALID'")
	}
	if AccrualStatusProcessing != "PROCESSING" {
		t.Error("AccrualStatusProcessing should be 'PROCESSING'")
	}
	if AccrualStatusProcessed != "PROCESSED" {
		t.Error("AccrualStatusProcessed should be 'PROCESSED'")
	}
}
