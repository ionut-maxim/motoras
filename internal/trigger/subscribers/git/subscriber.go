package git

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-viper/mapstructure/v2"

	"github.com/ionut-maxim/motoras/internal/trigger"
)

func init() {
	// Register the git subscriber with the trigger registry
	trigger.RegisterSubscriber("git", func() trigger.Subscriber {
		return &Subscriber{}
	})
}

type event struct {
	trigger *trigger.Trigger
	payload map[string]any
}

func (e event) Trigger() *trigger.Trigger { return e.trigger }
func (e event) Payload() map[string]any   { return e.payload }

type Subscriber struct {
	Repository   string        `mapstructure:"repository"`    // Local path or remote URL
	Branch       string        `mapstructure:"branch"`        // Branch to watch (default: "main")
	WatchTags    bool          `mapstructure:"watch_tags"`    // Whether to watch for new tags
	PollInterval time.Duration `mapstructure:"poll_interval"` // Polling interval, defaults to 30s

	// Authentication for private repositories
	AuthMethod  string `mapstructure:"auth_method"`  // "ssh" or "token"
	SSHKeyPath  string `mapstructure:"ssh_key_path"` // Path to SSH private key
	SSHPassword string `mapstructure:"ssh_password"` // Password for encrypted SSH key
	Username    string `mapstructure:"username"`     // Username for token/basic auth
	Token       string `mapstructure:"token"`        // Personal access token or password

	// Internal state
	repo          *git.Repository
	lastCommitSHA string
	knownTags     map[string]bool
}

func (s *Subscriber) Decode(input any) error {
	// Decode using mapstructure
	var temp struct {
		Repository   string `mapstructure:"repository"`
		Branch       string `mapstructure:"branch"`
		WatchTags    bool   `mapstructure:"watch_tags"`
		PollInterval any    `mapstructure:"poll_interval"`
		AuthMethod   string `mapstructure:"auth_method"`
		SSHKeyPath   string `mapstructure:"ssh_key_path"`
		SSHPassword  string `mapstructure:"ssh_password"`
		Username     string `mapstructure:"username"`
		Token        string `mapstructure:"token"`
	}

	if err := mapstructure.Decode(input, &temp); err != nil {
		return err
	}

	s.Repository = temp.Repository
	s.Branch = temp.Branch
	s.WatchTags = temp.WatchTags
	s.AuthMethod = temp.AuthMethod
	s.SSHKeyPath = temp.SSHKeyPath
	s.SSHPassword = temp.SSHPassword
	s.Username = temp.Username
	s.Token = temp.Token

	// Handle poll_interval as either string or int64
	if temp.PollInterval != nil {
		switch v := temp.PollInterval.(type) {
		case string:
			d, err := time.ParseDuration(v)
			if err != nil {
				return fmt.Errorf("invalid poll_interval: %w", err)
			}
			s.PollInterval = d
		case int64:
			s.PollInterval = time.Duration(v)
		case float64:
			s.PollInterval = time.Duration(v)
		}
	}

	// Set defaults
	if s.Branch == "" {
		s.Branch = "main"
	}
	if s.PollInterval == 0 {
		s.PollInterval = 30 * time.Second
	}

	return nil
}

