package trigger

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/google/uuid"
)

type Subscriber interface {
	Subscribe(ctx context.Context, logger *slog.Logger, workflowID uuid.UUID) (<-chan Event, error)
	Decode(input any) error
}

type Event interface {
	Workflow() uuid.UUID
}

type mockSubscriber struct {
	Input string `json:"input"`
}

type mockEvent struct {
	workflowID uuid.UUID
}

func (m mockEvent) Workflow() uuid.UUID { return m.workflowID }

func (m *mockSubscriber) Decode(input any) error {
	if err := mapstructure.Decode(input, &m); err != nil {
		return err
	}
	return nil
}

func (m *mockSubscriber) Subscribe(ctx context.Context, logger *slog.Logger, workflowID uuid.UUID) (<-chan Event, error) {
	results := make(chan Event)
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
				case results <- mockEvent{workflowID: workflowID}:
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
