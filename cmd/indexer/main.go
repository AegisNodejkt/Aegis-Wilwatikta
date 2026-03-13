package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/domain"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/rag/embedding"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/rag/parser"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/rag/store"
)

func main() {
	ctx := context.Background()

	var projectIDFlag string
	var maxWorkersFlag int
	flag.StringVar(&projectIDFlag, "project-id", "", "ID of the project")
	flag.IntVar(&maxWorkersFlag, "max-workers", 5, "Maximum number of concurrent workers")
	flag.Parse()

	command := flag.Arg(0)
	var targetFiles []string
	if command != "cleanup" && command != "" {
		targetFiles = flag.Args()
	}

	// Configuration
	neo4jURI := os.Getenv("NEO4J_URI")
	neo4jUser := os.Getenv("NEO4J_USER")
	neo4jPass := os.Getenv("NEO4J_PASS")
	neo4jDB := os.Getenv("NEO4J_DATABASE")
	maxWorkersEnv := os.Getenv("MAX_WORKERS")

	if maxWorkersEnv != "" {
		if val, err := strconv.Atoi(maxWorkersEnv); err == nil {
			maxWorkersFlag = val
		}
	}

	if neo4jURI == "" {
		log.Fatal("NEO4J_URI is required")
	}
	if neo4jDB == "" {
		neo4jDB = "neo4j"
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

	graph, err := store.NewNeo4jStore(neo4jURI, neo4jUser, neo4jPass, neo4jDB)
	if err != nil {
		log.Fatalf("failed to init graph store: %v", err)
	}
	defer graph.Close(ctx)

	projectID := os.Getenv("PROJECT_ID")
	if projectID == "" {
		projectID = projectIDFlag
	}
	if projectID == "" {
		projectID = deriveProjectID()
	}

	if command == "cleanup" {
		fmt.Printf("Cleaning up project %s...\n", projectID)
		err = graph.DeleteNodesByProject(ctx, projectID)
		if err != nil {
			log.Fatalf("cleanup failed: %v", err)
		}
		fmt.Println("Cleanup completed.")
		return
	}

	codeParser := parser.NewTSParser()

	filesToProcess := make(chan string, 100)
	var wg sync.WaitGroup

	// Start workers
	for w := 1; w <= maxWorkersFlag; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range filesToProcess {
				// Handle deletion if file doesn't exist
				if _, err := os.Stat(path); os.IsNotExist(err) {
					fmt.Printf("File %s deleted, removing from graph...\n", path)
					err = graph.DeleteNodesByFile(ctx, projectID, path)
					if err != nil {
						log.Printf("failed to delete nodes for %s: %v", path, err)
					}
					// Also delete the file node itself
					err = graph.DeleteFileNode(ctx, projectID, path)
					if err != nil {
						log.Printf("failed to delete file node for %s: %v", path, err)
					}
					continue
				}

				if !codeParser.Supports(filepath.Ext(path)) {
					continue
				}

				err := indexFile(ctx, projectID, path, codeParser, embedder, graph)
				if err != nil {
					log.Printf("failed to index %s: %v", path, err)
				}
			}
		}()
	}

	// Feed files to workers
	if len(targetFiles) > 0 {
		for _, path := range targetFiles {
			filesToProcess <- path
		}
	} else {
		err = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || isIgnored(path) || !codeParser.Supports(filepath.Ext(path)) {
				return nil
			}
			filesToProcess <- path
			return nil
		})
	}
	close(filesToProcess)
	wg.Wait()

	if err != nil {
		log.Fatalf("indexing failed: %v", err)
	}

	fmt.Println("Indexing completed successfully.")
}

func indexFile(ctx context.Context, projectID, path string, cp *parser.TSParser, emb embedding.EmbeddingProvider, graph store.GraphStore) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	nodes, relations, err := cp.ParseFile(ctx, path, content)
	if err != nil {
		return err
	}

	// Find the file node
	var fileNode domain.CodeNode
	for i := range nodes {
		nodes[i].ProjectID = projectID
		if nodes[i].Kind == domain.KindFile {
			fileNode = nodes[i]
		}
	}

	// Check if file has changed structurally (FR 2.2)
	oldHash, err := graph.GetFileHash(ctx, projectID, path)
	if err == nil && oldHash == fileNode.SignatureHash {
		fmt.Printf("Skipping %s (no structural changes)\n", path)
		// Update the file node anyway to sync content_hash if it changed
		_ = graph.UpsertNode(ctx, fileNode)
		return nil
	}

	fmt.Printf("Indexing %s...\n", path)

	// Prune old nodes before re-indexing (except the file node itself which we'll upsert)
	err = graph.DeleteNodesByFile(ctx, projectID, path)
	if err != nil {
		log.Printf("warning: failed to prune old nodes for %s: %v", path, err)
	}

	var signatures []string
	for _, node := range nodes {
		signatures = append(signatures, node.Signature)
	}

	embeddings, err := emb.EmbedBatch(ctx, signatures)
	for i := range nodes {
		if err == nil && i < len(embeddings) {
			nodes[i].Embedding = embeddings[i]
		}
		err = graph.UpsertNode(ctx, nodes[i])
		if err != nil {
			log.Printf("failed to upsert node %s: %v", nodes[i].ID, err)
		}
	}

	for _, rel := range relations {
		rel.ProjectID = projectID
		err = graph.UpsertRelation(ctx, rel)
		if err != nil {
			log.Printf("failed to upsert relation from %s to %s: %v", rel.From, rel.To, err)
		}
	}

	return nil
}

func deriveProjectID() string {
	dir, _ := os.Getwd()
	return filepath.Base(dir)
}

func isIgnored(path string) bool {
	ignoredDirs := []string{".git", "vendor", "node_modules", "testdata", "bin"}
	for _, d := range ignoredDirs {
		if strings.Contains(path, "/"+d+"/") || strings.HasPrefix(path, d+"/") {
			return true
		}
	}
	return false
}
