package tests

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ri5hii/peony/internal/storage"
)

func TestOpen_CreatesAndMigrates(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "nested", "peony.db")

	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	var current int
	err = db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations;`).Scan(&current)
	if err != nil {
		t.Fatalf("read schema_migrations: %v", err)
	}
	if current != storage.SchemaVersion {
		t.Fatalf("current version=%d, want %d", current, storage.SchemaVersion)
	}
}

func TestDefaultDBPath_ReturnsNonEmpty(t *testing.T) {
	p, err := storage.DefaultDBPath()
	if err != nil {
		t.Fatalf("default db path: %v", err)
	}
	if p == "" {
		t.Fatalf("default db path is empty")
	}
	if !strings.HasSuffix(p, string(filepath.Separator)+"peony.db") {
		t.Fatalf("default db path=%q, want it to end with %q", p, string(filepath.Separator)+"peony.db")
	}
}
