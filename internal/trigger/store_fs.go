package trigger

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
)

type FileSystemStore struct {
	path     string
	mu       sync.RWMutex
	updateCh chan Trigger
	wg       sync.WaitGroup
	watcher  *fsnotify.Watcher
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

	if err = os.MkdirAll(path, 0755); err != nil {
		return nil, err
	}

	store := &FileSystemStore{
		path:     path,
		updateCh: make(chan Trigger, 10),
	}

	return store, nil
}

func (f *FileSystemStore) All(ctx context.Context) ([]Trigger, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	entries, err := os.ReadDir(f.path)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	triggers := make([]Trigger, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		triggerPath := filepath.Join(f.path, entry.Name())
		data, err := os.ReadFile(triggerPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read trigger file %s: %w", entry.Name(), err)
		}

		var trigger Trigger
		if err := json.Unmarshal(data, &trigger); err != nil {
			return nil, fmt.Errorf("failed to unmarshal trigger %s: %w", entry.Name(), err)
		}

		triggers = append(triggers, trigger)
	}

	return triggers, nil
}

func (f *FileSystemStore) Add(ctx context.Context, trigger Trigger) error {
	return f.writeTrigger(trigger)
}

func (f *FileSystemStore) Update(ctx context.Context, trigger Trigger) error {
	return f.writeTrigger(trigger)
}

func (f *FileSystemStore) writeTrigger(trigger Trigger) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	data, err := json.MarshalIndent(trigger, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal trigger: %w", err)
	}

	filename := fmt.Sprintf("%s.json", trigger.ID)
	triggerPath := filepath.Join(f.path, filename)

	if err = os.WriteFile(triggerPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write trigger file: %w", err)
	}

	return nil
}

func (f *FileSystemStore) Subscribe(ctx context.Context) (<-chan Trigger, error) {
	var err error
	// Create filesystem watcher
	f.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	// Watch the triggers directory
	if err = f.watcher.Add(f.path); err != nil {
		f.watcher.Close()
		return nil, fmt.Errorf("failed to watch directory: %w", err)
	}

	// Start watching goroutine
	f.wg.Add(1)
	go f.watchChanges(ctx)

	return f.updateCh, nil
}

func (f *FileSystemStore) watchChanges(ctx context.Context) {
	defer f.wg.Done()

	for {
		select {
		case event, ok := <-f.watcher.Events:
			if !ok {
				return
			}

			// Only care about write and create events for .json files
			if filepath.Ext(event.Name) != ".json" {
				continue
			}
			if event.Op&fsnotify.Write != fsnotify.Write && event.Op&fsnotify.Create != fsnotify.Create {
				continue
			}

			f.handleFileChange(event.Name)

		case err, ok := <-f.watcher.Errors:
			if !ok {
				return
			}
			_ = err

		case <-ctx.Done():
			f.watcher.Close()
			return
		}
	}
}

func (f *FileSystemStore) handleFileChange(path string) {
	f.mu.RLock()
	data, err := os.ReadFile(path)
	f.mu.RUnlock()

	if err != nil {
		return
	}

	var trigger Trigger
	if err = json.Unmarshal(data, &trigger); err != nil {
		return
	}

	// Blocking send without holding lock - ensures updates are not dropped
	f.updateCh <- trigger
}

func (f *FileSystemStore) Shutdown(_ context.Context) {
	if f.watcher != nil {
		f.watcher.Close()
	}
	f.wg.Wait()
	close(f.updateCh)
}
