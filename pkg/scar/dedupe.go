package scar

import (
	"fmt"
	"sort"
)

// DedupeCandidate represents a pair of active scars with high lexical similarity.
type DedupeCandidate struct {
	ScarA      *Scar   `json:"scar_a"`
	ScarB      *Scar   `json:"scar_b"`
	Similarity float64 `json:"similarity"`
	Suggestion string  `json:"suggestion"`
}

// FindDuplicates scans active scars and finds pairwise near-duplicates based on Jaccard similarity.
func (s *Store) FindDuplicates(minSimilarity float64) ([]DedupeCandidate, error) {
	if minSimilarity <= 0 {
		minSimilarity = 0.85
	}

	summaries, err := s.ListScars()
	if err != nil {
		return nil, err
	}

	var active []*Scar
	for _, sum := range summaries {
		if sum.Scar.Lifecycle.State == LifecycleActive {
			active = append(active, sum.Scar)
		}
	}

	var candidates []DedupeCandidate
	for i := 0; i < len(active); i++ {
		for j := i + 1; j < len(active); j++ {
			s1, s2 := active[i], active[j]
			sim := jaccardSimilarity(s1.Title+" "+s1.Lesson, s2.Title+" "+s2.Lesson)
			if sim >= minSimilarity {
				candidates = append(candidates, DedupeCandidate{
					ScarA:      s1,
					ScarB:      s2,
					Similarity: sim,
					Suggestion: fmt.Sprintf("Consider running 'git-scars supersede %s --by %s'", s1.ID, s2.ID),
				})
			}
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Similarity != candidates[j].Similarity {
			return candidates[i].Similarity > candidates[j].Similarity
		}
		return candidates[i].ScarA.ID < candidates[j].ScarA.ID
	})

	return candidates, nil
}
