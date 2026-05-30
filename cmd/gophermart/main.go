// Package main is the entry point for gophermart service.
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/tradekmv/gophermart.git/internal/config"
	"github.com/tradekmv/gophermart.git/internal/handler"
	"github.com/tradekmv/gophermart.git/internal/middleware"
	"github.com/tradekmv/gophermart.git/internal/repository/storage"
	"github.com/tradekmv/gophermart.git/internal/service"
	accrualClient "github.com/tradekmv/gophermart.git/pkg/accrual"
	accrualAdapter "github.com/tradekmv/gophermart.git/pkg/accrual/adapter"
	"github.com/tradekmv/gophermart.git/pkg/auth"
	"github.com/tradekmv/gophermart.git/pkg/logger"
)

func main() {
	// Initialize logger
	appLogger := logger.Init()

	// Load configuration
	cfg := config.Load()

	// Validate required configuration
	if cfg.DatabaseURI == "" {
		appLogger.Fatal().Msg("DATABASE_URI is required")
	}

	// Connect to database
	store, err := storage.NewPostgresStorage(cfg.DatabaseURI)
	if err != nil {
		appLogger.Fatal().Err(err).Msg("connect to database")
	}
	defer store.Close()
	appLogger.Info().Msg("connected to database")

	// Initialize auth
	authService := auth.New(auth.Config{
		SecretKey: cfg.AuthSecretKey,
		RunEnv:    cfg.RunEnv,
	})

	// Initialize services
	userService := service.NewUserService(store)
	orderService := service.NewOrderService(store)
	balanceService := service.NewBalanceService(store)

	// Initialize accrual client and service
	var accrualService *service.AccrualService

	if cfg.AccrualSystemAddress != "" {
		client := accrualClient.NewClient(cfg.AccrualSystemAddress)
		svcAdapter := accrualAdapter.NewClient(client)
		accrualService = service.NewAccrualService(store, svcAdapter, cfg.AccrualInterval, appLogger)
		appLogger.Info().Str("address", cfg.AccrualSystemAddress).Int("interval_sec", cfg.AccrualInterval).Msg("accrual system configured")
	}

	// Initialize handlers
	userHandler := handler.NewUserHandler(userService, authService, appLogger, cfg.IsProduction())
	orderHandler := handler.NewOrderHandler(orderService, appLogger)
	balanceHandler := handler.NewBalanceHandler(balanceService, appLogger)
	withdrawalHandler := handler.NewWithdrawalHandler(balanceService, appLogger)

	// Initialize auth middleware
	authMiddleware := middleware.NewAuthMiddleware(authService)

	// Setup router
	router := chi.NewRouter()

	// Global middleware
	router.Use(chiMiddleware.RequestID)
	router.Use(chiMiddleware.RealIP)
	router.Use(middleware.CompressMiddleware)
	router.Use(middleware.LoggingMiddleware(appLogger))
	router.Use(chiMiddleware.Recoverer)

	// Public routes
	router.Route("/api/user", func(r chi.Router) {
		r.Post("/register", userHandler.Register)
		r.Post("/login", userHandler.Login)
	})

	// Protected routes
	router.Group(func(r chi.Router) {
		r.Use(authMiddleware.RequireAuth)

		r.Post("/api/user/orders", orderHandler.UploadOrder)
		r.Get("/api/user/orders", orderHandler.GetOrders)
		r.Get("/api/user/balance", balanceHandler.GetBalance)
		r.Post("/api/user/balance/withdraw", balanceHandler.Withdraw)
		r.Get("/api/user/withdrawals", withdrawalHandler.GetWithdrawals)
	})

	// Health check
	router.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Create HTTP server
	server := &http.Server{
		Addr:         cfg.ServerAddress,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start accrual service
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if accrualService != nil {
		accrualService.Start(ctx)
		appLogger.Info().Msg("accrual service started")
	}

	// Start server in goroutine
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appLogger.Fatal().Err(err).Msg("server failed")
		}
	}()

	appLogger.Info().Str("address", cfg.ServerAddress).Msg("server started")

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	appLogger.Info().Msg("shutting down server...")

	// Stop accrual service
	if accrualService != nil {
		accrualService.Stop()
		appLogger.Info().Msg("accrual service stopped")
	}

	// Cancel context
	cancel()

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		appLogger.Error().Err(err).Msg("server shutdown error")
	}

	appLogger.Info().Msg("server stopped")
}
