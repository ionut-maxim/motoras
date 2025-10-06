package application

import (
	"context"
	"log/slog"
	"net"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ionut-maxim/motoras/internal/config"
	"github.com/ionut-maxim/motoras/internal/server"
	"github.com/ionut-maxim/motoras/internal/trigger"
	"github.com/ionut-maxim/motoras/internal/workflow"
)

// Container holds all the dependencies required for running the application
type Container struct {
	logger *slog.Logger

	serverListener net.Listener

	store struct {
		trigger  trigger.Store
		workflow workflow.Store
	}

	service struct {
		trigger  *trigger.Service
		workflow *workflow.Service
	}
}

type Option func(*Container)

func WithServerListener(lis net.Listener) Option {
	return func(c *Container) {
		c.serverListener = lis
	}
}

func New(config config.Application, logger *slog.Logger, options ...Option) (*Container, error) {
	appCtx := context.Background()

	dbPool, err := pgxpool.New(appCtx, config.Postgres.URL)
	if err != nil {
		return nil, err
	}

	workflowStore := workflow.NewMockStore()
	workflowService, err := workflow.New(dbPool, workflow.WithLogger(logger), workflow.WithStore(workflowStore))
	if err != nil {
		return nil, err
	}

	triggerStore := trigger.NewMockStore()
	triggerService := trigger.New(trigger.WithLogger(logger), trigger.WithStore(triggerStore))

	container := &Container{
		logger: logger,
		store: struct {
			trigger  trigger.Store
			workflow workflow.Store
		}{trigger: triggerStore, workflow: workflowStore},
		service: struct {
			trigger  *trigger.Service
			workflow *workflow.Service
		}{trigger: triggerService, workflow: workflowService},
	}

	for _, option := range options {
		option(container)
	}

	return container, nil
}

func (app *Container) Start(ctx context.Context) error {
	events, err := app.service.trigger.Start(ctx)
	if err != nil {
		return err
	}

	// Listen for trigger events in a goroutine and start workflows per each event
	// TODO: Parse trigger expressions to figure out if a workflow should start.
	go func() {
		for event := range events {
			go func() {
				if err = app.service.workflow.StartWorkflow(ctx, event.Workflow()); err != nil {
					app.logger.Error("Failed to start workflow", "err", err)
				}
			}()
		}
	}()

	var options []server.Option
	if app.service.trigger != nil {
		options = append(options, server.WithListener(app.serverListener))
	}

	srv := server.New(
		app.logger.With("service", "server"),
		app.store.workflow,
		app.store.trigger,
		options...,
	)

	return srv.Start()
}
