package config

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Connectors ConnectorsConfig `yaml:"connectors"`
	Discovery  DiscoveryConfig  `yaml:"discovery"`
	Scorecards []Scorecard      `yaml:"scorecards"`
}

type ServerConfig struct {
	Port    int    `yaml:"port"`
	BaseURL string `yaml:"base_url"`
}

type ConnectorsConfig struct {
	GitHub     *GitHubConfig     `yaml:"github,omitempty"`
	Kubernetes *KubernetesConfig `yaml:"kubernetes,omitempty"`
	PagerDuty  *PagerDutyConfig  `yaml:"pagerduty,omitempty"`
}

type GitHubConfig struct {
	Org   string `yaml:"org"`
	Token string `yaml:"token"`
}

type KubernetesConfig struct {
	Kubeconfig string `yaml:"kubeconfig,omitempty"`
	InCluster  bool   `yaml:"in_cluster,omitempty"`
}

type PagerDutyConfig struct {
	APIKey string `yaml:"api_key"`
}

type DiscoveryConfig struct {
	ScanInterval   string `yaml:"scan_interval"`
	IgnoreArchived bool   `yaml:"ignore_archived"`
	IgnoreForks    bool   `yaml:"ignore_forks"`
	RepoFilter     string `yaml:"repo_filter,omitempty"`
}

type Scorecard struct {
	Name  string          `yaml:"name"`
	Rules []ScorecardRule `yaml:"rules"`
}

type ScorecardRule struct {
	Name   string `yaml:"name"`
	Check  string `yaml:"check"`
	Weight int    `yaml:"weight,omitempty"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	expandSecrets(&cfg)

	if cfg.Server.Port == 0 {
		cfg.Server.Port = 7007
	}
	if cfg.Discovery.ScanInterval == "" {
		cfg.Discovery.ScanInterval = "30m"
	}

	return &cfg, nil
}

func expandSecrets(cfg *Config) {
	if cfg.Connectors.GitHub != nil {
		cfg.Connectors.GitHub.Token = expandEnv(cfg.Connectors.GitHub.Token)
	}
	if cfg.Connectors.PagerDuty != nil {
		cfg.Connectors.PagerDuty.APIKey = expandEnv(cfg.Connectors.PagerDuty.APIKey)
	}
}

func expandEnv(s string) string {
	return os.Expand(s, func(key string) string {
		if v, ok := os.LookupEnv(key); ok {
			return v
		}
		return "${" + key + "}"
	})
}

func WriteDefault(path string) error {
	cfg := Config{
		Server: ServerConfig{
			Port:    7007,
			BaseURL: "http://localhost:7007",
		},
		Connectors: ConnectorsConfig{
			GitHub: &GitHubConfig{
				Org:   "your-org",
				Token: "${GITHUB_TOKEN}",
			},
		},
		Discovery: DiscoveryConfig{
			ScanInterval:   "30m",
			IgnoreArchived: true,
			IgnoreForks:    true,
		},
		Scorecards: []Scorecard{
			{
				Name: "production-readiness",
				Rules: []ScorecardRule{
					{Name: "Has CI", Check: `has_file(".github/workflows/*.yml")`, Weight: 2},
					{Name: "Has README", Check: `has_file("README.md")`, Weight: 1},
					{Name: "Has CODEOWNERS", Check: `has_file("CODEOWNERS")`, Weight: 1},
					{Name: "No Critical CVEs", Check: `cve_count("critical") == 0`, Weight: 3},
				},
			},
		},
	}

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("marshaling default config: %w", err)
	}

	header := "# Telvar configuration\n# Docs: https://telvar.dev/docs/config\n\n"
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("%s already exists; delete it first to regenerate", path)
		}
		return fmt.Errorf("creating %s: %w", path, err)
	}
	defer f.Close()

	if _, err := f.Write(append([]byte(header), data...)); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	fmt.Printf("Created %s\n", path)
	return nil
}
