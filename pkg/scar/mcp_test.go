package scar

import (
	"bytes"
	"encoding/json"
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
	s.Verifier.Result = VerificationFailToPass
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
	if len(tools) != 6 {
		t.Errorf("expected 6 tools in tools/list, got %d", len(tools))
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
