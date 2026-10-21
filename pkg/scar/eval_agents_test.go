package scar

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAgentSampleSizeMatchesPreregistration reproduces the sample size table
// in EVAL_PLAN.md section 5.2 from the documented quantiles.
func TestAgentSampleSizeMatchesPreregistration(t *testing.T) {
	if got := AgentSampleSize(0.75, 0.05); got != 928 {
		t.Errorf("expected 928 paired evaluations for the preregistered parameters, got %d", got)
	}
	// A wider margin needs fewer evaluations: 2 * 6.185169 * 0.1875 / 0.01.
	if got := AgentSampleSize(0.75, 0.10); got != 232 {
		t.Errorf("expected 232 paired evaluations for a 10%% margin, got %d", got)
	}
	// 2 * 6.185169 * 0.25 / 0.0025 = 1237.03, rounded up.
	if got := AgentSampleSize(0.50, 0.05); got != 1238 {
		t.Errorf("expected 1238 paired evaluations at p=0.5, got %d", got)
	}
	if got := AgentSampleSize(0.75, 0); got != 0 {
		t.Errorf("expected a zero margin to report no sample size, got %d", got)
	}
}

func agentRecord(task, arm string, passed bool) AgentEvalRecord {
	return AgentEvalRecord{TaskID: task, Arm: arm, Passed: passed}
}

// TestEvaluateAgentArmsHandComputedCases verifies the paired statistic against
// arithmetic done by hand.
//
// Case one: n = 20, one difference of +1, nineteen of 0. mean(w) = 0.10,
// the sample variance of w is 0.95/19 = 0.05, so
// z = sqrt(20) * 0.10 / sqrt(0.05) = 2.0 > 1.645: non-inferior.
//
// Case two: n = 20, nineteen differences of 0 and one of -1. mean(d) = -0.05,
// so the effect is exactly at the margin boundary and z = 0: not non-inferior.
//
// Case three: every pair identical (all pass both arms). The differences are
// all zero and the paired variance degenerates to zero; the effect is positive,
// so the statistic is +infinity and the comparison is non-inferior.
func TestEvaluateAgentArmsHandComputedCases(t *testing.T) {
	// Case one: arm 1 fails a single task that arm 3 passes, so the
	// differences are one +1 and nineteen zeros.
	var records []AgentEvalRecord
	tasks := make([]string, 20)
	for i := range tasks {
		tasks[i] = "task-one-" + strings.Repeat("x", i)
		records = append(records,
			agentRecord(tasks[i], AgentArmFlatMonolith, i != 0),
			agentRecord(tasks[i], AgentArmSelectiveRetrieval, true))
	}
	report, err := EvaluateAgentArms(records)
	if err != nil {
		t.Fatalf("evaluation failed: %v", err)
	}
	if report.Paired != 20 || !almostEqual(report.P1, 0.95) || !almostEqual(report.P3, 1) {
		t.Errorf("unexpected paired summary: n=%d p1=%v p3=%v", report.Paired, report.P1, report.P3)
	}
	if !almostEqual(report.ZStat, 2.0) {
		t.Errorf("expected the z statistic 2.0, got %v", report.ZStat)
	}
	if report.Decision != "non_inferior" {
		t.Errorf("expected a non-inferior decision, got %q", report.Decision)
	}
	// Twenty pairs against the preregistered 928 is a pilot.
	if report.Study != "pilot" {
		t.Errorf("expected a pilot study label, got %q", report.Study)
	}
	if report.Required != 928 {
		t.Errorf("expected the report to carry the preregistered sample size, got %d", report.Required)
	}

	// Case two.
	records = nil
	for i := range tasks {
		thirdArmPasses := i != 0
		records = append(records,
			agentRecord(tasks[i], AgentArmFlatMonolith, true),
			agentRecord(tasks[i], AgentArmSelectiveRetrieval, thirdArmPasses))
	}
	report, err = EvaluateAgentArms(records)
	if err != nil {
		t.Fatalf("evaluation failed: %v", err)
	}
	if !almostEqual(report.ZStat, 0) {
		t.Errorf("expected the z statistic 0 at the margin boundary, got %v", report.ZStat)
	}
	if report.Decision != "not_non_inferior" {
		t.Errorf("expected no rejection at the margin boundary, got %q", report.Decision)
	}

	// Case three.
	records = nil
	for i := range tasks {
		records = append(records,
			agentRecord(tasks[i], AgentArmFlatMonolith, true),
			agentRecord(tasks[i], AgentArmSelectiveRetrieval, true))
	}
	report, err = EvaluateAgentArms(records)
	if err != nil {
		t.Fatalf("evaluation failed: %v", err)
	}
	if got := report.ZStat; got <= agentEvalZAlpha {
		t.Errorf("expected an infinite statistic for degenerate paired variance, got %v", got)
	}
}

