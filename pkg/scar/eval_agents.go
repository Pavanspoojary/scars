package scar

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"
)

// The agent-level evaluation machinery implements the preregistered protocol
// in EVAL_PLAN.md section 5.2. It computes the paired one-sided
// non-inferiority comparison of the selective retrieval arm against the flat
// monolith arm, the exact sample size the protocol requires, and the median
// of a human authoring log. It never fabricates arm outcomes: every number it
// returns is derived from the recorded corpus handed to it.

// Agent evaluation arm identifiers from EVAL_PLAN.md section 2.
const (
	AgentArmNoMemory           = "no_memory"
	AgentArmFlatMonolith       = "flat_monolith"
	AgentArmAllScars           = "all_scars"
	AgentArmSelectiveRetrieval = "selective_retrieval"
)

// Preregistered decision parameters from EVAL_PLAN.md section 5.2. The two
// quantiles are the rounded values the plan documents (1.645 and 0.842), so
// the arithmetic reproduces the plan's sample size exactly.
const (
	AgentEvalDelta    = 0.05 // non-inferiority margin
	AgentEvalAlpha    = 0.05 // one-sided significance level
	AgentEvalPower    = 0.80 // statistical power
	AgentEvalAssumedP = 0.75 // assumed baseline pass rate

	agentEvalZAlpha = 1.645 // Z_{1-alpha} at alpha = 0.05
	agentEvalZPower = 0.842 // Z_{1-beta} at power = 0.80

	// AgentAuthoringTargetMinutes is the median authoring target from the
	// low-friction capture hypothesis in EVAL_PLAN.md section 5.1.
	AgentAuthoringTargetMinutes = 5
)

// AgentEvalRecord is one task evaluation: a single arm attempted a single
// task, and the attempt either passed or failed.
type AgentEvalRecord struct {
	TaskID string `json:"task_id"`
	Arm    string `json:"arm"`
	Passed bool   `json:"passed"`
}

type agentEvalCorpus struct {
	Results []AgentEvalRecord `json:"results"`
}

// AgentSampleSize returns the preregistered sample size for the given assumed
// pass rate and margin, using the plan's quantiles and rounding up.
func AgentSampleSize(assumedP, delta float64) int {
	if delta <= 0 {
		return 0
	}
	n := 2 * (agentEvalZAlpha + agentEvalZPower) * (agentEvalZAlpha + agentEvalZPower) * assumedP * (1 - assumedP) / (delta * delta)
	return int(math.Ceil(n))
}

// AgentArmObserved is the descriptive pass rate of one arm.
type AgentArmObserved struct {
	Arm      string
	Tasks    int
	Passed   int
	PassRate float64
}

// AgentEvalReport is the outcome of the paired comparison and the
// descriptive statistics for every recorded arm.
type AgentEvalReport struct {
	Paired   int      // number of task pairs with both arms recorded
	Excluded []string // tasks where one of the two arms is absent
	P1       float64  // flat monolith pass rate over paired tasks
	P3       float64  // selective retrieval pass rate over paired tasks
	Delta    float64  // preregistered margin
	ZStat    float64  // paired one-sided z statistic
	ZAlpha   float64  // decision threshold, 1.645 at alpha = 0.05
	Required int      // preregistered sample size at the plan's assumptions
	Decision string   // "non_inferior" or "not_non_inferior"
	Study    string   // "pilot" or "confirmatory"

	// Descriptive pass rates for every recorded arm. Arms outside the
	// preregistered comparison are reported here for context only.
	ArmObserved []AgentArmObserved
}

