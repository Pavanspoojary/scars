package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Scar represents a single failure record in the spike.
type Scar struct {
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	Files          []string `json:"files"`
	Symbols        []string `json:"symbols"`
	Dependencies   []string `json:"dependencies"`
	Commands       []string `json:"commands"`
	ErrorSignature string   `json:"error_signature"`
	Lesson         string   `json:"lesson"`
	Status         string   `json:"status"` // active, draft, etc.
}

// QueryCase represents a labeled evaluation query.
type QueryCase struct {
	ID              string   `json:"id"`
	Description     string   `json:"description"`
	Split           string   `json:"split"` // "dev" or "test"
	Task            string   `json:"task"`
	Files           []string `json:"files"`
	Symbols         []string `json:"symbols"`
	Commands        []string `json:"commands"`
	ErrorSignature  string   `json:"error_signature"`
	ExpectedScarIDs []string `json:"expected_scar_ids"`
}

// ScoredMatch represents a retrieval candidate.
type ScoredMatch struct {
	ScarID      string   `json:"scar_id"`
	Score       int      `json:"score"`
	Matches     []string `json:"matches"`
	Explanation string   `json:"explanation"`
}

// ScoreScar calculates score based on the deterministic weighting rules:
// Exact file anchor: +5
// Exact symbol anchor: +4
// Exact dependency: +3
// Error signature match: +3
// Command match: +2
// Task term overlap: 0-2
func ScoreScar(scar Scar, q QueryCase) ScoredMatch {
	match := ScoredMatch{ScarID: scar.ID, Matches: []string{}}
	score := 0

	// 1. Exact file anchor (+5) - at most one contribution per category
	fileMatched := false
	for _, qf := range q.Files {
		for _, sf := range scar.Files {
			if strings.EqualFold(filepath.Clean(qf), filepath.Clean(sf)) {
				fileMatched = true
				break
			}
		}
		if fileMatched {
			break
		}
	}
	if fileMatched {
		score += 5
		match.Matches = append(match.Matches, "file(+5)")
	}

	// 2. Exact symbol anchor (+4)
	symbolMatched := false
	for _, qs := range q.Symbols {
		for _, ss := range scar.Symbols {
			if qs == ss && qs != "" {
				symbolMatched = true
				break
			}
		}
		if symbolMatched {
			break
		}
	}
	if symbolMatched {
		score += 4
		match.Matches = append(match.Matches, "symbol(+4)")
	}

	// 3. Error signature match (+3)
	if scar.ErrorSignature != "" && q.ErrorSignature != "" {
		if strings.Contains(strings.ToLower(q.ErrorSignature), strings.ToLower(scar.ErrorSignature)) ||
			strings.Contains(strings.ToLower(scar.ErrorSignature), strings.ToLower(q.ErrorSignature)) {
			score += 3
			match.Matches = append(match.Matches, "error_sig(+3)")
		}
	}

	// 4. Command match (+2)
	cmdMatched := false
	for _, qc := range q.Commands {
		for _, sc := range scar.Commands {
			if strings.EqualFold(strings.TrimSpace(qc), strings.TrimSpace(sc)) && qc != "" {
				cmdMatched = true
				break
			}
		}
		if cmdMatched {
			break
		}
	}
	if cmdMatched {
		score += 2
		match.Matches = append(match.Matches, "command(+2)")
	}

	// 5. Task-term overlap (0 to 2)
	termScore := computeTermOverlap(scar.Title+" "+scar.Lesson, q.Task)
	if termScore > 0 {
		score += termScore
		match.Matches = append(match.Matches, fmt.Sprintf("task_terms(+%d)", termScore))
	}

	match.Score = score
	match.Explanation = strings.Join(match.Matches, ", ")
	return match
}

func computeTermOverlap(docText, queryText string) int {
	docWords := tokenize(docText)
	queryWords := tokenize(queryText)
	if len(queryWords) == 0 || len(docWords) == 0 {
		return 0
	}

	overlap := 0
	for qw := range queryWords {
		if docWords[qw] {
			overlap++
		}
	}

	if overlap >= 3 {
		return 2
	} else if overlap >= 1 {
		return 1
	}
	return 0
}

func tokenize(s string) map[string]bool {
	stopwords := map[string]bool{
		"a": true, "an": true, "and": true, "are": true, "as": true, "at": true, "be": true,
		"by": true, "for": true, "from": true, "has": true, "he": true, "in": true, "is": true,
		"it": true, "its": true, "of": true, "on": true, "that": true, "the": true, "to": true,
		"was": true, "were": true, "will": true, "with": true,
	}
	words := strings.Fields(strings.ToLower(s))
	m := make(map[string]bool)
	for _, w := range words {
		w = strings.Trim(w, `.,!?:;'"()[]{}<>/-_`)
		if len(w) > 2 && !stopwords[w] {
			m[w] = true
		}
	}
	return m
}

// Retrieve ranks scars for a given query, enforcing threshold and top-k limits.
func Retrieve(scars []Scar, q QueryCase, threshold int, topK int) []ScoredMatch {
	var candidates []ScoredMatch
	for _, s := range scars {
		if s.Status != "active" {
			continue // only active scars are retrieved in normal recall
		}
		sm := ScoreScar(s, q)
		if sm.Score >= threshold {
			candidates = append(candidates, sm)
		}
	}

	// Sort descending by score, then ascending by stable ID
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Score != candidates[j].Score {
			return candidates[i].Score > candidates[j].Score
		}
		return candidates[i].ScarID < candidates[j].ScarID
	})

	if len(candidates) > topK {
		candidates = candidates[:topK]
	}
	return candidates
}

