package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/agents"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/db"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/engine"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/platform"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/provider"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/rag/embedding"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/rag/store"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/server"
	pstore "github.com/aegis-wilwatikta/ai-reviewer/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Provider      string   `yaml:"provider"`
	ProjectID     string   `yaml:"project_id"`
	GeminiModel   string   `yaml:"gemini_model"`
	OpenAIModel   string   `yaml:"openai_model"`
	BaseBranch    string   `yaml:"base_branch"`
	IgnorePaths   []string `yaml:"ignore_paths"`
	WebhookSecret string   `yaml:"webhook_secret"`
	RAG           struct {
		Enabled           bool   `yaml:"enabled"`
		GraphDB           string `yaml:"graph_db"`
		ConnectionURL     string `yaml:"connection_url"`
		EmbeddingProvider string `yaml:"embedding_provider"`
	} `yaml:"rag"`
}

func main() {
	ctx := context.Background()

	config, err := loadConfig()
	if err != nil {
		log.Printf("Warning: failed to load config: %v. Using defaults.", err)
	}

	var database *db.DB
	database, err = db.New(ctx)
	if err != nil {
		log.Printf("Warning: failed to connect to database: %v. Reviews will not be persisted.", err)
		database = nil
	} else {
		log.Println("Connected to PostgreSQL database")
		if err := database.RunMigrations(ctx); err != nil {
			log.Printf("Warning: failed to run migrations: %v", err)
		} else {
			log.Println("Database migrations completed")
		}
		defer database.Close()
	}

	var reviewStore pstore.ReviewStore
	if database != nil {
		reviewStore = pstore.NewPostgresReviewStore(database)
	}

	plat := initPlatform()
	if plat == nil {
		log.Println("Warning: platform not configured. Set GITHUB_TOKEN to enable review functionality.")
	}

	aiProvider := initAIProvider(ctx, &config)
	if aiProvider == nil {
		log.Println("Warning: AI provider not configured. Set GEMINI_API_KEY or OPENAI_API_KEY to enable review functionality.")
	}

	projectID := os.Getenv("PROJECT_ID")
	if projectID == "" {
		projectID = config.ProjectID
	}

	scoutModel := getModel(os.Getenv("SCOUT_MODEL"), "flash", os.Getenv("PROVIDER"), &config)
	archModel := getModel(os.Getenv("ARCHITECT_MODEL"), "pro", os.Getenv("PROVIDER"), &config)
	dipModel := getModel(os.Getenv("DIPLOMAT_MODEL"), "flash", os.Getenv("PROVIDER"), &config)

	graphStore, embedder := initRAG(ctx, &config)

	var reviewerEngine *engine.ReviewerEngine
	if plat != nil && aiProvider != nil {
		scout := agents.NewScout(aiProvider, plat, graphStore, embedder, scoutModel, projectID)
		arch := agents.NewArchitect(aiProvider, archModel)
		dip := agents.NewDiplomat(aiProvider, dipModel)
		reviewerEngine = engine.NewReviewerEngine(plat, scout, arch, dip)
	}

	var svc *server.ReviewService
	if reviewerEngine != nil {
		svc = server.NewReviewService(plat, reviewerEngine, reviewStore)
	}

	webhookSecret := os.Getenv("GITHUB_WEBHOOK_SECRET")
	if webhookSecret == "" {
		webhookSecret = config.WebhookSecret
	}

	triggerReview := func(owner, repo string, prNumber int, action string, installationID int64) error {
		if svc == nil {
			log.Printf("Review service not available")
			return nil
		}
		record, err := svc.EnqueueReview(context.Background(), &server.EnqueueRequest{
			TenantID:  uuid.Nil,
			ProjectID: uuid.Nil,
			Owner:     owner,
			Repo:      repo,
			PRNumber:  prNumber,
		})
		if err != nil {
			return err
		}
		log.Printf("Queued review %s for %s/%s PR #%d", record.ID, owner, repo, prNumber)
		return nil
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	r.Get("/health", server.HandleHealth)

	if svc != nil {
		r.Route("/api/v1/reviews", func(r chi.Router) {
			r.Post("/", svc.HandleTriggerReview)
			r.Get("/{id}", svc.HandleGetReview)
			r.Route("/tenants/{tenantId}/projects/{projectId}", func(r chi.Router) {
				r.Get("/", svc.HandleListReviews)
			})
		})

		if webhookSecret != "" {
			r.Route("/webhook/github", server.GitHubWebhookRouter(webhookSecret, triggerReview))
		}
	}

	r.Route("/reviews", func(r chi.Router) {
		if svc != nil {
			r.Post("/", svc.HandleTriggerReview)
			r.Get("/{id}", svc.HandleGetReview)
		}
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 300 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("Aegis API server starting on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdown); err != nil {
		log.Fatalf("server forced to shutdown: %v", err)
	}
	log.Println("Server exited")
}

func initPlatform() platform.Platform {
	platformType := os.Getenv("PLATFORM")
	if platformType == "" {
		platformType = "github"
	}

	switch platformType {
	case "github":
		token := os.Getenv("GITHUB_TOKEN")
		if token == "" {
			return nil
		}
		return platform.NewGitHubPlatform(token)
	case "local":
		branch := os.Getenv("BASE_BRANCH")
		if branch == "" {
			branch = "main"
		}
		return platform.NewLocalPlatform(branch)
	default:
		log.Printf("unsupported platform: %s", platformType)
		return nil
	}
}

func initAIProvider(ctx context.Context, config *Config) provider.AIProvider {
	providerType := os.Getenv("PROVIDER")
	if providerType == "" {
		providerType = config.Provider
	}
	if providerType == "" {
		providerType = "gemini"
	}

	switch providerType {
	case "gemini":
		apiKey := os.Getenv("GEMINI_API_KEY")
		if apiKey == "" {
			return nil
		}
		p, err := provider.NewGeminiProvider(ctx, apiKey)
		if err != nil {
			log.Printf("Warning: failed to initialize gemini: %v", err)
			return nil
		}
		return p
	case "openai":
		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			return nil
		}
		return provider.NewOpenAIProvider(apiKey)
	default:
		log.Printf("unsupported provider: %s", providerType)
		return nil
	}
}

func initRAG(ctx context.Context, config *Config) (store.GraphStore, embedding.EmbeddingProvider) {
	if !config.RAG.Enabled {
		return nil, nil
	}

	neo4jUser := os.Getenv("NEO4J_USER")
	neo4jPass := os.Getenv("NEO4J_PASS")
	neo4jDB := os.Getenv("NEO4J_DATABASE")
	if neo4jDB == "" {
		neo4jDB = "neo4j"
	}

	graphStore, err := store.NewNeo4jStore(config.RAG.ConnectionURL, neo4jUser, neo4jPass, neo4jDB)
	if err != nil {
		log.Printf("Warning: failed to initialize graph store: %v. Continuing without RAG.", err)
		return nil, nil
	}

	embProvType := os.Getenv("EMBEDDING_PROVIDER")
	if embProvType == "" {
		embProvType = config.RAG.EmbeddingProvider
	}
	if embProvType == "" {
		embProvType = "google"
	}

	var embedder embedding.EmbeddingProvider
	switch embProvType {
	case "google":
		if apiKey := os.Getenv("GEMINI_API_KEY"); apiKey != "" {
			embedder, _ = embedding.NewGoogleEmbeddingProvider(ctx, apiKey, "")
		}
	case "openai":
		if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
			embedder = embedding.NewOpenAIEmbeddingProvider(apiKey, "")
		}
	}

	return graphStore, embedder
}

func loadConfig() (Config, error) {
	cfg := Config{
		Provider:    "gemini",
		GeminiModel: "gemini-1.5-flash",
		OpenAIModel: "gpt-4o-mini",
		BaseBranch:  "main",
	}

	f, err := os.Open(".ai-reviewer.yaml")
	if err != nil {
		return cfg, err
	}
	defer f.Close()

	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func getModel(envKey, tier, providerType string, config *Config) string {
	if m := os.Getenv(envKey); m != "" {
		return m
	}
	if providerType == "" {
		providerType = config.Provider
	}
	if providerType == "gemini" {
		if tier == "pro" {
			return "gemini-1.5-pro"
		}
		return config.GeminiModel
	}
	if providerType == "openai" {
		if tier == "pro" {
			return "gpt-4o"
		}
		return config.OpenAIModel
	}
	return ""
}
