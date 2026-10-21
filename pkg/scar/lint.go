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

	// 5. Anchor semantics: the guardrail must still point at code that exists
	// and that no longer contains the footgun it describes.
	findings = append(findings, lintAnchorSemantics(repoRoot, s)...)

	return findings
}

// lintAnchorSemantics verifies that the anchors of a scar still describe the
// code they claim to describe. A guardrail whose anchors have drifted no longer
// matches on retrieval signals and silently stops protecting anything.
//
// Patterns are expected to be literal snippets extracted from the anchored
// files so that presence and absence can be checked deterministically.
func lintAnchorSemantics(repoRoot string, s *Scar) []LintFinding {
	var findings []LintFinding

	// Anchored symbols must resolve in at least one anchored file. Files whose
	// language is not supported for symbol extraction cannot be verified.
	if symbolVerifiable(s.Anchors.Files) {
		for _, sym := range s.Anchors.Symbols {
			if strings.TrimSpace(sym) == "" {
				continue
			}
			if !symbolResolves(repoRoot, s.Anchors.Files, sym) {
				findings = append(findings, LintFinding{
					ScarID:   s.ID,
					Severity: "ERROR",
					Rule:     "anchor_symbol_missing",
					Message:  fmt.Sprintf("anchored symbol %q was not found in any anchored file", sym),
				})
			}
		}
	}

	// A bad pattern that is still present means the known footgun was never
	// removed, or was reintroduced after the guardrail was written.
	if s.BadPattern != "" {
		if file, found := findPatternInAnchors(repoRoot, s.Anchors.Files, s.BadPattern); found {
			findings = append(findings, LintFinding{
				ScarID:   s.ID,
				Severity: "ERROR",
				Rule:     "bad_pattern_present",
				Message:  fmt.Sprintf("the recorded bad pattern is still present in anchored file %s", file),
			})
		}
	}

	// A good pattern that cannot be found suggests the anchor no longer covers
	// the code that carries the fix.
	if s.GoodPattern != "" {
		if _, found := findPatternInAnchors(repoRoot, s.Anchors.Files, s.GoodPattern); !found {
			findings = append(findings, LintFinding{
				ScarID:   s.ID,
				Severity: "WARNING",
				Rule:     "good_pattern_absent",
				Message:  "the recorded good pattern was not found in any anchored file; the anchor may no longer cover the fixed code",
			})
		}
	}

	// An active guardrail must declare how serious it is, otherwise it cannot
	// be compared against a blocking threshold.
	if s.Lifecycle.State == LifecycleActive && strings.TrimSpace(s.Severity) == "" {
		findings = append(findings, LintFinding{
			ScarID:   s.ID,
			Severity: "WARNING",
			Rule:     "missing_severity",
			Message:  "active scar does not declare a severity, so it cannot be compared against a blocking threshold",
		})
	}

	return findings
}

// symbolVerifiable reports whether any anchored file supports symbol extraction.
func symbolVerifiable(anchorFiles []string) bool {
	for _, f := range anchorFiles {
		if IsSupportedSymbolFile(f) {
			return true
		}
	}
	return false
}

// symbolResolves reports whether a symbol is declared in any of the anchored files.
// A name that appears in the file text but is not a declaration is accepted, so
// that anchors to variables and fields are not reported as missing.
func symbolResolves(repoRoot string, anchorFiles []string, symbol string) bool {
	for _, rel := range anchorFiles {
		if !IsSupportedSymbolFile(rel) {
			continue
		}
		full := filepath.Join(repoRoot, rel)
		if _, err := ExtractSymbolHash(full, symbol); err == nil {
			return true
		}
		data, err := os.ReadFile(full)
		if err != nil {
			continue
		}
		if strings.Contains(string(data), symbol) {
			return true
		}
	}
	return false
}

// findPatternInAnchors returns the first anchored file that contains the pattern.
func findPatternInAnchors(repoRoot string, anchorFiles []string, pattern string) (string, bool) {
	for _, rel := range anchorFiles {
		data, err := os.ReadFile(filepath.Join(repoRoot, rel))
		if err != nil {
			continue
		}
		if strings.Contains(string(data), pattern) {
			return rel, true
		}
	}
	return "", false
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
