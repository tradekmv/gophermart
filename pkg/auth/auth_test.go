package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuth_New(t *testing.T) {
	tests := []struct {
		name      string
		cfg       Config
		wantEmpty bool
	}{
		{
			name:      "with secret key",
			cfg:       Config{SecretKey: "my-secret-key"},
			wantEmpty: false,
		},
		{
			name:      "empty secret key uses default",
			cfg:       Config{SecretKey: ""},
			wantEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := New(tt.cfg)
			if a == nil {
				t.Fatal("New() returned nil")
			}
		})
	}
}

func TestAuth_GenerateToken(t *testing.T) {
	a := New(Config{SecretKey: "test-secret"})

	token1, err := a.GenerateToken("user-123")
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}
	if token1 == "" {
		t.Error("GenerateToken() returned empty token")
	}

	// Token should be different for different users
	token2, err := a.GenerateToken("user-456")
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}
	if token1 == token2 {
		t.Error("GenerateToken() returned same token for different users")
	}

	// Token should be different each time (due to iat)
	token3, err := a.GenerateToken("user-123")
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}
	// Tokens may or may not be the same depending on timing
	_ = token3
}

func TestAuth_ValidateToken(t *testing.T) {
	a := New(Config{SecretKey: "test-secret"})

	// Generate token
	token, err := a.GenerateToken("user-123")
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	// Validate token
	userID, err := a.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}
	if userID != "user-123" {
		t.Errorf("ValidateToken() userID = %q, want %q", userID, "user-123")
	}
}

func TestAuth_ValidateToken_InvalidToken(t *testing.T) {
	a := New(Config{SecretKey: "test-secret"})

	_, err := a.ValidateToken("invalid-token")
	if err == nil {
		t.Error("ValidateToken() expected error for invalid token")
	}
}

func TestAuth_ValidateToken_WrongSecret(t *testing.T) {
	a1 := New(Config{SecretKey: "secret-1"})
	a2 := New(Config{SecretKey: "secret-2"})

	// Generate token with one secret
	token, _ := a1.GenerateToken("user-123")

	// Try to validate with different secret
	_, err := a2.ValidateToken(token)
	if err == nil {
		t.Error("ValidateToken() should fail with different secret")
	}
}

func TestAuth_SetCookie(t *testing.T) {
	a := New(Config{SecretKey: "test-secret"})

	// Test with production = false
	w := httptest.NewRecorder()
	a.SetCookie(w, "test-token", false)

	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}

	cookie := cookies[0]
	if cookie.Name != "session" {
		t.Errorf("cookie.Name = %q, want %q", cookie.Name, "session")
	}
	if cookie.Value != "test-token" {
		t.Errorf("cookie.Value = %q, want %q", cookie.Value, "test-token")
	}
	if !cookie.HttpOnly {
		t.Error("cookie.HttpOnly should be true")
	}
	if cookie.Secure {
		t.Error("cookie.Secure should be false for non-production")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie.SameSite = %v, want %v", cookie.SameSite, http.SameSiteLaxMode)
	}
}

func TestAuth_SetCookie_Production(t *testing.T) {
	a := New(Config{SecretKey: "test-secret"})

	w := httptest.NewRecorder()
	a.SetCookie(w, "test-token", true) // production = true

	cookies := w.Result().Cookies()
	if !cookies[0].Secure {
		t.Error("cookie.Secure should be true for production")
	}
}

func TestAuth_GetUserID_Cookie(t *testing.T) {
	a := New(Config{SecretKey: "test-secret"})

	token, _ := a.GenerateToken("user-789")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: token})

	userID, err := a.GetUserID(req)
	if err != nil {
		t.Fatalf("GetUserID() error = %v", err)
	}
	if userID != "user-789" {
		t.Errorf("GetUserID() = %q, want %q", userID, "user-789")
	}
}

func TestAuth_GetUserID_BearerHeader(t *testing.T) {
	a := New(Config{SecretKey: "test-secret"})

	token, _ := a.GenerateToken("user-bearer")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	userID, err := a.GetUserID(req)
	if err != nil {
		t.Fatalf("GetUserID() error = %v", err)
	}
	if userID != "user-bearer" {
		t.Errorf("GetUserID() = %q, want %q", userID, "user-bearer")
	}
}

func TestAuth_GetUserID_NoAuth(t *testing.T) {
	a := New(Config{SecretKey: "test-secret"})

	req := httptest.NewRequest(http.MethodGet, "/", nil)

	_, err := a.GetUserID(req)
	if err == nil {
		t.Error("GetUserID() expected error for no auth")
	}
}

func TestSignHMAC(t *testing.T) {
	key := []byte("secret-key")
	data := "test-data"

	signature1 := SignHMAC(data, key)
	signature2 := SignHMAC(data, key)

	if signature1 != signature2 {
		t.Error("SignHMAC() should return same signature for same input")
	}

	// Different data
	signature3 := SignHMAC("different-data", key)
	if signature1 == signature3 {
		t.Error("SignHMAC() should return different signature for different data")
	}
}

func TestValidateHMAC(t *testing.T) {
	key := []byte("secret-key")
	data := "test-data"
	signature := SignHMAC(data, key)

	if !ValidateHMAC(data, signature, key) {
		t.Error("ValidateHMAC() should return true for valid signature")
	}

	if ValidateHMAC(data, "wrong-signature", key) {
		t.Error("ValidateHMAC() should return false for invalid signature")
	}

	if ValidateHMAC("different-data", signature, key) {
		t.Error("ValidateHMAC() should return false for different data")
	}
}