// TestEvaluateAgentArmsReportsUnpairedTasksAndAllArms verifies that a task
// missing one of the two comparison arms is excluded and named, and that the
// descriptive statistics cover every arm that contributed records.
func TestEvaluateAgentArmsReportsUnpairedTasksAndAllArms(t *testing.T) {
	records := []AgentEvalRecord{
		agentRecord("paired-a", AgentArmFlatMonolith, true),
		agentRecord("paired-a", AgentArmSelectiveRetrieval, true),
		agentRecord("paired-b", AgentArmFlatMonolith, false),
		agentRecord("paired-b", AgentArmSelectiveRetrieval, true),
		agentRecord("orphan-c", AgentArmSelectiveRetrieval, true), // flat monolith absent
		agentRecord("orphan-d", AgentArmAllScars, false),
		agentRecord("orphan-e", AgentArmNoMemory, true),
	}
	report, err := EvaluateAgentArms(records)
	if err != nil {
		t.Fatalf("evaluation failed: %v", err)
	}
	if len(report.Excluded) != 1 || report.Excluded[0] != "orphan-c" {
		t.Errorf("expected orphan-c to be named as excluded, got %q", report.Excluded)
	}
	if report.Paired != 2 {
		t.Fatalf("expected 2 paired tasks, got %d", report.Paired)
	}
	observed := make(map[string]AgentArmObserved)
	for _, o := range report.ArmObserved {
		observed[o.Arm] = o
	}
	if o := observed[AgentArmSelectiveRetrieval]; o.Tasks != 3 || o.Passed != 3 {
		t.Errorf("expected 3 selective retrieval records with 3 passes, got %+v", o)
	}
	if o := observed[AgentArmNoMemory]; o.Tasks != 1 || o.PassRate != 1 {
		t.Errorf("expected the no memory arm to be reported descriptively, got %+v", o)
	}
}

// TestEvaluateAgentArmsLabelsConfirmatoryAtFullScale pins the pilot versus
// confirmatory labelling at the preregistered sample size.
func TestEvaluateAgentArmsLabelsConfirmatoryAtFullScale(t *testing.T) {
	var records []AgentEvalRecord
	for i := 0; i < 928; i++ {
		id := "sweep-" + strings.Repeat("t", i)
		records = append(records,
			agentRecord(id, AgentArmFlatMonolith, i%7 != 0),
			agentRecord(id, AgentArmSelectiveRetrieval, i%8 != 0))
	}
	report, err := EvaluateAgentArms(records)
	if err != nil {
		t.Fatalf("evaluation failed: %v", err)
	}
	if report.Study != "confirmatory" {
		t.Errorf("expected a corpus of 928 pairs to be labeled confirmatory, got %q", report.Study)
	}
}

// TestParseAgentEvalCorpusRejectsMalformedRecords pins the validation contract.
func TestParseAgentEvalCorpusRejectsMalformedRecords(t *testing.T) {
	records, err := ParseAgentEvalCorpus([]byte(`{"results":[{"task_id":"t","arm":"flat_monolith","passed":true}]}`))
	if err != nil {
		t.Fatalf("expected a well-formed single record corpus to parse, got %v", err)
	}
	if len(records) != 1 {
		t.Errorf("expected 1 record, got %d", len(records))
	}

	_, err = ParseAgentEvalCorpus([]byte(`{"results":[{"task_id":"t","arm":"matrix_decoys","passed":true}]}`))
	if err == nil || !strings.Contains(err.Error(), "unknown arm") {
		t.Errorf("expected an unknown arm rejection, got %v", err)
	}

	_, err = ParseAgentEvalCorpus([]byte(`{"results":[
		{"task_id":"t","arm":"flat_monolith","passed":true},
		{"task_id":"t","arm":"flat_monolith","passed":false}]}`))
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("expected a duplicate pair rejection, got %v", err)
	}

	_, err = ParseAgentEvalCorpus([]byte(`{"results":[{"task_id":"","arm":"flat_monolith","passed":true}]}`))
	if err == nil || !strings.Contains(err.Error(), "no task id") {
		t.Errorf("expected an empty task id rejection, got %v", err)
	}

	_, err = ParseAgentEvalCorpus([]byte(`[]`))
	if err == nil || !strings.Contains(err.Error(), "no evaluation records") {
		t.Errorf("expected an empty corpus rejection, got %v", err)
	}

	_, err = ParseAgentEvalCorpus([]byte(`{"nope": 1}`))
	if err == nil {
		t.Errorf("expected a structurally invalid corpus rejection, got %v", err)
	}
}

