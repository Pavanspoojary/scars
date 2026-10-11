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

