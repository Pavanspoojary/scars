package scar

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Store manages loading, creating, and updating Scar records on disk.
type Store struct {
	RepoRoot string
	ScarsDir string
	LocalDir string
}

// NewStore initializes a Store rooted at the repository directory.
func NewStore(repoRoot string) *Store {
	return &Store{
		RepoRoot: repoRoot,
		ScarsDir: filepath.Join(repoRoot, DefaultStoreDir, ScarsDirName),
		LocalDir: filepath.Join(repoRoot, DefaultStoreDir, LocalDirName),
	}
}

// Init initializes the .gitscars directory structure and config.
func (s *Store) Init() error {
	if err := os.MkdirAll(s.ScarsDir, 0755); err != nil {
		return fmt.Errorf("failed creating scars directory: %w", err)
	}
	if err := os.MkdirAll(s.LocalDir, 0755); err != nil {
		return fmt.Errorf("failed creating local state directory: %w", err)
	}

	// Ensure .local is gitignored
	gitignorePath := filepath.Join(s.RepoRoot, DefaultStoreDir, ".gitignore")
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		if err := os.WriteFile(gitignorePath, []byte(".local/\n"), 0644); err != nil {
			return fmt.Errorf("failed writing .gitscars/.gitignore: %w", err)
		}
	}

	// Write default config.yaml if missing
	configPath := filepath.Join(s.RepoRoot, DefaultStoreDir, ConfigFileName)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		defaultConfig := `# GitScars Configuration
schema_version: "1"
abstain_threshold: 3
top_k: 3
`
		if err := os.WriteFile(configPath, []byte(defaultConfig), 0644); err != nil {
			return fmt.Errorf("failed writing %s: %w", ConfigFileName, err)
		}
	}

	return nil
}

// CreateDraft creates a new draft Scar file on disk and returns the Scar.
func (s *Store) CreateDraft(title, lesson string, files []string) (*Scar, string, error) {
	slug := Slugify(title)
	draft, err := NewDraft(slug, title, lesson, files)
	if err != nil {
		return nil, "", err
	}

	filename := fmt.Sprintf("%s.md", draft.ID)
	targetPath := filepath.Join(s.ScarsDir, filename)

	data, err := FormatFrontmatter(draft)
	if err != nil {
		return nil, "", fmt.Errorf("failed formatting frontmatter: %w", err)
	}

	if err := os.WriteFile(targetPath, data, 0644); err != nil {
		return nil, "", fmt.Errorf("failed writing scar file: %w", err)
	}

	return draft, targetPath, nil
}

// LoadScar loads a single Scar by ID or filename.
func (s *Store) LoadScar(idOrFile string) (*Scar, string, error) {
	var targetPath string
	if strings.HasSuffix(idOrFile, ".md") {
		targetPath = filepath.Join(s.ScarsDir, idOrFile)
		if _, err := os.Stat(targetPath); os.IsNotExist(err) {
			targetPath = idOrFile // try direct path
		}
	} else {
		targetPath = filepath.Join(s.ScarsDir, idOrFile+".md")
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		// Try scanning directory for partial ID match
		matchedPath, findErr := s.findByID(idOrFile)
		if findErr != nil {
			return nil, "", fmt.Errorf("scar %q not found: %w", idOrFile, err)
		}
		targetPath = matchedPath
		data, err = os.ReadFile(targetPath)
		if err != nil {
			return nil, "", fmt.Errorf("failed reading scar at %s: %w", targetPath, err)
		}
	}

	scar, err := ParseFrontmatter(data)
	if err != nil {
		return nil, targetPath, fmt.Errorf("failed parsing scar %s: %w", targetPath, err)
	}

	return scar, targetPath, nil
}

func (s *Store) findByID(id string) (string, error) {
	entries, err := os.ReadDir(s.ScarsDir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			name := strings.TrimSuffix(e.Name(), ".md")
			if name == id || strings.HasSuffix(name, "-"+id) {
				return filepath.Join(s.ScarsDir, e.Name()), nil
			}
		}
	}
	return "", fmt.Errorf("scar ID %q not found in %s", id, s.ScarsDir)
}

// SaveScar writes an updated Scar back to disk.
func (s *Store) SaveScar(scar *Scar) (string, error) {
	if err := scar.ValidateInvariants(); err != nil {
		return "", fmt.Errorf("cannot save scar with invalid invariants: %w", err)
	}

	filename := fmt.Sprintf("%s.md", scar.ID)
	targetPath := filepath.Join(s.ScarsDir, filename)

	data, err := FormatFrontmatter(scar)
	if err != nil {
		return "", fmt.Errorf("failed formatting scar: %w", err)
	}

	if err := os.WriteFile(targetPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed writing scar to %s: %w", targetPath, err)
	}

	return targetPath, nil
}

