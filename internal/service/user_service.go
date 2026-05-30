// Package service implements business logic for the gophermart service.
package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/tradekmv/gophermart.git/internal/repository/storage"
	"golang.org/x/crypto/bcrypt"
)

var (
	// ErrUserExists is returned when user already exists.
	ErrUserExists = errors.New("user already exists")
	// ErrInvalidCredentials is returned when login or password is invalid.
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrNotFound is returned when resource is not found.
	ErrNotFound = errors.New("not found")
)

// UserService handles user-related business logic.
type UserService struct {
	storage storage.Storage
}

// NewUserService creates a new UserService.
func NewUserService(storage storage.Storage) *UserService {
	return &UserService{storage: storage}
}

// Register creates a new user account.
func (s *UserService) Register(ctx context.Context, login, password string) (string, error) {
	// Validate input
	if login == "" || password == "" {
		return "", ErrInvalidCredentials
	}

	// Check if user already exists
	_, err := s.storage.GetUserByLogin(ctx, login)
	if err == nil {
		return "", ErrUserExists
	}
	if !errors.Is(err, storage.ErrNotFound) {
		return "", fmt.Errorf("check user: %w", err)
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	// Create user with balance
	userID, err := s.storage.CreateUserWithBalance(ctx, login, string(hash))
	if err != nil {
		if errors.Is(err, storage.ErrUserExists) {
			return "", ErrUserExists
		}
		return "", fmt.Errorf("create user: %w", err)
	}

	return userID, nil
}

// Login authenticates a user and returns their ID.
func (s *UserService) Login(ctx context.Context, login, password string) (string, error) {
	// Validate input
	if login == "" || password == "" {
		return "", ErrInvalidCredentials
	}

	// Get user by login
	user, err := s.storage.GetUserByLogin(ctx, login)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return "", ErrInvalidCredentials
		}
		return "", fmt.Errorf("get user: %w", err)
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", ErrInvalidCredentials
	}

	return user.ID, nil
}
