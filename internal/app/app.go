package app

import (
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/divijg19/peony/internal/core"
	"github.com/divijg19/peony/internal/storage"
)

// Service coordinates Peony lifecycle operations for interactive surfaces.
type Service struct {
	store *storage.Store
}

// OpenDefault opens Peony's configured local store.
func OpenDefault() (*Service, func(), error) {
	dbPath, err := storage.ResolveDBPath()
	if err != nil {
		return nil, nil, fmt.Errorf("resolve db path: %w", err)
	}

	db, err := storage.Open(dbPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open db: %w", err)
	}

	st, err := storage.New(db)
	if err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("new store: %w", err)
	}

	closeFn := func() {
		_ = db.Close()
	}
	return New(st), closeFn, nil
}

// New creates a Service bound to an existing store.
func New(store *storage.Store) *Service {
	return &Service{store: store}
}

// NewForDB creates a Service from a database handle. It is mainly useful in tests.
func NewForDB(db *sql.DB) (*Service, error) {
	st, err := storage.New(db)
	if err != nil {
		return nil, err
	}
	return New(st), nil
}

// GardenThought is the TUI-friendly projection of a thought.
type GardenThought struct {
	Thought core.Thought
	Events  []core.Event
	Ready   bool
}

// ZoneKind identifies one of Bloom's high-level garden zones.
type ZoneKind string

const (
	ZoneReady   ZoneKind = "ready"
	ZoneResting ZoneKind = "resting"
	ZoneMemory  ZoneKind = "memory"
)

// GardenZone groups thoughts by the way they should feel in Bloom.
type GardenZone struct {
	Kind     ZoneKind
	Title    string
	Empty    string
	Thoughts []GardenThought
}

// GardenSnapshot is the current browse state for the terminal garden.
type GardenSnapshot struct {
	Thoughts   []GardenThought
	Zones      []GardenZone
	ReadyCount int
	Filter     core.State
	Query      string
}

// Capture stores a new thought and records its initial event.
func (s *Service) Capture(content string) (int64, error) {
	if s == nil || s.store == nil {
		return -1, fmt.Errorf("capture: service is nil")
	}

	content = strings.TrimSpace(content)
	if content == "" {
		return -1, fmt.Errorf("capture: content is empty")
	}

	id, err := s.store.CreateThought(content)
	if err != nil {
		return -1, err
	}

	next := core.StateCaptured
	if err := s.store.AppendEvent(id, "captured", nil, &next, nil); err != nil {
		return -1, err
	}
	return id, nil
}

// Snapshot returns a filtered, searchable garden projection.
func (s *Service) Snapshot(filter core.State, query string) (GardenSnapshot, error) {
	if s == nil || s.store == nil {
		return GardenSnapshot{}, fmt.Errorf("snapshot: service is nil")
	}

	all, err := s.loadAllThoughts()
	if err != nil {
		return GardenSnapshot{}, err
	}

	query = strings.ToLower(strings.TrimSpace(query))
	now := time.Now().UTC()
	thoughts := make([]GardenThought, 0, len(all))
	readyCount := 0

	for _, item := range all {
		item.Ready = core.EligibleToSurface(item.Thought, now)
		if item.Ready {
			readyCount++
		}

		if filter != "" && item.Thought.CurrentState != filter {
			continue
		}
		if query != "" && !matchesQuery(item, query) {
			continue
		}
		thoughts = append(thoughts, item)
	}

	sort.SliceStable(thoughts, func(i, j int) bool {
		left := thoughts[i]
		right := thoughts[j]
		if left.Ready != right.Ready {
			return left.Ready
		}
		if left.Ready {
			if !left.Thought.EligibilityAt.Equal(right.Thought.EligibilityAt) {
				return left.Thought.EligibilityAt.Before(right.Thought.EligibilityAt)
			}
			return left.Thought.ID < right.Thought.ID
		}
		if !left.Thought.UpdatedAt.Equal(right.Thought.UpdatedAt) {
			return left.Thought.UpdatedAt.After(right.Thought.UpdatedAt)
		}
		return left.Thought.ID < right.Thought.ID
	})

	zones := buildZones(thoughts)

	return GardenSnapshot{
		Thoughts:   thoughts,
		Zones:      zones,
		ReadyCount: readyCount,
		Filter:     filter,
		Query:      query,
	}, nil
}

