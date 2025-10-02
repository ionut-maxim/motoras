package trigger

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"time"
)

type Poller interface {
	Interval() time.Duration
	Pull(ctx context.Context) (Result, error)
	Name() string
}

func Run(ctx context.Context, trigger Poller, logger *slog.Logger) (<-chan Result, error) {
	results := make(chan Result)

	go pull(ctx, trigger, logger, results)

	return results, nil
}

func pull(ctx context.Context, trigger Poller, logger *slog.Logger, results chan Result) {
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

type mockPuller struct {
	interval    time.Duration
	shouldError bool
}

func newMockPuller(interval time.Duration, shouldError bool) *mockPuller {
	return &mockPuller{interval: interval, shouldError: shouldError}
}

func (f *mockPuller) Pull(_ context.Context) (Result, error) {
	if f.shouldError {
		chances := rand.IntN(2)
		if chances == 0 {
			return Result{}, errors.New("fake puller error")
		}

	}
	return Result{Data: fmt.Sprint(time.Now().String())}, nil
}

func (f *mockPuller) Interval() time.Duration {
	return time.Second
}

func (f *mockPuller) Name() string {
	return "fake_trigger"
}
