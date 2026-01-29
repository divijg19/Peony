package storage

import (
	"database/sql"
	"fmt"
)

// SchemaVersion is the latest schema version supported by the migrator.
const SchemaVersion = 1

// Migrate ensures the SQLite schema exists and is upgraded to SchemaVersion.
func Migrate(db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("migrate: db is nil")
	}

	// err holds the error returned by each DDL/DML operation during migration.
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY);`)
	if err != nil {
		return fmt.Errorf("migrate: create schema_migrations: %w", err)
	}

	// current is the highest schema version recorded in schema_migrations.
	var current int
	err = db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations;`).Scan(&current)
	if err != nil {
		return fmt.Errorf("migrate: read current version: %w", err)
	}

	if current >= SchemaVersion {
		return nil
	}

	// transaction groups schema changes so the migration is applied atomically.
	transaction, err := db.Begin()
	if err != nil {
		return fmt.Errorf("migrate: begin transaction: %w", err)
	}
	defer func() {
		_ = transaction.Rollback()
	}()

	_, err = transaction.Exec(`
		CREATE TABLE IF NOT EXISTS thoughts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			content TEXT NOT NULL,
			current_state TEXT NOT NULL,
			tend_counter INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			last_tended_at TEXT NULL,
			eligibility_at TEXT NOT NULL,
			valence INTEGER NULL,
			energy INTEGER NULL
		);
	`)
	if err != nil {
		return fmt.Errorf("migrate: create thoughts table: %w", err)
	}

	_, err = transaction.Exec(`
		CREATE TABLE IF NOT EXISTS events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			thought_id INTEGER NOT NULL,
			kind TEXT NOT NULL,
			at TEXT NOT NULL,
			previous_state TEXT NULL,
			next_state TEXT NULL,
			note TEXT NULL,
			FOREIGN KEY(thought_id) REFERENCES thoughts(id)
		);
	`)
	if err != nil {
		return fmt.Errorf("migrate: create events table: %w", err)
	}

	_, err = transaction.Exec(`CREATE INDEX IF NOT EXISTS idx_thoughts_state_eligibility ON thoughts(current_state, eligibility_at);`)
	if err != nil {
		return fmt.Errorf("migrate: create idx_thoughts_state_eligibility: %w", err)
	}

	_, err = transaction.Exec(`CREATE INDEX IF NOT EXISTS idx_events_thought_id_at ON events(thought_id, at);`)
	if err != nil {
		return fmt.Errorf("migrate: create idx_events_thought_id_at: %w", err)
	}

	_, err = transaction.Exec(`INSERT INTO schema_migrations(version) VALUES (?);`, SchemaVersion)
	if err != nil {
		return fmt.Errorf("migrate: record schema version: %w", err)
	}

	err = transaction.Commit()
	if err != nil {
		return fmt.Errorf("migrate: commit transaction: %w", err)
	}

	return nil
}
