package pagerduty

import (
	"context"
	"log/slog"

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
	oncalls, err := s.client.ListOnCallNow(ctx)
	if err != nil {
		return err
	}

	slog.Info("pagerduty oncalls fetched", "count", len(oncalls))

	// TODO: paginate for large catalogs
	entities, err := s.store.ListEntities("", 0)
	if err != nil {
		return err
	}

	matches := MatchOnCallToEntities(oncalls, entities)
	slog.Info("pagerduty matches", "matched", len(matches), "total_entities", len(entities))

	for id, oc := range matches {
		entity, err := s.store.GetEntity(id)
		if err != nil || entity == nil {
			continue
		}

		if entity.Metadata == nil {
			entity.Metadata = make(map[string]string)
		}

		entity.Metadata["pagerduty.oncall_name"] = oc.UserName
		entity.Metadata["pagerduty.oncall_email"] = oc.UserEmail
		entity.Metadata["pagerduty.schedule"] = oc.ScheduleName
		entity.Metadata["pagerduty.escalation_policy"] = oc.EscalationPolicy

		if err := s.store.UpsertEntity(entity); err != nil {
			slog.Warn("failed to update entity with oncall data", "entity", id, "error", err)
		}
	}

	return nil
}
