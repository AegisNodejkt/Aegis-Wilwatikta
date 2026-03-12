package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/rag/embedding"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/rag/parser"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/rag/store"
)

func main() {
	ctx := context.Background()

	var targetFiles []string
	if len(os.Args) > 1 {
		targetFiles = os.Args[1:]
	}

	// Configuration (In production, these come from Env/YAML)
	neo4jURI := os.Getenv("NEO4J_URI")
	neo4jUser := os.Getenv("NEO4J_USER")
	neo4jPass := os.Getenv("NEO4J_PASS")

	if neo4jURI == "" {
		log.Fatal("NEO4J_URI is required")
	}

	// Initialize components
	var embedder embedding.EmbeddingProvider
	var err error
	provider := os.Getenv("EMBEDDING_PROVIDER")
	if provider == "" {
		provider = "google"
	}

	switch provider {
	case "google":
		geminiKey := os.Getenv("GEMINI_API_KEY")
		if geminiKey == "" {
			log.Fatal("GEMINI_API_KEY is required for google embedding provider")
		}
		embedder, err = embedding.NewGoogleEmbeddingProvider(ctx, geminiKey, "")
	case "openai":
		openaiKey := os.Getenv("OPENAI_API_KEY")
		if openaiKey == "" {
			log.Fatal("OPENAI_API_KEY is required for openai embedding provider")
		}
		embedder = embedding.NewOpenAIEmbeddingProvider(openaiKey, "")
	default:
		log.Fatalf("unsupported embedding provider: %s", provider)
	}

	if err != nil {
		log.Fatalf("failed to init embedder: %v", err)
	}

	graph, err := store.NewNeo4jStore(neo4jURI, neo4jUser, neo4jPass)
	if err != nil {
		log.Fatalf("failed to init graph store: %v", err)
	}

	codeParser := parser.NewTSParser()

	// Crawl and Index
	if len(targetFiles) > 0 {
		for _, path := range targetFiles {
			if !codeParser.Supports(filepath.Ext(path)) {
				continue
			}
			err = indexFile(ctx, path, codeParser, embedder, graph)
			if err != nil {
				log.Printf("failed to index %s: %v", path, err)
			}
		}
	} else {
		err = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || isIgnored(path) || !codeParser.Supports(filepath.Ext(path)) {
				return nil
			}

			return indexFile(ctx, path, codeParser, embedder, graph)
		})
	}

	if err != nil {
		log.Fatalf("indexing failed: %v", err)
	}

	fmt.Println("Indexing completed successfully.")
}

func indexFile(ctx context.Context, path string, cp *parser.TSParser, emb embedding.EmbeddingProvider, graph store.GraphStore) error {
	fmt.Printf("Indexing %s...\n", path)
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	nodes, relations, err := cp.ParseFile(ctx, path, content)
	if err != nil {
		return err
	}

	var signatures []string
	for _, node := range nodes {
		signatures = append(signatures, node.Signature)
	}

	embeddings, err := emb.EmbedBatch(ctx, signatures)
	for i, node := range nodes {
		if err == nil && i < len(embeddings) {
			node.Embedding = embeddings[i]
		}
		graph.UpsertNode(ctx, node)
	}

	for _, rel := range relations {
		graph.UpsertRelation(ctx, rel)
	}

	return nil
}

func isIgnored(path string) bool {
	ignoredDirs := []string{".git", "vendor", "node_modules", "testdata"}
	for _, d := range ignoredDirs {
		if strings.Contains(path, "/"+d+"/") || strings.HasPrefix(path, d+"/") {
			return true
		}
	}
	return false
}