// ParseAgentEvalCorpus reads a recorded corpus from JSON bytes. It rejects
// unknown arms, duplicate task/arm pairs, and records without a task, so a
// mislabelled log fails loudly instead of entering the statistics.
func ParseAgentEvalCorpus(data []byte) ([]AgentEvalRecord, error) {
	var records []AgentEvalRecord
	var corpus agentEvalCorpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		var bare []AgentEvalRecord
		if err2 := json.Unmarshal(data, &bare); err2 != nil {
			return nil, fmt.Errorf("corpus is neither an object with \"results\" nor an array: %w", err)
		}
		records = bare
	} else {
		records = corpus.Results
	}
	if len(records) == 0 {
		return nil, errors.New("corpus holds no evaluation records")
	}

	knownArms := map[string]bool{
		AgentArmNoMemory:           true,
		AgentArmFlatMonolith:       true,
		AgentArmAllScars:           true,
		AgentArmSelectiveRetrieval: true,
	}
	seen := make(map[string]bool)
	for i, r := range records {
		if strings.TrimSpace(r.TaskID) == "" {
			return nil, fmt.Errorf("record %d carries no task id", i)
		}
		if !knownArms[r.Arm] {
			return nil, fmt.Errorf("record %d names unknown arm %q", i, r.Arm)
		}
		key := r.TaskID + "\x00" + r.Arm
		if seen[key] {
			return nil, fmt.Errorf("task %q has a duplicate evaluation for arm %q", r.TaskID, r.Arm)
		}
		seen[key] = true
	}
	return records, nil
}

// LoadAgentEvalCorpus reads and validates a corpus file.
func LoadAgentEvalCorpus(path string) ([]AgentEvalRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed reading agent eval corpus %s: %w", path, err)
	}
	return ParseAgentEvalCorpus(data)
}

// EvaluateAgentArms compares the selective retrieval arm against the flat
// monolith arm under the preregistered paired one-sided non-inferiority test.
//
// For every task attempted by both arms the difference is
//
//	d_i = x3_i - x1_i in {-1, 0, +1}
//
// and the statistic follows the paired construction of Newcombe: w_i = d_i +
// delta with the sample variance of w over the pairs, so
//
//	z = sqrt(n) * (mean(d_i) + delta) / sd(w_i)
//
// The null hypothesis p3 <= p1 - delta is rejected at alpha when z exceeds
// Z_{1-alpha}. A corpus with fewer pairs than the preregistered sample size is
// labeled a pilot and can never support a confirmatory claim.
func EvaluateAgentArms(records []AgentEvalRecord) (*AgentEvalReport, error) {
	pass := make(map[string]map[string]bool)
	byArm := make(map[string][]bool)
	for _, r := range records {
		if pass[r.TaskID] == nil {
			pass[r.TaskID] = make(map[string]bool)
		}
		if _, seen := pass[r.TaskID][r.Arm]; seen {
			return nil, fmt.Errorf("task %q has a duplicate evaluation for arm %q", r.TaskID, r.Arm)
		}
		pass[r.TaskID][r.Arm] = r.Passed
		byArm[r.Arm] = append(byArm[r.Arm], r.Passed)
	}

	report := &AgentEvalReport{
		Delta:  AgentEvalDelta,
		ZAlpha: agentEvalZAlpha,
	}
	report.Required = AgentSampleSize(AgentEvalAssumedP, AgentEvalDelta)

	var d []float64
	var sumMono, sumSel float64
	for taskID, arms := range pass {
		mono, hasMono := arms[AgentArmFlatMonolith]
		sel, hasSel := arms[AgentArmSelectiveRetrieval]
		if hasMono != hasSel {
			report.Excluded = append(report.Excluded, taskID)
			continue
		}
		if !hasMono {
			continue
		}
		di := boolWeight(sel) - boolWeight(mono)
		d = append(d, di)
		sumMono += boolWeight(mono)
		sumSel += boolWeight(sel)
	}
	sort.Strings(report.Excluded)
	report.Paired = len(d)

	if report.Paired < 2 {
		return nil, fmt.Errorf("the corpus holds %d paired task(s); the paired comparison needs at least 2", report.Paired)
	}

	n := float64(report.Paired)
	report.P1 = sumMono / n
	report.P3 = sumSel / n

	var sumW, sumWW float64
	for _, di := range d {
		w := di + AgentEvalDelta
		sumW += w
		sumWW += w * w
	}
	meanW := sumW / n
	meanD := meanW - AgentEvalDelta
	variance := (sumWW - n*meanW*meanW) / (n - 1)

	effect := meanD + AgentEvalDelta
	switch {
	case variance <= 0:
		switch {
		case effect > 0:
			report.ZStat = math.Inf(1)
		case effect < 0:
			report.ZStat = math.Inf(-1)
		default:
			report.ZStat = 0
		}
	default:
		report.ZStat = effect / math.Sqrt(variance/n)
	}

	if report.ZStat > agentEvalZAlpha {
		report.Decision = "non_inferior"
	} else {
		report.Decision = "not_non_inferior"
	}
	if report.Paired >= report.Required {
		report.Study = "confirmatory"
	} else {
		report.Study = "pilot"
	}

	for arm, outcomes := range byArm {
		passed := 0
		for _, ok := range outcomes {
			if ok {
				passed++
			}
		}
		report.ArmObserved = append(report.ArmObserved, AgentArmObserved{
			Arm:      arm,
			Tasks:    len(outcomes),
			Passed:   passed,
			PassRate: float64(passed) / float64(len(outcomes)),
		})
	}
	sort.Slice(report.ArmObserved, func(i, j int) bool {
		return report.ArmObserved[i].Arm < report.ArmObserved[j].Arm
	})

	return report, nil
}

