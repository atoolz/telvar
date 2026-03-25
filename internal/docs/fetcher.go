package docs

import (
	"context"
	"fmt"
	"html/template"

	"github.com/ahlert/telvar/internal/catalog"
)

type GitHubClient interface {
	GetFileContent(ctx context.Context, repo, path string) ([]byte, error)
}

type Fetcher struct {
	client GitHubClient
}

func NewFetcher(client GitHubClient) *Fetcher {
	return &Fetcher{client: client}
}

func (f *Fetcher) FetchAndRender(ctx context.Context, entity *catalog.Entity) (raw string, rendered template.HTML, err error) {
	if entity == nil {
		return "", "", fmt.Errorf("entity is nil")
	}

	repo := entity.Name

	readmeFiles := []string{"README.md", "readme.md", "README.rst", "README.txt", "README"}
	var content []byte

	for _, path := range readmeFiles {
		data, fetchErr := f.client.GetFileContent(ctx, repo, path)
		if fetchErr == nil {
			content = data
			break
		}
	}

	if content == nil {
		return "", "", fmt.Errorf("no README found for %s", repo)
	}

	rawStr := string(content)
	html, renderErr := RenderMarkdown(content)
	if renderErr != nil {
		return rawStr, "", fmt.Errorf("rendering markdown for %s: %w", repo, renderErr)
	}

	return rawStr, html, nil
}
