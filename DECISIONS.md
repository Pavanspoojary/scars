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

---

## ADR-0019: Measured Evaluation, Fail-to-Pass Reproduction, and a Populated Catalogue

- **Date**: 2026-10-21
- **Status**: Accepted
- **Context**:
  - The retrieval evaluation reimplemented the scoring rules in a parallel copy, so the published metrics described a copy of the algorithm rather than the algorithm itself.
  - The scale invariance claim compared a ten scar catalogue against the full catalogue. The smaller catalogue did not contain the target scars of every query, so the comparison measured catalogue coverage rather than retrieval quality.
  - The verification command passed no test files to the fail-to-pass procedure. The base commit was therefore checked with the tests that already existed at the base. A regression test is normally introduced by the fix commit, so the base ran without it, passed, and every genuine regression was reported as not reproducible. The fail-to-pass claim could not be established through the command that is supposed to establish it.
  - Mined drafts carried no commit references, so they could only ever be verified as an exception.
  - The catalogue held a single seed scar for the whole history of the repository.
  - The protection against YAML aliases rejected any frontmatter line containing an asterisk or an ampersand, including a quoted value. A recorded pattern containing a pointer dereference made the whole record unloadable.
  - The anchor symbol lint rule was a warning, so a guardrail anchored to a symbol that no longer existed was reported but never blocked.
  - Continuous integration annotated guardrail matches but discarded the exit code, so a change that reintroduced a known regression was annotated and then merged.
- **Decision**:
  - The evaluation measures the production retrieval path. The benchmark dataset is stored with the package that consumes it.
  - Two distractor populations are measured: decoys that overlap in wording with disjoint code anchors, and decoys that share the basename of the target file.
  - The verification overlays the test files of the fix commit onto the base worktree.
  - Mined drafts record the fix commit and its first parent, and the mined lesson states what the commit fixed.
  - The alias rejection inspects only the unquoted parts of a frontmatter line.
  - The anchor symbol lint rule is an error, and a symbol anchor is satisfied by a declaration or by any occurrence of the name in the anchored file.
  - Continuous integration fails the job when the pre-edit check blocks, while still emitting the SARIF report.
  - The catalogue is populated with the real defects that were found and fixed, each verified as a genuine fail-to-pass regression.
- **Consequences**:
  - The published metrics describe the shipped tool. The preregistered precision target of 0.70 and false positive limit of 0.10 hold on the frozen test split, measured at 1.000 and 0.0%.
  - RQ2 is supported for realistic distractors, which leave precision unchanged. Under the adversarial basename construction precision falls to 0.778, which isolates and records the cost of the ADR-0010 trade-off.
  - The earlier scale invariance claim is withdrawn as invalid.
  - Verification genuinely reproduces a regression. A regression test that fails at the base and passes at the fix is recorded as fail_to_pass.
  - The catalogue contains nine active memories, each anchored to the code that carries the defect and each backed by a verifier that reproduces it.
  - A quoted pattern containing a pointer dereference no longer makes a record unloadable.
  - The stricter lint severity and the blocking continuous integration step may fail pipelines that previously passed, which is a breaking change for a repository that relied on either being permissive.


---

## ADR-0020: Reachable Mining, Visible Configuration, a Complete Agent Surface, and Executable Evaluation Machinery

- **Date**: 2026-10-21
- **Status**: Accepted
- **Context**:
  - The harvest confidence model granted the focus bonus only when a commit touched at most three files, while the mining filter itself accepts at most `--max-files`. A four or five file fix with co-located tests scored five and was rejected, and three of the most valuable defects in this repository's history were unreachable for exactly that reason.
  - An issue reference cited only in the commit body was not counted, although many repositories cite the tracked issue only there.
  - A root commit was invisible to the diff walker, because `diff-tree` without `--root` reports no changes for the first commit.
  - A configuration entry that could not be interpreted was dropped without comment, so a misspelled key such as `abstain_treshold` silently took the documented default and a repository that believed its retrieval was tuned had no way of discovering otherwise.
  - The agent-facing surface exposed only part of what retrieval and triage understood: recall accepted no command anchors and used hardcoded limits instead of the repository configuration, triage could not be narrowed to files, injection exposed no symbols or task terms, planning exposed neither commands nor an error signature, and a draft could not be given symbol anchors or a calibrated severity. `get_scar` did not surface the severity that planning and enforcement decisions depend on.
  - The preregistered agent success protocol existed only as formulas in a document. Nothing could read a recorded corpus, compute the statistic, or state the sample size, so any agent-level number would have been assembled by hand and therefore unreproducible.
