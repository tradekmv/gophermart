// Package middleware provides HTTP middleware for the gophermart service.
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/tradekmv/gophermart.git/pkg/auth"
)

// ContextKey is the type for context keys.
type ContextKey string

const userIDKey ContextKey = "userID"

// AuthMiddleware provides authentication middleware.
type AuthMiddleware struct {
	auth *auth.Auth
}

// NewAuthMiddleware creates a new AuthMiddleware.
func NewAuthMiddleware(auth *auth.Auth) *AuthMiddleware {
	return &AuthMiddleware{auth: auth}
}

// RequireAuth is middleware that requires authentication.
func (m *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, err := m.auth.GetUserID(r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// Add userID to context
		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetUserID extracts user ID from context.
func GetUserID(ctx context.Context) string {
	if userID, ok := ctx.Value(userIDKey).(string); ok {
		return userID
	}
	return ""
}

// ExtractBearerToken extracts token from Authorization header.
func ExtractBearerToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}
	return ""
}
