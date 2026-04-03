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
	Provider        string   `yaml:"provider"`
	ProjectID       string   `yaml:"project_id"`
	GeminiModel     string   `yaml:"gemini_model"`
	OpenAIModel     string   `yaml:"openai_model"`
	GLMModel        string   `yaml:"glm_model"`
	OpenRouterModel string   `yaml:"openrouter_model"`
	QwenModel       string   `yaml:"qwen_model"`
	BaseBranch      string   `yaml:"base_branch"`
	IgnorePaths     []string `yaml:"ignore_paths"`
	RAG             struct {
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
	providerType := os.Getenv("PROVIDER")
	if providerType == "" {
		providerType = config.Provider
	}

	validProviders := []string{"gemini", "openai", "glm", "openrouter", "qwen"}
	isValidProvider := false
	for _, p := range validProviders {
		if providerType == p {
			isValidProvider = true
			break
		}
	}
	if !isValidProvider {
		log.Fatalf("Critical Configuration Error: Unsupported provider '%s' specified. Valid options are: gemini, openai, glm, openrouter, qwen.", providerType)
	}

	adapterConfig := provider.AdapterConfig{
		GeminiModel:     config.GeminiModel,
		OpenAIModel:     config.OpenAIModel,
		GLMModel:        config.GLMModel,
		OpenRouterModel: config.OpenRouterModel,
		QwenModel:       config.QwenModel,
	}
	aiAdapter := provider.NewAdapter(adapterConfig)

	aiProvider, err := aiAdapter.CreateProvider(ctx, providerType)
	if err != nil {
		log.Fatalf("failed to initialize provider: %v", err)
	}

	// 4. Initialize Agents
	scoutModel := os.Getenv("SCOUT_MODEL")
	if scoutModel == "" {
		scoutModel = aiAdapter.GetModelForProvider(ctx, aiProvider, providerType, "flash")
	}
	archModel := os.Getenv("ARCHITECT_MODEL")
	if archModel == "" {
		archModel = aiAdapter.GetModelForProvider(ctx, aiProvider, providerType, "pro")
	}
	dipModel := os.Getenv("DIPLOMAT_MODEL")
	if dipModel == "" {
		dipModel = aiAdapter.GetModelForProvider(ctx, aiProvider, providerType, "flash")
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
		Provider:        "gemini",
		GeminiModel:     "gemini-2.5-flash-lite",
		OpenAIModel:     "gpt-4o-mini",
		GLMModel:        "glm-4-flash",
		OpenRouterModel: "qwen/qwen3.6-plus:free",
		QwenModel:       "qwen-turbo",
		BaseBranch:      "main",
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
