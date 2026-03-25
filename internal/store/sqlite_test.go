package store

import (
	"testing"
	"time"

	"github.com/ahlert/telvar/internal/catalog"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func makeEntity(id, name string) *catalog.Entity {
	return &catalog.Entity{
		ID:           id,
		Name:         name,
		Kind:         catalog.KindComponent,
		Description:  "test entity",
		Owner:        "test-team",
		Language:     "go",
		Framework:    "go-service",
		RepoURL:      "https://github.com/test/" + name,
		Dependencies: []string{"go:github.com/spf13/cobra"},
		Tags:         map[string]string{"env": "prod"},
		Metadata:     map[string]string{"stars": "42"},
		DiscoveredAt: time.Now().UTC(),
	}
}

func TestUpsertAndGetEntity(t *testing.T) {
	s := newTestStore(t)
	e := makeEntity("svc-abc123", "my-service")

	if err := s.UpsertEntity(e); err != nil {
		t.Fatalf("UpsertEntity: %v", err)
	}

	got, err := s.GetEntity("svc-abc123")
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if got == nil {
		t.Fatal("GetEntity returned nil")
	}

	if got.Name != "my-service" {
		t.Errorf("Name: got %q, want my-service", got.Name)
	}
	if got.Language != "go" {
		t.Errorf("Language: got %q, want go", got.Language)
	}
	if got.Owner != "test-team" {
		t.Errorf("Owner: got %q, want test-team", got.Owner)
	}
	if len(got.Dependencies) != 1 || got.Dependencies[0] != "go:github.com/spf13/cobra" {
		t.Errorf("Dependencies: got %v", got.Dependencies)
	}
	if got.Tags["env"] != "prod" {
		t.Errorf("Tags: got %v", got.Tags)
	}
	if got.Metadata["stars"] != "42" {
		t.Errorf("Metadata: got %v", got.Metadata)
	}
}

func TestUpsertUpdatesExisting(t *testing.T) {
	s := newTestStore(t)
	e := makeEntity("svc-abc123", "my-service")

	if err := s.UpsertEntity(e); err != nil {
		t.Fatalf("first UpsertEntity: %v", err)
	}

	e.Language = "rust"
	e.Owner = "new-team"
	if err := s.UpsertEntity(e); err != nil {
		t.Fatalf("second UpsertEntity: %v", err)
	}

	got, err := s.GetEntity("svc-abc123")
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if got.Language != "rust" {
		t.Errorf("Language after upsert: got %q, want rust", got.Language)
	}
	if got.Owner != "new-team" {
		t.Errorf("Owner after upsert: got %q, want new-team", got.Owner)
	}
}

func TestGetEntityNotFound(t *testing.T) {
	s := newTestStore(t)

	got, err := s.GetEntity("nonexistent")
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for missing entity, got %+v", got)
	}
}

func TestListEntitiesAll(t *testing.T) {
	s := newTestStore(t)

	for i := range 5 {
		e := makeEntity("svc-"+string(rune('a'+i))+"00000", "service-"+string(rune('a'+i)))
		if err := s.UpsertEntity(e); err != nil {
			t.Fatalf("UpsertEntity %d: %v", i, err)
		}
	}

	entities, err := s.ListEntities("", 0)
	if err != nil {
		t.Fatalf("ListEntities: %v", err)
	}
	if len(entities) != 5 {
		t.Errorf("expected 5 entities, got %d", len(entities))
	}
}

func TestListEntitiesFilterByKind(t *testing.T) {
	s := newTestStore(t)

	comp := makeEntity("comp-abc123", "component")
	comp.Kind = catalog.KindComponent
	api := makeEntity("api-abc123", "api-service")
	api.Kind = catalog.KindAPI

	if err := s.UpsertEntity(comp); err != nil {
		t.Fatalf("UpsertEntity comp: %v", err)
	}
	if err := s.UpsertEntity(api); err != nil {
		t.Fatalf("UpsertEntity api: %v", err)
	}

	entities, err := s.ListEntities(string(catalog.KindAPI), 0)
	if err != nil {
		t.Fatalf("ListEntities: %v", err)
	}
	if len(entities) != 1 {
		t.Errorf("expected 1 API entity, got %d", len(entities))
	}
	if len(entities) > 0 && entities[0].Kind != catalog.KindAPI {
		t.Errorf("expected KindAPI, got %q", entities[0].Kind)
	}
}

func TestListEntitiesWithLimit(t *testing.T) {
	s := newTestStore(t)

	for i := range 10 {
		e := makeEntity("svc-"+string(rune('a'+i))+"00000", "service-"+string(rune('a'+i)))
		if err := s.UpsertEntity(e); err != nil {
			t.Fatalf("UpsertEntity %d: %v", i, err)
		}
	}

	entities, err := s.ListEntities("", 3)
	if err != nil {
		t.Fatalf("ListEntities: %v", err)
	}
	if len(entities) != 3 {
		t.Errorf("expected 3 entities, got %d", len(entities))
	}
}

func TestCountEntities(t *testing.T) {
	s := newTestStore(t)

	count, err := s.CountEntities()
	if err != nil {
		t.Fatalf("CountEntities: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 entities, got %d", count)
	}

	if err := s.UpsertEntity(makeEntity("svc-abc123", "service-a")); err != nil {
		t.Fatalf("UpsertEntity a: %v", err)
	}
	if err := s.UpsertEntity(makeEntity("svc-def456", "service-b")); err != nil {
		t.Fatalf("UpsertEntity b: %v", err)
	}

	count, err = s.CountEntities()
	if err != nil {
		t.Fatalf("CountEntities: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 entities, got %d", count)
	}
}

func TestEntityWithScore(t *testing.T) {
	s := newTestStore(t)
	e := makeEntity("svc-abc123", "scored-service")
	e.Score = &catalog.ScoreResult{
		Total:    5,
		MaxTotal: 7,
		Percent:  71,
		Rules: []catalog.RuleResult{
			{Name: "Has CI", Passed: true, Weight: 2},
			{Name: "Has README", Passed: true, Weight: 1},
			{Name: "No CVEs", Passed: false, Weight: 3, Detail: "2 critical CVEs"},
		},
	}

	if err := s.UpsertEntity(e); err != nil {
		t.Fatalf("UpsertEntity: %v", err)
	}

	got, err := s.GetEntity("svc-abc123")
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if got.Score == nil {
		t.Fatal("Score is nil")
	}
	if got.Score.Percent != 71 {
		t.Errorf("Score.Percent: got %d, want 71", got.Score.Percent)
	}
	if len(got.Score.Rules) != 3 {
		t.Errorf("Score.Rules: got %d rules, want 3", len(got.Score.Rules))
	}
	if got.Score.Rules[2].Detail != "2 critical CVEs" {
		t.Errorf("Rule detail: got %q", got.Score.Rules[2].Detail)
	}
}

func TestUpsertDoesNotMutateTimestamps(t *testing.T) {
	s := newTestStore(t)
	e := makeEntity("svc-abc123", "my-service")
	original := e.DiscoveredAt

	if err := s.UpsertEntity(e); err != nil {
		t.Fatalf("UpsertEntity: %v", err)
	}

	if !e.DiscoveredAt.Equal(original) {
		t.Error("UpsertEntity mutated DiscoveredAt on the caller's entity")
	}
}
