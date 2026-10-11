# Architecture Decision Records (ADR)

This document tracks significant design choices, trade-offs, evaluated alternatives, and rationale for `si-scars`.

---

## ADR-0001: Repository Foundation and Governance Baseline

- **Date**: 2026-10-11
- **Status**: Accepted
- **Context**: Setting up repository standards, CI workflow, agent instructions, and contribution guidelines before initiating product implementation.
- **Decision**:
  - Establish `AGENTS.md` for AI agent behavioral rules and `CONTRIBUTING.md` for human contributors to avoid duplication.
  - Require branch isolation (no direct commits to `main`), conventional commits, and pre-commit verification.
  - Implement GitHub Actions CI checking `gofmt`, `go vet`, `go test -race ./...`, and `go build ./...`.
- **Consequences**:
  - Ensures consistent formatting, reproducible builds, and clear separation of agent vs human guidance from day one.

---

## ADR-0002: Open Source License Selection

- **Date**: 2026-10-11
- **Status**: Accepted
- **Context**: Selecting the open-source license for `si-scars`. Evaluated Apache 2.0, MIT, and MPL-2.0.
- **Decision**:
  - Retain and confirm the **Apache License 2.0**.
- **Consequences**:
  - Provides a permissive open-source license with explicit patent protection grants and trademark restrictions suitable for developer tooling.

---

## ADR-0003: Namespace and Project Identity Selection

- **Date**: 2026-10-11
- **Status**: Accepted
- **Context**: Stage 0a namespace evaluation to avoid collisions across GitHub, package registries, and developer tooling (specifically `scar-cli`, `fscars`, and `gitmem`).
- **Decision**:
  - Adopt **`git-scars`** as the canonical CLI binary name and package namespace.
  - Adopt **`.gitscars/`** as the repository metadata directory containing committed scars (`.gitscars/scars/`) and gitignored local state (`.gitscars/.local/`).
  - Discard generic `reposcars` and naked `scar` to eliminate naming confusion with existing tools.
- **Consequences**:
  - Provides natural Git integration (`git scars ...`), clean packaging, and avoids collisions with existing Python `scar-cli` or `fscars`.

---

## ADR-0004: Git Pre-Commit Hook & Model Context Protocol (MCP) Server

- **Date**: 2026-10-11
- **Status**: Accepted
- **Context**: Providing friction-free guardrails for both human developers (pre-commit CLI checks) and AI coding assistants (Claude Desktop, Cursor, Antigravity) without requiring external daemon infrastructure or cloud services.
- **Decision**:
  - Standardize Git pre-commit hooks via `git-scars hook [install|uninstall|status]` using a POSIX shell script that invokes `git-scars check --diff HEAD`.
  - Implement a pure Go standard-library JSON-RPC 2.0 stdio MCP server (`git-scars mcp`) exposing `list_scars`, `get_scar`, and `recall_scars`.
- **Consequences**:
  - Developers receive warnings or blocks directly at commit time when touching code with active scars.
  - Coding agents can natively discover, inspect, and retrieve scars over stdio JSON-RPC without ad-hoc shell parsing.
  - Zero external dependencies; 100% Go standard library.

---

## ADR-0005: Dependency Lockfile Anchoring and Rebuildable Index Cache

- **Date**: 2026-10-11
- **Status**: Accepted
- **Context**: Production regressions frequently correlate with specific library versions (breaking changes, CVEs). Additionally, parsing hundreds of markdown files sequentially in large repositories could add unnecessary I/O overhead.
- **Decision**:
  - Implement zero-dependency lockfile parsing for Go (`go.mod`) and Node.js (`package-lock.json`), matching dependencies during retrieval.
  - Implement a rebuildable local cache `.gitscars/.local/index.json` generated via `git-scars index`.
- **Consequences**:
  - Lockfile version changes automatically match relevant scars during recall.
  - Index cache resides in gitignored `.local/`, preserving clean repository state while providing instant lookup summaries.

---

## ADR-0006: Multi-Ecosystem Lockfiles and Anchor Baseline Diff Inspector

- **Date**: 2026-10-11
- **Status**: Accepted
- **Context**: Developers need to inspect the precise unified line diff of modified anchor files before executing reaffirmations on changed scars. Additionally, Rust (`Cargo.lock`) and Python (`poetry.lock`, `requirements.txt`) projects require dependency anchoring without external tooling.
- **Decision**:
  - Implement zero-dependency TOML and requirements line scanners for Rust `Cargo.lock` and Python `poetry.lock` / `requirements.txt`.
  - Implement `git-scars diff <id>` to inspect differences between current working tree files and recorded baseline hashes/commits.
- **Consequences**:
  - Rust and Python projects seamlessly match version-dependent failure memories during recall.
  - Reviewers can inspect exact line-by-line drift prior to reaffirmation or re-verification.

---

## ADR-0007: Repository Statistics & Metrics Aggregation Command

- **Date**: 2026-10-11
- **Status**: Accepted
- **Context**: Repository maintainers, CI dashboards, and agents need visibility into overall memory system health: active vs draft scars, current vs stale anchors, monitored symbols, and recorded feedback utility rates.
- **Decision**:
  - Implement `git-scars stats` (and `git-scars stats --json`) to aggregate lifecycle distributions, dynamic freshness counts, anchor file/symbol tallies, and feedback satisfaction percentages from local state.
- **Consequences**:
  - Provides instant, zero-dependency visibility into repository regression guardrail coverage.
  - Generates structured JSON output for CI reporting and agent telemetry.

---

## ADR-0008: Repository Health Diagnostics (`git-scars doctor`) and Interactive Capture Wizard

- **Date**: 2026-10-11
- **Status**: Accepted
- **Context**: Users, CI pipelines, and autonomous coding agents require a fast, unified health check to diagnose Git environment readiness, directory structures, schema validity, lifecycle invariants, anchor freshness drift, pre-commit hook installations, verifier trust lists, and index cache integrity without running multiple disparate commands. Additionally, manual scar drafting needed a zero-friction interactive terminal mode (`git-scars add -i`) alongside commit-based drafting.
- **Decision**:
  - Implement `git-scars doctor` (`--json`) executing 8 distinct diagnostic probes across Git worktree, store structure, schemas/lint, invariants, drift, hooks, trust store, and index cache.
  - Implement interactive wizard mode for `git-scars add -i` using standard library `bufio.Scanner` prompts for title, lesson, and anchor files.
- **Consequences**:
  - Provides a single point of validation for developers and automated workflows.
  - Delivers actionable remediation messages for each warning (e.g. instructing exact commands for hook installation or reaffirmation).
  - Preserves pure Go standard library design with zero external dependencies.




