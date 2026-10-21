package scar

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EvaluationDataset is a labelled retrieval benchmark.
type EvaluationDataset struct {
	Scars   []EvaluationScar  `json:"scars"`
	Queries []EvaluationQuery `json:"queries"`
}

// EvaluationScar is a labelled scar in the benchmark.
type EvaluationScar struct {
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	Files          []string `json:"files"`
	Symbols        []string `json:"symbols"`
	Dependencies   []string `json:"dependencies"`
	Commands       []string `json:"commands"`
	ErrorSignature string   `json:"error_signature"`
	Lesson         string   `json:"lesson"`
	Status         string   `json:"status"`
}

// EvaluationQuery is a labelled retrieval query.
type EvaluationQuery struct {
	ID              string   `json:"id"`
	Description     string   `json:"description"`
	Split           string   `json:"split"`
	Task            string   `json:"task"`
	Files           []string `json:"files"`
	Symbols         []string `json:"symbols"`
	Commands        []string `json:"commands"`
	ErrorSignature  string   `json:"error_signature"`
	ExpectedScarIDs []string `json:"expected_scar_ids"`
}

// EvaluationMetrics holds the measured retrieval quality of one evaluation run.
type EvaluationMetrics struct {
	TotalQueries       int     `json:"total_queries"`
	MatchQueries       int     `json:"match_queries"`
	NoMatchQueries     int     `json:"no_match_queries"`
	AbstainCount       int     `json:"abstain_count"`
	AbstainRate        float64 `json:"abstain_rate"`
	FalsePositives     int     `json:"false_positives"`
	NoMatchFPRate      float64 `json:"no_match_fp_rate"`
	PrecisionAt3       float64 `json:"precision_at_3"`
	RecallAt3          float64 `json:"recall_at_3"`
	MeanReciprocalRank float64 `json:"mrr"`
}

// LoadEvaluationDataset parses a labelled benchmark from JSON.
func LoadEvaluationDataset(data []byte) (*EvaluationDataset, error) {
	var ds EvaluationDataset
	if err := json.Unmarshal(data, &ds); err != nil {
		return nil, fmt.Errorf("failed parsing evaluation dataset: %w", err)
	}
	return &ds, nil
}

// QueriesForSplit returns the queries belonging to one split.
func (d *EvaluationDataset) QueriesForSplit(split string) []EvaluationQuery {
	var out []EvaluationQuery
	for _, q := range d.Queries {
		if q.Split == split {
			out = append(out, q)
		}
	}
	return out
}

// toScar converts a labelled benchmark scar into the production representation.
// The verifier evidence is made internally consistent so that the record
// satisfies the same invariants as a recorded scar.
func (e EvaluationScar) toScar() *Scar {
	deps := make([]DependencyAnchor, 0, len(e.Dependencies))
	for _, d := range e.Dependencies {
		name, version, _ := strings.Cut(d, "@")
		deps = append(deps, DependencyAnchor{Name: name, Version: version})
	}

	var argv []string
	if len(e.Commands) > 0 {
		argv = strings.Fields(e.Commands[0])
	}

	state := LifecycleDraft
	if e.Status == LifecycleActive {
		state = LifecycleActive
	}

	return &Scar{
		SchemaVersion: CurrentSchemaVersion,
		ID:            e.ID,
		Title:         e.Title,
		Lesson:        e.Lesson,
		Anchors: Anchors{
			Files:           e.Files,
			Symbols:         e.Symbols,
			Dependencies:    deps,
			Commands:        e.Commands,
			ErrorSignatures: []string{e.ErrorSignature},
		},
		Verifier: Verifier{
			Argv:        argv,
			Result:      VerificationFailToPass,
			CommandHash: ComputeCommandHash(argv, "", nil),
		},
		Review: Review{Status: ReviewApproved},
		Lifecycle: Lifecycle{
			State: state,
		},
	}
}

// Materialize writes the benchmark scars into a store so that retrieval is
// measured through the production path rather than a parallel implementation.
func (d *EvaluationDataset) Materialize(store *Store) error {
	if err := store.Init(); err != nil {
		return err
	}
	for _, e := range d.Scars {
		if _, err := store.SaveScar(e.toScar()); err != nil {
			return fmt.Errorf("failed materializing benchmark scar %s: %w", e.ID, err)
		}
	}
	if _, err := store.RebuildIndex(); err != nil {
		return fmt.Errorf("failed rebuilding benchmark index: %w", err)
	}
	return nil
}

