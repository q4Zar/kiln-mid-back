package application

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/q4ZAr/kiln-mid-back/tezos-delegation-service/internal/domain"
	"github.com/q4ZAr/kiln-mid-back/tezos-delegation-service/internal/infrastructure/tzkt"
	"github.com/q4ZAr/kiln-mid-back/tezos-delegation-service/pkg/config"
	"github.com/q4ZAr/kiln-mid-back/tezos-delegation-service/pkg/logger"
	"github.com/q4ZAr/kiln-mid-back/tezos-delegation-service/pkg/metrics"
	"golang.org/x/sync/errgroup"
)

type Service struct {
	repo           domain.DelegationRepository
	tzktClient     *tzkt.Client
	config         *config.TzktAPI
	logger         *logger.Logger
	pollingTicker  *time.Ticker
	stopPolling    chan struct{}
	pollingStarted bool
	mu             sync.RWMutex
}

func NewService(
	repo domain.DelegationRepository,
	tzktClient *tzkt.Client,
	config *config.TzktAPI,
	logger *logger.Logger,
) *Service {
	return &Service{
		repo:        repo,
		tzktClient:  tzktClient,
		config:      config,
		logger:      logger,
		stopPolling: make(chan struct{}),
	}
}

func (s *Service) GetDelegations(year *int) ([]domain.Delegation, error) {
	return s.repo.FindAll(year)
}

func (s *Service) IndexDelegations(fromLevel int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	batchSize := 100
	currentLevel := fromLevel

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		delegations, err := s.tzktClient.GetDelegationsFromLevel(ctx, currentLevel, batchSize)
		if err != nil {
			s.logger.Errorw("Failed to fetch delegations", "error", err, "level", currentLevel)
			return fmt.Errorf("failed to fetch delegations from level %d: %w", currentLevel, err)
		}

		if len(delegations) == 0 {
			s.logger.Info("No more delegations to index")
			break
		}

		domainDelegations := s.convertToDomainDelegations(delegations)

		if err := s.repo.SaveBatch(domainDelegations); err != nil {
			s.logger.Errorw("Failed to save batch", "error", err)
			return fmt.Errorf("failed to save batch: %w", err)
		}

		lastDelegation := delegations[len(delegations)-1]
		currentLevel = lastDelegation.Level + 1

		s.logger.Infow("Indexed batch of delegations",
			"count", len(delegations),
			"lastLevel", lastDelegation.Level,
			"lastTimestamp", lastDelegation.Timestamp,
		)

		if len(delegations) < batchSize {
			break
		}

		time.Sleep(100 * time.Millisecond)
	}

	return nil
}

func (s *Service) StartPolling() error {
	s.mu.Lock()
	if s.pollingStarted {
		s.mu.Unlock()
		return fmt.Errorf("polling already started")
	}
	s.pollingStarted = true
	s.mu.Unlock()

	if s.config.HistoricalIndexing {
		s.logger.Info("Starting historical indexing...")
		if err := s.indexHistorical(); err != nil {
			s.logger.Errorw("Historical indexing failed", "error", err)
		} else {
			s.logger.Info("Historical indexing completed successfully")
		}
	}

	s.pollingTicker = time.NewTicker(s.config.PollingInterval)

	go s.pollLoop()

	s.logger.Infow("Polling started", "interval", s.config.PollingInterval)
	return nil
}

func (s *Service) StopPolling() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.pollingStarted {
		return
	}

	close(s.stopPolling)
	if s.pollingTicker != nil {
		s.pollingTicker.Stop()
	}
	s.pollingStarted = false
	s.logger.Info("Polling stopped")
}

func (s *Service) pollLoop() {
	s.pollOnce()

	for {
		select {
		case <-s.pollingTicker.C:
			s.pollOnce()
		case <-s.stopPolling:
			return
		}
	}
}

