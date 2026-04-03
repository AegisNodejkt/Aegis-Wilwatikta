package provider

import (
	"context"
	"fmt"
	"log"
	"os"
)

// AdapterConfig holds model configurations for different providers
type AdapterConfig struct {
	GeminiModel     string
	OpenAIModel     string
	GLMModel        string
	OpenRouterModel string
	QwenModel       string
}

// Adapter creates and configures AI Providers
type Adapter struct {
	Config AdapterConfig
}

// NewAdapter initializes a new Adapter
func NewAdapter(config AdapterConfig) *Adapter {
	return &Adapter{
		Config: config,
	}
}

// CreateProvider instantiates the specified AI provider using environment variables
func (a *Adapter) CreateProvider(ctx context.Context, providerType string) (AIProvider, error) {
	switch providerType {
	case "gemini":
		apiKey := os.Getenv("GEMINI_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("GEMINI_API_KEY is required for gemini provider")
		}
		aiProvider, err := NewGeminiProvider(ctx, apiKey)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize gemini: %w", err)
		}
		models, err := aiProvider.ListAvailableModels(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list models: %w", err)
		}
		log.Printf("Available models: %v", models)
		return aiProvider, nil

	case "openai":
		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("OPENAI_API_KEY is required for openai provider")
		}
		return NewOpenAIProvider(apiKey), nil

	case "glm":
		apiKey := os.Getenv("GLM_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("GLM_API_KEY is required for glm provider")
		}
		return NewGLMProvider(apiKey), nil

	case "openrouter":
		apiKey := os.Getenv("OPENROUTER_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("OPENROUTER_API_KEY is required for openrouter provider")
		}
		return NewOpenRouterProvider(apiKey), nil

	case "qwen":
		apiKey := os.Getenv("QWEN_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("QWEN_API_KEY is required for qwen provider")
		}
		return NewQwenProvider(apiKey), nil

	default:
		return nil, fmt.Errorf("unsupported provider: %s", providerType)
	}
}

// GetModelForProvider returns the correct model string given the provider tier
func (a *Adapter) GetModelForProvider(ctx context.Context, provider AIProvider, p string, tier string) string {
	var targetModel string
	switch p {
	case "gemini":
		if tier == "pro" {
			targetModel = "gemini-2.5-flash"
		} else {
			targetModel = a.Config.GeminiModel
		}
	case "openai":
		if tier == "pro" {
			targetModel = "gpt-4o"
		} else {
			targetModel = a.Config.OpenAIModel
		}
	case "glm":
		if tier == "pro" {
			targetModel = "glm-4"
		} else {
			targetModel = a.Config.GLMModel
		}
	case "openrouter":
		if tier == "pro" {
			targetModel = "qwen/qwen3.6-plus:free"
		} else {
			targetModel = a.Config.OpenRouterModel
		}
	case "qwen":
		if tier == "pro" {
			targetModel = "qwen-plus" // qwen-plus supports up to 131,072 tokens, while qwen-max is limited to 30,720 tokens context.
		} else {
			targetModel = a.Config.QwenModel
		}
	default:
		return ""
	}

	if provider != nil {
		available, err := provider.ListAvailableModels(ctx)
		if err == nil && len(available) > 0 {
			found := false
			for _, m := range available {
				if m == targetModel {
					found = true
					break
				}
			}
			if !found {
				log.Printf("Warning: Target model %s was not found in the provider's list of available models. Falling back to first available model: %s", targetModel, available[0])
				return available[0]
			}
		}
	}
	
	return targetModel
}
