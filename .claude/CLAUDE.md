# Telvar

## What is this

Telvar is a zero-authoring developer portal. A single Go binary that auto-discovers services from GitHub orgs and Kubernetes clusters, builds a service catalog, runs scorecards, and renders docs. No YAML to write, no React plugins to maintain, no PostgreSQL to provision.

## Philosophy

"Your infrastructure already describes itself. We just read it."

- **Discovery over declaration**: Connect GitHub + K8s, Telvar crawls and infers. No catalog-info.yaml.
- **Config, not code**: Every customization is YAML/TOML, not TypeScript plugins.
- **Single binary, zero deps**: Go binary + SQLite. No Node.js, no PostgreSQL, no build pipeline.
- **Opinionated defaults**: One good version of each feature instead of 100 plugins.

## Tech Stack

- **Language**: Go 1.24+
- **Database**: SQLite (via modernc.org/sqlite, pure Go, no CGO)
- **Frontend**: Server-rendered HTML + htmx (no React, no Node.js, no build step)
- **Templates**: Go html/template
- **CSS**: Minimal, no framework (or Pico CSS)
- **Markdown**: goldmark

## Architecture

```
cmd/telvar/           → CLI entrypoint (cobra)
internal/
  catalog/            → Entity model (Component, API, Resource), inference engine
  connector/
    github/           → GitHub org scanner (repos, CODEOWNERS, deps, CI)
    kubernetes/       → K8s cluster scanner (deployments, pods, services)
    pagerduty/        → On-call schedule fetcher
  scorecard/          → Rule engine (YAML-defined rules)
  vuln/               → OSV/CVE checker (parse lockfiles, check osv.dev)
  docs/               → Markdown fetcher/renderer (goldmark)
  web/                → HTTP handlers, htmx endpoints
  store/              → SQLite repository layer
  config/             → Config file parsing
templates/            → Go HTML templates
static/               → CSS, htmx.min.js
```

## Key Design Decisions

- htmx for frontend interactions, NOT React. This is a deliberate choice that Backstage cannot copy.
- SQLite as default database. Backup = copy one file. No migration headaches.
- GitHub connector first, GitLab second. GitHub is the primary target market.
- Scorecards defined in YAML, evaluated server-side. No Rego, no custom DSL.
- Entity inference from existing files (Dockerfile, package.json, go.mod, helm/Chart.yaml).

## Coding Conventions

- Go standard project layout
- Error handling: wrap with fmt.Errorf("context: %w", err)
- No global state. Dependency injection via constructor functions.
- Tests next to source files (_test.go)
- Use slog for structured logging
- CLI via cobra

## MVP Scope (what to build first)

### Month 1: The Crawler
- GitHub org connector (OAuth, scan repos)
- Entity inference engine (detect what each repo is)
- SQLite storage
- Web UI: service list with search/filter

### Month 2: The Value Layer
- Scorecards (YAML rules)
- Dependency tracking + CVE checking
- Docs viewer (render repo markdown)

### Month 3: The Hook
- Kubernetes integration
- PagerDuty integration
- Team pages
- Search

## What we DON'T build
- Software templates/scaffolding
- Custom plugin system
- API catalog (OpenAPI rendering)
- Workflow automation
- Self-service actions
- SSO/SAML (use a reverse proxy)
