package scar

import (
	"fmt"
	"strings"
)

// EnforceOptions specifies parameters for pre-edit enforcement.
type EnforceOptions struct {
	Files     []string `json:"files"`
	Symbols   []string `json:"symbols,omitempty"`
	FailOn    string   `json:"fail_on,omitempty"`   // "critical", "high" (default), "medium", "low"
	Threshold int      `json:"threshold,omitempty"` // default: 2
	Format    string   `json:"format,omitempty"`    // "text" or "json"
}

// EnforceResult contains the outcome of the pre-edit check.
type EnforceResult struct {
	Blocked         bool         `json:"blocked"`
	Reason          string       `json:"reason,omitempty"`
	HighestSeverity string       `json:"highest_severity,omitempty"`
	BlockingScars   []ScoredScar `json:"blocking_scars,omitempty"`
}

func severityInt(sev string) int {
	switch strings.ToLower(sev) {
	case SeverityCritical:
		return 4
	case SeverityHigh:
		return 3
	case SeverityMedium:
		return 2
	case SeverityLow:
		return 1
	default:
		return 2 // default medium
	}
}

// Enforce evaluates target files and symbols against active scars.
// Returns Blocked = true if any matching scar has score >= 7 OR
// has severity >= FailOn with score >= 3.
func (s *Store) Enforce(opts EnforceOptions) (*EnforceResult, error) {
	threshold := opts.Threshold
	if threshold <= 0 {
		threshold = 2
	}

	failOn := opts.FailOn
	if failOn == "" {
		failOn = SeverityHigh
	}
	failOnLevel := severityInt(failOn)

	q := QueryOptions{
		Files:        opts.Files,
		Symbols:      opts.Symbols,
		IncludeStale: true,
		Threshold:    threshold,
		TopK:         10,
	}

	candidates, err := s.Recall(q)
	if err != nil {
		return nil, fmt.Errorf("recall failed during enforce check: %w", err)
	}

	res := &EnforceResult{
		Blocked:       false,
		BlockingScars: []ScoredScar{},
	}

	maxLevel := 0
	for _, sc := range candidates {
		sev := sc.Scar.Severity
		if sev == "" {
			sev = SeverityMedium
		}
		level := severityInt(sev)
		if level > maxLevel {
			maxLevel = level
			res.HighestSeverity = sev
		}

		// Block condition:
		// 1) High score (score >= 7) indicates strong correlation with a known failure
		// 2) Severity level >= failOnLevel AND score >= 3 (meets standard abstain threshold)
		if sc.Score >= 7 || (level >= failOnLevel && sc.Score >= 3) {
			res.Blocked = true
			res.BlockingScars = append(res.BlockingScars, sc)
		}
	}

	if res.Blocked {
		res.Reason = fmt.Sprintf("blocked by %d active guardrail(s) at or above %s severity", len(res.BlockingScars), failOn)
	}

	return res, nil
}
