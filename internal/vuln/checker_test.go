package vuln

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ahlert/telvar/internal/catalog"
)

func TestCheckEntityNoDeps(t *testing.T) {
	checker := NewChecker()
	entity := &catalog.Entity{Name: "empty"}

	findings, err := checker.CheckEntity(context.Background(), entity)
	if err != nil {
		t.Fatalf("CheckEntity: %v", err)
	}
	if findings != nil {
		t.Errorf("expected nil findings for no deps, got %v", findings)
	}
}

func TestCheckEntityNilEntity(t *testing.T) {
	checker := NewChecker()

	findings, err := checker.CheckEntity(context.Background(), nil)
	if err != nil {
		t.Fatalf("CheckEntity: %v", err)
	}
	if findings != nil {
		t.Errorf("expected nil findings for nil entity, got %v", findings)
	}
}

func TestCheckEntityWithMockOSV(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := OSVBatchResponse{
			Results: []OSVResult{
				{Vulns: []OSVVuln{
					{ID: "GHSA-1234", Summary: "test vuln", Aliases: []string{"CVE-2024-1234"}},
				}},
				{Vulns: nil},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	checker := NewChecker()
	checker.client.http = srv.Client()
	origURL := osvBatchURL

	oldTransport := checker.client.http.Transport
	checker.client.http.Transport = &rewriteTransport{
		base:    oldTransport,
		baseURL: srv.URL,
	}
	_ = origURL

	entity := &catalog.Entity{
		Name: "test-svc",
		Dependencies: []string{
			"go:github.com/vuln/pkg",
			"go:github.com/safe/pkg",
		},
		Metadata: make(map[string]string),
	}

	checker.AnnotateEntity(context.Background(), entity)

	if entity.Metadata["vuln.total"] != "1" {
		t.Errorf("vuln.total: got %s, want 1", entity.Metadata["vuln.total"])
	}
}

func TestMapEcosystem(t *testing.T) {
	tests := []struct {
		prefix   string
		expected string
	}{
		{"go", "Go"},
		{"npm", "npm"},
		{"python", "PyPI"},
		{"rust", "crates.io"},
		{"unknown", ""},
	}
	for _, tt := range tests {
		t.Run(tt.prefix, func(t *testing.T) {
			if got := mapEcosystem(tt.prefix); got != tt.expected {
				t.Errorf("mapEcosystem(%q): got %q, want %q", tt.prefix, got, tt.expected)
			}
		})
	}
}

func TestClassifySeverity(t *testing.T) {
	tests := []struct {
		name     string
		vuln     OSVVuln
		expected string
	}{
		{"critical cvss", OSVVuln{Severity: []OSVSeverity{{Type: "CVSS_V3", Score: "9.8/..."}}}, "critical"},
		{"high cvss", OSVVuln{Severity: []OSVSeverity{{Type: "CVSS_V3", Score: "7.5/..."}}}, "high"},
		{"medium cvss", OSVVuln{Severity: []OSVSeverity{{Type: "CVSS_V3", Score: "5.0/..."}}}, "medium"},
		{"low cvss", OSVVuln{Severity: []OSVSeverity{{Type: "CVSS_V3", Score: "2.0/..."}}}, "low"},
		{"cve alias no score", OSVVuln{Aliases: []string{"CVE-2024-1234"}}, "medium"},
		{"zero score", OSVVuln{Severity: []OSVSeverity{{Type: "CVSS_V3", Score: "0.0"}}}, "low"},
		{"no info", OSVVuln{}, "low"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifySeverity(tt.vuln); got != tt.expected {
				t.Errorf("classifySeverity: got %q, want %q", got, tt.expected)
			}
		})
	}
}

type rewriteTransport struct {
	base    http.RoundTripper
	baseURL string
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL.Scheme = "http"
	clone.URL.Host = t.baseURL[len("http://"):]
	if t.base != nil {
		return t.base.RoundTrip(clone)
	}
	return http.DefaultTransport.RoundTrip(clone)
}
