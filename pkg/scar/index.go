package scar

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
		UpdatedAt:     time.Now().UTC().Format(time.RFC3339),
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

	return index, nil
}

// LoadIndex reads .gitscars/.local/index.json if present.
func (s *Store) LoadIndex() (*IndexCache, error) {
	indexPath := filepath.Join(s.LocalDir, IndexFileName)
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, err
	}

	var index IndexCache
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("malformed index file %s: %w", indexPath, err)
	}

	return &index, nil
}
