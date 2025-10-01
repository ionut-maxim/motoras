package trigger

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"testing/synctest"
	"time"
)

func Test_Puller(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(5*time.Second))
		defer cancel()

		puller := newMockPuller(time.Second, true)

		results, err := Run(ctx, puller, slog.New(slog.NewTextHandler(os.Stdout, nil)))
		if err != nil {
			t.Fatalf("subscribe failed: %v", err)
		}

		for result := range results {
			if result.Data != nil {
				t.Log(result.Data)
			}
		}
	})
}
