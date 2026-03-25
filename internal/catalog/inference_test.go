package catalog

import (
	"fmt"
	"path/filepath"
	"testing"
)

type mapFileChecker struct {
	files map[string][]byte
}

func (m *mapFileChecker) HasFile(p string) bool {
	_, ok := m.files[p]
	return ok
}

func (m *mapFileChecker) HasFileGlob(pattern string) bool {
	for p := range m.files {
		if matched, _ := filepath.Match(pattern, p); matched {
			return true
		}
	}
	return false
}

func (m *mapFileChecker) ReadFile(p string) ([]byte, error) {
	data, ok := m.files[p]
	if !ok {
		return nil, fmt.Errorf("file not found: %s", p)
	}
	return data, nil
}

func newChecker(files ...string) *mapFileChecker {
	m := &mapFileChecker{files: make(map[string][]byte)}
	for _, f := range files {
		m.files[f] = []byte{}
	}
	return m
}

func newCheckerWithContent(files map[string]string) *mapFileChecker {
	m := &mapFileChecker{files: make(map[string][]byte)}
	for k, v := range files {
		m.files[k] = []byte(v)
	}
	return m
}

func TestInferLanguage(t *testing.T) {
	tests := []struct {
		name     string
		file     string
		expected string
	}{
		{"go", "go.mod", "go"},
		{"rust", "Cargo.toml", "rust"},
		{"javascript", "package.json", "javascript"},
		{"python pyproject", "pyproject.toml", "python"},
		{"python setup", "setup.py", "python"},
		{"python requirements", "requirements.txt", "python"},
		{"ruby", "Gemfile", "ruby"},
		{"java maven", "pom.xml", "java"},
		{"java gradle", "build.gradle", "java"},
		{"php", "composer.json", "php"},
		{"elixir", "mix.exs", "elixir"},
		{"dart", "pubspec.yaml", "dart"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fc := newChecker(tt.file)
			got := inferLanguage(fc)
			if got != tt.expected {
				t.Errorf("inferLanguage with %s: got %q, want %q", tt.file, got, tt.expected)
			}
		})
	}

	t.Run("unknown", func(t *testing.T) {
		fc := newChecker("README.md")
		got := inferLanguage(fc)
		if got != "" {
			t.Errorf("inferLanguage with no lang file: got %q, want empty", got)
		}
	})
}

func TestInferLanguagePriority(t *testing.T) {
	fc := newChecker("go.mod", "package.json")
	got := inferLanguage(fc)
	if got != "go" {
		t.Errorf("go.mod should take priority over package.json: got %q", got)
	}
}

func TestInferFramework(t *testing.T) {
	tests := []struct {
		name      string
		lang      string
		files     []string
		expected  string
	}{
		{"nextjs", "javascript", []string{"package.json", "next.config.js"}, "nextjs"},
		{"nextjs mjs", "javascript", []string{"package.json", "next.config.mjs"}, "nextjs"},
		{"nuxt", "javascript", []string{"package.json", "nuxt.config.ts"}, "nuxt"},
		{"astro", "javascript", []string{"package.json", "astro.config.mjs"}, "astro"},
		{"django", "python", []string{"pyproject.toml", "manage.py"}, "django"},
		{"rails", "ruby", []string{"Gemfile", "config/routes.rb"}, "rails"},
		{"go service", "go", []string{"go.mod", "main.go"}, "go-service"},
		{"rust service", "rust", []string{"Cargo.toml", "src/main.rs"}, "rust-service"},
		{"spring properties", "java", []string{"pom.xml", "src/main/resources/application.properties"}, "spring"},
		{"spring yml", "java", []string{"pom.xml", "src/main/resources/application.yml"}, "spring"},
		{"plain js", "javascript", []string{"package.json"}, ""},
		{"plain python", "python", []string{"pyproject.toml"}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fc := newChecker(tt.files...)
			got := inferFramework(fc, tt.lang)
			if got != tt.expected {
				t.Errorf("inferFramework(%s): got %q, want %q", tt.name, got, tt.expected)
			}
		})
	}
}

