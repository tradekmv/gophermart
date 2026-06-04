package accrual

import (
	"context"
	"encoding/json"
	"errors"
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

func TestClient_FetchRaw_Success(t *testing.T) {
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

	resp, err := client.FetchRaw(ctx, "79927398713")
	if err != nil {
		t.Fatalf("FetchRaw() error = %v", err)
	}

	if resp.Order != "79927398713" {
		t.Errorf("FetchRaw() Order = %q, want %q", resp.Order, "79927398713")
	}
	if resp.Status != "PROCESSED" {
		t.Errorf("FetchRaw() Status = %q, want %q", resp.Status, "PROCESSED")
	}
	expected := 100.50
	if resp.Accrual == nil || *resp.Accrual != expected {
		t.Errorf("FetchRaw() Accrual = %v, want %v", resp.Accrual, expected)
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
	// Сервер возвращает 429 с Retry-After=0 (без задержки). Клиент должен
	// сразу вернуть *RateLimitError, а не делать повторный запрос (retry —
	// ответственность потребителя, см. RateLimitError).
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.Fetch(ctx, "79927398713")
	if err == nil {
		t.Fatal("Fetch() expected error for rate limit")
	}

	var rle *RateLimitError
	if !errors.As(err, &rle) {
		t.Fatalf("Fetch() error = %v, want *RateLimitError", err)
	}
	if rle.RetryAfter != 0 {
		t.Errorf("Fetch() RetryAfter = %s, want 0", rle.RetryAfter)
	}
}

func TestClient_Fetch_RateLimit_WithRetryAfter(t *testing.T) {
	// Сервер возвращает 429 с Retry-After=1. Клиент должен:
	// 1. Подождать указанное время.
	// 2. Вернуть *RateLimitError с этим значением в RetryAfter.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	_, err := client.Fetch(ctx, "79927398713")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Fetch() expected error")
	}

	var rle *RateLimitError
	if !errors.As(err, &rle) {
		t.Fatalf("Fetch() error = %v, want *RateLimitError", err)
	}
	if rle.RetryAfter != 1*time.Second {
		t.Errorf("Fetch() RetryAfter = %s, want 1s", rle.RetryAfter)
	}
	// Должен был подождать примерно 1s (допускаем погрешность).
	if elapsed < 900*time.Millisecond {
		t.Errorf("Fetch() returned too early: %s, want >= 1s", elapsed)
	}
}

func TestClient_Fetch_RateLimit_ContextCancelled(t *testing.T) {
	// Сервер возвращает 429 с Retry-After=10. Клиент ждёт — мы отменяем
	// контекст и должны получить context.Canceled.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "10")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := client.Fetch(ctx, "79927398713")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Fetch() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestClient_Fetch_ContextCancelled(t *testing.T) {
	// Сервер сразу отвечает 200, но клиент ждёт тело. Контекст клиента
	// отменяется почти сразу — клиент должен вернуть ошибку не дожидаясь
	// тела (тело ему не нужно — статус 200, парсинг в порядке).
	// Чтобы не блокировать тест 10s, делаем сервер, который никогда не
	// ответит, и используем 100ms timeout.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done() // ждём, пока клиент отвалится
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := client.Fetch(ctx, "79927398713")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Fetch() expected error for cancelled context")
	}
	if elapsed > 1*time.Second {
		t.Errorf("Fetch() took too long: %s, want < 1s", elapsed)
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
	info, err := client.Fetch(context.Background(), "79927398713")
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if info.Status != "PROCESSING" {
		t.Errorf("Fetch() Status = %q, want %q", info.Status, "PROCESSING")
	}
	if info.Accrual != 25.5 {
		t.Errorf("Fetch() Accrual = %v, want %v", info.Accrual, 25.5)
	}
}
