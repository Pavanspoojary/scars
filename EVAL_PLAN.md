# Evaluation Plan and Preregistration Protocol

**Date**: 2026-10-11  
**Project**: GitScars (`git-scars`)  
**Status**: Preregistered & Frozen for Test Evaluation  
**Last measured**: 2026-10-21

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
- **Partial file signals**: same basename $+2$, same directory $+1$. These were introduced in ADR-0010 to recover moved files. They are below the abstain threshold on their own, but they may combine with another signal.

---

## 4. Preregistered Thresholds and Measurability

| Metric | Target / Threshold | Measurability in Current Environment | Current Measured Value |
| :--- | :--- | :--- | :--- |
| **Precision@3** | $\ge 0.70$ (70%) | Measurable on labeled benchmark | **1.000** (frozen test split) |
| **Recall@3** | $\ge 0.70$ (70%) | Measurable on labeled benchmark | **1.000** (frozen test split) |
| **No-Match FP Rate** | $\le 0.10$ (10%) | Measurable on labeled benchmark | **0.0%** (frozen test split) |
| **Mean Reciprocal Rank** | Not preregistered | Measurable on labeled benchmark | **1.000** (frozen test split) |
| **Abstain Rate** | Not preregistered | Measurable on labeled benchmark | **25.0%** (frozen test split) |
| **Precision@3 under 50 keyword distractors** | Not preregistered | Measurable on labeled benchmark | **1.000** (no degradation) |
| **Precision@3 under 25 basename-sharing decoys** | Not preregistered | Measurable on labeled benchmark | **0.778** (above the 0.70 target) |
| **Median Authoring Time** | $\le 5$ minutes | **Not Measured** (Requires human panel) | N/A (Protocol below) |
| **Agent Success Non-Inferiority** | Within 5% of Arm 1 | **Not Measured** (Requires external multi-agent harness) | N/A (Pilot protocol below) |

The measured values above are produced by `pkg/scar/evaluation_test.go` and by `go run ./spike/eval.go`. The evaluation runs through the production retrieval path rather than a parallel reimplementation of the scoring rules, so the numbers describe the shipped tool.

### 4.1 Measured Findings

- **RQ1 is supported.** Precision@3 and Recall@3 are 1.000 on the frozen test split, the no-match false positive rate is 0.0%, and the mean reciprocal rank is 1.000. The tool abstains on 25% of queries, which is the intended behaviour when no candidate reaches the threshold.
- **RQ2 is supported for realistic distractors.** Fifty decoys that overlap in wording but are anchored to unrelated code leave precision unchanged at 1.000. Keyword similarity alone does not produce a retrieval.
- **RQ2 has a bounded cost under an adversarial construction.** Twenty-five decoys that share the *basename* of the target file, while living in an unrelated directory, reduce precision from 1.000 to 0.778. The decoys are retrieved through the same-basename signal combined with task term overlap. This is the direct consequence of the ADR-0010 trade-off: the same-basename fallback exists to recover files that were moved or renamed, and it cannot distinguish a moved file from an unrelated file that happens to share a name. The preregistered precision target of 0.70 still holds under this stress. An earlier revision of this document reported scale invariance by comparing a 10-scar catalogue against the full catalogue; that comparison was invalid, because the smaller catalogue simply did not contain the target scars of every query, and the lower figure measured catalogue coverage rather than retrieval quality.

### 4.2 What Is Not Measured

- **Median authoring time** and **agent task success** are not measurable in this environment. They require the human panel and the multi-agent harness described in section 5. No estimate is offered in their place.
- **Arm 2 and Arm 3 token volumes** for real agent runs are not measured here. `SCENARIO_ANALYSIS.md` reports modelled prompt token footprints, which are heuristic estimates and are explicitly not agent measurements.

---

## 5. Protocols for Unmeasured Items

