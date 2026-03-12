package agents

import (
	"github.com/aegis-wilwatikta/ai-reviewer/internal/domain"
)

type ImpactTier string

const (
	TierBreaking ImpactTier = "BREAKING"
	TierLogic    ImpactTier = "LOGIC"
	TierLeaf     ImpactTier = "LEAF"
)

type AggregatedImpact struct {
	Tier         ImpactTier
	AffectedNode domain.CodeNode
	Reason       string
}

func AggregateImpacts(reports []*domain.ImpactReport) map[ImpactTier][]AggregatedImpact {
	aggregated := make(map[ImpactTier][]AggregatedImpact)

	seen := make(map[string]bool)

	for _, report := range reports {
		for _, affected := range report.AffectedNodes {
			if seen[affected.Node.ID] {
				continue
			}
			seen[affected.Node.ID] = true

			tier := TierLeaf
			reason := "Leaf node affected"

			if affected.Node.Kind == domain.KindInterface || affected.Node.Kind == domain.KindStruct {
				if affected.Relation == domain.RelImplements || affected.Relation == domain.RelUses {
					tier = TierBreaking
					reason = "Potential breaking change to interface/struct"
				}
			} else if affected.Node.Kind == domain.KindFunction || affected.Node.Kind == domain.KindMethod {
				if affected.Relation == domain.RelCalls {
					tier = TierLogic
					reason = "Downstream logic flow affected"
				}
			}

			if affected.Depth > 1 && tier != TierBreaking {
				tier = TierLeaf // Lower tier for distant impacts
			}

			aggregated[tier] = append(aggregated[tier], AggregatedImpact{
				Tier:         tier,
				AffectedNode: affected.Node,
				Reason:       reason,
			})
		}
	}

	return aggregated
}

func CalculateHealthScore(aggregated map[ImpactTier][]AggregatedImpact) int {
	score := 100
	score -= len(aggregated[TierBreaking]) * 10
	score -= len(aggregated[TierLogic]) * 5
	score -= len(aggregated[TierLeaf]) * 2

	if score < 0 {
		return 0
	}
	return score
}
