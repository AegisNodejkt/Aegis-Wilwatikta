# 🏁 Getting Started with Aegis-Wilwatikta

Welcome to the future of AI-assisted engineering! This guide will help you get Aegis-Wilwatikta up and running in minutes.

---

## 🛠️ Choose Your Path

Aegis-Wilwatikta is designed to be flexible. You can run it as a **GitHub Action** for automated reviews, or as a **CLI tool** for local development.

### 1. GitHub Action (Recommended)

The easiest way to integrate Aegis into your workflow.

1.  **Add Secrets:** In your GitHub repository, go to `Settings > Secrets and variables > Actions` and add:
    - `GEMINI_API_KEY` (or `OPENAI_API_KEY`)
    - `NEO4J_URI`, `NEO4J_USER`, `NEO4J_PASS` (Optional, for Graph-RAG features)
2.  **Create Workflow:** Add `.github/workflows/ai-review.yml`:

```yaml
name: AI Code Review
on:
  pull_request:
    types: [opened, synchronize]

jobs:
  review:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      pull-requests: write
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0 # Important for diff calculation
      - name: Aegis Review
        uses: aegis-wilwatikta/ai-reviewer@main
        env:
          GEMINI_API_KEY: ${{ secrets.GEMINI_API_KEY }}
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

---

### 2. Local CLI (Solo Developers)

Perfect for a pre-push "sanity check" of your changes.

#### Prerequisites
- [Go 1.25+](https://golang.org/dl/)
- (Optional) [Docker](https://www.docker.com/) for running Neo4j locally.

#### Installation
```bash
# Clone the repository
git clone https://github.com/aegis-wilwatikta/ai-reviewer.git
cd ai-reviewer

# Build the binaries
go build -o bin/aegis ./cmd/cli
go build -o bin/indexer ./cmd/indexer
```

#### Running a Local Review
1.  **Set Environment Variables:**
    ```bash
    export GEMINI_API_KEY="your-key-here"
    export PLATFORM="local"
    ```
2.  **Run the Reviewer:**
    ```bash
    ./bin/aegis
    ```
    *Note: The local platform uses `git diff main...HEAD` by default.*

---

### 3. Enabling Graph-RAG (The "Pro" Setup)

To give Aegis-Wilwatikta a "brain" that remembers your entire codebase:

1.  **Spin up Neo4j:**
    ```bash
    docker run -d --name neo4j -p 7474:7474 -p 7687:7687 \
      -e NEO4J_AUTH=neo4j/password neo4j:5
    ```
2.  **Index your codebase:**
    ```bash
    export NEO4J_URI="bolt://localhost:7687"
    export NEO4J_USER="neo4j"
    export NEO4J_PASS="password"
    ./bin/indexer --project-id=my-project .
    ```
3.  **Enable RAG in `.ai-reviewer.yaml`:**
    ```yaml
    rag:
      enabled: true
      connection_url: "bolt://localhost:7687"
    ```

---

## 🧩 Configuration

Aegis looks for a `.ai-reviewer.yaml` file in your project root.

```yaml
provider: gemini
project_id: my-awesome-project
gemini_model: gemini-1.5-flash
base_branch: main
ignore_paths:
  - "vendor/**"
  - "**/mock_*.go"
```

[See the full Configuration Reference →](CONFIGURATION.md)
