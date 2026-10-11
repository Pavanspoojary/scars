package scar

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// QueryOptions represents all inputs for a recall request.
type QueryOptions struct {
	Task                  string            `json:"task,omitempty"`
	Files                 []string          `json:"files,omitempty"`
	Symbols               []string          `json:"symbols,omitempty"`
	Commands              []string          `json:"commands,omitempty"`
	ErrorSignature        string            `json:"error_signature,omitempty"`
	DiffRef               string            `json:"diff_ref,omitempty"`
	IncludeStale          bool              `json:"include_stale,omitempty"`
	Threshold             int               `json:"threshold,omitempty"`
	TopK                  int               `json:"top_k,omitempty"`
	InstalledDependencies map[string]string `json:"installed_dependencies,omitempty"`
}

// ScoredScar represents a candidate scar with match scoring details.
type ScoredScar struct {
	Scar        *Scar    `json:"scar"`
	Score       int      `json:"score"`
	Confidence  float64  `json:"confidence,omitempty"`
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
	Confidence  float64  `json:"confidence,omitempty"`
	Freshness   string   `json:"freshness"`
	Matches     []string `json:"matches"`
	Explanation string   `json:"explanation"`
	Anchors     Anchors  `json:"anchors"`
}

// MaxPossibleScore is the sum of max per-category contributions.
// File(5) + Symbol(4) + Dependency(3) + Error(3) + Command(2) + TaskTerms(2) = 19.
const MaxPossibleScore = 19

// ConfidenceForScore converts a raw score into a 0..1 confidence value.
func ConfidenceForScore(score int) float64 {
	if score <= 0 {
		return 0
	}
	c := float64(score) / float64(MaxPossibleScore)
	if c > 1 {
		return 1
	}
	return c
}

