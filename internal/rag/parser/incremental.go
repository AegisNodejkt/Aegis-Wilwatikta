package parser

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/domain"
)

type FileHash struct {
	Path          string    `json:"path"`
	ContentHash   string    `json:"content_hash"`
	SignatureHash string    `json:"signature_hash"`
	ParsedAt      time.Time `json:"parsed_at"`
}

type IncrementalParser struct {
	parser    *TSParser
	cache     map[string]FileHash
	cachePath string
	mu        sync.RWMutex
}

func NewIncrementalParser(parser *TSParser, cachePath string) *IncrementalParser {
	ip := &IncrementalParser{
		parser:    parser,
		cache:     make(map[string]FileHash),
		cachePath: cachePath,
	}
	ip.loadCache()
	return ip
}

func (ip *IncrementalParser) loadCache() {
	if ip.cachePath == "" {
		return
	}
	data, err := os.ReadFile(ip.cachePath)
	if err != nil {
		return
	}
	var hashes []FileHash
	if err := json.Unmarshal(data, &hashes); err != nil {
		return
	}
	for _, h := range hashes {
		ip.cache[h.Path] = h
	}
}

func (ip *IncrementalParser) saveCache() error {
	if ip.cachePath == "" {
		return nil
	}
	ip.mu.RLock()
	hashes := make([]FileHash, 0, len(ip.cache))
	for _, h := range ip.cache {
		hashes = append(hashes, h)
	}
	ip.mu.RUnlock()
	data, err := json.MarshalIndent(hashes, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(ip.cachePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(ip.cachePath, data, 0644)
}

func (ip *IncrementalParser) computeContentHash(content []byte) string {
	hasher := sha256.New()
	hasher.Write(content)
	return hex.EncodeToString(hasher.Sum(nil))
}

func (ip *IncrementalParser) NeedsParse(path string, content []byte) bool {
	ip.mu.RLock()
	defer ip.mu.RUnlock()
	cached, exists := ip.cache[path]
	if !exists {
		return true
	}
	contentHash := ip.computeContentHash(content)
	return cached.ContentHash != contentHash
}

type ParseChange struct {
	Path      string
	Content   []byte
	Nodes     []domain.CodeNode
	Relations []domain.CodeRelation
	Errors    []ParseError
}

func (ip *IncrementalParser) ParseIncremental(ctx context.Context, changes []ParseChange) (*IncrementalResult, error) {
	result := &IncrementalResult{
		ParsedFiles:  make([]string, 0),
		CachedFiles:  make([]string, 0),
		AllNodes:     make([]domain.CodeNode, 0),
		AllRelations: make([]domain.CodeRelation, 0),
		AllErrors:    make([]ParseError, 0),
	}

	for _, change := range changes {
		if !ip.parser.Supports(filepath.Ext(change.Path)) {
			continue
		}

		if !ip.NeedsParse(change.Path, change.Content) {
			result.CachedFiles = append(result.CachedFiles, change.Path)
			continue
		}

		parseResult, err := ip.parser.ParseFileWithErrors(ctx, change.Path, change.Content)
		if err != nil {
			result.ParsedFiles = append(result.ParsedFiles, change.Path)
			continue
		}

		contentHash := ip.computeContentHash(change.Content)

		var sigHash string
		for _, n := range parseResult.Nodes {
			if n.Kind == domain.KindFile {
				sigHash = n.SignatureHash
				break
			}
		}

		ip.mu.Lock()
		ip.cache[change.Path] = FileHash{
			Path:          change.Path,
			ContentHash:   contentHash,
			SignatureHash: sigHash,
			ParsedAt:      time.Now(),
		}
		ip.mu.Unlock()

		result.ParsedFiles = append(result.ParsedFiles, change.Path)
		result.AllNodes = append(result.AllNodes, parseResult.Nodes...)
		result.AllRelations = append(result.AllRelations, parseResult.Relations...)
		result.AllErrors = append(result.AllErrors, parseResult.Errors...)
	}

	if err := ip.saveCache(); err != nil {
		return result, fmt.Errorf("failed to save cache: %w", err)
	}

	return result, nil
}

func (ip *IncrementalParser) ParseFull(ctx context.Context, changes []ParseChange) (*IncrementalResult, error) {
	ip.mu.Lock()
	for path := range ip.cache {
		delete(ip.cache, path)
	}
	ip.mu.Unlock()
	return ip.ParseIncremental(ctx, changes)
}

func (ip *IncrementalParser) GetCacheStats() map[string]interface{} {
	ip.mu.RLock()
	defer ip.mu.RUnlock()
	return map[string]interface{}{
		"cached_files": len(ip.cache),
	}
}

type IncrementalResult struct {
	ParsedFiles  []string
	CachedFiles  []string
	AllNodes     []domain.CodeNode
	AllRelations []domain.CodeRelation
	AllErrors    []ParseError
}

func (r *IncrementalResult) HasErrors() bool {
	return len(r.AllErrors) > 0
}

func (r *IncrementalResult) ErrorSummary() string {
	if !r.HasErrors() {
		return ""
	}
	summary := fmt.Sprintf("%d files with syntax errors:\n", len(r.ParsedFiles))
	for i, file := range r.ParsedFiles {
		if i >= 10 {
			summary += "... and more\n"
			break
		}
		summary += fmt.Sprintf("  - %s\n", file)
	}
	return summary
}
