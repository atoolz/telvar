# Telvar Architecture

## Overview

Telvar is a zero-authoring developer portal built as a single Go binary. It discovers services from external sources (GitHub, Kubernetes, PagerDuty), infers metadata, evaluates quality scorecards, and renders a web dashboard.

## Component Graph

```
                          telvar.yaml
                              |
                         cmd/telvar
                         (cobra CLI)
                              |
                    +---------+---------+
                    |                   |
               telvar serve        telvar discover
                    |                   |
              +-----+-----+       +----+----+
              |           |       |         |
          scheduler    web server  (one-shot scan)
          (ticker)    (net/http)        |
              |           |        same pipeline
              v           v             |
         +---------+  +---------+      |
         |         |  |         |      |
    connectors     |  | handlers|      |
         |         |  | (htmx)  |      |
         v         |  +---------+      |
   +-----+-----+  |       |           |
   |     |     |  |       v           |
 GitHub K8s  PD  |    templates       |
   |     |     |  |   (embedded)      |
   v     v     v  |                   |
   +-----+-----+ |                   |
         |        |                   |
    inference     |                   |
    engine        |                   |
         |        |                   |
    vuln checker  |                   |
    (osv.dev)     |                   |
         |        |                   |
    scorecard     |                   |
    evaluator     |                   |
         |        |                   |
         v        v                   v
      +-------------+          +-------------+
      |   SQLite    |          |   SQLite    |
      |  (store)    |          |  (store)    |
      +-------------+          +-------------+
           |
      telvar.db
```

## Data Pipeline

Every data source feeds into a single pipeline:

```
Connector -> Inference -> Vuln Check -> Scorecard -> Store -> Web
```

1. **Connector** fetches raw data from external sources (GitHub API, K8s API, PagerDuty API)
2. **Inference** determines entity type, language, framework, owner, dependencies from file patterns
3. **Vuln Check** queries osv.dev for known vulnerabilities in dependencies
4. **Scorecard** evaluates YAML-defined rules against the entity
5. **Store** persists to SQLite with upsert semantics (new entities insert, existing merge)
6. **Web** renders the catalog via server-side HTML + htmx

## Package Responsibilities

### `cmd/telvar`
CLI entrypoint. Constructs all dependencies, wires connectors into schedulers, starts the web server. No business logic here.

### `internal/catalog`
Core domain model. Defines `Entity`, `EntityKind`, `ScoreResult`. Contains the inference engine that detects language, framework, kind, and owner from file patterns. Extracts dependencies from lockfiles (go.mod, package.json, requirements.txt, Cargo.toml).

### `internal/config`
Loads `telvar.yaml`. Expands environment variables in secret fields only (token, api_key) after YAML parsing. Provides defaults.

### `internal/connector/github`
GitHub API client (net/http, no SDK). Lists repos with pagination, fetches file trees, reads file content. `RepoFileChecker` implements `catalog.FileChecker` for the inference engine. `Scanner` orchestrates concurrent discovery (10 goroutines, semaphore-bounded) with context cancellation support.

### `internal/connector/kubernetes`
K8s client (client-go). Lists Deployments across namespaces, maps to entities, merges metadata into existing GitHub-discovered entities by name slug matching.

### `internal/connector/pagerduty`
PagerDuty API client (net/http). Fetches current on-call schedules, matches to entities by normalized schedule/owner name, annotates entity metadata.

### `internal/docs`
Markdown renderer (goldmark with GFM tables, strikethrough, task lists). Fetches README from GitHub, renders to safe HTML. No Linkify extension (prevents javascript: URL XSS).

### `internal/scheduler`
Generic scheduler. Takes any `Scanner` (interface with `Run(ctx) error`), runs it immediately then on a configurable interval. Mutex prevents concurrent runs. Respects context cancellation.

### `internal/scorecard`
Rule evaluator with mini-DSL: `has_file()`, `cve_count()`, `owner_set()`, `language_is()`, `has_topic()`, logical `&&`/`||`, numeric `==`/`!=`/`>`/`<`. Runner evaluates all rules, computes weighted score percentage.

### `internal/store`
SQLite repository layer. WAL mode, single connection (`MaxOpenConns(1)`). FTS5 virtual table for full-text search with auto-sync triggers. Named returns with `rows.Close()` error capture throughout.

### `internal/vuln`
CVE checker via osv.dev batch API. Parses lockfiles by ecosystem (Go, npm, PyPI, crates.io). Classifies severity from CVSS scores (V4 > V3 > V2 priority). Annotates entity metadata with vulnerability counts.

### `internal/web`
HTTP server with htmx. Templates embedded via `//go:embed`. Renders into `bytes.Buffer` before writing to prevent partial responses on template errors. `safeURL` template function prevents javascript: URL injection.

## Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| Single binary (Go) | Zero ops, no runtime dependencies, deploy anywhere |
| SQLite + WAL | No external database, backup = copy one file |
| htmx (no React) | No JS build step, server-rendered, Backstage can't copy this |
| net/http (no GitHub SDK) | Keep binary lean (~20MB), avoid transitive dependency bloat |
| FTS5 for search | Built into SQLite, no external search service |
| No plugin system | Opinionated defaults over infinite configurability |
| AGPLv3 | Community improvements flow back, prevents proprietary SaaS forks |

## Security Model

- GitHub/PagerDuty tokens expanded from env vars only, never stored in config
- All HTML output auto-escaped via `html/template`
- Markdown rendered without raw HTML passthrough or Linkify
- FTS5 queries sanitized (double-quote escaped, phrase prefix matching)
- Path values sanitized in handlers (alphanumeric + `-_.` only)
- No authentication in MVP (use a reverse proxy like Authentik/Keycloak)
- SQLite file should be mode 0600
