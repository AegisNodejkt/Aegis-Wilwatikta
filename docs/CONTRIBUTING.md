# 🤝 Contributing to Aegis-Wilwatikta

First off, thank you for considering contributing to Aegis-Wilwatikta! It's people like you who make this project a great tool for the community.

---

## 🛣️ Roadmap

- [ ] Support for **GitLab** and **Bitbucket**.
- [ ] More Language Parsers (Rust, Python, TypeScript are next).
- [ ] Local Graph-DB support with **SQLite/DuckDB**.
- [ ] Custom Agent Personas via `.ai-reviewer.yaml`.

---

## 🛠️ Development Setup

### Prerequisites
- **Go 1.25+**
- **Docker** (for Neo4j)
- A **Gemini API Key** (or OpenAI)

### 1. Clone & Build
```bash
git clone https://github.com/aegis-wilwatikta/ai-reviewer.git
cd ai-reviewer
go build -o bin/aegis ./cmd/cli
go build -o bin/indexer ./cmd/indexer
```

### 2. Run Neo4j
```bash
docker run -d --name aegis-neo4j -p 7474:7474 -p 7687:7687 -e NEO4J_AUTH=neo4j/password neo4j:5
```

---

## 🧪 Running Tests

We value high-quality tests. Before submitting a PR, please ensure all tests pass:

```bash
go test ./...
```

---

## 📜 Pull Request Guidelines

1.  **Atomic Commits:** Keep your changes focused. One feature/fix per PR.
2.  **Documentation:** If you add a new feature, update the relevant files in `docs/`.
3.  **Consistency:** Follow the existing Go coding style (run `go fmt`).
4.  **Agent Logic:** If you modify an agent's prompt, provide examples of the before/after behavior in your PR description.

---

## ⚖️ Code of Conduct

We are committed to fostering a welcoming and inclusive community. Please be respectful and constructive in all interactions.

---

## 💎 License

By contributing, you agree that your contributions will be licensed under the **MIT License**.
