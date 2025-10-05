package workflow

import (
	"context"
	"errors"
	"sync"

	"github.com/google/uuid"
)

type Store interface {
	Get(ctx context.Context, id uuid.UUID) (Workflow, error)
	Update(ctx context.Context, id uuid.UUID, w Workflow) error
	Delete(ctx context.Context, id uuid.UUID) error
	Create(ctx context.Context, w Workflow) (uuid.UUID, error)
	List(ctx context.Context) ([]Workflow, error)
}

type MockStore struct {
	workflows []Workflow
	mu        sync.RWMutex
}

func defaultStore() Store {
	return NewMockStore()
}

func NewMockStore() *MockStore {
	return &MockStore{workflows: make([]Workflow, 0), mu: sync.RWMutex{}}
}

func (s *MockStore) Get(ctx context.Context, id uuid.UUID) (Workflow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if id == uuid.Nil {
		return Workflow{}, errors.New("invalid workflow id")
	}

	for _, w := range s.workflows {
		if w.ID == id {
			return w, nil
		}
	}

	return Workflow{}, ErrWorkflowNotFound
}

func (s *MockStore) Update(ctx context.Context, id uuid.UUID, w Workflow) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var found bool
	for i := range s.workflows {
		if s.workflows[i].ID == id {
			found = true
			s.workflows[i].Steps = w.Steps
		}
	}

	if !found {
		return ErrWorkflowNotFound
	}

	return nil
}

func (s *MockStore) Delete(ctx context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var found bool
	for i := range s.workflows {
		if s.workflows[i].ID == id {
			found = true
			s.workflows = append(s.workflows[:i], s.workflows[i+1:]...)
		}
	}

	if !found {
		return ErrWorkflowNotFound
	}

	return nil
}

func (s *MockStore) Create(ctx context.Context, w Workflow) (uuid.UUID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if w.ID == uuid.Nil {
		id, err := uuid.NewV7()
		if err != nil {
			return uuid.Nil, ErrUnableToGenerateID
		}
		w.ID = id
	}

	for i := range s.workflows {
		if s.workflows[i].ID == w.ID {
			return uuid.Nil, ErrWorkflowAlreadyExists
		}
	}

	s.workflows = append(s.workflows, w)

	return w.ID, nil
}

func (s *MockStore) List(ctx context.Context) ([]Workflow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.workflows, nil
}
