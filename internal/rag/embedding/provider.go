package embedding

import "context"

// EmbeddingProvider defines the interface for generating vector embeddings of text
type EmbeddingProvider interface {
	// EmbedText generates a vector embedding for a given text snippet
	EmbedText(ctx context.Context, text string) ([]float32, error)
	// EmbedBatch generates vector embeddings for multiple text snippets
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
	// Dimension returns the dimensionality of the generated vectors
	Dimension() int
}