func (s *Service) pollOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	lastLevel, err := s.repo.GetLastIndexedLevel()
	if err != nil {
		s.logger.Errorw("Failed to get last indexed level", "error", err)
		metrics.PollingErrors.Inc()
		return
	}
	metrics.UpdateLastIndexedLevel(lastLevel)

	if lastLevel == 0 {
		thirtyDaysAgo := time.Now().Add(-30 * 24 * time.Hour)
		delegations, err := s.tzktClient.GetDelegationsSince(ctx, thirtyDaysAgo, 1000)
		if err != nil {
			s.logger.Errorw("Failed to fetch recent delegations", "error", err)
			return
		}

		if len(delegations) > 0 {
			domainDelegations := s.convertToDomainDelegations(delegations)
			if err := s.repo.SaveBatch(domainDelegations); err != nil {
				s.logger.Errorw("Failed to save delegations", "error", err)
				metrics.RecordDelegationProcessed("error")
			} else {
				s.logger.Infow("Saved recent delegations", "count", len(delegations))
				metrics.DelegationsStored.Add(float64(len(delegations)))
				metrics.RecordDelegationProcessed("success")
			}
		}
	} else {
		delegations, err := s.tzktClient.GetDelegationsFromLevel(ctx, lastLevel+1, 100)
		if err != nil {
			s.logger.Errorw("Failed to fetch new delegations", "error", err, "fromLevel", lastLevel+1)
			metrics.PollingErrors.Inc()
			return
		}

		if len(delegations) > 0 {
			domainDelegations := s.convertToDomainDelegations(delegations)
			if err := s.repo.SaveBatch(domainDelegations); err != nil {
				s.logger.Errorw("Failed to save new delegations", "error", err)
				metrics.RecordDelegationProcessed("error")
			} else {
				s.logger.Infow("Saved new delegations", "count", len(delegations), "fromLevel", lastLevel+1)
				metrics.DelegationsStored.Add(float64(len(delegations)))
				metrics.RecordDelegationProcessed("success")
				metrics.UpdateLastIndexedLevel(lastLevel + 1)
			}
		}
	}
}

func (s *Service) indexHistorical() error {
	startDate, err := time.Parse("2006-01-02", s.config.HistoricalStartDate)
	if err != nil {
		return fmt.Errorf("invalid historical start date: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()

	g, gctx := errgroup.WithContext(ctx)

	delegationsChan, errorChan := s.tzktClient.GetHistoricalDelegations(gctx, startDate, 500)

	processedCount := 0
	batchBuffer := make([]domain.Delegation, 0, 1000)

	g.Go(func() error {
		for {
			select {
			case delegations, ok := <-delegationsChan:
				if !ok {
					if len(batchBuffer) > 0 {
						if err := s.repo.SaveBatch(batchBuffer); err != nil {
							return fmt.Errorf("failed to save final batch: %w", err)
						}
						metrics.DelegationsStored.Add(float64(len(batchBuffer)))
						metrics.RecordDelegationProcessed("success")
						s.logger.Infow("Saved final batch", "count", len(batchBuffer))
					}
					return nil
				}

				domainDelegations := s.convertToDomainDelegations(delegations)
				batchBuffer = append(batchBuffer, domainDelegations...)
				processedCount += len(delegations)

				if len(batchBuffer) >= 1000 {
					if err := s.repo.SaveBatch(batchBuffer); err != nil {
						return fmt.Errorf("failed to save batch: %w", err)
					}
					metrics.DelegationsStored.Add(float64(len(batchBuffer)))
					metrics.RecordDelegationProcessed("success")
					s.logger.Infow("Historical indexing progress",
						"processed", processedCount,
						"lastTimestamp", delegations[len(delegations)-1].Timestamp,
					)
					batchBuffer = make([]domain.Delegation, 0, 1000)
				}

			case err := <-errorChan:
				if err != nil {
					return fmt.Errorf("error fetching historical data: %w", err)
				}
			case <-gctx.Done():
				return gctx.Err()
			}
		}
	})

	if err := g.Wait(); err != nil {
		return fmt.Errorf("historical indexing failed: %w", err)
	}

	metrics.HistoricalIndexingProgress.Set(100)
	s.logger.Infow("Historical indexing completed", "totalProcessed", processedCount)
	return nil
}

func (s *Service) convertToDomainDelegations(tzktDelegations []tzkt.DelegationResponse) []domain.Delegation {
	delegations := make([]domain.Delegation, 0, len(tzktDelegations))

	for _, d := range tzktDelegations {
		delegation := domain.Delegation{
			ID:        uuid.New().String(),
			Timestamp: d.Timestamp,
			Amount:    strconv.FormatInt(d.Amount, 10),
			Delegator: d.Sender.Address,
			Level:     strconv.FormatInt(d.Level, 10),
			BlockHash: d.Block,
			CreatedAt: time.Now(),
		}
		delegations = append(delegations, delegation)
	}

	return delegations
}

func (s *Service) GetStats() (map[string]interface{}, error) {
	delegations, err := s.repo.FindAll(nil)
	if err != nil {
		return nil, err
	}

	stats := make(map[string]interface{})
	stats["total_delegations"] = len(delegations)

	if len(delegations) > 0 {
		stats["latest_delegation"] = delegations[0].Timestamp
		stats["oldest_delegation"] = delegations[len(delegations)-1].Timestamp
	}

	uniqueDelegators := make(map[string]bool)
	totalAmount := int64(0)
	for _, d := range delegations {
		uniqueDelegators[d.Delegator] = true
		if amount, err := strconv.ParseInt(d.Amount, 10, 64); err == nil {
			totalAmount += amount
		}
	}

	stats["unique_delegators"] = len(uniqueDelegators)
	stats["total_amount"] = strconv.FormatInt(totalAmount, 10)

	return stats, nil
}
