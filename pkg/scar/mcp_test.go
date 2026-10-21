package scar

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMCPServerHandshakeAndTools(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "gitscars-mcp-test-*")
	defer os.RemoveAll(tmpDir)

	store := NewStore(tmpDir)
	_ = store.Init()

	// Seed an active scar
	s, _, _ := store.CreateDraft("DNS Resolution Hang", "Check resolver timeout", []string{"pkg/net.go"})
	s.Lifecycle.State = LifecycleActive
	s.Review.Status = ReviewApproved
	s.Verifier = failToPassVerifier("go", "test", "-race", "./...")
	_ = os.MkdirAll(filepath.Join(tmpDir, "pkg"), 0755)
	_ = os.WriteFile(filepath.Join(tmpDir, "pkg/net.go"), []byte("package net\n"), 0644)
	_ = store.RecordBaseline(s, "hash-net")
	_, _ = store.SaveScar(s)

	// Simulate client requests
	req1 := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n"
	req2 := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}` + "\n"
	req3 := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"recall_scars","arguments":{"files":["pkg/net.go"]}}}` + "\n"
	req4 := `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"list_scars","arguments":{}}}` + "\n"

	in := bytes.NewBufferString(req1 + req2 + req3 + req4)
	out := new(bytes.Buffer)

	if err := store.ServeMCP(in, out); err != nil {
		t.Fatalf("ServeMCP error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) < 4 {
		t.Fatalf("expected at least 4 responses, got %d: %s", len(lines), out.String())
	}

	// Response 1: initialize
	var resp1 JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[0]), &resp1); err != nil {
		t.Fatalf("failed unmarshaling resp1: %v", err)
	}
	if resp1.Error != nil {
		t.Errorf("resp1 returned error: %v", resp1.Error)
	}

	// Response 2: tools/list
	var resp2 JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[1]), &resp2); err != nil {
		t.Fatalf("failed unmarshaling resp2: %v", err)
	}
	toolsMap := resp2.Result.(map[string]interface{})
	tools := toolsMap["tools"].([]interface{})
	if len(tools) != 9 {
		t.Errorf("expected 9 tools in tools/list, got %d", len(tools))
	}

	// Response 3: recall_scars
	var resp3 JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[2]), &resp3); err != nil {
		t.Fatalf("failed unmarshaling resp3: %v", err)
	}
	callRes := resp3.Result.(map[string]interface{})
	contents := callRes["content"].([]interface{})
	text := contents[0].(map[string]interface{})["text"].(string)
	if !strings.Contains(text, "DNS Resolution Hang") {
		t.Errorf("expected recall result to contain scar title, got: %s", text)
	}
}

