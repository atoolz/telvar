package catalog

import (
	"encoding/json"
	"strings"
)

func inferDependencies(files FileChecker, lang string) []string {
	switch lang {
	case "go":
		return parseGoMod(files)
	case "javascript":
		return parsePackageJSON(files)
	case "python":
		return parseRequirementsTxt(files)
	case "rust":
		return parseCargoToml(files)
	}
	return nil
}

func parseGoMod(files FileChecker) []string {
	data, err := files.ReadFile("go.mod")
	if err != nil {
		return nil
	}

	var deps []string
	inRequire := false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)

		if line == "require (" {
			inRequire = true
			continue
		}
		if line == ")" {
			inRequire = false
			continue
		}

		if inRequire {
			parts := strings.Fields(line)
			if len(parts) >= 1 && !strings.HasPrefix(parts[0], "//") {
				mod := parts[0]
				if strings.Contains(mod, "/") {
					deps = append(deps, "go:"+mod)
				}
			}
			continue
		}

		if strings.HasPrefix(line, "require ") && !strings.HasSuffix(line, "(") {
			parts := strings.Fields(line)
			if len(parts) >= 3 && strings.Contains(parts[1], "/") {
				deps = append(deps, "go:"+parts[1])
			}
		}
	}

	return deps
}

func parsePackageJSON(files FileChecker) []string {
	data, err := files.ReadFile("package.json")
	if err != nil {
		return nil
	}

	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}

	var deps []string
	for name := range pkg.Dependencies {
		deps = append(deps, "npm:"+name)
	}
	for name := range pkg.DevDependencies {
		deps = append(deps, "npm:"+name)
	}

	return deps
}

func parseRequirementsTxt(files FileChecker) []string {
	data, err := files.ReadFile("requirements.txt")
	if err != nil {
		return nil
	}

	var deps []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-") {
			continue
		}

		name := line
		for _, sep := range []string{"==", ">=", "<=", "!=", "~=", "<", ">"} {
			if idx := strings.Index(name, sep); idx >= 0 {
				name = name[:idx]
				break
			}
		}
		if idx := strings.Index(name, "["); idx >= 0 {
			name = name[:idx]
		}
		name = strings.TrimSpace(name)
		if name != "" {
			deps = append(deps, "python:"+name)
		}
	}

	return deps
}

// parseCargoToml extracts [dependencies] and [dev-dependencies] only.
// [build-dependencies] are intentionally excluded as they don't affect runtime.
func parseCargoToml(files FileChecker) []string {
	data, err := files.ReadFile("Cargo.toml")
	if err != nil {
		return nil
	}

	var deps []string
	inDeps := false
	inMultiLine := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)

		if inMultiLine {
			if strings.Contains(trimmed, "}") {
				inMultiLine = false
			}
			continue
		}

		if trimmed == "[dependencies]" || trimmed == "[dev-dependencies]" {
			inDeps = true
			continue
		}
		if strings.HasPrefix(trimmed, "[") {
			inDeps = false
			continue
		}

		if inDeps {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 {
				name := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				if name != "" && !strings.HasPrefix(name, "#") {
					if strings.HasPrefix(value, "{") && !strings.Contains(value, "}") {
						inMultiLine = true
					}
					deps = append(deps, "rust:"+name)
				}
			}
		}
	}

	return deps
}
