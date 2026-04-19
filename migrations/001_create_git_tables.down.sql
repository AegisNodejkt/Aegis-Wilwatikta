-- M1.2: Drop all git integration tables in reverse dependency order.

DROP TABLE IF EXISTS pr_ingestion_jobs;
DROP TABLE IF EXISTS git_webhook_events;
DROP TABLE IF EXISTS project_git_integrations;
