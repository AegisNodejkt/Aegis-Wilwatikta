# 🛡️ Aegis-Wilwatikta: The Next-Gen Agentic Code Reviewer

[![Go Report Card](https://goreportcard.com/badge/github.com/aegis-wilwatikta/ai-reviewer)](https://goreportcard.com/report/github.com/aegis-wilwatikta/ai-reviewer)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/github/go-mod/go-version/aegis-wilwatikta/ai-reviewer)](https://golang.org)
[![PR Reviews](https://img.shields.io/badge/Reviewer-Agentic-blueviolet)](https://github.com/aegis-wilwatikta/ai-reviewer)

**Stop reviewing code like it's 2023.** Aegis-Wilwatikta is a high-performance, multi-agent AI reviewer built for modern engineering teams. It doesn't just "read" your code; it **understands** it using Graph-RAG (Neo4j) and an elite squad of specialized AI agents.

[Get Started](docs/GETTING_STARTED.md) • [How it Works](docs/ARCHITECTURE.md) • [Configuration](docs/CONFIGURATION.md) • [Contributing](docs/CONTRIBUTING.md)

---

## 🚀 Why Aegis-Wilwatikta?

Modern development moves fast. Traditional AI reviewers often suffer from "context-blindness" or "hallucination noise." Aegis-Wilwatikta solves this with:

- **🤖 Agentic Orchestration:** A specialized pipeline where agents (**Scout**, **Architect**, **Diplomat**) handle distinct parts of the review lifecycle.
- **🕸️ Graph-RAG (Neo4j):** We map your entire codebase into a graph database to understand dependencies, side effects, and architectural impact.
- **🔌 Fully Agnostic:** Switch between **Gemini 1.5 Pro/Flash**, **GPT-4o**, or your local models. Deploy on **GitHub**, **GitLab**, or run it **Locally**.
- **🎯 High-Signal Feedback:** Catch concurrency bugs, security leaks, and architectural drifts while ignoring the "nitpick noise."

---

## 👥 Meet The Squad

We’ve decoupled the "thinking" from the "doing." Each agent has a specific persona and mission:

| Agent | Role | Focus |
| :--- | :--- | :--- |
| **🔍 The Scout** | Data Gatherer | Context optimization, pruning lockfiles, and identifying "Hot Files" impacted by changes. |
| **🏗️ The Architect** | Senior Reviewer | Deep reasoning on logic, safety (concurrency/race conditions), and Clean Architecture compliance. |
| **📜 The Diplomat** | Communication | Formatting feedback into actionable, professional, and human-friendly reviews. |

---

## 🏛️ Philosophical Roots

- **Aegis:** The shield of the gods, symbolizing the protection and safety we bring to your codebase.
- **Wilwatikta:** The formal name of the Majapahit Empire, reflecting our ambition to build a strong, expansive, and enduring foundation for AI-native engineering.

---

## ⚡ Quick Start (GitHub Action)

Add this to your `.github/workflows/ai-review.yml`:

```yaml
name: AI Code Review
on:
  pull_request:
    types: [opened, synchronize]

jobs:
  review:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Aegis Review
        uses: aegis-wilwatikta/ai-reviewer@v1
        env:
          GEMINI_API_KEY: ${{ secrets.GEMINI_API_KEY }}
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          NEO4J_URI: ${{ secrets.NEO4J_URI }} # Optional for Graph-RAG
```

[Read the Full Installation Guide →](docs/GETTING_STARTED.md)

---

## 🤝 Contributing

We love contributors! Check out our [Contributing Guidelines](docs/CONTRIBUTING.md) to get started.

---

<p align="center">
  Built with ❤️ for the Agentic Future.
</p>
