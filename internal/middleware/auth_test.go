package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tradekmv/gophermart.git/pkg/auth"
)

func TestAuthMiddleware_RequireAuth_NoCookie(t *testing.T) {
	authSvc := auth.New(auth.Config{SecretKey: "test-secret"})
	middleware := NewAuthMiddleware(authSvc)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rr := httptest.NewRecorder()

	handler := middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestAuthMiddleware_RequireAuth_InvalidCookie(t *testing.T) {
	authSvc := auth.New(auth.Config{SecretKey: "test-secret"})
	middleware := NewAuthMiddleware(authSvc)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "invalid-token"})
	rr := httptest.NewRecorder()

	handler := middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestAuthMiddleware_RequireAuth_ValidToken(t *testing.T) {
	authSvc := auth.New(auth.Config{SecretKey: "test-secret"})
	middleware := NewAuthMiddleware(authSvc)

	// Generate valid token
	token, err := authSvc.GenerateToken("user-123")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: token})
	rr := httptest.NewRecorder()

	var handlerUserID string
	handler := middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerUserID = GetUserID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	if handlerUserID != "user-123" {
		t.Errorf("expected userID 'user-123', got '%s'", handlerUserID)
	}
}

func TestAuthMiddleware_RequireAuth_BearerToken(t *testing.T) {
	authSvc := auth.New(auth.Config{SecretKey: "test-secret"})
	middleware := NewAuthMiddleware(authSvc)

	// Generate valid token
	token, err := authSvc.GenerateToken("user-456")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	var handlerUserID string
	handler := middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerUserID = GetUserID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	if handlerUserID != "user-456" {
		t.Errorf("expected userID 'user-456', got '%s'", handlerUserID)
	}
}

func TestGetUserID_EmptyContext(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	userID := GetUserID(req.Context())

	if userID != "" {
		t.Errorf("expected empty userID, got '%s'", userID)
	}
}

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{"valid bearer", "Bearer abc123token", "abc123token"},
		{"invalid prefix", "Basic abc123token", ""},
		{"empty header", "", ""},
		{"bearer only", "Bearer ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Authorization", tt.header)

			got := ExtractBearerToken(req)
			if got != tt.want {
				t.Errorf("ExtractBearerToken() = %q, want %q", got, tt.want)
			}
		})
	}
}
