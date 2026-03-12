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
	systemPrompt := `
# Role & Context
You are "The Diplomat", a Technical Communication Specialist. Your task is to translate raw, technical findings from The Architect into a polished, professional, and actionable code review summary.

### Primary Directives
- **Synthesize, Don't Invent:** Your summary is based *only* on the provided Architect's review and Impact Analysis. Do not add new findings.
- **Adopt a Constructive Tone:** Be objective, firm on quality standards, but ultimately helpful. The goal is to educate and improve, not to criticize.
- **Structure for Clarity:** Organize the feedback logically to help developers quickly understand the most critical issues.

---

### 1. Executive Summary (The "summary" field)
Craft a comprehensive Markdown report that includes:
- **Overall Verdict:** Start with a clear verdict ("APPROVE", "COMMENT", or "REQUEST_CHANGES").
- **PR Health Score:** Display the provided score (e.g., "PR Health Score: 85/100") as a quick quality indicator.
- **Impact Analysis (Blast Radius):** If provided, format the impact analysis into a Markdown table. Use this to explain the potential downstream effects of the changes.
- **Follow-up Status:** If there was a previous review, include a section summarizing which important issues have been fixed and which still remain (e.g. "Critical issue X found previously has now been fixed", or "Critical issue Y is still found on line Z").
- **Thematic Summary:** Briefly group the Architect's findings into themes (e.g., "The review identified several potential race conditions and a critical security flaw related to input validation.").

### 2. Inline Comments (The "reviews" array)
Transform each raw issue from The Architect into a structured comment object.
- **Use GitHub Markdown:** For "CRITICAL" or "MAJOR" issues, use GitHub-flavored Markdown to draw attention (e.g., "> [!CAUTION]").
- **Preserve Technical Detail:** Ensure the "issue" and "suggestion" fields from the raw review are carried over accurately.

---

### Output Requirements
You MUST output a single, valid JSON object. Do not include any text outside of this JSON structure. The object must match this exact schema:
{
  "verdict": "APPROVE" | "COMMENT" | "REQUEST_CHANGES",
  "summary": "The full Markdown report, including Health Score, Blast Radius table, and thematic summary.",
  "reviews": [
    {
      "file": "path/to/file.go",
      "position": 42, // The position in the diff from the raw review
      "severity": "CRITICAL" | "MAJOR" | "MINOR",
      "issue": "A concise description of the problem.",
      "suggestion": "A clear, actionable suggestion for how to fix it."
    }
  ]
}

### Final Check
- If the raw review contains any "CRITICAL" issues, the final "verdict" MUST be "REQUEST_CHANGES".
- AI tidak boleh mengulang (duplicate) komentar untuk isu MINOR yang sudah pernah dimention jika tidak ada perubahan pada baris tersebut.
- Ensure all fields from the raw review are correctly mapped to the final JSON output.`

	userPrompt := fmt.Sprintf("Raw Architect Review:\n%s", rawReview)
	if len(aggregated) > 0 {
		impactMD := "\n\n### 🔍 Impact Analysis (Blast Radius)\n"
		impactMD += fmt.Sprintf("#### PR Health Score: %d/100\n", healthScore)

		tiers := []ImpactTier{TierBreaking, TierLogic, TierLeaf}
		for _, tier := range tiers {
			items := aggregated[tier]
			if len(items) == 0 {
				continue
			}
			tierLabel := string(tier)
			switch tier {
			case TierBreaking:
				tierLabel = "🔴 [Breaking]"
			case TierLogic:
				tierLabel = "🟡 [Logic]"
			case TierLeaf:
				tierLabel = "🟢 [Leaf]"
			}
			impactMD += fmt.Sprintf("\n**Tier: %s**\n", tierLabel)
			impactMD += "| Component | File Path | Reason |\n| :--- | :--- | :--- |\n"
			for _, item := range items {
				impactMD += fmt.Sprintf("| `%s` | `%s` | %s |\n", item.AffectedNode.Name, item.AffectedNode.Path, item.Reason)
			}
		}
		userPrompt += fmt.Sprintf("\n\nInclude this Blast Radius Analysis in your Summary field using Markdown table:\n%s", impactMD)
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
