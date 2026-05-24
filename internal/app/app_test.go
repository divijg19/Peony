package app

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/divijg19/peony/internal/core"
	"github.com/divijg19/peony/internal/storage"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "peony.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	service, err := NewForDB(db)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	return service
}

func withSettleDuration(t *testing.T, duration time.Duration) {
	t.Helper()
	previous := core.SettleDuration
	core.SettleDuration = duration
	t.Cleanup(func() {
		core.SettleDuration = previous
	})
}

func TestCaptureSnapshotAndSearch(t *testing.T) {
	withSettleDuration(t, 0)
	service := newTestService(t)

	firstID, err := service.Capture("  learn softer terminal design  ")
	if err != nil {
		t.Fatalf("capture first: %v", err)
	}
	if firstID != 1 {
		t.Fatalf("first id = %d, want 1", firstID)
	}
	if _, err := service.Capture("plan a small release"); err != nil {
		t.Fatalf("capture second: %v", err)
	}

	snapshot, err := service.Snapshot("", "soft")
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if snapshot.ReadyCount != 2 {
		t.Fatalf("ready count = %d, want 2", snapshot.ReadyCount)
	}
	if len(snapshot.Thoughts) != 1 {
		t.Fatalf("filtered thoughts = %d, want 1", len(snapshot.Thoughts))
	}
	if snapshot.Thoughts[0].Thought.Content != "learn softer terminal design" {
		t.Fatalf("content was not trimmed: %q", snapshot.Thoughts[0].Thought.Content)
	}
	if !snapshot.Thoughts[0].Ready {
		t.Fatal("captured thought should be ready when settle duration is zero")
	}
}

func TestBuildZonesGroupsThoughtsByBloomMeaning(t *testing.T) {
	zones := buildZones([]GardenThought{
		{Thought: core.Thought{ID: 1, CurrentState: core.StateCaptured}, Ready: true},
		{Thought: core.Thought{ID: 2, CurrentState: core.StateResting}},
		{Thought: core.Thought{ID: 3, CurrentState: core.StateEvolved}},
		{Thought: core.Thought{ID: 4, CurrentState: core.StateTended}},
	})

	if len(zones) != 3 {
		t.Fatalf("zone count = %d, want 3", len(zones))
	}
	if zones[0].Kind != ZoneReady || len(zones[0].Thoughts) != 2 {
		t.Fatalf("ready zone = %s/%d, want ready/2", zones[0].Kind, len(zones[0].Thoughts))
	}
	if zones[1].Kind != ZoneResting || len(zones[1].Thoughts) != 1 {
		t.Fatalf("resting zone = %s/%d, want resting/1", zones[1].Kind, len(zones[1].Thoughts))
	}
	if zones[2].Kind != ZoneMemory || len(zones[2].Thoughts) != 1 {
		t.Fatalf("memory zone = %s/%d, want memory/1", zones[2].Kind, len(zones[2].Thoughts))
	}
}

