package config

import (
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	// Global
	ProjectID  string   `yaml:"project_id"`
	TenantID   string   `yaml:"tenant_id"`
	Platform   string   `yaml:"platform"`
	Provider   string   `yaml:"provider"`
	BaseBranch string   `yaml:"base_branch"`
	IgnorePaths []string `yaml:"ignore_paths"`

	// Dashboard Integration
	DashboardURL    string `yaml:"dashboard_url"`
	DashboardAPIKey string `yaml:"dashboard_api_key"`

	// Platform Specific
	GitHub struct {
		Token      string `yaml:"token"`
		Repository string `yaml:"repository"` // owner/repo
		PRNumber   int    `yaml:"pr_number"`
	} `yaml:"github"`

	// AI Provider Keys & Models
	GeminiAPIKey   string `yaml:"gemini_api_key"`
	OpenAIAPIKey   string `yaml:"openai_api_key"`
	GeminiModel    string `yaml:"gemini_model"`
	OpenAIModel    string `yaml:"openai_model"`
	ScoutModel     string `yaml:"scout_model"`
	ArchitectModel string `yaml:"architect_model"`
	DiplomatModel  string `yaml:"diplomat_model"`

	// RAG / Neo4j
	RAG struct {
		Enabled           bool   `yaml:"enabled"`
		Neo4jURI          string `yaml:"neo4j_uri"`
		Neo4jUser         string `yaml:"neo4j_user"`
		Neo4jPass         string `yaml:"neo4j_pass"`
		Neo4jDB           string `yaml:"neo4j_database"`
		EmbeddingProvider string `yaml:"embedding_provider"`
		// Legacy fields from CLI config for backward compatibility
		ConnectionURL     string `yaml:"connection_url"`
		GraphDB           string `yaml:"graph_db"`
	} `yaml:"rag"`
}

func LoadConfig(configPath string) (*Config, error) {
	cfg := &Config{}
	
	// 1. Set Defaults
	cfg.Platform = "github"
	cfg.Provider = "gemini"
	cfg.BaseBranch = "main"
	cfg.GeminiModel = "gemini-1.5-flash"
	cfg.OpenAIModel = "gpt-4o-mini"
	cfg.RAG.Neo4jDB = "neo4j"
	cfg.RAG.EmbeddingProvider = "google"

	// 2. Load from YAML if exists
	if configPath == "" {
		configPath = ".ai-reviewer.yaml"
	}
	if f, err := os.Open(configPath); err == nil {
		defer f.Close()
		_ = yaml.NewDecoder(f).Decode(cfg)
	}

	// 3. Override with Environment Variables
	if v := os.Getenv("PROJECT_ID"); v != "" {
		cfg.ProjectID = v
	}
	if cfg.ProjectID == "" {
		cfg.ProjectID = deriveProjectID()
	}

	if v := os.Getenv("TENANT_ID"); v != "" {
		cfg.TenantID = v
	}

	if v := os.Getenv("AEGIS_DASHBOARD_URL"); v != "" {
		cfg.DashboardURL = v
	}
	if v := os.Getenv("AEGIS_DASHBOARD_API_KEY"); v != "" {
		cfg.DashboardAPIKey = v
	}

	if v := os.Getenv("PLATFORM"); v != "" {
		cfg.Platform = v
	}
	if v := os.Getenv("REPOSITORY_NAME"); v != "" {
		cfg.GitHub.Repository = v
	}
	if v := os.Getenv("GITHUB_REPOSITORY"); v != "" && cfg.GitHub.Repository == "" {
		cfg.GitHub.Repository = v
	}
	if v := os.Getenv("GITHUB_TOKEN"); v != "" {
		cfg.GitHub.Token = v
	}
	if v := os.Getenv("PR_NUMBER"); v != "" {
		cfg.GitHub.PRNumber, _ = strconv.Atoi(v)
	}

	if v := os.Getenv("PROVIDER"); v != "" {
		cfg.Provider = v
	}
	if v := os.Getenv("GEMINI_API_KEY"); v != "" {
		cfg.GeminiAPIKey = v
	}
	if v := os.Getenv("OPENAI_API_KEY"); v != "" {
		cfg.OpenAIAPIKey = v
	}
	if v := os.Getenv("SCOUT_MODEL"); v != "" {
		cfg.ScoutModel = v
	}
	if v := os.Getenv("ARCHITECT_MODEL"); v != "" {
		cfg.ArchitectModel = v
	}
	if v := os.Getenv("DIPLOMAT_MODEL"); v != "" {
		cfg.DiplomatModel = v
	}

	// RAG Overrides
	if v := os.Getenv("NEO4J_URI"); v != "" {
		cfg.RAG.Neo4jURI = v
	}
	if cfg.RAG.Neo4jURI == "" && cfg.RAG.ConnectionURL != "" {
		cfg.RAG.Neo4jURI = cfg.RAG.ConnectionURL
	}
	if v := os.Getenv("NEO4J_USER"); v != "" {
		cfg.RAG.Neo4jUser = v
	}
	if v := os.Getenv("NEO4J_PASS"); v != "" {
		cfg.RAG.Neo4jPass = v
	}
	if v := os.Getenv("NEO4J_DATABASE"); v != "" {
		cfg.RAG.Neo4jDB = v
	}
	if cfg.RAG.Neo4jDB == "neo4j" && cfg.RAG.GraphDB != "" {
		cfg.RAG.Neo4jDB = cfg.RAG.GraphDB
	}
	if v := os.Getenv("EMBEDDING_PROVIDER"); v != "" {
		cfg.RAG.EmbeddingProvider = v
	}

	// Dynamic Model Resolution (Backwards compat for tiers)
	if cfg.ScoutModel == "" {
		cfg.ScoutModel = getModelForTier(cfg, "flash")
	}
	if cfg.ArchitectModel == "" {
		cfg.ArchitectModel = getModelForTier(cfg, "pro")
	}
	if cfg.DiplomatModel == "" {
		cfg.DiplomatModel = getModelForTier(cfg, "flash")
	}

	return cfg, nil
}

func getModelForTier(cfg *Config, tier string) string {
	if cfg.Provider == "gemini" {
		if tier == "pro" {
			return "gemini-1.5-pro"
		}
		return cfg.GeminiModel
	}
	if cfg.Provider == "openai" {
		if tier == "pro" {
			return "gpt-4o"
		}
		return cfg.OpenAIModel
	}
	return ""
}

func deriveProjectID() string {
	dir, err := os.Getwd()
	if err != nil {
		return "unknown"
	}
	return filepath.Base(dir)
}
