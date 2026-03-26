package main

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/agents"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/config"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/engine"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/platform"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/provider"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/rag/embedding"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/rag/store"
)

func main() {
	ctx := context.Background()

	// 1. Load Configuration
	cfg, err := config.LoadConfig(".ai-reviewer.yaml")
	if err != nil {
		log.Printf("Warning: failed to load config: %v. Using defaults.", err)
	}

	// 2. Initialize Platform
	var plat platform.Platform
	platformType := cfg.Platform

	switch platformType {
	case "github":
		if cfg.GitHub.Token == "" {
			log.Fatal("GITHUB_TOKEN is required for github platform")
		}
		plat = platform.NewGitHubPlatform(cfg.GitHub.Token)
	case "local":
		plat = platform.NewLocalPlatform(cfg.BaseBranch)
	default:
		log.Fatalf("unsupported platform: %s", platformType)
	}

	// 3. Initialize AI Provider
	var aiProvider provider.AIProvider
	providerType := cfg.Provider

	switch providerType {
	case "gemini":
		if cfg.GeminiAPIKey == "" {
			log.Fatal("GEMINI_API_KEY is required for gemini provider")
		}
		aiProvider, err = provider.NewGeminiProvider(ctx, cfg.GeminiAPIKey)
		if err != nil {
			log.Fatalf("failed to initialize gemini: %v", err)
		}
	case "openai":
		if cfg.OpenAIAPIKey == "" {
			log.Fatal("OPENAI_API_KEY is required for openai provider")
		}
		aiProvider = provider.NewOpenAIProvider(cfg.OpenAIAPIKey)
	default:
		log.Fatalf("unsupported provider: %s", providerType)
	}

	// 4. Initialize Agents
	var graphStore store.GraphStore
	var embedder embedding.EmbeddingProvider
	if cfg.RAG.Enabled {
		if cfg.RAG.Neo4jURI == "" {
			log.Fatal("NEO4J_URI (or connection_url in config) is required for RAG")
		}
		graphStore, err = store.NewNeo4jStore(cfg.RAG.Neo4jURI, cfg.RAG.Neo4jUser, cfg.RAG.Neo4jPass, cfg.RAG.Neo4jDB)
		if err != nil {
			log.Printf("Warning: failed to initialize graph store: %v. Continuing without RAG.", err)
		}

		embProvType := cfg.RAG.EmbeddingProvider
		switch embProvType {
		case "google":
			if cfg.GeminiAPIKey != "" {
				embedder, err = embedding.NewGoogleEmbeddingProvider(ctx, cfg.GeminiAPIKey, "")
				if err != nil {
					log.Printf("Warning: failed to initialize google embedder: %v", err)
				}
			}
		case "openai":
			if cfg.OpenAIAPIKey != "" {
				embedder = embedding.NewOpenAIEmbeddingProvider(cfg.OpenAIAPIKey, "")
			}
		}
	}

	scout := agents.NewScout(aiProvider, plat, graphStore, embedder, cfg.ScoutModel, cfg.TenantID, cfg.ProjectID)
	arch := agents.NewArchitect(aiProvider, cfg.ArchitectModel)
	dip := agents.NewDiplomat(aiProvider, cfg.DiplomatModel, cfg.DashboardURL, cfg.DashboardAPIKey)

	// 5. Initialize and Run Engine
	reviewer := engine.NewReviewerEngine(plat, scout, arch, dip)

	owner, repo := parseRepo(cfg.GitHub.Repository)
	prNumber := cfg.GitHub.PRNumber
	err = reviewer.RunReview(ctx, owner, repo, prNumber)
	if err != nil {
		log.Fatalf("review failed: %v", err)
	}
}

func parseRepo(repoName string) (string, string) {
	if repoName == "" {
		repoName = os.Getenv("GITHUB_REPOSITORY")
	}

	parts := strings.Split(repoName, "/")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", ""
}
