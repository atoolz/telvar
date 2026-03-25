package vuln

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/ahlert/telvar/internal/catalog"
)

type Checker struct {
	client *OSVClient
}

func NewChecker() *Checker {
	return &Checker{client: NewOSVClient()}
}

type Finding struct {
	Package  string
	Version  string
	VulnID   string
	Severity string
	Summary  string
}

func (c *Checker) CheckEntity(ctx context.Context, entity *catalog.Entity) ([]Finding, error) {
	if entity == nil || len(entity.Dependencies) == 0 {
		return nil, nil
	}

	var queries []OSVQuery
	for _, dep := range entity.Dependencies {
		parts := strings.SplitN(dep, ":", 2)
		if len(parts) != 2 {
			continue
		}

		ecosystem := mapEcosystem(parts[0])
		if ecosystem == "" {
			continue
		}

		name := parts[1]
		queries = append(queries, OSVQuery{
			Package: OSVPackage{
				Name:      name,
				Ecosystem: ecosystem,
			},
		})
	}

	if len(queries) == 0 {
		return nil, nil
	}

	results, err := c.client.QueryBatch(ctx, queries)
	if err != nil {
		return nil, fmt.Errorf("querying osv.dev: %w", err)
	}

	var findings []Finding
	for i, result := range results {
		if i >= len(queries) {
			break
		}
		for _, vuln := range result.Vulns {
			severity := classifySeverity(vuln)
			findings = append(findings, Finding{
				Package:  queries[i].Package.Name,
				VulnID:   vuln.ID,
				Severity: severity,
				Summary:  vuln.Summary,
			})
		}
	}

	return findings, nil
}

func (c *Checker) AnnotateEntity(ctx context.Context, entity *catalog.Entity) {
	if entity == nil {
		return
	}
	if entity.Metadata == nil {
		entity.Metadata = make(map[string]string)
	}

	findings, err := c.CheckEntity(ctx, entity)
	if err != nil {
		slog.Warn("CVE check failed", "entity", entity.Name, "error", err)
		return
	}

	counts := map[string]int{
		"critical": 0,
		"high":     0,
		"medium":   0,
		"low":      0,
	}

	for _, f := range findings {
		sev := f.Severity
		if _, ok := counts[sev]; !ok {
			sev = "low"
		}
		counts[sev]++
	}

	entity.Metadata["vuln.total"] = strconv.Itoa(len(findings))
	entity.Metadata["vuln.critical"] = strconv.Itoa(counts["critical"])
	entity.Metadata["vuln.high"] = strconv.Itoa(counts["high"])
	entity.Metadata["vuln.medium"] = strconv.Itoa(counts["medium"])
	entity.Metadata["vuln.low"] = strconv.Itoa(counts["low"])
}

func mapEcosystem(prefix string) string {
	switch prefix {
	case "go":
		return "Go"
	case "npm":
		return "npm"
	case "python":
		return "PyPI"
	case "rust":
		return "crates.io"
	default:
		return ""
	}
}

func classifySeverity(vuln OSVVuln) string {
	priority := []string{"cvss_v4", "cvss_v3", "cvss_v2"}
	for _, want := range priority {
		for _, s := range vuln.Severity {
			if strings.ToLower(s.Type) == want {
				return parseCVSSScore(s.Score)
			}
		}
	}

	for _, alias := range vuln.Aliases {
		if strings.HasPrefix(alias, "CVE-") {
			return "medium"
		}
	}

	return "low"
}

func parseCVSSScore(scoreStr string) string {
	score := 0.0

	if len(scoreStr) == 0 || (scoreStr[0] < '0' || scoreStr[0] > '9') {
		return "medium"
	}

	if _, err := fmt.Sscanf(scoreStr, "%f", &score); err != nil {
		return "medium"
	}

	switch {
	case score >= 9.0:
		return "critical"
	case score >= 7.0:
		return "high"
	case score >= 4.0:
		return "medium"
	default:
		return "low"
	}
}
