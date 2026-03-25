package web

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/ahlert/telvar/internal/docs"
	"github.com/ahlert/telvar/internal/store"
)

var tmplFuncs = template.FuncMap{
	"safeURL": func(u string) template.URL {
		if strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "http://") {
			return template.URL(u)
		}
		return template.URL("#unsafe-url")
	},
}

type Server struct {
	store    *store.Store
	docs     *docs.Fetcher
	pages    map[string]*template.Template
	staticFS fs.FS
	mux      *http.ServeMux
}

func New(s *store.Store, tmplFS fs.FS, statFS fs.FS, docsFetcher *docs.Fetcher) (*Server, error) {
	pages := map[string][]string{
		"catalog_list":  {"layout.html", "catalog_list.html", "entity_cards.html"},
		"entity_detail": {"layout.html", "entity_detail.html"},
		"entity_docs":   {"layout.html", "entity_docs.html"},
		"teams_list":    {"layout.html", "teams_list.html"},
		"team_detail":   {"layout.html", "team_detail.html", "entity_cards.html"},
	}

	templates := make(map[string]*template.Template)
	for name, files := range pages {
		t, err := template.New(name).Funcs(tmplFuncs).ParseFS(tmplFS, files...)
		if err != nil {
			return nil, fmt.Errorf("parsing template %s: %w", name, err)
		}
		templates[name] = t
	}

	cardsT, err := template.New("entity_cards").Funcs(tmplFuncs).ParseFS(tmplFS, "entity_cards.html")
	if err != nil {
		return nil, fmt.Errorf("parsing entity_cards partial: %w", err)
	}
	templates["entity_cards"] = cardsT

	srv := &Server{
		store:    s,
		docs:     docsFetcher,
		pages:    templates,
		staticFS: statFS,
		mux:      http.NewServeMux(),
	}

	srv.routes()
	return srv, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) ListenAndServe(ctx context.Context, addr string) error {
	srv := &http.Server{Addr: addr, Handler: s}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("web server starting", "addr", addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		slog.Info("shutting down web server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

func (s *Server) routes() {
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(s.staticFS)))

	s.mux.HandleFunc("GET /", s.handleCatalogList)
	s.mux.HandleFunc("GET /entity/{id}", s.handleEntityDetail)
	s.mux.HandleFunc("GET /entity/{id}/docs", s.handleEntityDocs)
	s.mux.HandleFunc("GET /teams", s.handleTeamsList)
	s.mux.HandleFunc("GET /teams/{team}", s.handleTeamDetail)
	s.mux.HandleFunc("GET /htmx/catalog/list", s.handleCatalogListPartial)
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
}

func (s *Server) render(w http.ResponseWriter, page string, data any) {
	t, ok := s.pages[page]
	if !ok {
		slog.Error("template not found", "page", page)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout", data); err != nil {
		slog.Error("template render failed", "page", page, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, writeErr := buf.WriteTo(w); writeErr != nil {
		slog.Error("writing response", "error", writeErr)
	}
}

func (s *Server) renderPartial(w http.ResponseWriter, partial string, data any) {
	t, ok := s.pages[partial]
	if !ok {
		slog.Error("partial template not found", "partial", partial)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, partial, data); err != nil {
		slog.Error("partial render failed", "partial", partial, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, writeErr := buf.WriteTo(w); writeErr != nil {
		slog.Error("writing response", "error", writeErr)
	}
}
