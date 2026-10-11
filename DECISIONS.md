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

