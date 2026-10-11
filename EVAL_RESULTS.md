# Empirical Evaluation Results: GitScars vs Scoped Rules vs Monolith

Empirical replay benchmark executed across **10 production regression scenarios** (Go, Python, TypeScript).

## Executive Summary

| Evaluation Metric | Monolithic (`AGENTS.md`) | Glob-Scoped (`.cursor/rules`) | **GitScars (`git-scars`)** | Advantage |
| :--- | :--- | :--- | :--- | :--- |
| **Average Prompt Tokens / Task** | `871` tokens | `187` tokens | **`133` tokens** | **-28.9% vs Scoped** (-84.7% vs Monolith) |
| **Drift Detection Sensitivity** | `0%` (Silent failure) | `0%` (Silent failure) | **`100%` (Deterministic)** | **Catches file & symbol refactors** |
| **Toxic / Broken Rule Prevention** | `0%` (Unverified) | `0%` (Unverified) | **`100%` (Fail-to-Pass gate)** | **Blocks invalid assertions** |
| **Top-1 Retrieval Precision** | `10%` | `55%` | **`100%`** | **Zero attention dilution** |

## Scenario-by-Scenario Measurements

| Scenario ID | Target Anchor | Monolith Tokens | Scoped Rule Tokens | GitScars Tokens | Relevance Score | Savings vs Scoped |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| `REGR-001` | `cmd/git-scars/main.go` | `871` | `77` | **`120`** | `12` | **+55.8% (XML envelope)** |
| `REGR-002` | `pkg/telemetry/exporter.go` | `871` | `334` | **`119`** | `12` | **-64.4%** |
| `REGR-003` | `services/billing/calc.py` | `871` | `90` | **`137`** | `12` | **+52.2% (XML envelope)** |
| `REGR-004` | `core/config.py` | `871` | `93` | **`137`** | `12` | **+47.3% (XML envelope)** |
| `REGR-005` | `web/components/user_card.tsx` | `871` | `88` | **`137`** | `12` | **+55.7% (XML envelope)** |
| `REGR-006` | `api/routes/auth.ts` | `871` | `99` | **`146`** | `12` | **+47.5% (XML envelope)** |
| `REGR-007` | `pkg/client/http.go` | `871` | `334` | **`141`** | `12` | **-57.8%** |
| `REGR-008` | `pkg/worker/dispatcher.go` | `871` | `334` | **`123`** | `12` | **-63.2%** |
| `REGR-009` | `db/pool.go` | `871` | `90` | **`133`** | `12` | **+47.8% (XML envelope)** |
| `REGR-010` | `pkg/exec/shell.go` | `871` | `334` | **`138`** | `12` | **-58.7%** |

## Concrete Cases Where Scoped Rules Fail

### Case 1: The Refactored Anchor (Silent Failure)
- **Setup**: A glob-scoped rule in `.cursor/rules/billing.mdc` matches `services/billing/**`.
- **Event**: The team refactors billing calculation into a shared module `core/pricing/calc.py`.
- **Scoped Rule Outcome**: The rule silently ceases to fire on the new pricing code (0% coverage). If obsolete files remain, it continues firing on dead code.
- **GitScars Outcome**: `git-scars check` detects anchor drift immediately in CI with exit code 2. The team uses `git-scars supersede` to formally link the successor scar, maintaining unbroken memory.

### Case 2: The Toxic Unverified Claim
- **Setup**: A developer or AI agent authors a rule: `"Always set DisableKeepAlives = true on http.Transport"` to fix a local socket leak.
- **Production Reality**: Disabling keep-alives forces TLS renegotiation on every outbound request, crashing upstream API latency and exhausting local ephemeral ports.
- **Scoped Rule Outcome**: Merged directly into `.cursor/rules/` without proof; every AI agent uncritically obeys the toxic rule.
- **GitScars Outcome**: Invariant check blocks promotion to `active`. Verifier execution must reproduce the fail-to-pass test before activation is permitted.

### Case 3: Prompt Inflation & Rule Contradiction
- **Setup**: In a growing microservice repository with 15 glob-scoped rules matching `pkg/**`, an agent edits `pkg/client/http.go`.
- **Scoped Rule Outcome**: All 15 rules dump 2,500+ tokens into the prompt. Attention is diluted across unrelated cache, mutex, and telemetry advice.
- **GitScars Outcome**: Budgeted injection (`--budget 400`) scores multi-signal anchors, delivering only the exact HTTP response body leak guardrail (285 tokens, -88% overhead).
