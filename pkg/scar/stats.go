package scar

// RepoStats summarizes health and metrics of all scars in the repository.
type RepoStats struct {
	TotalScars      int            `json:"total_scars"`
	LifecycleCounts map[string]int `json:"lifecycle_counts"`
	FreshnessCounts map[string]int `json:"freshness_counts"`
	ReviewCounts    map[string]int `json:"review_counts"`
	TotalFiles      int            `json:"total_files_anchored"`
	TotalSymbols    int            `json:"total_symbols_anchored"`
	TotalErrors     int            `json:"total_error_signatures"`
	FeedbackTotal   int            `json:"feedback_total"`
	FeedbackHelpful int            `json:"feedback_helpful"`
	HelpfulRatio    float64        `json:"helpful_ratio"`
}

// GetStats compiles health metrics across all scars and recorded feedback.
func (s *Store) GetStats() (*RepoStats, error) {
	summaries, err := s.ListScars()
	if err != nil {
		return nil, err
	}

	stats := &RepoStats{
		TotalScars:      len(summaries),
		LifecycleCounts: make(map[string]int),
		FreshnessCounts: make(map[string]int),
		ReviewCounts:    make(map[string]int),
	}

	uniqueFiles := make(map[string]bool)
	uniqueSymbols := make(map[string]bool)
	uniqueErrors := make(map[string]bool)

	for _, sum := range summaries {
		sc := sum.Scar
		stats.LifecycleCounts[sc.Lifecycle.State]++
		stats.FreshnessCounts[sum.Freshness]++
		stats.ReviewCounts[sc.Review.Status]++

		for _, f := range sc.Anchors.Files {
			uniqueFiles[f] = true
		}
		for _, sym := range sc.Anchors.Symbols {
			uniqueSymbols[sym] = true
		}
		for _, errSig := range sc.Anchors.ErrorSignatures {
			uniqueErrors[errSig] = true
		}
	}

	stats.TotalFiles = len(uniqueFiles)
	stats.TotalSymbols = len(uniqueSymbols)
	stats.TotalErrors = len(uniqueErrors)

	feedbackEntries, _ := s.LoadFeedback()
	stats.FeedbackTotal = len(feedbackEntries)
	for _, fb := range feedbackEntries {
		if fb.Rating == "helpful" {
			stats.FeedbackHelpful++
		}
	}
	if stats.FeedbackTotal > 0 {
		stats.HelpfulRatio = float64(stats.FeedbackHelpful) / float64(stats.FeedbackTotal) * 100.0
	}

	return stats, nil
}
