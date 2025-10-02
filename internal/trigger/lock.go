package trigger

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Locker represents a distributed locker
type Locker interface {
	Acquire(ctx context.Context, id int64) (bool, error)
	Release(ctx context.Context, id int64) error
}

func defaultLocker() Locker {
	return &mockLocker{}
}

type mockLocker struct {
}

func (m *mockLocker) Acquire(_ context.Context, _ int64) (bool, error) {
	return true, nil
}

func (m *mockLocker) Release(_ context.Context, _ int64) error {
	return nil
}

type postgresAdvisoryLock struct {
	conn *pgx.Conn
}

func newPostgresAdvisoryLock(conn *pgx.Conn) *postgresAdvisoryLock {
	return &postgresAdvisoryLock{conn: conn}
}

func (l *postgresAdvisoryLock) Acquire(ctx context.Context, id int64) (bool, error) {
	var locked bool
	query := "SELECT pg_try_advisory_lock($1)"
	err := l.conn.QueryRow(ctx, query, id).Scan(&locked)
	return locked, err
}

func (l *postgresAdvisoryLock) Release(ctx context.Context, id int64) error {
	_, err := l.conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", id)
	return err
}
