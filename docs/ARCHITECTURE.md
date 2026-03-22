# 🏛️ Architecture & Philosophy

Aegis-Wilwatikta is built on the principle of **Agentic Decoupling**. We believe that reviewing code isn't a single task, but a workflow that requires different specialized "minds."

---

## 🧩 High-Level Design

The system follows a **Hexagonal Architecture** (Ports and Adapters). This ensures that the core logic is completely decoupled from the CI/CD platform (GitHub/GitLab) and the AI provider (Gemini/OpenAI).

### 🏗️ The Core Engine
The `Engine` coordinates the lifecycle of a review. It doesn't know *how* to talk to GitHub or *how* to generate a response; it simply orchestrates the agents and the platform adapters.

### 🔌 Adapters
- **Platform Adapters:** Handle communication with Git providers (fetching diffs, posting comments).
- **AI Adapters:** Standardize the interface for different LLMs, ensuring the same prompt logic works across providers.

---

## 🤖 The Multi-Agent System

We use a "Chain of Agents" pattern to ensure high-quality reviews:

### 1. 🔍 The Scout (Context Ingestion)
- **The Problem:** LLMs have limited context windows (or get "lost in the middle").
- **The Solution:** The Scout doesn't just read the diff. It uses **Graph-RAG** to find "Hot Files"—related files that are impacted by the change but not included in the diff. It prunes the noise (lockfiles, assets) and prepares a high-signal payload.

### 2. 🏗️ The Architect (Technical Reasoning)
- **The Problem:** Generalist AI often nitpicks style but misses deep bugs like race conditions or architectural violations.
- **The Solution:** The Architect is given a "Senior Backend Engineer" persona. It focuses on concurrency, safety, Clean Architecture compliance, and performance. It produces a raw JSON of findings.

### 3. 📜 The Diplomat (Refinement & Delivery)
- **The Problem:** Raw AI output can be blunt, incorrectly formatted, or hallucinate line numbers.
- **The Solution:** The Diplomat reviews the Architect's findings, ensures they are mapped to the correct line numbers, and formats them into professional, actionable feedback. It also determines the final verdict (`APPROVE` vs `REQUEST_CHANGES`).

---

## 🕸️ Graph-RAG (The Memory)

Aegis-Wilwatikta uses a **Neo4j Graph Database** to store an AST-level representation of your codebase.

- **Nodes:** Files, Functions, Structs, Interfaces, Methods.
- **Relationships:** `CALLS`, `IMPLEMENTS`, `DEFINES`, `IMPORT`.

When a PR is opened, the system queries the graph: *"What interfaces are implemented by this changed struct?"* or *"What functions call this modified method?"*. This context is fed to the Architect, allowing it to spot breaking changes that a standard diff-only reviewer would miss.

---

## 🚀 Performance & Scalability

- **Language:** Written in **Go** for near-instant startup times (crucial for CI/CD).
- **Concurrent Indexing:** The `indexer` uses a worker pool to parse and embed large repositories in parallel.
- **Incremental Sync:** We use structural signature hashing to only re-index files that have actually changed.
