package scar

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractErrorSignatures(t *testing.T) {
	sampleLog := `=== RUN   TestWorkerBurst
fatal error: all goroutines are asleep - deadlock!
goroutine 1 [chan send]:
main.main()
	/app/cmd/worker.go:42 +0x120
exit status 2
FAIL	pkg/worker	0.012s
`
	sigs := ExtractErrorSignatures(sampleLog)
	if len(sigs) == 0 {
		t.Fatalf("expected extracted signatures, got none")
	}

	foundDeadlock := false
	for _, s := range sigs {
		if strings.Contains(s, "deadlock") {
			foundDeadlock = true
			break
		}
	}
	if !foundDeadlock {
		t.Errorf("expected deadlock signature, got: %v", sigs)
	}
}

func TestTriageError(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)
	_ = store.Init()

	anchorPath := filepath.Join(tmpDir, "pkg", "worker.go")
	_ = os.MkdirAll(filepath.Dir(anchorPath), 0755)
	_ = os.WriteFile(anchorPath, []byte("package worker\n"), 0644)

	s, _, _ := store.CreateDraft("Worker Channel Deadlock", "Buffer worker task channels", []string{"pkg/worker.go"})
	s.Anchors.ErrorSignatures = []string{"all goroutines are asleep - deadlock"}
	s.Lifecycle.State = LifecycleActive
	s.Review.Status = ReviewApproved
	s.Verifier = failToPassVerifier("go", "test", "-race", "./...")
	_ = store.RecordBaseline(s, s.Verifier.CommandHash)
	_, _ = store.SaveScar(s)

	trace := `fatal error: all goroutines are asleep - deadlock!
goroutine 19 [chan send]:
pkg/worker.go:45`

	res, err := store.TriageError(trace)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Matches) == 0 {
		t.Fatalf("expected triage matches for deadlock trace, got 0")
	}
	if res.Matches[0].Scar.ID != s.ID {
		t.Errorf("expected matched scar %s, got %s", s.ID, res.Matches[0].Scar.ID)
	}
}

func TestParseFailureLogMultiLanguage(t *testing.T) {
	// 1. Python Traceback
	pyLog := `Traceback (most recent call last):
  File "app/server.py", line 45, in handle_request
    db.connect()
  File "pkg/db.py", line 12, in connect
ConnectionRefusedError: Unable to connect to host 127.0.0.1:5432`

	pyCtx := ParseFailureLog(pyLog)
	if pyCtx.Language != "python" {
		t.Errorf("expected python language, got %s", pyCtx.Language)
	}
	if len(pyCtx.Files) < 2 || pyCtx.Files[0] != "app/server.py" {
		t.Errorf("expected python files extracted, got %v", pyCtx.Files)
	}
	if len(pyCtx.Symbols) == 0 || pyCtx.Symbols[0] != "handle_request" {
		t.Errorf("expected symbol handle_request, got %v", pyCtx.Symbols)
	}

	// 2. TypeScript / Node.js Error
	tsLog := `TypeError: Cannot read properties of undefined (reading 'token')
    at Authenticator.verify (src/auth/service.ts:89:14)
    at handleAuth (src/api/routes.ts:32:10)`

	tsCtx := ParseFailureLog(tsLog)
	if tsCtx.Language != "javascript" {
		t.Errorf("expected javascript language, got %s", tsCtx.Language)
	}
	if len(tsCtx.Files) < 2 || tsCtx.Files[0] != "src/auth/service.ts" {
		t.Errorf("expected ts files extracted, got %v", tsCtx.Files)
	}
	if len(tsCtx.Symbols) == 0 || tsCtx.Symbols[0] != "Authenticator.verify" {
		t.Errorf("expected symbol Authenticator.verify, got %v", tsCtx.Symbols)
	}

	// 3. Rust panic
	rustLog := `thread 'main' panicked at 'assertion failed: lock.is_locked()', src/sync/mutex.rs:114:9
stack backtrace:
   0: rust_begin_unwind`

	rsCtx := ParseFailureLog(rustLog)
	if rsCtx.Language != "rust" {
		t.Errorf("expected rust language, got %s", rsCtx.Language)
	}
	if len(rsCtx.Files) == 0 || rsCtx.Files[0] != "src/sync/mutex.rs" {
		t.Errorf("expected rust file src/sync/mutex.rs, got %v", rsCtx.Files)
	}
}