func (s *Subscriber) Subscribe(ctx context.Context, logger *slog.Logger, t trigger.Trigger) (<-chan trigger.Event, error) {
	results := make(chan trigger.Event, 10)

	// Open or clone the repository
	var err error
	s.repo, err = s.openRepository(ctx, logger)
	if err != nil {
		close(results)
		return nil, fmt.Errorf("failed to open repository: %w", err)
	}

	// Initialize state
	if err := s.initializeState(logger); err != nil {
		close(results)
		return nil, fmt.Errorf("failed to initialize state: %w", err)
	}

	ticker := time.NewTicker(s.PollInterval)

	go func() {
		defer close(results)
		defer ticker.Stop()

		logger.Info("Git subscriber started",
			"repository", s.Repository,
			"branch", s.Branch,
			"watch_tags", s.WatchTags,
			"poll_interval", s.PollInterval)

		for {
			select {
			case <-ticker.C:
				// Fetch latest changes
				if err := s.fetch(ctx, logger); err != nil {
					logger.Warn("Failed to fetch", "err", err)
					continue
				}

				// Check for new commits
				if evt := s.checkNewCommits(logger, &t); evt != nil {
					select {
					case results <- evt:
					case <-ctx.Done():
						return
					}
				}

				// Check for new tags if enabled
				if s.WatchTags {
					if tagEvents := s.checkNewTags(logger, &t); len(tagEvents) > 0 {
						for _, evt := range tagEvents {
							select {
							case results <- evt:
							case <-ctx.Done():
								return
							}
						}
					}
				}

			case <-ctx.Done():
				logger.Info("Git subscriber stopped")
				return
			}
		}
	}()

	return results, nil
}

func (s *Subscriber) getAuth(logger *slog.Logger) (transport.AuthMethod, error) {
	if s.AuthMethod == "" {
		return nil, nil
	}

	switch s.AuthMethod {
	case "ssh":
		// Default to ~/.ssh/id_rsa if no key path specified
		keyPath := s.SSHKeyPath
		if keyPath == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("failed to get home directory: %w", err)
			}
			keyPath = home + "/.ssh/id_rsa"
		}

		// Read the private key
		auth, err := ssh.NewPublicKeysFromFile("git", keyPath, s.SSHPassword)
		if err != nil {
			return nil, fmt.Errorf("failed to load SSH key from %s: %w", keyPath, err)
		}

		logger.Debug("Using SSH authentication", "key_path", keyPath)
		return auth, nil

	case "token":
		// Use token/password authentication
		auth := &http.BasicAuth{
			Username: s.Username,
			Password: s.Token,
		}

		logger.Debug("Using token authentication", "username", s.Username)
		return auth, nil

	default:
		return nil, fmt.Errorf("unknown auth method: %s", s.AuthMethod)
	}
}

func (s *Subscriber) openRepository(ctx context.Context, logger *slog.Logger) (*git.Repository, error) {
	// Check if repository is a local path
	if _, err := os.Stat(s.Repository); err == nil {
		// Local repository exists
		return git.PlainOpen(s.Repository)
	}

	// Get authentication if configured
	auth, err := s.getAuth(logger)
	if err != nil {
		return nil, fmt.Errorf("failed to get auth: %w", err)
	}

	// Try to clone the repository to a temporary directory
	tempDir, err := os.MkdirTemp("", "motoras-git-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	logger.Info("Cloning repository", "url", s.Repository, "dest", tempDir)

	repo, err := git.PlainCloneContext(ctx, tempDir, false, &git.CloneOptions{
		URL:      s.Repository,
		Auth:     auth,
		Progress: nil, // Silent clone
	})
	if err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("failed to clone repository: %w", err)
	}

	// Update repository path to temp dir
	s.Repository = tempDir

	return repo, nil
}

func (s *Subscriber) initializeState(logger *slog.Logger) error {
	// Get current HEAD commit for the branch
	ref, err := s.repo.Reference(plumbing.NewBranchReferenceName(s.Branch), true)
	if err != nil {
		return fmt.Errorf("failed to get branch reference: %w", err)
	}

	s.lastCommitSHA = ref.Hash().String()
	logger.Debug("Initialized with commit", "sha", s.lastCommitSHA)

	// Initialize known tags if watching tags
	if s.WatchTags {
		s.knownTags = make(map[string]bool)
		tags, err := s.repo.Tags()
		if err != nil {
			return fmt.Errorf("failed to list tags: %w", err)
		}

		err = tags.ForEach(func(ref *plumbing.Reference) error {
			s.knownTags[ref.Name().Short()] = true
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to iterate tags: %w", err)
		}

		logger.Debug("Initialized known tags", "count", len(s.knownTags))
	}

	return nil
}

func (s *Subscriber) fetch(ctx context.Context, logger *slog.Logger) error {
	// Check if remote exists
	_, err := s.repo.Remote("origin")
	if err != nil {
		// No remote - this is a local repository, skip fetch
		return nil
	}

	// Get authentication if configured
	auth, err := s.getAuth(logger)
	if err != nil {
		return fmt.Errorf("failed to get auth: %w", err)
	}

	err = s.repo.FetchContext(ctx, &git.FetchOptions{
		RemoteName: "origin",
		Auth:       auth,
		Force:      true,
	})

	// git.NoErrAlreadyUpToDate is not an error
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return err
	}

	return nil
}

