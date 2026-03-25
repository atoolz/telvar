package scorecard

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/ahlert/telvar/internal/catalog"
	"github.com/ahlert/telvar/internal/config"
)

type mockFiles struct {
	files map[string]bool
}

func (m *mockFiles) HasFile(p string) bool            { return m.files[p] }
func (m *mockFiles) HasFileGlob(pattern string) bool {
	for p := range m.files {
		if matched, _ := filepath.Match(pattern, p); matched {
			return true
		}
	}
	return false
}
func (m *mockFiles) ReadFile(p string) ([]byte, error) {
	if m.files[p] {
		return []byte{}, nil
	}
	return nil, fmt.Errorf("not found: %s", p)
}

func newFiles(paths ...string) *mockFiles {
	m := &mockFiles{files: make(map[string]bool)}
	for _, p := range paths {
		m.files[p] = true
	}
	return m
}

func TestHasFileExact(t *testing.T) {
	files := newFiles("README.md", "go.mod")
	entity := &catalog.Entity{Metadata: map[string]string{}}

	rule := config.ScorecardRule{Name: "Has README", Check: `has_file("README.md")`, Weight: 1}
	result := EvaluateRule(rule, files, entity)

	if !result.Passed {
		t.Error("expected has_file(README.md) to pass")
	}
}

func TestHasFileMissing(t *testing.T) {
	files := newFiles("go.mod")
	entity := &catalog.Entity{Metadata: map[string]string{}}

	rule := config.ScorecardRule{Name: "Has README", Check: `has_file("README.md")`, Weight: 1}
	result := EvaluateRule(rule, files, entity)

	if result.Passed {
		t.Error("expected has_file(README.md) to fail when missing")
	}
}

func TestHasFileGlob(t *testing.T) {
	files := newFiles("go.mod", ".github/workflows/ci.yml")
	entity := &catalog.Entity{Metadata: map[string]string{}}

	rule := config.ScorecardRule{Name: "Has CI", Check: `has_file(".github/workflows/*.yml")`, Weight: 2}
	result := EvaluateRule(rule, files, entity)

	if !result.Passed {
		t.Error("expected glob match to pass")
	}
	if result.Weight != 2 {
		t.Errorf("weight: got %d, want 2", result.Weight)
	}
}

func TestCveCountZero(t *testing.T) {
	files := newFiles()
	entity := &catalog.Entity{Metadata: map[string]string{"vuln.critical": "0"}}

	rule := config.ScorecardRule{Name: "No CVEs", Check: `cve_count("critical") == 0`, Weight: 3}
	result := EvaluateRule(rule, files, entity)

	if !result.Passed {
		t.Error("expected cve_count(critical) == 0 to pass when count is 0")
	}
}

func TestCveCountNonZero(t *testing.T) {
	files := newFiles()
	entity := &catalog.Entity{Metadata: map[string]string{"vuln.critical": "5"}}

	rule := config.ScorecardRule{Name: "No CVEs", Check: `cve_count("critical") == 0`, Weight: 3}
	result := EvaluateRule(rule, files, entity)

	if result.Passed {
		t.Error("expected cve_count(critical) == 0 to fail when count is 5")
	}
}

func TestCveCountMissing(t *testing.T) {
	files := newFiles()
	entity := &catalog.Entity{Metadata: map[string]string{}}

	rule := config.ScorecardRule{Name: "No CVEs", Check: `cve_count("critical") == 0`, Weight: 3}
	result := EvaluateRule(rule, files, entity)

	if !result.Passed {
		t.Error("expected cve_count to be 0 when metadata key is missing")
	}
}

func TestOwnerSet(t *testing.T) {
	files := newFiles()

	t.Run("set", func(t *testing.T) {
		entity := &catalog.Entity{Owner: "platform-team", Metadata: map[string]string{}}
		rule := config.ScorecardRule{Name: "Has Owner", Check: "owner_set()", Weight: 1}
		result := EvaluateRule(rule, files, entity)
		if !result.Passed {
			t.Error("expected owner_set() to pass when owner is set")
		}
	})

	t.Run("empty", func(t *testing.T) {
		entity := &catalog.Entity{Owner: "", Metadata: map[string]string{}}
		rule := config.ScorecardRule{Name: "Has Owner", Check: "owner_set()", Weight: 1}
		result := EvaluateRule(rule, files, entity)
		if result.Passed {
			t.Error("expected owner_set() to fail when owner is empty")
		}
	})
}

func TestLanguageIs(t *testing.T) {
	files := newFiles()
	entity := &catalog.Entity{Language: "go", Metadata: map[string]string{}}

	t.Run("match", func(t *testing.T) {
		rule := config.ScorecardRule{Name: "Is Go", Check: `language_is("go")`, Weight: 1}
		result := EvaluateRule(rule, files, entity)
		if !result.Passed {
			t.Error("expected language_is(go) to pass")
		}
	})

	t.Run("no match", func(t *testing.T) {
		rule := config.ScorecardRule{Name: "Is Rust", Check: `language_is("rust")`, Weight: 1}
		result := EvaluateRule(rule, files, entity)
		if result.Passed {
			t.Error("expected language_is(rust) to fail for go entity")
		}
	})
}

