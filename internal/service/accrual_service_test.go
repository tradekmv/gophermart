package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"go.uber.org/mock/gomock"

	"github.com/tradekmv/gophermart.git/internal/repository/storage"
	storagemock "github.com/tradekmv/gophermart.git/internal/repository/storage/mock"
	"github.com/tradekmv/gophermart.git/pkg/accrual"
	accrualmock "github.com/tradekmv/gophermart.git/pkg/accrual/mock"
)

// nopLogger — no-op логгер для всех тестов.
// Используем именно zerolog.Nop(), а не nil: компоненты рассчитаны на то,
// что *zerolog.Logger всегда валиден (см. NewAccrualService и др.).
var nopLogger = zerolog.Nop()

func TestAccrualService_New(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := storagemock.NewMockStorage(ctrl)
	client := accrualmock.NewMockClientInterface(ctrl)

	svc := NewAccrualService(store, client, 1, 5, &nopLogger)
	if svc == nil {
		t.Fatal("NewAccrualService() returned nil")
	}
	if svc.storage != store {
		t.Error("storage not set correctly")
	}
	if svc.client != client {
		t.Error("client not set correctly")
	}
}

func TestAccrualService_StartWithoutClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := storagemock.NewMockStorage(ctrl)

	svc := NewAccrualService(store, nil, 1, 5, &nopLogger)

	// Не должно стартовать и падать: если client == nil, Start ничего не делает.
	svc.Start(context.Background())
}

func TestAccrualService_Sync_Processed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := storagemock.NewMockStorage(ctrl)
	client := accrualmock.NewMockClientInterface(ctrl)

	order := storage.Order{
		ID:     "order-1",
		UserID: "user-1",
		Number: "79927398713",
		Status: "NEW",
	}

	store.EXPECT().
		GetPendingOrders(gomock.Any()).
		Return([]storage.Order{order}, nil)

	client.EXPECT().
		Fetch(gomock.Any(), order.Number).
		Return(accrual.AccrualInfo{Status: "PROCESSED", Accrual: 150.50}, nil)

	store.EXPECT().
		AddAccrual(gomock.Any(), order.UserID, order.Number, 150.50).
		Return(nil)

	svc := NewAccrualService(store, client, 1, 5, &nopLogger)
	svc.sync(context.Background())
}

func TestAccrualService_Sync_FetchError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := storagemock.NewMockStorage(ctrl)
	client := accrualmock.NewMockClientInterface(ctrl)

	order := storage.Order{
		ID:     "order-1",
		UserID: "user-1",
		Number: "79927398713",
		Status: "NEW",
	}

	store.EXPECT().
		GetPendingOrders(gomock.Any()).
		Return([]storage.Order{order}, nil)

	client.EXPECT().
		Fetch(gomock.Any(), order.Number).
		Return(accrual.AccrualInfo{}, context.DeadlineExceeded)

	// AddAccrual / UpdateOrderByNumber НЕ должны вызываться при ошибке Fetch.

	svc := NewAccrualService(store, client, 1, 5, &nopLogger)
	svc.sync(context.Background())
}

func TestAccrualService_Sync_RegisteredMapsToProcessing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := storagemock.NewMockStorage(ctrl)
	client := accrualmock.NewMockClientInterface(ctrl)

	order := storage.Order{
		ID:     "order-1",
		UserID: "user-1",
		Number: "79927398713",
		Status: "NEW",
	}

	store.EXPECT().
		GetPendingOrders(gomock.Any()).
		Return([]storage.Order{order}, nil)

	client.EXPECT().
		Fetch(gomock.Any(), order.Number).
		Return(accrual.AccrualInfo{Status: "REGISTERED", Accrual: 0}, nil)

	// accrual=0, поэтому AddAccrual не вызывается — только UpdateOrderByNumber с nil accrual.
	var nilPtr *float64
	store.EXPECT().
		UpdateOrderByNumber(gomock.Any(), order.Number, "PROCESSING", nilPtr).
		Return(nil)

	svc := NewAccrualService(store, client, 1, 5, &nopLogger)
	svc.sync(context.Background())
}

