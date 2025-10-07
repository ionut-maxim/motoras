package trigger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"
)

type Service struct {
	store  Store
	locker Locker
	logger *slog.Logger

	ownedTriggers map[int64]*triggerWorker
	mu            sync.RWMutex

	once sync.Once
	wg   sync.WaitGroup

	eventCh chan Event
}

type triggerWorker struct {
	cancel      context.CancelFunc
	skipCleanup bool
	cleanupMu   sync.Mutex
}

type Option func(*Service)

func WithStore(store Store) Option {
	return func(s *Service) {
		s.store = store
	}
}

func WithLocker(locker Locker) Option {
	return func(s *Service) {
		s.locker = locker
	}
}

func WithLogger(logger *slog.Logger) Option {
	return func(s *Service) {
		s.logger = logger
	}
}

func WithEventBufferSize(bufferSize int) Option {
	return func(s *Service) {
		s.eventCh = make(chan Event, bufferSize)
	}
}

func New(options ...Option) *Service {
	service := &Service{
		eventCh:       make(chan Event, 100),
		ownedTriggers: make(map[int64]*triggerWorker),
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

	serviceAttr := slog.String("service", "trigger")

	if service.logger == nil {
		handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
		logger := slog.New(handler).With(serviceAttr)
		service.logger = logger
	} else {
		service.logger = service.logger.With(serviceAttr)
	}

	return service
}

func (s *Service) Start(ctx context.Context) (<-chan Event, error) {
	triggers, err := s.store.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("service: %w", err)
	}

	for _, trigger := range triggers {
		if err = s.startTriggerWorker(ctx, trigger); err != nil {
			return nil, fmt.Errorf("service: %w", err)
		}
	}

	if err = s.startUpdateListener(ctx); err != nil {
		return nil, fmt.Errorf("service: failed to start update listener: %w", err)
	}

	go func() {
		<-ctx.Done()
		s.Shutdown()
	}()

	return s.eventCh, nil
}

func (s *Service) Shutdown() {
	s.mu.Lock()
	for _, worker := range s.ownedTriggers {
		worker.cancel()
	}
	s.mu.Unlock()

	s.wg.Wait()

	s.once.Do(func() {
		close(s.eventCh)
	})
}

func (s *Service) startUpdateListener(ctx context.Context) error {
	updateCh, err := s.store.Subscribe(ctx)
	if err != nil {
		return err
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		for {
			select {
			case update, ok := <-updateCh:
				if !ok {
					return
				}

				s.mu.RLock()
				worker, exists := s.ownedTriggers[update.InternalID]
				s.mu.RUnlock()

				if exists {
					// Stop the old worker but keep the lock
					s.logger.Debug("Restarting trigger worker with new config", "trigger.id", update.ID)
					worker.cleanupMu.Lock()
					worker.skipCleanup = true
					worker.cleanupMu.Unlock()
					worker.cancel()

					// Start new worker with updated trigger (will skip lock acquisition)
					if err := s.startTriggerWorkerWithLock(ctx, update); err != nil {
						s.logger.Error("Failed to restart worker", "trigger.id", update.ID, "err", err)
						// Failed to restart - release the lock
						if releaseErr := s.locker.Release(ctx, update.InternalID); releaseErr != nil {
							s.logger.Error("Failed to release lock after restart failure", "err", releaseErr)
						}
					}
				} else {
					// New trigger - start a worker for it
					if err := s.startTriggerWorker(ctx, update); err != nil {
						s.logger.Error("Failed to start worker for new trigger", "trigger.id", update.ID, "err", err)
					}
				}

			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (s *Service) startTriggerWorker(ctx context.Context, trigger Trigger) error {
	return s.startTriggerWorkerInternal(ctx, trigger, false)
}

func (s *Service) startTriggerWorkerWithLock(ctx context.Context, trigger Trigger) error {
	return s.startTriggerWorkerInternal(ctx, trigger, true)
}

func (s *Service) startTriggerWorkerInternal(ctx context.Context, trigger Trigger, alreadyLocked bool) error {
	logger := s.logger.WithGroup("trigger").With("id", trigger.ID, "name", trigger.Name)

	// Try to acquire distributed lock if we don't already own it
	if !alreadyLocked {
		acquired, err := s.locker.Acquire(ctx, trigger.InternalID)
		if err != nil {
			return fmt.Errorf("failed to acquire lock: %w", err)
		}
		if !acquired {
			logger.Debug("Trigger already locked by another instance")
			return nil
		}
	}

	subscriber, err := trigger.mapSubscriber(ctx)
	if err != nil {
		if releaseErr := s.locker.Release(ctx, trigger.InternalID); releaseErr != nil {
			logger.Error("Failed to release lock after subscriber creation failure", "err", releaseErr)
		}
		return err
	}

	if err = subscriber.Decode(trigger.Data); err != nil {
		if releaseErr := s.locker.Release(ctx, trigger.InternalID); releaseErr != nil {
			logger.Error("Failed to release lock after decode failure", "err", releaseErr)
		}
		return fmt.Errorf("failed to decode trigger: %w", err)
	}

	workerCtx, cancel := context.WithCancel(ctx)

	worker := &triggerWorker{
		cancel: cancel,
	}

	s.mu.Lock()
	s.ownedTriggers[trigger.InternalID] = worker
	s.mu.Unlock()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer func() {
			// Check if we should skip cleanup (lock release)
			worker.cleanupMu.Lock()
			skip := worker.skipCleanup
			worker.cleanupMu.Unlock()

			if skip {
				logger.Debug("Worker replaced, skipping cleanup")
				return
			}

			// Only delete from map if we're still the current worker
			s.mu.Lock()
			if s.ownedTriggers[trigger.InternalID] == worker {
				delete(s.ownedTriggers, trigger.InternalID)
			}
			s.mu.Unlock()

			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cleanupCancel()

			if err = s.locker.Release(cleanupCtx, trigger.InternalID); err != nil {
				logger.Error("Failed to release lock", "err", err)
			}
		}()

		logger.Info("Trigger started")

		triggerCh, err := subscriber.Subscribe(workerCtx, logger, trigger)
		if err != nil {
			logger.Error("Failed to subscribe", "err", err)
			return
		}

		for {
			select {
			case event, ok := <-triggerCh:
				if !ok {
					logger.Info("Trigger stopped")
					return
				}
				select {
				case s.eventCh <- event:
				case <-workerCtx.Done():
					return
				}

			case <-workerCtx.Done():
				logger.Info("Trigger cancelled")
				return
			}
		}
	}()

	return nil
}
