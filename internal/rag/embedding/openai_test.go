package embedding

import (
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestOpenAIEmbeddingProvider_Dimension(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected int
	}{
		{"default", "", 1536},
		{"small", string(openai.SmallEmbedding3), 1536},
		{"large", string(openai.LargeEmbedding3), 3072},
		{"ada", string(openai.AdaEmbeddingV2), 1536},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewOpenAIEmbeddingProvider("dummy", tt.model)
			if got := p.Dimension(); got != tt.expected {
				t.Errorf("Dimension() = %v, want %v", got, tt.expected)
			}
		})
	}
}
