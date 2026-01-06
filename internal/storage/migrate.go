package storage

import (
	"database/sql"
	"fmt"
)

// SchemaVersion is the current schema version for Peony's local SQLite database.
const SchemaVersion = 1

// Migrate ensures the SQLite schema exists and is at the current SchemaVersion.
func Migrate(db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("migrate: db is nil")
	}

	// Check if the schema_migrations table exists
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY);`)
	if err != nil {
		return fmt.Errorf("migrate: create schema_migrations: %w", err)
	}

	// Check the current schema version
	var current int
	err = db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations;`).Scan(&current)
	if err != nil {
		return fmt.Errorf("migrate: read current version: %w", err)
	}

	// If the current version is greater than or equal to the schema version, no migration is needed
	if current >= SchemaVersion {
		return nil
	}

	// Start a transaction for migration.
	transaction, err := db.Begin()
	if err != nil {
		return fmt.Errorf("migrate: begin transaction: %w", err)
	}
	defer func() { _ = transaction.Rollback() }()

	// Create the thoughts table
	_, err = transaction.Exec(`
		CREATE TABLE IF NOT EXISTS thoughts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			content TEXT NOT NULL,
			state TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			last_tended_at TEXT NULL,
			rest_until TEXT NULL,
			valence INTEGER NULL,
    		energy INTEGER NULL
		);
	`)
	if err != nil {
		return fmt.Errorf("migrate: create thoughts table: %w", err)
	}

	// Create the events table
	_, err = transaction.Exec(`
		CREATE TABLE IF NOT EXISTS events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			thought_id INTEGER NOT NULL,
			kind TEXT NOT NULL,
			at TEXT NOT NULL,
			note TEXT NULL,
			FOREIGN KEY(thought_id) REFERENCES thoughts(id)
		);
	`)
	if err != nil {
		return fmt.Errorf("migrate: create events table: %w", err)
	}

	// Create index on thoughts rest_until column
	_, err = transaction.Exec(`CREATE INDEX IF NOT EXISTS idx_thoughts_rest_until ON thoughts(rest_until);`)
	if err != nil {
		return fmt.Errorf("migrate: create idx_thoughts_rest_until: %w", err)
	}

	// Create index on events thought_id and at columns
	_, err = transaction.Exec(`CREATE INDEX IF NOT EXISTS idx_events_thought_id_at ON events(thought_id, at);`)
	if err != nil {
		return fmt.Errorf("migrate: create idx_events_thought_id_at: %w", err)
	}

	// Record the current schema version
	_, err = transaction.Exec(`INSERT INTO schema_migrations(version) VALUES (?);`, SchemaVersion)
	if err != nil {
		return fmt.Errorf("migrate: record schema version: %w", err)
	}

	// Commit the transaction
	err = transaction.Commit()
	if err != nil {
		return fmt.Errorf("migrate: commit transaction: %w", err)
	}

	return nil
}
