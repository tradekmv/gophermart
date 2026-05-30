package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
)

func TestLoggingMiddleware_Success(t *testing.T) {
	logger := zerolog.Nop()

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	handler := LoggingMiddleware(&logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestLoggingMiddleware_NotFound(t *testing.T) {
	logger := zerolog.Nop()

	req := httptest.NewRequest(http.MethodGet, "/notfound", nil)
	rr := httptest.NewRecorder()

	handler := LoggingMiddleware(&logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestLoggingMiddleware_ServerError(t *testing.T) {
	logger := zerolog.Nop()

	req := httptest.NewRequest(http.MethodPost, "/api/data", nil)
	rr := httptest.NewRecorder()

	handler := LoggingMiddleware(&logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected %d, got %d", http.StatusInternalServerError, rr.Code)
	}
}

func TestResponseWriter_Write(t *testing.T) {
	_ = httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	rw := &responseWriter{
		w:          rr,
		statusCode: http.StatusOK,
	}

	n, err := rw.Write([]byte("hello"))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if n != 5 {
		t.Errorf("expected 5 bytes written, got %d", n)
	}
}

func TestResponseWriter_WriteHeader(t *testing.T) {
	_ = httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	rw := &responseWriter{
		w:          rr,
		statusCode: http.StatusOK,
	}

	rw.WriteHeader(http.StatusCreated)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected %d, got %d", http.StatusCreated, rr.Code)
	}
	if rw.statusCode != http.StatusCreated {
		t.Errorf("expected status %d in struct, got %d", http.StatusCreated, rw.statusCode)
	}
}

func TestResponseWriter_Header(t *testing.T) {
	_ = httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	rw := &responseWriter{
		w:          rr,
		statusCode: http.StatusOK,
	}

	h := rw.Header()
	h.Set("X-Test", "value")

	if rr.Header().Get("X-Test") != "value" {
		t.Error("expected header to be set")
	}
}
