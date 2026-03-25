# Contributing to Telvar

Thanks for your interest in contributing. This guide will get you from zero to a running local instance in under 10 minutes.

## Prerequisites

- **Go 1.25+** (check with `go version`)
- **Git**
- A GitHub personal access token (for testing discovery against real orgs)
- No CGO, no Node.js, no external database required

## Getting started

```bash
git clone https://github.com/atoolz/telvar.git
cd telvar
make build
./telvar config init
# Edit telvar.yaml with your GitHub org and token
./telvar discover
./telvar serve
```

Open `http://localhost:7007` to see the catalog.

## Development workflow

```bash
# Build
make build

# Run tests
make test

# Run with live reload (rebuild + restart)
make build && ./telvar serve --config telvar.yaml --db telvar.db

# Run a single package's tests
go test ./internal/scorecard/... -v

# Run with race detector
go test ./... -race -count=1
```

## Project structure

```
cmd/telvar/          CLI entrypoint (cobra commands)
assets/              Embedded templates and static files
  templates/         Go html/template files
  static/            CSS, htmx.min.js
internal/
  catalog/           Entity model, inference engine, dependency extraction
  config/            YAML config loader
  connector/
    github/          GitHub org scanner (repos, files, tree)
    kubernetes/      K8s deployment scanner
    pagerduty/       On-call schedule fetcher
  docs/              Markdown renderer (goldmark)
  scheduler/         Background discovery scheduler
  scorecard/         Rule evaluator and scorer
  store/             SQLite repository layer
  vuln/              CVE checker via osv.dev
  web/               HTTP server, handlers, htmx routes
```

## Adding a new connector

1. Create `internal/connector/yourservice/client.go` with API client
2. Create `internal/connector/yourservice/scanner.go` implementing:
   ```go
   func (s *Scanner) Run(ctx context.Context) error
   ```
3. Add config struct to `internal/config/config.go`
4. Wire into `cmd/telvar/main.go` serve command (follow the K8s/PagerDuty pattern)
5. Add tests with `net/http/httptest` mock server

## Adding a new scorecard check

1. Add the function name to `evalCall` in `internal/scorecard/evaluator.go`
2. The function receives `(expr string, files catalog.FileChecker, entity *catalog.Entity)` and returns `(bool, string, error)`
3. Add test cases to `internal/scorecard/evaluator_test.go`
4. Document the function in the config example

## Code conventions

- **Error handling**: wrap with `fmt.Errorf("context: %w", err)`
- **No global state**: dependency injection via constructors
- **Tests**: next to source files (`_test.go`), use `":memory:"` for store tests
- **Logging**: `log/slog` with structured fields
- **Templates**: server-rendered HTML + htmx, no React/JS framework
- **Database**: SQLite with WAL mode, `SetMaxOpenConns(1)`

## Pull request process

1. Fork the repo, create a feature branch from `main`
2. Write tests for new functionality
3. Ensure `go vet ./...` and `go test ./... -race` pass
4. Commit with a descriptive message (see existing commits for style)
5. Open a PR against `main`

## What we don't accept

- React/JavaScript frontend changes (htmx is a deliberate choice)
- Plugin systems or extension mechanisms
- Features that require external databases
- SSO/SAML implementations (use a reverse proxy)
- AI-powered features in the core product

## License

By contributing, you agree that your contributions will be licensed under the AGPLv3 license.
