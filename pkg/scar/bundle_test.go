package scar

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBundleExportAndImportRoundtrip(t *testing.T) {
	// 1. Setup Source Store
	srcDir := t.TempDir()
	srcStore := NewStore(srcDir)
	_ = srcStore.Init()

	s1, _, _ := srcStore.CreateDraft("Goroutine Leak", "Always close done channel", []string{"pkg/worker.go"})
	s1.Lifecycle.State = LifecycleActive
	s1.Review.Status = ReviewApproved
	s1.Verifier.Result = VerificationFailToPass
	s1.Severity = SeverityHigh
	_ = os.MkdirAll(filepath.Join(srcDir, "pkg"), 0755)
	_ = os.WriteFile(filepath.Join(srcDir, "pkg/worker.go"), []byte("package worker\n"), 0644)
	_ = srcStore.RecordBaseline(s1, "hash-cmd")
	_, _ = srcStore.SaveScar(s1)

	// 2. Export Bundle
	bundleBytes, err := srcStore.ExportBundle(BundleExportOptions{})
	if err != nil {
		t.Fatalf("export bundle failed: %v", err)
	}

	var parsedBundle MemoryBundle
	if err := json.Unmarshal(bundleBytes, &parsedBundle); err != nil {
		t.Fatalf("unmarshal bundle failed: %v", err)
	}
	if parsedBundle.Count != 1 || len(parsedBundle.Scars) != 1 {
		t.Fatalf("expected 1 scar in bundle, got %d", parsedBundle.Count)
	}
	if parsedBundle.Checksum == "" {
		t.Errorf("expected non-empty checksum in bundle")
	}

	// 3. Setup Target Store
	targetDir := t.TempDir()
	targetStore := NewStore(targetDir)
	_ = targetStore.Init()

	// 4. Test Dry-Run Import
	dryRes, err := targetStore.ImportBundle(bundleBytes, BundleImportOptions{
		IDPrefix: "org-",
		DryRun:   true,
	})
	if err != nil {
		t.Fatalf("dry-run import failed: %v", err)
	}
	if dryRes.Imported != 1 {
		t.Errorf("expected 1 imported candidate on dry-run, got %d", dryRes.Imported)
	}
	// Verify no files were actually written to target
	targetScars, _ := targetStore.ListScars()
	if len(targetScars) != 0 {
		t.Errorf("expected 0 scars written on dry-run, found %d", len(targetScars))
	}

	// 5. Test Real Import
	realRes, err := targetStore.ImportBundle(bundleBytes, BundleImportOptions{
		IDPrefix: "org-",
		DryRun:   false,
	})
	if err != nil {
		t.Fatalf("real import failed: %v", err)
	}
	if realRes.Imported != 1 || len(realRes.ImportedIDs) != 1 {
		t.Fatalf("expected 1 imported scar, got %d", realRes.Imported)
	}

	expectedID := "org-" + s1.ID
	if realRes.ImportedIDs[0] != expectedID {
		t.Errorf("expected imported ID %s, got %s", expectedID, realRes.ImportedIDs[0])
	}

	// 6. Verify Target Invariants: Must be draft, unreviewed, with fresh unbaselined state
	importedScar, _, err := targetStore.LoadScar(expectedID)
	if err != nil {
		t.Fatalf("failed loading imported scar: %v", err)
	}
	if importedScar.Lifecycle.State != LifecycleDraft {
		t.Errorf("expected draft state for imported scar, got %s", importedScar.Lifecycle.State)
	}
	if importedScar.Review.Status != ReviewUnreviewed {
		t.Errorf("expected unreviewed status for imported scar, got %s", importedScar.Review.Status)
	}
	if importedScar.Verifier.Result != VerificationNotAttempted {
		t.Errorf("expected verification not_attempted, got %s", importedScar.Verifier.Result)
	}

	// 7. Test Tamper Checksum Validation
	parsedBundle.Checksum = "bad-hash-tampered"
	tamperedBytes, _ := json.Marshal(parsedBundle)
	_, err = targetStore.ImportBundle(tamperedBytes, BundleImportOptions{})
	if err == nil {
		t.Errorf("expected error when importing bundle with tampered checksum")
	}
}
