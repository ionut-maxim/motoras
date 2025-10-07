package mock

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-viper/mapstructure/v2"

	"github.com/ionut-maxim/motoras/internal/trigger"
)

func init() {
	// Register the mock subscriber with the trigger registry
	trigger.RegisterSubscriber("mock", func() trigger.Subscriber {
		return &Subscriber{}
	})
}

type event struct {
	trigger *trigger.Trigger
	payload map[string]any
}

func (e event) Trigger() *trigger.Trigger { return e.trigger }
func (e event) Payload() map[string]any   { return e.payload }

type Subscriber struct {
	Input        string        `json:"input"`
	TickInterval time.Duration `json:"tick_interval"` // Tick interval, defaults to 5s if zero
}

func (s *Subscriber) Decode(input any) error {
	// First decode into a temporary struct to handle string duration
	var temp struct {
		Input        string `mapstructure:"input"`
		TickInterval any    `mapstructure:"tick_interval"`
	}

	if err := mapstructure.Decode(input, &temp); err != nil {
		return err
	}

	s.Input = temp.Input

	// Handle tick_interval as either string or int64
	if temp.TickInterval != nil {
		switch v := temp.TickInterval.(type) {
		case string:
			d, err := time.ParseDuration(v)
			if err != nil {
				return fmt.Errorf("invalid tick_interval: %w", err)
			}
			s.TickInterval = d
		case int64:
			s.TickInterval = time.Duration(v)
		case float64:
			s.TickInterval = time.Duration(v)
		}
	}

	return nil
}

func (s *Subscriber) Subscribe(ctx context.Context, logger *slog.Logger, t trigger.Trigger) (<-chan trigger.Event, error) {
	results := make(chan trigger.Event)

	interval := s.TickInterval
	if interval == 0 {
		interval = 5 * time.Second
	}
	ticker := time.Tick(interval)

	go func() {
		defer close(results)
		for {
			select {
			case <-ticker:
				nanos := time.Now().UnixNano()
				data := fmt.Sprintf("%s: %d", s.Input, nanos)
				logger.Debug("Received a message", slog.String("message", data))

				evt := event{
					trigger: &t,
					payload: map[string]any{
						"message":   data,
						"timestamp": nanos,
					},
				}

				select {
				case results <- evt:
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
