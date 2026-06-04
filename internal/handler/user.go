// Package handler provides HTTP handlers for the gophermart service.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/rs/zerolog"
	"github.com/tradekmv/gophermart.git/internal/middleware"
	"github.com/tradekmv/gophermart.git/internal/model"
	"github.com/tradekmv/gophermart.git/internal/service"
	"github.com/tradekmv/gophermart.git/pkg/auth"
)

// UserServiceInterface defines the interface for user operations
type UserServiceInterface interface {
	Register(ctx context.Context, login, password string) (string, error)
	Login(ctx context.Context, login, password string) (string, error)
}

// AuthInterface defines the interface for authentication operations
type AuthInterface interface {
	GenerateToken(userID string) (string, error)
	ValidateToken(tokenString string) (string, error)
	SetCookie(w http.ResponseWriter, token string, isProduction bool)
	GetUserID(r *http.Request) (string, error)
}

// UserHandler handles user-related HTTP requests.
type UserHandler struct {
	service      UserServiceInterface
	auth         AuthInterface
	logger       *zerolog.Logger
	isProduction bool
}

// NewUserHandler creates a new UserHandler.
func NewUserHandler(svc *service.UserService, authSvc *auth.Auth, logger *zerolog.Logger, isProduction bool) *UserHandler {
	return &UserHandler{
		service:      svc,
		auth:         authSvc,
		logger:       logger,
		isProduction: isProduction,
	}
}

// Register handles user registration.
func (h *UserHandler) Register(w http.ResponseWriter, r *http.Request) {
	// Check Content-Type
	ct := r.Header.Get("Content-Type")
	if ct != "" && !strings.HasPrefix(ct, "application/json") {
		http.Error(w, "Content-Type must be application/json", http.StatusBadRequest)
		return
	}

	// Decode request
	var req model.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Register user
	userID, err := h.service.Register(r.Context(), req.Login, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrUserExists) {
			http.Error(w, "user already exists", http.StatusConflict)
			return
		}
		if errors.Is(err, service.ErrInvalidCredentials) {
			http.Error(w, "invalid credentials", http.StatusBadRequest)
			return
		}
		h.logger.Error().Err(err).Msg("register user")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Generate token and set cookie
	token, err := h.auth.GenerateToken(userID)
	if err != nil {
		h.logger.Error().Err(err).Msg("generate token")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	h.auth.SetCookie(w, token, h.isProduction)
	w.WriteHeader(http.StatusOK)
}

// Login handles user authentication.
func (h *UserHandler) Login(w http.ResponseWriter, r *http.Request) {
	// Check Content-Type
	ct := r.Header.Get("Content-Type")
	if ct != "" && !strings.HasPrefix(ct, "application/json") {
		http.Error(w, "Content-Type must be application/json", http.StatusBadRequest)
		return
	}

	// Decode request
	var req model.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Login user
	userID, err := h.service.Login(r.Context(), req.Login, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		h.logger.Error().Err(err).Msg("login user")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Generate token and set cookie
	token, err := h.auth.GenerateToken(userID)
	if err != nil {
		h.logger.Error().Err(err).Msg("generate token")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	h.auth.SetCookie(w, token, h.isProduction)
	w.WriteHeader(http.StatusOK)
}

// getUserIDFromContext extracts user ID from request context.
func getUserIDFromContext(ctx context.Context) string {
	return middleware.GetUserID(ctx)
}