func TestInferKind(t *testing.T) {
	tests := []struct {
		name     string
		files    []string
		expected EntityKind
	}{
		{"openapi yaml", []string{"openapi.yaml"}, KindAPI},
		{"openapi json", []string{"openapi.json"}, KindAPI},
		{"swagger yaml", []string{"swagger.yaml"}, KindAPI},
		{"swagger json", []string{"swagger.json"}, KindAPI},
		{"terraform", []string{"main.tf"}, KindResource},
		{"terraform subdir", []string{"terraform/main.tf"}, KindResource},
		{"api with dockerfile", []string{"openapi.yaml", "Dockerfile"}, KindAPI},
		{"plain dockerfile", []string{"Dockerfile"}, KindComponent},
		{"docker compose", []string{"docker-compose.yml"}, KindComponent},
		{"nothing", []string{"README.md"}, KindComponent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fc := newChecker(tt.files...)
			got := inferKind(fc, "")
			if got != tt.expected {
				t.Errorf("inferKind(%s): got %q, want %q", tt.name, got, tt.expected)
			}
		})
	}
}

func TestInferKindAPIPriority(t *testing.T) {
	fc := newChecker("openapi.yaml", "Dockerfile", "main.tf")
	got := inferKind(fc, "")
	if got != KindAPI {
		t.Errorf("OpenAPI should take priority: got %q, want %q", got, KindAPI)
	}
}

func TestInferOwner(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		expected string
	}{
		{
			"codeowners root",
			map[string]string{"CODEOWNERS": "* @platform-team\n"},
			"platform-team",
		},
		{
			"codeowners github dir",
			map[string]string{".github/CODEOWNERS": "# Global\n* @backend-team\n"},
			"backend-team",
		},
		{
			"codeowners with comments",
			map[string]string{"CODEOWNERS": "# This is a comment\n\n* @infra-team\n/docs @docs-team\n"},
			"infra-team",
		},
		{
			"no codeowners",
			map[string]string{"README.md": "# Hello"},
			"",
		},
		{
			"codeowners no wildcard",
			map[string]string{"CODEOWNERS": "/src @src-team\n"},
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fc := newCheckerWithContent(tt.files)
			got := inferOwner(fc)
			if got != tt.expected {
				t.Errorf("inferOwner(%s): got %q, want %q", tt.name, got, tt.expected)
			}
		})
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"my-service", "my-service"},
		{"My Service", "my-service"},
		{"my_service", "my-service"},
		{"my.service", "my-service"},
		{"my/service", "my-service"},
		{"MY-SERVICE-123", "my-service-123"},
		{"---", "unnamed"},
		{"...", "unnamed"},
		{"", "unnamed"},
		{"@special#chars!", "special-chars"},
		{"  spaces  ", "spaces"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := slugify(tt.input)
			if got != tt.expected {
				t.Errorf("slugify(%q): got %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestUniqueSlugDeterminism(t *testing.T) {
	a := uniqueSlug("my-service")
	b := uniqueSlug("my-service")
	if a != b {
		t.Errorf("uniqueSlug should be deterministic: %q != %q", a, b)
	}
}

func TestUniqueSlugCollisionResistance(t *testing.T) {
	a := uniqueSlug("my.service")
	b := uniqueSlug("my-service")
	if a == b {
		t.Errorf("uniqueSlug should produce different IDs for my.service and my-service: both got %q", a)
	}
}

func TestInferEntityFull(t *testing.T) {
	fc := newCheckerWithContent(map[string]string{
		"go.mod":             "module github.com/example/svc",
		"main.go":            "package main",
		"Dockerfile":         "FROM golang",
		"openapi.yaml":       "openapi: 3.0.0",
		".github/CODEOWNERS": "* @platform\n",
	})

	e := InferEntity("my-api", "https://github.com/example/my-api", fc)

	if e.Language != "go" {
		t.Errorf("Language: got %q, want go", e.Language)
	}
	if e.Framework != "go-service" {
		t.Errorf("Framework: got %q, want go-service", e.Framework)
	}
	if e.Kind != KindAPI {
		t.Errorf("Kind: got %q, want %q", e.Kind, KindAPI)
	}
	if e.Owner != "platform" {
		t.Errorf("Owner: got %q, want platform", e.Owner)
	}
	if e.Name != "my-api" {
		t.Errorf("Name: got %q, want my-api", e.Name)
	}
	if e.RepoURL != "https://github.com/example/my-api" {
		t.Errorf("RepoURL: got %q", e.RepoURL)
	}
	if e.ID == "" {
		t.Error("ID should not be empty")
	}
}
