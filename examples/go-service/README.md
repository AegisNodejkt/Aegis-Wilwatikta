# 🚀 Aegis-Wilwatikta: Example Go Project

This directory contains a sample Go project designed to demonstrate the power of **Aegis-Wilwatikta** in action. It purposefully includes common (and some subtle) security and concurrency issues for the AI agents to catch!

---

## 🏗️ The Setup

### 1. Spin up the "Brain" (Graph-RAG)
To see Aegis-Wilwatikta perform deep architectural analysis, you'll need a local Neo4j instance. We've included a `docker-compose.yaml` to make this easy:

```bash
docker compose up -d
```

### 2. Prepare Your Environment
Ensure you have your API keys ready (from the root of the `ai-reviewer` repo):

```bash
export GEMINI_API_KEY="your-gemini-key"
export NEO4J_URI="bolt://localhost:7687"
export NEO4J_USER="neo4j"
export NEO4J_PASS="password"
```

### 3. Build & Index the Codebase
Aegis needs to "learn" about this project first:

```bash
# Build the indexer from the root repo
go build -o ../../bin/indexer ../../cmd/indexer

# Index this example project
../../bin/indexer --project-id=example-go-service .
```

---

## 🛡️ Run the Review

Now, pretend you've just made changes and want a local review:

```bash
# Build the CLI from the root repo
go build -o ../../bin/aegis ../../cmd/cli

# Run a local review
export PLATFORM="local"
../../bin/aegis
```

---

## 🔍 What to Look For

The `internal/auth/auth.go` and `cmd/server/main.go` files contain several "Easter Eggs" for Aegis-Wilwatikta to find:
- **Security:** Plain-text password comparisons.
- **Safety:** Sensitive data (tokens) exposed in response headers.
- **Concurrency:** Potential race conditions in map access.

**Happy Reviewing!**
