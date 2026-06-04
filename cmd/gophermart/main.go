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
	"github.com/tradekmv/gophermart.git/pkg/accrual"
	"github.com/tradekmv/gophermart.git/pkg/auth"
	"github.com/tradekmv/gophermart.git/pkg/logger"
)

func main() {
	// Initialize logger
	appLogger := logger.Init()

	// Дочерние логгеры для DI в компоненты. Каждый получает поле `component`
	// для фильтрации в логах. Все компоненты получают *zerolog.Logger,
	// а не глобальный singleton.
	handlerLogger := appLogger.With().Str("component", "handler").Logger()
	accrualLogger := appLogger.With().Str("component", "accrual").Logger()
	storageLogger := appLogger.With().Str("component", "storage").Logger()
	mwLogger := appLogger.With().Str("component", "middleware").Logger()

	// Load configuration
	cfg := config.Load()

	// Validate required configuration
	if cfg.DatabaseURI == "" {
		appLogger.Fatal().Msg("DATABASE_URI is required")
	}

	// Connect to database
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store, err := storage.NewPostgresStorage(ctx, cfg.DatabaseURI, &storageLogger)
	if err != nil {
		appLogger.Fatal().Err(err).Msg("connect to database")
	}
	// defer закрывает БД в порядке LIFO: сначала отменится root context,
	// затем освободится shutdown context, затем закроется пул.
	defer func() {
		if err := store.Close(); err != nil {
			appLogger.Error().Err(err).Msg("storage close error")
		}
	}()
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
		client := accrual.NewClient(cfg.AccrualSystemAddress)
		// *accrual.Client реализует accrual.ClientInterface — адаптер больше не нужен.
		accrualService = service.NewAccrualService(store, client, cfg.AccrualInterval, cfg.AccrualWorkers, &accrualLogger)
		appLogger.Info().
			Str("address", cfg.AccrualSystemAddress).
			Int("interval_sec", cfg.AccrualInterval).
			Int("workers", cfg.AccrualWorkers).
			Msg("accrual system configured")
	}

	// Initialize handlers
	userHandler := handler.NewUserHandler(userService, authService, &handlerLogger, cfg.IsProduction())
	orderHandler := handler.NewOrderHandler(orderService, &handlerLogger)
	balanceHandler := handler.NewBalanceHandler(balanceService, &handlerLogger)
	withdrawalHandler := handler.NewWithdrawalHandler(balanceService, &handlerLogger)

	// Initialize auth middleware (8.7.1: прокидываем логгер для логирования 401)
	authMiddleware := middleware.NewAuthMiddleware(authService, &mwLogger)

	// Setup router
	router := chi.NewRouter()

	// Global middleware
	router.Use(chiMiddleware.RequestID)
	router.Use(chiMiddleware.RealIP)
	router.Use(middleware.CompressMiddleware)
	router.Use(middleware.LoggingMiddleware(&mwLogger))
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
	if accrualService != nil {
		accrualService.Start(ctx)
		appLogger.Info().Msg("accrual service started")
	}

	// Start server in goroutine. Любая ошибка старта (не graceful shutdown)
	// считается фатальной — отдаём её через канал, а не вызываем log.Fatal
	// из горутины (это небезопасно согласно https://pkg.go.dev/log#Fatal).
	serverErr := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
		close(serverErr)
	}()

	appLogger.Info().Str("address", cfg.ServerAddress).Msg("server started")

	// Wait for interrupt signal OR server startup error
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		appLogger.Info().Str("signal", sig.String()).Msg("shutting down server...")
	case err := <-serverErr:
		appLogger.Error().Err(err).Msg("server failed")
	}

	// Сначала останавливаем accrual — у него могут быть активные HTTP-запросы
	// к внешнему сервису. Передаём stop-контекст с таймаутом, чтобы Stop
	// не завис.
	if accrualService != nil {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := accrualService.Stop(stopCtx); err != nil {
			appLogger.Error().Err(err).Msg("accrual service stop error")
		}
		stopCancel()
		appLogger.Info().Msg("accrual service stopped")
	}

	// Cancel root context — это остановит фоновую горутину Start, если
	// Stop не дождался (например, по таймауту).
	cancel()

	// Graceful shutdown HTTP-сервера с таймаутом.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		appLogger.Error().Err(err).Msg("server shutdown error")
	}

	// store.Close() выполнится в defer (LIFO), гарантируя закрытие пула
	// строго после server.Shutdown и accrual.Stop.

	appLogger.Info().Msg("server stopped")
}
