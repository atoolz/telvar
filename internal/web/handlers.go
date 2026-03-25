package web

import (
	"log/slog"
	"net/http"
	"sort"
	"strings"

	"github.com/ahlert/telvar/internal/catalog"
)

type catalogListData struct {
	CurrentPath string
	Entities    []catalog.Entity
	TotalCount  int
	Query       string
	Kind        string
	Lang        string
	Languages   []string
}

type entityDetailData struct {
	CurrentPath string
	Entity      *catalog.Entity
}

func (s *Server) handleCatalogList(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	q := r.URL.Query().Get("q")
	kind := r.URL.Query().Get("kind")
	lang := r.URL.Query().Get("lang")

	entities, err := s.searchEntities(q, kind, lang)
	if err != nil {
		slog.Error("listing entities", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	total, _ := s.store.CountEntities()
	languages := s.collectLanguages()

	s.render(w, "catalog_list", catalogListData{
		CurrentPath: "/",
		Entities:    entities,
		TotalCount:  total,
		Query:       q,
		Kind:        kind,
		Lang:        lang,
		Languages:   languages,
	})
}

func (s *Server) handleCatalogListPartial(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	kind := r.URL.Query().Get("kind")
	lang := r.URL.Query().Get("lang")

	entities, err := s.searchEntities(q, kind, lang)
	if err != nil {
		slog.Error("searching entities", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	s.renderPartial(w, "entity_cards", entities)
}

func (s *Server) handleEntityDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	entity, err := s.store.GetEntity(id)
	if err != nil {
		slog.Error("getting entity", "id", id, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if entity == nil {
		http.NotFound(w, r)
		return
	}

	s.render(w, "entity_detail", entityDetailData{
		CurrentPath: "/entity/" + id,
		Entity:      entity,
	})
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

func (s *Server) searchEntities(q, kind, lang string) ([]catalog.Entity, error) {
	entities, err := s.store.ListEntities(kind, 0)
	if err != nil {
		return nil, err
	}

	if q == "" && lang == "" {
		return entities, nil
	}

	var filtered []catalog.Entity
	for _, e := range entities {
		if lang != "" && e.Language != lang {
			continue
		}
		if q != "" && !matchesQuery(e, q) {
			continue
		}
		filtered = append(filtered, e)
	}

	return filtered, nil
}

func matchesQuery(e catalog.Entity, q string) bool {
	q = strings.ToLower(q)
	return strings.Contains(strings.ToLower(e.Name), q) ||
		strings.Contains(strings.ToLower(e.Description), q) ||
		strings.Contains(strings.ToLower(e.Owner), q) ||
		strings.Contains(strings.ToLower(string(e.Kind)), q)
}

func (s *Server) collectLanguages() []string {
	all, err := s.store.ListEntities("", 0)
	if err != nil {
		slog.Error("collecting languages", "error", err)
		return nil
	}

	seen := make(map[string]bool)
	for _, e := range all {
		if e.Language != "" {
			seen[e.Language] = true
		}
	}

	langs := make([]string, 0, len(seen))
	for l := range seen {
		langs = append(langs, l)
	}
	sort.Strings(langs)
	return langs
}

