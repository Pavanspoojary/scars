package scar

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// HarvestOptions defines parameters for walking git history and detecting regression bug fixes.
type HarvestOptions struct {
	SinceRef      string   `json:"since_ref,omitempty"`
	MaxCommits    int      `json:"max_commits,omitempty"`
	DryRun        bool     `json:"dry_run,omitempty"`
	MatchPatterns []string `json:"match_patterns,omitempty"`
	AllowUntested bool     `json:"allow_untested,omitempty"`
	MaxFiles      int      `json:"max_files,omitempty"`
	MinConfidence int      `json:"min_confidence,omitempty"`
}

// HarvestCandidate represents a discovered bug fix commit suitable for scar drafting.
type HarvestCandidate struct {
	CommitSHA   string   `json:"commit_sha"`
	Subject     string   `json:"subject"`
	Author      string   `json:"author"`
	Files       []string `json:"files"`
	TestFiles   []string `json:"test_files"`
	Verifier    []string `json:"verifier"`
	SuggestedID string   `json:"suggested_id"`
	Title       string   `json:"title"`
	Lesson      string   `json:"lesson"`
	Confidence  int      `json:"confidence"`
	Reason      string   `json:"reason,omitempty"`
	DraftPath   string   `json:"draft_path,omitempty"`
}

// HarvestResult contains the aggregated outcome of the harvest operation.
type HarvestResult struct {
	ScannedCommits int                `json:"scanned_commits"`
	FoundFixes     int                `json:"found_fixes"`
	DraftsCreated  int                `json:"drafts_created"`
	Candidates     []HarvestCandidate `json:"candidates"`
}

var defaultFixRegexes = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^(fix|hotfix|patch|bugfix)(\([^)]+\))?:\s*(.+)`),
	regexp.MustCompile(`(?i)^(resolve|resolves|close|closes)\s+(#\d+|[A-Z]+-\d+):\s*(.+)`),
	regexp.MustCompile(`(?i)^revert[:\s]+(.+)`),
}

var issueRefRegex = regexp.MustCompile(`(?i)(#\d+|[A-Z]+-\d+|CVE-\d+-\d+)`)

// HarvestCommits scans Git history for bug-fixing commits and generates candidate scar drafts.
func (s *Store) HarvestCommits(opts HarvestOptions) (*HarvestResult, error) {
	if !GitAvailable(s.RepoRoot) {
		return nil, fmt.Errorf("git binary not available or repository lacks .git worktree")
	}

	maxCommits := opts.MaxCommits
	if maxCommits <= 0 {
		maxCommits = 50
	}

	maxFiles := opts.MaxFiles
	if maxFiles <= 0 {
		maxFiles = 5
	}

	minConfidence := opts.MinConfidence
	if minConfidence <= 0 {
		minConfidence = 7
	}

	gitArgs := []string{"log", fmt.Sprintf("-n%d", maxCommits), "--format=%H%x09%an%x09%s"}
	if opts.SinceRef != "" {
		gitArgs = append(gitArgs, opts.SinceRef+"..HEAD")
	}

	cmd := exec.Command("git", gitArgs...)
	cmd.Dir = s.RepoRoot
	out, err := cmd.Output()
	if err != nil {
		cmd = exec.Command("git", "log", fmt.Sprintf("-n%d", maxCommits), "--format=%H%x09%an%x09%s")
		cmd.Dir = s.RepoRoot
		out, err = cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("failed executing git log: %w", err)
		}
	}

	existingScars, _ := s.ListScars()
	knownCommits := make(map[string]bool)
	for _, sum := range existingScars {
		if strings.HasPrefix(sum.Scar.Lifecycle.Provenance, "git-commit:") {
			knownCommits[strings.TrimPrefix(sum.Scar.Lifecycle.Provenance, "git-commit:")] = true
		}
	}

	var candidates []HarvestCandidate
	scannedCount := 0

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		scannedCount++

		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}
		sha := parts[0]
		author := parts[1]
		subject := parts[2]

		if knownCommits[sha] {
			continue
		}

		isFix := isFixCommit(subject, opts.MatchPatterns)
		if !isFix {
			continue
		}

		diffFiles, err := getCommitFiles(s.RepoRoot, sha)
		if err != nil || len(diffFiles) == 0 {
			continue
		}

		// Noise filter 1: diffuse commits touching > maxFiles files are large refactors, not atomic bug fixes
		if len(diffFiles) > maxFiles {
			continue
		}

		var prodFiles []string
		var testFiles []string
		for _, f := range diffFiles {
			if isTestFile(f) {
				testFiles = append(testFiles, f)
			} else if isSourceFile(f) {
				prodFiles = append(prodFiles, f)
			}
		}

		if len(prodFiles) == 0 {
			continue
		}

		// Noise filter 2: require co-located test unless explicitly allowed
		if !opts.AllowUntested && len(testFiles) == 0 {
			continue
		}

		// Confidence calculation (1-10)
		confidence := 2
		var reasons []string

		if issueRefRegex.MatchString(subject) {
			confidence += 3
			reasons = append(reasons, "issue_ref(+3)")
		}
		if len(testFiles) > 0 {
			confidence += 3
			reasons = append(reasons, "test_colocated(+3)")
		}
		if len(diffFiles) <= 3 {
			confidence += 2
			reasons = append(reasons, "focused_diff(+2)")
		}
		if confidence > 10 {
			confidence = 10
		}

		// Noise filter 3: drop low-confidence candidates
		if confidence < minConfidence {
			continue
		}

		cleanTitle := cleanSubjectTitle(subject)
		lesson := fmt.Sprintf("Prevention lesson derived from fix commit %s by %s: ensure regression checks cover %s.", sha[:min(7, len(sha))], author, strings.Join(prodFiles, ", "))

		candidate := HarvestCandidate{
			CommitSHA:  sha,
			Subject:    subject,
			Author:     author,
			Files:      prodFiles,
			TestFiles:  testFiles,
			Title:      cleanTitle,
			Lesson:     lesson,
			Confidence: confidence,
			Reason:     strings.Join(reasons, ","),
		}

		// Infer multi-language verifier command
		candidate.Verifier = inferVerifierCommand(testFiles)

		if !opts.DryRun {
			draft, path, err := s.CreateDraft(candidate.Title, candidate.Lesson, candidate.Files)
			if err == nil {
				draft.Lifecycle.Provenance = "git-commit:" + sha
				if len(candidate.Verifier) > 0 {
					draft.Verifier.Argv = candidate.Verifier
				}
				_, _ = s.SaveScar(draft)
				candidate.SuggestedID = draft.ID
				candidate.DraftPath = path
			}
		}

		candidates = append(candidates, candidate)
	}

	result := &HarvestResult{
		ScannedCommits: scannedCount,
		FoundFixes:     len(candidates),
		Candidates:     candidates,
	}
	if !opts.DryRun {
		result.DraftsCreated = len(candidates)
		_, _ = s.RebuildIndex()
	}

	return result, nil
}

