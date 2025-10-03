package trigger

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-viper/mapstructure/v2"
)

type Subscriber interface {
	Subscribe(ctx context.Context, logger *slog.Logger) (<-chan Result, error)
	Decode(input any) error
}

type Result struct {
	Data any
}

type mockSubscriber struct {
	Input string `json:"input"`
}

func (m *mockSubscriber) Decode(input any) error {
	if err := mapstructure.Decode(input, &m); err != nil {
		return err
	}
	return nil
}

func (m *mockSubscriber) Subscribe(ctx context.Context, logger *slog.Logger) (<-chan Result, error) {
	results := make(chan Result)
	ticker := time.Tick(5 * time.Second)

	go func() {
		defer close(results)
		for {
			select {
			case <-ticker:
				nanos := time.Now().UnixNano()
				data := fmt.Sprintf("%s: %d", m.Input, nanos)
				logger.Debug("Received a message", slog.String("message", data))

				select {
				case results <- Result{Data: data}:
				case <-ctx.Done():
					logger.Info("Stopping subscriber")
					return
				}

			case <-ctx.Done():
				logger.Info("Stopping subscriber")
				return
			}
		}
	}()

	return results, nil
}