### 5.1 Protocol for Human Authoring Effort
- **Participants**: 5 professional software engineers.
- **Task**: Given a commit diff that resolved a regression, create a new Scar using `git-scars add from-commit <sha>` and review the resulting draft.
- **Measurement**: Wall-clock duration from command start to finalized, committed Scar. If median $> 5$ minutes, the low-friction capture hypothesis is rejected.
- **Log format and execution**: each session is recorded as `{"engineer", "task_id", "started_at", "finished_at"}` with RFC 3339 timestamps. `git-scars eval-authoring --log <sessions.json>` validates every session and reports the median, the range, and the verdict against the five minute target. The protocol has no recorded sessions yet, so the measurement remains Not Measured.

### 5.2 Protocol for Agent Task Success and Sample Size Calculation
- **Non-inferiority Margin ($\Delta$)**: 5% ($\Delta = 0.05$).
- **Significance Level ($\alpha$)**: 0.05 (one-sided).
- **Power ($1 - \beta$)**: 0.80 (80%).
- **Assumed Baseline Pass Rate ($p$)**: 75% ($0.75$).
- **Sample Size Formula**:
  $$n = \frac{2 \cdot (Z_{1-\alpha} + Z_{1-\beta})^2 \cdot p(1-p)}{\Delta^2}$$
  With $Z_{0.95} = 1.645$ and $Z_{0.80} = 0.842$:
  $$n \approx \frac{2 \cdot (2.487)^2 \cdot (0.75 \cdot 0.25)}{0.0025} \approx \frac{12.37 \cdot 0.1875}{0.0025} \approx 928 \text{ task evaluations}$$
- **Test statistic (paired)**: each task is attempted by both Arm 1 and Arm 3, and the difference $d_i = x_{3,i} - x_{1,i} \in \{-1, 0, +1\}$ is recorded. The comparison uses the paired construction of Newcombe with $w_i = d_i + \Delta$:
  $$z = \sqrt{n} \cdot \frac{\bar{w}}{\operatorname{sd}(w)}$$
  where $\bar{w}$ is the mean of $w_i$ (equal to $\hat{p}_3 - \hat{p}_1 + \Delta$). The null hypothesis $p_3 \le p_1 - \Delta$ is rejected when $z > 1.645$. When the paired variance degenerates to zero, the statistic is $\pm\infty$ depending on the sign of $\hat{p}_3 - \hat{p}_1 + \Delta$. The implementation uses the plan's rounded quantiles, so the arithmetic reproduces the sample size exactly.
- **Corpus format and execution**: the recorded outcomes live in a JSON corpus of `{"task_id", "arm", "passed"}` records, one per task per arm, with the arm names `no_memory`, `flat_monolith`, `all_scars`, and `selective_retrieval`. A task where one of the two compared arms is absent is excluded from the pairing and named in the report. An unknown arm, a duplicate task/arm pair, or a record without a task identifier is rejected rather than entering the statistics.
- **Conclusion**: With $<30$ recurrence tasks currently available, any agent-level evaluation in this phase is strictly labeled a **Pilot Study**. Confirmatory claims are deferred until large-scale test harnesses are executed. The tooling labels the result itself: a corpus with fewer paired evaluations than the preregistered $n$ is reported as a pilot, and no pilot outcome is ever rendered as a confirmatory claim.
- **Agent-level success protocol remains Not Measured**: no agent runs have been recorded on a task corpus, and this repository does not simulate them. The machinery executes the arithmetic on whatever outcomes an external harness records.

---

## 6. Reproducing the Measurement

```bash
# Executable verification against the preregistered thresholds
go test -race ./pkg/scar -run TestRetrievalEvaluation -v

# Human-readable report across both splits and both distractor populations
go run ./spike/eval.go

# Agent-level machinery (executed arithmetic on a recorded corpus, not a
# confirmation): paired non-inferiority of the selective retrieval arm
# against the flat monolith arm, labeled a pilot below 928 pairs
git-scars eval-agents --report <recorded_corpus.json>

# Human authoring protocol summary against the five minute median target
git-scars eval-authoring --log <recorded_sessions.json>
```

The benchmark dataset lives at `pkg/scar/testdata/retrieval_dataset.json` (50 Scars, 20 Queries split into 8 dev, 12 test). It is stored with the package that consumes it so that the labelled data and the code that measures it cannot drift apart unnoticed.
