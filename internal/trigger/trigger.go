package trigger

import (
	"context"

	"github.com/google/uuid"
)

type Trigger struct {
	LockID     int64     `json:"lock_id"`
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	WorkflowID uuid.UUID `json:"workflow_id"`
	Data       any       `json:"data"`
}

var triggerRegistry = map[string]func() Subscriber{
	"mock": func() Subscriber { return &mockSubscriber{} },
}

func (t *Trigger) mapSubscriber(_ context.Context) (Subscriber, error) {
	factory, ok := triggerRegistry[t.Type]
	if !ok {
		return nil, ErrUnknownSubscriberType
	}
	return factory(), nil
}
