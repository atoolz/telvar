package pagerduty

import (
	"strings"

	"github.com/ahlert/telvar/internal/catalog"
)

// MatchOnCallToEntities matches on-call entries to entities by normalized name.
// When multiple entries share the same schedule/policy key, the last one wins.
func MatchOnCallToEntities(oncalls []OnCall, entities []catalog.Entity) map[string]OnCall {
	matches := make(map[string]OnCall)

	scheduleMap := make(map[string]OnCall)
	for _, oc := range oncalls {
		key := normalize(oc.ScheduleName)
		if key != "" {
			scheduleMap[key] = oc
		}
		epKey := normalize(oc.EscalationPolicy)
		if epKey != "" {
			scheduleMap[epKey] = oc
		}
	}

	for _, e := range entities {
		nameKey := normalize(e.Name)
		ownerKey := normalize(e.Owner)

		if oc, ok := scheduleMap[nameKey]; ok {
			matches[e.ID] = oc
			continue
		}
		if ownerKey != "" {
			if oc, ok := scheduleMap[ownerKey]; ok {
				matches[e.ID] = oc
			}
		}
	}

	return matches
}

func normalize(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	return s
}