func buildZones(thoughts []GardenThought) []GardenZone {
	zones := []GardenZone{
		{Kind: ZoneReady, Title: "Ready", Empty: "Nothing needs you right now."},
		{Kind: ZoneResting, Title: "Resting", Empty: "Your thoughts are settling."},
		{Kind: ZoneMemory, Title: "Memory", Empty: "Nothing has been placed in memory yet."},
	}

	for _, item := range thoughts {
		switch {
		case item.Ready || item.Thought.CurrentState == core.StateTended:
			zones[0].Thoughts = append(zones[0].Thoughts, item)
		case item.Thought.CurrentState == core.StateCaptured || item.Thought.CurrentState == core.StateResting:
			zones[1].Thoughts = append(zones[1].Thoughts, item)
		default:
			zones[2].Thoughts = append(zones[2].Thoughts, item)
		}
	}

	return zones
}

func (s *Service) loadAllThoughts() ([]GardenThought, error) {
	const pageSize = 100
	var items []GardenThought
	for page := 0; ; page++ {
		pageThoughts, err := s.store.ListThoughtsByPagination(pageSize, page*pageSize)
		if err != nil {
			return nil, fmt.Errorf("snapshot: list thoughts: %w", err)
		}
		for _, partial := range pageThoughts {
			thought, events, err := s.store.GetThought(partial.ID)
			if err != nil {
				return nil, fmt.Errorf("snapshot: get thought %d: %w", partial.ID, err)
			}
			items = append(items, GardenThought{Thought: thought, Events: events})
		}
		if len(pageThoughts) < pageSize {
			break
		}
	}
	return items, nil
}

func matchesQuery(item GardenThought, query string) bool {
	thought := item.Thought
	if strings.Contains(strings.ToLower(thought.Content), query) {
		return true
	}
	if strings.Contains(strings.ToLower(string(thought.CurrentState)), query) {
		return true
	}
	if strings.Contains(strconv.FormatInt(thought.ID, 10), query) {
		return true
	}
	for _, event := range item.Events {
		if strings.Contains(strings.ToLower(event.Kind), query) {
			return true
		}
		if event.Note != nil && strings.Contains(strings.ToLower(*event.Note), query) {
			return true
		}
	}
	return false
}

// Tend updates content, marks the thought as tended, and stores an optional note.
func (s *Service) Tend(id int64, content string, note *string) error {
	if s == nil || s.store == nil {
		return fmt.Errorf("tend: service is nil")
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return fmt.Errorf("tend: content is empty")
	}
	if note != nil {
		trimmed := strings.TrimSpace(*note)
		if trimmed == "" {
			note = nil
		} else {
			note = &trimmed
		}
	}
	if err := s.store.UpdateThoughtContent(id, content); err != nil {
		return err
	}
	return s.store.MarkThoughtTended(id, note)
}

// Rest returns a tended thought to resting.
func (s *Service) Rest(id int64, note *string) error {
	if s == nil || s.store == nil {
		return fmt.Errorf("rest: service is nil")
	}
	return s.store.TransitionPostTendResolutionStrict(id, core.StateResting, normalizeNote(note))
}

// Evolve marks a thought as evolved.
func (s *Service) Evolve(id int64) error {
	if s == nil || s.store == nil {
		return fmt.Errorf("evolve: service is nil")
	}
	return s.store.ToEvolve(id)
}

// Archive marks a thought as archived.
func (s *Service) Archive(id int64) error {
	if s == nil || s.store == nil {
		return fmt.Errorf("archive: service is nil")
	}
	return s.store.ToArchive(id)
}

// ReleasePermanent permanently deletes a thought and reindexes local IDs.
func (s *Service) ReleasePermanent(id int64) error {
	if s == nil || s.store == nil {
		return fmt.Errorf("release: service is nil")
	}
	if err := s.store.ReleaseThought(id); err != nil {
		return err
	}
	return s.store.ReindexThoughtIDs()
}

func normalizeNote(note *string) *string {
	if note == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*note)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
