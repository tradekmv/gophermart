// Package service implements business logic for the gophermart service.
package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"

	"github.com/tradekmv/gophermart.git/internal/repository/storage"
	"github.com/tradekmv/gophermart.git/pkg/accrual"
)

// ErrAccrualStopped is returned by processOrder when the service is shutting down.
var ErrAccrualStopped = errors.New("accrual service stopped")

// AccrualClient — алиас для accrual.ClientInterface.
// Используется в DI, чтобы не зависеть от конкретного типа клиента
// и легко подменять в тестах через pkg/accrual/mock.
type AccrualClient = accrual.ClientInterface

// AccrualService handles background synchronization with accrual system.
type AccrualService struct {
	storage  storage.Storage
	client   AccrualClient
	stopChan chan struct{}
	stopOnce sync.Once
	interval time.Duration
	logger   *zerolog.Logger

	// Worker pool: параллельная обработка pending-заказов через N воркеров.
	// При 429 все воркеры останавливаются и ждут Retry-After.
	workerCount int

	// Graceful shutdown — Stop(ctx) дожидается завершения активного sync.
	syncMu     sync.Mutex
	syncWG     sync.WaitGroup
	syncActive bool
}

// NewAccrualService creates a new AccrualService with configurable interval and workers.
func NewAccrualService(s storage.Storage, client AccrualClient, intervalSec, workerCount int, logger *zerolog.Logger) *AccrualService {
	interval := time.Duration(intervalSec) * time.Second
	if interval <= 0 {
		interval = 1 * time.Second // default 1 second
	}
	if workerCount <= 0 {
		workerCount = 5 // default
	}
	return &AccrualService{
		storage:     s,
		client:      client,
		stopChan:    make(chan struct{}),
		interval:    interval,
		logger:      logger,
		workerCount: workerCount,
	}
}

// mapAccrualStatus converts external status to internal status.
func mapAccrualStatus(external string) string {
	switch external {
	case "REGISTERED":
		return "PROCESSING"
	default:
		return external
	}
}

// sync performs one synchronization cycle через worker pool.
//
// Алгоритм:
//  1. Загружаем список pending-заказов из БД.
//  2. Запускаем workerCount воркеров, читающих из jobs-канала.
//  3. Распределяем заказы по воркерам.
//  4. Если воркер получает *RateLimitError, он через atomic.Pointer.CompareAndSwap
//     записывает время пробуждения (только первый!) и отменяет rlCtx — все
//     остальные воркеры видят rlCtx.Done() и быстро выходят.
//  5. После завершения воркеров main-goroutine ждёт delay (если был rate-limit).
func (s *AccrualService) sync(ctx context.Context) {
	orders, err := s.storage.GetPendingOrders(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("accrual sync: get pending orders")
		return
	}
	if len(orders) == 0 {
		return
	}

	// Регистрируем активный sync для graceful Stop(ctx).
	s.syncMu.Lock()
	if s.syncActive {
		// Уже идёт другой sync — не запускаем параллельный (можно снять
		// ограничение, если тесты начнут падать из-за гонки).
		s.syncMu.Unlock()
		return
	}
	s.syncActive = true
	s.syncWG.Add(1)
	s.syncMu.Unlock()
	defer func() {
		s.syncMu.Lock()
		s.syncActive = false
		s.syncMu.Unlock()
		s.syncWG.Done()
	}()

	rlCtx, rlCancel := context.WithCancel(ctx)
	defer rlCancel()

	// Atomic.Pointer: первый воркер, получивший 429, записывает время
	// пробуждения и отменяет rlCtx. Остальные воркеры увидят false в CAS
	// и не будут перезаписывать.
	var rlRetryAfter atomic.Pointer[time.Time]

	jobs := make(chan storage.Order)
	var wg sync.WaitGroup
	for i := 0; i < s.workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for order := range jobs {
				select {
				case <-rlCtx.Done():
					// rate-limit сработал — не обрабатываем оставшиеся
					continue
				case <-ctx.Done():
					return
				default:
				}
				s.processOrder(ctx, rlCtx, rlCancel, &rlRetryAfter, order)
			}
		}()
	}

loop:
	for _, o := range orders {
		select {
		case jobs <- o:
		case <-rlCtx.Done():
			break loop
		case <-ctx.Done():
			break loop
		}
	}
	close(jobs)
	wg.Wait()

	// happens-before через wg.Wait: безопасно читать rlRetryAfter.
	if p := rlRetryAfter.Load(); p != nil {
		delay := time.Until(*p)
		if delay > 0 {
			select {
			case <-ctx.Done():
			case <-time.After(delay):
			}
		}
	}
}

// processOrder обрабатывает один заказ. При *RateLimitError записывает
// retry-after в rlRetryAfter (через CAS) и отменяет rlCtx.
func (s *AccrualService) processOrder(
	ctx, rlCtx context.Context,
	rlCancel context.CancelFunc,
	rlRetryAfter *atomic.Pointer[time.Time],
	order storage.Order,
) {
	if s.client == nil {
		return
	}

	accrualInfo, err := s.client.Fetch(rlCtx, order.Number)
	if err != nil {
		var rle *accrual.RateLimitError
		if errors.As(err, &rle) {
			// Только первый воркер запишет delay и отменит rlCtx.
			delay := time.Now().Add(rle.RetryAfter)
			if rlRetryAfter.CompareAndSwap(nil, &delay) {
				rlCancel()
			}
			return
		}
		s.logger.Error().Err(err).Str("order", order.Number).Msg("accrual sync: fetch order")
		return
	}

	internalStatus := mapAccrualStatus(accrualInfo.Status)

	if internalStatus == "PROCESSED" && accrualInfo.Accrual > 0 {
		// AddAccrual atomically updates both balance and order status + accrual
		if err := s.storage.AddAccrual(ctx, order.UserID, order.Number, accrualInfo.Accrual); err != nil {
			s.logger.Error().Err(err).Str("order", order.Number).Msg("accrual sync: add accrual")
		}
	} else {
		var accrualPtr *float64
		if accrualInfo.Accrual > 0 {
			accrualPtr = &accrualInfo.Accrual
		}
		if err := s.storage.UpdateOrderByNumber(ctx, order.Number, internalStatus, accrualPtr); err != nil {
			s.logger.Error().Err(err).Str("order", order.Number).Msg("accrual sync: update order")
		}
	}
}

// Start begins the background synchronization.
func (s *AccrualService) Start(ctx context.Context) {
	if s.client == nil {
		return
	}

	ticker := time.NewTicker(s.interval)

	go func() {
		defer ticker.Stop()

		// Initial sync
		s.sync(ctx)

		for {
			select {
			case <-ticker.C:
				s.sync(ctx)
			case <-s.stopChan:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Stop gracefully stops the synchronization.
// Принимает ctx с таймаутом, чтобы не зависнуть при активном HTTP-запросе
// к accrual. Возвращает ctx.Err(), если таймаут истёк до завершения sync.
func (s *AccrualService) Stop(ctx context.Context) error {
	s.stopOnce.Do(func() {
		close(s.stopChan)
	})

	done := make(chan struct{})
	go func() {
		s.syncWG.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("accrual stop: %w", ctx.Err())
	}
}
