package github

import (
	"context"
	"testing"
)

func TestRepoFileCheckerHasFile(t *testing.T) {
	fc := NewRepoFileChecker(context.Background(), nil, "test-repo", []string{
		"go.mod",
		"main.go",
		"internal/app.go",
		"Dockerfile",
	})

	tests := []struct {
		path string
		want bool
	}{
		{"go.mod", true},
		{"main.go", true},
		{"internal/app.go", true},
		{"Dockerfile", true},
		{"missing.txt", false},
		{"GO.MOD", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := fc.HasFile(tt.path); got != tt.want {
				t.Errorf("HasFile(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestRepoFileCheckerHasFileGlob(t *testing.T) {
	fc := NewRepoFileChecker(context.Background(), nil, "test-repo", []string{
		"next.config.js",
		"terraform/main.tf",
		"terraform/variables.tf",
		"openapi.yaml",
		"src/main.rs",
		"helm/mychart/Chart.yaml",
		"deploy/k8s/base/Dockerfile",
	})

	tests := []struct {
		pattern string
		want    bool
	}{
		{"next.config.*", true},
		{"terraform/*.tf", true},
		{"*.tf", false},
		{"*.yaml", true},
		{"src/*.rs", true},
		{"*.go", false},
		{"**/Chart.yaml", true},
		{"**/*.tf", true},
		{"**/Dockerfile", true},
		{"**/missing.txt", false},
		{"helm/**/Chart.yaml", true},
		{"**/helm/**/Chart.yaml", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			if got := fc.HasFileGlob(tt.pattern); got != tt.want {
				t.Errorf("HasFileGlob(%q) = %v, want %v", tt.pattern, got, tt.want)
			}
		})
	}
}

func TestRepoFileCheckerReadFileNotFound(t *testing.T) {
	fc := NewRepoFileChecker(context.Background(), nil, "test-repo", []string{"go.mod"})

	_, err := fc.ReadFile("missing.txt")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
