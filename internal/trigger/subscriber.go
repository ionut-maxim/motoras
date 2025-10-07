package trigger

import (
	"context"
	"log/slog"
)

type Subscriber interface {
	Subscribe(ctx context.Context, logger *slog.Logger, trigger Trigger) (<-chan Event, error)
	Decode(input any) error
}

type Event interface {
	// Trigger returns the trigger that emitted this event.
	// Each event holds a pointer to a snapshot of the trigger configuration
	// at the time the event was emitted.
	Trigger() *Trigger
	// Payload returns event-specific data from the subscriber
	Payload() map[string]any
}
