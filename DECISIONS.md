# Architecture Decision Records (ADR)

This document tracks significant design choices, trade-offs, evaluated alternatives, and rationale for `git-scars`.

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
- **Context**: Selecting the open-source license for `git-scars`. Evaluated Apache 2.0, MIT, and MPL-2.0.
- **Decision**:
  - Retain and confirm the **Apache License 2.0**.
- **Consequences**:
  - Provides a permissive open-source license with explicit patent protection grants and trademark restrictions suitable for developer tooling.

---

## ADR-0003: Namespace and Project Identity Selection

- **Date**: 2026-10-11
- **Status**: Accepted
- **Context**: Stage 0a namespace evaluation to avoid collisions across GitHub, package registries, and developer tooling (specifically `scar-cli`, `fscars`, and `gitmem`). The repository was previously named `si-scars` under that working title, and earlier records refer to the project by that name.
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

---

## ADR-0013: Transparent Threat Model, Multi-Language Symbol Freshness, and Scoped-Rule Differentiation

- **Date**: 2026-10-12
- **Status**: Accepted
- **Context**:
  - Comparative claims must be grounded in empirical evidence rather than speculative marketing. The primary competitor to GitScars in developer workflows is not vector RAG, but glob-scoped rule engines (`.cursor/rules/*.mdc`, nested directory `CLAUDE.md`).
  - AST symbol hashing was previously implemented only for Go. In multi-language repositories (Python, TypeScript, JavaScript), file SHA hashing marked scars stale on any cosmetic edit to hot files, causing human reaffirmation fatigue.
  - Claiming "isolated verifier execution" overclaims security properties. While `cleanEnv()` scrubs environment variables and `exec.Command` avoids shell interpolation, verifiers execute on the host system with the permissions, filesystem access, and network access of the host process.
  - Mining historical commits (`git-scars harvest`) without strict quality heuristics can produce excessive, low-signal drafts, degrading review habits and adoption.
- **Decision**:
  - Implemented empirical replay evaluation (`pkg/scar/benchmark_eval.go`, `EVAL_RESULTS.md`) across 10 production regression scenarios in Go, Python, and TypeScript, measuring exact token footprints, drift sensitivity, toxic claim rejection, and retrieval precision.
  - Documented the three concrete failure modes of glob-scoped rules: anchor refactoring (silent failure), unverified toxic claims, and multi-rule prompt inflation in shared packages.
  - Implemented regex-based multi-language AST/symbol extraction for Python (`def`, `class`, `async def`) and TypeScript/JavaScript (functions, classes, interfaces, types, exported symbols) in `pkg/scar/ast.go`, extending symbol freshness to all major languages.
  - Formally updated terminology from "isolated" to "scrubbed-environment verifier" and established explicit documentation for the host execution threat model and trust boundaries.
  - Hardened `git-scars harvest` with diff-size caps (`<= 5` files), required co-located test modifications by default (`--allow-untested` flag), and quality confidence scoring (`>= 7/10`).
- **Consequences**:
  - Clear, measurable competitive differentiation against glob-scoped rules backed by reproducible benchmarks.
  - Eliminates reaffirmation fatigue across Go, Python, and TypeScript codebases.
  - Transparent security documentation prevents false assumptions regarding container sandboxing.
  - Harvested drafts are high-signal and reviewable, ensuring sustainable human governance.
  - Retains 100% Go standard library implementation with zero third-party dependencies.

---

## ADR-0014: Purge of Synthetic Metrics and Explicit Demarcation of Scenario Analysis vs Agent Empirical Protocols

- **Date**: 2026-10-12
- **Status**: Accepted
- **Context**:
  - The core thesis of GitScars is that "unverified claims are toxic." Presenting synthetic estimates or hardcoded constants (`PrecisionAt1: 1.00`, `DriftDetectionRate: 1.00`, `0.55`, `0.10`) as "Empirical Evaluation Results" directly undermined that thesis.
  - No downstream LLM agents had been executed in that harness; token counts were heuristic string-length estimations (`len(text)/4`); and the 10 scenarios were illustrative models rather than agent experiment runs.
  - Presenting unverified numbers as empirical measurements compromises the intellectual integrity of the project and conflates design modeling with the preregistered empirical multi-agent trial protocol outlined in `EVAL_PLAN.md`.
