// Package auth provides cookie-based authentication with HMAC-SHA256 signing.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	// ErrUnauthorized is returned when authentication fails.
	ErrUnauthorized = errors.New("unauthorized")
	// ErrInvalidToken is returned when token validation fails.
	ErrInvalidToken = errors.New("invalid token")
)

const (
	cookieName  = "session"
	tokenExpiry = 24 * 60 * 60 // 24 hours in seconds
)

// Auth provides authentication functionality.
type Auth struct {
	secretKey []byte
	mu        sync.RWMutex
}

// Config holds authentication configuration.
type Config struct {
	SecretKey string
	RunEnv    string // "production" enables Secure cookies
}

// New creates a new Auth instance.
func New(cfg Config) *Auth {
	secretKey := []byte(cfg.SecretKey)
	if len(secretKey) == 0 {
		secretKey = []byte("default-secret")
	}
	return &Auth{
		secretKey: secretKey,
	}
}

// GenerateToken creates a new JWT token for the user.
func (a *Auth) GenerateToken(userID string) (string, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	})

	return token.SignedString(a.secretKey)
}

// ValidateToken validates a JWT token and returns the user ID.
func (a *Auth) ValidateToken(tokenString string) (string, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return a.secretKey, nil
	})

	if err != nil {
		return "", ErrInvalidToken
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		userID, ok := claims["user_id"].(string)
		if !ok {
			return "", ErrInvalidToken
		}
		return userID, nil
	}

	return "", ErrInvalidToken
}

// SetCookie sets the session cookie on the response.
func (a *Auth) SetCookie(w http.ResponseWriter, token string, isProduction bool) {
	cookie := &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   isProduction,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   tokenExpiry,
	}
	http.SetCookie(w, cookie)
}

// GetUserID extracts user ID from request cookie or Authorization header.
func (a *Auth) GetUserID(r *http.Request) (string, error) {
	// Try cookie first
	cookie, err := r.Cookie(cookieName)
	if err == nil && cookie.Value != "" {
		return a.ValidateToken(cookie.Value)
	}

	// Fallback: Authorization header (Bearer token)
	if token := GetBearerToken(r); token != "" {
		return a.ValidateToken(token)
	}

	return "", ErrUnauthorized
}

// GetBearerToken extracts the bearer token from the Authorization header.
func GetBearerToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		return authHeader[7:]
	}
	return ""
}

// SignHMAC creates an HMAC-SHA256 signature for a string.
func SignHMAC(data string, key []byte) string {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

// ValidateHMAC validates an HMAC-SHA256 signature.
func ValidateHMAC(data, signature string, key []byte) bool {
	expected := SignHMAC(data, key)
	return hmac.Equal([]byte(expected), []byte(signature))
}
