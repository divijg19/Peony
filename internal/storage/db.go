package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// DefaultDBPath returns the default filesystem location for Peony's SQLite database.
func DefaultDBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting user home directory: %w", err)
	}
	return filepath.Join(home, ".local", "share", "peony", "peony.db"), nil
}

// ResolveDBPath returns the database path from PEONY_DB_PATH if set, otherwise DefaultDBPath.
func ResolveDBPath() (string, error) {
	p := os.Getenv("PEONY_DB_PATH")
	if p != "" {
		return p, nil
	}
	return DefaultDBPath()
}

// Open opens (or creates) a SQLite database at dbPath and applies migrations.
func Open(dbPath string) (*sql.DB, error) {
	if dbPath == "" {
		return nil, fmt.Errorf("open: empty db path")
	}

	err := os.MkdirAll(filepath.Dir(dbPath), 0o755)
	if err != nil {
		return nil, fmt.Errorf("open: create db dir: %w", err)
	}

	dsn := "file:" + dbPath + "?mode=rwc&_pragma=foreign_keys(1)"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open: sql open: %w", err)
	}

	err = db.Ping()
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("open: ping: %w", err)
	}

	err = Migrate(db)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("open: migrate: %w", err)
	}

	return db, nil
}
