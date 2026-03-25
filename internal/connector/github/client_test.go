package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ahlert/telvar/internal/config"
)

func newTestServer(t *testing.T, handler http.Handler) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	client := NewClient(&config.GitHubConfig{
		Org:   "test-org",
		Token: "test-token",
	})
	client.baseURL = srv.URL

	return client, srv
}

func TestListReposPagination(t *testing.T) {
	page1 := []Repo{
		{Name: "repo-a", FullName: "test-org/repo-a", DefaultBranch: "main"},
		{Name: "repo-b", FullName: "test-org/repo-b", DefaultBranch: "main"},
	}
	page2 := []Repo{
		{Name: "repo-c", FullName: "test-org/repo-c", DefaultBranch: "main"},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/orgs/test-org/repos", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("missing auth header")
		}

		page := r.URL.Query().Get("page")
		switch page {
		case "1", "":
			w.Header().Set("Link", fmt.Sprintf(`<http://%s/orgs/test-org/repos?page=2>; rel="next"`, r.Host))
			json.NewEncoder(w).Encode(page1)
		case "2":
			json.NewEncoder(w).Encode(page2)
		default:
			t.Errorf("unexpected page: %s", page)
		}
	})

	client, _ := newTestServer(t, mux)
	repos, err := client.ListRepos(context.Background())
	if err != nil {
		t.Fatalf("ListRepos: %v", err)
	}

	if len(repos) != 3 {
		t.Errorf("expected 3 repos, got %d", len(repos))
	}
}

func TestListReposSinglePage(t *testing.T) {
	repos := []Repo{
		{Name: "only-repo", DefaultBranch: "main"},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/orgs/test-org/repos", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(repos)
	})

	client, _ := newTestServer(t, mux)
	result, err := client.ListRepos(context.Background())
	if err != nil {
		t.Fatalf("ListRepos: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 repo, got %d", len(result))
	}
}

func TestGetTree(t *testing.T) {
	tree := treeResponse{
		SHA: "abc123",
		Tree: []treeNode{
			{Path: "go.mod", Type: "blob"},
			{Path: "main.go", Type: "blob"},
			{Path: "internal", Type: "tree"},
			{Path: "internal/app.go", Type: "blob"},
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/test-org/my-repo/git/trees/sha123", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(tree)
	})

	client, _ := newTestServer(t, mux)
	paths, err := client.GetTree(context.Background(), "my-repo", "sha123")
	if err != nil {
		t.Fatalf("GetTree: %v", err)
	}

	if len(paths) != 3 {
		t.Errorf("expected 3 blob paths, got %d: %v", len(paths), paths)
	}

	expected := map[string]bool{"go.mod": true, "main.go": true, "internal/app.go": true}
	for _, p := range paths {
		if !expected[p] {
			t.Errorf("unexpected path: %s", p)
		}
	}
}

func TestGetFileContent(t *testing.T) {
	content := "package main\n\nfunc main() {}\n"
	encoded := base64.StdEncoding.EncodeToString([]byte(content))

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/test-org/my-repo/contents/main.go", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(fileContentResponse{
			Content:  encoded,
			Encoding: "base64",
		})
	})

	client, _ := newTestServer(t, mux)
	data, err := client.GetFileContent(context.Background(), "my-repo", "main.go")
	if err != nil {
		t.Fatalf("GetFileContent: %v", err)
	}

	if string(data) != content {
		t.Errorf("content mismatch: got %q, want %q", string(data), content)
	}
}

func TestGetFileContentNotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/test-org/my-repo/contents/missing.go", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message": "Not Found"}`))
	})

	client, _ := newTestServer(t, mux)
	_, err := client.GetFileContent(context.Background(), "my-repo", "missing.go")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestRateLimitHeader(t *testing.T) {
	calls := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/orgs/test-org/repos", func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("X-RateLimit-Remaining", "50")
		json.NewEncoder(w).Encode([]Repo{})
	})

	client, _ := newTestServer(t, mux)
	_, err := client.ListRepos(context.Background())
	if err != nil {
		t.Fatalf("ListRepos: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}
