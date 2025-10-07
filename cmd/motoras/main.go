package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/charmbracelet/log"

	"github.com/ionut-maxim/motoras/internal/application"
	"github.com/ionut-maxim/motoras/internal/config"

	// Register trigger subscribers
	_ "github.com/ionut-maxim/motoras/internal/trigger/subscribers/git"
	_ "github.com/ionut-maxim/motoras/internal/trigger/subscribers/mock"
)

func main() {
	ctx := context.Background()

	// TODO: Log handlers should be set by the application config
	prettyHandler := log.NewWithOptions(os.Stderr, log.Options{
		Level:           log.DebugLevel,
		ReportTimestamp: true,
		ReportCaller:    true,
	})

	logger := slog.New(prettyHandler)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("Unable to load config", "error", err)
		os.Exit(1)
	}

	app, err := application.New(cfg, logger)
	if err != nil {
		logger.Error("Failed to create application", "error", err)
		os.Exit(1)
	}

	if err = app.Start(ctx); err != nil {
		logger.Error("Failed to start application", "error", err)
		os.Exit(1)
	}
}
