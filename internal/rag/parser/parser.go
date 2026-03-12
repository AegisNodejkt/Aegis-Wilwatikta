package parser

import (
	"context"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/domain"
)

// CodeParser defines the interface for parsing source code into domain entities and relations
type CodeParser interface {
	// ParseFile parses a raw file into a list of entities and relations
	ParseFile(ctx context.Context, path string, content []byte) ([]domain.CodeNode, []domain.CodeRelation, error)
	// Supports returns true if the parser can handle the given file extension
	Supports(extension string) bool
}
