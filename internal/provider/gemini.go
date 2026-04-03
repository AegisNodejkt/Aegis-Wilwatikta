package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

type GeminiProvider struct {
	client  *genai.Client
	limiter RateLimiter
}

func NewGeminiProvider(ctx context.Context, apiKey string) (*GeminiProvider, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, err
	}
	return &GeminiProvider{
		client:  client,
		limiter: NewGeminiRateLimiter(),
	}, nil
}

func NewGeminiProviderWithLimiter(ctx context.Context, apiKey string, limiter RateLimiter) (*GeminiProvider, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, err
	}
	return &GeminiProvider{
		client:  client,
		limiter: limiter,
	}, nil
}

func (p *GeminiProvider) SendMessage(ctx context.Context, systemPrompt string, userPrompt string, modelName string) (string, error) {
	if p.limiter != nil {
		if err := p.limiter.Wait(ctx); err != nil {
			return "", fmt.Errorf("rate limit wait failed: %w", err)
		}
	}

	model := p.client.GenerativeModel(modelName)
	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text(systemPrompt)},
	}

	resp, err := model.GenerateContent(ctx, genai.Text(userPrompt))
	if err != nil {
		return "", err
	}

	if len(resp.Candidates) == 0 {
		return "", fmt.Errorf("no candidates in response")
	}

	var result string
	for _, part := range resp.Candidates[0].Content.Parts {
		if text, ok := part.(genai.Text); ok {
			result += string(text)
		}
	}

	return result, nil
}

func (p *GeminiProvider) Name() string {
	return "gemini"
}

func (p *GeminiProvider) Close() error {
	return p.client.Close()
}

func (p *GeminiProvider) ListAvailableModels(ctx context.Context) ([]string, error) {
	var modelNames []string
	iter := p.client.ListModels(ctx)
	for {
		m, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list models: %w", err)
		}
		// Extracting just the short name (e.g., "gemini-1.5-flash")
		modelNames = append(modelNames, strings.TrimPrefix(m.Name, "models/"))
	}
	return modelNames, nil
}