func (s *Subscriber) checkNewCommits(logger *slog.Logger, t *trigger.Trigger) trigger.Event {
	// Get current HEAD for the branch
	ref, err := s.repo.Reference(plumbing.NewBranchReferenceName(s.Branch), true)
	if err != nil {
		logger.Warn("Failed to get branch reference", "err", err)
		return nil
	}

	currentSHA := ref.Hash().String()

	// Check if there's a new commit
	if currentSHA == s.lastCommitSHA {
		return nil
	}

	// Get commit details
	commit, err := s.repo.CommitObject(ref.Hash())
	if err != nil {
		logger.Warn("Failed to get commit object", "err", err)
		return nil
	}

	logger.Info("New commit detected",
		"sha", currentSHA,
		"author", commit.Author.Name,
		"message", commit.Message)

	// Update last commit SHA
	s.lastCommitSHA = currentSHA

	// Create event
	evt := event{
		trigger: t,
		payload: map[string]any{
			"type":            "commit",
			"sha":             currentSHA,
			"short_sha":       currentSHA[:7],
			"branch":          s.Branch,
			"author":          commit.Author.Name,
			"author_email":    commit.Author.Email,
			"committer":       commit.Committer.Name,
			"committer_email": commit.Committer.Email,
			"message":         commit.Message,
			"timestamp":       commit.Author.When.Unix(),
		},
	}

	return evt
}

func (s *Subscriber) checkNewTags(logger *slog.Logger, t *trigger.Trigger) []trigger.Event {
	tags, err := s.repo.Tags()
	if err != nil {
		logger.Warn("Failed to list tags", "err", err)
		return nil
	}

	var events []trigger.Event

	err = tags.ForEach(func(ref *plumbing.Reference) error {
		tagName := ref.Name().Short()

		// Check if this is a new tag
		if s.knownTags[tagName] {
			return nil
		}

		// Mark as known
		s.knownTags[tagName] = true

		// Get tag/commit object
		tagSHA := ref.Hash().String()

		var commitSHA, message, author, authorEmail string
		var timestamp int64

		// Try to get annotated tag
		tagObj, err := s.repo.TagObject(ref.Hash())
		if err == nil {
			// Annotated tag
			message = tagObj.Message
			author = tagObj.Tagger.Name
			authorEmail = tagObj.Tagger.Email
			timestamp = tagObj.Tagger.When.Unix()
			commitSHA = tagObj.Target.String()
		} else {
			// Lightweight tag - points directly to commit
			commit, err := s.repo.CommitObject(ref.Hash())
			if err != nil {
				logger.Warn("Failed to get commit for tag", "tag", tagName, "err", err)
				return nil
			}
			commitSHA = ref.Hash().String()
			message = commit.Message
			author = commit.Author.Name
			authorEmail = commit.Author.Email
			timestamp = commit.Author.When.Unix()
		}

		logger.Info("New tag detected",
			"name", tagName,
			"sha", tagSHA,
			"commit", commitSHA)

		evt := event{
			trigger: t,
			payload: map[string]any{
				"type":         "tag",
				"tag":          tagName,
				"tag_sha":      tagSHA,
				"commit_sha":   commitSHA,
				"message":      message,
				"author":       author,
				"author_email": authorEmail,
				"timestamp":    timestamp,
			},
		}

		events = append(events, evt)
		return nil
	})

	if err != nil {
		logger.Warn("Failed to iterate tags", "err", err)
	}

	return events
}
