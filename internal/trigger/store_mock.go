package trigger

import (
	"context"
	"sync"

	"github.com/google/uuid"
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

func (m *MockStore) Get(_ context.Context, id uuid.UUID) (Trigger, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, trigger := range m.triggers {
		if trigger.ID == id {
			return trigger, nil
		}
	}

	return Trigger{}, ErrTriggerNotFound
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

	found := false
	for i := range m.triggers {
		if m.triggers[i].ID == trigger.ID {
			m.triggers[i].Name = trigger.Name
			m.triggers[i].Data = trigger.Data
			found = true
			break
		}
	}

	if !found {
		return ErrTriggerNotFound
	}

	if m.updateCh != nil {
		select {
		case m.updateCh <- trigger:
		default:
		}
	}
	return nil
}

func (m *MockStore) Delete(_ context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, trigger := range m.triggers {
		if trigger.ID == id {
			m.triggers = append(m.triggers[:i], m.triggers[i+1:]...)
			return nil
		}
	}

	return ErrTriggerNotFound
}
