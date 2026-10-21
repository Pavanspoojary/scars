// Command eval reports the measured retrieval quality of the GitScars
// implementation against a labelled benchmark.
//
// The evaluation runs through the production retrieval path. It does not
// reimplement the scoring rules: a parallel implementation would measure a
// copy of the algorithm rather than the algorithm itself, and the two would
// drift apart.
package main

import (
	"fmt"
	"os"

	"github.com/Pavanspoojary/scars/pkg/scar"
)

const defaultDataset = "pkg/scar/testdata/retrieval_dataset.json"

func main() {
	path := defaultDataset
	if len(os.Args) > 1 {
		path = os.Args[1]
	}

	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed reading dataset: %v\n", err)
		os.Exit(1)
	}

	dataset, err := scar.LoadEvaluationDataset(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed loading dataset: %v\n", err)
		os.Exit(1)
	}

	workdir, err := os.MkdirTemp("", "gitscars-eval-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed creating working directory: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(workdir)

	store := scar.NewStore(workdir)
	if err := dataset.Materialize(store); err != nil {
		fmt.Fprintf(os.Stderr, "failed materializing benchmark: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("=== GitScars Retrieval Evaluation ===")
	fmt.Printf("Dataset: %s\n", path)
	fmt.Printf("Scars: %d | Queries: %d\n\n", len(dataset.Scars), len(dataset.Queries))

	for _, split := range []string{"dev", "test"} {
		queries := dataset.QueriesForSplit(split)
		if len(queries) == 0 {
			continue
		}
		fmt.Printf("--- %s split (threshold >= 3, top_k = 3) ---\n", split)
		printMetrics(scar.EvaluateRetrieval(store, queries, 3, 3))
	}

	fmt.Println("\n--- Distractor invariance (test split) ---")
	queries := dataset.QueriesForSplit("test")

	baselineStore := scar.NewStore(mustTempDir())
	defer os.RemoveAll(baselineStore.RepoRoot)
	if err := dataset.Materialize(baselineStore); err != nil {
		fmt.Fprintf(os.Stderr, "failed materializing benchmark: %v\n", err)
		os.Exit(1)
	}
	baseline := scar.EvaluateRetrieval(baselineStore, queries, 3, 3)

	realisticStore := scar.NewStore(mustTempDir())
	defer os.RemoveAll(realisticStore.RepoRoot)
	realistic := dataset
	realistic.Scars = append(append([]scar.EvaluationScar{}, dataset.Scars...), scar.DistractorScars(*dataset, 50)...)
	if err := realistic.Materialize(realisticStore); err != nil {
		fmt.Fprintf(os.Stderr, "failed materializing benchmark: %v\n", err)
		os.Exit(1)
	}
	realisticMetrics := scar.EvaluateRetrieval(realisticStore, queries, 3, 3)

	adversarialStore := scar.NewStore(mustTempDir())
	defer os.RemoveAll(adversarialStore.RepoRoot)
	adversarial := dataset
	adversarial.Scars = append(append([]scar.EvaluationScar{}, dataset.Scars...), scar.DistractorScarsWithSharedBasename(*dataset, 25)...)
	if err := adversarial.Materialize(adversarialStore); err != nil {
		fmt.Fprintf(os.Stderr, "failed materializing benchmark: %v\n", err)
		os.Exit(1)
	}
	adversarialMetrics := scar.EvaluateRetrieval(adversarialStore, queries, 3, 3)

	fmt.Printf("Baseline:                    Precision@3 %.3f | No-Match FP %.1f%%\n",
		baseline.PrecisionAt3, baseline.NoMatchFPRate*100)
	fmt.Printf("50 keyword distractors:      Precision@3 %.3f | No-Match FP %.1f%%\n",
		realisticMetrics.PrecisionAt3, realisticMetrics.NoMatchFPRate*100)
	fmt.Printf("25 basename-sharing decoys:  Precision@3 %.3f | No-Match FP %.1f%%\n",
		adversarialMetrics.PrecisionAt3, adversarialMetrics.NoMatchFPRate*100)
}

func mustTempDir() string {
	dir, err := os.MkdirTemp("", "gitscars-eval-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed creating working directory: %v\n", err)
		os.Exit(1)
	}
	return dir
}

func printMetrics(m scar.EvaluationMetrics) {
	fmt.Printf("  Total Queries:         %d\n", m.TotalQueries)
	fmt.Printf("  Match Queries:         %d\n", m.MatchQueries)
	fmt.Printf("  No-Match Queries:      %d\n", m.NoMatchQueries)
	fmt.Printf("  Precision@3:           %.3f (Target: >= 0.700)\n", m.PrecisionAt3)
	fmt.Printf("  Recall@3:              %.3f\n", m.RecallAt3)
	fmt.Printf("  Mean Reciprocal Rank:  %.3f\n", m.MeanReciprocalRank)
	fmt.Printf("  No-Match FP Rate:      %.1f%% (Target: <= 10.0%%)\n", m.NoMatchFPRate*100)
	fmt.Printf("  Abstain Rate:          %.1f%%\n", m.AbstainRate*100)
}
