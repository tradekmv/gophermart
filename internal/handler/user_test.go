package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/tradekmv/gophermart.git/internal/middleware"
	"github.com/tradekmv/gophermart.git/internal/model"
	"github.com/tradekmv/gophermart.git/internal/service"
)

type MockUserService struct {
	registerResp string
	registerErr  error
	loginResp    string
	loginErr     error
}

func (m *MockUserService) Register(ctx context.Context, login, password string) (string, error) {
	return m.registerResp, m.registerErr
}

func (m *MockUserService) Login(ctx context.Context, login, password string) (string, error) {
	return m.loginResp, m.loginErr
}

type MockAuthForHandler struct {
	token    string
	tokenErr error
}

func (m *MockAuthForHandler) GenerateToken(userID string) (string, error) {
	if m.tokenErr != nil {
		return "", m.tokenErr
	}
	return m.token, nil
}

func (m *MockAuthForHandler) ValidateToken(tokenString string) (string, error) {
	return "user-from-token", nil
}

func (m *MockAuthForHandler) SetCookie(w http.ResponseWriter, token string, isProduction bool) {
	http.SetCookie(w, &http.Cookie{Name: "session", Value: token})
}

func (m *MockAuthForHandler) GetUserID(r *http.Request) (string, error) {
	return "", nil
}

func TestUserHandler_Register_InvalidContentType(t *testing.T) {
	logger := zerolog.Nop()
	reqBody := model.RegisterRequest{Login: "test", Password: "password"}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()
	h := &UserHandler{service: nil, auth: nil, logger: &logger, isProduction: false}
	h.Register(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestUserHandler_Register_InvalidJSON(t *testing.T) {
	logger := zerolog.Nop()
	req := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h := &UserHandler{service: nil, auth: nil, logger: &logger, isProduction: false}
	h.Register(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestUserHandler_Login_InvalidContentType(t *testing.T) {
	logger := zerolog.Nop()
	reqBody := model.LoginRequest{Login: "test", Password: "password"}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/user/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()
	h := &UserHandler{service: nil, auth: nil, logger: &logger, isProduction: false}
	h.Login(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestUserHandler_Login_InvalidJSON(t *testing.T) {
	logger := zerolog.Nop()
	req := httptest.NewRequest(http.MethodPost, "/api/user/login", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h := &UserHandler{service: nil, auth: nil, logger: &logger, isProduction: false}
	h.Login(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestUserHandler_Register_Success(t *testing.T) {
	logger := zerolog.Nop()
	reqBody := model.RegisterRequest{Login: "testuser", Password: "password123"}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mockService := &MockUserService{registerResp: "user-123"}
	mockAuth := &MockAuthForHandler{token: "valid-jwt-token"}
	h := &UserHandler{service: mockService, auth: mockAuth, logger: &logger}
	h.Register(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rr.Code)
	}
	if len(rr.Result().Cookies()) == 0 {
		t.Error("expected cookie")
	}
}

func TestUserHandler_Register_UserExists(t *testing.T) {
	logger := zerolog.Nop()
	reqBody := model.RegisterRequest{Login: "existinguser", Password: "password123"}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mockService := &MockUserService{registerErr: service.ErrUserExists}
	h := &UserHandler{service: mockService, auth: &MockAuthForHandler{}, logger: &logger}
	h.Register(rr, req)
	if rr.Code != http.StatusConflict {
		t.Errorf("expected %d, got %d", http.StatusConflict, rr.Code)
	}
}

func TestUserHandler_Register_InvalidCredentials(t *testing.T) {
	logger := zerolog.Nop()
	reqBody := model.RegisterRequest{Login: "", Password: "password123"}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mockService := &MockUserService{registerErr: service.ErrInvalidCredentials}
	h := &UserHandler{service: mockService, auth: &MockAuthForHandler{}, logger: &logger}
	h.Register(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestUserHandler_Register_TokenError(t *testing.T) {
	logger := zerolog.Nop()
	reqBody := model.RegisterRequest{Login: "testuser", Password: "password123"}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mockService := &MockUserService{registerResp: "user-123"}
	mockAuth := &MockAuthForHandler{tokenErr: errors.New("token failed")}
	h := &UserHandler{service: mockService, auth: mockAuth, logger: &logger}
	h.Register(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected %d, got %d", http.StatusInternalServerError, rr.Code)
	}
}

func TestUserHandler_Register_InternalError(t *testing.T) {
	logger := zerolog.Nop()
	reqBody := model.RegisterRequest{Login: "testuser", Password: "password123"}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mockService := &MockUserService{registerErr: errors.New("db error")}
	h := &UserHandler{service: mockService, auth: &MockAuthForHandler{}, logger: &logger}
	h.Register(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected %d, got %d", http.StatusInternalServerError, rr.Code)
	}
}

func TestUserHandler_Login_Success(t *testing.T) {
	logger := zerolog.Nop()
	reqBody := model.LoginRequest{Login: "testuser", Password: "password123"}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/user/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mockService := &MockUserService{loginResp: "user-456"}
	mockAuth := &MockAuthForHandler{token: "valid-jwt-token"}
	h := &UserHandler{service: mockService, auth: mockAuth, logger: &logger}
	h.Login(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rr.Code)
	}
	if len(rr.Result().Cookies()) == 0 {
		t.Error("expected cookie")
	}
}

func TestUserHandler_Login_InvalidCredentials(t *testing.T) {
	logger := zerolog.Nop()
	reqBody := model.LoginRequest{Login: "wronguser", Password: "wrongpassword"}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/user/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mockService := &MockUserService{loginErr: service.ErrInvalidCredentials}
	h := &UserHandler{service: mockService, auth: &MockAuthForHandler{}, logger: &logger}
	h.Login(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestUserHandler_Login_InternalError(t *testing.T) {
	logger := zerolog.Nop()
	reqBody := model.LoginRequest{Login: "testuser", Password: "password123"}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/user/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mockService := &MockUserService{loginErr: errors.New("db error")}
	h := &UserHandler{service: mockService, auth: &MockAuthForHandler{}, logger: &logger}
	h.Login(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected %d, got %d", http.StatusInternalServerError, rr.Code)
	}
}

func TestUserHandler_Login_TokenError(t *testing.T) {
	logger := zerolog.Nop()
	reqBody := model.LoginRequest{Login: "testuser", Password: "password123"}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/user/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mockService := &MockUserService{loginResp: "user-789"}
	mockAuth := &MockAuthForHandler{tokenErr: errors.New("jwt failed")}
	h := &UserHandler{service: mockService, auth: mockAuth, logger: &logger}
	h.Login(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected %d, got %d", http.StatusInternalServerError, rr.Code)
	}
}

func TestGetUserIDFromContext(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := middleware.WithUserID(req.Context(), "test-user-123")
	req = req.WithContext(ctx)
	userID := getUserIDFromContext(req.Context())
	if userID != "test-user-123" {
		t.Errorf("expected 'test-user-123', got '%s'", userID)
	}
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	userID2 := getUserIDFromContext(req2.Context())
	if userID2 != "" {
		t.Errorf("expected empty, got '%s'", userID2)
	}
}
