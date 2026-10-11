package scar

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCommandHashComputation(t *testing.T) {
	h1 := ComputeCommandHash([]string{"go", "test", "./..."}, "pkg", []string{"FOO"})
	h2 := ComputeCommandHash([]string{"go", "test", "./..."}, "pkg", []string{"FOO"})
	if h1 != h2 {
		t.Errorf("expected deterministic command hash, got %s != %s", h1, h2)
	}

	// Any change invalidates the hash
	h3 := ComputeCommandHash([]string{"go", "test", "-v", "./..."}, "pkg", []string{"FOO"})
	if h1 == h3 {
		t.Error("expected different hash after modifying argv")
	}
}

func TestTrustListOperations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gitscars-trust-test-*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewStore(tmpDir)
	_ = store.Init()

	cmdHash := "hash-abc-12345"
	if store.IsCommandTrusted(cmdHash) {
		t.Error("expected command to be untrusted initially")
	}

	if err := store.AddTrustEntry(cmdHash); err != nil {
		t.Fatalf("failed adding trust entry: %v", err)
	}

	if !store.IsCommandTrusted(cmdHash) {
		t.Error("expected command to be trusted after addition")
	}

	// Verify file was written to untracked .local/
	trustFilePath := filepath.Join(store.LocalDir, TrustedVerifiersFile)
	if _, err := os.Stat(trustFilePath); os.IsNotExist(err) {
		t.Errorf("trusted-verifiers.json not created at %s", trustFilePath)
	}
}

func TestRedactOutput(t *testing.T) {
	input := `Error: connection failed with Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.doNotLeakThis
-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEA0Y1
secretKeyDataHere
-----END RSA PRIVATE KEY-----
password = supersecret123`

	redacted := RedactOutput(input)

	if strings.Contains(redacted, "doNotLeakThis") {
		t.Error("failed redacting Bearer token")
	}
	if strings.Contains(redacted, "secretKeyDataHere") {
		t.Error("failed redacting private key block")
	}
	if strings.Contains(redacted, "supersecret123") {
		t.Error("failed redacting password")
	}
	if !strings.Contains(redacted, "[REDACTED_TOKEN]") {
		t.Error("missing [REDACTED_TOKEN] replacement")
	}
	if !strings.Contains(redacted, "[REDACTED_PRIVATE_KEY]") {
		t.Error("missing [REDACTED_PRIVATE_KEY] replacement")
	}
}

func TestBuildScrubbedEnv(t *testing.T) {
	t.Setenv("SECRET_API_KEY", "super-secret-value")
	t.Setenv("ALLOWLISTED_VAR", "allowed-value")

	scrubbed := BuildScrubbedEnv([]string{"ALLOWLISTED_VAR"})

	hasSecret := false
	hasAllowed := false
	hasPath := false

	for _, item := range scrubbed {
		if strings.HasPrefix(item, "SECRET_API_KEY=") {
			hasSecret = true
		}
		if strings.HasPrefix(item, "ALLOWLISTED_VAR=") {
			hasAllowed = true
		}
		if strings.HasPrefix(item, "PATH=") {
			hasPath = true
		}
	}

	if hasSecret {
		t.Error("scrubbed env leaked SECRET_API_KEY")
	}
	if !hasAllowed {
		t.Error("scrubbed env omitted allowlisted var")
	}
	if !hasPath {
		t.Error("scrubbed env omitted PATH")
	}
}

func TestExecuteVerifierExecution(t *testing.T) {
	ctx := context.Background()
	tmpDir, _ := os.MkdirTemp("", "gitscars-exec-test-*")
	defer os.RemoveAll(tmpDir)

	// Simple echo check
	res, err := ExecuteVerifier(ctx, tmpDir, []string{"echo", "hello world"}, "", nil, 5*time.Second)
	if err != nil {
		t.Fatalf("failed executing verifier: %v", err)
	}
	if res.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", res.ExitCode)
	}
	if !strings.Contains(res.OutputExcerpt, "hello world") {
		t.Errorf("expected output excerpt to contain 'hello world', got: %s", res.OutputExcerpt)
	}
	if res.OutputHash == "" {
		t.Error("expected non-empty output hash")
	}

	// Timeout test
	timeOutRes, _ := ExecuteVerifier(ctx, tmpDir, []string{"sleep", "2"}, "", nil, 100*time.Millisecond)
	if !timeOutRes.TimedOut {
		t.Error("expected verifier to time out")
	}
}

func TestVerifyDoesNotImplicitlyActivate(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "gitscars-verify-invariant-*")
	defer os.RemoveAll(tmpDir)

	store := NewStore(tmpDir)
	_ = store.Init()

	draft, _, err := store.CreateDraft("Test Scar", "Lesson", []string{"foo.go"})
	if err != nil {
		t.Fatalf("create draft failed: %v", err)
	}

	draft.Verifier.Argv = []string{"echo", "pass"}
	cmdHash := ComputeCommandHash(draft.Verifier.Argv, "", nil)
	_ = store.AddTrustEntry(cmdHash)

	outcome, err := store.VerifyFailToPass(context.Background(), draft, nil, true)
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	if outcome != VerificationException {
		t.Errorf("expected exception due to missing base commit, got %s", outcome)
	}

	// Critical invariant: verifier execution MUST NOT promote or approve the scar
	if draft.Lifecycle.State != LifecycleDraft {
		t.Errorf("expected scar to remain draft, got %s", draft.Lifecycle.State)
	}
	if draft.Review.Status != ReviewUnreviewed {
		t.Errorf("expected scar to remain unreviewed, got %s", draft.Review.Status)
	}
}
