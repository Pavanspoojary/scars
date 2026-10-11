package scar

import (
	"fmt"
	"os"
	"path/filepath"
)

// CheckResult records findings from a read-only audit of scars in the repository.
type CheckResult struct {
	ExitCode      int
	TotalScars    int
	CurrentCount  int
	ChangedCount  int
	StaleScars    []string
	InvalidScars  []string
	LintFindings  []LintFinding
	IsShallowWarn bool
}

// RunReadOnlyCheck performs a strictly read-only validation of scars in the repo.
// Invariant: This function NEVER writes, mutates files, or executes verifiers.
func (s *Store) RunReadOnlyCheck(diffRef string) (*CheckResult, error) {
	summaries, err := s.ListScars()
	if err != nil {
		return &CheckResult{ExitCode: 3}, fmt.Errorf("operational error reading scars: %w", err)
	}

	res := &CheckResult{
		TotalScars: len(summaries),
		ExitCode:   0,
	}

	var scarPtrs []*Scar
	for _, summary := range summaries {
		scar := summary.Scar
		scarPtrs = append(scarPtrs, scar)

		// 1. Invariant and schema validation
		if err := scar.ValidateInvariants(); err != nil {
			res.InvalidScars = append(res.InvalidScars, fmt.Sprintf("%s: %v", scar.ID, err))
			res.ExitCode = 1
		}

		// 2. Lint individual scar
		findings := LintScar(s.RepoRoot, scar)
		for _, f := range findings {
			res.LintFindings = append(res.LintFindings, f)
			if f.Severity == "ERROR" && res.ExitCode < 1 {
				res.ExitCode = 1
			}
		}

		// 3. Dynamic freshness check
		freshness := summary.Freshness
		switch freshness {
		case FreshnessCurrent:
			res.CurrentCount++
		case FreshnessChanged:
			res.ChangedCount++
			if scar.Lifecycle.State == LifecycleActive {
				res.StaleScars = append(res.StaleScars, scar.ID)
				if res.ExitCode < 2 && res.ExitCode == 0 {
					res.ExitCode = 2 // Stale evidence requiring review
				}
			}
		}

		// 4. Missing anchor files
		for _, f := range scar.Anchors.Files {
			fullPath := filepath.Join(s.RepoRoot, f)
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				res.LintFindings = append(res.LintFindings, LintFinding{
					ScarID:   scar.ID,
					Severity: "WARNING",
					Rule:     "missing_anchor_file",
					Message:  fmt.Sprintf("anchored file %s does not exist on disk", f),
				})
			}
		}
	}

	// Cross-scar linting
	crossFindings := LintCollection(scarPtrs)
	res.LintFindings = append(res.LintFindings, crossFindings...)

	// Check shallow clone warning (must not trigger exit code 2 on its own)
	if CheckShallowClone(s.RepoRoot) {
		res.IsShallowWarn = true
		res.LintFindings = append(res.LintFindings, LintFinding{
			ScarID:   "*",
			Severity: "WARNING",
			Rule:     "shallow_clone",
			Message:  "repository is a shallow clone; commit histories and base objects may be unavailable",
		})
	}

	// Diff relevance reporting if diffRef is provided
	if diffRef != "" && GitAvailable(s.RepoRoot) {
		diffFiles, err := getDiffFiles(s.RepoRoot, diffRef)
		if err == nil && len(diffFiles) > 0 {
			diffSet := make(map[string]bool)
			for _, df := range diffFiles {
				diffSet[filepath.Clean(df)] = true
			}
			for _, summary := range summaries {
				for _, af := range summary.Scar.Anchors.Files {
					if diffSet[filepath.Clean(af)] {
						res.LintFindings = append(res.LintFindings, LintFinding{
							ScarID:   summary.Scar.ID,
							Severity: "INFO",
							Rule:     "diff_overlap",
							Message:  fmt.Sprintf("anchor %s modified in diff %s", af, diffRef),
						})
						break
					}
				}
			}
		}
	}

	return res, nil
}
