// Package middleware provides HTTP middleware for the gophermart service.
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/rs/zerolog"

	"github.com/tradekmv/gophermart.git/pkg/auth"
)

// contextKey — неэкспортируемый тип для ключей context (Go-идиома: избегаем коллизий).
type contextKey string

// userIDKey — ключ для хранения userID в context.
const userIDKey contextKey = "userID"

// WithUserID возвращает context, содержащий userID под неэкспортируемым ключом.
// Использование вместо context.WithValue(...) напрямую гарантирует, что все
// потребители работают через один и тот же ключ.
func WithUserID(parent context.Context, userID string) context.Context {
	return context.WithValue(parent, userIDKey, userID)
}

// FromContext извлекает userID из context. Возвращает пустую строку и false,
// если значение отсутствует или имеет неожиданный тип.
func FromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(userIDKey).(string)
	return id, ok
}

// AuthMiddleware provides authentication middleware.
type AuthMiddleware struct {
	auth   *auth.Auth
	logger *zerolog.Logger
}

// NewAuthMiddleware creates a new AuthMiddleware.
// logger — опциональный, может быть nil. Если передан — 401-ответы логируются.
func NewAuthMiddleware(auth *auth.Auth, logger *zerolog.Logger) *AuthMiddleware {
	return &AuthMiddleware{auth: auth, logger: logger}
}

// RequireAuth is middleware that requires authentication.
func (m *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, err := m.auth.GetUserID(r)
		if err != nil {
			if m.logger != nil {
				m.logger.Warn().
					Str("remote_addr", r.RemoteAddr).
					Str("path", r.URL.Path).
					Err(err).
					Msg("auth failed")
			}
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// Add userID to context через helper
		ctx := WithUserID(r.Context(), userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetUserID extracts user ID from context.
// Deprecated: используйте FromContext для явной проверки наличия.
func GetUserID(ctx context.Context) string {
	id, _ := FromContext(ctx)
	return id
}

// ExtractBearerToken extracts token from Authorization header.
func ExtractBearerToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}
	return ""
}
