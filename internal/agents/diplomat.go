package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/domain"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/provider"
)

type Diplomat struct {
	provider provider.AIProvider
	model    string
}

func NewDiplomat(p provider.AIProvider, model string) *Diplomat {
	return &Diplomat{provider: p, model: model}
}

func (d *Diplomat) FormatReview(ctx context.Context, rawReview string, aggregated map[ImpactTier][]AggregatedImpact, healthScore int) (*domain.ReviewResult, error) {
	systemPrompt := `You are "The Diplomat", a communication specialist.
Your goal is to translate raw technical findings into constructive, human-readable feedback in JSON format.
You will receive a raw review from "The Architect".
You MUST output a valid JSON matching this schema:
{
  "verdict": "APPROVE" | "COMMENT" | "REQUEST_CHANGES",
  "summary": "A high-level summary of the review",
  "reviews": [
    {
      "file": "path/to/file",
      "line": 123,
      "severity": "LOW" | "MEDIUM" | "HIGH" | "CRITICAL",
      "issue": "Description of the issue",
      "suggestion": "How to fix it"
    }
  ]
}

Ensure the tone is professional and helpful. If there are critical issues, the verdict MUST be REQUEST_CHANGES. If there are only minor suggestions, use COMMENT or APPROVE.`

	userPrompt := fmt.Sprintf("Raw Architect Review:\n%s", rawReview)
	if len(aggregated) > 0 {
		impactMD := "\n\n### 🔍 Impact Analysis (Graph RAG)\n"
		impactMD += fmt.Sprintf("#### Review Health Score: %d/100\n", healthScore)

		tiers := []ImpactTier{TierBreaking, TierLogic, TierLeaf}
		for _, tier := range tiers {
			items := aggregated[tier]
			if len(items) == 0 {
				continue
			}
			impactMD += fmt.Sprintf("\n**Tier: %s**\n", tier)
			impactMD += "| Component | File Path | Reason |\n| :--- | :--- | :--- |\n"
			for _, item := range items {
				impactMD += fmt.Sprintf("| `%s` | `%s` | %s |\n", item.AffectedNode.Name, item.AffectedNode.Path, item.Reason)
			}
		}
		userPrompt += fmt.Sprintf("\n\nInclude this Impact Analysis in your summary:\n%s", impactMD)
	}

	response, err := d.provider.SendMessage(ctx, systemPrompt, userPrompt, d.model)
	if err != nil {
		return nil, err
	}

	// Extract JSON block using regex to be more robust
	re := regexp.MustCompile("(?s)\\{.*\\}")
	jsonStr := re.FindString(response)
	if jsonStr == "" {
		return nil, fmt.Errorf("could not find JSON block in diplomat response: %s", response)
	}

	var result domain.ReviewResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("failed to parse diplomat response as JSON: %w. Response was: %s", err, jsonStr)
	}

	return &result, nil
}
