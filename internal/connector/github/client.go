package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ahlert/telvar/internal/config"
)

type Client struct {
	http      *http.Client
	baseURL   string
	org       string
	token     string
	rateMu    sync.Mutex
	rateReset time.Time
}

func NewClient(cfg *config.GitHubConfig) *Client {
	return &Client{
		http:    &http.Client{Timeout: 30 * time.Second},
		baseURL: "https://api.github.com",
		org:     cfg.Org,
		token:   cfg.Token,
	}
}

func (c *Client) Org() string {
	return c.org
}

func (c *Client) ListRepos(ctx context.Context) ([]Repo, error) {
	var all []Repo
	page := 1

	for {
		url := fmt.Sprintf("%s/orgs/%s/repos?per_page=100&page=%d", c.baseURL, c.org, page)

		body, headers, err := c.get(ctx, url)
		if err != nil {
			return nil, fmt.Errorf("listing repos page %d: %w", page, err)
		}

		var repos []Repo
		if err := json.Unmarshal(body, &repos); err != nil {
			return nil, fmt.Errorf("parsing repos page %d: %w", page, err)
		}

		all = append(all, repos...)

		if !hasNextPage(headers) {
			break
		}
		page++
	}

	return all, nil
}

func (c *Client) GetTreeSHA(ctx context.Context, repo, branch string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/branches/%s", c.baseURL, c.org, repo, branch)

	body, _, err := c.get(ctx, url)
	if err != nil {
		return "", fmt.Errorf("getting branch %s/%s: %w", repo, branch, err)
	}

	var br branchResponse
	if err := json.Unmarshal(body, &br); err != nil {
		return "", fmt.Errorf("parsing branch response: %w", err)
	}

	if br.Commit.SHA == "" {
		return "", fmt.Errorf("empty commit SHA for branch %s/%s", repo, branch)
	}

	return br.Commit.SHA, nil
}

func (c *Client) GetTree(ctx context.Context, repo, sha string) ([]string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/git/trees/%s?recursive=1", c.baseURL, c.org, repo, sha)

	body, _, err := c.get(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("getting tree %s/%s: %w", repo, sha, err)
	}

	var tree treeResponse
	if err := json.Unmarshal(body, &tree); err != nil {
		return nil, fmt.Errorf("parsing tree response: %w", err)
	}

	if tree.Truncated {
		slog.Warn("tree response truncated, some files may be missing",
			"repo", repo,
			"entries", len(tree.Tree),
		)
	}

	paths := make([]string, 0, len(tree.Tree))
	for _, node := range tree.Tree {
		if node.Type == "blob" {
			paths = append(paths, node.Path)
		}
	}

	return paths, nil
}

func (c *Client) GetFileContent(ctx context.Context, repo, filePath string) ([]byte, error) {
	encodedPath := encodePath(filePath)
	reqURL := fmt.Sprintf("%s/repos/%s/%s/contents/%s", c.baseURL, c.org, repo, encodedPath)

	body, _, err := c.get(ctx, reqURL)
	if err != nil {
		return nil, fmt.Errorf("getting file %s/%s: %w", repo, filePath, err)
	}

	var fc fileContentResponse
	if err := json.Unmarshal(body, &fc); err != nil {
		return nil, fmt.Errorf("parsing file content response: %w", err)
	}

	if fc.Encoding != "base64" {
		return nil, fmt.Errorf("unexpected encoding %q for %s/%s", fc.Encoding, repo, filePath)
	}

	clean := strings.ReplaceAll(fc.Content, "\n", "")
	decoded, err := base64.StdEncoding.DecodeString(clean)
	if err != nil {
		return nil, fmt.Errorf("decoding base64 for %s/%s: %w", repo, filePath, err)
	}

	return decoded, nil
}

func (c *Client) get(ctx context.Context, url string) ([]byte, http.Header, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return nil, nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, resp.Header, fmt.Errorf("not found: %s", url)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.Header, fmt.Errorf("unexpected status %d for %s: %s", resp.StatusCode, url, string(body))
	}

	c.handleRateLimit(ctx, resp.Header)

	return body, resp.Header, nil
}

func (c *Client) handleRateLimit(ctx context.Context, h http.Header) {
	remaining := h.Get("X-RateLimit-Remaining")
	if remaining == "" {
		return
	}

	rem, err := strconv.Atoi(remaining)
	if err != nil {
		return
	}

	if rem >= 10 {
		return
	}

	resetStr := h.Get("X-RateLimit-Reset")
	resetUnix, err := strconv.ParseInt(resetStr, 10, 64)
	if err != nil {
		return
	}

	resetTime := time.Unix(resetUnix, 0)

	c.rateMu.Lock()
	if !c.rateReset.IsZero() && time.Now().Before(c.rateReset) {
		c.rateMu.Unlock()
		wait := time.Until(c.rateReset)
		select {
		case <-time.After(wait):
		case <-ctx.Done():
		}
		return
	}

	wait := time.Until(resetTime)
	if wait <= 0 || wait > 15*time.Minute {
		c.rateMu.Unlock()
		return
	}

	c.rateReset = resetTime.Add(time.Second)
	c.rateMu.Unlock()

	slog.Warn("GitHub rate limit low, sleeping",
		"remaining", rem,
		"reset_in", wait.Round(time.Second),
	)

	select {
	case <-time.After(wait + time.Second):
	case <-ctx.Done():
	}
}

func hasNextPage(h http.Header) bool {
	link := h.Get("Link")
	if link == "" {
		return false
	}
	return strings.Contains(link, `rel="next"`)
}

func encodePath(p string) string {
	segments := strings.Split(p, "/")
	for i, seg := range segments {
		segments[i] = url.PathEscape(seg)
	}
	return strings.Join(segments, "/")
}