- **Decision**:
  - The focus signal mirrors the mining cap: a commit that already passed the `--max-files` filter is focused by construction, so a capped fix with co-located tests reaches the review bar without an external issue reference, while an untested or uncapped commit still needs one. The sample size discipline is unchanged: the default review bar stays at 7.
  - An issue reference is counted when it appears in the subject or in the body.
  - The diff walker passes `--root`, so a fix recorded in the root commit is minable.
  - The reader reports every present-but-unusable entry through `doctor`, never blocks retrieval on a mistake, and sets aside a file that declares an unsupported schema version whole rather than partially applying it. Aliases such as mapping `threshold` to `abstain_threshold` were rejected: silently reinterpreting a key is worse than naming it. An absent `schema_version` is accepted without warning so a hand written file does not produce permanent doctor noise.
  - The agent-facing surface declares and honours the full argument set, and `recall_scars` defers its limits to the repository configuration exactly as the command line does. `plan_task` keeps a deliberate recall threshold of 2, because planning surfaces candidates for consideration before work begins and does not block. Injection stays deliberately more sensitive than recall, so a task term overlap alone is enough there and task-only injection is documented behaviour rather than a defect.
  - The evaluation machinery computes the paired one-sided non-inferiority test from recorded outcomes only, using the paired construction of Newcombe with the preregistered margin and quantiles, and labels any corpus below the required 928 paired evaluations a pilot study. The commands never simulate arms, so an executed protocol and a planned one remain distinguishable.
- **Consequences**:
  - Three additional historical fixes in this repository become reachable by the miner, including the parse and invariant separation defect; the coarse scope signal now rewards exactly what the filter accepts.
  - A repository that misreads its own configuration learns of it in the first doctor run instead of never.
  - An agent can exploit the same anchoring richness through MCP that the command line offers, and a configured threshold governs both surfaces.
  - The agent success and authoring protocols are executable on recorded outcomes, while both measurements themselves remain Not Measured in this repository until an external harness and a human panel record data.
  - A root commit fix is now minable; the historical commits of this repository contain none, so the catalogue is unaffected by that part of the change.

---

## ADR-0021: A Stamp-Keyed Process Cache on the Retrieval Hot Path

- **Date**: 2026-10-26
- **Status**: Accepted
- **Context**:
  - Benchmarked over a 200-scar corpus on an Apple M4, a single `Recall` cost 7.0 ms, 9.95 MB, and 26,836 allocations, and `InjectGuardrails` cost 7.5 ms. The profile attributed roughly 69 percent of the time to file-open syscalls.
  - Every call re-read and re-parsed each candidate scar file (`LoadScar`, 34 percent), re-hashed every anchor file for freshness (`ComputeFreshness`, 34 percent), re-decoded `index.json` (536 us), re-validated index freshness with a directory scan (379 us), and re-parsed the dependency manifests through `GetRepoDependencies`.
  - Those costs are paid again on every request in a long-lived process. The MCP server holds one `Store` for its lifetime and recalls on every tool call, and the pre-commit hook runs the same path on every commit, so a store that never changes still paid full disk and parse cost per call.
- **Decision**:
  - `Store` gains a per-process `scarCache` holding parsed scar records, anchor digests, the decoded index, and the parsed dependency set. Every entry is keyed by a `fileStamp` of size plus modification time in nanoseconds, taken **before** the file is read, so a write that races a read leaves a mismatching stamp and the entry is refreshed rather than pinning stale bytes.
  - The cache never hands out its own values. `LoadScar`, the catalog listing, and recall's candidate materialisation receive deep copies (`Scar.clone`), so a caller mutating a loaded scar cannot corrupt what the next recall sees.
  - The store invalidates its own writes explicitly (`SaveScar`, `RebuildIndex` call `cache.drop`), which closes the window where a rewrite preserves both size and modification time. `RecordBaseline` deliberately keeps the direct hash path: recording a baseline is correctness-critical and rare, so it never reads a digest it did not just compute.
  - Recall keeps the documented architecture: the index fast path is still validated against the scar directory on every call, but its decode is memoised on the index file's own stamp. Retrieval weights, thresholds, and the fail-closed invariant gate are untouched; only the cost of arriving at the same answer changed.
  - Tokenisation stops building a stopword map per call (package-level `stopwordSet`), and task-term overlap short-circuits when the query carries no task.
- **Consequences**:
  - Measured on the same corpus, `Recall` falls from 7,014 us to 1,187 us (5.9x), from 9.95 MB to 0.84 MB (11.9x), and from 26,836 to 6,016 allocations (4.5x); `InjectGuardrails` falls to about 1.8 ms and the stale-index fallback to 0.75 ms. `ScoreScarItem` falls from 1,713 ns to 255 ns per scar.
  - A one-shot CLI process starts with an empty cache and behaves exactly as before. The win accrues to the MCP server and the hook, which now pay one directory read, one stat per scar file, and a clone of each returned candidate instead of a full read-parse-hash cycle.
  - Freshness now relies on size plus modification time for the *cached* digest. An external edit that preserves both would be missed until the file is touched again; an edit from `git-scars` itself always invalidates. This matches the trust model of other index-based tools and is recorded here because it is a real, if narrow, weakening of the previous always-rehash behaviour for long-lived processes.
  - The parsed corpus stays resident for the life of the store. For a catalogue of the size this tool targets (tens to low hundreds of scars) that is well under a megabyte; a very large catalogue would trade memory for the same latency win.
  - `LoadIndex` returns a shared value and is documented read-only, and the dependency map returned by `repoDependencies` is treated as read-only by every internal caller.
