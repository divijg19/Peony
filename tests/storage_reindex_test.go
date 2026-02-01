package tests

import (
	"path/filepath"
	"testing"

	"github.com/ri5hii/peony/internal/storage"
)

// TestStore_ReindexThoughtIDs verifies renumbering thoughts.id updates events.thought_id.
func TestStore_ReindexThoughtIDs(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "peony.db")

	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	st, err := storage.New(db)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	id1, err := st.CreateThought("a")
	if err != nil {
		t.Fatalf("create thought a: %v", err)
	}
	id2, err := st.CreateThought("b")
	if err != nil {
		t.Fatalf("create thought b: %v", err)
	}
	id3, err := st.CreateThought("c")
	if err != nil {
		t.Fatalf("create thought c: %v", err)
	}

	if err := st.AppendEvent(id1, "evt", nil, nil, nil); err != nil {
		t.Fatalf("append event a: %v", err)
	}
	if err := st.AppendEvent(id2, "evt", nil, nil, nil); err != nil {
		t.Fatalf("append event b: %v", err)
	}
	if err := st.AppendEvent(id3, "evt", nil, nil, nil); err != nil {
		t.Fatalf("append event c: %v", err)
	}

	// Delete the middle thought to create a gap.
	if err := st.ReleaseThought(id2); err != nil {
		t.Fatalf("release thought: %v", err)
	}

	if err := st.ReindexThoughtIDs(); err != nil {
		t.Fatalf("reindex thought ids: %v", err)
	}

	// IDs should be dense again: old 1 stays 1, old 3 becomes 2.
	th1, ev1, err := st.GetThought(1)
	if err != nil {
		t.Fatalf("get thought 1: %v", err)
	}
	if th1.Content != "a" {
		t.Fatalf("thought 1 content = %q, want %q", th1.Content, "a")
	}
	if len(ev1) == 0 {
		t.Fatalf("thought 1 events missing after reindex")
	}

	th2, ev2, err := st.GetThought(2)
	if err != nil {
		t.Fatalf("get thought 2: %v", err)
	}
	if th2.Content != "c" {
		t.Fatalf("thought 2 content = %q, want %q", th2.Content, "c")
	}
	if len(ev2) == 0 {
		t.Fatalf("thought 2 events missing after reindex")
	}

	if _, _, err := st.GetThought(3); err == nil {
		t.Fatalf("expected thought 3 to be missing after reindex")
	}

	// New inserts should continue at max(id)+1.
	idNew, err := st.CreateThought("d")
	if err != nil {
		t.Fatalf("create thought d: %v", err)
	}
	if idNew != 3 {
		t.Fatalf("new thought id = %d, want 3", idNew)
	}
}
