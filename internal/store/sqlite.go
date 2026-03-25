package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ahlert/telvar/internal/catalog"
	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	db.SetMaxOpenConns(1)

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("setting WAL mode: %w", err)
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func migrate(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS entities (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		kind TEXT NOT NULL,
		description TEXT DEFAULT '',
		owner TEXT DEFAULT '',
		team TEXT DEFAULT '',
		language TEXT DEFAULT '',
		framework TEXT DEFAULT '',
		repo_url TEXT DEFAULT '',
		dependencies TEXT DEFAULT '[]',
		tags TEXT DEFAULT '{}',
		metadata TEXT DEFAULT '{}',
		score_json TEXT DEFAULT NULL,
		discovered_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_entities_kind ON entities(kind);
	CREATE INDEX IF NOT EXISTS idx_entities_owner ON entities(owner);
	CREATE INDEX IF NOT EXISTS idx_entities_language ON entities(language);

	CREATE VIRTUAL TABLE IF NOT EXISTS entities_fts USING fts5(
		name,
		description,
		owner,
		language,
		content='entities',
		content_rowid='rowid'
	);

	CREATE TRIGGER IF NOT EXISTS entities_ai AFTER INSERT ON entities BEGIN
		INSERT INTO entities_fts(rowid, name, description, owner, language)
		VALUES (new.rowid, new.name, new.description, new.owner, new.language);
	END;

	CREATE TRIGGER IF NOT EXISTS entities_au AFTER UPDATE ON entities BEGIN
		INSERT INTO entities_fts(entities_fts, rowid, name, description, owner, language)
		VALUES ('delete', old.rowid, old.name, old.description, old.owner, old.language);
		INSERT INTO entities_fts(rowid, name, description, owner, language)
		VALUES (new.rowid, new.name, new.description, new.owner, new.language);
	END;

	CREATE TABLE IF NOT EXISTS discovery_runs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		source TEXT NOT NULL,
		started_at DATETIME NOT NULL,
		finished_at DATETIME,
		entities_found INTEGER DEFAULT 0,
		status TEXT DEFAULT 'running'
	);
	`
	_, err := db.Exec(schema)
	return err
}

func (s *Store) UpsertEntity(e *catalog.Entity) error {
	deps, err := json.Marshal(e.Dependencies)
	if err != nil {
		return fmt.Errorf("marshaling dependencies for %s: %w", e.ID, err)
	}
	tags, err := json.Marshal(e.Tags)
	if err != nil {
		return fmt.Errorf("marshaling tags for %s: %w", e.ID, err)
	}
	meta, err := json.Marshal(e.Metadata)
	if err != nil {
		return fmt.Errorf("marshaling metadata for %s: %w", e.ID, err)
	}

	var scoreJSON *string
	if e.Score != nil {
		data, err := json.Marshal(e.Score)
		if err != nil {
			return fmt.Errorf("marshaling score for %s: %w", e.ID, err)
		}
		str := string(data)
		scoreJSON = &str
	}

	now := time.Now().UTC()
	discoveredAt := e.DiscoveredAt
	if discoveredAt.IsZero() {
		discoveredAt = now
	}
	updatedAt := now

	_, err = s.db.Exec(`
		INSERT INTO entities (id, name, kind, description, owner, team, language, framework, repo_url, dependencies, tags, metadata, score_json, discovered_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name, kind=excluded.kind, description=excluded.description,
			owner=excluded.owner, team=excluded.team, language=excluded.language,
			framework=excluded.framework, repo_url=excluded.repo_url,
			dependencies=excluded.dependencies, tags=excluded.tags,
			metadata=excluded.metadata, score_json=excluded.score_json,
			updated_at=excluded.updated_at
	`, e.ID, e.Name, e.Kind, e.Description, e.Owner, e.Team, e.Language, e.Framework, e.RepoURL,
		string(deps), string(tags), string(meta), scoreJSON, discoveredAt, updatedAt)

	return err
}

func (s *Store) ListEntities(kind string, limit int) (_ []catalog.Entity, err error) {
	query := "SELECT id, name, kind, description, owner, team, language, framework, repo_url, dependencies, tags, metadata, score_json, discovered_at, updated_at FROM entities"
	var args []any

	if kind != "" {
		query += " WHERE kind = ?"
		args = append(args, kind)
	}
	query += " ORDER BY name ASC"
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	var entities []catalog.Entity
	for rows.Next() {
		var e catalog.Entity
		var deps, tags, meta string
		var scoreJSON *string

		if scanErr := rows.Scan(&e.ID, &e.Name, &e.Kind, &e.Description, &e.Owner, &e.Team, &e.Language, &e.Framework, &e.RepoURL, &deps, &tags, &meta, &scoreJSON, &e.DiscoveredAt, &e.UpdatedAt); scanErr != nil {
			err = fmt.Errorf("scanning entity: %w", scanErr)
			return nil, err
		}

		if parseErr := json.Unmarshal([]byte(deps), &e.Dependencies); parseErr != nil {
			err = fmt.Errorf("parsing dependencies for %s: %w", e.ID, parseErr)
			return nil, err
		}
		if parseErr := json.Unmarshal([]byte(tags), &e.Tags); parseErr != nil {
			err = fmt.Errorf("parsing tags for %s: %w", e.ID, parseErr)
			return nil, err
		}
		if parseErr := json.Unmarshal([]byte(meta), &e.Metadata); parseErr != nil {
			err = fmt.Errorf("parsing metadata for %s: %w", e.ID, parseErr)
			return nil, err
		}
		if scoreJSON != nil {
			var score catalog.ScoreResult
			if parseErr := json.Unmarshal([]byte(*scoreJSON), &score); parseErr != nil {
				err = fmt.Errorf("parsing score for %s: %w", e.ID, parseErr)
				return nil, err
			}
			e.Score = &score
		}

		entities = append(entities, e)
	}

	return entities, rows.Err()
}

func (s *Store) GetEntity(id string) (*catalog.Entity, error) {
	var e catalog.Entity
	var deps, tags, meta string
	var scoreJSON *string

	err := s.db.QueryRow(`
		SELECT id, name, kind, description, owner, team, language, framework, repo_url, dependencies, tags, metadata, score_json, discovered_at, updated_at
		FROM entities WHERE id = ?
	`, id).Scan(&e.ID, &e.Name, &e.Kind, &e.Description, &e.Owner, &e.Team, &e.Language, &e.Framework, &e.RepoURL, &deps, &tags, &meta, &scoreJSON, &e.DiscoveredAt, &e.UpdatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(deps), &e.Dependencies); err != nil {
		return nil, fmt.Errorf("parsing dependencies for %s: %w", e.ID, err)
	}
	if err := json.Unmarshal([]byte(tags), &e.Tags); err != nil {
		return nil, fmt.Errorf("parsing tags for %s: %w", e.ID, err)
	}
	if err := json.Unmarshal([]byte(meta), &e.Metadata); err != nil {
		return nil, fmt.Errorf("parsing metadata for %s: %w", e.ID, err)
	}
	if scoreJSON != nil {
		var score catalog.ScoreResult
		if err := json.Unmarshal([]byte(*scoreJSON), &score); err != nil {
			return nil, fmt.Errorf("parsing score for %s: %w", e.ID, err)
		}
		e.Score = &score
	}

	return &e, nil
}

func (s *Store) SearchEntities(query string, limit int) (_ []catalog.Entity, err error) {
	if query == "" {
		return s.ListEntities("", limit)
	}

	escaped := strings.ReplaceAll(query, `"`, `""`)
	ftsQuery := `"` + escaped + `"*`

	sqlQuery := `
		SELECT e.id, e.name, e.kind, e.description, e.owner, e.team, e.language, e.framework, e.repo_url, e.dependencies, e.tags, e.metadata, e.score_json, e.discovered_at, e.updated_at
		FROM entities e
		JOIN entities_fts f ON e.rowid = f.rowid
		WHERE entities_fts MATCH ?
		ORDER BY rank
	`
	var args []any
	args = append(args, ftsQuery)

	if limit > 0 {
		sqlQuery += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := s.db.Query(sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("searching entities: %w", err)
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	var entities []catalog.Entity
	for rows.Next() {
		var e catalog.Entity
		var deps, tags, meta string
		var scoreJSON *string

		if scanErr := rows.Scan(&e.ID, &e.Name, &e.Kind, &e.Description, &e.Owner, &e.Team, &e.Language, &e.Framework, &e.RepoURL, &deps, &tags, &meta, &scoreJSON, &e.DiscoveredAt, &e.UpdatedAt); scanErr != nil {
			return nil, fmt.Errorf("scanning search result: %w", scanErr)
		}

		if parseErr := json.Unmarshal([]byte(deps), &e.Dependencies); parseErr != nil {
			return nil, fmt.Errorf("parsing dependencies for %s: %w", e.ID, parseErr)
		}
		if parseErr := json.Unmarshal([]byte(tags), &e.Tags); parseErr != nil {
			return nil, fmt.Errorf("parsing tags for %s: %w", e.ID, parseErr)
		}
		if parseErr := json.Unmarshal([]byte(meta), &e.Metadata); parseErr != nil {
			return nil, fmt.Errorf("parsing metadata for %s: %w", e.ID, parseErr)
		}
		if scoreJSON != nil {
			var score catalog.ScoreResult
			if parseErr := json.Unmarshal([]byte(*scoreJSON), &score); parseErr != nil {
				return nil, fmt.Errorf("parsing score for %s: %w", e.ID, parseErr)
			}
			e.Score = &score
		}

		entities = append(entities, e)
	}

	return entities, rows.Err()
}

type TeamSummary struct {
	Owner       string
	EntityCount int
	AvgScore    int
}

func (s *Store) ListTeams() (_ []TeamSummary, err error) {
	rows, err := s.db.Query(`
		SELECT owner, COUNT(*) as cnt,
			COALESCE(AVG(CASE WHEN score_json IS NOT NULL
				THEN json_extract(score_json, '$.percent')
				ELSE NULL END), 0) as avg_score
		FROM entities
		WHERE owner != ''
		GROUP BY owner
		ORDER BY cnt DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("listing teams: %w", err)
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	var teams []TeamSummary
	for rows.Next() {
		var t TeamSummary
		if err := rows.Scan(&t.Owner, &t.EntityCount, &t.AvgScore); err != nil {
			return nil, fmt.Errorf("scanning team: %w", err)
		}
		teams = append(teams, t)
	}
	return teams, rows.Err()
}

func (s *Store) ListEntitiesByOwner(owner string) ([]catalog.Entity, error) {
	return s.listEntitiesWhere("owner = ?", owner)
}

// listEntitiesWhere queries entities with a WHERE clause. The `where` parameter
// MUST be a hardcoded parameterized fragment (e.g., "owner = ?"). Never pass
// user input directly into `where`; use `args` for user-provided values.
func (s *Store) listEntitiesWhere(where string, args ...any) (_ []catalog.Entity, err error) {
	query := "SELECT id, name, kind, description, owner, team, language, framework, repo_url, dependencies, tags, metadata, score_json, discovered_at, updated_at FROM entities WHERE " + where + " ORDER BY name ASC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	var entities []catalog.Entity
	for rows.Next() {
		var e catalog.Entity
		var deps, tags, meta string
		var scoreJSON *string

		if err := rows.Scan(&e.ID, &e.Name, &e.Kind, &e.Description, &e.Owner, &e.Team, &e.Language, &e.Framework, &e.RepoURL, &deps, &tags, &meta, &scoreJSON, &e.DiscoveredAt, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning entity: %w", err)
		}

		if err := json.Unmarshal([]byte(deps), &e.Dependencies); err != nil {
			return nil, fmt.Errorf("parsing dependencies for %s: %w", e.ID, err)
		}
		if err := json.Unmarshal([]byte(tags), &e.Tags); err != nil {
			return nil, fmt.Errorf("parsing tags for %s: %w", e.ID, err)
		}
		if err := json.Unmarshal([]byte(meta), &e.Metadata); err != nil {
			return nil, fmt.Errorf("parsing metadata for %s: %w", e.ID, err)
		}
		if scoreJSON != nil {
			var score catalog.ScoreResult
			if err := json.Unmarshal([]byte(*scoreJSON), &score); err != nil {
				return nil, fmt.Errorf("parsing score for %s: %w", e.ID, err)
			}
			e.Score = &score
		}

		entities = append(entities, e)
	}
	return entities, rows.Err()
}

func (s *Store) ListLanguages() ([]string, error) {
	rows, err := s.db.Query("SELECT DISTINCT language FROM entities WHERE language != '' ORDER BY language")
	if err != nil {
		return nil, fmt.Errorf("listing languages: %w", err)
	}
	defer rows.Close()

	var langs []string
	for rows.Next() {
		var lang string
		if err := rows.Scan(&lang); err != nil {
			return nil, fmt.Errorf("scanning language: %w", err)
		}
		langs = append(langs, lang)
	}
	return langs, rows.Err()
}

func (s *Store) CountEntities() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM entities").Scan(&count)
	return count, err
}

func (s *Store) StartDiscoveryRun(source string) (int64, error) {
	result, err := s.db.Exec(
		"INSERT INTO discovery_runs (source, started_at, status) VALUES (?, ?, ?)",
		source, time.Now().UTC(), "running",
	)
	if err != nil {
		return 0, fmt.Errorf("inserting discovery run: %w", err)
	}
	return result.LastInsertId()
}

func (s *Store) FinishDiscoveryRun(id int64, entitiesFound int, status string) error {
	result, err := s.db.Exec(
		"UPDATE discovery_runs SET finished_at = ?, entities_found = ?, status = ? WHERE id = ?",
		time.Now().UTC(), entitiesFound, status, id,
	)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return fmt.Errorf("discovery run %d not found", id)
	}
	return nil
}

const (
	RunStatusRunning   = "running"
	RunStatusOK        = "ok"
	RunStatusFailed    = "failed"
	RunStatusCancelled = "cancelled"
)

type DiscoveryRun struct {
	ID            int64
	Source        string
	StartedAt     time.Time
	FinishedAt    *time.Time
	EntitiesFound int
	Status        string
}

func (s *Store) LatestDiscoveryRun() (*DiscoveryRun, error) {
	var r DiscoveryRun
	var finishedAt sql.NullTime

	err := s.db.QueryRow(`
		SELECT id, source, started_at, finished_at, entities_found, status
		FROM discovery_runs ORDER BY id DESC LIMIT 1
	`).Scan(&r.ID, &r.Source, &r.StartedAt, &finishedAt, &r.EntitiesFound, &r.Status)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if finishedAt.Valid {
		r.FinishedAt = &finishedAt.Time
	}
	return &r, nil
}
