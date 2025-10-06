package trigger

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/uuid"
)

func Test_SyncService(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		store := NewMockStore()
		defer store.Shutdown(ctx)

		service := New(WithStore(store))
		results, err := service.Start(ctx)
		if err != nil {
			t.Fatal(err)
		}

		go func() {
			time.Sleep(10 * time.Second)

			newTrigger := Trigger{
				ID:   uuid.New(),
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
				ID:   uuid.New(),
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
				ID:   uuid.New(),
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
