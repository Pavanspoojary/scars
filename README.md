# GitScars (`git-scars`)

[![Go Reference](https://pkg.go.dev/badge/github.com/Pavanspoojary/si-scars.svg)](https://pkg.go.dev/github.com/Pavanspoojary/si-scars)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/Pavanspoojary/si-scars)](https://goreportcard.com/report/github.com/Pavanspoojary/si-scars)

**GitScars** is a Git-native failure memory and regression guardrail system for developers and AI coding agents.

It records lessons learned from production failures, subtle bugs, and edge cases as versioned, human-reviewable Markdown documents anchored directly to repository code locations. When developers or coding agents touch those files or encounter error signatures, GitScars deterministically retrieves the relevant lessons before mistakes repeat.

---

## The Problem

Repository knowledge often disappears into old commits, closed pull requests, Slack threads, and ephemeral agent transcripts. As codebases grow:
- **Monolithic Instruction Files (`AGENTS.md`, `CLAUDE.md`)** suffer from prompt bloat, attention dilution ("lost in the middle"), and cannot bind lessons to specific file versions or symbol anchors.
- **Vector Memory (RAG)** introduces semantic hallucinations, noisy false-positive retrieval (see [VibeMemBench arXiv:2609.23570](https://arxiv.org/abs/2609.23570)), and is vulnerable to prompt poisoning ([MemSecBench arXiv:2607.27080](https://arxiv.org/abs/2607.27080)).
- **Silent Code Drift**: Code changes over time. When files change, lessons written months ago become stale or invalid, but flat files provide no way to know.

---

## Core Product Principles

1. **Evidence is Separate from Trust**: A test verifier passing does not mean a lesson is approved. Automated evidence, human review, and lifecycle state remain strictly decoupled.
2. **Dynamic Freshness Without Mutation**: Code drift is calculated on the fly by hashing current files against recorded baselines. Read-only checks never mutate repository files.
3. **Deterministic Retrieval with Abstention**: Returns top $\le 3$ relevant Scars based on explicit anchors (files, symbols, dependency lockfiles, error signatures, commands). If match confidence is low, it abstains with an empty result.
4. **Untrusted Data by Design**: Scar content is treated as untrusted data. Verifiers execute as argv arrays (never raw shell), in scrubbed environments, with process-tree timeouts and local SHA-256 command-hash allowlists.
5. **Zero Cloud Dependencies**: Pure Go single static binary. Works completely offline without external APIs, model providers, or vector databases.

---

## Installation & Build

Requires Go 1.22+:

```bash
git clone https://github.com/Pavanspoojary/si-scars.git
cd si-scars
go build -o git-scars ./cmd/git-scars
```

Optionally move `git-scars` into your `$PATH` (e.g. `/usr/local/bin`). Once in your path, you can invoke it as `git scars <subcommand>`.

---

## Quickstart Workflow

### 1. Initialize GitScars
```bash
git-scars init
```
Creates `.gitscars/scars/` for versioned lessons and `.gitscars/.local/` (automatically gitignored) for derived local state.

### 2. Capture a Failure Lesson
Create a new draft manually:
```bash
git-scars add \
  --title "Nil pointer dereference in auth context" \
  --lesson "Always use comma-ok idiom when reading user profile from request context" \
  --files "pkg/auth/context.go"
```

Or capture automatically from an existing fix commit (without requiring an LLM):
```bash
git-scars add from-commit 8a1b3f9
```
*Prefills changed files, parent base commit, fix commit, and inferred test commands into a new draft.*

### 3. Record Verification Evidence
Execute the verifier safely against the codebase:
```bash
git-scars verify nil-pointer-dereference-8a1b3f --interactive
```
Computes command hashes, scrubs the environment, redacts secrets, and tests the worktree.

### 4. Review and Promote
Human review is required before a Scar can become active:
```bash
git-scars promote nil-pointer-dereference-8a1b3f \
  --status active \
  --reviewer "alice" \
  --rationale "Reviewed test reproduction in PR #42"
```

### 5. Selective Retrieval
Retrieve relevant lessons during coding tasks:
```bash
# By target file
git-scars recall --files pkg/auth/context.go

# Diff-aware retrieval (against a branch or commit)
git-scars recall --diff origin/main

# Failure-aware retrieval from failing test stderr
go test ./... 2>&1 | git-scars recall --stdin

# Machine-readable output for coding agents
git-scars recall --files pkg/auth/context.go --json
```

### 6. Read-Only Freshness Check in CI
Add to your CI workflow to ensure no active Scars have stale anchors:
```bash
git-scars check
```
Stable Exit Codes:
- `0`: All Scars valid and current.
- `1`: Invalid format or invariant violation.
- `2`: Stale anchor detected (code modified since baseline, requires reaffirmation).
- `3`: Operational error.

### 7. Git Pre-Commit Hook Guardrail
Install the Git pre-commit hook to catch stale active scars before staging and committing:
```bash
# Install hook into .git/hooks/pre-commit
git-scars hook install

# Inspect hook status
git-scars hook status

# Uninstall hook
git-scars hook uninstall
```

---

## File Format & Repository Storage

Each Scar is stored as a single, human-readable, Git-diffable Markdown file in `.gitscars/scars/<slug>-<shortid>.md` with strict YAML frontmatter:

```markdown
---
schema_version: "1"
id: "map-write-crash-391945"
title: "Concurrent map write in telemetry exporter"
lesson: "Metrics map must be protected by sync.RWMutex during RecordMetric calls."
anchors:
  files:
    - "pkg/telemetry/exporter.go"
  symbols:
    - "RecordMetric"
  commands:
    - "go test -race ./pkg/telemetry/..."
  error_signatures:
    - "fatal error: concurrent map writes"
verifier:
  result: "fail_to_pass"
  argv:
    - "go"
    - "test"
    - "-race"
    - "./pkg/telemetry/..."
  command_hash: "2d4b1ee683d6be106b1633c27fedb692d3f757a5aa049397e28ef1cbd6d52736"
baseline:
  recorded_at: "2026-10-11T08:00:00Z"
  pkg/telemetry/exporter.go: "a3b98f12c4..."
review:
  status: "approved"
  reviewer: "bob"
  date: "2026-10-11T08:05:00Z"
  rationale: "Reproduced with race detector in PR #104"
lifecycle:
  state: "active"
---

### Root Cause
Under heavy concurrent load, multiple goroutines called RecordMetric simultaneously without synchronization.

### Fix
Replaced raw map with sync.Map or guarded reads and writes with sync.RWMutex.
```

---

## Agent Integration Guide

Coding agents (Claude Code, Cursor, Windsurf, Antigravity) can query GitScars by invoking `git scars recall --json`:

```bash
git scars recall --files "$MODIFIED_FILES" --json
```

Returns versioned `reposcars.recall.v1` JSON:
```json
{
  "schema_version": "reposcars.recall.v1",
  "timestamp": "2026-10-11T08:00:00Z",
  "query": { "files": ["pkg/telemetry/exporter.go"], "threshold": 3, "top_k": 3 },
  "count": 1,
  "results": [
    {
      "id": "map-write-crash-391945",
      "title": "Concurrent map write in telemetry exporter",
      "lesson": "Metrics map must be protected by sync.RWMutex during RecordMetric calls.",
      "score": 5,
      "freshness": "current",
      "matches": ["file(+5)"],
      "explanation": "file(+5)",
      "anchors": { "files": ["pkg/telemetry/exporter.go"] }
    }
  ]
}
```

### Model Context Protocol (MCP) Server

GitScars provides a built-in stdio Model Context Protocol (MCP) server for native integration with coding assistants:

```bash
git-scars mcp
```

The MCP server exposes three read-only tools:
- `list_scars`: List all repository scars with their lifecycle and dynamic freshness status.
- `get_scar`: Retrieve full frontmatter, anchors, verifier results, and markdown body by scar ID.
- `recall_scars`: Deterministically retrieve matching scars for target files, symbols, error messages, and commands.

#### Client Configuration (Claude Desktop, Cursor, Antigravity, Windsurf)

Add to your assistant's MCP configuration (e.g. `claude_desktop_config.json` or `.cursor/mcp.json`):

```json
{
  "mcpServers": {
    "git-scars": {
      "command": "git-scars",
      "args": ["mcp"]
    }
  }
}
```

### Dependency Lockfile Anchors

GitScars automatically inspects repository dependency lockfiles to anchor lessons to exact dependency versions without manual specification:
- **Go**: Parses direct dependencies and versions from `go.mod`.
- **Node.js**: Parses packages and versions from `package-lock.json`.

When dependencies are bumped or downgraded, Scars anchored to vulnerable or breaking versions automatically trigger match signals during recall.

---

## Security & Privacy Baseline

- **Verifier Execution**: Verifiers are represented as `argv` arrays and executed without shell interpolation by default.
- **Environment Scrubbing**: Only a minimal set of environment variables (`PATH`, `HOME`, `LANG`, `TMPDIR`, `USER`) are passed into verifier sub-processes.
- **Secret Redaction**: Automated redaction strips Bearer tokens, private keys, and common password/API keys before recording excerpts.
- **Zero Network Telemetry**: Feedback is stored strictly in the untracked `.gitscars/.local/feedback.jsonl` file.

---

## Documentation

- [PRIOR_ART.md](PRIOR_ART.md): Primary-source comparative analysis (`Daily-Nerd/Scar`, `fscars`, `gitmem`, arXiv:2609.23570).
- [EVAL_PLAN.md](EVAL_PLAN.md): Preregistered benchmark evaluation plan, scoring baseline weights, and power calculations.
- [DECISIONS.md](DECISIONS.md): Architecture Decision Records (ADRs).
