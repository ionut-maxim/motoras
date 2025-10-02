package trigger

import (
	"context"
)

type Trigger struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
	Data any    `json:"data"`
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
