package trigger

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type Store interface {
	All(ctx context.Context) ([]Trigger, error)
}

func defaultStore() Store {
	return newMockStore()
}

type mockStore struct {
	triggers []Trigger
}

func newMockStore() *mockStore {
	triggers := []Trigger{
		{
			ID:   1,
			Name: "mock-trigger-1",
			Type: "mock",
			Data: map[string]string{
				"input": "Hello from mock 1",
			},
		},
		{
			ID:   2,
			Name: "mock-trigger-2",
			Type: "mock",
			Data: map[string]string{
				"input": "Hello from mock 2",
			},
		},
	}
	return &mockStore{triggers}
}

func (s *mockStore) All(ctx context.Context) ([]Trigger, error) {
	return s.triggers, nil
}

type postgresStore struct {
	conn *pgx.Conn
}

func newPostgresStore(conn *pgx.Conn) *postgresStore {
	return &postgresStore{conn}
}

func (s *postgresStore) All(ctx context.Context) ([]Trigger, error) {
	return nil, ErrNotImplemented
}