// ScoreScarItem computes the deterministic score for a single scar against query inputs.
// Retrieval v2: exact file(+5) else basename(+2) else same-dir(+1); exact symbol(+4)
// else normalized suffix(+2); normalized error substring(+3). Partial file signals
// remain below the abstain threshold alone to preserve retrieval precision.
// At most one contribution per category to preserve EVAL determinism.
func ScoreScarItem(s *Scar, q QueryOptions) ScoredScar {
	res := ScoredScar{Scar: s, Matches: []string{}}
	score := 0

	// 1. File anchor match: exact(+5) > basename(+2) > same-dir(+1)
	if fs, label := matchFileAnchor(q.Files, s.Anchors.Files); fs > 0 {
		score += fs
		res.Matches = append(res.Matches, label)
	}

	// 2. Symbol anchor match: exact(+4) else normalized(+2)
	if ss, label := matchSymbolAnchor(q.Symbols, s.Anchors.Symbols); ss > 0 {
		score += ss
		res.Matches = append(res.Matches, label)
	}

	// 3. Exact dependency and compatible version match (+3)
	depMatched := false
	if len(q.InstalledDependencies) > 0 {
		for _, da := range s.Anchors.Dependencies {
			if MatchDependency(q.InstalledDependencies, da) {
				depMatched = true
				break
			}
		}
	}
	if depMatched {
		score += 3
		res.Matches = append(res.Matches, "dependency(+3)")
	}

	// 4. Error signature match (+3) on normalized text
	if q.ErrorSignature != "" {
		normQ := NormalizeErrorSignature(q.ErrorSignature)
		matched := false
		for _, es := range s.Anchors.ErrorSignatures {
			if es == "" {
				continue
			}
			normE := NormalizeErrorSignature(es)
			if normE != "" && (strings.Contains(normQ, normE) ||
				strings.Contains(normE, normQ) ||
				strings.Contains(strings.ToLower(q.ErrorSignature), strings.ToLower(es)) ||
				strings.Contains(strings.ToLower(es), strings.ToLower(q.ErrorSignature))) {
				matched = true
				break
			}
		}
		if matched {
			score += 3
			res.Matches = append(res.Matches, "error_sig(+3)")
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
	res.Confidence = ConfidenceForScore(score)
	res.Explanation = strings.Join(res.Matches, ", ")
	return res
}

// matchFileAnchor returns the best single file contribution.
// Exact path(+5), same basename(+2, likely move/rename), same directory(+1,
// module proximity). Partial signals stay below the abstain threshold (3)
// unless combined with an additional signal, preserving precision.
func matchFileAnchor(queryFiles, anchorFiles []string) (int, string) {
	best := 0
	label := ""
	for _, qf := range queryFiles {
		if qf == "" {
			continue
		}
		cleanQf := filepath.Clean(qf)
		qBase := strings.ToLower(filepath.Base(cleanQf))
		qDir := strings.ToLower(filepath.Dir(cleanQf))
		for _, sf := range anchorFiles {
			cleanSf := filepath.Clean(sf)
			if strings.EqualFold(cleanQf, cleanSf) {
				return 5, "file(+5)"
			}
			sDir := strings.ToLower(filepath.Dir(cleanSf))
			sBase := strings.ToLower(filepath.Base(cleanSf))
			if qBase != "" && qBase == sBase && best < 2 {
				best = 2
				label = "file_base(+2)"
			} else if qDir != "" && qDir != "." && qDir == sDir && best < 1 {
				best = 1
				label = "file_dir(+1)"
			}
		}
	}
	return best, label
}

// NormalizeSymbol strips receivers and package qualifiers:
// "(*Exporter).RecordMetric" -> "recordmetric", "pkg/auth.Context" -> "context".
func NormalizeSymbol(sym string) string {
	s := strings.TrimSpace(sym)
	// Strip receiver prefix like "(*T)." or "(T)."
	if idx := strings.LastIndex(s, ")."); idx != -1 {
		s = s[idx+2:]
	} else if idx := strings.LastIndex(s, "."); idx != -1 {
		s = s[idx+1:]
	}
	s = strings.Trim(s, "() *")
	return strings.ToLower(s)
}

// matchSymbolAnchor returns exact(+4) else normalized suffix(+2).
func matchSymbolAnchor(querySyms, anchorSyms []string) (int, string) {
	for _, qs := range querySyms {
		if qs == "" {
			continue
		}
		for _, ss := range anchorSyms {
			if qs == ss {
				return 4, "symbol(+4)"
			}
		}
	}
	for _, qs := range querySyms {
		if qs == "" {
			continue
		}
		nq := NormalizeSymbol(qs)
		if nq == "" {
			continue
		}
		for _, ss := range anchorSyms {
			if nq == NormalizeSymbol(ss) {
				return 2, "symbol_norm(+2)"
			}
		}
	}
	return 0, ""
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

var (
	errHexRe  = regexp.MustCompile(`0x[0-9a-fA-F]+`)
	errNumRe  = regexp.MustCompile(`\b\d+(\.\d+)*\b`)
	errPathRe = regexp.MustCompile(`[\w\-./\\]+\.\w+:\d+(:\d+)?`)
	errTimeRe = regexp.MustCompile(`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}(:\d{2})?`)
	errAddrRe = regexp.MustCompile(`\+0x[0-9a-fA-F]+`)
)

// NormalizeErrorSignature strips volatile tokens (hex, line numbers, timestamps)
// and lowercases for stable deterministic matching.
func NormalizeErrorSignature(s string) string {
	n := strings.ToLower(strings.TrimSpace(s))
	n = errAddrRe.ReplaceAllString(n, "")
	n = errHexRe.ReplaceAllString(n, "<hex>")
	n = errPathRe.ReplaceAllString(n, "<loc>")
	n = errTimeRe.ReplaceAllString(n, "<time>")
	// Collapse leftover standalone numbers to avoid line-number brittleness,
	// but keep short words intact.
	n = errNumRe.ReplaceAllString(n, "<n>")
	n = strings.Join(strings.Fields(n), " ")
	return n
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

	if q.InstalledDependencies == nil {
		q.InstalledDependencies = GetRepoDependencies(s.RepoRoot)
	}

	var candidates []ScoredScar

	// Fast-path: score candidates directly from in-memory index cache if fresh
	idx, idxErr := s.LoadIndex()
	if idxErr == nil && s.IsIndexFresh(idx) {
		for _, entry := range idx.Scars {
			if entry.Lifecycle != LifecycleActive {
				continue
			}
			scarStub := &Scar{
				ID:        entry.ID,
				Title:     entry.Title,
				Lesson:    entry.Lesson,
				Lifecycle: Lifecycle{State: entry.Lifecycle},
				Review:    Review{Status: entry.ReviewStatus},
				Anchors: Anchors{
					Files:           entry.Files,
					Symbols:         entry.Symbols,
					Commands:        entry.Commands,
					ErrorSignatures: entry.ErrorSignatures,
					Dependencies:    entry.Dependencies,
				},
			}
			scored := ScoreScarItem(scarStub, q)
			if scored.Score >= threshold {
				fullScar, _, err := s.LoadScar(entry.ID)
				if err != nil {
					continue
				}
				freshness := s.ComputeFreshness(fullScar)
				if freshness != FreshnessCurrent && !q.IncludeStale {
					continue
				}
				scored.Scar = fullScar
				scored.Freshness = freshness
				candidates = append(candidates, scored)
			}
		}
	} else {
		// Fallback path: full disk scan and baseline calculation across all scars
		scars, err := s.ListScars()
		if err != nil {
			return nil, err
		}

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
			Confidence:  c.Confidence,
			Freshness:   c.Freshness,
			Matches:     c.Matches,
			Explanation: c.Explanation,
			Anchors:     c.Scar.Anchors,
		})
	}

	return json.MarshalIndent(result, "", "  ")
}
