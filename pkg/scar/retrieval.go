package scar

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// QueryOptions represents all inputs for a recall request.
type QueryOptions struct {
	Task           string   `json:"task,omitempty"`
	Files          []string `json:"files,omitempty"`
	Symbols        []string `json:"symbols,omitempty"`
	Commands       []string `json:"commands,omitempty"`
	ErrorSignature string   `json:"error_signature,omitempty"`
	DiffRef        string   `json:"diff_ref,omitempty"`
	IncludeStale   bool     `json:"include_stale,omitempty"`
	Threshold      int      `json:"threshold,omitempty"`
	TopK           int      `json:"top_k,omitempty"`
}

// ScoredScar represents a candidate scar with match scoring details.
type ScoredScar struct {
	Scar        *Scar    `json:"scar"`
	Score       int      `json:"score"`
	Freshness   string   `json:"freshness"`
	Matches     []string `json:"matches"`
	Explanation string   `json:"explanation"`
}

// RecallResultV1 represents versioned JSON output for reposcars.recall.v1.
type RecallResultV1 struct {
	SchemaVersion string          `json:"schema_version"` // "reposcars.recall.v1"
	Timestamp     string          `json:"timestamp"`
	Query         QueryOptions    `json:"query"`
	Count         int             `json:"count"`
	Results       []ScoredEntryV1 `json:"results"`
}

// ScoredEntryV1 represents an item in versioned recall output.
type ScoredEntryV1 struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Lesson      string   `json:"lesson"` // JSON string field prevents delimiter impersonation
	Score       int      `json:"score"`
	Freshness   string   `json:"freshness"`
	Matches     []string `json:"matches"`
	Explanation string   `json:"explanation"`
	Anchors     Anchors  `json:"anchors"`
}

// ScoreScarItem computes the deterministic score for a single scar against query inputs.
func ScoreScarItem(s *Scar, q QueryOptions) ScoredScar {
	res := ScoredScar{Scar: s, Matches: []string{}}
	score := 0

	// 1. File anchor match (+5)
	fileMatched := false
	for _, qf := range q.Files {
		cleanQf := filepath.Clean(qf)
		for _, sf := range s.Anchors.Files {
			if strings.EqualFold(cleanQf, filepath.Clean(sf)) {
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
		res.Matches = append(res.Matches, "file(+5)")
	}

	// 2. Symbol anchor match (+4)
	symbolMatched := false
	for _, qs := range q.Symbols {
		if qs == "" {
			continue
		}
		for _, ss := range s.Anchors.Symbols {
			if qs == ss {
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
		res.Matches = append(res.Matches, "symbol(+4)")
	}

	// 3. Error signature match (+3)
	if q.ErrorSignature != "" {
		for _, es := range s.Anchors.ErrorSignatures {
			if es != "" && (strings.Contains(strings.ToLower(q.ErrorSignature), strings.ToLower(es)) ||
				strings.Contains(strings.ToLower(es), strings.ToLower(q.ErrorSignature))) {
				score += 3
				res.Matches = append(res.Matches, "error_sig(+3)")
				break
			}
		}
	}

	// 4. Command match (+2)
	cmdMatched := false
	for _, qc := range q.Commands {
		if qc == "" {
			continue
		}
		for _, sc := range s.Anchors.Commands {
			if strings.EqualFold(strings.TrimSpace(qc), strings.TrimSpace(sc)) {
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
		res.Matches = append(res.Matches, "command(+2)")
	}

	// 5. Task-term overlap (0 to 2)
	termScore := computeOverlap(s.Title+" "+s.Lesson, q.Task)
	if termScore > 0 {
		score += termScore
		res.Matches = append(res.Matches, fmt.Sprintf("task_terms(+%d)", termScore))
	}

	res.Score = score
	res.Explanation = strings.Join(res.Matches, ", ")
	return res
}

func computeOverlap(docText, queryText string) int {
	docWords := tokenizeWords(docText)
	queryWords := tokenizeWords(queryText)
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

func tokenizeWords(s string) map[string]bool {
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

// Recall retrieves scars matching query options.
func (s *Store) Recall(q QueryOptions) ([]ScoredScar, error) {
	threshold := q.Threshold
	if threshold <= 0 {
		threshold = 3
	}
	topK := q.TopK
	if topK <= 0 {
		topK = 3
	}

	// Expand files from diff if diff_ref provided
	if q.DiffRef != "" && GitAvailable(s.RepoRoot) {
		diffFiles, err := getDiffFiles(s.RepoRoot, q.DiffRef)
		if err == nil {
			q.Files = append(q.Files, diffFiles...)
		}
	}

	scars, err := s.ListScars()
	if err != nil {
		return nil, err
	}

	var candidates []ScoredScar
	for _, summary := range scars {
		scar := summary.Scar

		// Only active scars are retrieved in normal recall
		if scar.Lifecycle.State != LifecycleActive {
			continue
		}

		// Freshness rule: exclude changed or unavailable scars unless --include-stale
		if summary.Freshness != FreshnessCurrent && !q.IncludeStale {
			continue
		}

		scored := ScoreScarItem(scar, q)
		scored.Freshness = summary.Freshness

		if scored.Score >= threshold {
			candidates = append(candidates, scored)
		}
	}

	// Sort descending by score, then ascending by stable ID
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Score != candidates[j].Score {
			return candidates[i].Score > candidates[j].Score
		}
		return candidates[i].Scar.ID < candidates[j].Scar.ID
	})

	if len(candidates) > topK {
		candidates = candidates[:topK]
	}

	return candidates, nil
}

func getDiffFiles(repoRoot, ref string) ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only", ref)
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// FormatRecallJSON formats recall results in reposcars.recall.v1 schema.
func FormatRecallJSON(query QueryOptions, candidates []ScoredScar) ([]byte, error) {
	result := RecallResultV1{
		SchemaVersion: "reposcars.recall.v1",
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		Query:         query,
		Count:         len(candidates),
		Results:       make([]ScoredEntryV1, 0, len(candidates)),
	}

	for _, c := range candidates {
		result.Results = append(result.Results, ScoredEntryV1{
			ID:          c.Scar.ID,
			Title:       c.Scar.Title,
			Lesson:      c.Scar.Lesson,
			Score:       c.Score,
			Freshness:   c.Freshness,
			Matches:     c.Matches,
			Explanation: c.Explanation,
			Anchors:     c.Scar.Anchors,
		})
	}

	return json.MarshalIndent(result, "", "  ")
}