- **Decision**:
  - Completely purged all synthetic rate constants (`DriftDetectionRate`, `ToxicClaimRejectionRate`, `PrecisionAt1`) from `pkg/scar/benchmark_eval.go` and its unit tests.
  - Relabeled `EVAL_RESULTS.md` to `SCENARIO_ANALYSIS.md` ("Design Estimates & Illustrative Scenario Analysis"), adding a prominent, explicit methodology disclosure box defining what is modeled (prompt token footprints via `EstimateTokens`) versus what is deferred to the multi-agent harness in `EVAL_PLAN.md` (agent repeat rates requiring $\ge 900$ task runs).
  - Authored `pkg/scar/scenario_mechanics_test.go` to provide executable unit test verification of architectural claims (anchor drift detection on symbol edits, glob fragility on moved files, and verifier gating on active promotion) directly in code.
  - Aligned `README.md` comparison tables to strictly report architectural capabilities and estimated token footprints rather than synthetic precision percentages.
- **Consequences**:
  - 100% intellectual honesty and alignment with repository standards.
  - Clear separation between static scenario modeling, executable mechanical tests, and future empirical agent trials.
  - Preserves the credibility of genuine architectural breakthroughs (AST symbol drift detection, scrubbed fail-to-pass verifiers, and token-budgeted prompt injection).

---

## ADR-0015: Separation of Parsing from Invariant Validation and Surfacing of Load Failures

- **Date**: 2026-10-21
- **Status**: Accepted
- **Context**:
  - `ParseFrontmatter` called `ValidateInvariants` before returning a parsed record. Any record violating a lifecycle, review, or verification invariant therefore failed to parse.
  - `ListScars` discarded every file that failed to parse. The combined effect was that invariant-violating records were removed from the catalog before any consumer could inspect them.
  - `RunReadOnlyCheck`, `runLint`, and the `RunDoctor` "Lifecycle Invariants" probe all re-validate invariants on already-parsed records. Because invalid records never reached them, these probes were unreachable in practice: `git-scars check` reported exit code `0` and zero findings against a repository containing a record that claimed `lifecycle.state: active` without human approval or verification evidence.
  - A malformed record was equally invisible: it was skipped without any diagnostic.
  - The repository's central thesis is that unverified claims are toxic. A guardrail that cannot observe an unverified claim does not enforce the thesis.
- **Decision**:
  - Split schema compatibility out of `ValidateInvariants` into `validateSchemaVersion`. Schema compatibility remains a parse-time gate so that a build refuses to read a record format it does not understand.
  - `ParseFrontmatter` is now syntactic only. Lifecycle, review, and verification invariants are enforced by `ValidateInvariants`, which the reporting commands apply to loaded records.
  - Add `ListScarsDetailed`, returning load failures alongside the loadable catalog. `ListScars` delegates to it and preserves its existing contract.
  - `RunReadOnlyCheck` reports load failures as invalid scars and exits `1`. `runLint` emits an `unloadable_scar` error finding. `RunDoctor` gains a "Scar Loadability" probe that fails the report.
- **Consequences**:
  - Invariant violations and malformed records are now observable in CI, which restores the enforcement guarantee the project is built on.
  - `ParseFrontmatter` no longer guarantees semantic validity. Callers that persist a parsed record remain protected because `SaveScar` validates invariants before writing.
  - The catalog reads remain a single pass: load failures are collected during the existing worker pool rather than through a second scan.
  - Exit code `1` from `git-scars check` now covers the full set of invalid records it documents.

---

## ADR-0016: Authorable and Round-Trippable Dependency Anchors

- **Date**: 2026-10-21
- **Status**: Accepted
- **Context**:
  - `Anchors.Dependencies` was consumed by retrieval scoring and matched against parsed lockfiles, but it was never written. The parser had an empty stub branch for dependency entries, `FormatFrontmatter` omitted the section, and no command could author an anchor.
  - The dependency matching, lockfile parsing, and version scoring path was therefore unreachable in practice, and the capability described in the README and in ADR-0005 and ADR-0006 could not be exercised.
  - Hand-written dependency anchors were not merely ignored. Because the serializer omitted the section, the next write to the record discarded the anchor entirely.
- **Decision**:
  - Parse dependency entries from the frontmatter and serialize them back, so anchors survive verification, reaffirmation, promotion, and synchronization cycles.
  - Author dependency anchors through `git-scars add --deps` using the `name@version` form.
- **Consequences**:
  - Dependency anchors become a functional retrieval signal: a matching installed version scores `+3`, which is sufficient on its own to cross the abstain threshold.
  - A dependency upgrade now surfaces the scars anchored to the previous version, as the README describes.
  - The parser remains a hand-written state machine with no third-party dependency, consistent with the zero-dependency constraint.

---

## ADR-0017: Anchor Semantics, Evidence Binding, and Fail-Closed Retrieval

