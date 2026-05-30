// Package service implements business logic for the gophermart service.
package service

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/tradekmv/gophermart.git/internal/repository/storage"
)

// AccrualClient interface for fetching accrual data.
type AccrualClient interface {
	Fetch(ctx context.Context, orderNumber string) (AccrualInfo, error)
}

// AccrualInfo contains accrual data from external system.
type AccrualInfo struct {
	Status  string
	Accrual float64
}

// AccrualService handles background synchronization with accrual system.
type AccrualService struct {
	storage  storage.Storage
	client   AccrualClient
	ticker   *time.Ticker
	stopChan chan struct{}
	wg       sync.WaitGroup
	stopOnce sync.Once
	interval time.Duration
	logger   *zerolog.Logger
}

// NewAccrualService creates a new AccrualService with configurable interval.
func NewAccrualService(s storage.Storage, client AccrualClient, intervalSec int, logger *zerolog.Logger) *AccrualService {
	interval := time.Duration(intervalSec) * time.Second
	if interval <= 0 {
		interval = 1 * time.Second // default 1 second
	}
	return &AccrualService{
		storage:  s,
		client:   client,
		stopChan: make(chan struct{}),
		interval: interval,
		logger:   logger,
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

// sync performs one synchronization cycle.
func (s *AccrualService) sync(ctx context.Context) {
	orders, err := s.storage.GetPendingOrders(ctx)
	if err != nil {
		if s.logger != nil {
			s.logger.Error().Err(err).Msg("accrual sync: get pending orders")
		}
		return
	}

	for _, order := range orders {
		if s.client == nil {
			continue
		}

		accrualInfo, err := s.client.Fetch(ctx, order.Number)
		if err != nil {
			if s.logger != nil {
				s.logger.Error().Err(err).Str("order", order.Number).Msg("accrual sync: fetch order")
			}
			continue
		}

		internalStatus := mapAccrualStatus(accrualInfo.Status)

		if internalStatus == "PROCESSED" && accrualInfo.Accrual > 0 {
			// AddAccrual atomically updates both balance and order status + accrual
			if err := s.storage.AddAccrual(ctx, order.UserID, order.Number, accrualInfo.Accrual); err != nil {
				if s.logger != nil {
					s.logger.Error().Err(err).Str("order", order.Number).Msg("accrual sync: add accrual")
				}
			}
		} else {
			var accrual *float64
			if accrualInfo.Accrual > 0 {
				accrual = &accrualInfo.Accrual
			}
			if err := s.storage.UpdateOrderByNumber(ctx, order.Number, internalStatus, accrual); err != nil {
				if s.logger != nil {
					s.logger.Error().Err(err).Str("order", order.Number).Msg("accrual sync: update order")
				}
			}
		}
	}
}

// Start begins the background synchronization.
func (s *AccrualService) Start(ctx context.Context) {
	if s.client == nil {
		return
	}

	s.ticker = time.NewTicker(s.interval)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		// Initial sync
		s.sync(ctx)

		for {
			select {
			case <-s.ticker.C:
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
func (s *AccrualService) Stop() {
	s.stopOnce.Do(func() {
		if s.ticker != nil {
			s.ticker.Stop()
		}
		close(s.stopChan)
		s.wg.Wait()
	})
}
