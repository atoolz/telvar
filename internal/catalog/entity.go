package catalog

import "time"

type EntityKind string

const (
	KindComponent EntityKind = "component"
	KindAPI       EntityKind = "api"
	KindResource  EntityKind = "resource"
	KindSystem    EntityKind = "system"
)

type Entity struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Kind         EntityKind        `json:"kind"`
	Description  string            `json:"description,omitempty"`
	Owner        string            `json:"owner,omitempty"`
	Team         string            `json:"team,omitempty"`
	Language     string            `json:"language,omitempty"`
	Framework    string            `json:"framework,omitempty"`
	RepoURL      string            `json:"repo_url,omitempty"`
	Dependencies []string          `json:"dependencies,omitempty"`
	Tags         map[string]string `json:"tags,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	Score        *ScoreResult      `json:"score,omitempty"`
	DiscoveredAt time.Time         `json:"discovered_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

type ScoreResult struct {
	Total    float64            `json:"total"`
	MaxTotal float64            `json:"max_total"`
	Percent  int                `json:"percent"`
	Rules    []RuleResult       `json:"rules"`
}

type RuleResult struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Weight int    `json:"weight"`
	Detail string `json:"detail,omitempty"`
}
