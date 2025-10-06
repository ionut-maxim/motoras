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

	ownedTriggers map[int64]*triggerWorker
	mu            sync.RWMutex

	once sync.Once
	wg   sync.WaitGroup

	eventCh chan Event
}

type triggerWorker struct {
	cancel   context.CancelFunc
	updateCh chan Trigger
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

func New(options ...Option) *Service {
	service := &Service{
		eventCh:       make(chan Event),
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
		s.mu.RLock()
		_, exists := s.ownedTriggers[trigger.LockID]
		s.mu.RUnlock()
		if exists {
			continue
		}

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
		close(worker.updateCh)
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
				worker, exists := s.ownedTriggers[update.LockID]
				s.mu.RUnlock()

				if exists {
					select {
					case worker.updateCh <- update:
						s.logger.Debug("Sent update to trigger worker", "trigger.id", update.ID)
					case <-ctx.Done():
						return
					}
				} else {
					if err = s.startTriggerWorker(ctx, update); err != nil {
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
	logger := s.logger.WithGroup("trigger").With("id", trigger.ID, "name", trigger.Name)

	subscriber, err := trigger.mapSubscriber(ctx)
	if err != nil {
		return err
	}

	workerCtx, cancel := context.WithCancel(ctx)
	updateCh := make(chan Trigger, 1)

	s.mu.Lock()
	s.ownedTriggers[trigger.LockID] = &triggerWorker{
		cancel:   cancel,
		updateCh: updateCh,
	}
	s.mu.Unlock()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer func() {
			s.mu.Lock()
			delete(s.ownedTriggers, trigger.LockID)
			s.mu.Unlock()

			if err := s.locker.Release(ctx, trigger.LockID); err != nil {
				logger.Error("Failed to release lock", "err", err)
			}
		}()

		logger.Info("Trigger started")

		if err = subscriber.Decode(trigger.Data); err != nil {
			logger.Error("Failed to decode trigger", "err", err)
			return
		}

		var subCtx context.Context
		var subCancel context.CancelFunc
		var triggerCh <-chan Event

		subCtx, subCancel = context.WithCancel(workerCtx)
		triggerCh, err = subscriber.Subscribe(subCtx, logger, trigger.WorkflowID)
		if err != nil {
			logger.Error("Failed to subscribe", "err", err)
			subCancel()
			return
		}

		currentTrigger := trigger

		for {
			select {
			case result, ok := <-triggerCh:
				if !ok {
					subCancel()
					return
				}
				select {
				case s.eventCh <- result:
				case <-workerCtx.Done():
					subCancel()
					return
				}

			case updatedTrigger := <-updateCh:
				logger = s.logger.WithGroup("trigger").With("id", updatedTrigger.ID, "name", updatedTrigger.Name)
				logger.Info("Received trigger update", "old.data", currentTrigger.Data, "new.data", updatedTrigger.Data)
				currentTrigger = updatedTrigger

				subCancel()

				// Drain the old subscriber's channel to ensure all messages are processed
				// before starting a new subscriber.
				for range triggerCh {
				}

				newSubscriber, err := updatedTrigger.mapSubscriber(workerCtx)
				if err != nil {
					logger.Error("Failed to create new subscriber", "err", err)
					return
				}

				if err = newSubscriber.Decode(updatedTrigger.Data); err != nil {
					logger.Error("Failed to decode updated trigger", "err", err)
					return
				}

				subCtx, subCancel = context.WithCancel(workerCtx)
				triggerCh, err = newSubscriber.Subscribe(subCtx, logger, updatedTrigger.WorkflowID)
				if err != nil {
					logger.Error("Failed to subscribe with updated trigger", "err", err)
					subCancel()
					return
				}

				subscriber = newSubscriber
				logger.Info("Subscriber restarted with new configuration")

			case <-workerCtx.Done():
				subCancel()
				return
			}
		}
	}()

	return nil
}
