# AGENTS.md: AI-Driven Code Reviewer Multi-Agent System

This document defines the multi-agent architecture for the **Agnostic AI Reviewer**. The system transitions from a linear script to an **Agentic Workflow**, where specialized agents handle distinct parts of the review lifecycle to ensure high-signal feedback and cost efficiency.

---

## 1. System Philosophy
To remain **Agnostic** and **Modular**, the system decouples the "How to fetch data" (Platform) from the "How to think" (AI). Each agent is a logical entity that can be powered by different LLMs (Gemini, OpenAI, or Local Models) depending on task complexity and budget.



---

## 2. Agent Definitions

### A. The Scout (Context & Ingestion Agent)
**Role:** Data Gatherer & Context Optimizer  
**Objective:** Provide the most relevant, high-signal context to the brain while minimizing token noise.
* **Responsibilities:**
    * Extracting code `diffs` from the Orchestrator (GitHub/GitLab).
    * Identifying "Hot Files" (e.g., if a Service changes, fetch the corresponding Interface or DTO).
    * Pruning irrelevant files (lockfiles, assets, vendor, or auto-generated code).
    * Mapping the PRD/Issue requirements to the specific code changes.
* **Target Model:** Gemini 1.5 Flash (Optimized for speed and high context window).

### B. The Architect (Senior Technical Reviewer)
**Role:** Logic, Safety, & Standards Validator  
**Objective:** Evaluate code against senior-level backend engineering standards (Go/Rust focus).
* **Responsibilities:**
    * **Concurrency & Safety:** Detecting race conditions, improper mutex usage, or goroutine leaks.
    * **Architecture Compliance:** Ensuring code adheres to Clean Architecture, SOLID, or DRY principles.
    * **Performance:** Identifying N+1 queries, inefficient loops, or unnecessary memory allocations.
    * **Security:** Spotting OWASP vulnerabilities or insecure sensitive data handling.
* **Target Model:** Gemini 1.5 Pro or GPT-4o (Optimized for deep reasoning).

### C. The Diplomat (Feedback & Reporting Agent)
**Role:** Communication & Formatting Specialist  
**Objective:** Translate technical findings into constructive, human-readable feedback.
* **Responsibilities:**
    * Aggregating raw technical issues from **The Architect**.
    * Formatting inline comments using Markdown and Platform-specific syntax (e.g., GitHub's `> [!IMPORTANT]`).
    * Determining the final `Verdict` (`APPROVE`, `COMMENT`, or `REQUEST_CHANGES`).
    * Ensuring a professional, helpful, and concise tone of voice.
* **Target Model:** Gemini 1.5 Flash (Optimized for instruction following).

---

## 3. The Orchestration Pipeline

The system follows an agnostic sequece managed by a Runner (Golang/CLI):

1.  **Trigger:** Orchestrator (e.g., GitHub Action) starts the binary.
2.  **Phase 1 (Ingestion):** **The Scout** prepares the `Payload` (Diff + Metadata).
3.  **Phase 2 (Analysis):** **The Architect** processes the `Payload` using specialized System Instructions.
4.  **Phase 3 (Refinement):** **The Diplomat** sanitizes the output and maps it to the Platform's API schema.
5.  **Completion:** The Runner posts the results back to the Pull Request.

---

## 4. Communication Schema (Standardized Review JSON)

To maintain **AI-Agnosticism**, all agents communicate using this standardized structure:

```json
{
  "agent_metadata": {
    "id": "architect_v1",
    "model": "gemini-1.5-pro"
  },
  "summary": {
    "verdict": "REQUEST_CHANGES",
    "overall_logic_score": 75
  },
  "reviews": [
    {
      "file": "internal/service/auth.go",
      "line": 88,
      "severity": "CRITICAL",
      "issue": "Potential nil pointer dereference if user is not found.",
      "suggestion": "Add an explicit nil check before accessing `user.ID`."
    }
  ]
}