// EvaluateRetrieval measures retrieval quality over the given queries using the
// production retrieval implementation.
//
// Freshness is deliberately not under evaluation: benchmark scars carry no
// baseline, so they would be filtered out by the freshness gate before any
// ranking decision is made. The benchmark therefore measures ranking quality.
func EvaluateRetrieval(store *Store, queries []EvaluationQuery, threshold, topK int) EvaluationMetrics {
	m := EvaluationMetrics{TotalQueries: len(queries)}

	var totalPrecision, totalRecall, sumRR float64
	matchQueries := 0

	for _, q := range queries {
		results, err := store.Recall(QueryOptions{
			Task:           q.Task,
			Files:          q.Files,
			Symbols:        q.Symbols,
			Commands:       q.Commands,
			ErrorSignature: q.ErrorSignature,
			IncludeStale:   true,
			Threshold:      threshold,
			TopK:           topK,
		})
		if err != nil {
			continue
		}

		if len(results) == 0 {
			m.AbstainCount++
		}

		if len(q.ExpectedScarIDs) == 0 {
			m.NoMatchQueries++
			if len(results) > 0 {
				m.FalsePositives++
			}
			continue
		}

		matchQueries++
		expected := make(map[string]bool, len(q.ExpectedScarIDs))
		for _, id := range q.ExpectedScarIDs {
			expected[id] = true
		}

		hits := 0
		firstRank := 0
		for i, r := range results {
			if expected[r.Scar.ID] {
				hits++
				if firstRank == 0 {
					firstRank = i + 1
				}
			}
		}

		if len(results) > 0 {
			totalPrecision += float64(hits) / float64(len(results))
		}
		totalRecall += float64(hits) / float64(len(q.ExpectedScarIDs))
		if firstRank > 0 {
			sumRR += 1.0 / float64(firstRank)
		}
	}

	if m.TotalQueries > 0 {
		m.AbstainRate = float64(m.AbstainCount) / float64(m.TotalQueries)
	}
	if m.NoMatchQueries > 0 {
		m.NoMatchFPRate = float64(m.FalsePositives) / float64(m.NoMatchQueries)
	}
	if matchQueries > 0 {
		m.PrecisionAt3 = totalPrecision / float64(matchQueries)
		m.RecallAt3 = totalRecall / float64(matchQueries)
		m.MeanReciprocalRank = sumRR / float64(matchQueries)
	}
	m.MatchQueries = matchQueries

	return m
}

// DistractorScars generates decoy scars that overlap in wording with the
// labelled scars but whose code anchors are disjoint. They model the realistic
// case of a second scar about a similar subject in an unrelated part of the
// codebase, and they exist to test whether keyword similarity alone produces
// false retrievals.
func DistractorScars(dataset EvaluationDataset, count int) []EvaluationScar {
	return buildDistractors(dataset, count, false)
}

// DistractorScarsWithSharedBasename generates decoy scars whose anchored files
// share the basename of the labelled target files while living in unrelated
// directories. This is the adversarial form of RQ2: it isolates the cost of
// the same-basename fallback that ADR-0010 introduced for moved files.
func DistractorScarsWithSharedBasename(dataset EvaluationDataset, count int) []EvaluationScar {
	return buildDistractors(dataset, count, true)
}

func buildDistractors(dataset EvaluationDataset, count int, sharedBasename bool) []EvaluationScar {
	if len(dataset.Scars) == 0 {
		return nil
	}
	out := make([]EvaluationScar, 0, count)
	for i := 0; i < count; i++ {
		src := dataset.Scars[i%len(dataset.Scars)]
		src.ID = fmt.Sprintf("distractor-%03d", i+1)

		if sharedBasename {
			// Keep the basename of the source anchor and change only the
			// directory, which is what a file move looks like.
			base := "component"
			if len(src.Files) > 0 {
				base = filepath.Base(src.Files[0])
			}
			src.Files = []string{filepath.Join(fmt.Sprintf("internal/distractor/%03d", i+1), base)}
		} else {
			src.Files = []string{fmt.Sprintf("internal/distractor/%03d/component_%03d.go", i+1, i+1)}
		}

		src.Symbols = nil
		src.Commands = nil
		src.Dependencies = nil
		src.ErrorSignature = ""
		out = append(out, src)
	}
	return out
}

// WriteEvaluationDataset writes a dataset to disk in the benchmark format.
func WriteEvaluationDataset(path string, dataset EvaluationDataset) error {
	data, err := json.MarshalIndent(dataset, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