- **Alternatives considered**:
  - Ripping the index fast path out of `Recall` in favour of a plain cached directory scan: measured faster at 200 scars (about 0.35 ms) but it would make `index.json` vestigial for retrieval and silently demote a documented command, so the architecture was kept and its decode memoised instead.
  - A TTL cache: rejected because a time-based rule can serve stale guardrails for the length of the TTL, while a stamp check is both cheaper (one stat) and exact under the documented assumption.
  - Hash buffers pooled with `sync.Pool`: rejected because the stamp cache removes almost all hashing from the warm path anyway, and the added machinery had no profile to justify it.

## ADR-0022: The Freshness Check Gates Continuous Integration; Enforcement Annotates

- **Date**: 2026-10-26
- **Status**: Accepted
- **Context**:
  - The `git-scars-freshness` code scanning configuration reported "Workflow runs failing" after the v3.0.0, v3.1.0, and v3.2.0 merges. The `Collect guardrail matches on changed files` step ran `enforce --fail-on high`, which blocks on severity and file overlap alone (`IncludeStale: true`), so any change touching a file anchored by a high-severity scar exited 2 even after the scar had been reaffirmed. The step's own guidance ("reaffirm or supersede the guardrail to record the review") could never turn the build green, so the gate was unpassable by its stated resolution.
  - The failing step skipped `Collect freshness findings`, which carried no `if: always()`, so `Upload freshness annotations` then failed on the missing `git-scars-check.sarif`. That upload failure, not the guardrail finding, is what code scanning reported as the error.
  - The verdict that can distinguish a reviewed change from an unreviewed one already existed and already ran: `check` exits 2 when an anchored symbol changed since the recorded baseline, symbol-level freshness keeps unrelated edits inside the same file current, and the `Validate GitScars integrity and freshness` step passed on the same run in which the enforce step failed.
  - ADR-0018 records the intent of the continuous integration step as collecting changed files, evaluating them, and uploading the result as annotations under a dedicated code scanning category, and README section 9 documents `check` as the CI freshness gate with stable exit codes. The later hard-fail on `enforce` drifted from both.
  - The generated workflow from `git-scars init --ci github` carried the same wiring with an additional inversion: `enforce` failed the build while `check` ran with `continue-on-error`, so a stale anchor never failed a scaffolded build.
- **Decision**:
  - In continuous integration, `enforce` collects SARIF annotations and reports a match (exit 2) as a `::warning::` annotation; only an operational failure (any exit other than 0 or 2) fails the collecting step. `check` remains the failing gate: a guardrailed symbol changed without a recorded review fails the build with exit 2 and the stale-scar list.
  - Both collection steps run under `if: always()` so the annotation chain survives an earlier failure, and both `upload-sarif` steps are guarded with `hashFiles(...) != ''` so an upload never fails on a missing or empty file.
  - The generated workflow is wired identically: `check` gates the build, `enforce` annotates, and the uploads are guarded.
  - Action pins move to `github/codeql-action/upload-sarif@v4` (v3 is deprecated for December 2026), `actions/checkout@v7`, and `actions/setup-go@v7` (the Node 20 runtime deprecation warnings).
  - The `Enforce` command itself is unchanged: its pre-edit semantics (block on a match regardless of freshness) are pinned by tests and README section 3 and serve the local hook and the MCP pre-edit surface, where a fresh scar must still block before an edit is made.
- **Consequences**:
  - A change that touches guardrailed files goes green exactly when the scar's baseline covers the current content, so reaffirming in the branch makes continuous integration pass as the message always promised. A change that alters a guardrailed symbol without reaffirmation fails at `Validate GitScars integrity and freshness` with the stale-scar list.
  - Edits inside an anchored file that preserve every anchored symbol no longer fail the build; symbol-level freshness already declared those edits outside the guarded behaviour, and the file-overlap rule no longer contradicts that verdict.
  - SARIF uploads complete on every run that produces them, so the `git-scars-guardrails` and `git-scars-freshness` code scanning configurations stop reporting failing workflow runs, and guardrail annotations still appear on the pull requests that need them most.
  - The scaffolded workflow no longer needs a follow-up patch by each consumer to become passable.
- **Alternatives considered**:
  - A freshness-aware mode of the `enforce` command (a flag failing only on stale matches): rejected as indirection, because `check` already computes exactly that verdict and is already the documented CI gate. A second command reaching the same answer would duplicate the freshness path and widen the public CLI surface.
  - Keeping the enforce step as a hard gate and repairing only the upload chain: rejected because the build would stay permanently red for every change to a file anchored by a high-severity scar, which is the reported failure itself rather than a fix for it.
  - Making `Enforce` itself freshness-aware: rejected because pre-edit enforcement must block on a match before any edit exists, and `TestEnforceBlocked` plus README section 3 pin that behaviour.
