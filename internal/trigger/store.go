package trigger

import (
	"context"

	"github.com/google/uuid"
)

type Store interface {
	All(ctx context.Context) ([]Trigger, error)
	Subscribe(ctx context.Context) (<-chan Trigger, error)
	Add(ctx context.Context, trigger Trigger) error
	Update(ctx context.Context, trigger Trigger) error
	Shutdown(ctx context.Context)
}

type Trigger struct {
	InternalID int64     `json:"internal_id"` // Database primary key, used for distributed locking
	ID         uuid.UUID `json:"id"`          // Public/external identifier
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	WorkflowID uuid.UUID `json:"workflow_id"`
	Data       any       `json:"data"`
}

var triggerRegistry = map[string]func() Subscriber{}

// RegisterSubscriber registers a subscriber factory for a given type
func RegisterSubscriber(subscriberType string, factory func() Subscriber) {
	triggerRegistry[subscriberType] = factory
}

func (t *Trigger) mapSubscriber(_ context.Context) (Subscriber, error) {
	factory, ok := triggerRegistry[t.Type]
	if !ok {
		return nil, ErrUnknownSubscriberType
	}
	return factory(), nil
}

func defaultStore() Store {
	return NewMockStore()
}
