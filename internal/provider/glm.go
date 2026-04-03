package provider

import (
	"context"
	"fmt"

	"github.com/sashabaranov/go-openai"
)

type GLMProvider struct {
	client  *openai.Client
	limiter RateLimiter
}

func NewGLMProvider(apiKey string) *GLMProvider {
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = "https://api.z.ai/api/paas/v4/"
	return &GLMProvider{
		client:  openai.NewClientWithConfig(config),
		limiter: NewOpenAIRateLimiter(), // reuse OpenAI rate limiter logic since it has a similar rate format
	}
}

func NewGLMProviderWithLimiter(apiKey string, limiter RateLimiter) *GLMProvider {
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = "https://api.z.ai/api/paas/v4/"
	return &GLMProvider{
		client:  openai.NewClientWithConfig(config),
		limiter: limiter,
	}
}

func (p *GLMProvider) SendMessage(ctx context.Context, systemPrompt string, userPrompt string, modelName string) (string, error) {
	if p.limiter != nil {
		if err := p.limiter.Wait(ctx); err != nil {
			return "", fmt.Errorf("rate limit wait failed: %w", err)
		}
	}

	resp, err := p.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: modelName,
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

func (p *GLMProvider) Name() string {
	return "glm"
}

func (p *GLMProvider) GetTokenUsage(resp *openai.ChatCompletionResponse) (int, int) {
	return resp.Usage.PromptTokens, resp.Usage.CompletionTokens
}

func (p *GLMProvider) ListAvailableModels(ctx context.Context) ([]string, error) {
	return nil, fmt.Errorf("not implemented")
}
