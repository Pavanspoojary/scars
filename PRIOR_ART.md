# Prior Art and Namespace Assessment

**Date**: 2026-10-11  
**Project**: GitScars (formerly provisional RepoScars)  
**Namespace**: `git-scars` (CLI: `git-scars`, repository store: `.gitscars/`)

---

## 1. Primary Source Overlaps

We analyzed adjacent approaches and existing systems in the developer tooling and agent-memory ecosystem:

| Approach / Tool | Primary Model | Storage / Location | Core Strengths | Documented Limitations |
| :--- | :--- | :--- | :--- | :--- |
| **Flat Instruction Files** (`AGENTS.md`, `CLAUDE.md`, Cursor rules) | Static text injection | Repo root (`AGENTS.md`) | Zero setup; universally read by coding agents. | Context bloating; rule dilution ("lost-in-the-middle"); no symbol/dependency anchoring; no automated staleness detection; no test execution evidence. |
| **Repository Memory Banks** (Cline `memory-bank/`, Roo-Code) | Agent self-maintained notes | `memory-bank/*.md` | Retains active session context across prompts. | Unverified agent summaries; non-deterministic updates; hallucination compounding; high token overhead on writes. |
| **Architecture Decision Records (ADRs)** | Human-authored architectural records | `docs/adr/*.md` | High-level design intent; PR-reviewed; versioned. | Not operationalized for recurring bugs; no test verifiers attached; not queryable by runtime test failure output or AST anchors. |
| **Mem0 & Letta** | Graph & vector agent memory | Cloud APIs / Vector DB | Long-term conversational user profile & chat recall. | Cloud/API dependent; unaware of Git commits or file diffs; vulnerable to memory poisoning (MemSecBench arXiv:2607.27080); not PR-reviewable. |
| **[Daily-Nerd/Scar](https://github.com/Daily-Nerd/Scar)** (`scar-cli`) | Negative knowledge fences | Python CLI hooks | Pre-edit fences preventing agent landmines. | Focuses on agent tool-call interception; lacks reproducible fail-to-pass test overlays, lockfile range anchors, and Git-native review lifecycle. |
| **[VDP89/fscars](https://github.com/VDP89/fscars)** | Correction primitive for "Lucy Syndrome" | Python package (`fscars`) | Blocks repeated agent mistakes via tool-call hooks. | Bound to specific agent configs (Claude Code/Codex); lacks repository-level versioned file schemas, dynamic code-drift detection, and verifier sandboxing. |
| **[gitmem-dev/gitmem](https://github.com/gitmem-dev/gitmem)** | MCP institutional memory server | JSON / MCP service | Tracks "scars, wins, patterns" across sessions. | Session-level memory rather than test-anchored verifiers; relies on LLM summarization; no AST/dependency staleness checking. |

---

## 2. Research Benchmark Findings

1. **VibeMemBench ([arXiv:2609.23570](https://arxiv.org/abs/2609.23570), Sept 2026)**:
   - Evaluated memory systems on real repository coding benchmarks.
   - Finding: Indiscriminate memory injection degrades coding agent pass rates due to context noise and false relevance.
   - Implication: Memory retrieval must be selective, high-precision, and abstain aggressively when no direct match exists.
2. **Agent Retrieval Bench ([arXiv:2607.24882](https://arxiv.org/abs/2607.24882), July 2026)**:
   - Evaluated repository context acquisition.
   - Finding: Deterministic structural signals (file paths, imported symbols, error signatures) outperform vector embeddings for repository bug localization.
3. **MemSecBench ([arXiv:2607.27080](https://arxiv.org/abs/2607.27080), July 2026)**:
   - Evaluated memory poisoning attacks across the "Write-Execute-Forget" lifecycle.
   - Finding: Storing unvetted instructions in agent memory leads to privilege escalation and unauthorized code execution.
   - Implication: Scars must be treated as untrusted data, never privileged instructions. Verifier execution requires argv arrays, environment scrubbing, interactive consent or local SHA256 allowlists, and output redaction.

---

## 3. The Credible Remaining Gap

Existing tools fall into two unaligned extremes:
1. **Unverified prose instructions** (`AGENTS.md`, `CLAUDE.md`, memory banks) that suffer from drift, context bloat, and zero execution proof.
2. **Agent-specific runtime interception hooks** (`fscars`, `scar-cli`, `gitmem`) that require specific agent harnesses and lack Git-native, PR-reviewed, fail-to-pass test evidence tied to code anchors.

**GitScars' Wedge**: A lightweight, standalone, single-binary CLI storing Git-versioned failure lessons in `.gitscars/` with:
- Strict separation of verification evidence, human review, and lifecycle state.
- Dynamic freshness computation (anchor hashing) that detects code drift without mutating files.
- Deterministic, multi-signal retrieval (files, symbols, lockfile dependencies, error signatures) with an abstain threshold.
- Fail-to-pass Git worktree test verification and sandboxed execution.

---

## 4. Candidate Differentiators (Stated as Hypotheses)

- **Hypothesis 1 (Selective vs Flat)**: On repositories with $>20$ accumulated failure lessons, deterministic selective retrieval achieves lower token consumption and non-inferior bug prevention compared to a monolithic `AGENTS.md`.
- **Hypothesis 2 (Code Anchoring & Drift)**: Comparing current file/symbol hashes against the recorded anchor baseline reliably flags stale lessons before agents act on obsolete assumptions.
- **Hypothesis 3 (Safe Verifier Sandboxing)**: Executing verifiers via argv arrays with scrubbed environments and command-hash allowlists prevents prompt-injection execution vectors highlighted by MemSecBench.

---

## 5. Falsification Conditions

GitScars' core premise will be considered **falsified** if:
1. **Ergonomic Overhead**: Median Scar authoring time exceeds 5 minutes, or anchor drift alert fatigue causes maintainers to ignore warnings.
2. **Retrieval Failure**: Selective retrieval achieves $<70\%$ precision@3 or $>10\%$ false-positive rate on non-matching tasks on the frozen test split.
3. **Flat-File Parity**: In a controlled evaluation with 50 scars, supplying all lessons in a single flat `AGENTS.md` yields equal or higher agent success rates with simpler maintenance.
