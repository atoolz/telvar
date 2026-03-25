package catalog

import (
	"testing"
)

func TestParseGoMod(t *testing.T) {
	files := newCheckerWithContent(map[string]string{
		"go.mod": `module github.com/example/svc

go 1.24

require (
	github.com/spf13/cobra v1.10.0
	github.com/spf13/pflag v1.0.9
	gopkg.in/yaml.v3 v3.0.1
)

require (
	// indirect
	github.com/inconshreveable/mousetrap v1.1.0
)
`,
	})

	deps := parseGoMod(files)
	if len(deps) != 4 {
		t.Errorf("expected 4 deps, got %d: %v", len(deps), deps)
	}

	expected := map[string]bool{
		"go:github.com/spf13/cobra":              true,
		"go:github.com/spf13/pflag":              true,
		"go:gopkg.in/yaml.v3":                    true,
		"go:github.com/inconshreveable/mousetrap": true,
	}
	for _, d := range deps {
		if !expected[d] {
			t.Errorf("unexpected dep: %s", d)
		}
	}
}

func TestParseGoModSingleLine(t *testing.T) {
	files := newCheckerWithContent(map[string]string{
		"go.mod": `module github.com/example/svc

go 1.24

require github.com/spf13/cobra v1.10.0
`,
	})

	deps := parseGoMod(files)
	if len(deps) != 1 || deps[0] != "go:github.com/spf13/cobra" {
		t.Errorf("expected [go:github.com/spf13/cobra], got %v", deps)
	}
}

func TestParsePackageJSON(t *testing.T) {
	files := newCheckerWithContent(map[string]string{
		"package.json": `{
	"name": "my-app",
	"dependencies": {
		"react": "^18.0.0",
		"next": "^14.0.0"
	},
	"devDependencies": {
		"typescript": "^5.0.0"
	}
}`,
	})

	deps := parsePackageJSON(files)
	if len(deps) != 3 {
		t.Errorf("expected 3 deps, got %d: %v", len(deps), deps)
	}

	has := make(map[string]bool)
	for _, d := range deps {
		has[d] = true
	}
	if !has["npm:react"] || !has["npm:next"] || !has["npm:typescript"] {
		t.Errorf("missing expected deps: %v", deps)
	}
}

func TestParseRequirementsTxt(t *testing.T) {
	files := newCheckerWithContent(map[string]string{
		"requirements.txt": `# Core
flask==2.3.0
requests>=2.28.0
boto3~=1.26
# Dev
-e git+https://github.com/example/lib.git
pytest
`,
	})

	deps := parseRequirementsTxt(files)
	expected := map[string]bool{
		"python:flask":    true,
		"python:requests": true,
		"python:boto3":    true,
		"python:pytest":   true,
	}

	if len(deps) != 4 {
		t.Errorf("expected 4 deps, got %d: %v", len(deps), deps)
	}
	for _, d := range deps {
		if !expected[d] {
			t.Errorf("unexpected dep: %s", d)
		}
	}
}

func TestParseCargoToml(t *testing.T) {
	files := newCheckerWithContent(map[string]string{
		"Cargo.toml": `[package]
name = "my-crate"
version = "0.1.0"

[dependencies]
serde = "1.0"
tokio = { version = "1", features = ["full"] }

[dev-dependencies]
pretty_assertions = "1.0"

[build-dependencies]
cc = "1.0"
`,
	})

	deps := parseCargoToml(files)
	expected := map[string]bool{
		"rust:serde":             true,
		"rust:tokio":             true,
		"rust:pretty_assertions": true,
	}

	if len(deps) != 3 {
		t.Errorf("expected 3 deps, got %d: %v", len(deps), deps)
	}
	for _, d := range deps {
		if !expected[d] {
			t.Errorf("unexpected dep: %s", d)
		}
	}
}

func TestParseGoModMissing(t *testing.T) {
	files := newCheckerWithContent(map[string]string{})
	deps := parseGoMod(files)
	if deps != nil {
		t.Errorf("expected nil for missing go.mod, got %v", deps)
	}
}

func TestInferDependenciesIntegration(t *testing.T) {
	files := newCheckerWithContent(map[string]string{
		"go.mod": `module example
go 1.24
require github.com/spf13/cobra v1.10.0
`,
	})

	entity := InferEntity("my-svc", "https://github.com/test/my-svc", files)
	if len(entity.Dependencies) != 1 {
		t.Errorf("expected 1 dependency, got %d: %v", len(entity.Dependencies), entity.Dependencies)
	}
}
