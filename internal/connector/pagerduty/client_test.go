package pagerduty

import (
	"context"
	"net/http"
	"net/url"
	"net/http/httptest"
	"testing"

	"github.com/ahlert/telvar/internal/config"
)

func TestListOnCallNow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Token token=test-key" {
			t.Error("missing auth header")
		}

		resp := `{
			"oncalls": [{
				"user": {"summary": "Alice", "email": "alice@example.com"},
				"schedule": {"summary": "Backend Schedule"},
				"escalation_policy": {"summary": "Backend Policy"}
			}]
		}`
		w.Write([]byte(resp))
	}))
	defer srv.Close()

	client := NewClient(&config.PagerDutyConfig{APIKey: "test-key"})
	client.http = srv.Client()

	origTransport := client.http.Transport
	client.http.Transport = &rewriteTransport{base: origTransport, url: srv.URL}

	oncalls, err := client.ListOnCallNow(context.Background())
	if err != nil {
		t.Fatalf("ListOnCallNow: %v", err)
	}

	if len(oncalls) != 1 {
		t.Fatalf("expected 1 oncall, got %d", len(oncalls))
	}
	if oncalls[0].UserName != "Alice" {
		t.Errorf("UserName: got %q, want Alice", oncalls[0].UserName)
	}
	if oncalls[0].UserEmail != "alice@example.com" {
		t.Errorf("UserEmail: got %q", oncalls[0].UserEmail)
	}
	if oncalls[0].ScheduleName != "Backend Schedule" {
		t.Errorf("ScheduleName: got %q", oncalls[0].ScheduleName)
	}
}

func TestListOnCallNowError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"message":"Invalid API key"}}`))
	}))
	defer srv.Close()

	client := NewClient(&config.PagerDutyConfig{APIKey: "bad-key"})
	client.http = srv.Client()
	client.http.Transport = &rewriteTransport{base: nil, url: srv.URL}

	_, err := client.ListOnCallNow(context.Background())
	if err == nil {
		t.Fatal("expected error for 401")
	}
}

type rewriteTransport struct {
	base http.RoundTripper
	url  string
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	parsed, _ := url.Parse(t.url)
	clone.URL.Scheme = parsed.Scheme
	clone.URL.Host = parsed.Host
	if t.base != nil {
		return t.base.RoundTrip(clone)
	}
	return http.DefaultTransport.RoundTrip(clone)
}
