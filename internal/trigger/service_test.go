package trigger_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/ionut-maxim/motoras/internal/trigger"

	// Register mock subscriber
	_ "github.com/ionut-maxim/motoras/internal/trigger/subscribers/mock"
)

func Test_StoreImplementations(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(t *testing.T) (trigger.Store, func(context.Context, trigger.Trigger) error, func())
	}{
		{
			name: "MockStore",
			setupFunc: func(t *testing.T) (trigger.Store, func(context.Context, trigger.Trigger) error, func()) {
				store := trigger.NewMockStore()
				cleanup := func() {
					store.Shutdown(context.Background())
				}
				// For MockStore, use the same store for writes
				writeFunc := func(ctx context.Context, t trigger.Trigger) error {
					return store.Add(ctx, t)
				}
				return store, writeFunc, cleanup
			},
		},
		{
			name: "FileSystemStore",
			setupFunc: func(t *testing.T) (trigger.Store, func(context.Context, trigger.Trigger) error, func()) {
				tempDir := t.TempDir()
				storePath := filepath.Join(tempDir, "triggers")
				store, err := trigger.NewFileSystemStore(storePath)
				if err != nil {
					t.Fatalf("failed to create filesystem store: %s", err)
				}
				cleanup := func() {
					store.Shutdown(context.Background())
				}
				// For FileSystemStore, use the same store for writes
				writeFunc := func(ctx context.Context, t trigger.Trigger) error {
					return store.Add(ctx, t)
				}
				return store, writeFunc, cleanup
			},
		},
		{
			name: "PostgresStore",
			setupFunc: func(t *testing.T) (trigger.Store, func(context.Context, trigger.Trigger) error, func()) {
				ctx := context.Background()

				// Start Postgres container
				pgContainer, err := postgres.Run(ctx,
					"postgres:17-alpine",
					postgres.WithDatabase("testdb"),
					postgres.WithUsername("testuser"),
					postgres.WithPassword("testpass"),
					testcontainers.WithWaitStrategy(
						wait.ForLog("database system is ready to accept connections").
							WithOccurrence(2).
							WithStartupTimeout(60*time.Second)),
				)
				if err != nil {
					t.Fatalf("failed to start postgres container: %s", err)
				}

				// Get connection string
				connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
				if err != nil {
					t.Fatalf("failed to get connection string: %s", err)
				}

				// Connect to database for main store
				conn, err := pgx.Connect(ctx, connStr)
				if err != nil {
					t.Fatalf("failed to connect to database: %s", err)
				}

				// Create store (runs migrations)
				store, err := trigger.NewPostgresStore(ctx, conn)
				if err != nil {
					t.Fatalf("failed to create postgres store: %s", err)
				}

				// For PostgresStore, create a separate connection for writes since Subscribe() blocks the main connection
				// Use upsert logic to handle both Add and Update
				writeFunc := func(ctx context.Context, trig trigger.Trigger) error {
					writeConn, err := pgx.Connect(ctx, connStr)
					if err != nil {
						return err
					}
					defer writeConn.Close(ctx)

					// Try to add first, if it fails with duplicate key, try update
					writeStore, err := trigger.NewPostgresStore(ctx, writeConn)
					if err != nil {
						return err
					}
					defer writeStore.Shutdown(ctx)

					err = writeStore.Add(ctx, trig)
					if err != nil {
						// If duplicate key error, try update instead
						if strings.Contains(err.Error(), "duplicate key") {
							return writeStore.Update(ctx, trig)
						}
						return err
					}
					return nil
				}

				cleanup := func() {
					store.Shutdown(context.Background())
					conn.Close(context.Background())
					if err := testcontainers.TerminateContainer(pgContainer); err != nil {
						t.Logf("failed to terminate container: %s", err)
					}
				}

				return store, writeFunc, cleanup
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel() // Run store implementations in parallel

			t.Run("Service_integration", func(t *testing.T) {
				// Common test function for all stores
				runTest := func(t *testing.T) {
					store, writeFunc, cleanup := tt.setupFunc(t)
					defer cleanup()

					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()

					service := trigger.New(trigger.WithStore(store))
					results, err := service.Start(ctx)
					if err != nil {
						t.Fatal(err)
					}

					eventCount := 0
					done := make(chan struct{})

					go func() {
						// Wait for store to be ready (fsnotify, postgres LISTEN, etc)
						time.Sleep(100 * time.Millisecond)

						newTrigger := trigger.Trigger{
							ID:   uuid.New(),
							Name: "dynamically-added",
							Type: "mock",
							Data: map[string]any{
								"input":         "Added",
								"tick_interval": "1s", // 1 second tick for faster tests
							},
						}

						if err = writeFunc(ctx, newTrigger); err != nil {
							t.Error(err)
							return
						}

						time.Sleep(200 * time.Millisecond)

						updatedTrigger := trigger.Trigger{
							ID:   newTrigger.ID,
							Name: "dynamically-updated",
							Type: "mock",
							Data: map[string]any{
								"input":         "Updated",
								"tick_interval": "1s",
							},
						}

						if err = writeFunc(ctx, updatedTrigger); err != nil {
							t.Error(err)
							return
						}

						// Wait for at least one event (mock subscriber emits every 1s)
						time.Sleep(2500 * time.Millisecond)
						close(done)
					}()

					for {
						select {
						case event, ok := <-results:
							if !ok {
								return
							}
							t.Log(event)
							eventCount++
						case <-done:
							if eventCount == 0 {
								t.Error("expected at least one event")
							}
							cancel() // Trigger shutdown
						case <-ctx.Done():
							return
						}
					}
				}

				// MockStore works with synctest (virtual time), others need real time
				if tt.name == "MockStore" {
					synctest.Test(t, runTest)
				} else {
					// FileSystemStore and PostgresStore use real OS/network operations
					// that don't work with synctest's virtual time
					runTest(t)
				}
			})
		})
	}
}
