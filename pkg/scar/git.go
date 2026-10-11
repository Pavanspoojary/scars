package scar

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitCommitInfo contains metadata extracted from a git commit.
type GitCommitInfo struct {
	SHA        string
	ParentSHA  string
	Subject    string
	Body       string
	Files      []string
	TestFiles  []string
	IsShallow  bool
}

// InspectCommit extracts commit metadata without running an LLM.
func InspectCommit(repoRoot, sha string) (*GitCommitInfo, error) {
	// Check if object exists
	cmd := exec.Command("git", "rev-parse", "--verify", sha+"^{commit}")
	cmd.Dir = repoRoot
	fullSHABuf, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("commit %q not found or unavailable (may be shallow clone): %w", sha, err)
	}
	fullSHA := strings.TrimSpace(string(fullSHABuf))

	// Get subject and body
	cmd = exec.Command("git", "show", "-s", "--format=%s%n---BODY---%n%b", fullSHA)
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed reading commit message for %s: %w", fullSHA, err)
	}

	parts := strings.Split(string(out), "---BODY---")
	subject := strings.TrimSpace(parts[0])
	body := ""
	if len(parts) > 1 {
		body = strings.TrimSpace(parts[1])
	}

	// Get parent SHA
	cmd = exec.Command("git", "rev-parse", fullSHA+"^")
	cmd.Dir = repoRoot
	parentBuf, _ := cmd.Output()
	parentSHA := strings.TrimSpace(string(parentBuf))

	// Get changed files
	cmd = exec.Command("git", "diff-tree", "--no-commit-id", "--name-only", "-r", fullSHA)
	cmd.Dir = repoRoot
	diffOut, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed listing changed files for %s: %w", fullSHA, err)
	}

	var regularFiles []string
	var testFiles []string
	for _, f := range strings.Split(string(diffOut), "\n") {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		if isTestFile(f) {
			testFiles = append(testFiles, f)
		} else {
			regularFiles = append(regularFiles, f)
		}
	}

	return &GitCommitInfo{
		SHA:       fullSHA,
		ParentSHA: parentSHA,
		Subject:   subject,
		Body:      body,
		Files:     regularFiles,
		TestFiles: testFiles,
	}, nil
}

func isTestFile(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, "_test.go") ||
		strings.HasSuffix(lower, ".test.js") ||
		strings.HasSuffix(lower, ".test.ts") ||
		strings.HasSuffix(lower, ".spec.js") ||
		strings.HasSuffix(lower, ".spec.ts") ||
		strings.HasPrefix(filepath.Base(lower), "test_") ||
		strings.HasSuffix(lower, "_test.py") ||
		strings.Contains(lower, "/tests/") ||
		strings.Contains(lower, "/test/")
}

// DraftFromCommit builds a draft Scar initialized from a Git commit.
// Invariant: The draft MUST remain in draft state, unreviewed, with verification not_attempted.
func (s *Store) DraftFromCommit(sha string) (*Scar, string, error) {
	info, err := InspectCommit(s.RepoRoot, sha)
	if err != nil {
		return nil, "", err
	}

	title := info.Subject
	if title == "" {
		title = fmt.Sprintf("Fix in %s", sha[:7])
	}

	lesson := info.Body
	if lesson == "" {
		lesson = "Document prevention lesson and root cause here."
	}

	allAnchors := append([]string{}, info.Files...)
	if len(allAnchors) == 0 && len(info.TestFiles) > 0 {
		allAnchors = append(allAnchors, info.TestFiles...)
	}

	draft, path, err := s.CreateDraft(title, lesson, allAnchors)
	if err != nil {
		return nil, "", err
	}

	draft.FixCommit = info.SHA
	draft.BaseCommit = info.ParentSHA

	// Infer verifier command if Go test files found
	if len(info.TestFiles) > 0 {
		for _, tf := range info.TestFiles {
			if strings.HasSuffix(tf, "_test.go") {
				pkgDir := filepath.Dir(tf)
				if !strings.HasPrefix(pkgDir, "./") && pkgDir != "." {
					pkgDir = "./" + pkgDir
				}
				draft.Verifier.Argv = []string{"go", "test", "-race", pkgDir + "/..."}
				break
			}
		}
	}

	// Strictly enforce initial invariant states
	draft.Lifecycle.State = LifecycleDraft
	draft.Review.Status = ReviewUnreviewed
	draft.Verifier.Result = VerificationNotAttempted

	if _, err := s.SaveScar(draft); err != nil {
		return nil, "", err
	}

	return draft, path, nil
}

// CheckShallowClone returns true if the git repository is a shallow clone.
func CheckShallowClone(repoRoot string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-shallow-repository")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
}

// GitAvailable verifies if git executable is in path and repository is a git worktree.
func GitAvailable(repoRoot string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = repoRoot
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	return err == nil && strings.TrimSpace(out.String()) == "true"
}
