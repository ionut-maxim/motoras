package trigger

import (
	"context"
	"testing"
	"testing/synctest"
	"time"
)

func Test_SyncService(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		service := New()
		results, err := service.Start(ctx)
		if err != nil {
			t.Fatal(err)
		}

		for range results {

		}
	})
}
