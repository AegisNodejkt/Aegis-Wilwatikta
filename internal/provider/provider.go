package provider

import (
	"context"
)

type AIProvider interface {
	SendMessage(ctx context.Context, systemPrompt string, userPrompt string, model string) (string, error)
	ListAvailableModels(ctx context.Context) ([]string, error)
	Name() string
}
