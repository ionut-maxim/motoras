package trigger

import (
	"context"
	"log/slog"
	"time"
)

type Result struct {
	Data any
}

type Poller interface {
	Interval() time.Duration
	Pull(ctx context.Context) (Result, error)
}

type Trigger interface {
	Poller
	Name() string
}

func Run(ctx context.Context, trigger Trigger, logger *slog.Logger) (<-chan Result, error) {
	results := make(chan Result)

	go pull(ctx, trigger, logger, results)

	return results, nil
}

func pull(ctx context.Context, trigger Trigger, logger *slog.Logger, results chan Result) {
	logger = logger.With("trigger", trigger.Name())
	ticker := time.Tick(trigger.Interval())

	for {
		select {
		case <-ctx.Done():
			close(results)
			return
		case <-ticker:
			result, err := trigger.Pull(ctx)
			if err != nil {
				logger.Error("error pulling data", "error", err)
			}
			results <- result
		}
	}
}
