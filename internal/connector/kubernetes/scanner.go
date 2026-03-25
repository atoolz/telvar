package kubernetes

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ahlert/telvar/internal/catalog"
	"github.com/ahlert/telvar/internal/store"
)

type Scanner struct {
	client *Client
	store  *store.Store
}

func NewScanner(client *Client, store *store.Store) *Scanner {
	return &Scanner{client: client, store: store}
}

func (s *Scanner) Run(ctx context.Context) error {
	deployments, err := s.client.ListDeployments(ctx)
	if err != nil {
		return fmt.Errorf("listing deployments: %w", err)
	}

	slog.Info("k8s deployments found", "count", len(deployments))

	merged := 0
	for _, d := range deployments {
		entity := DeploymentToEntity(d)

		existing, err := s.store.GetEntity(catalog.SlugFor(d.Name))
		if err != nil {
			slog.Warn("looking up entity for k8s merge, treating as new", "name", d.Name, "error", err)
			existing = nil
		}

		if existing != nil {
			if existing.Metadata == nil {
				existing.Metadata = make(map[string]string)
			}
			for k, v := range entity.Metadata {
				existing.Metadata[k] = v
			}
			for k, v := range entity.Tags {
				existing.Tags[k] = v
			}
			if existing.Owner == "" && entity.Owner != "" {
				existing.Owner = entity.Owner
			}
			if err := s.store.UpsertEntity(existing); err != nil {
				slog.Warn("merging k8s data", "entity", existing.ID, "error", err)
			}
			merged++
		} else {
			entity.ID = catalog.SlugFor(d.Name)
			if err := s.store.UpsertEntity(&entity); err != nil {
				slog.Warn("storing k8s entity", "name", d.Name, "error", err)
			}
		}
	}

	slog.Info("k8s scan complete", "deployments", len(deployments), "merged", merged)
	return nil
}
