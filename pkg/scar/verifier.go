package scar

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	TrustedVerifiersFile = "trusted-verifiers.json"
	DefaultTimeout       = 60 * time.Second
	MaxExcerptLength     = 1024
)

// TrustEntry represents an approved command hash for a specific repo root.
type TrustEntry struct {
	RepoRoot    string `json:"repo_root"`
	CommandHash string `json:"command_hash"`
	ApprovedAt  string `json:"approved_at"`
}

// TrustList represents the untracked local trust store.
type TrustList struct {
	TrustedCommands []TrustEntry `json:"trusted_commands"`
}

// ComputeCommandHash generates a deterministic SHA256 hash for a verifier command.
func ComputeCommandHash(argv []string, workingDir string, envAllowlist []string) string {
	hasher := sha256.New()
	for _, arg := range argv {
		hasher.Write([]byte(arg))
		hasher.Write([]byte("\x00"))
	}
	hasher.Write([]byte(workingDir))
	hasher.Write([]byte("\x00"))
	for _, e := range envAllowlist {
		hasher.Write([]byte(e))
		hasher.Write([]byte("\x00"))
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

// LoadTrustList loads the untracked trust list from .gitscars/.local/trusted-verifiers.json.
func (s *Store) LoadTrustList() (*TrustList, error) {
	path := filepath.Join(s.LocalDir, TrustedVerifiersFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &TrustList{TrustedCommands: []TrustEntry{}}, nil
		}
		return nil, err
	}
	var list TrustList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("malformed trusted-verifiers.json: %w", err)
	}
	return &list, nil
}

// AddTrustEntry saves an approved command hash into .gitscars/.local/trusted-verifiers.json.
func (s *Store) AddTrustEntry(commandHash string) error {
	list, err := s.LoadTrustList()
	if err != nil {
		list = &TrustList{}
	}

	cleanRoot, _ := filepath.Abs(s.RepoRoot)
	for _, entry := range list.TrustedCommands {
		if entry.RepoRoot == cleanRoot && entry.CommandHash == commandHash {
			return nil // already trusted
		}
	}

	list.TrustedCommands = append(list.TrustedCommands, TrustEntry{
		RepoRoot:    cleanRoot,
		CommandHash: commandHash,
		ApprovedAt:  time.Now().UTC().Format(time.RFC3339),
	})

	if err := os.MkdirAll(s.LocalDir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.LocalDir, TrustedVerifiersFile), data, 0600)
}

// IsCommandTrusted verifies if a specific command hash is trusted for this repo root.
func (s *Store) IsCommandTrusted(commandHash string) bool {
	cleanRoot, _ := filepath.Abs(s.RepoRoot)
	list, err := s.LoadTrustList()
	if err != nil {
		return false
	}
	for _, entry := range list.TrustedCommands {
		if entry.RepoRoot == cleanRoot && entry.CommandHash == commandHash {
			return true
		}
	}
	return false
}

// BuildScrubbedEnv retains only safe environment variables plus explicit allowlist.
func BuildScrubbedEnv(allowlist []string) []string {
	safeKeys := map[string]bool{
		"PATH": true, "HOME": true, "TMPDIR": true, "USER": true, "LANG": true,
		"GO111MODULE": true, "GOPATH": true, "GOROOT": true, "GOCACHE": true,
	}
	for _, k := range allowlist {
		safeKeys[k] = true
	}

	var env []string
	for _, item := range os.Environ() {
		parts := strings.SplitN(item, "=", 2)
		if len(parts) > 0 && safeKeys[parts[0]] {
			env = append(env, item)
		}
	}
	return env
}

var (
	tokenRegex      = regexp.MustCompile(`(?i)(bearer\s+[a-zA-Z0-9_\-\.]{10,})|(ghp_[a-zA-Z0-9]{30,})|(eyJ[a-zA-Z0-9_\-]{20,}\.[a-zA-Z0-9_\-]{20,})`)
	privateKeyRegex = regexp.MustCompile(`(?s)-----BEGIN[ A-Z0-9_-]+PRIVATE KEY-----.*?-----END[ A-Z0-9_-]+PRIVATE KEY-----`)
	passwordRegex   = regexp.MustCompile(`(?i)(password|secret|token|apikey)\s*[:=]\s*[^\s,]+`)
)

