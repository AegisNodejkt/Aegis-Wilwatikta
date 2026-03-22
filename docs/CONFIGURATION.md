# ⚙️ Configuration Reference

Aegis-Wilwatikta can be customized via a `.ai-reviewer.yaml` file and environment variables.

---

## 📄 `.ai-reviewer.yaml`

Place this file in your repository root to configure the reviewer's behavior.

| Field | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `provider` | string | `gemini` | AI provider to use (`gemini` or `openai`). |
| `project_id` | string | (Auto) | Unique ID for your project (used for Graph-RAG isolation). |
| `gemini_model` | string | `gemini-1.5-flash` | The model used for Scout and Diplomat agents. |
| `openai_model` | string | `gpt-4o-mini` | The model used for Scout and Diplomat agents when using OpenAI. |
| `base_branch` | string | `main` | The branch to compare against for local reviews. |
| `ignore_paths` | list | `[]` | Glob patterns of files to exclude from reviews (e.g., `vendor/**`). |
| `rag.enabled` | boolean | `false` | Enable Graph-RAG features. |
| `rag.connection_url` | string | `""` | The URL of your Neo4j instance. |
| `rag.embedding_provider`| string | `google` | Provider for embeddings (`google` or `openai`). |

### Example
```yaml
provider: gemini
project_id: aegis-core
gemini_model: gemini-1.5-flash
base_branch: develop
ignore_paths:
  - "third_party/**"
  - "docs/**"
rag:
  enabled: true
  connection_url: "bolt://neo4j.example.com:7687"
  embedding_provider: google
```

---

## 🔐 Environment Variables

Sensitive data and system overrides are managed via environment variables.

### Required (Depending on Provider)
- `GEMINI_API_KEY`: Required if using `provider: gemini`.
- `OPENAI_API_KEY`: Required if using `provider: openai`.
- `GITHUB_TOKEN`: Required for posting comments on GitHub.

### Graph-RAG (Optional)
- `NEO4J_URI`: The bolt/neo4j URL.
- `NEO4J_USER`: Database username.
- `NEO4J_PASS`: Database password.
- `NEO4J_DATABASE`: Database name (defaults to `neo4j`).

### Agent Overrides
You can force specific models for each agent:
- `SCOUT_MODEL`: Model ID for the Scout agent.
- `ARCHITECT_MODEL`: Model ID for the Architect agent.
- `DIPLOMAT_MODEL`: Model ID for the Diplomat agent.

---

## 🕵️ Choosing Models

We recommend the following "Gold Standard" setups:

| Tier | Scout (Context) | Architect (Logic) | Diplomat (Formatting) |
| :--- | :--- | :--- | :--- |
| **High Performance** | `gemini-1.5-flash` | `gemini-1.5-pro` | `gemini-1.5-flash` |
| **OpenAI Equivalent** | `gpt-4o-mini` | `gpt-4o` | `gpt-4o-mini` |

*Note: The Architect agent always defaults to the most powerful model ("pro" or "gpt-4o") unless overridden.*
