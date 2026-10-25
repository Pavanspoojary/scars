package scar

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
)

// AnchorDiffResult represents diff status for a single anchor file.
type AnchorDiffResult struct {
	FilePath string
	Status   string // "current", "modified", "missing", "unrecorded"
	Diff     string
}

// DiffScarAnchors inspects changes in anchor files for a scar compared to recorded baseline.
func (s *Store) DiffScarAnchors(scar *Scar) ([]AnchorDiffResult, error) {
	var results []AnchorDiffResult

	for _, relPath := range scar.Anchors.Files {
		fullPath := filepath.Join(s.RepoRoot, relPath)
		currentHash, err := s.cachedHashFile(fullPath)
		if err != nil {
			results = append(results, AnchorDiffResult{
				FilePath: relPath,
				Status:   "missing",
				Diff:     fmt.Sprintf("File %s not found on disk", relPath),
			})
			continue
		}

		expectedHash, hasBaseline := scar.Baseline.AnchorHashes[relPath]
		if !hasBaseline {
			results = append(results, AnchorDiffResult{
				FilePath: relPath,
				Status:   "unrecorded",
				Diff:     "No baseline hash recorded for this anchor",
			})
			continue
		}

		if currentHash == expectedHash {
			results = append(results, AnchorDiffResult{
				FilePath: relPath,
				Status:   "current",
				Diff:     "",
			})
			continue
		}

		// File is modified. Attempt git diff if git repository exists.
		diffText := ""
		if scar.FixCommit != "" {
			diffText = gitDiffFiles(s.RepoRoot, scar.FixCommit, relPath)
		}
		if diffText == "" && scar.BaseCommit != "" {
			diffText = gitDiffFiles(s.RepoRoot, scar.BaseCommit, relPath)
		}
		if diffText == "" {
			diffText = gitDiffFiles(s.RepoRoot, "HEAD", relPath)
		}
		if diffText == "" {
			diffText = fmt.Sprintf("Hash mismatch: current %s != baseline %s", currentHash[:12], expectedHash[:12])
		}

		results = append(results, AnchorDiffResult{
			FilePath: relPath,
			Status:   "modified",
			Diff:     diffText,
		})
	}

	return results, nil
}

func gitDiffFiles(repoRoot, ref, relPath string) string {
	cmd := exec.Command("git", "diff", ref, "--", relPath)
	cmd.Dir = repoRoot
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err == nil && out.Len() > 0 {
		return out.String()
	}
	return ""
}
