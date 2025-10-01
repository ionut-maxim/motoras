package trigger

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"
)

type mockPuller struct {
	interval    time.Duration
	shouldError bool
}

func newMockPuller(interval time.Duration, shouldError bool) *mockPuller {
	return &mockPuller{interval: interval, shouldError: shouldError}
}

func (f *mockPuller) Pull(_ context.Context) (Result, error) {
	if f.shouldError {
		chances := rand.New(rand.NewSource(time.Now().UnixNano())).Intn(2)
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
