package scorecard

import (
	"github.com/ahlert/telvar/internal/catalog"
	"github.com/ahlert/telvar/internal/config"
)

type Runner struct {
	scorecards []config.Scorecard
}

// NewRunner creates a scorecard runner. Currently evaluates only the first
// scorecard in the list. Multi-scorecard support is planned for a future release.
func NewRunner(scorecards []config.Scorecard) *Runner {
	return &Runner{scorecards: scorecards}
}

func (r *Runner) Score(entity *catalog.Entity, files catalog.FileChecker) *catalog.ScoreResult {
	if entity == nil || len(r.scorecards) == 0 {
		return nil
	}

	sc := r.scorecards[0]
	if len(sc.Rules) == 0 {
		return nil
	}

	var (
		total    float64
		maxTotal float64
		rules    []catalog.RuleResult
	)

	for _, rule := range sc.Rules {
		result := EvaluateRule(rule, files, entity)
		rules = append(rules, result)
		maxTotal += float64(result.Weight)
		if result.Passed {
			total += float64(result.Weight)
		}
	}

	percent := 0
	if maxTotal > 0 {
		percent = int(total / maxTotal * 100)
	}

	return &catalog.ScoreResult{
		Total:    total,
		MaxTotal: maxTotal,
		Percent:  percent,
		Rules:    rules,
	}
}
