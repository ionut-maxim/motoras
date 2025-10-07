package trigger

import (
	"context"
	"sync"
)

type MockStore struct {
	triggers []Trigger
	mu       sync.RWMutex

	updateCh chan Trigger
}

func NewMockStore() *MockStore {
	return &MockStore{triggers: []Trigger{}, updateCh: make(chan Trigger)}
}

func (m *MockStore) All(_ context.Context) ([]Trigger, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Trigger, len(m.triggers))
	copy(result, m.triggers)
	return result, nil
}

func (m *MockStore) Add(_ context.Context, trigger Trigger) error {
	m.mu.Lock()
	m.triggers = append(m.triggers, trigger)
	m.mu.Unlock()

	if m.updateCh != nil {
		select {
		case m.updateCh <- trigger:
		default:
		}
	}
	return nil
}

func (m *MockStore) Subscribe(_ context.Context) (<-chan Trigger, error) {
	return m.updateCh, nil
}

func (m *MockStore) Shutdown(_ context.Context) {
	close(m.updateCh)
}

func (m *MockStore) Update(_ context.Context, trigger Trigger) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.triggers {
		if m.triggers[i].ID == trigger.ID {
			m.triggers[i].Name = trigger.Name
			m.triggers[i].Data = trigger.Data
		}
	}

	if m.updateCh != nil {
		select {
		case m.updateCh <- trigger:
		default:
		}
	}
	return nil
}
