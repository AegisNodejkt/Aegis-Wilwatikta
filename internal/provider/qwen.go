package provider

import (
	"context"
	"fmt"

	"github.com/sashabaranov/go-openai"
)

type QwenProvider struct {
	client  *openai.Client
	limiter RateLimiter
}

func NewQwenProvider(apiKey string) *QwenProvider {
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = "https://dashscope-intl.aliyuncs.com/compatible-mode/v1"
	return &QwenProvider{
		client:  openai.NewClientWithConfig(config),
		limiter: NewOpenAIRateLimiter(), // reuse OpenAI rate limiter logic
	}
}

func NewQwenProviderWithLimiter(apiKey string, limiter RateLimiter) *QwenProvider {
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	return &QwenProvider{
		client:  openai.NewClientWithConfig(config),
		limiter: limiter,
	}
}

func (p *QwenProvider) SendMessage(ctx context.Context, systemPrompt string, userPrompt string, modelName string) (string, error) {
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

func (p *QwenProvider) Name() string {
	return "qwen"
}

func (p *QwenProvider) GetTokenUsage(resp *openai.ChatCompletionResponse) (int, int) {
	return resp.Usage.PromptTokens, resp.Usage.CompletionTokens
}

func (p *QwenProvider) ListAvailableModels(ctx context.Context) ([]string, error) {
	return nil, fmt.Errorf("not implemented")
}
