package sqlitedb

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"skeleton/pkg/wfm/adapter/persistence/sqlitedb/db"

	"modernc.org/sqlite"
)

type DataStore struct {
	*db.Queries
	database *sql.DB
}

//go:embed schema.sql
var dbSchema string

const configureConnectionSQL = `
	PRAGMA foreign_keys = ON; -- enable foreign key support
`

func New(ctx context.Context, dbPath string) (*DataStore, error) {
	// register a hook to configure database connections (e.g. enable foreign key support)
	sqlite.RegisterConnectionHook(func(conn sqlite.ExecQuerierContext, _ string) error {
		_, err := conn.ExecContext(context.Background(), configureConnectionSQL, nil)
		return err
	})

	var err error
	var database *sql.DB
	if database, err = sql.Open("sqlite", dbPath); err != nil {
		return nil, fmt.Errorf("failed to open database at %v: %w", dbPath, err)
	}
	if err = database.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect to database at %v: %w", dbPath, err)
	}

	return &DataStore{
		database: database,
		Queries:  db.New(database),
	}, nil
}

func (ds *DataStore) Migrate(ctx context.Context) error {
	// Run database migrations
	if _, err := ds.database.ExecContext(ctx, dbSchema); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}
	return nil
}

func (ds *DataStore) BeginTransaction(ctx context.Context) (*sql.Tx, error) {
	return ds.database.BeginTx(ctx, nil)
}

func (ds *DataStore) Close() error {
	return ds.database.Close()
}
