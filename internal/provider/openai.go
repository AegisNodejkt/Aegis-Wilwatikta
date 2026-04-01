package provider

import (
	"context"
	"fmt"

	"github.com/sashabaranov/go-openai"
)

type OpenAIProvider struct {
	client  *openai.Client
	limiter RateLimiter
}

func NewOpenAIProvider(apiKey string) *OpenAIProvider {
	return &OpenAIProvider{
		client:  openai.NewClient(apiKey),
		limiter: NewOpenAIRateLimiter(),
	}
}

func NewOpenAIProviderWithLimiter(apiKey string, limiter RateLimiter) *OpenAIProvider {
	return &OpenAIProvider{
		client:  openai.NewClient(apiKey),
		limiter: limiter,
	}
}

func (p *OpenAIProvider) SendMessage(ctx context.Context, systemPrompt string, userPrompt string, modelName string) (string, error) {
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

func (p *OpenAIProvider) Name() string {
	return "openai"
}

func (p *OpenAIProvider) GetTokenUsage(resp *openai.ChatCompletionResponse) (int, int) {
	return resp.Usage.PromptTokens, resp.Usage.CompletionTokens
}
