-- M1.2: Create tables for git integrations, webhook events, and PR ingestion jobs.
-- Tech doc §4.1 (Data Schema)

-- Enable UUID generation
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

--------------------------------------------------------------------------------
-- 1. project_git_integrations
-- Stores per-project git provider (GitHub/GitLab) configurations.
--------------------------------------------------------------------------------
CREATE TABLE project_git_integrations (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id       UUID NOT NULL,
    project_id      UUID NOT NULL,
    provider        VARCHAR(32)  NOT NULL CHECK (provider IN ('github', 'gitlab')),
    repository_url  TEXT         NOT NULL,
    webhook_secret  TEXT         NOT NULL DEFAULT '',
    install_id      TEXT         NOT NULL DEFAULT '',
    access_token    TEXT         NOT NULL DEFAULT '',
    refresh_token   TEXT         NOT NULL DEFAULT '',
    default_branch  VARCHAR(255) NOT NULL DEFAULT 'main',
    is_active       BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Index for tenant+project scoped lookups
CREATE INDEX idx_git_integrations_tenant_project
    ON project_git_integrations (tenant_id, project_id);

-- Index for active integrations
CREATE INDEX idx_git_integrations_active
    ON project_git_integrations (is_active) WHERE is_active = TRUE;

--------------------------------------------------------------------------------
-- 2. git_webhook_events
-- Stores incoming webhook payloads with idempotency for dedup.
--------------------------------------------------------------------------------
CREATE TABLE git_webhook_events (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id        UUID                    NOT NULL,
    integration_id   UUID                    NOT NULL REFERENCES project_git_integrations(id) ON DELETE CASCADE,
    idempotency_key  TEXT                    NOT NULL,
    event_type       VARCHAR(128)            NOT NULL,
    payload          JSONB                   NOT NULL DEFAULT '{}',
    status           VARCHAR(32)             NOT NULL DEFAULT 'received'
                     CHECK (status IN ('received', 'processing', 'processed', 'failed')),
    processed_at     TIMESTAMPTZ,
    error            TEXT                    DEFAULT '',
    created_at       TIMESTAMPTZ             NOT NULL DEFAULT NOW()
);

-- UNIQUE index for idempotency key (deduplication)
CREATE UNIQUE INDEX idx_webhook_events_idempotency
    ON git_webhook_events (idempotency_key);

-- Index for tenant-scoped lookups
CREATE INDEX idx_webhook_events_tenant
    ON git_webhook_events (tenant_id);

-- Index for status-based queries
CREATE INDEX idx_webhook_events_status
    ON git_webhook_events (status);

-- Index for integration lookups
CREATE INDEX idx_webhook_events_integration
    ON git_webhook_events (integration_id);

--------------------------------------------------------------------------------
-- 3. pr_ingestion_jobs
-- Tracks PR processing jobs spawned from webhook events.
--------------------------------------------------------------------------------
CREATE TABLE pr_ingestion_jobs (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id       UUID                    NOT NULL,
    integration_id  UUID                    NOT NULL REFERENCES project_git_integrations(id) ON DELETE CASCADE,
    event_id        UUID                    NOT NULL REFERENCES git_webhook_events(id) ON DELETE CASCADE,
    repository      VARCHAR(512)            NOT NULL,
    pr_number       INTEGER                 NOT NULL,
    status          VARCHAR(32)             NOT NULL DEFAULT 'queued'
                    CHECK (status IN ('queued', 'running', 'completed', 'failed', 'cancelled')),
    error           TEXT                    DEFAULT '',
    retry_count     INTEGER                 NOT NULL DEFAULT 0,
    max_retries     INTEGER                 NOT NULL DEFAULT 3,
    created_at      TIMESTAMPTZ             NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ             NOT NULL DEFAULT NOW()
);

-- Index for status-based queue polling (critical for worker performance)
CREATE INDEX idx_ingestion_jobs_status
    ON pr_ingestion_jobs (status);

-- Index for tenant-scoped lookups
CREATE INDEX idx_ingestion_jobs_tenant
    ON pr_ingestion_jobs (tenant_id);

-- Index for event lookups
CREATE INDEX idx_ingestion_jobs_event
    ON pr_ingestion_jobs (event_id);

-- Index for integration + PR lookups
CREATE INDEX idx_ingestion_jobs_integration_pr
    ON pr_ingestion_jobs (integration_id, pr_number);