func TestHasTopic(t *testing.T) {
	files := newFiles()
	entity := &catalog.Entity{
		Tags:     map[string]string{"topics": "kubernetes,monitoring,go"},
		Metadata: map[string]string{},
	}

	t.Run("match", func(t *testing.T) {
		rule := config.ScorecardRule{Name: "Has K8s", Check: `has_topic("kubernetes")`, Weight: 1}
		result := EvaluateRule(rule, files, entity)
		if !result.Passed {
			t.Error("expected has_topic(kubernetes) to pass")
		}
	})

	t.Run("no match", func(t *testing.T) {
		rule := config.ScorecardRule{Name: "Has Rust", Check: `has_topic("rust")`, Weight: 1}
		result := EvaluateRule(rule, files, entity)
		if result.Passed {
			t.Error("expected has_topic(rust) to fail")
		}
	})
}

func TestLogicalAnd(t *testing.T) {
	files := newFiles("README.md", ".github/workflows/ci.yml")
	entity := &catalog.Entity{Metadata: map[string]string{}}

	rule := config.ScorecardRule{
		Name:   "README and CI",
		Check:  `has_file("README.md") && has_file(".github/workflows/*.yml")`,
		Weight: 2,
	}
	result := EvaluateRule(rule, files, entity)
	if !result.Passed {
		t.Error("expected AND to pass when both are true")
	}
}

func TestLogicalAndFails(t *testing.T) {
	files := newFiles("README.md")
	entity := &catalog.Entity{Metadata: map[string]string{}}

	rule := config.ScorecardRule{
		Name:   "README and CI",
		Check:  `has_file("README.md") && has_file("CODEOWNERS")`,
		Weight: 2,
	}
	result := EvaluateRule(rule, files, entity)
	if result.Passed {
		t.Error("expected AND to fail when second is false")
	}
}

func TestLogicalOr(t *testing.T) {
	files := newFiles("README.md")
	entity := &catalog.Entity{Metadata: map[string]string{}}

	rule := config.ScorecardRule{
		Name:  "README or CODEOWNERS",
		Check: `has_file("README.md") || has_file("CODEOWNERS")`,
	}
	result := EvaluateRule(rule, files, entity)
	if !result.Passed {
		t.Error("expected OR to pass when first is true")
	}
}

func TestLogicalOrRightBranch(t *testing.T) {
	files := newFiles("CODEOWNERS")
	entity := &catalog.Entity{Metadata: map[string]string{}}

	rule := config.ScorecardRule{
		Name:  "README or CODEOWNERS",
		Check: `has_file("README.md") || has_file("CODEOWNERS")`,
	}
	result := EvaluateRule(rule, files, entity)
	if !result.Passed {
		t.Error("expected OR to pass when right branch is true")
	}
}

func TestLessThanComparison(t *testing.T) {
	files := newFiles()

	t.Run("passes when less", func(t *testing.T) {
		entity := &catalog.Entity{Metadata: map[string]string{"vuln.high": "3"}}
		rule := config.ScorecardRule{Name: "Low CVEs", Check: `cve_count("high") < 5`, Weight: 1}
		result := EvaluateRule(rule, files, entity)
		if !result.Passed {
			t.Error("expected cve_count(high) < 5 to pass when count is 3")
		}
	})

	t.Run("fails when equal", func(t *testing.T) {
		entity := &catalog.Entity{Metadata: map[string]string{"vuln.high": "5"}}
		rule := config.ScorecardRule{Name: "Low CVEs", Check: `cve_count("high") < 5`, Weight: 1}
		result := EvaluateRule(rule, files, entity)
		if result.Passed {
			t.Error("expected cve_count(high) < 5 to fail when count is 5")
		}
	})

	t.Run("fails when greater", func(t *testing.T) {
		entity := &catalog.Entity{Metadata: map[string]string{"vuln.high": "10"}}
		rule := config.ScorecardRule{Name: "Low CVEs", Check: `cve_count("high") < 5`, Weight: 1}
		result := EvaluateRule(rule, files, entity)
		if result.Passed {
			t.Error("expected cve_count(high) < 5 to fail when count is 10")
		}
	})
}

func TestTopicExactMatch(t *testing.T) {
	files := newFiles()
	entity := &catalog.Entity{
		Tags:     map[string]string{"topics": "kubernetes,monitoring,go"},
		Metadata: map[string]string{},
	}

	t.Run("partial should not match", func(t *testing.T) {
		rule := config.ScorecardRule{Name: "Kube", Check: `has_topic("kube")`, Weight: 1}
		result := EvaluateRule(rule, files, entity)
		if result.Passed {
			t.Error("expected has_topic(kube) to NOT match kubernetes (exact match required)")
		}
	})

	t.Run("exact should match", func(t *testing.T) {
		rule := config.ScorecardRule{Name: "Go", Check: `has_topic("go")`, Weight: 1}
		result := EvaluateRule(rule, files, entity)
		if !result.Passed {
			t.Error("expected has_topic(go) to match exactly")
		}
	})
}

func TestUnknownCheck(t *testing.T) {
	files := newFiles()
	entity := &catalog.Entity{Metadata: map[string]string{}}

	rule := config.ScorecardRule{Name: "Unknown", Check: "something_random()", Weight: 1}
	result := EvaluateRule(rule, files, entity)
	if result.Passed {
		t.Error("expected unknown check to fail")
	}
	if result.Detail == "" {
		t.Error("expected detail to contain error message")
	}
}

func TestDefaultWeight(t *testing.T) {
	files := newFiles("README.md")
	entity := &catalog.Entity{Metadata: map[string]string{}}

	rule := config.ScorecardRule{Name: "No Weight", Check: `has_file("README.md")`}
	result := EvaluateRule(rule, files, entity)
	if result.Weight != 1 {
		t.Errorf("default weight: got %d, want 1", result.Weight)
	}
}
