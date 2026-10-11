package scar

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DoctorCheck represents the outcome of an individual diagnostic check.
type DoctorCheck struct {
	Name    string   `json:"name"`
	Status  string   `json:"status"` // "OK", "WARN", "FAIL"
	Message string   `json:"message"`
	Details []string `json:"details,omitempty"`
}

// DoctorReport contains the aggregated health analysis for the repository.
type DoctorReport struct {
	RepoRoot  string        `json:"repo_root"`
	StorePath string        `json:"store_path"`
	Healthy   bool          `json:"healthy"`
	Checks    []DoctorCheck `json:"checks"`
	Errors    int           `json:"errors"`
	Warnings  int           `json:"warnings"`
}

// RunDoctor performs comprehensive repository diagnostics across Git, store, schemas,
// invariants, anchor freshness, pre-commit hooks, trust lists, and index caches.
func (s *Store) RunDoctor() *DoctorReport {
	report := &DoctorReport{
		RepoRoot:  s.RepoRoot,
		StorePath: filepath.Join(s.RepoRoot, DefaultStoreDir),
		Healthy:   true,
		Checks:    make([]DoctorCheck, 0),
	}

	addCheck := func(name, status, message string, details ...string) {
		if status == "FAIL" {
			report.Errors++
			report.Healthy = false
		} else if status == "WARN" {
			report.Warnings++
		}
		report.Checks = append(report.Checks, DoctorCheck{
			Name:    name,
			Status:  status,
			Message: message,
			Details: details,
		})
	}

	// 1. Git Repository & Binary Check
	gitDir := filepath.Join(s.RepoRoot, ".git")
	if info, err := os.Stat(gitDir); err != nil || !info.IsDir() {
		addCheck("Git Repository", "FAIL", "Repository root lacks a valid .git directory")
	} else {
		gitPath, err := exec.LookPath("git")
		if err != nil {
			addCheck("Git Binary", "WARN", "git executable not found in system PATH")
		} else {
			out, _ := exec.Command("git", "version").Output()
			verStr := strings.TrimSpace(string(out))
			if verStr == "" {
				verStr = gitPath
			}
			addCheck("Git Repository", "OK", fmt.Sprintf("Valid git worktree (%s)", verStr))
		}
	}

	// 2. Store Structure Check
	storeDir := filepath.Join(s.RepoRoot, DefaultStoreDir)
	scarsDir := filepath.Join(storeDir, "scars")
	localDir := filepath.Join(storeDir, ".local")

	if info, err := os.Stat(storeDir); err != nil || !info.IsDir() {
		addCheck("Store Metadata", "FAIL", fmt.Sprintf("Directory %s missing. Run 'git-scars init'", DefaultStoreDir))
	} else {
		var missingParts []string
		if _, err := os.Stat(scarsDir); os.IsNotExist(err) {
			missingParts = append(missingParts, "scars/")
		}
		if _, err := os.Stat(localDir); os.IsNotExist(err) {
			missingParts = append(missingParts, ".local/")
		}
		if len(missingParts) > 0 {
			addCheck("Store Structure", "WARN", fmt.Sprintf("Incomplete store layout: missing %s", strings.Join(missingParts, ", ")))
		} else {
			addCheck("Store Structure", "OK", fmt.Sprintf("%s metadata layout intact", DefaultStoreDir))
		}
	}

	// 3. Scars, Lint & Invariant Checks
	summaries, err := s.ListScars()
	if err != nil {
		addCheck("Scar Catalog", "FAIL", fmt.Sprintf("Failed reading scars: %v", err))
	} else {
		var lintErrors []string
		var invariantErrors []string
		var staleScars []string
		activeCount := 0

		for _, sum := range summaries {
			sc := sum.Scar
			if sc.Lifecycle.State == LifecycleActive {
				activeCount++
			}

			// Invariants
			if err := sc.ValidateInvariants(); err != nil {
				invariantErrors = append(invariantErrors, fmt.Sprintf("%s: %v", sc.ID, err))
			}

			// Linting
			findings := LintScar(s.RepoRoot, sc)
			for _, f := range findings {
				if f.Severity == "ERROR" {
					lintErrors = append(lintErrors, fmt.Sprintf("%s [%s]: %s", sc.ID, f.Rule, f.Message))
				}
			}

			// Freshness check on active scars
			if sc.Lifecycle.State == LifecycleActive && sum.Freshness == FreshnessChanged {
				staleScars = append(staleScars, sc.ID)
			}
		}

		// Schema / Lint evaluation
		if len(lintErrors) > 0 {
			addCheck("Schema & Lint", "FAIL", fmt.Sprintf("%d lint error(s) discovered", len(lintErrors)), lintErrors...)
		} else {
			addCheck("Schema & Lint", "OK", fmt.Sprintf("%d scar(s) validated against schema", len(summaries)))
		}

		// Invariants evaluation
		if len(invariantErrors) > 0 {
			addCheck("Lifecycle Invariants", "FAIL", fmt.Sprintf("%d invariant violation(s) found", len(invariantErrors)), invariantErrors...)
		} else {
			addCheck("Lifecycle Invariants", "OK", "All lifecycle and review invariants verified")
		}

		// Anchor freshness evaluation
		if len(staleScars) > 0 {
			addCheck("Anchor Freshness", "WARN", fmt.Sprintf("%d active scar(s) drifted from baseline", len(staleScars)), staleScars...)
		} else {
			addCheck("Anchor Freshness", "OK", fmt.Sprintf("%d active scar(s) synchronized with anchor baselines", activeCount))
		}
	}

	// 4. Git Pre-Commit Hook Check
	if !IsHookInstalled(s.RepoRoot) {
		addCheck("Pre-Commit Hook", "WARN", "Git pre-commit hook is not installed. Run 'git-scars hook install'")
	} else {
		addCheck("Pre-Commit Hook", "OK", "Active (.git/hooks/pre-commit)")
	}

	// 5. Trusted Verifiers Check
	trustList, err := s.LoadTrustList()
	if err != nil {
		addCheck("Trust List", "WARN", fmt.Sprintf("Failed reading trusted verifiers: %v", err))
	} else {
		addCheck("Trust List", "OK", fmt.Sprintf("%d verifier hash(es) registered in trust store", len(trustList.TrustedCommands)))
	}

	// 6. Fast Index Cache Check
	indexPath := filepath.Join(s.LocalDir, "index.json")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		addCheck("Index Cache", "WARN", "Local cache .local/index.json missing. Run 'git-scars index'")
	} else {
		idx, err := s.LoadIndex()
		if err != nil {
			addCheck("Index Cache", "WARN", fmt.Sprintf("Local cache unreadable: %v", err))
		} else {
			addCheck("Index Cache", "OK", fmt.Sprintf("Cached %d entries", len(idx.Scars)))
		}
	}

	return report
}
