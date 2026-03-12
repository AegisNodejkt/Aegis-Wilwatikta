package embedding

import (
	"context"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type GoogleEmbeddingProvider struct {
	client *genai.Client
	model  string
}

func NewGoogleEmbeddingProvider(ctx context.Context, apiKey string, model string) (*GoogleEmbeddingProvider, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, err
	}
	if model == "" {
		model = "text-embedding-004"
	}
	return &GoogleEmbeddingProvider{
		client: client,
		model:  model,
	}, nil
}

func (p *GoogleEmbeddingProvider) EmbedText(ctx context.Context, text string) ([]float32, error) {
	em := p.client.EmbeddingModel(p.model)
	res, err := em.EmbedContent(ctx, genai.Text(text))
	if err != nil {
		return nil, err
	}
	return res.Embedding.Values, nil
}

func (p *GoogleEmbeddingProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	em := p.client.EmbeddingModel(p.model)
	batch := em.NewBatch()
	for _, t := range texts {
		batch.AddContent(genai.Text(t))
	}
	res, err := em.BatchEmbedContents(ctx, batch)
	if err != nil {
		return nil, err
	}

	var results [][]float32
	for _, e := range res.Embeddings {
		results = append(results, e.Values)
	}
	return results, nil
}

func (p *GoogleEmbeddingProvider) Dimension() int {
	// text-embedding-004 is 768-dimensional
	return 768
}

func (p *GoogleEmbeddingProvider) Close() error {
	return p.client.Close()
}
