package git_test

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/google/uuid"

	"github.com/ionut-maxim/motoras/internal/trigger"
	gitsub "github.com/ionut-maxim/motoras/internal/trigger/subscribers/git"
)

func TestGitSubscriber_NewCommits(t *testing.T) {
	// Create a temporary git repository
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "test-repo")

	repo, err := git.PlainInit(repoPath, false)
	if err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	// Create initial commit
	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatalf("failed to get worktree: %v", err)
	}

	testFile := filepath.Join(repoPath, "test.txt")
	if err := os.WriteFile(testFile, []byte("initial content"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err = worktree.Add("test.txt")
	if err != nil {
		t.Fatalf("failed to add file: %v", err)
	}

	commit1, err := worktree.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Create subscriber
	sub := &gitsub.Subscriber{}
	err = sub.Decode(map[string]any{
		"repository":    repoPath,
		"branch":        "master",
		"poll_interval": "100ms",
	})
	if err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	trig := trigger.Trigger{
		ID:   uuid.New(),
		Name: "test-git-trigger",
		Type: "git",
	}

	eventCh, err := sub.Subscribe(ctx, logger, trig)
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	// Wait a bit for initial state to be set
	time.Sleep(200 * time.Millisecond)

	// Create a new commit
	if err := os.WriteFile(testFile, []byte("updated content"), 0644); err != nil {
		t.Fatalf("failed to update test file: %v", err)
	}

	_, err = worktree.Add("test.txt")
	if err != nil {
		t.Fatalf("failed to add updated file: %v", err)
	}

	commit2, err := worktree.Commit("Second commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("failed to create second commit: %v", err)
	}

	// Wait for event
	select {
	case evt := <-eventCh:
		if evt == nil {
			t.Fatal("received nil event")
		}

		payload := evt.Payload()
		if payload["type"] != "commit" {
			t.Errorf("expected type 'commit', got %v", payload["type"])
		}

		if payload["sha"] != commit2.String() {
			t.Errorf("expected SHA %s, got %v", commit2.String(), payload["sha"])
		}

		if payload["message"] != "Second commit" {
			t.Errorf("expected message 'Second commit', got %v", payload["message"])
		}

		t.Logf("Received commit event: %+v", payload)

	case <-ctx.Done():
		t.Fatal("timeout waiting for commit event")
	}

	// Verify no duplicate events
	select {
	case evt := <-eventCh:
		if evt != nil {
			t.Errorf("unexpected duplicate event: %+v", evt.Payload())
		}
	case <-time.After(300 * time.Millisecond):
		// Good - no duplicate
	}

	_ = commit1 // Use commit1 to avoid unused variable error
}

func TestGitSubscriber_NewTags(t *testing.T) {
	// Create a temporary git repository
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "test-repo")

	repo, err := git.PlainInit(repoPath, false)
	if err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	// Create initial commit
	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatalf("failed to get worktree: %v", err)
	}

	testFile := filepath.Join(repoPath, "test.txt")
	if err := os.WriteFile(testFile, []byte("initial content"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err = worktree.Add("test.txt")
	if err != nil {
		t.Fatalf("failed to add file: %v", err)
	}

	_, err = worktree.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Create subscriber with tag watching enabled
	sub := &gitsub.Subscriber{}
	err = sub.Decode(map[string]any{
		"repository":    repoPath,
		"branch":        "master",
		"watch_tags":    true,
		"poll_interval": "100ms",
	})
	if err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	trig := trigger.Trigger{
		ID:   uuid.New(),
		Name: "test-git-trigger",
		Type: "git",
	}

	eventCh, err := sub.Subscribe(ctx, logger, trig)
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	// Wait a bit for initial state to be set
	time.Sleep(200 * time.Millisecond)

	// Create a tag
	head, err := repo.Head()
	if err != nil {
		t.Fatalf("failed to get HEAD: %v", err)
	}

	_, err = repo.CreateTag("v1.0.0", head.Hash(), &git.CreateTagOptions{
		Message: "Release v1.0.0",
		Tagger: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("failed to create tag: %v", err)
	}

	// Wait for event
	select {
	case evt := <-eventCh:
		if evt == nil {
			t.Fatal("received nil event")
		}

		payload := evt.Payload()
		if payload["type"] != "tag" {
			t.Errorf("expected type 'tag', got %v", payload["type"])
		}

		if payload["tag"] != "v1.0.0" {
			t.Errorf("expected tag 'v1.0.0', got %v", payload["tag"])
		}

		// Message may have trailing newline from git
		message := payload["message"].(string)
		if message != "Release v1.0.0" && message != "Release v1.0.0\n" {
			t.Errorf("expected message 'Release v1.0.0', got %q", payload["message"])
		}

		t.Logf("Received tag event: %+v", payload)

	case <-ctx.Done():
		t.Fatal("timeout waiting for tag event")
	}

	// Verify no duplicate events
	select {
	case evt := <-eventCh:
		if evt != nil {
			t.Errorf("unexpected duplicate event: %+v", evt.Payload())
		}
	case <-time.After(300 * time.Millisecond):
		// Good - no duplicate
	}
}