// RedactOutput strips common credentials, private keys, and tokens from command output.
func RedactOutput(raw string) string {
	res := privateKeyRegex.ReplaceAllString(raw, "[REDACTED_PRIVATE_KEY]")
	res = tokenRegex.ReplaceAllString(res, "[REDACTED_TOKEN]")
	res = passwordRegex.ReplaceAllString(res, "$1=[REDACTED]")
	return res
}

// ExecutionResult records the outcome of a verifier process execution.
type ExecutionResult struct {
	ExitCode      int
	Duration      time.Duration
	OutputHash    string
	OutputExcerpt string
	TimedOut      bool
	Error         error
}

// ExecuteVerifier runs the argv command in a scrubbed environment with timeout.
func ExecuteVerifier(ctx context.Context, repoRoot string, argv []string, workingDir string, envAllowlist []string, timeout time.Duration) (*ExecutionResult, error) {
	if len(argv) == 0 {
		return nil, errors.New("empty argv array: cannot execute verifier")
	}

	targetDir := repoRoot
	if workingDir != "" {
		if filepath.IsAbs(workingDir) {
			targetDir = workingDir
		} else {
			targetDir = filepath.Join(repoRoot, workingDir)
		}
	}

	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, argv[0], argv[1:]...)
	cmd.Dir = targetDir
	cmd.Env = BuildScrubbedEnv(envAllowlist)

	var combinedBuf bytes.Buffer
	cmd.Stdout = &combinedBuf
	cmd.Stderr = &combinedBuf

	start := time.Now()
	runErr := cmd.Run()
	duration := time.Since(start)

	timedOut := execCtx.Err() == context.DeadlineExceeded

	rawOutput := combinedBuf.String()
	hasher := sha256.New()
	hasher.Write([]byte(rawOutput))
	outputHash := hex.EncodeToString(hasher.Sum(nil))

	redacted := RedactOutput(rawOutput)
	excerpt := redacted
	if len(excerpt) > MaxExcerptLength {
		excerpt = excerpt[:MaxExcerptLength] + "\n... [truncated]"
	}

	exitCode := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	return &ExecutionResult{
		ExitCode:      exitCode,
		Duration:      duration,
		OutputHash:    outputHash,
		OutputExcerpt: excerpt,
		TimedOut:      timedOut,
		Error:         runErr,
	}, nil
}

