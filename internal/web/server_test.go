package web

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ahlert/telvar/assets"
	"github.com/ahlert/telvar/internal/catalog"
	"github.com/ahlert/telvar/internal/store"
)

func newTestServer(t *testing.T) (*Server, *store.Store) {
	t.Helper()

	s, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	tmplFS, err := fs.Sub(assets.Templates, "templates")
	if err != nil {
		t.Fatalf("fs.Sub templates: %v", err)
	}
	statFS, err := fs.Sub(assets.Static, "static")
	if err != nil {
		t.Fatalf("fs.Sub static: %v", err)
	}

	srv, err := New(s, tmplFS, statFS)
	if err != nil {
		t.Fatalf("web.New: %v", err)
	}

	return srv, s
}

func TestHealthz(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("healthz: got %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"ok":true`) {
		t.Errorf("healthz body: %s", w.Body.String())
	}
}

func TestCatalogListEmpty(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("catalog list: got %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Services") {
		t.Error("page should contain 'Services' heading")
	}
	if !strings.Contains(body, "No services found") {
		t.Error("empty state should show 'No services found'")
	}
}

func TestCatalogListWithEntities(t *testing.T) {
	srv, s := newTestServer(t)

	e := &catalog.Entity{
		ID:          "my-api-abc123",
		Name:        "my-api",
		Kind:        catalog.KindAPI,
		Description: "My awesome API",
		Language:    "go",
		Owner:       "platform",
		RepoURL:     "https://github.com/test/my-api",
	}
	if err := s.UpsertEntity(e); err != nil {
		t.Fatalf("UpsertEntity: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "my-api") {
		t.Error("page should contain entity name 'my-api'")
	}
	if !strings.Contains(body, "My awesome API") {
		t.Error("page should contain entity description")
	}
	if !strings.Contains(body, "api") {
		t.Error("page should contain kind badge")
	}
}

func TestEntityDetailFound(t *testing.T) {
	srv, s := newTestServer(t)

	e := &catalog.Entity{
		ID:          "my-svc-abc123",
		Name:        "my-svc",
		Kind:        catalog.KindComponent,
		Description: "Test service",
		Language:    "rust",
		Owner:       "infra-team",
		RepoURL:     "https://github.com/test/my-svc",
		Tags:        map[string]string{"env": "prod"},
		Metadata:    map[string]string{"stars": "99"},
	}
	if err := s.UpsertEntity(e); err != nil {
		t.Fatalf("UpsertEntity: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/entity/my-svc-abc123", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("entity detail: got %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "my-svc") {
		t.Error("page should contain entity name")
	}
	if !strings.Contains(body, "infra-team") {
		t.Error("page should contain owner")
	}
	if !strings.Contains(body, "rust") {
		t.Error("page should contain language")
	}
}

func TestEntityDetailNotFound(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/entity/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("entity not found: got %d, want 404", w.Code)
	}
}

func TestHtmxCatalogPartial(t *testing.T) {
	srv, s := newTestServer(t)

	s.UpsertEntity(&catalog.Entity{
		ID: "go-svc-abc", Name: "go-service", Kind: catalog.KindComponent, Language: "go",
	})
	s.UpsertEntity(&catalog.Entity{
		ID: "py-svc-abc", Name: "py-service", Kind: catalog.KindComponent, Language: "python",
	})

	req := httptest.NewRequest(http.MethodGet, "/htmx/catalog/list?lang=go", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("htmx partial: got %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "go-service") {
		t.Error("should contain go-service")
	}
	if strings.Contains(body, "py-service") {
		t.Error("should NOT contain py-service when filtering by go")
	}
}

func TestStaticFiles(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/static/style.css", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("static css: got %d, want 200", w.Code)
	}
	if !strings.Contains(w.Header().Get("Content-Type"), "text/css") {
		t.Errorf("Content-Type: %s", w.Header().Get("Content-Type"))
	}
}