// ListScars returns all loaded scars and their current dynamic freshness.
type ScarSummary struct {
	Scar      *Scar
	FilePath  string
	Freshness string
}

// ParseFailure records a scar file that could not be loaded from disk.
type ParseFailure struct {
	FilePath string
	ID       string
	Error    string
}

// ListScars returns the catalog of loadable scars. Files that fail to load are
// omitted; use ListScarsDetailed to surface them.
func (s *Store) ListScars() ([]ScarSummary, error) {
	scars, _, err := s.ListScarsDetailed()
	return scars, err
}

// ListScarsDetailed returns the catalog of loadable scars together with the
// files that failed to load. Load failures are reported rather than discarded so
// that 'git-scars check', 'lint', and 'doctor' can surface malformed records
// instead of silently passing them.
func (s *Store) ListScarsDetailed() ([]ScarSummary, []ParseFailure, error) {
	if _, err := os.Stat(s.ScarsDir); os.IsNotExist(err) {
		return nil, nil, nil
	}

	entries, err := os.ReadDir(s.ScarsDir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed reading scars directory: %w", err)
	}

	var mdFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			mdFiles = append(mdFiles, filepath.Join(s.ScarsDir, e.Name()))
		}
	}

	if len(mdFiles) == 0 {
		return nil, nil, nil
	}

	workers := 8
	if len(mdFiles) < workers {
		workers = len(mdFiles)
	}

	jobs := make(chan string, len(mdFiles))
	for _, p := range mdFiles {
		jobs <- p
	}
	close(jobs)

	var (
		mu       sync.Mutex
		results  []ScarSummary
		failures []ParseFailure
		wg       sync.WaitGroup
	)

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				data, err := os.ReadFile(path)
				if err != nil {
					mu.Lock()
					failures = append(failures, ParseFailure{
						FilePath: path,
						ID:       strings.TrimSuffix(filepath.Base(path), ".md"),
						Error:    err.Error(),
					})
					mu.Unlock()
					continue
				}
				scar, err := ParseFrontmatter(data)
				if err != nil {
					mu.Lock()
					failures = append(failures, ParseFailure{
						FilePath: path,
						ID:       strings.TrimSuffix(filepath.Base(path), ".md"),
						Error:    err.Error(),
					})
					mu.Unlock()
					continue
				}
				freshness := s.ComputeFreshness(scar)
				summary := ScarSummary{
					Scar:      scar,
					FilePath:  path,
					Freshness: freshness,
				}
				mu.Lock()
				results = append(results, summary)
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// Sort stably by ID
	sort.Slice(results, func(i, j int) bool {
		return results[i].Scar.ID < results[j].Scar.ID
	})
	sort.Slice(failures, func(i, j int) bool {
		return failures[i].FilePath < failures[j].FilePath
	})

	return results, failures, nil
}