// TestEvaluateAgentArmsRejectsTooFewPairs verifies the minimum corpus rule.
func TestEvaluateAgentArmsRejectsTooFewPairs(t *testing.T) {
	_, err := EvaluateAgentArms([]AgentEvalRecord{
		agentRecord("only", AgentArmFlatMonolith, true),
		agentRecord("only", AgentArmSelectiveRetrieval, true),
	})
	if err == nil || !strings.Contains(err.Error(), "at least 2") {
		t.Errorf("expected a rejection for a single pair, got %v", err)
	}
}

// TestEvaluateAuthoringLogComputesMedianAndVerdict verifies the 5.1 protocol
// summary against hand-computed medians.
func TestEvaluateAuthoringLogComputesMedianAndVerdict(t *testing.T) {
	entries := []AgentAuthoringEntry{
		{Engineer: "e1", TaskID: "t1", StartedAt: "2026-10-20T10:00:00Z", FinishedAt: "2026-10-20T10:04:00Z"},
		{Engineer: "e2", TaskID: "t2", StartedAt: "2026-10-20T11:00:00Z", FinishedAt: "2026-10-20T11:07:00Z"},
		{Engineer: "e3", TaskID: "t3", StartedAt: "2026-10-20T12:00:00Z", FinishedAt: "2026-10-20T12:03:30Z"},
	}
	report, err := EvaluateAuthoringLog(entries)
	if err != nil {
		t.Fatalf("evaluation failed: %v", err)
	}
	if report.Sessions != 3 || report.MedianMinutes != 4 || report.MinMinutes != 3.5 || report.MaxMinutes != 7 {
		t.Errorf("unexpected authoring summary: %+v", report)
	}
	if report.Verdict != "target_met" {
		t.Errorf("expected the target to be met at a median of 4, got %q", report.Verdict)
	}

	entries = append(entries, AgentAuthoringEntry{Engineer: "e4", TaskID: "t4", StartedAt: "2026-10-20T13:00:00Z", FinishedAt: "2026-10-20T13:09:00Z"})
	report, err = EvaluateAuthoringLog(entries)
	if err != nil {
		t.Fatalf("evaluation failed: %v", err)
	}
	if report.MedianMinutes != 5.5 || report.Verdict != "target_exceeded" {
		t.Errorf("expected a median of 5.5 to exceed the target, got %+v", report)
	}
	if report.TargetMinutes != 5 {
		t.Errorf("expected the target to be carried in the report, got %+v", report)
	}
}

func TestEvaluateAuthoringLogRejectsMalformedSessions(t *testing.T) {
	if _, err := EvaluateAuthoringLog(nil); err == nil || !strings.Contains(err.Error(), "no sessions") {
		t.Errorf("expected an empty log rejection, got %v", err)
	}
	_, err := EvaluateAuthoringLog([]AgentAuthoringEntry{{TaskID: "t", StartedAt: "not-a-time", FinishedAt: "2026-10-20T10:00:00Z"}})
	if err == nil || !strings.Contains(err.Error(), "RFC 3339") {
		t.Errorf("expected a timestamp rejection, got %v", err)
	}
	_, err = EvaluateAuthoringLog([]AgentAuthoringEntry{{TaskID: "t", StartedAt: "2026-10-20T10:05:00Z", FinishedAt: "2026-10-20T10:00:00Z"}})
	if err == nil || !strings.Contains(err.Error(), "finishes before it starts") {
		t.Errorf("expected a reversed interval rejection, got %v", err)
	}
}

// TestLoadAgentEvalCorpusReadsFiles covers the file entry point and its error.
func TestLoadAgentEvalCorpusReadsFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent_corpus.json")
	body := `{"results":[
		{"task_id":"a","arm":"flat_monolith","passed":true},
		{"task_id":"a","arm":"selective_retrieval","passed":true},
		{"task_id":"b","arm":"flat_monolith","passed":false},
		{"task_id":"b","arm":"selective_retrieval","passed":true},
		{"task_id":"c","arm":"all_scars","passed":true}
	]}`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("failed writing corpus: %v", err)
	}
	records, err := LoadAgentEvalCorpus(path)
	if err != nil {
		t.Fatalf("failed loading corpus: %v", err)
	}
	if len(records) != 5 {
		t.Errorf("expected 5 records, got %d", len(records))
	}

	if _, err := LoadAgentEvalCorpus(filepath.Join(t.TempDir(), "absent.json")); err == nil {
		t.Error("expected a rejection for a missing corpus file")
	}
}

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}
