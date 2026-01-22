package tests

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/ri5hii/peony/internal/storage"
	_ "modernc.org/sqlite"
)

func TestMigrate_IdempotentAndRecordsVersion(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "peony.db")

	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=rwc&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("sql open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	err = storage.Migrate(db)
	if err != nil {
		t.Fatalf("migrate first: %v", err)
	}

	err = storage.Migrate(db)
	if err != nil {
		t.Fatalf("migrate second: %v", err)
	}

	var current int
	err = db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations;`).Scan(&current)
	if err != nil {
		t.Fatalf("read schema_migrations: %v", err)
	}
	if current != storage.SchemaVersion {
		t.Fatalf("current version=%d, want %d", current, storage.SchemaVersion)
	}
}
