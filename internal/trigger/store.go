package trigger

import (
	"context"
	"sync"

	"github.com/jackc/pgx/v5"
)

type Store interface {
	All(ctx context.Context) ([]Trigger, error)
	Subscribe(ctx context.Context) (<-chan Trigger, error)
	Add(ctx context.Context, trigger Trigger) error
	Update(ctx context.Context, trigger Trigger) error
	Shutdown(ctx context.Context)
}

func defaultStore() Store {
	return newMockStore()
}

type mockStore struct {
	triggers []Trigger
	mu       sync.RWMutex

	updateCh chan Trigger
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
	return &mockStore{triggers: triggers, updateCh: make(chan Trigger)}
}

func (m *mockStore) All(_ context.Context) ([]Trigger, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Trigger, len(m.triggers))
	copy(result, m.triggers)
	return result, nil
}

func (m *mockStore) Add(_ context.Context, trigger Trigger) error {
	m.mu.Lock()
	m.triggers = append(m.triggers, trigger)
	m.mu.Unlock()

	if m.updateCh != nil {
		m.updateCh <- trigger
	}
	return nil
}

func (m *mockStore) Subscribe(_ context.Context) (<-chan Trigger, error) {
	return m.updateCh, nil
}

func (m *mockStore) Shutdown(_ context.Context) {
	close(m.updateCh)
}

func (m *mockStore) Update(_ context.Context, trigger Trigger) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.triggers {
		if m.triggers[i].ID == trigger.ID {
			m.triggers[i].Name = trigger.Name
			m.triggers[i].Data = trigger.Data
		}
	}

	if m.updateCh != nil {
		m.updateCh <- trigger
	}
	return nil
}

type postgresStore struct {
	conn *pgx.Conn
}

func newPostgresStore(conn *pgx.Conn) *postgresStore {
	return &postgresStore{conn}
}

func (s *postgresStore) All(_ context.Context) ([]Trigger, error) {
	return nil, ErrNotImplemented
}
