package github

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
)

type RepoFileChecker struct {
	client  *Client
	repo    string
	ctx     context.Context
	paths   map[string]bool
	cache   map[string][]byte
	cacheMu sync.Mutex
}

func NewRepoFileChecker(ctx context.Context, client *Client, repo string, paths []string) *RepoFileChecker {
	m := make(map[string]bool, len(paths))
	for _, p := range paths {
		m[p] = true
	}
	return &RepoFileChecker{
		client: client,
		repo:   repo,
		ctx:    ctx,
		paths:  m,
		cache:  make(map[string][]byte),
	}
}

func (r *RepoFileChecker) HasFile(path string) bool {
	return r.paths[path]
}

func (r *RepoFileChecker) HasFileGlob(pattern string) bool {
	for p := range r.paths {
		if matchGlob(pattern, p) {
			return true
		}
	}
	return false
}

// matchGlob supports single ** for recursive directory matching.
// Patterns with multiple ** segments are not supported and return false.
func matchGlob(pattern, path string) bool {
	if strings.Contains(pattern, "**") {
		if strings.Count(pattern, "**") > 1 {
			return false
		}
		parts := strings.SplitN(pattern, "**", 2)
		prefix := strings.TrimSuffix(parts[0], "/")
		suffix := strings.TrimPrefix(parts[1], "/")

		if prefix != "" && !strings.HasPrefix(path, prefix+"/") && path != prefix {
			return false
		}

		if suffix == "" {
			return true
		}

		remaining := path
		if prefix != "" {
			remaining = strings.TrimPrefix(path, prefix+"/")
		}

		segments := strings.Split(remaining, "/")
		for i := range segments {
			candidate := strings.Join(segments[i:], "/")
			if matched, _ := filepath.Match(suffix, candidate); matched {
				return true
			}
		}
		return false
	}

	matched, _ := filepath.Match(pattern, path)
	return matched
}

func (r *RepoFileChecker) ReadFile(path string) ([]byte, error) {
	if !r.paths[path] {
		return nil, fmt.Errorf("file not found: %s", path)
	}

	r.cacheMu.Lock()
	if data, ok := r.cache[path]; ok {
		r.cacheMu.Unlock()
		return data, nil
	}
	r.cacheMu.Unlock()

	data, err := r.client.GetFileContent(r.ctx, r.repo, path)
	if err != nil {
		return nil, fmt.Errorf("fetching %s from %s: %w", path, r.repo, err)
	}

	r.cacheMu.Lock()
	r.cache[path] = data
	r.cacheMu.Unlock()

	return data, nil
}
