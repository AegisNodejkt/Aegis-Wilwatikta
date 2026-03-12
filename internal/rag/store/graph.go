package store

import (
	"context"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/domain"
)

// GraphStore defines the interface for RAG operations using a graph database
type GraphStore interface {
	// UpsertNode saves or updates a code entity (func, struct, etc.)
	UpsertNode(ctx context.Context, node domain.CodeNode) error
	// UpsertRelation connects two entities (CALLS, IMPLEMENTS)
	UpsertRelation(ctx context.Context, rel domain.CodeRelation) error
	// GetImpactContext retrieves downstream effects for a given file path
	GetImpactContext(ctx context.Context, projectID, filePath string) (*domain.ImpactReport, error)
	// QueryContext retrieves sub-graph or relevant context for a file
	QueryContext(ctx context.Context, projectID, filePath string) ([]domain.CodeNode, error)
	// FindRelatedByEmbedding retrieves nodes that are semantically similar
	FindRelatedByEmbedding(ctx context.Context, projectID string, embedding []float32, limit int) ([]domain.CodeNode, error)
	// GetFileHash retrieves the stored hash for a file node
	GetFileHash(ctx context.Context, projectID, path string) (string, error)
	// DeleteNodesByFile removes all nodes and relations associated with a file
	DeleteNodesByFile(ctx context.Context, projectID, path string) error
	// DeleteNodesByProject removes all nodes and relations associated with a project
	DeleteNodesByProject(ctx context.Context, projectID string) error
}
