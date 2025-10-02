package trigger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
)

type Service struct {
	store  Store
	locker Locker
	logger *slog.Logger

	ownedTriggers map[int64]context.CancelFunc
	mu            sync.RWMutex

	once sync.Once

	resultCh chan Result
}

type Option func(*Service)

// WithStore sets the underlying storage for workflow triggers
func WithStore(store Store) Option {
	return func(s *Service) {
		s.store = store
	}
}

// WithLocker sets a distributed locker for the triggers
func WithLocker(locker Locker) Option {
	return func(s *Service) {
		s.locker = locker
	}
}

// WithLogger sets a logger if not a default one is used
func WithLogger(logger *slog.Logger) Option {
	return func(s *Service) {
		s.logger = logger
	}
}

func New(options ...Option) *Service {
	service := &Service{
		resultCh:      make(chan Result),
		ownedTriggers: make(map[int64]context.CancelFunc),
	}
	for _, option := range options {
		option(service)
	}

	if service.store == nil {
		service.store = defaultStore()
	}

	if service.locker == nil {
		service.locker = defaultLocker()
	}

	if service.logger == nil {
		handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
		logger := slog.New(handler).With(slog.String("service", "trigger"))
		service.logger = logger
	}

	return service
}

func (s *Service) Start(ctx context.Context) (<-chan Result, error) {
	triggers, err := s.store.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("service: %w", err)
	}

	for _, trigger := range triggers {
		s.mu.RLock()
		_, exists := s.ownedTriggers[trigger.ID]
		s.mu.RUnlock()
		if exists {
			continue
		}

		if err = s.startTriggerWorker(ctx, trigger); err != nil {
			return nil, fmt.Errorf("service: %w", err)
		}

	}

	go func() {
		<-ctx.Done()
		s.Shutdown()
	}()

	return s.resultCh, nil
}

func (s *Service) Shutdown() {
	s.mu.Lock()

	for _, cancel := range s.ownedTriggers {
		cancel()
	}
	s.mu.Unlock()

	s.once.Do(func() {
		close(s.resultCh)
	})
}

func (s *Service) startTriggerWorker(ctx context.Context, trigger Trigger) error {
	logger := s.logger.WithGroup("trigger").With("id", trigger.ID, "name", trigger.Name)

	subscriber, err := trigger.mapSubscriber(ctx)
	if err != nil {
		return err
	}

	workerCtx, cancel := context.WithCancel(ctx)

	s.mu.Lock()
	s.ownedTriggers[trigger.ID] = cancel
	s.mu.Unlock()

	go func() {
		defer func() {
			// Cleanup on exit
			s.mu.Lock()
			delete(s.ownedTriggers, trigger.ID)
			s.mu.Unlock()

			if err := s.locker.Release(ctx, trigger.ID); err != nil {
				logger.Error("Failed to release lock", "err", err)
			}
		}()

		logger.Info("Trigger started")

		if err = subscriber.Decode(trigger.Data); err != nil {
			logger.Error("Failed to decode trigger", "err", err)
			return
		}

		triggerCh, err := subscriber.Subscribe(workerCtx, logger)
		if err != nil {
			logger.Error("Failed to subscribe", "err", err)
			return
		}

		for result := range triggerCh {
			select {
			case s.resultCh <- result:
			case <-workerCtx.Done():
				return
			}
		}
	}()

	return nil
}
