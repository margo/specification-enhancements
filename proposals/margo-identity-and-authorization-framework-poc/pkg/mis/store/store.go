// Package store owns the single SQLite database that backs every
// persistent MIS sub-system.
//
// Sub-stores are obtained via accessor methods on *Store; each returns
// a value implementing the package-local interface its caller already
// depends on. Callers outside cmd/mis and tests should not import this
// package directly.
package store

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

// Store holds the single *sql.DB backing every MIS sub-system's
// persistent state.
type Store struct {
	db *sql.DB
}

// Open opens (or creates) the SQLite file at path, applies the unified schema
// idempotently, and returns a ready *Store.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("store: open sqlite: %w", err)
	}
	if _, err := db.ExecContext(context.Background(), schemaSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: apply schema: %w", err)
	}
	return &Store{db: db}, nil
}

// Close closes the underlying *sql.DB. Safe to call once.
func (s *Store) Close() error { return s.db.Close() }

// DB returns the raw *sql.DB. Intended for sub-store constructors in
// this package and for integration-test helpers; production callers
// should use the typed accessors.
func (s *Store) DB() *sql.DB { return s.db }
