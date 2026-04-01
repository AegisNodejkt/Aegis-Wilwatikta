package db

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	Pool *pgxpool.Pool
}

func New(ctx context.Context) (*DB, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = fmt.Sprintf(
			"postgres://%s:%s@%s:%s/%s?sslmode=%s",
			getEnv("DB_USER", "postgres"),
			getEnv("DB_PASS", "postgres"),
			getEnv("DB_HOST", "localhost"),
			getEnv("DB_PORT", "5432"),
			getEnv("DB_NAME", "aegis"),
			getEnv("DB_SSLMODE", "disable"),
		)
	}

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database config: %w", err)
	}

	poolConfig.MaxConns = 20
	poolConfig.MinConns = 2
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{Pool: pool}, nil
}

func (d *DB) Close() {
	if d != nil && d.Pool != nil {
		d.Pool.Close()
	}
}

func (d *DB) RunMigrations(ctx context.Context) error {
	migrations := []string{
		migrationReviews,
		migrationReviewViolations,
	}

	for i, migration := range migrations {
		_, err := d.Pool.Exec(ctx, migration)
		if err != nil {
			return fmt.Errorf("migration %d failed: %w", i+1, err)
		}
	}

	return nil
}

const migrationReviews = `
CREATE TABLE IF NOT EXISTS backend_reviews (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    project_id UUID NOT NULL,
    owner VARCHAR(255) NOT NULL,
    repo VARCHAR(255) NOT NULL,
    pr_number INTEGER NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    verdict VARCHAR(50),
    summary TEXT,
    health_score INTEGER,
    guardrail_violations JSONB DEFAULT '[]'::jsonb,
    raw_review TEXT,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_reviews_tenant_project
    ON backend_reviews(tenant_id, project_id);
CREATE INDEX IF NOT EXISTS idx_reviews_owner_repo_pr
    ON backend_reviews(owner, repo, pr_number);
CREATE INDEX IF NOT EXISTS idx_reviews_status
    ON backend_reviews(status);
`

const migrationReviewViolations = `
CREATE TABLE IF NOT EXISTS backend_review_violations (
    id UUID PRIMARY KEY,
    review_id UUID NOT NULL REFERENCES backend_reviews(id) ON DELETE CASCADE,
    file VARCHAR(1024) NOT NULL,
    line INTEGER NOT NULL,
    severity VARCHAR(20) NOT NULL,
    issue TEXT NOT NULL,
    suggestion TEXT,
    rule_id VARCHAR(255),
    rule_name VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_violations_review_id
    ON backend_review_violations(review_id);
`

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
