package pagerduty

import (
	"testing"

	"github.com/ahlert/telvar/internal/catalog"
)

func TestMatchOnCallToEntities(t *testing.T) {
	oncalls := []OnCall{
		{UserName: "Alice", UserEmail: "alice@ex.com", ScheduleName: "Backend Schedule", EscalationPolicy: "Backend Policy"},
		{UserName: "Bob", UserEmail: "bob@ex.com", ScheduleName: "Frontend Schedule", EscalationPolicy: "Frontend Policy"},
	}

	entities := []catalog.Entity{
		{ID: "backend-abc", Name: "backend-schedule", Owner: "backend"},
		{ID: "frontend-abc", Name: "frontend", Owner: "frontend-schedule"},
		{ID: "unmatched-abc", Name: "unrelated", Owner: "nobody"},
	}

	matches := MatchOnCallToEntities(oncalls, entities)

	if len(matches) != 2 {
		t.Errorf("expected 2 matches, got %d", len(matches))
	}

	if oc, ok := matches["backend-abc"]; !ok || oc.UserName != "Alice" {
		t.Errorf("backend should match Alice, got %+v", matches["backend-abc"])
	}

	if oc, ok := matches["frontend-abc"]; !ok || oc.UserName != "Bob" {
		t.Errorf("frontend should match Bob via owner, got %+v", matches["frontend-abc"])
	}

	if _, ok := matches["unmatched-abc"]; ok {
		t.Error("unrelated entity should not match")
	}
}

func TestMatchNormalization(t *testing.T) {
	oncalls := []OnCall{
		{UserName: "Alice", ScheduleName: "My Team Schedule"},
	}
	entities := []catalog.Entity{
		{ID: "svc-abc", Name: "my-team-schedule"},
	}

	matches := MatchOnCallToEntities(oncalls, entities)
	if len(matches) != 1 {
		t.Error("normalization should match 'My Team Schedule' to 'my-team-schedule'")
	}
}

func TestMatchEmpty(t *testing.T) {
	matches := MatchOnCallToEntities(nil, nil)
	if len(matches) != 0 {
		t.Error("expected 0 matches for nil inputs")
	}
}
