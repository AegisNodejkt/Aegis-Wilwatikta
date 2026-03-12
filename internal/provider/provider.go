package provider

import (
	"context"
)

type AIProvider interface {
	SendMessage(ctx context.Context, systemPrompt string, userPrompt string, model string) (string, error)
	Name() string
}