type EvalMetrics struct {
	TotalQueries       int     `json:"total_queries"`
	NoMatchQueries     int     `json:"no_match_queries"`
	AbstainCount       int     `json:"abstain_count"`
	AbstainRate        float64 `json:"abstain_rate"`
	FalsePositives     int     `json:"false_positives"` // retrieved anything for no-match query
	NoMatchFPRate      float64 `json:"no_match_fp_rate"`
	PrecisionAt3       float64 `json:"precision_at_3"`
	RecallAt3          float64 `json:"recall_at_3"`
	MeanReciprocalRank float64 `json:"mrr"`
}

func Evaluate(scars []Scar, queries []QueryCase, threshold int, topK int) EvalMetrics {
	var m EvalMetrics
	m.TotalQueries = len(queries)

	var totalPrecision float64
	var totalRecall float64
	matchQueriesCount := 0
	var sumRR float64

	for _, q := range queries {
		results := Retrieve(scars, q, threshold, topK)
		isNoMatch := len(q.ExpectedScarIDs) == 0

		if len(results) == 0 {
			m.AbstainCount++
		}

		if isNoMatch {
			m.NoMatchQueries++
			if len(results) > 0 {
				m.FalsePositives++
			}
		} else {
			matchQueriesCount++
			expectedMap := make(map[string]bool)
			for _, id := range q.ExpectedScarIDs {
				expectedMap[id] = true
			}

			hits := 0
			firstRank := 0
			for idx, r := range results {
				if expectedMap[r.ScarID] {
					hits++
					if firstRank == 0 {
						firstRank = idx + 1
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
	}

	if m.TotalQueries > 0 {
		m.AbstainRate = float64(m.AbstainCount) / float64(m.TotalQueries)
	}
	if m.NoMatchQueries > 0 {
		m.NoMatchFPRate = float64(m.FalsePositives) / float64(m.NoMatchQueries)
	}
	if matchQueriesCount > 0 {
		m.PrecisionAt3 = totalPrecision / float64(matchQueriesCount)
		m.RecallAt3 = totalRecall / float64(matchQueriesCount)
		m.MeanReciprocalRank = sumRR / float64(matchQueriesCount)
	}

	return m
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run eval.go <path-to-dataset.json>")
		os.Exit(1)
	}

	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed reading dataset: %v\n", err)
		os.Exit(1)
	}

	var ds struct {
		Scars   []Scar      `json:"scars"`
		Queries []QueryCase `json:"queries"`
	}
	if err := json.Unmarshal(data, &ds); err != nil {
		fmt.Fprintf(os.Stderr, "failed unmarshaling dataset: %v\n", err)
		os.Exit(1)
	}

	// Filter dev and test queries
	var devQueries, testQueries []QueryCase
	for _, q := range ds.Queries {
		if q.Split == "dev" {
			devQueries = append(devQueries, q)
		} else if q.Split == "test" {
			testQueries = append(testQueries, q)
		}
	}

	fmt.Printf("=== GitScars Retrieval Spike Evaluation ===\n")
	fmt.Printf("Total Scars in Dataset: %d (Active: %d)\n", len(ds.Scars), countActive(ds.Scars))
	fmt.Printf("Queries: %d dev, %d test (Total: %d)\n\n", len(devQueries), len(testQueries), len(ds.Queries))

	// 1. Dev split evaluation (used for weight and threshold calibration)
	fmt.Println("--- Development Split Evaluation (Threshold >= 3, TopK = 3) ---")
	devMetrics := Evaluate(ds.Scars, devQueries, 3, 3)
	printMetrics(devMetrics)

	// 2. Test split evaluation (frozen weights and threshold)
	fmt.Println("\n--- Frozen Test Split Evaluation (Threshold >= 3, TopK = 3) ---")
	testMetrics := Evaluate(ds.Scars, testQueries, 3, 3)
	printMetrics(testMetrics)

	// 3. Scale test: 10 vs 50 scars on test queries
	fmt.Println("\n--- Scale Comparison on Test Split ---")
	if len(ds.Scars) >= 10 {
		m10 := Evaluate(ds.Scars[:10], testQueries, 3, 3)
		fmt.Printf("N=10 Scars -> Precision@3: %.2f | Recall@3: %.2f | No-Match FP Rate: %.1f%%\n",
			m10.PrecisionAt3, m10.RecallAt3, m10.NoMatchFPRate*100)
	}
	fmt.Printf("N=%d Scars -> Precision@3: %.2f | Recall@3: %.2f | No-Match FP Rate: %.1f%%\n",
		len(ds.Scars), testMetrics.PrecisionAt3, testMetrics.RecallAt3, testMetrics.NoMatchFPRate*100)
}

func countActive(scars []Scar) int {
	c := 0
	for _, s := range scars {
		if s.Status == "active" {
			c++
		}
	}
	return c
}

func printMetrics(m EvalMetrics) {
	fmt.Printf("  Total Queries:         %d\n", m.TotalQueries)
	fmt.Printf("  Precision@3:           %.3f (Target: >= 0.700)\n", m.PrecisionAt3)
	fmt.Printf("  Recall@3:              %.3f\n", m.RecallAt3)
	fmt.Printf("  Mean Reciprocal Rank:  %.3f\n", m.MeanReciprocalRank)
	fmt.Printf("  No-Match Queries:      %d\n", m.NoMatchQueries)
	fmt.Printf("  No-Match FP Rate:      %.1f%% (Target: <= 10.0%%)\n", m.NoMatchFPRate*100)
	fmt.Printf("  Abstain Rate:          %.1f%%\n", m.AbstainRate*100)
}