func TestMCPServerCompositeTools(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "gitscars-mcp-composite-*")
	defer os.RemoveAll(tmpDir)

	store := NewStore(tmpDir)
	_ = store.Init()

	// Seed an active scar with severity high
	s, _, _ := store.CreateDraft("Worker Deadlock", "Buffer worker channel", []string{"pkg/worker.go"})
	s.Lifecycle.State = LifecycleActive
	s.Review.Status = ReviewApproved
	s.Verifier = failToPassVerifier("go", "test", "-race", "./...")
	s.Severity = "high"
	s.FixHint = "Buffer task channel"
	s.BadPattern = "make(chan int)"
	s.GoodPattern = "make(chan int, 10)"
	_ = os.MkdirAll(filepath.Join(tmpDir, "pkg"), 0755)
	_ = os.WriteFile(filepath.Join(tmpDir, "pkg/worker.go"), []byte("package worker\n"), 0644)
	_ = store.RecordBaseline(s, "hash-worker")
	_, _ = store.SaveScar(s)

	// Test 1: pre_edit_check
	reqPreEdit := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"pre_edit_check","arguments":{"files":["pkg/worker.go"],"fail_on":"high"}}}` + "\n"
	// Test 2: plan_task
	reqPlan := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"plan_task","arguments":{"task":"refactor worker channel","files":["pkg/worker.go"]}}}` + "\n"
	// Test 3: post_failure_learn
	reqLearn := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"post_failure_learn","arguments":{"error_output":"fatal error: all goroutines asleep - deadlock!\npkg/worker.go:10 +0x25\n"}}}` + "\n"

	in := bytes.NewBufferString(reqPreEdit + reqPlan + reqLearn)
	out := new(bytes.Buffer)

	if err := store.ServeMCP(in, out); err != nil {
		t.Fatalf("ServeMCP error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected 3 responses, got %d: %s", len(lines), out.String())
	}

	// 1. Verify pre_edit_check response
	var respPreEdit JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[0]), &respPreEdit); err != nil {
		t.Fatalf("failed unmarshaling pre_edit_check resp: %v", err)
	}
	preEditMap := respPreEdit.Result.(map[string]interface{})
	preEditContent := preEditMap["content"].([]interface{})
	preEditText := preEditContent[0].(map[string]interface{})["text"].(string)
	if !strings.Contains(preEditText, `"blocked": true`) && !strings.Contains(preEditText, `"blocked":true`) {
		t.Errorf("expected pre_edit_check to be blocked, got: %s", preEditText)
	}

	// 2. Verify plan_task response
	var respPlan JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[1]), &respPlan); err != nil {
		t.Fatalf("failed unmarshaling plan_task resp: %v", err)
	}
	planMap := respPlan.Result.(map[string]interface{})
	planContent := planMap["content"].([]interface{})
	planText := planContent[0].(map[string]interface{})["text"].(string)
	if !strings.Contains(planText, "Worker Deadlock") {
		t.Errorf("expected plan_task to contain scar, got: %s", planText)
	}

	// 3. Verify post_failure_learn response
	var respLearn JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[2]), &respLearn); err != nil {
		t.Fatalf("failed unmarshaling post_failure_learn resp: %v", err)
	}
	learnMap := respLearn.Result.(map[string]interface{})
	learnContent := learnMap["content"].([]interface{})
	learnText := learnContent[0].(map[string]interface{})["text"].(string)
	if !strings.Contains(learnText, "Worker Deadlock") {
		t.Errorf("expected post_failure_learn to triage deadlock, got: %s", learnText)
	}
}

func TestMCPServerTwoWayToolsAndResources(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "gitscars-mcp-twoway-*")
	defer os.RemoveAll(tmpDir)

	store := NewStore(tmpDir)
	_ = store.Init()

	// 1. Agent drafts a scar via MCP
	reqDraft := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"draft_scar","arguments":{"title":"DB Leak","lesson":"Close rows","files":["pkg/db.go"],"verifier_argv":["go","test","./..."]}}}` + "\n"
	// 2. Read resources list
	reqResList := `{"jsonrpc":"2.0","id":2,"method":"resources/list","params":{}}` + "\n"
	// 3. Read active resource
	reqResRead := `{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"scars://active"}}` + "\n"
	// 4. Inject guardrails tool
	reqInject := `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"inject_guardrails","arguments":{"files":["pkg/db.go"],"budget_tokens":300}}}` + "\n"

	in := bytes.NewBufferString(reqDraft + reqResList + reqResRead + reqInject)
	out := new(bytes.Buffer)

	if err := store.ServeMCP(in, out); err != nil {
		t.Fatalf("ServeMCP error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) < 4 {
		t.Fatalf("expected 4 responses, got %d: %s", len(lines), out.String())
	}

	// Verify draft creation
	var respDraft JSONRPCResponse
	_ = json.Unmarshal([]byte(lines[0]), &respDraft)
	draftResult := respDraft.Result.(map[string]interface{})
	draftContent := draftResult["content"].([]interface{})
	draftText := draftContent[0].(map[string]interface{})["text"].(string)
	if !strings.Contains(draftText, "draft_created") {
		t.Errorf("expected draft_created status, got: %s", draftText)
	}

	// Verify resources list
	var respResList JSONRPCResponse
	_ = json.Unmarshal([]byte(lines[1]), &respResList)
	resMap := respResList.Result.(map[string]interface{})
	resources := resMap["resources"].([]interface{})
	if len(resources) < 2 {
		t.Errorf("expected at least 2 resources, got %d", len(resources))
	}
}

