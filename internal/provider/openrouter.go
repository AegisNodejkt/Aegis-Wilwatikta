package provider

import (
	"context"
	"fmt"

	"github.com/sashabaranov/go-openai"
)

type OpenRouterProvider struct {
	client  *openai.Client
	limiter RateLimiter
}

func NewOpenRouterProvider(apiKey string) *OpenRouterProvider {
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = "https://openrouter.ai/api/v1"
	return &OpenRouterProvider{
		client:  openai.NewClientWithConfig(config),
		limiter: NewOpenAIRateLimiter(),
	}
}

func NewOpenRouterProviderWithLimiter(apiKey string, limiter RateLimiter) *OpenRouterProvider {
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = "https://openrouter.ai/api/v1"
	return &OpenRouterProvider{
		client:  openai.NewClientWithConfig(config),
		limiter: limiter,
	}
}

func (p *OpenRouterProvider) SendMessage(ctx context.Context, systemPrompt string, userPrompt string, modelName string) (string, error) {
	if p.limiter != nil {
		if err := p.limiter.Wait(ctx); err != nil {
			return "", fmt.Errorf("rate limit wait failed: %w", err)
		}
	}

	resp, err := p.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model:     modelName,
			MaxTokens: 2048,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: systemPrompt,
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: userPrompt,
				},
			},
		},
	)

	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return resp.Choices[0].Message.Content, nil
}

func (p *OpenRouterProvider) Name() string {
	return "openrouter"
}

func (p *OpenRouterProvider) GetTokenUsage(resp *openai.ChatCompletionResponse) (int, int) {
	return resp.Usage.PromptTokens, resp.Usage.CompletionTokens
}

func (p *OpenRouterProvider) ListAvailableModels(ctx context.Context) ([]string, error) {
	return nil, fmt.Errorf("not implemented")
}
