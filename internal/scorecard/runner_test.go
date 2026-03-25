package scorecard

import (
	"testing"

	"github.com/ahlert/telvar/internal/catalog"
	"github.com/ahlert/telvar/internal/config"
)

func TestRunnerScore(t *testing.T) {
	scorecards := []config.Scorecard{
		{
			Name: "production-readiness",
			Rules: []config.ScorecardRule{
				{Name: "Has README", Check: `has_file("README.md")`, Weight: 1},
				{Name: "Has CI", Check: `has_file(".github/workflows/*.yml")`, Weight: 2},
				{Name: "Has CODEOWNERS", Check: `has_file("CODEOWNERS")`, Weight: 1},
			},
		},
	}

	runner := NewRunner(scorecards)

	files := newFiles("README.md", ".github/workflows/ci.yml")
	entity := &catalog.Entity{Metadata: map[string]string{}}

	result := runner.Score(entity, files)
	if result == nil {
		t.Fatal("expected non-nil score result")
	}

	if result.Total != 3 {
		t.Errorf("Total: got %v, want 3 (README + CI)", result.Total)
	}
	if result.MaxTotal != 4 {
		t.Errorf("MaxTotal: got %v, want 4", result.MaxTotal)
	}
	if result.Percent != 75 {
		t.Errorf("Percent: got %d, want 75", result.Percent)
	}
	if len(result.Rules) != 3 {
		t.Errorf("Rules count: got %d, want 3", len(result.Rules))
	}

	if !result.Rules[0].Passed {
		t.Error("Has README should pass")
	}
	if !result.Rules[1].Passed {
		t.Error("Has CI should pass")
	}
	if result.Rules[2].Passed {
		t.Error("Has CODEOWNERS should fail")
	}
}

func TestRunnerNoScorecards(t *testing.T) {
	runner := NewRunner(nil)
	entity := &catalog.Entity{Metadata: map[string]string{}}

	result := runner.Score(entity, newFiles())
	if result != nil {
		t.Error("expected nil result when no scorecards configured")
	}
}

func TestRunnerEmptyRules(t *testing.T) {
	runner := NewRunner([]config.Scorecard{{Name: "empty", Rules: nil}})
	entity := &catalog.Entity{Metadata: map[string]string{}}

	result := runner.Score(entity, newFiles())
	if result != nil {
		t.Error("expected nil result when no rules defined")
	}
}

func TestRunnerPerfectScore(t *testing.T) {
	scorecards := []config.Scorecard{
		{
			Name: "test",
			Rules: []config.ScorecardRule{
				{Name: "R1", Check: `has_file("a")`, Weight: 1},
				{Name: "R2", Check: `has_file("b")`, Weight: 1},
			},
		},
	}

	runner := NewRunner(scorecards)
	files := newFiles("a", "b")
	entity := &catalog.Entity{Metadata: map[string]string{}}

	result := runner.Score(entity, files)
	if result.Percent != 100 {
		t.Errorf("Percent: got %d, want 100", result.Percent)
	}
}

func TestRunnerZeroScore(t *testing.T) {
	scorecards := []config.Scorecard{
		{
			Name: "test",
			Rules: []config.ScorecardRule{
				{Name: "R1", Check: `has_file("missing1")`, Weight: 1},
				{Name: "R2", Check: `has_file("missing2")`, Weight: 2},
			},
		},
	}

	runner := NewRunner(scorecards)
	files := newFiles()
	entity := &catalog.Entity{Metadata: map[string]string{}}

	result := runner.Score(entity, files)
	if result.Percent != 0 {
		t.Errorf("Percent: got %d, want 0", result.Percent)
	}
}
