# Evaluation Plan and Preregistration Protocol

**Date**: 2026-10-11  
**Project**: GitScars (`git-scars`)  
**Status**: Preregistered & Frozen for Test Evaluation  

---

## 1. Research Questions and Hypotheses

1. **RQ1 (Retrieval Quality)**: Can a deterministic multi-signal scoring model (anchors + error signatures + command + term overlap) achieve $\ge 70\%$ Precision@3 and $\le 10\%$ False Positive Rate on no-match queries without semantic embeddings or external LLM re-ranking?
2. **RQ2 (Distractor Invariance)**: Does the addition of keyword-overlapping distractor Scars degrade retrieval precision when code anchors (files, symbols) differ?
3. **RQ3 (Downstream Non-Inferiority)**: Does selective retrieval perform non-inferiorly to monolithic flat instruction injection (`AGENTS.md`) within a 5 percentage point margin, while reducing prompt context token volume?

---

## 2. Evaluation Arms

On held-out recurrent bug tasks, we specify four comparative conditions:

1. **Arm 0 (No Memory)**: Agent receives repository code and task prompt without prior failure knowledge.
2. **Arm 1 (Flat Monolith)**: Agent receives all lessons concatenated in a single flat `AGENTS.md` file.
3. **Arm 2 (All Scars Injected)**: Agent receives all structured `.gitscars/scars/*.md` documents in full context without selective filtering.
4. **Arm 3 (Selective Retrieval)**: Agent receives top $\le 3$ Scars filtered by `git-scars recall` satisfying the abstain threshold (score $\ge 3$).

---

## 3. Frozen Scoring Baseline & Weights

The scoring function is frozen across all evaluations:

```
Score = (Exact File Anchor    ? 5 : 0)
      + (Exact Symbol Anchor  ? 4 : 0)
      + (Dependency & Version ? 3 : 0)
      + (Error Signature      ? 3 : 0)
      + (Command Match        ? 2 : 0)
      + (Task Term Overlap    : [0, 2])
```

- **Constraint**: At most one contribution per category.
- **Abstain Threshold**: Score $\ge 3$. If max score $< 3$, the system abstains (returns 0 candidates, exit code 0).
- **Top-K Limit**: At most 3 candidates, ordered by score descending, then stable ID ascending.

---

## 4. Preregistered Thresholds and Measurability

| Metric | Target / Threshold | Measurability in Current Environment | Current Measured Value |
| :--- | :--- | :--- | :--- |
| **Precision@3** | $\ge 0.70$ (70%) | Measurable on labeled benchmark | **1.000** (Test split) |
| **Recall@3** | $\ge 0.70$ (70%) | Measurable on labeled benchmark | **1.000** (Test split) |
| **No-Match FP Rate** | $\le 0.10$ (10%) | Measurable on labeled benchmark | **0.0%** (Test split) |
| **Median Authoring Time** | $\le 5$ minutes | **Not Measured** (Requires human panel) | N/A (Protocol below) |
| **Agent Success Non-Inferiority** | Within 5% of Arm 1 | **Not Measured** (Requires external multi-agent harness) | N/A (Pilot protocol below) |

---

## 5. Protocols for Unmeasured Items

### 5.1 Protocol for Human Authoring Effort
- **Participants**: 5 professional software engineers.
- **Task**: Given a commit diff that resolved a regression, create a new Scar using `git-scars add from-commit <sha>` and review the resulting draft.
- **Measurement**: Wall-clock duration from command start to finalized, committed Scar. If median $> 5$ minutes, the low-friction capture hypothesis is rejected.

### 5.2 Protocol for Agent Task Success and Sample Size Calculation
- **Non-inferiority Margin ($\Delta$)**: 5% ($\Delta = 0.05$).
- **Significance Level ($\alpha$)**: 0.05 (one-sided).
- **Power ($1 - \beta$)**: 0.80 (80%).
- **Assumed Baseline Pass Rate ($p$)**: 75% ($0.75$).
- **Sample Size Formula**:
  $$n = \frac{2 \cdot (Z_{1-\alpha} + Z_{1-\beta})^2 \cdot p(1-p)}{\Delta^2}$$
  With $Z_{0.95} = 1.645$ and $Z_{0.80} = 0.842$:
  $$n \approx \frac{2 \cdot (2.487)^2 \cdot (0.75 \cdot 0.25)}{0.0025} \approx \frac{12.37 \cdot 0.1875}{0.0025} \approx 928 \text{ task evaluations}$$
- **Conclusion**: With $<30$ recurrence tasks currently available, any agent-level evaluation in this phase is strictly labeled a **Pilot Study**. Confirmatory claims are deferred until large-scale test harnesses are executed.

---

## 6. Spike Retrieval Results Summary

Evaluated against `spike/dataset.json` (50 Scars, 20 Queries split into 8 dev, 12 test):
- **Development Split**: Precision@3 = 1.00, Recall@3 = 1.00, No-Match FP Rate = 0.0%, Abstain Rate = 25.0%.
- **Frozen Test Split**: Precision@3 = 1.00, Recall@3 = 1.00, No-Match FP Rate = 0.0%, Abstain Rate = 25.0%.
- **Scale Invariance**: Zero false positives across 10-scar and 50-scar pools when distractor scars with overlapping words were introduced.