func TestAccrualService_Sync_UpdateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := storagemock.NewMockStorage(ctrl)
	client := accrualmock.NewMockClientInterface(ctrl)

	order := storage.Order{
		ID:     "order-1",
		UserID: "user-1",
		Number: "79927398713",
		Status: "NEW",
	}

	store.EXPECT().
		GetPendingOrders(gomock.Any()).
		Return([]storage.Order{order}, nil)

	client.EXPECT().
		Fetch(gomock.Any(), order.Number).
		Return(accrual.AccrualInfo{Status: "PROCESSING", Accrual: 0}, nil)

	store.EXPECT().
		UpdateOrderByNumber(gomock.Any(), order.Number, "PROCESSING", nil).
		Return(context.DeadlineExceeded)

	svc := NewAccrualService(store, client, 1, 5, &nopLogger)
	svc.sync(context.Background())
}

func TestAccrualService_Stop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := storagemock.NewMockStorage(ctrl)
	client := accrualmock.NewMockClientInterface(ctrl)

	// Start вызывает sync() один раз сразу при старте. Допускаем любой
	// результат — нас интересует только факт корректной остановки.
	store.EXPECT().
		GetPendingOrders(gomock.Any()).
		Return(nil, nil).
		AnyTimes()

	svc := NewAccrualService(store, client, 1, 5, &nopLogger)
	svc.Start(context.Background())

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()

	done := make(chan error, 1)
	go func() {
		done <- svc.Stop(stopCtx)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Stop() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Stop() timed out")
	}
}

func TestAccrualService_RateLimitCoordinates(t *testing.T) {
	// Тест: при 429 на одном воркере остальные воркеры быстро завершают
	// работу (видит rlCtx.Done()), а sync() ждёт Retry-After.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := storagemock.NewMockStorage(ctrl)
	client := accrualmock.NewMockClientInterface(ctrl)

	// 3 заказа — воркеры 1 и 2 должны получить Fetch и затем выйти
	// по rlCtx.Done() (без вызова AddAccrual/UpdateOrderByNumber).
	orders := []storage.Order{
		{ID: "o1", UserID: "u1", Number: "79927398713", Status: "NEW"},
		{ID: "o2", UserID: "u1", Number: "1234567890", Status: "NEW"},
		{ID: "o3", UserID: "u1", Number: "1111111111", Status: "NEW"},
	}

	store.EXPECT().
		GetPendingOrders(gomock.Any()).
		Return(orders, nil)

	// Каждый Fetch возвращает RateLimitError с RetryAfter=50ms.
	// Несколько воркеров могут успеть вызвать Fetch ДО того, как
	// rlCancel дойдёт — это нормально для worker pool, и все Fetch
	// просто возвратят rate-limit, после чего main sync задержится
	// на Retry-After.
	client.EXPECT().
		Fetch(gomock.Any(), gomock.Any()).
		Return(accrual.AccrualInfo{}, &accrual.RateLimitError{RetryAfter: 50 * time.Millisecond}).
		AnyTimes()

	svc := NewAccrualService(store, client, 1, 5, &nopLogger)

	start := time.Now()
	svc.sync(context.Background())
	elapsed := time.Since(start)

	// sync должен был подождать ~50ms после rate-limit (или больше — воркеры
	// могли пропустить сигнал и зайти в Fetch).
	if elapsed < 30*time.Millisecond {
		t.Errorf("sync() returned too early: %s, want >= 50ms", elapsed)
	}
}

func TestAccrualService_Stop_Timeout(t *testing.T) {
	// Stop(ctx) должен вернуть ctx.Err() если sync не завершился за отведённое время.
	// Симулируем «зависший» sync через Active=true (без запуска воркеров).
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := storagemock.NewMockStorage(ctrl)
	svc := NewAccrualService(store, nil, 1, 1, &nopLogger)

	// Эмулируем активный sync.
	svc.syncMu.Lock()
	svc.syncActive = true
	svc.syncWG.Add(1)
	svc.syncMu.Unlock()
	defer svc.syncWG.Done()

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer stopCancel()

	err := svc.Stop(stopCtx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Stop() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestMapAccrualStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"REGISTERED", "PROCESSING"},
		{"PROCESSING", "PROCESSING"},
		{"INVALID", "INVALID"},
		{"PROCESSED", "PROCESSED"},
		{"UNKNOWN", "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := mapAccrualStatus(tt.input); got != tt.expected {
				t.Errorf("mapAccrualStatus(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
