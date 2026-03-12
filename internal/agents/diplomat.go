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

func (d *Diplomat) FormatReview(ctx context.Context, rawReview string) (*domain.ReviewResult, error) {
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
