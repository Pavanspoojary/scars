package scar

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LintFinding represents a warning or error discovered by the linter.
type LintFinding struct {
	ScarID   string
	Severity string // "ERROR" or "WARNING"
	Rule     string
	Message  string
}

// LintScar evaluates a single scar against schema, safety, and anchor hygiene rules.
func LintScar(repoRoot string, s *Scar) []LintFinding {
	var findings []LintFinding

	// 1. Invariant check
	if err := s.ValidateInvariants(); err != nil {
		findings = append(findings, LintFinding{
			ScarID:   s.ID,
			Severity: "ERROR",
			Rule:     "invariant_validity",
			Message:  err.Error(),
		})
	}

	// 2. Anchor safety rules
	for _, f := range s.Anchors.Files {
		// Rule: No path traversal (..)
		if strings.Contains(f, "..") {
			findings = append(findings, LintFinding{
				ScarID:   s.ID,
				Severity: "ERROR",
				Rule:     "path_traversal",
				Message:  fmt.Sprintf("path %q contains unsafe '..' traversal segment", f),
			})
		}

		// Rule: No absolute paths
		if filepath.IsAbs(f) || strings.HasPrefix(f, "/") || strings.HasPrefix(f, "\\") {
			findings = append(findings, LintFinding{
				ScarID:   s.ID,
				Severity: "ERROR",
				Rule:     "absolute_path",
				Message:  fmt.Sprintf("path %q must be repository-relative, not absolute", f),
			})
		}

		// Rule: No overbroad wildcards
		if f == "**" || f == "*" || f == ".*" || strings.Contains(f, "/**") {
			findings = append(findings, LintFinding{
				ScarID:   s.ID,
				Severity: "ERROR",
				Rule:     "overbroad_anchor",
				Message:  fmt.Sprintf("overbroad wildcard anchor pattern %q is prohibited", f),
			})
		}

		// Check symlink escapes
		cleanPath := filepath.Join(repoRoot, f)
		fi, err := os.Lstat(cleanPath)
		if err == nil && (fi.Mode()&os.ModeSymlink != 0) {
			target, err := filepath.EvalSymlinks(cleanPath)
			if err == nil {
				rel, err := filepath.Rel(repoRoot, target)
				if err != nil || strings.HasPrefix(rel, "..") {
					findings = append(findings, LintFinding{
						ScarID:   s.ID,
						Severity: "ERROR",
						Rule:     "symlink_escape",
						Message:  fmt.Sprintf("symlink %q resolves outside the repository: %s", f, target),
					})
				}
			}
		}
	}

	// 3. Security: Prompt injection patterns in untrusted scar content
	untrustedText := strings.ToLower(s.Title + " " + s.Lesson + " " + s.Body)
	suspiciousPatterns := []string{
		"ignore previous instructions",
		"ignore all previous commands",
		"system prompt",
		"you are now in developer mode",
		"sudo rm -rf",
		"curl http://",
		"curl https://",
	}

	for _, pat := range suspiciousPatterns {
		if strings.Contains(untrustedText, pat) {
			findings = append(findings, LintFinding{
				ScarID:   s.ID,
				Severity: "WARNING",
				Rule:     "suspicious_instruction_pattern",
				Message:  fmt.Sprintf("lesson contains instruction-like or injection-style pattern: %q", pat),
			})
		}
	}

	// 4. Excessive lesson length check
	if len(s.Lesson) > 4000 {
		findings = append(findings, LintFinding{
			ScarID:   s.ID,
			Severity: "WARNING",
			Rule:     "oversized_lesson",
			Message:  fmt.Sprintf("lesson is %d characters; recommend concise, actionable guidance (<4000 chars)", len(s.Lesson)),
		})
	}

	return findings
}

// LintCollection audits cross-scar properties like duplicates and contradictions.
func LintCollection(scars []*Scar) []LintFinding {
	var findings []LintFinding

	// Check duplicates and contradictions
	for i := 0; i < len(scars); i++ {
		for j := i + 1; j < len(scars); j++ {
			s1, s2 := scars[i], scars[j]

			// Explicit conflict references
			for _, c := range s1.Lifecycle.ConflictsWith {
				if c == s2.ID {
					findings = append(findings, LintFinding{
						ScarID:   s1.ID,
						Severity: "WARNING",
						Rule:     "declared_conflict",
						Message:  fmt.Sprintf("declares explicit conflict with scar %s", s2.ID),
					})
				}
			}

			// Near duplicate detection using word Jaccard similarity
			sim := jaccardSimilarity(s1.Title+" "+s1.Lesson, s2.Title+" "+s2.Lesson)
			if sim >= 0.80 {
				findings = append(findings, LintFinding{
					ScarID:   s1.ID,
					Severity: "WARNING",
					Rule:     "near_duplicate",
					Message:  fmt.Sprintf("near-duplicate of scar %s (lexical similarity %.2f)", s2.ID, sim),
				})
			}
		}
	}

	return findings
}

func jaccardSimilarity(textA, textB string) float64 {
	wordsA := tokenizeWords(textA)
	wordsB := tokenizeWords(textB)
	if len(wordsA) == 0 || len(wordsB) == 0 {
		return 0
	}

	intersection := 0
	for w := range wordsA {
		if wordsB[w] {
			intersection++
		}
	}

	union := len(wordsA)
	for w := range wordsB {
		if !wordsA[w] {
			union++
		}
	}

	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}
