package accrual

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func ptrFloat64(v float64) *float64 { return &v }

func TestNewClient(t *testing.T) {
	client := NewClient("http://localhost:8080")
	if client == nil {
		t.Error("NewClient() returned nil")
	}
	if client.baseURL != "http://localhost:8080" {
		t.Errorf("NewClient() baseURL = %q, want %q", client.baseURL, "http://localhost:8080")
	}
}

func TestClient_Fetch_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/orders/79927398713" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		resp := AccrualResponse{
			Order:   "79927398713",
			Status:  "PROCESSED",
			Accrual: ptrFloat64(100.50),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Fetch(ctx, "79927398713")
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if resp.Order != "79927398713" {
		t.Errorf("Fetch() Order = %q, want %q", resp.Order, "79927398713")
	}
	if resp.Status != "PROCESSED" {
		t.Errorf("Fetch() Status = %q, want %q", resp.Status, "PROCESSED")
	}
	expected := ptrFloat64(100.50)
	if resp.Accrual == nil || *resp.Accrual != *expected {
		t.Errorf("Fetch() Accrual = %v, want %v", resp.Accrual, 100.50)
	}
}

func TestClient_Fetch_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.Fetch(ctx, "invalid")
	if err == nil {
		t.Error("Fetch() expected error for not found")
	}
	if err != ErrOrderNotFound {
		t.Errorf("Fetch() error = %v, want %v", err, ErrOrderNotFound)
	}
}

func TestClient_Fetch_RateLimit(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}

		resp := AccrualResponse{
			Order:   "79927398713",
			Status:  "PROCESSED",
			Accrual: ptrFloat64(50),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.Fetch(ctx, "79927398713")
	if err != nil {
		t.Fatalf("Fetch() error after rate limit: %v", err)
	}

	expected := ptrFloat64(50)
	if resp.Accrual == nil || *resp.Accrual != *expected {
		t.Errorf("Fetch() Accrual = %v, want %v", resp.Accrual, 50)
	}

	if requestCount != 2 {
		t.Errorf("expected 2 requests, got %d", requestCount)
	}
}

func TestClient_Fetch_MaxRetries(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := client.Fetch(ctx, "79927398713")
	if err == nil {
		t.Error("Fetch() expected error after max retries")
	}

	if requestCount != 3 {
		t.Errorf("expected 3 requests (maxRetries), got %d", requestCount)
	}
}

func TestClient_Fetch_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := client.Fetch(ctx, "79927398713")
	if err == nil {
		t.Error("Fetch() expected error for cancelled context")
	}
}

func TestClient_FetchInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := AccrualResponse{
			Order:   "79927398713",
			Status:  "PROCESSING",
			Accrual: ptrFloat64(25.5),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	info, err := client.FetchInfo(context.Background(), "79927398713")
	if err != nil {
		t.Fatalf("FetchInfo() error = %v", err)
	}

	if info.Status != "PROCESSING" {
		t.Errorf("FetchInfo() Status = %q, want %q", info.Status, "PROCESSING")
	}
	if info.Accrual != 25.5 {
		t.Errorf("FetchInfo() Accrual = %v, want %v", info.Accrual, 25.5)
	}
}
