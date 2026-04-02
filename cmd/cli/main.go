package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/agents"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/engine"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/platform"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/provider"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/rag/embedding"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/rag/store"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Provider    string   `yaml:"provider"`
	ProjectID   string   `yaml:"project_id"`
	GeminiModel string   `yaml:"gemini_model"`
	OpenAIModel string   `yaml:"openai_model"`
	GLMModel    string   `yaml:"glm_model"`
	BaseBranch  string   `yaml:"base_branch"`
	IgnorePaths []string `yaml:"ignore_paths"`
	RAG         struct {
		Enabled           bool   `yaml:"enabled"`
		GraphDB           string `yaml:"graph_db"`
		ConnectionURL     string `yaml:"connection_url"`
		EmbeddingProvider string `yaml:"embedding_provider"`
	} `yaml:"rag"`
}

func main() {
	ctx := context.Background()

	// 1. Load Configuration
	config, err := loadConfig()
	if err != nil {
		log.Printf("Warning: failed to load config: %v. Using defaults.", err)
	}

	// 2. Initialize Platform
	var plat platform.Platform
	platformType := os.Getenv("PLATFORM")
	if platformType == "" {
		platformType = "github"
	}

	repoName := os.Getenv("REPOSITORY_NAME") // owner/repo
	prNumStr := os.Getenv("PR_NUMBER")
	prNumber, _ := strconv.Atoi(prNumStr)

	switch platformType {
	case "github":
		token := os.Getenv("GITHUB_TOKEN")
		if token == "" {
			log.Fatal("GITHUB_TOKEN is required for github platform")
		}
		plat = platform.NewGitHubPlatform(token)
	case "local":
		plat = platform.NewLocalPlatform(config.BaseBranch)
	default:
		log.Fatalf("unsupported platform: %s", platformType)
	}

	// 3. Initialize AI Provider
	var aiProvider provider.AIProvider
	providerType := os.Getenv("PROVIDER")
	if providerType == "" {
		providerType = config.Provider
	}

	switch providerType {
	case "gemini":
		apiKey := os.Getenv("GEMINI_API_KEY")
		if apiKey == "" {
			log.Fatal("GEMINI_API_KEY is required for gemini provider")
		}
		aiProvider, err = provider.NewGeminiProvider(ctx, apiKey)
		if err != nil {
			log.Fatalf("failed to initialize gemini: %v", err)
		}
		models, err := aiProvider.ListAvailableModels(ctx)
		if err != nil {
			log.Fatalf("failed to list models: %v", err)
		}
		log.Printf("Available models: %v", models)
	case "openai":
		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			log.Fatal("OPENAI_API_KEY is required for openai provider")
		}
		aiProvider = provider.NewOpenAIProvider(apiKey)
	case "glm":
		apiKey := os.Getenv("GLM_API_KEY")
		if apiKey == "" {
			log.Fatal("GLM_API_KEY is required for glm provider")
		}
		aiProvider = provider.NewGLMProvider(apiKey)
	default:
		log.Fatalf("unsupported provider: %s", providerType)
	}

	// 4. Initialize Agents
	scoutModel := os.Getenv("SCOUT_MODEL")
	if scoutModel == "" {
		scoutModel = getModelForProvider(providerType, config, "flash")
	}
	archModel := os.Getenv("ARCHITECT_MODEL")
	if archModel == "" {
		archModel = getModelForProvider(providerType, config, "pro")
	}
	dipModel := os.Getenv("DIPLOMAT_MODEL")
	if dipModel == "" {
		dipModel = getModelForProvider(providerType, config, "flash")
	}

	var graphStore store.GraphStore
	var embedder embedding.EmbeddingProvider
	if config.RAG.Enabled {
		neo4jUser := os.Getenv("NEO4J_USER")
		neo4jPass := os.Getenv("NEO4J_PASS")
		neo4jDB := os.Getenv("NEO4J_DATABASE")
		if neo4jDB == "" {
			neo4jDB = "neo4j"
		}
		graphStore, err = store.NewNeo4jStore(config.RAG.ConnectionURL, neo4jUser, neo4jPass, neo4jDB)
		if err != nil {
			log.Printf("Warning: failed to initialize graph store: %v. Continuing without RAG.", err)
		}

		embProvType := os.Getenv("EMBEDDING_PROVIDER")
		if embProvType == "" {
			embProvType = config.RAG.EmbeddingProvider
		}
		if embProvType == "" {
			embProvType = "google"
		}

		switch embProvType {
		case "google":
			apiKey := os.Getenv("GEMINI_API_KEY")
			if apiKey != "" {
				embedder, err = embedding.NewGoogleEmbeddingProvider(ctx, apiKey, "")
				if err != nil {
					log.Printf("Warning: failed to initialize google embedder: %v", err)
				}
			}
		case "openai":
			apiKey := os.Getenv("OPENAI_API_KEY")
			if apiKey != "" {
				embedder = embedding.NewOpenAIEmbeddingProvider(apiKey, "")
			}
		}
	}

	projectID := os.Getenv("PROJECT_ID")
	if projectID == "" {
		projectID = config.ProjectID
	}
	if projectID == "" {
		projectID = repoName
	}

	scout := agents.NewScout(aiProvider, plat, graphStore, embedder, scoutModel, projectID)
	arch := agents.NewArchitect(aiProvider, archModel)
	dip := agents.NewDiplomat(aiProvider, dipModel)

	// 5. Initialize and Run Engine
	pipelineConfig := engine.DefaultPipelineConfig()
	reviewer := engine.NewPipelinedReviewerEngine(plat, scout, arch, dip, pipelineConfig)

	owner, repo := parseRepo(repoName)
	result, err := reviewer.RunReviewWithGracefulDegradation(ctx, owner, repo, prNumber)
	if err != nil {
		log.Fatalf("review failed: %v", err)
	}

	if result != nil && len(result.Errors) > 0 {
		log.Printf("Warning: Pipeline encountered errors (fallback might have been used): %v", result.Errors)
	}
}

func loadConfig() (Config, error) {
	config := Config{
		Provider:    "gemini",
		GeminiModel: "gemini-2.5-flash-lite",
		OpenAIModel: "gpt-4o-mini",
		GLMModel:    "glm-4-flash",
		BaseBranch:  "main",
	}

	f, err := os.Open(".ai-reviewer.yaml")
	if err != nil {
		return config, err
	}
	defer f.Close()

	if err := yaml.NewDecoder(f).Decode(&config); err != nil {
		return config, err
	}

	return config, nil
}

func getModelForProvider(p string, config Config, tier string) string {
	if p == "gemini" {
		if tier == "pro" {
			return "gemini-2.5-flash"
		}
		return config.GeminiModel
	}
	if p == "openai" {
		if tier == "pro" {
			return "gpt-4o"
		}
		return config.OpenAIModel
	}
	if p == "glm" {
		if tier == "pro" {
			return "glm-4"
		}
		return config.GLMModel
	}
	return ""
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
