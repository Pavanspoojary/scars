package scar

import (
	"bufio"
	"strings"
)

// TriageResult represents a diagnosed failure match.
type TriageResult struct {
	MatchedSignature string       `json:"matched_signature,omitempty"`
	ExtractedError   string       `json:"extracted_error"`
	Matches          []ScoredScar `json:"matches"`
}

// ExtractErrorSignatures scans compiler or test output to isolate error signatures and traces.
func ExtractErrorSignatures(logOutput string) []string {
	var signatures []string
	scanner := bufio.NewScanner(strings.NewReader(logOutput))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "panic:") ||
			strings.HasPrefix(lower, "fatal error:") ||
			strings.Contains(lower, "deadlock") ||
			strings.Contains(lower, "race detected") ||
			strings.HasPrefix(lower, "error:") ||
			strings.Contains(lower, "assertionerror") ||
			strings.Contains(lower, "segmentation fault") ||
			strings.HasPrefix(lower, "--- fail:") {
			signatures = append(signatures, line)
		}
	}

	return signatures
}

// TriageError matches a raw error log or panic trace against recorded repository scars.
func (s *Store) TriageError(logOutput string) (*TriageResult, error) {
	signatures := ExtractErrorSignatures(logOutput)
	extracted := strings.Join(signatures, "\n")
	if extracted == "" {
		extracted = strings.TrimSpace(logOutput)
	}

	q := QueryOptions{
		ErrorSignature: extracted,
		Task:           extracted,
		Threshold:      2,
		TopK:           5,
	}

	// Extract potential file paths from stack traces
	for _, line := range strings.Split(logOutput, "\n") {
		fields := strings.Fields(line)
		for _, f := range fields {
			clean := strings.Trim(f, "():,")
			if strings.HasSuffix(clean, ".go") ||
				strings.HasSuffix(clean, ".js") ||
				strings.HasSuffix(clean, ".ts") ||
				strings.HasSuffix(clean, ".py") ||
				strings.HasSuffix(clean, ".rs") {
				q.Files = append(q.Files, clean)
			}
		}
	}

	matches, err := s.Recall(q)
	if err != nil {
		return nil, err
	}

	primarySig := ""
	if len(signatures) > 0 {
		primarySig = signatures[0]
	}

	return &TriageResult{
		MatchedSignature: primarySig,
		ExtractedError:   extracted,
		Matches:          matches,
	}, nil
}