func boolWeight(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// AgentAuthoringEntry records one authoring session for the EVAL_PLAN 5.1
// protocol: an engineer turning a resolved regression into a committed scar.
type AgentAuthoringEntry struct {
	Engineer   string `json:"engineer"`
	TaskID     string `json:"task_id"`
	StartedAt  string `json:"started_at"`  // RFC 3339
	FinishedAt string `json:"finished_at"` // RFC 3339
}

// AgentAuthoringReport summarises an authoring log against the preregistered
// five minute target.
type AgentAuthoringReport struct {
	Sessions      int
	MedianMinutes float64
	MinMinutes    float64
	MaxMinutes    float64
	TargetMinutes float64
	Verdict       string // "target_met" or "target_exceeded"
}

// EvaluateAuthoringLog validates an authoring log and computes the summary
// statistics of the 5.1 protocol.
func EvaluateAuthoringLog(entries []AgentAuthoringEntry) (*AgentAuthoringReport, error) {
	if len(entries) == 0 {
		return nil, errors.New("authoring log holds no sessions")
	}

	durationBySession := make([]float64, 0, len(entries))
	for i, e := range entries {
		started, err := time.Parse(time.RFC3339, e.StartedAt)
		if err != nil {
			return nil, fmt.Errorf("session %d (task %q) records its start as %q, which is not RFC 3339", i, e.TaskID, e.StartedAt)
		}
		finished, err := time.Parse(time.RFC3339, e.FinishedAt)
		if err != nil {
			return nil, fmt.Errorf("session %d (task %q) records its finish as %q, which is not RFC 3339", i, e.TaskID, e.FinishedAt)
		}
		if finished.Before(started) {
			return nil, fmt.Errorf("session %d (task %q) finishes before it starts", i, e.TaskID)
		}
		durationBySession = append(durationBySession, finished.Sub(started).Minutes())
	}
	sort.Float64s(durationBySession)

	report := &AgentAuthoringReport{
		Sessions:      len(entries),
		TargetMinutes: AgentAuthoringTargetMinutes,
		MinMinutes:    durationBySession[0],
		MaxMinutes:    durationBySession[len(durationBySession)-1],
	}
	report.MedianMinutes = medianOf(durationBySession)
	if report.MedianMinutes <= AgentAuthoringTargetMinutes {
		report.Verdict = "target_met"
	} else {
		report.Verdict = "target_exceeded"
	}
	return report, nil
}

func medianOf(sorted []float64) float64 {
	n := len(sorted)
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}

// LoadAgentAuthoringLog reads an authoring log from JSON bytes.
func LoadAgentAuthoringLog(path string) ([]AgentAuthoringEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed reading authoring log %s: %w", path, err)
	}
	var entries []AgentAuthoringEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("authoring log is not an array of sessions: %w", err)
	}
	return entries, nil
}
