package scar

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// BundleExportOptions defines filtering parameters when exporting a memory bundle.
type BundleExportOptions struct {
	FilterSeverity string   `json:"filter_severity,omitempty"`
	IncludeDrafts  bool     `json:"include_drafts,omitempty"`
	IDs            []string `json:"ids,omitempty"`
}

// BundleImportOptions defines options when importing a memory bundle into a repository.
type BundleImportOptions struct {
	IDPrefix string `json:"id_prefix,omitempty"` // e.g. "org-" or "team-"
	DryRun   bool   `json:"dry_run,omitempty"`
}

// MemoryBundle represents an exportable, portable pack of scars and metadata.
type MemoryBundle struct {
	Format     string `json:"format"` // "gitscars.bundle.v1"
	ExportedAt string `json:"exported_at"`
	Checksum   string `json:"checksum"`
	Count      int    `json:"count"`
	Scars      []Scar `json:"scars"`
}

// BundleImportResult summarizes the outcome of importing a bundle.
type BundleImportResult struct {
	TotalInBundle int      `json:"total_in_bundle"`
	Imported      int      `json:"imported"`
	Skipped       int      `json:"skipped"`
	ImportedIDs   []string `json:"imported_ids"`
}

// ExportBundle serializes repository scars into a portable verified JSON bundle.
func (s *Store) ExportBundle(opts BundleExportOptions) ([]byte, error) {
	summaries, err := s.ListScars()
	if err != nil {
		return nil, err
	}

	idFilter := make(map[string]bool)
	for _, id := range opts.IDs {
		idFilter[id] = true
	}

	var toExport []Scar
	hasher := sha256.New()

	for _, sum := range summaries {
		sc := sum.Scar

		// By default only active scars are exported unless IncludeDrafts is true
		if sc.Lifecycle.State != LifecycleActive && !opts.IncludeDrafts {
			continue
		}

		// ID filter
		if len(idFilter) > 0 && !idFilter[sc.ID] {
			continue
		}

		// Severity filter
		if opts.FilterSeverity != "" && !strings.EqualFold(sc.Severity, opts.FilterSeverity) {
			continue
		}

		toExport = append(toExport, *sc)
		hasher.Write([]byte(sc.ID + ":" + sc.Title + ":" + sc.Lesson + "\n"))
	}

	checksum := hex.EncodeToString(hasher.Sum(nil))

	bundle := MemoryBundle{
		Format:     "gitscars.bundle.v1",
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Checksum:   checksum,
		Count:      len(toExport),
		Scars:      toExport,
	}

	return json.MarshalIndent(bundle, "", "  ")
}

// ImportBundle ingests a verified memory bundle into the repository store as reviewable drafts.
func (s *Store) ImportBundle(data []byte, opts BundleImportOptions) (*BundleImportResult, error) {
	var bundle MemoryBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return nil, fmt.Errorf("invalid bundle JSON payload: %w", err)
	}

	if bundle.Format != "gitscars.bundle.v1" {
		return nil, fmt.Errorf("unsupported bundle format %q (expected 'gitscars.bundle.v1')", bundle.Format)
	}

	// Verify checksum integrity
	hasher := sha256.New()
	for _, sc := range bundle.Scars {
		hasher.Write([]byte(sc.ID + ":" + sc.Title + ":" + sc.Lesson + "\n"))
	}
	expectedChecksum := hex.EncodeToString(hasher.Sum(nil))
	if bundle.Checksum != "" && bundle.Checksum != expectedChecksum {
		return nil, fmt.Errorf("bundle checksum mismatch (corrupted or tampered archive)")
	}

	res := &BundleImportResult{
		TotalInBundle: len(bundle.Scars),
		ImportedIDs:   make([]string, 0),
	}

	for _, sc := range bundle.Scars {
		targetScar := sc

		// Apply optional ID prefix
		if opts.IDPrefix != "" {
			targetScar.ID = opts.IDPrefix + targetScar.ID
		}

		// Invariant: Imported scars from external sources MUST be imported as draft
		// and require human/agent review and fresh baseline verification in this workspace.
		targetScar.Lifecycle.State = LifecycleDraft
		targetScar.Review.Status = ReviewUnreviewed
		targetScar.Review.Reviewer = ""
		targetScar.Review.Date = ""
		targetScar.Verifier.Result = VerificationNotAttempted
		targetScar.Baseline.AnchorHashes = make(map[string]string)
		targetScar.Baseline.SymbolHashes = make(map[string]string)
		targetScar.Baseline.RecordedAt = ""

		if !opts.DryRun {
			if _, err := s.SaveScar(&targetScar); err != nil {
				res.Skipped++
				continue
			}
		}

		res.Imported++
		res.ImportedIDs = append(res.ImportedIDs, targetScar.ID)
	}

	if !opts.DryRun && res.Imported > 0 {
		_, _ = s.RebuildIndex()
	}

	return res, nil
}