// TestMCPServerCoversCommandFileSymbolAndSeverityArguments exercises the
// arguments the composite surfaces gained: command anchors in recall and in
// planning, file scoping in triage, symbol and task scoping in injection, and
// symbol and severity fields on a drafted scar.
func TestMCPServerCoversCommandFileSymbolAndSeverityArguments(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "gitscars-mcp-coverage-*")
	defer os.RemoveAll(tmpDir)

	store := NewStore(tmpDir)
	_ = store.Init()

	// Scar A: anchored by the command that the task will run.
	a, _, _ := store.CreateDraft("Race In Exporter", "Protect the metrics map", []string{"pkg/telemetry/exporter.go"})
	a.Lifecycle.State = LifecycleActive
	a.Review.Status = ReviewApproved
	a.Severity = "high"
	a.Anchors.Commands = []string{"go test -race ./pkg/telemetry/..."}
	a.Anchors.ErrorSignatures = []string{"fatal error: concurrent map writes"}
	a.Verifier = failToPassVerifier("go", "test", "-race", "./pkg/telemetry/...")
	_ = os.MkdirAll(filepath.Join(tmpDir, "pkg", "telemetry"), 0755)
	_ = os.WriteFile(filepath.Join(tmpDir, "pkg/telemetry/exporter.go"), []byte("package telemetry\n"), 0644)
	_ = store.RecordBaseline(a, a.Verifier.CommandHash)
	_, _ = store.SaveScar(a)

	// Scar B: anchored by a symbol, for the injection assertions.
	b, _, _ := store.CreateDraft("Symbol Guard", "Guard the record function", []string{"pkg/telemetry/metrics.go"})
	b.Lifecycle.State = LifecycleActive
	b.Review.Status = ReviewApproved
	b.Severity = "medium"
	b.Anchors.Symbols = []string{"RecordMetric"}
	b.Verifier = failToPassVerifier("go", "test", "-race", "./pkg/telemetry/...")
	_ = os.WriteFile(filepath.Join(tmpDir, "pkg/telemetry/metrics.go"), []byte("package telemetry\n\nfunc RecordMetric() {}\n"), 0644)
	_ = store.RecordBaseline(b, b.Verifier.CommandHash)
	_, _ = store.SaveScar(b)

	call := func(name string, arguments string) string {
		t.Helper()
		req := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":%q,"arguments":%s}}`+"\n", name, arguments)
		var out bytes.Buffer
		if err := store.ServeMCP(bytes.NewBufferString(req), &out); err != nil {
			t.Fatalf("ServeMCP error: %v", err)
		}
		lines := strings.Split(strings.TrimSpace(out.String()), "\n")
		if len(lines) == 0 {
			t.Fatalf("expected a response for %s, got none", name)
		}
		var resp JSONRPCResponse
		if err := json.Unmarshal([]byte(lines[0]), &resp); err != nil {
			t.Fatalf("failed unmarshaling response: %v\n%s", err, out.String())
		}
		if resp.Result == nil {
			t.Fatalf("expected a result for %s, got: %v", name, resp.Error)
		}
		content := resp.Result.(map[string]interface{})["content"].([]interface{})
		return content[0].(map[string]interface{})["text"].(string)
	}

	// A command anchor contributes two points, which a task term lifts
	// past the threshold.
	if got := call("recall_scars", `{"commands":["go test -race ./pkg/telemetry/..."],"task":"protect the metrics map"}`); !strings.Contains(got, "Race In Exporter") {
		t.Errorf("expected a command anchored scar to be recalled, got: %s", got)
	}

	if got := call("recall_scars", `{"files":["pkg/telemetry/exporter.go"]}`); !strings.Contains(got, `"count": 1`) && !strings.Contains(got, `"count":1`) {
		t.Errorf("expected file retrieval to honour the configured default threshold, got: %s", got)
	}

	// A configured abstain threshold above every achievable score defers
	// retrieval, which proves the tool defers to configuration instead of the
	// previously hardcoded recall limits.
	if err := os.MkdirAll(filepath.Join(tmpDir, DefaultStoreDir), 0755); err != nil {
		t.Fatalf("failed creating store dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, DefaultStoreDir, ConfigFileName), []byte("schema_version: \"1\"\nabstain_threshold: 9\ntop_k: 3\n"), 0644); err != nil {
		t.Fatalf("failed writing config: %v", err)
	}
	if got := call("recall_scars", `{"files":["pkg/telemetry/exporter.go"]}`); !strings.Contains(got, `"count": 0`) && !strings.Contains(got, `"count":0`) {
		t.Errorf("expected the configured threshold to govern recall, got: %s", got)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, DefaultStoreDir, ConfigFileName), []byte("schema_version: \"1\"\nabstain_threshold: 3\ntop_k: 3\n"), 0644); err != nil {
		t.Fatalf("failed restoring config: %v", err)
	}

	if got := call("triage_error", `{"error_output":"fatal error: concurrent map writes\npkg/telemetry/exporter.go:12\n","files":["pkg/telemetry/exporter.go"]}`); !strings.Contains(got, "Race In Exporter") {
		t.Errorf("expected triage to use the supplied files, got: %s", got)
	}

	if got := call("inject_guardrails", `{"symbols":["RecordMetric"],"task":"touch the record function"}`); !strings.Contains(got, "Symbol Guard") {
		t.Errorf("expected injection to honour the symbol anchor, got: %s", got)
	}

	// Injection is deliberately more sensitive than recall: a task term
	// overlap of two points is enough there, while recall would abstain.
	if got := call("inject_guardrails", `{"task":"protect the metrics map"}`); !strings.Contains(got, "Race In Exporter") {
		t.Errorf("expected injection to honour the task terms, got: %s", got)
	}

	// The recall behind planning carries the command and error signature
	// signals the caller supplied.
	planned := call("plan_task", `{"task":"fix the telemetry race","files":["pkg/telemetry/exporter.go"],"commands":["go test -race ./pkg/telemetry/..."],"error_signature":"fatal error: concurrent map writes"}`)
	var plannedReport struct {
		MatchesCount int `json:"matches_count"`
		Matches      []ScoredScar
	}
	if err := json.Unmarshal([]byte(planned), &plannedReport); err != nil {
		t.Fatalf("failed unmarshaling plan_task output: %v\n%s", err, planned)
	}
	if plannedReport.MatchesCount != 1 || len(plannedReport.Matches) != 1 {
		t.Fatalf("expected one anchored plan candidate, got %d", plannedReport.MatchesCount)
	}
	joined := strings.Join(plannedReport.Matches[0].Matches, ",")
	if !strings.Contains(joined, "file(+5)") || !strings.Contains(joined, "command(+2)") || !strings.Contains(joined, "error_sig(+3)") {
		t.Errorf("expected the plan candidate to carry the file, command, and error signature signals, got %q", joined)
	}

	created := call("draft_scar", `{"title":"Draft With Symbols","lesson":"Anchor symbols as well","severity":"medium","symbols":["DraftRecord"]}`)
	if !strings.Contains(created, `"status": "draft_created"`) && !strings.Contains(created, `"status":"draft_created"`) {
		t.Errorf("expected the enriched draft to be created, got: %s", created)
	}

	var createdRecord struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(created), &createdRecord); err != nil {
		t.Fatalf("failed reading the created draft id: %v", err)
	}
	if got := call("get_scar", `{"id":"`+createdRecord.ID+`"}`); !strings.Contains(got, `"medium"`) || !strings.Contains(got, "DraftRecord") {
		t.Errorf("expected the drafted scar to carry its symbols and severity, got: %s", got)
	}
}