- **Date**: 2026-10-21
- **Status**: Accepted
- **Context**:
  - Anchor correctness was trusted rather than checked. A guardrail could point at a file or symbol that no longer existed, or at a file unrelated to the failure it described, and nothing would report it.
  - The repository's own seed scar was an instance of this. It was anchored to `cmd/git-scars/main.go` and the symbol `run`, neither of which contains a channel or a goroutine. The worker pool it describes lives in `pkg/scar/store.go`. Recall for the file containing the worker pool returned zero scars, and enforcement never blocked, so the guardrail protected nothing.
  - The verifier command hash was recorded but never compared against the argv it described. An active scar could claim fail-to-pass verification with a fabricated hash.
  - `Recall` returned any active scar with current anchors, without validating it. A record that failed an invariant could still be injected into an agent prompt.
  - An active scar without a severity was scored as medium by enforcement, so it could never reach the default high blocking threshold.
  - `RecordBaseline` merged new anchor hashes into the existing map. A re-anchored scar stayed coupled to the file it no longer referenced.
- **Decision**:
  - Lint verifies anchor semantics: anchored symbols must resolve, the recorded bad pattern must be absent from the anchored code, the recorded good pattern must be present, and an active guardrail must declare a severity.
  - Patterns must be literal snippets of code so that presence and absence are deterministic.
  - An active scar claiming fail-to-pass verification must record a command hash derived from its own argv, working directory, and environment allowlist.
  - `Recall` refuses to return a scar that violates an invariant. Retrieval fails closed.
  - Enforcement treats an undeclared severity as high.
  - `RecordBaseline` replaces the recorded anchor and symbol sets rather than merging into them.
  - The seed scar is re-anchored to the implementation it describes and is given a severity, a fix hint, and literal patterns.
- **Consequences**:
  - The seed guardrail now fires: recall for `pkg/scar/store.go` returns it, and enforcement blocks pre-edit with actionable do and do-not instructions.
  - Verification evidence is bound to the command it proves. A record whose evidence cannot be reproduced from its own metadata is rejected rather than trusted.
  - Invalid guardrails are excluded from retrieval instead of being injected into agent prompts.
  - Anchor drift that previously passed silently is now reported by `check`, `lint`, and `doctor`.
  - The stricter invariant rejects records that were previously accepted, which is a breaking change for any repository holding active scars whose recorded hash does not match their argv. The release is therefore versioned as a major increment.

---

## ADR-0018: Operational Consistency of Configuration, Output, and Continuous Integration

- **Date**: 2026-10-21
- **Status**: Accepted
- **Context**:
  - `git-scars init` wrote `.gitscars/config.yaml`, but no code ever read it. The abstain threshold and the result limit were hardcoded in the library and duplicated as flag defaults in the CLI, so the file was inert. The repository's own copy had drifted to a stale key set that no longer matched what `init` writes.
  - The SARIF rule array was built by iterating a map, so the ordering of rules differed between runs and reports of identical findings were not byte-identical.
  - `git-scars stats` detected its JSON flag by scanning the argument list for a literal string, so `--json=false` produced JSON output and unknown flags were accepted silently.
  - The heal command reused the observed command's exit code as its own, so a failing test run that exited 1 was indistinguishable from an operational error of git-scars itself.
  - The MCP server described itself as strictly read-only while the `draft_scar` tool writes a new record, and the `post_failure_learn` tool declared a files argument that was ignored.
  - Generated guardrail headers and diagnostic output contained non-ASCII pictographs, which conflicts with the repository standard requiring ASCII output without emoji.
  - Continuous integration validated the scar catalog but never evaluated the guardrails against the files a change touched. The pre-edit check existed only as a local hook.
- **Decision**:
  - The configuration file supplies the abstain threshold and the result limit. An explicit flag overrides it, a value of zero defers to it, and an unreadable, malformed, or individually unusable value falls back to the defaults rather than failing the command.
  - SARIF rules are emitted in a stable order, and an empty result set is emitted as an empty array.
  - Every subcommand parses its flags with the standard flag set.
  - Heal exits 4 when the observed command failed and 0 when it succeeded. The observed command's own exit code remains in the output.
  - The MCP server states what it actually does, and `post_failure_learn` passes its files argument into the retrieval query.
  - Generated and diagnostic output is plain ASCII.
  - Continuous integration collects the files changed by the event, evaluates them with the pre-edit enforcement check, and uploads the result as SARIF annotations under a dedicated code scanning category. The generated workflow from `git-scars init --ci github` performs the same step.
  - The file list argument accepts a comma or newline separated list so that it can be piped directly from a git command.
- **Consequences**:
  - Repository-scoped retrieval limits are configurable and the configuration file is meaningful.
  - Reports of identical findings are reproducible, so continuous integration logs and uploaded artefacts no longer churn.
  - Continuous integration surfaces guardrail matches on the changed lines of a pull request instead of only enforcing them locally.
  - A failure of the observed command can no longer be mistaken for a failure of git-scars itself.
  - Output is portable across terminals and fonts, and conforms to the repository standard.
  - The changes are additive and do not invalidate any recorded scar, so the release is versioned as a minor increment.

