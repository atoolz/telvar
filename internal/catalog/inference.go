package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

type FileChecker interface {
	HasFile(path string) bool
	HasFileGlob(pattern string) bool
	ReadFile(path string) ([]byte, error)
}

func InferEntity(name string, repoURL string, files FileChecker) Entity {
	e := Entity{
		ID:      uniqueSlug(name),
		Name:    name,
		Kind:    KindComponent,
		RepoURL: repoURL,
		Tags:     make(map[string]string),
		Metadata: make(map[string]string),
	}

	e.Language = inferLanguage(files)
	e.Framework = inferFramework(files, e.Language)
	e.Kind = inferKind(files, e.Language)

	if owner := inferOwner(files); owner != "" {
		e.Owner = owner
	}

	e.Dependencies = inferDependencies(files, e.Language)

	return e
}

func inferLanguage(files FileChecker) string {
	patterns := []struct {
		file string
		lang string
	}{
		{"go.mod", "go"},
		{"Cargo.toml", "rust"},
		{"package.json", "javascript"},
		{"pyproject.toml", "python"},
		{"setup.py", "python"},
		{"requirements.txt", "python"},
		{"Gemfile", "ruby"},
		{"pom.xml", "java"},
		{"build.gradle", "java"},
		{"composer.json", "php"},
		{"mix.exs", "elixir"},
		{"pubspec.yaml", "dart"},
	}

	for _, p := range patterns {
		if files.HasFile(p.file) {
			return p.lang
		}
	}
	return ""
}

func inferFramework(files FileChecker, lang string) string {
	switch lang {
	case "javascript":
		if files.HasFileGlob("next.config.*") {
			return "nextjs"
		}
		if files.HasFileGlob("nuxt.config.*") {
			return "nuxt"
		}
		if files.HasFileGlob("astro.config.*") {
			return "astro"
		}
	case "python":
		if files.HasFile("manage.py") {
			return "django"
		}
	case "ruby":
		if files.HasFile("config/routes.rb") {
			return "rails"
		}
	case "go":
		if files.HasFile("main.go") {
			return "go-service"
		}
	case "rust":
		if files.HasFile("src/main.rs") {
			return "rust-service"
		}
	case "java":
		if files.HasFile("src/main/resources/application.properties") || files.HasFile("src/main/resources/application.yml") {
			return "spring"
		}
	}
	return ""
}

func inferKind(files FileChecker, lang string) EntityKind {
	if files.HasFile("openapi.yaml") || files.HasFile("openapi.json") || files.HasFile("swagger.yaml") || files.HasFile("swagger.json") {
		return KindAPI
	}
	if files.HasFileGlob("terraform/*.tf") || files.HasFileGlob("*.tf") {
		return KindResource
	}
	return KindComponent
}

func inferOwner(files FileChecker) string {
	for _, path := range []string{"CODEOWNERS", ".github/CODEOWNERS", "docs/CODEOWNERS"} {
		data, err := files.ReadFile(path)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.Fields(line)
			if len(parts) >= 2 && parts[0] == "*" {
				return strings.TrimPrefix(parts[1], "@")
			}
		}
	}
	return ""
}

func uniqueSlug(name string) string {
	slug := slugify(name)
	h := sha256.Sum256([]byte(name))
	short := hex.EncodeToString(h[:3])
	return slug + "-" + short
}

func slugify(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	prev := '-'
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prev = r
		default:
			if prev != '-' {
				b.WriteRune('-')
				prev = '-'
			}
		}
	}
	result := strings.Trim(b.String(), "-")
	if result == "" {
		return "unnamed"
	}
	return result
}
