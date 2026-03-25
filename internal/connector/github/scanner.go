package github

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ahlert/telvar/internal/catalog"
	"github.com/ahlert/telvar/internal/config"
	"github.com/ahlert/telvar/internal/scorecard"
	"github.com/ahlert/telvar/internal/store"
)

type Scanner struct {
	client   *Client
	store    *store.Store
	cfg      *config.DiscoveryConfig
	scorer   *scorecard.Runner
}

func NewScanner(client *Client, store *store.Store, cfg *config.DiscoveryConfig, scorer *scorecard.Runner) *Scanner {
	return &Scanner{
		client: client,
		store:  store,
		cfg:    cfg,
		scorer: scorer,
	}
}

func (s *Scanner) Run(ctx context.Context) error {
	runID, err := s.store.StartDiscoveryRun("github")
	if err != nil {
		return fmt.Errorf("starting discovery run: %w", err)
	}

	slog.Info("discovery started", "source", "github", "org", s.client.Org())

	repos, err := s.client.ListRepos(ctx)
	if err != nil {
		if finErr := s.store.FinishDiscoveryRun(runID, 0, store.RunStatusFailed); finErr != nil {
			slog.Warn("failed to mark discovery run as failed", "error", finErr)
		}
		return fmt.Errorf("listing repos: %w", err)
	}

	filtered := s.filterRepos(repos)
	slog.Info("repos found", "total", len(repos), "after_filter", len(filtered))

	var (
		count int
		mu    sync.Mutex
		sem   = make(chan struct{}, 10)
		wg    sync.WaitGroup
	)

	cancelled := false
dispatch:
	for _, repo := range filtered {
		select {
		case <-ctx.Done():
			mu.Lock()
			cancelled = true
			mu.Unlock()
			break dispatch
		case sem <- struct{}{}:
		}

		wg.Add(1)
		go func(r Repo) {
			defer wg.Done()
			defer func() { <-sem }()

			if err := s.processRepo(ctx, r); err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					mu.Lock()
					cancelled = true
					mu.Unlock()
				}
				slog.Warn("skipping repo",
					"repo", r.Name,
					"error", err,
				)
				return
			}

			mu.Lock()
			count++
			mu.Unlock()
		}(repo)
	}

	wg.Wait()

	status := store.RunStatusOK
	if cancelled {
		status = store.RunStatusCancelled
	}
	if err := s.store.FinishDiscoveryRun(runID, count, status); err != nil {
		return fmt.Errorf("finishing discovery run: %w", err)
	}

	slog.Info("discovery finished",
		"source", "github",
		"entities", count,
	)

	return nil
}

func (s *Scanner) processRepo(ctx context.Context, repo Repo) error {
	sha, err := s.client.GetTreeSHA(ctx, repo.Name, repo.DefaultBranch)
	if err != nil {
		return fmt.Errorf("getting tree SHA: %w", err)
	}

	paths, err := s.client.GetTree(ctx, repo.Name, sha)
	if err != nil {
		return fmt.Errorf("getting tree: %w", err)
	}

	fc := NewRepoFileChecker(ctx, s.client, repo.Name, paths)
	entity := catalog.InferEntity(repo.Name, repo.HTMLURL, fc)

	entity.Description = repo.Description
	if len(repo.Topics) > 0 {
		entity.Tags["topics"] = strings.Join(repo.Topics, ",")
	}
	entity.Metadata["github_stars"] = strconv.Itoa(repo.StarCount)
	entity.Metadata["github_pushed_at"] = repo.PushedAt.Format(time.RFC3339)
	entity.Metadata["default_branch"] = repo.DefaultBranch

	if s.scorer != nil {
		entity.Score = s.scorer.Score(&entity, fc)
	}

	if err := s.store.UpsertEntity(&entity); err != nil {
		return fmt.Errorf("storing entity %s: %w", entity.Name, err)
	}

	return nil
}

func (s *Scanner) filterRepos(repos []Repo) []Repo {
	var filtered []Repo
	for _, r := range repos {
		if s.cfg.IgnoreArchived && r.Archived {
			continue
		}
		if s.cfg.IgnoreForks && r.Fork {
			continue
		}
		filtered = append(filtered, r)
	}
	return filtered
}