func inferVerifierCommand(testFiles []string) []string {
	if len(testFiles) == 0 {
		return nil
	}

	for _, tf := range testFiles {
		// Go
		if strings.HasSuffix(tf, "_test.go") {
			return []string{"go", "test", "-race", "./" + filepath.Dir(tf) + "/..."}
		}
		// TypeScript / JavaScript
		if strings.HasSuffix(tf, ".test.ts") || strings.HasSuffix(tf, ".spec.ts") ||
			strings.HasSuffix(tf, ".test.js") || strings.HasSuffix(tf, ".spec.js") {
			return []string{"npm", "test", "--", tf}
		}
		// Python
		if strings.HasPrefix(filepath.Base(tf), "test_") || strings.HasSuffix(tf, "_test.py") {
			return []string{"pytest", tf}
		}
		// Rust
		if strings.HasSuffix(tf, ".rs") {
			return []string{"cargo", "test"}
		}
	}
	return nil
}

func isFixCommit(subject string, customPatterns []string) bool {
	for _, re := range defaultFixRegexes {
		if re.MatchString(subject) {
			return true
		}
	}
	for _, pat := range customPatterns {
		if matched, _ := regexp.MatchString(pat, subject); matched {
			return true
		}
	}
	return false
}

func getCommitFiles(repoRoot, sha string) ([]string, error) {
	cmd := exec.Command("git", "diff-tree", "--no-commit-id", "--name-only", "-r", sha)
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var files []string
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		f := strings.TrimSpace(scanner.Text())
		if f != "" {
			files = append(files, f)
		}
	}
	return files, nil
}

func isSourceFile(f string) bool {
	ext := filepath.Ext(f)
	switch ext {
	case ".go", ".ts", ".js", ".py", ".rs", ".java", ".c", ".cpp", ".h":
		return true
	default:
		return false
	}
}

func cleanSubjectTitle(subject string) string {
	s := strings.TrimSpace(subject)
	for _, re := range defaultFixRegexes {
		if m := re.FindStringSubmatch(s); len(m) > 0 {
			s = m[len(m)-1]
			break
		}
	}
	s = strings.TrimSpace(s)
	if len(s) > 0 {
		return strings.ToUpper(s[:1]) + s[1:]
	}
	return subject
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
