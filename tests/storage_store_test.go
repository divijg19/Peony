package tests

import (
	"path/filepath"
	"testing"

	"github.com/ri5hii/peony/internal/storage"
)

func TestStore_CreateThought_AndAppendEvent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "peony.db")

	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	store, err := storage.New(db)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	id, err := store.CreateThought("hello")
	if err != nil {
		t.Fatalf("create thought: %v", err)
	}
	if id <= 0 {
		t.Fatalf("create thought returned id=%d", id)
	}

	err = store.AppendEvent(id, "captured", nil)
	if err != nil {
		t.Fatalf("append event (nil note): %v", err)
	}

	note := "seed"
	err = store.AppendEvent(id, "tended", &note)
	if err != nil {
		t.Fatalf("append event (note): %v", err)
	}
}
