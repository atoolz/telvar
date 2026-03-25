package web

import (
	"html/template"
	"log/slog"
	"net/http"
	"sort"

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

type entityDocsData struct {
	CurrentPath  string
	Entity       *catalog.Entity
	RenderedHTML template.HTML
	Error        string
}

func (s *Server) handleEntityDocs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	entity, err := s.store.GetEntity(id)
	if err != nil {
		slog.Error("getting entity for docs", "id", id, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if entity == nil {
		http.NotFound(w, r)
		return
	}

	data := entityDocsData{
		CurrentPath: "/entity/" + id + "/docs",
		Entity:      entity,
	}

	if s.docs != nil {
		_, rendered, fetchErr := s.docs.FetchAndRender(r.Context(), entity)
		if fetchErr != nil {
			data.Error = fetchErr.Error()
		} else {
			data.RenderedHTML = rendered
		}
	} else {
		data.Error = "Documentation fetcher not configured"
	}

	s.render(w, "entity_docs", data)
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

func (s *Server) searchEntities(q, kind, lang string) ([]catalog.Entity, error) {
	if q != "" {
		entities, err := s.store.SearchEntities(q, 0)
		if err != nil {
			return nil, err
		}

		if kind == "" && lang == "" {
			return entities, nil
		}

		var filtered []catalog.Entity
		for _, e := range entities {
			if kind != "" && string(e.Kind) != kind {
				continue
			}
			if lang != "" && e.Language != lang {
				continue
			}
			filtered = append(filtered, e)
		}
		return filtered, nil
	}

	entities, err := s.store.ListEntities(kind, 0)
	if err != nil {
		return nil, err
	}

	if lang == "" {
		return entities, nil
	}

	var filtered []catalog.Entity
	for _, e := range entities {
		if e.Language != lang {
			continue
		}
		filtered = append(filtered, e)
	}
	return filtered, nil
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