// ComputeFreshness dynamically compares current file hashes to recorded baseline hashes.
// This is strictly READ-ONLY and does not mutate any file.
func (s *Store) ComputeFreshness(scar *Scar) string {
	if len(scar.Baseline.AnchorHashes) == 0 {
		return FreshnessUnavailable
	}

	if len(scar.Baseline.AnchorHashes) == 1 {
		for relPath, expectedHash := range scar.Baseline.AnchorHashes {
			fullPath := filepath.Join(s.RepoRoot, relPath)
			currentHash, err := hashFile(fullPath)
			if err != nil {
				return FreshnessUnavailable
			}
			if currentHash != expectedHash {
				// Fine-grained symbol check: if anchored symbols are recorded and unchanged, preserve current status
				if len(scar.Baseline.SymbolHashes) > 0 && IsSupportedSymbolFile(relPath) {
					allPreserved := true
					for sym, baselineHash := range scar.Baseline.SymbolHashes {
						currSymHash, err := ExtractSymbolHash(fullPath, sym)
						if err != nil || currSymHash != baselineHash {
							allPreserved = false
							break
						}
					}
					if allPreserved {
						return FreshnessCurrent
					}
				}
				return FreshnessChanged
			}
		}
		return FreshnessCurrent
	}

	type checkResult struct {
		relPath     string
		unavailable bool
		changed     bool
	}

	results := make(chan checkResult, len(scar.Baseline.AnchorHashes))
	var wg sync.WaitGroup

	for relPath, expectedHash := range scar.Baseline.AnchorHashes {
		wg.Add(1)
		go func(rp, exp string) {
			defer wg.Done()
			fullPath := filepath.Join(s.RepoRoot, rp)
			curr, err := hashFile(fullPath)
			if err != nil {
				results <- checkResult{relPath: rp, unavailable: true}
				return
			}
			if curr != exp {
				results <- checkResult{relPath: rp, changed: true}
				return
			}
			results <- checkResult{relPath: rp}
		}(relPath, expectedHash)
	}

	wg.Wait()
	close(results)

	hasChanged := false
	for r := range results {
		if r.unavailable {
			return FreshnessUnavailable
		}
		if r.changed {
			hasChanged = true
		}
	}

	if hasChanged {
		// Fine-grained symbol verification across modified files
		if len(scar.Baseline.SymbolHashes) > 0 {
			allPreserved := true
			for sym, baselineHash := range scar.Baseline.SymbolHashes {
				found := false
				for _, f := range scar.Anchors.Files {
					if !IsSupportedSymbolFile(f) {
						continue
					}
					fullPath := filepath.Join(s.RepoRoot, f)
					currSymHash, err := ExtractSymbolHash(fullPath, sym)
					if err == nil {
						found = true
						if currSymHash != baselineHash {
							allPreserved = false
							break
						}
					}
				}
				if !found || !allPreserved {
					allPreserved = false
					break
				}
			}
			if allPreserved {
				return FreshnessCurrent
			}
		}
		return FreshnessChanged
	}
	return FreshnessCurrent
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// RecordBaseline calculates current hashes of anchor files and updates the scar's baseline.
func (s *Store) RecordBaseline(scar *Scar, commandHash string) error {
	// The baseline is replaced rather than merged. A scar that is re-anchored
	// must not stay coupled to a file it no longer references.
	anchorHashes := make(map[string]string)
	for _, f := range scar.Anchors.Files {
		fullPath := filepath.Join(s.RepoRoot, f)
		h, err := hashFile(fullPath)
		if err == nil {
			anchorHashes[f] = h
		}
	}
	scar.Baseline.AnchorHashes = anchorHashes

	scar.Baseline.RecordedAt = time.Now().UTC().Format(time.RFC3339)

	// Compute and record fine-grained AST symbol baselines
	scar.Baseline.SymbolHashes = s.ComputeSymbolBaselines(scar)

	if commandHash != "" {
		scar.Baseline.CommandHash = commandHash
	}

	return nil
}

// Reaffirm updates a scar's anchor baseline with explicit human reviewer rationale.
func (s *Store) Reaffirm(id, reviewer, rationale string) (*Scar, error) {
	if strings.TrimSpace(reviewer) == "" {
		return nil, errors.New("reaffirmation requires an explicit reviewer name")
	}
	if strings.TrimSpace(rationale) == "" {
		return nil, errors.New("reaffirmation requires a documented rationale")
	}

	scar, _, err := s.LoadScar(id)
	if err != nil {
		return nil, err
	}

	if err := s.RecordBaseline(scar, scar.Verifier.CommandHash); err != nil {
		return nil, fmt.Errorf("failed recording baseline: %w", err)
	}

	scar.Review.Reviewer = reviewer
	scar.Review.Rationale = fmt.Sprintf("Reaffirmed: %s", rationale)

	if _, err := s.SaveScar(scar); err != nil {
		return nil, err
	}

	return scar, nil
}

// ContradictScar transitions a scar to contradicted state with a conflict reference and rationale.
func (s *Store) ContradictScar(oldID, byID, rationale string) (*Scar, error) {
	scar, _, err := s.LoadScar(oldID)
	if err != nil {
		return nil, err
	}

	if err := scar.Contradict(byID, rationale); err != nil {
		return nil, err
	}

	if _, err := s.SaveScar(scar); err != nil {
		return nil, err
	}

	_, _ = s.RebuildIndex()
	return scar, nil
}

// SupersedeScar transitions a scar to superseded state by a successor scar.
func (s *Store) SupersedeScar(oldID, byID string) (*Scar, error) {
	oldScar, _, err := s.LoadScar(oldID)
	if err != nil {
		return nil, err
	}

	if err := oldScar.Supersede(byID); err != nil {
		return nil, err
	}

	if _, err := s.SaveScar(oldScar); err != nil {
		return nil, err
	}

	// Also record superseded ID in successor scar if successor exists
	if newScar, _, err := s.LoadScar(byID); err == nil {
		found := false
		for _, sup := range newScar.Lifecycle.Supersedes {
			if sup == oldID {
				found = true
				break
			}
		}
		if !found {
			newScar.Lifecycle.Supersedes = append(newScar.Lifecycle.Supersedes, oldID)
			_, _ = s.SaveScar(newScar)
		}
	}

	_, _ = s.RebuildIndex()
	return oldScar, nil
}
