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
	Subscribe(ctx context.Context, logger *slog.Logger, workflowID uuid.UUID) (<-chan Result, error)
	Decode(input any) error
}

// TODO: Transform Result into an interface
//   - It should have a method to return data
//   - Another method that maybe Executes a worfklow, or returns a function that executes something based on the trigger or maybe
//     it just returns the workflow ID
//   - Maybe another function that evaluates the data using the expression parser and returns a boolean?
type Result struct {
	Data       any
	WorkflowID uuid.UUID
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

func (m *mockSubscriber) Subscribe(ctx context.Context, logger *slog.Logger, workflowID uuid.UUID) (<-chan Result, error) {
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
				case results <- Result{Data: data, WorkflowID: workflowID}:
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
