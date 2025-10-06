package workflow

import (
	"context"
	"log/slog"
	"os"

	"github.com/dbos-inc/dbos-transact-golang/dbos"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	ctx    dbos.DBOSContext
	logger *slog.Logger
	store  Store
}

type Option func(*Service)

func WithLogger(logger *slog.Logger) Option {
	return func(s *Service) {
		s.logger = logger
	}
}

func WithStore(store Store) Option {
	return func(s *Service) {
		s.store = store
	}
}

func New(dbosPool *pgxpool.Pool, options ...Option) (*Service, error) {
	s := &Service{}

	for _, option := range options {
		option(s)
	}

	if s.store == nil {
		s.store = defaultStore()
	}

	serviceAttr := slog.String("service", "workflow")
	if s.logger == nil {
		s.logger = slog.New(slog.NewTextHandler(os.Stdout, nil)).With(serviceAttr)
	} else {
		s.logger = s.logger.With(serviceAttr)
	}

	dbosConfig := dbos.Config{SystemDBPool: dbosPool, Logger: s.logger, AppName: "workflows"}
	dbosContext, err := dbos.NewDBOSContext(context.Background(), dbosConfig)
	if err != nil {
		return nil, err
	}
	s.ctx = dbosContext

	return s, nil
}

func (s *Service) StartWorkflow(ctx context.Context, id uuid.UUID) error {
	wf, err := s.store.Get(ctx, id)
	if err != nil {
		return err
	}

	s.logger.Debug("Starting workflow", "id", id)
	_, err = run(s.ctx, wf)
	if err != nil {
		return err
	}

	return nil
}
