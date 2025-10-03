package trigger

import (
	"context"
	"log/slog"
	"testing"
	"testing/synctest"
	"time"
)

func Test_SyncService(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		store := newMockStore()
		defer store.Shutdown(ctx)

		service := New(WithStore(store), WithLogger(slog.New(slog.DiscardHandler)))
		results, err := service.Start(ctx)
		if err != nil {
			t.Fatal(err)
		}

		go func() {
			time.Sleep(10 * time.Second)

			newTrigger := Trigger{
				ID:   3,
				Name: "dynamically-added",
				Type: "mock",
				Data: map[string]string{
					"input": "Added",
				},
			}

			if err = store.Add(ctx, newTrigger); err != nil {
				t.Error(err)
				return
			}

			time.Sleep(10 * time.Second)

			updatedTrigger := Trigger{
				ID:   1,
				Name: "dynamically-updated",
				Type: "mock",
				Data: map[string]string{
					"input": "Updated",
				},
			}

			if err = store.Update(ctx, updatedTrigger); err != nil {
				t.Error(err)
				return
			}

			time.Sleep(10 * time.Second)

			updatedTrigger = Trigger{
				ID:   1,
				Name: "dynamically-updated-twice",
				Type: "mock",
				Data: map[string]string{
					"input": "Updated again",
				},
			}

			if err = store.Update(ctx, updatedTrigger); err != nil {
				t.Error(err)
			}
		}()

		for range results {
		}
	})
}