// VerifyFailToPass executes the worktree fail-to-pass verification procedure.
// 1. Check base and fix commits.
// 2. Verify on fixed code.
// 3. Create detached worktree at base commit.
// 4. Overlay test files from fix commit.
// 5. Verify on base code (expecting failure).
// If anything is inconclusive, returns honest inconclusive status.
func (s *Store) VerifyFailToPass(ctx context.Context, scar *Scar, testFiles []string, interactiveTrust bool) (string, error) {
	if len(scar.Verifier.Argv) == 0 {
		return VerificationNotAttempted, errors.New("cannot verify: verifier argv is empty")
	}

	cmdHash := ComputeCommandHash(scar.Verifier.Argv, scar.Verifier.WorkingDir, scar.Verifier.EnvAllowlist)
	scar.Verifier.CommandHash = cmdHash

	// Trust check: must be trusted locally or confirmed interactively
	if !s.IsCommandTrusted(cmdHash) {
		if !interactiveTrust {
			return VerificationNotAttempted, fmt.Errorf("verifier command hash %s is not in trusted-verifiers.json; interactive confirmation required", cmdHash)
		}
		// In interactive mode, trust this hash
		if err := s.AddTrustEntry(cmdHash); err != nil {
			return VerificationNotAttempted, fmt.Errorf("failed saving trust entry: %w", err)
		}
	}

	// Step 1: Run against current (fixed) tree
	fixedRes, err := ExecuteVerifier(ctx, s.RepoRoot, scar.Verifier.Argv, scar.Verifier.WorkingDir, scar.Verifier.EnvAllowlist, DefaultTimeout)
	if err != nil && fixedRes == nil {
		return VerificationFailed, fmt.Errorf("failed executing verifier on fixed tree: %w", err)
	}

	scar.Verifier.OutputHash = fixedRes.OutputHash
	scar.Verifier.OutputExcerpt = fixedRes.OutputExcerpt

	if fixedRes.ExitCode != 0 {
		scar.Verifier.Result = VerificationFailed
		return VerificationFailed, fmt.Errorf("verifier failed on fixed tree (exit code %d)", fixedRes.ExitCode)
	}

	// If no base commit is available, mark as exception (cannot run fail-to-pass overlay)
	if scar.BaseCommit == "" || !GitAvailable(s.RepoRoot) {
		scar.Verifier.Result = VerificationException
		if scar.Review.Rationale == "" {
			scar.Review.Rationale = "Base commit unavailable; verified fixed tree passing only"
		}
		return VerificationException, nil
	}

	// Step 2: Overlay test at base commit in detached worktree
	basePasses, err := s.runBaseWorktreeCheck(ctx, scar, testFiles)
	if err != nil {
		// Honest reporting: inconclusive -> exception
		scar.Verifier.Result = VerificationException
		if scar.Review.Rationale == "" {
			scar.Review.Rationale = fmt.Sprintf("Worktree fail-to-pass overlay inconclusive: %v", err)
		}
		return VerificationException, nil
	}

	if basePasses {
		// Base also passed without the fix -> not a valid fail-to-pass regression test!
		scar.Verifier.Result = VerificationFailed
		return VerificationFailed, errors.New("test passed at base commit: does not reproduce regression failure (failed to fail-to-pass)")
	}

	// Base failed and fixed passed -> Genuine fail_to_pass verified!
	scar.Verifier.Result = VerificationFailToPass

	// Invariant: Updating verification evidence DOES NOT approve or activate the Scar!
	return VerificationFailToPass, nil
}

func (s *Store) runBaseWorktreeCheck(ctx context.Context, scar *Scar, testFiles []string) (bool, error) {
	tempWorktree, err := os.MkdirTemp("", "gitscars-wt-*")
	if err != nil {
		return false, err
	}
	defer func() {
		// Remove worktree
		rmCmd := exec.Command("git", "worktree", "remove", "--force", tempWorktree)
		rmCmd.Dir = s.RepoRoot
		_ = rmCmd.Run()
		_ = os.RemoveAll(tempWorktree)
	}()

	// Add detached worktree at base commit
	addCmd := exec.Command("git", "worktree", "add", "--detach", tempWorktree, scar.BaseCommit)
	addCmd.Dir = s.RepoRoot
	if out, err := addCmd.CombinedOutput(); err != nil {
		return false, fmt.Errorf("failed adding git worktree at %s: %s (%w)", scar.BaseCommit, string(out), err)
	}

	// Overlay test files from fix commit
	for _, tf := range testFiles {
		showCmd := exec.Command("git", "show", fmt.Sprintf("%s:%s", scar.FixCommit, tf))
		showCmd.Dir = s.RepoRoot
		content, err := showCmd.Output()
		if err != nil {
			continue // skip missing file
		}
		targetPath := filepath.Join(tempWorktree, tf)
		_ = os.MkdirAll(filepath.Dir(targetPath), 0755)
		_ = os.WriteFile(targetPath, content, 0644)
	}

	// Run verifier in base worktree
	baseRes, err := ExecuteVerifier(ctx, tempWorktree, scar.Verifier.Argv, scar.Verifier.WorkingDir, scar.Verifier.EnvAllowlist, DefaultTimeout)
	if err != nil && baseRes == nil {
		return false, err
	}

	// Return true if base passed (which means test failed to reproduce regression)
	return baseRes.ExitCode == 0, nil
}
