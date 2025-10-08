package trigger

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

//go:embed migrations/postgres/*.sql
var postgresMigrationsFs embed.FS

const (
	triggerMigrationModule = "trigger"

	createMigrationsTableSQL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    module TEXT NOT NULL,
    version TEXT NOT NULL,
    applied_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (module, version)
);
`
)

type PostgresStore struct {
	conn     *pgx.Conn
	updateCh chan Trigger
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

type migration struct {
	version string
	content string
}

func NewPostgresStore(ctx context.Context, conn *pgx.Conn) (*PostgresStore, error) {
	// Run migrations for trigger module
	if err := runPostgresMigrations(ctx, conn, triggerMigrationModule); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	store := &PostgresStore{
		conn:     conn,
		updateCh: make(chan Trigger, 10),
		stopCh:   make(chan struct{}),
	}

	return store, nil
}

func (s *PostgresStore) All(ctx context.Context) ([]Trigger, error) {
	query := `
		SELECT internal_id, id, name, type, workflow_id, data
		FROM triggers
		ORDER BY internal_id
	`

	rows, err := s.conn.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query triggers: %w", err)
	}
	defer rows.Close()

	var triggers []Trigger
	for rows.Next() {
		var t Trigger
		var dataJSON []byte

		if err := rows.Scan(&t.InternalID, &t.ID, &t.Name, &t.Type, &t.WorkflowID, &dataJSON); err != nil {
			return nil, fmt.Errorf("failed to scan trigger: %w", err)
		}

		// Unmarshal JSONB data
		if err := json.Unmarshal(dataJSON, &t.Data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal trigger data: %w", err)
		}

		triggers = append(triggers, t)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating triggers: %w", err)
	}

	return triggers, nil
}

func (s *PostgresStore) Get(ctx context.Context, id uuid.UUID) (Trigger, error) {
	query := `
		SELECT internal_id, id, name, type, workflow_id, data
		FROM triggers
		WHERE id = $1
	`

	var t Trigger
	var dataJSON []byte

	err := s.conn.QueryRow(ctx, query, id).Scan(&t.InternalID, &t.ID, &t.Name, &t.Type, &t.WorkflowID, &dataJSON)
	if err != nil {
		if err == pgx.ErrNoRows {
			return Trigger{}, ErrTriggerNotFound
		}
		return Trigger{}, fmt.Errorf("failed to query trigger: %w", err)
	}

	// Unmarshal JSONB data
	if err := json.Unmarshal(dataJSON, &t.Data); err != nil {
		return Trigger{}, fmt.Errorf("failed to unmarshal trigger data: %w", err)
	}

	return t, nil
}

func (s *PostgresStore) Add(ctx context.Context, trigger Trigger) error {
	// If ID is not set, generate one
	if trigger.ID == uuid.Nil {
		trigger.ID = uuid.New()
	}

	dataJSON, err := json.Marshal(trigger.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal trigger data: %w", err)
	}

	query := `
		INSERT INTO triggers (id, name, type, workflow_id, data)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING internal_id
	`

	err = s.conn.QueryRow(ctx, query, trigger.ID, trigger.Name, trigger.Type, trigger.WorkflowID, dataJSON).Scan(&trigger.InternalID)
	if err != nil {
		return fmt.Errorf("failed to insert trigger: %w", err)
	}

	return nil
}

func (s *PostgresStore) Update(ctx context.Context, trigger Trigger) error {
	dataJSON, err := json.Marshal(trigger.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal trigger data: %w", err)
	}

	query := `
		UPDATE triggers
		SET name = $2, type = $3, workflow_id = $4, data = $5, updated_at = NOW()
		WHERE id = $1
	`

	result, err := s.conn.Exec(ctx, query, trigger.ID, trigger.Name, trigger.Type, trigger.WorkflowID, dataJSON)
	if err != nil {
		return fmt.Errorf("failed to update trigger: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrTriggerNotFound
	}

	return nil
}

func (s *PostgresStore) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM triggers WHERE id = $1`

	result, err := s.conn.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete trigger: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrTriggerNotFound
	}

	return nil
}

func (s *PostgresStore) Subscribe(ctx context.Context) (<-chan Trigger, error) {
	// Start listening for notifications
	if _, err := s.conn.Exec(ctx, "LISTEN trigger_updates"); err != nil {
		return nil, fmt.Errorf("failed to listen to trigger_updates: %w", err)
	}

	s.wg.Add(1)
	go s.listenForUpdates(ctx)

	return s.updateCh, nil
}

func (s *PostgresStore) listenForUpdates(ctx context.Context) {
	defer s.wg.Done()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ctx.Done():
			return
		default:
			// Wait for notification with timeout
			notification, err := s.conn.WaitForNotification(ctx)
			if err != nil {
				// Context cancelled or connection closed
				if ctx.Err() != nil {
					return
				}
				// Log error but continue
				continue
			}

			// Parse the notification payload (JSON)
			var trigger Trigger
			if err := json.Unmarshal([]byte(notification.Payload), &trigger); err != nil {
				// Invalid payload, skip
				continue
			}

			// Send to update channel (non-blocking)
			select {
			case s.updateCh <- trigger:
			default:
				// Channel full, skip this update
			}
		}
	}
}

func (s *PostgresStore) Shutdown(ctx context.Context) {
	// Unlisten from the channel
	_, _ = s.conn.Exec(ctx, "UNLISTEN trigger_updates")

	close(s.stopCh)
	s.wg.Wait()
	close(s.updateCh)
}

// runPostgresMigrations runs all pending migrations for the given module
func runPostgresMigrations(ctx context.Context, conn *pgx.Conn, module string) error {
	// Create migrations tracking table
	if _, err := conn.Exec(ctx, createMigrationsTableSQL); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get all migration files
	migrations, err := loadPostgresMigrations()
	if err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	// Get applied migrations for this module
	applied, err := getAppliedMigrations(ctx, conn, module)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Apply pending migrations in order
	for _, m := range migrations {
		if applied[m.version] {
			continue
		}

		if err := applyMigration(ctx, conn, module, m); err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", m.version, err)
		}
	}

	return nil
}

func loadPostgresMigrations() ([]migration, error) {
	entries, err := postgresMigrationsFs.ReadDir("migrations/postgres")
	if err != nil {
		return nil, err
	}

	var migrations []migration
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		content, err := postgresMigrationsFs.ReadFile(filepath.Join("migrations/postgres", entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("failed to read migration %s: %w", entry.Name(), err)
		}

		// Extract version from filename (e.g., "001_initial_schema.sql" -> "001")
		version := strings.SplitN(entry.Name(), "_", 2)[0]

		migrations = append(migrations, migration{
			version: version,
			content: string(content),
		})
	}

	// Sort by version
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].version < migrations[j].version
	})

	return migrations, nil
}

func getAppliedMigrations(ctx context.Context, conn *pgx.Conn, module string) (map[string]bool, error) {
	rows, err := conn.Query(ctx, "SELECT version FROM schema_migrations WHERE module = $1", module)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}

	return applied, rows.Err()
}

func applyMigration(ctx context.Context, conn *pgx.Conn, module string, m migration) error {
	// Begin transaction
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Execute migration SQL
	if _, err := tx.Exec(ctx, m.content); err != nil {
		return fmt.Errorf("failed to execute migration SQL: %w", err)
	}

	// Record migration as applied
	query := "INSERT INTO schema_migrations (module, version) VALUES ($1, $2)"
	if _, err := tx.Exec(ctx, query, module, m.version); err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	return nil
}
