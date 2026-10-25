package scar

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const IndexFileName = "index.json"

// IndexCache represents a rebuildable local cache of all repository scars.
type IndexCache struct {
	SchemaVersion string       `json:"schema_version"`
	UpdatedAt     string       `json:"updated_at"`
	TotalScars    int          `json:"total_scars"`
	Scars         []IndexEntry `json:"scars"`
}

// IndexEntry holds summary metadata and anchor definitions for fast lookups.
type IndexEntry struct {
	ID              string             `json:"id"`
	Title           string             `json:"title"`
	Lesson          string             `json:"lesson"`
	Lifecycle       string             `json:"lifecycle"`
	ReviewStatus    string             `json:"review_status"`
	Severity        string             `json:"severity,omitempty"`
	Files           []string           `json:"files,omitempty"`
	Symbols         []string           `json:"symbols,omitempty"`
	Commands        []string           `json:"commands,omitempty"`
	ErrorSignatures []string           `json:"error_signatures,omitempty"`
	Dependencies    []DependencyAnchor `json:"dependencies,omitempty"`
	Freshness       string             `json:"freshness,omitempty"`
	FileName        string             `json:"file_name"`
}

// RebuildIndex scans all scars from disk and writes .gitscars/.local/index.json.
func (s *Store) RebuildIndex() (*IndexCache, error) {
	summaries, err := s.ListScars()
	if err != nil {
		return nil, fmt.Errorf("failed listing scars for index: %w", err)
	}

	if err := os.MkdirAll(s.LocalDir, 0755); err != nil {
		return nil, fmt.Errorf("failed creating local state directory: %w", err)
	}

	index := &IndexCache{
		SchemaVersion: "reposcars.index.v1",
		UpdatedAt:     time.Now().UTC().Format(time.RFC3339Nano),
		TotalScars:    len(summaries),
		Scars:         make([]IndexEntry, 0, len(summaries)),
	}

	for _, sum := range summaries {
		sc := sum.Scar
		entry := IndexEntry{
			ID:              sc.ID,
			Title:           sc.Title,
			Lesson:          sc.Lesson,
			Lifecycle:       sc.Lifecycle.State,
			ReviewStatus:    sc.Review.Status,
			Severity:        sc.Severity,
			Files:           sc.Anchors.Files,
			Symbols:         sc.Anchors.Symbols,
			Commands:        sc.Anchors.Commands,
			ErrorSignatures: sc.Anchors.ErrorSignatures,
			Dependencies:    sc.Anchors.Dependencies,
			Freshness:       sum.Freshness,
			FileName:        filepath.Base(sum.FilePath),
		}
		index.Scars = append(index.Scars, entry)
	}

	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed marshaling index: %w", err)
	}

	indexPath := filepath.Join(s.LocalDir, IndexFileName)
	if err := os.WriteFile(indexPath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed writing index file %s: %w", indexPath, err)
	}

	// The rewritten file invalidates any decode this process is holding.
	s.cache.drop(indexPath)

	return index, nil
}

// LoadIndex reads .gitscars/.local/index.json if present.
//
// The decoded index is reused while the file keeps its stamp; callers must
// treat the returned value as read-only. Recall revalidates the index
// against the scar directory on every call, so a stale index is never
// served for retrieval even though its decode is memoised here.
func (s *Store) LoadIndex() (*IndexCache, error) {
	indexPath := filepath.Join(s.LocalDir, IndexFileName)

	// Stamp before reading: a write between the stamp and the read then
	// produces a mismatching stamp on the next lookup instead of pinning
	// stale bytes under the new stamp.
	stamp := stampFile(indexPath)
	if idx, ok := s.cache.getIndex(indexPath, stamp); ok {
		return idx, nil
	}

	data, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, err
	}

	var index IndexCache
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("malformed index file %s: %w", indexPath, err)
	}

	s.cache.putIndex(indexPath, stamp, &index)
	return &index, nil
}

// IsIndexFresh verifies that index.json is newer than all .md scar files in ScarsDir
// and that total count matches. Returns false if any file is newer or count differs.
func (s *Store) IsIndexFresh(idx *IndexCache) bool {
	if idx == nil {
		return false
	}
	updatedAt, err := time.Parse(time.RFC3339Nano, idx.UpdatedAt)
	if err != nil {
		updatedAt, err = time.Parse(time.RFC3339, idx.UpdatedAt)
		if err != nil {
			return false
		}
	}

	entries, err := os.ReadDir(s.ScarsDir)
	if err != nil {
		return false
	}

	count := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		count++
		info, err := e.Info()
		if err != nil {
			return false
		}
		if info.ModTime().After(updatedAt) {
			return false
		}
	}

	return count == idx.TotalScars
}
