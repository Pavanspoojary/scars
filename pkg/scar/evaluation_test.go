package scar

import (
	"os"
	"path/filepath"
	"testing"
)

const (
	// Preregistered retrieval thresholds from EVAL_PLAN.md.
	preregisteredPrecisionAt3  = 0.70
	preregisteredNoMatchFPRate = 0.10
)

func loadBenchmark(t *testing.T) EvaluationDataset {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "retrieval_dataset.json"))
	if err != nil {
		t.Fatalf("failed reading benchmark dataset: %v", err)
	}
	ds, err := LoadEvaluationDataset(data)
	if err != nil {
		t.Fatalf("failed loading benchmark dataset: %v", err)
	}
	return *ds
}

func materializeBenchmark(t *testing.T, dataset EvaluationDataset, extra []EvaluationScar) *Store {
	t.Helper()
	store := NewStore(t.TempDir())
	for _, d := range extra {
		dataset.Scars = append(dataset.Scars, d)
	}
	if err := dataset.Materialize(store); err != nil {
		t.Fatalf("failed materializing benchmark: %v", err)
	}
	return store
}

// TestRetrievalEvaluationMeetsPreregisteredThresholds verifies the frozen
// test split against the thresholds registered in EVAL_PLAN.md.
func TestRetrievalEvaluationMeetsPreregisteredThresholds(t *testing.T) {
	dataset := loadBenchmark(t)
	store := materializeBenchmark(t, dataset, nil)

	queries := dataset.QueriesForSplit("test")
	if len(queries) == 0 {
		t.Fatal("expected a non-empty test split")
	}

	m := EvaluateRetrieval(store, queries, 3, 3)
	t.Logf("test split: precision@3=%.3f recall@3=%.3f no-match fp=%.1f%% abstain=%.1f%% mrr=%.3f",
		m.PrecisionAt3, m.RecallAt3, m.NoMatchFPRate*100, m.AbstainRate*100, m.MeanReciprocalRank)

	if m.PrecisionAt3 < preregisteredPrecisionAt3 {
		t.Errorf("precision@3 %.3f is below the preregistered target of %.2f", m.PrecisionAt3, preregisteredPrecisionAt3)
	}
	if m.NoMatchFPRate > preregisteredNoMatchFPRate {
		t.Errorf("no-match false positive rate %.3f exceeds the preregistered limit of %.2f", m.NoMatchFPRate, preregisteredNoMatchFPRate)
	}
	if m.RecallAt3 < preregisteredPrecisionAt3 {
		t.Errorf("recall@3 %.3f is below the preregistered target of %.2f", m.RecallAt3, preregisteredPrecisionAt3)
	}
}

// TestRetrievalEvaluationReportsDevelopmentSplit records the development split
// metrics. The development split is used for calibration and is not a
// preregistered claim.
func TestRetrievalEvaluationReportsDevelopmentSplit(t *testing.T) {
	dataset := loadBenchmark(t)
	store := materializeBenchmark(t, dataset, nil)

	queries := dataset.QueriesForSplit("dev")
	if len(queries) == 0 {
		t.Fatal("expected a non-empty development split")
	}

	m := EvaluateRetrieval(store, queries, 3, 3)
	t.Logf("dev split: precision@3=%.3f recall@3=%.3f no-match fp=%.1f%% abstain=%.1f%%",
		m.PrecisionAt3, m.RecallAt3, m.NoMatchFPRate*100, m.AbstainRate*100)
}

// TestRetrievalEvaluationIsInvariantToDistractors answers RQ2: keyword
// overlapping decoys whose code anchors differ must not degrade precision.
func TestRetrievalEvaluationIsInvariantToDistractors(t *testing.T) {
	dataset := loadBenchmark(t)
	queries := dataset.QueriesForSplit("test")

	baselineStore := materializeBenchmark(t, dataset, nil)
	baseline := EvaluateRetrieval(baselineStore, queries, 3, 3)

	distractorStore := materializeBenchmark(t, dataset, DistractorScars(dataset, 50))
	withDistractors := EvaluateRetrieval(distractorStore, queries, 3, 3)

	t.Logf("baseline precision@3=%.3f | with 50 realistic distractors precision@3=%.3f",
		baseline.PrecisionAt3, withDistractors.PrecisionAt3)

	if withDistractors.PrecisionAt3 < baseline.PrecisionAt3 {
		t.Errorf("precision degraded from %.3f to %.3f after adding distractors",
			baseline.PrecisionAt3, withDistractors.PrecisionAt3)
	}
	if withDistractors.NoMatchFPRate > preregisteredNoMatchFPRate {
		t.Errorf("no-match false positive rate %.3f exceeds the preregistered limit of %.2f",
			withDistractors.NoMatchFPRate, preregisteredNoMatchFPRate)
	}
}

// TestRetrievalEvaluationUnderAdversarialBasenameDistractors measures the cost
// of the same-basename fallback that ADR-0010 introduced for moved files. The
// decoys share the basename of the target file but live in an unrelated
// directory, so they can be retrieved through the basename signal combined
// with task term overlap.
//
// The preregistered precision threshold must still hold. The measured value is
// recorded so that the cost of the fallback stays visible.
func TestRetrievalEvaluationUnderAdversarialBasenameDistractors(t *testing.T) {
	dataset := loadBenchmark(t)
	queries := dataset.QueriesForSplit("test")

	baselineStore := materializeBenchmark(t, dataset, nil)
	baseline := EvaluateRetrieval(baselineStore, queries, 3, 3)

	distractorStore := materializeBenchmark(t, dataset, DistractorScarsWithSharedBasename(dataset, 25))
	withDistractors := EvaluateRetrieval(distractorStore, queries, 3, 3)

	t.Logf("baseline precision@3=%.3f | with 25 basename-sharing distractors precision@3=%.3f",
		baseline.PrecisionAt3, withDistractors.PrecisionAt3)

	if withDistractors.PrecisionAt3 < preregisteredPrecisionAt3 {
		t.Errorf("precision@3 %.3f fell below the preregistered target of %.2f under adversarial distractors",
			withDistractors.PrecisionAt3, preregisteredPrecisionAt3)
	}
	if withDistractors.NoMatchFPRate > preregisteredNoMatchFPRate {
		t.Errorf("no-match false positive rate %.3f exceeds the preregistered limit of %.2f",
			withDistractors.NoMatchFPRate, preregisteredNoMatchFPRate)
	}
}
