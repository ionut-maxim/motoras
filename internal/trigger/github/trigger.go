package github

import (
	"context"
	"time"

	"github.com/ionut-maxim/motoras/internal/trigger"
)

type Trigger struct {
	interval time.Duration
}

func New(interval time.Duration) *Trigger {
	return &Trigger{interval: interval}
}

func (t *Trigger) Pull(ctx context.Context) (trigger.Result, error) {
	return trigger.Result{}, nil
}

func (t *Trigger) Name() string {
	return "GitHub Trigger"
}

func (t *Trigger) Interval() time.Duration {
	return t.interval
}
