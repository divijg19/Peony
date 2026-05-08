package storage

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/divijg19/peony/internal/core"
)

func openTestStore(t *testing.T) (*Store, *sql.DB) {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "peony.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	st, err := New(db)
	if err != nil {
		_ = db.Close()
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return st, db
}

func withStoreSettleDuration(t *testing.T, duration time.Duration) {
	t.Helper()
	previous := core.SettleDuration
	core.SettleDuration = duration
	t.Cleanup(func() {
		core.SettleDuration = previous
	})
}

func TestMigrateIsIdempotentAndCreatesExpectedTables(t *testing.T) {
	_, db := openTestStore(t)

	if err := Migrate(db); err != nil {
		t.Fatalf("second migrate: %v", err)
	}

	for _, table := range []string{"schema_migrations", "thoughts", "events", "app_state"} {
		var name string
		err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name)
		if err != nil {
			t.Fatalf("table %s missing: %v", table, err)
		}
	}

	var version int
	if err := db.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&version); err != nil {
		t.Fatalf("read schema version: %v", err)
	}
	if version != SchemaVersion {
		t.Fatalf("schema version = %d, want %d", version, SchemaVersion)
	}
}

func TestListTendThoughtsHonorsEligibilityAndTerminalStates(t *testing.T) {
	withStoreSettleDuration(t, time.Hour)
	st, _ := openTestStore(t)

	waitingID, err := st.CreateThought("not ready")
	if err != nil {
		t.Fatalf("create waiting: %v", err)
	}
	if _, err := st.CreateThought("ready"); err != nil {
		t.Fatalf("create ready: %v", err)
	}
	if _, err := st.CreateThought("terminal"); err != nil {
		t.Fatalf("create terminal: %v", err)
	}

	_, err = st.db.Exec(`UPDATE thoughts SET eligibility_at = ? WHERE content = ?`, time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano), "ready")
	if err != nil {
		t.Fatalf("make ready: %v", err)
	}
	_, err = st.db.Exec(`UPDATE thoughts SET current_state = ?, eligibility_at = ? WHERE content = ?`, string(core.StateEvolved), time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano), "terminal")
	if err != nil {
		t.Fatalf("make terminal: %v", err)
	}

	thoughts, err := st.ListTendThoughtsByPagination(10, 0)
	if err != nil {
		t.Fatalf("list tend: %v", err)
	}
	if len(thoughts) != 1 || thoughts[0].Content != "ready" {
		t.Fatalf("eligible thoughts = %+v, want only ready", thoughts)
	}
	if _, _, err := st.GetTendThought(waitingID); err == nil {
		t.Fatal("GetTendThought returned a waiting thought")
	}
}

func TestStrictTendResolutionAndTerminalGuards(t *testing.T) {
	withStoreSettleDuration(t, 0)
	st, _ := openTestStore(t)

	id, err := st.CreateThought("resolve me")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := st.TransitionPostTendResolutionStrict(id, core.StateResting, nil); err == nil {
		t.Fatal("post-tend transition succeeded before thought was tended")
	}
	if err := st.MarkThoughtTended(id, nil); err != nil {
		t.Fatalf("mark tended: %v", err)
	}
	if err := st.TransitionPostTendResolutionStrict(id, core.StateReleased, nil); err != nil {
		t.Fatalf("transition to released: %v", err)
	}
	if err := st.MarkThoughtTended(id, nil); err == nil || !strings.Contains(err.Error(), "terminal state") {
		t.Fatalf("mark terminal error = %v, want terminal state guard", err)
	}
	if err := st.ToEvolve(id); err == nil || !strings.Contains(err.Error(), "terminal state") {
		t.Fatalf("evolve terminal error = %v, want terminal state guard", err)
	}
	if err := st.ToArchive(id); err == nil || !strings.Contains(err.Error(), "terminal state") {
		t.Fatalf("archive terminal error = %v, want terminal state guard", err)
	}
}

func TestReleaseAndReindexPreservesRemainingEvents(t *testing.T) {
	withStoreSettleDuration(t, 0)
	st, _ := openTestStore(t)

	firstID, err := st.CreateThought("first")
	if err != nil {
		t.Fatalf("create first: %v", err)
	}
	secondID, err := st.CreateThought("second")
	if err != nil {
		t.Fatalf("create second: %v", err)
	}
	note := "kept after reindex"
	if err := st.MarkThoughtTended(secondID, &note); err != nil {
		t.Fatalf("mark second tended: %v", err)
	}

	if err := st.ReleaseThought(firstID); err != nil {
		t.Fatalf("release first: %v", err)
	}
	if err := st.ReindexThoughtIDs(); err != nil {
		t.Fatalf("reindex: %v", err)
	}

	thought, events, err := st.GetThought(1)
	if err != nil {
		t.Fatalf("get reindexed thought: %v", err)
	}
	if thought.Content != "second" {
		t.Fatalf("remaining content = %q, want second", thought.Content)
	}
	if len(events) != 1 {
		t.Fatalf("event count = %d, want 1", len(events))
	}
	if events[0].ThoughtID != 1 {
		t.Fatalf("event thought id = %d, want reindexed 1", events[0].ThoughtID)
	}
	if events[0].Note == nil || *events[0].Note != note {
		t.Fatalf("event note = %#v, want %q", events[0].Note, note)
	}
}

func TestDidCountTendChangePersistsOnlyChanges(t *testing.T) {
	st, _ := openTestStore(t)

	if !st.DidCountTendChange(2) {
		t.Fatal("first count should be treated as a change")
	}
	if st.DidCountTendChange(2) {
		t.Fatal("same count should not be treated as a change")
	}
	if !st.DidCountTendChange(3) {
		t.Fatal("new count should be treated as a change")
	}
}
