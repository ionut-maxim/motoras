package trigger

import (
	"context"
	"os"
	"path/filepath"
	"sync"
)

type FileSystemStore struct {
	path     string
	mu       sync.RWMutex
	updateCh chan Trigger
}

func NewFileSystemStore(path string) (*FileSystemStore, error) {
	var err error
	if path == "" {
		path, err = os.Getwd()
		if err != nil {
			return nil, err
		}
		path = filepath.Join(path, ".data/trigger")
	}

	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, err
	}

	return &FileSystemStore{mu: sync.RWMutex{}, path: path}, nil
}

func (f *FileSystemStore) All(_ context.Context) ([]Trigger, error) {
	return nil, ErrNotImplemented
}

func (f *FileSystemStore) Add(_ context.Context, trigger Trigger) error {
	return ErrNotImplemented
}

func (f *FileSystemStore) Subscribe(_ context.Context) (<-chan Trigger, error) {
	return f.updateCh, nil
}

func (f *FileSystemStore) Shutdown(_ context.Context) {
	close(f.updateCh)
}

func (f *FileSystemStore) Update(_ context.Context, trigger Trigger) error {
	return ErrNotImplemented
}
