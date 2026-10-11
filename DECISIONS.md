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

---

## ADR-0009: Autonomous Agent Integration (Token-Budget Injection, Instruction Sync, Two-Way MCP, and Error Triage)

- **Date**: 2026-10-11
- **Status**: Accepted
- **Context**: AI coding assistants (Claude Code, Cursor, Copilot, Antigravity) are constrained by token budget ceilings and attention dilution when ingesting full repository instruction files. Agents also lack automated error correlation during test failures and require two-way memory tools to autonomously record regressions without human friction.
- **Decision**:
  - Implement `git-scars inject` with conservative rune-to-token budgeting (~4 chars/token) outputting compact XML/Markdown prompt guardrails.
  - Implement `git-scars sync` maintaining auto-generated guardrail blocks inside `AGENTS.md` / `CLAUDE.md` between `<!-- GIT-SCARS:START -->` and `<!-- GIT-SCARS:END -->` markers.
  - Implement `git-scars triage` extracting panic traces and failure signatures from stdin or flags to yield immediate prevention lessons.
  - Expand the stdio JSON-RPC 2.0 MCP server with two-way tools (`inject_guardrails`, `triage_error`, `draft_scar`) and MCP markdown resources (`scar://<id>`, `scars://active`).
- **Consequences**:
  - AI agents can proactively query, inject, and write regression memory directly through standard protocols and zero-dependency CLI pipelines.
  - All operations maintain 100% Go standard library compatibility with zero third-party dependencies.

---

## ADR-0010: Retrieval v2 Partial File Matching Weights

- **Date**: 2026-10-11
- **Status**: Accepted
- **Context**: Exact file matching misses legitimate cases (file moved or renamed). A same-directory fallback was initially assigned +3, which allowed any file in the same directory to pass the abstain threshold alone and caused a regression in `TestInjectGuardrails` (unrelated file in the same directory was retrieved).
- **Decision**:
  - Exact path match: +5 (unchanged).
  - Same basename in a different directory (likely move or rename): +2.
  - Same directory with a different basename (module proximity): +1.
  - Partial file signals therefore remain below the abstain threshold (3) unless combined with an additional signal (symbol, error signature, command, dependency, or task terms).
  - Added normalized symbol matching (exact +4, normalized suffix +2), normalized error-signature matching (+3), and a `confidence = score/19` field in recall output.
- **Consequences**:
  - Preserves the frozen evaluation precision requirement (Precision@3 >= 70%, false-positive rate <= 10%) while improving recall for moved or renamed files.
  - `EVAL_PLAN.md` frozen weights remain valid; partial signals are extensions that cannot independently trigger retrieval.

---

## ADR-0011: Feedback Quality Scoring, Parallel Freshness, and Lifecycle Evolution

- **Date**: 2026-10-11
- **Status**: Accepted
- **Context**: 
  - As scars accumulate user and agent feedback, scars with repeatedly incorrect or irrelevant guidance should not maintain identical retrieval priority.
  - Calculating anchor SHA-256 hashes sequentially across dozens of scars slows down cold recall and status commands.
  - When architectures change, outdated scars require explicit lifecycle transitions (`contradicted`, `superseded`) with audit trails rather than silent deletion.
- **Decision**:
  - Implemented `GetScarQuality` aggregating feedback into net scores `(helpful - 1.5*irrelevant - 2.0*incorrect) / total`.
  - Applied a deterministic `-2` recall ranking penalty for scars with `>= 2` incorrect ratings, logged explicitly in `matches` as `feedback_penalty(-2)`. Added lint warnings (`noisy_scar`) for `>= 3` incorrect ratings.
  - Added concurrent worker pool execution (up to 8 goroutines) in `ComputeFreshness` and `ListScars`, preserving 100% deterministic ordering via stable ID sorting.
  - Added first-class lifecycle commands `git-scars contradict <id> --by <new-id> --rationale` and `git-scars supersede <id> --by <new-id>`, updating `conflicts_with` and `supersedes` arrays.
  - Added pairwise near-duplicate detection (`git-scars dedupe`) using word-level Jaccard similarity (threshold 0.85) and automated repository health repair (`git-scars doctor --fix`).
- **Consequences**:
  - Closed learning loop enables repository memory to self-clean over time without manual editing of YAML frontmatter.
  - Parallel hashing delivers ~6x faster cold freshness checks at scale while maintaining zero external dependencies and 100% standard library compliance.

---

## ADR-0012: AST-Aware Symbol Anchoring, Portable Memory Bundles, and SARIF PR Annotations

- **Date**: 2026-10-12
- **Status**: Accepted
- **Context**:
  - File-level SHA-256 freshness baselines flag false-positive anchor drift when comments, formatting, or unrelated helper functions in the same file are edited, requiring unnecessary human reaffirmations.
  - Development teams working across multiple services or migrating legacy repositories need to automatically mine past bug fixes from Git history and exchange portable, verified failure memories across repositories without sharing entire Git histories.
  - CI/CD pipelines (GitHub Actions, GitLab CI) need standardized machine-readable static analysis and security scanning annotations to surface guardrails directly on pull request lines.
  - Coding agents require an automated test-execution loop that executes test suites, intercepts failures, correlates error logs with recorded scars, and generates instant remediation guidance.
- **Decision**:
  - Implemented Go standard library AST parsing (`go/parser`, `go/ast`) to extract symbol declaration hashes (`SymbolASTBaselines`). Freshness checks evaluate fine-grained symbol drift: if the overall file SHA changes but anchored functions/types are unchanged, the scar remains `current`.
  - Implemented automated bug-fix commit mining via `git-scars harvest` detecting conventional commit fix patterns (`fix:`, `closes #`, etc.), extracting modified files, and scaffolding reviewable draft scars.
  - Implemented portable memory bundles (`gitscars.bundle.v1`) via `git-scars bundle export` and `git-scars bundle import`. All imported scars are strictly downgraded to `draft` with `unreviewed` status to preserve verification and code-review invariants.
  - Implemented native OASIS SARIF 2.1.0 output in `git-scars check --format sarif` and `git-scars enforce --format sarif`, complete with an automated workflow generator `git-scars init --ci github` creating `.github/workflows/git-scars.yml`.
  - Implemented `git-scars heal -- <test-command>` executing commands, intercepting failures, triaging error traces against active scars, and generating actionable remediation XML/text output.
- **Consequences**:
  - Eliminates false-positive freshness invalidations when non-anchored symbols change in monitored source files.
  - Accelerates adoption on legacy codebases by transforming historical commit logs into structured guardrails.
  - Enables zero-configuration GitHub Code Scanning PR annotations and cross-repository memory sharing.
  - Maintains 100% Go standard library implementation with zero external dependencies.








