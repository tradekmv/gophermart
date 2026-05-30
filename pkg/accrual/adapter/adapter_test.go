package adapter

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tradekmv/gophermart.git/internal/service"
	"github.com/tradekmv/gophermart.git/pkg/accrual"
)

func TestNewClient(t *testing.T) {
	accrualClient := accrual.NewClient("http://localhost:8080")
	adapter := NewClient(accrualClient)

	if adapter == nil {
		t.Error("NewClient() returned nil")
	}
	if adapter.client != accrualClient {
		t.Error("adapter.client not set correctly")
	}
}

func TestClient_Fetch_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"order":"79927398713","status":"PROCESSED","accrual":150.50}`))
	}))
	defer server.Close()

	accrualClient := accrual.NewClient(server.URL)
	adapter := NewClient(accrualClient)

	info, err := adapter.Fetch(context.Background(), "79927398713")
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if info.Status != "PROCESSED" {
		t.Errorf("Fetch() Status = %q, want %q", info.Status, "PROCESSED")
	}
	if info.Accrual != 150.50 {
		t.Errorf("Fetch() Accrual = %v, want %v", info.Accrual, 150.50)
	}
}

func TestClient_Fetch_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	accrualClient := accrual.NewClient(server.URL)
	adapter := NewClient(accrualClient)

	_, err := adapter.Fetch(context.Background(), "invalid")
	if err == nil {
		t.Error("Fetch() expected error for not found")
	}
	if !errors.Is(err, accrual.ErrOrderNotFound) {
		t.Errorf("Fetch() error = %v, want %v", err, accrual.ErrOrderNotFound)
	}
}

// Verify adapter implements service.AccrualClient interface
var _ service.AccrualClient = (*Client)(nil)
