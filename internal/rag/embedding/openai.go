package embedding

import (
	"context"

	"github.com/sashabaranov/go-openai"
)

type OpenAIEmbeddingProvider struct {
	client *openai.Client
	model  openai.EmbeddingModel
}

func NewOpenAIEmbeddingProvider(apiKey string, model string) *OpenAIEmbeddingProvider {
	if model == "" {
		model = string(openai.SmallEmbedding3)
	}
	return &OpenAIEmbeddingProvider{
		client: openai.NewClient(apiKey),
		model:  openai.EmbeddingModel(model),
	}
}

func (p *OpenAIEmbeddingProvider) EmbedText(ctx context.Context, text string) ([]float32, error) {
	resp, err := p.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Input: []string{text},
		Model: p.model,
	})
	if err != nil {
		return nil, err
	}
	return resp.Data[0].Embedding, nil
}

func (p *OpenAIEmbeddingProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	resp, err := p.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Input: texts,
		Model: p.model,
	})
	if err != nil {
		return nil, err
	}
	var results [][]float32
	for _, data := range resp.Data {
		results = append(results, data.Embedding)
	}
	return results, nil
}

func (p *OpenAIEmbeddingProvider) Dimension() int {
	switch p.model {
	case openai.SmallEmbedding3:
		return 1536
	case openai.LargeEmbedding3:
		return 3072
	case openai.AdaEmbeddingV2:
		return 1536
	default:
		return 1536
	}
}