func TestSnapshotBloomFocusedQueueFiltersAndCounts(t *testing.T) {
	service := newTestService(t)
	withSettleDuration(t, 0)
	readyID, err := service.Capture("ready item")
	if err != nil {
		t.Fatalf("capture ready: %v", err)
	}
	tendedID, err := service.Capture("tended item")
	if err != nil {
		t.Fatalf("capture tended: %v", err)
	}
	if err := service.Tend(tendedID, "tended item", nil); err != nil {
		t.Fatalf("tend: %v", err)
	}

	previous := core.SettleDuration
	core.SettleDuration = time.Hour
	restingID, err := service.Capture("resting item")
	core.SettleDuration = previous
	if err != nil {
		t.Fatalf("capture resting: %v", err)
	}

	memoryID, err := service.Capture("memory item")
	if err != nil {
		t.Fatalf("capture memory: %v", err)
	}
	if err := service.Archive(memoryID); err != nil {
		t.Fatalf("archive: %v", err)
	}

	snapshot, err := service.SnapshotBloom(BloomFilterReady, "")
	if err != nil {
		t.Fatalf("snapshot ready: %v", err)
	}
	if snapshot.Counts.Ready != 2 || snapshot.Counts.Resting != 1 || snapshot.Counts.Memory != 1 || snapshot.Counts.All != 4 {
		t.Fatalf("counts = %+v, want ready/resting/memory/all 2/1/1/4", snapshot.Counts)
	}
	if len(snapshot.Thoughts) != 2 || snapshot.Thoughts[0].Thought.ID != readyID || snapshot.Thoughts[1].Thought.ID != tendedID {
		t.Fatalf("ready queue = %+v, want ready then tended", snapshot.Thoughts)
	}

	snapshot, err = service.SnapshotBloom(BloomFilterResting, "")
	if err != nil {
		t.Fatalf("snapshot resting: %v", err)
	}
	if len(snapshot.Thoughts) != 1 || snapshot.Thoughts[0].Thought.ID != restingID {
		t.Fatalf("resting queue = %+v", snapshot.Thoughts)
	}

	snapshot, err = service.SnapshotBloom(BloomFilterMemory, "memory")
	if err != nil {
		t.Fatalf("snapshot memory search: %v", err)
	}
	if len(snapshot.Thoughts) != 1 || snapshot.Thoughts[0].Thought.ID != memoryID {
		t.Fatalf("memory search = %+v", snapshot.Thoughts)
	}
}

func TestTendRestEvolveArchiveAndRelease(t *testing.T) {
	withSettleDuration(t, 0)
	service := newTestService(t)

	firstID, err := service.Capture("first thought")
	if err != nil {
		t.Fatalf("capture first: %v", err)
	}
	secondID, err := service.Capture("second thought")
	if err != nil {
		t.Fatalf("capture second: %v", err)
	}
	thirdID, err := service.Capture("third thought")
	if err != nil {
		t.Fatalf("capture third: %v", err)
	}

	note := "made it clearer"
	if err := service.Tend(firstID, "first thought, revised", &note); err != nil {
		t.Fatalf("tend: %v", err)
	}
	snapshot, err := service.Snapshot(core.StateTended, "")
	if err != nil {
		t.Fatalf("snapshot tended: %v", err)
	}
	if len(snapshot.Thoughts) != 1 || snapshot.Thoughts[0].Thought.Content != "first thought, revised" {
		t.Fatalf("unexpected tended snapshot: %+v", snapshot.Thoughts)
	}

	if err := service.Rest(firstID, nil); err != nil {
		t.Fatalf("rest: %v", err)
	}
	snapshot, err = service.Snapshot(core.StateResting, "")
	if err != nil {
		t.Fatalf("snapshot resting: %v", err)
	}
	if len(snapshot.Thoughts) != 1 || snapshot.Thoughts[0].Thought.ID != firstID {
		t.Fatalf("rested thought missing: %+v", snapshot.Thoughts)
	}

	if err := service.Evolve(secondID); err != nil {
		t.Fatalf("evolve: %v", err)
	}
	if err := service.Archive(thirdID); err != nil {
		t.Fatalf("archive: %v", err)
	}
	if err := service.ReleasePermanent(secondID); err != nil {
		t.Fatalf("release permanent: %v", err)
	}

	snapshot, err = service.Snapshot("", "")
	if err != nil {
		t.Fatalf("snapshot all: %v", err)
	}
	if len(snapshot.Thoughts) != 2 {
		t.Fatalf("thought count after release = %d, want 2", len(snapshot.Thoughts))
	}
	ids := map[int64]bool{}
	for _, item := range snapshot.Thoughts {
		ids[item.Thought.ID] = true
	}
	if !ids[1] || !ids[2] {
		t.Fatalf("ids after reindex = %#v, want 1 and 2", ids)
	}
}
