# 🛡️ Aegis-Wilwatikta: The Agnostic Agentic Code Reviewer

[![Go Report Card](https://goreportcard.com/badge/github.com/aegis-wilwatikta/ai-reviewer)](https://goreportcard.com/report/github.com/aegis-wilwatikta/ai-reviewer)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/github/go-mod/go-version/aegis-wilwatikta/ai-reviewer)](https://golang.org)

**Stop reviewing code like it's 2023.** Aegis-Wilwatikta is a next-gen, multi-agent AI reviewer designed for modern software houses and engineering teams. It doesn't just "read" your code; it **understands** it using Graph-RAG and an elite squad of specialized AI agents.

---

## 🚀 Why Aegis-Wilwatikta?

Modern development moves fast. Traditional AI reviewers often suffer from "context-blindness" or "hallucination noise." Aegis-Wilwatikta solves this with:

- **🤖 Agentic Orchestration:** A specialized pipeline where agents (Scout, Architect, Diplomat) handle distinct parts of the review lifecycle.
- **🕸️ Graph-RAG (Neo4j):** We don't just look at the diff. We map your entire codebase into a graph database to understand dependencies, side effects, and architectural impact.
- **🔌 Fully Agnostic:** Switch between **Gemini 1.5 Pro/Flash**, **GPT-4o**, or your local models. Deploy on **GitHub**, **GitLab**, or run it **Locally**.
- **🎯 High-Signal Feedback:** Optimized to catch concurrency bugs, security leaks, and architectural drifts while ignoring the "nitpick noise."

---

## 👥 Meet The Squad

We’ve decoupled the "thinking" from the "doing." Each agent has a specific persona and mission:

| Agent | Role | Focus |
| :--- | :--- | :--- |
| **🔍 The Scout** | Data Gatherer | Context optimization, pruning lockfiles, and identifying "Hot Files" impacted by changes. |
| **🏗️ The Architect** | Senior Reviewer | Deep reasoning on logic, safety (concurrency/race conditions), and Clean Architecture compliance. |
| **📜 The Diplomat** | Communication | Formatting feedback into actionable, professional, and human-friendly reviews. |

---

## 🛠️ Tech Stack

Built with performance and modularity in mind:
- **Core:** Go (Golang)
- **Brain:** Multi-Model Support (Gemini, OpenAI)
- **Memory:** Neo4j (Graph Database) + Vector Embeddings
- **Parser:** Tree-sitter (for structural API analysis)
- **Architecture:** Hexagonal (Ports & Adapters)

---

## 🚦 Getting Started

### 1. Installation
Aegis-Wilwatikta is primarily designed to run as a **GitHub Action**, but can also be used as a CLI tool.

```bash
# Clone the repository
git clone https://github.com/aegis-wilwatikta/ai-reviewer.git
cd ai-reviewer

# Build the CLI and Indexer
go build -o aegis ./cmd/cli
go build -o indexer ./cmd/indexer
```

### 2. Indexing your codebase (Graph-RAG)
Before the agents can perform deep architectural reviews, you need to seed your Graph database:

```bash
# Seed the Neo4j graph with your repository structure
./indexer seed --path .
```

### 3. Configuration
Create a `.ai-reviewer.yaml` in your root directory:

```yaml
provider: gemini
gemini_model: gemini-1.5-flash
openai_model: gpt-4o-mini
base_branch: main
ignore_paths:
  - "vendor/**"
  - "**/mock_*.go"
```

### 4. Environment Variables
Ensure the following are set in your environment or GitHub Secrets:
- `GEMINI_API_KEY` (or `OPENAI_API_KEY`)
- `NEO4J_URI`, `NEO4J_USER`, `NEO4J_PASS` (for Graph-RAG features)
- `GITHUB_TOKEN`

---

## 🏛️ Philosophical Roots

**Aegis** represents the shield of the gods, symbolizing protection and the safety we bring to your codebase.
**Wilwatikta** is the formal name of the Majapahit Empire, reflecting our ambition to build a strong, expansive, and enduring foundation for AI-native engineering.

---

## 🤝 Contributing

We love contributors! Whether it's adding support for a new LLM provider or improving the Graph-RAG parser, check out our `CONTRIBUTING.md` (coming soon) and `ARCHITECTURE` documents.

---

## 📄 License

Distributed under the MIT License. See `LICENSE` for more information.

---

<p align="center">
  Built with ❤️ for the Agentic Future.
</p>
