package state

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"

	"github.com/rebelopsio/archy/internal/state/db"
)

//go:embed schema.sql
var schemaSQL string

// Store is the public façade over the SQLite-backed state.
type Store struct {
	mu     sync.Mutex
	db     *sql.DB
	q      *db.Queries
	closed bool
}

// Open opens or creates the state database at path. The parent directory
// is created if missing. On first open the schema is applied; subsequent
// opens are no-ops because every CREATE statement uses IF NOT EXISTS.
//
// Returns ErrOpen wrapped with the underlying cause on failure.
func Open(ctx context.Context, path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("%w: mkdir parent: %v", ErrOpen, err)
	}

	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("%w: open: %v", ErrOpen, err)
	}
	// SQLite's WAL mode allows concurrent readers but a single writer.
	// archy's load is small; one connection avoids busy-retry headaches.
	sqlDB.SetMaxOpenConns(1)

	pragmas := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA busy_timeout=5000;",
		"PRAGMA foreign_keys=on;",
	}
	for _, p := range pragmas {
		if _, err := sqlDB.ExecContext(ctx, p); err != nil {
			_ = sqlDB.Close()
			return nil, fmt.Errorf("%w: %s: %v", ErrOpen, p, err)
		}
	}

	if _, err := sqlDB.ExecContext(ctx, schemaSQL); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("%w: apply schema: %v", ErrOpen, err)
	}

	return &Store{db: sqlDB, q: db.New(sqlDB)}, nil
}

// Close releases all resources. Safe to call multiple times.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	return s.db.Close()
}

// queries returns the sqlc-generated query handle, or nil after Close.
// Callers should check for nil and return an error if the store is closed.
func (s *Store) queries() (*db.Queries, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, errors.New("state: store is closed")
	}
	return s.q, nil
}
