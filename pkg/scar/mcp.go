package scar

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// JSONRPCRequest represents an incoming JSON-RPC 2.0 call.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse represents an outgoing JSON-RPC 2.0 response.
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

// RPCError defines a JSON-RPC error payload.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCPToolCallParams defines parameters for tools/call.
type MCPToolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// MCPContent defines tool result content blocks.
type MCPContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// MCPToolResult defines tools/call output schema.
type MCPToolResult struct {
	Content []MCPContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

// ServeMCP runs a Model Context Protocol (MCP) server over stdin/stdout.
// Invariant: This server never executes verifiers and never modifies an
// existing scar. The draft_scar tool writes a new draft record, which is a
// creation rather than a mutation of recorded failure memory.
func (s *Store) ServeMCP(in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	// Allow large buffers for multi-line JSON
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			sendRPCError(out, nil, -32700, "Parse error")
			continue
		}

		handleMCPRequest(s, req, out)
	}

	return scanner.Err()
}

func handleMCPRequest(store *Store, req JSONRPCRequest, out io.Writer) {
	switch req.Method {
	case "initialize":
		res := map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"serverInfo": map[string]string{
				"name":    "git-scars-mcp",
				"version": Version,
			},
			"capabilities": map[string]interface{}{
				"tools":     map[string]interface{}{},
				"resources": map[string]interface{}{},
			},
		}
		sendRPCResult(out, req.ID, res)

	case "notifications/initialized":
		// No response required for notifications

	case "ping":
		sendRPCResult(out, req.ID, map[string]interface{}{})

	case "resources/list":
		summaries, _ := store.ListScars()
		resources := []map[string]interface{}{
			{
				"uri":         "scars://active",
				"name":        "Active Repository Scars Table",
				"description": "Auto-generated markdown table of all active scars and guardrails",
				"mimeType":    "text/markdown",
			},
		}
		for _, s := range summaries {
			resources = append(resources, map[string]interface{}{
				"uri":         fmt.Sprintf("scar://%s", s.Scar.ID),
				"name":        fmt.Sprintf("Scar: %s", s.Scar.Title),
				"description": s.Scar.Lesson,
				"mimeType":    "text/markdown",
			})
		}
		sendRPCResult(out, req.ID, map[string]interface{}{"resources": resources})

	case "resources/read":
		var rParams struct {
			URI string `json:"uri"`
		}
		if err := json.Unmarshal(req.Params, &rParams); err != nil {
			sendRPCError(out, req.ID, -32602, "Invalid params")
			return
		}

		if rParams.URI == "scars://active" {
			block, _ := store.GenerateSyncBlock()
			contents := []map[string]interface{}{
				{
					"uri":      rParams.URI,
					"mimeType": "text/markdown",
					"text":     block,
				},
			}
			sendRPCResult(out, req.ID, map[string]interface{}{"contents": contents})
			return
		}

		if strings.HasPrefix(rParams.URI, "scar://") {
			id := strings.TrimPrefix(rParams.URI, "scar://")
			s, _, err := store.LoadScar(id)
			if err != nil {
				sendRPCError(out, req.ID, -32602, fmt.Sprintf("Resource not found: %v", err))
				return
			}
			data, _ := FormatFrontmatter(s)
			contents := []map[string]interface{}{
				{
					"uri":      rParams.URI,
					"mimeType": "text/markdown",
					"text":     string(data) + "\n\n" + s.Body,
				},
			}
			sendRPCResult(out, req.ID, map[string]interface{}{"contents": contents})
			return
		}

		sendRPCError(out, req.ID, -32602, fmt.Sprintf("Unsupported URI scheme %q", rParams.URI))

	case "tools/list":
		tools := []map[string]interface{}{
			{
				"name":        "list_scars",
				"description": "List all recorded failure scars and their dynamic freshness in the repository",
				"inputSchema": map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
			{
				"name":        "get_scar",
				"description": "Get detailed metadata, lesson text, and anchors for a specific scar by ID",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type":        "string",
							"description": "The unique scar ID (e.g. map-write-crash-391945)",
						},
					},
					"required": []string{"id"},
				},
			},
			{
				"name":        "recall_scars",
				"description": "Deterministically retrieve applicable failure scars anchored to files, symbols, error signatures, or task descriptions",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"files": map[string]interface{}{
							"type":        "array",
							"items":       map[string]string{"type": "string"},
							"description": "Repository-relative file paths currently being modified or inspected",
						},
						"symbols": map[string]interface{}{
							"type":        "array",
							"items":       map[string]string{"type": "string"},
							"description": "Functions, methods, or type names involved in the task",
						},
						"error_signature": map[string]interface{}{
							"type":        "string",
							"description": "Stack trace, compiler error, or failing test output excerpt",
						},
						"task": map[string]interface{}{
							"type":        "string",
							"description": "High-level description of the coding task being performed",
						},
					},
				},
			},
			{
				"name":        "inject_guardrails",
				"description": "Generate an ultra-compact, token-budget-capped prompt guardrail block for specific files",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"files": map[string]interface{}{
							"type":        "array",
							"items":       map[string]string{"type": "string"},
							"description": "Target file paths being inspected or edited",
						},
						"budget_tokens": map[string]interface{}{
							"type":        "integer",
							"description": "Maximum estimated tokens to consume (default: 400)",
						},
						"format": map[string]interface{}{
							"type":        "string",
							"description": "Output format: 'xml' or 'markdown'",
						},
					},
				},
			},
			{
				"name":        "triage_error",
				"description": "Diagnose raw compiler errors, panic stack traces, or test outputs against recorded scars",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"error_output": map[string]interface{}{
							"type":        "string",
							"description": "Raw compiler error, test failure log, or panic stack trace",
						},
					},
					"required": []string{"error_output"},
				},
			},
			{
				"name":        "draft_scar",
				"description": "Autonomously record a new failure memory draft into repository Git memory",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"title": map[string]interface{}{
							"type":        "string",
							"description": "Descriptive title of the failure or regression",
						},
						"lesson": map[string]interface{}{
							"type":        "string",
							"description": "Actionable prevention rule or architectural guidance",
						},
						"files": map[string]interface{}{
							"type":        "array",
							"items":       map[string]string{"type": "string"},
							"description": "Repository-relative file paths involved in the failure",
						},
						"verifier_argv": map[string]interface{}{
							"type":        "array",
							"items":       map[string]string{"type": "string"},
							"description": "Command arguments to verify fix (e.g. ['go', 'test', '-race', './...'])",
						},
					},
					"required": []string{"title", "lesson"},
				},
			},
			{
				"name":        "pre_edit_check",
				"description": "Pre-edit blocking guardrail check. Inspects target files and symbols before code modifications and returns blocking scars with actionable do/don't guidance",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"files": map[string]interface{}{
							"type":        "array",
							"items":       map[string]string{"type": "string"},
							"description": "Target file paths being inspected or edited",
						},
						"symbols": map[string]interface{}{
							"type":        "array",
							"items":       map[string]string{"type": "string"},
							"description": "Functions or symbols involved in the edit",
						},
						"fail_on": map[string]interface{}{
							"type":        "string",
							"description": "Severity threshold to block on: 'critical', 'high' (default), 'medium', 'low'",
						},
					},
					"required": []string{"files"},
				},
			},
			{
				"name":        "plan_task",
				"description": "Composite task planning tool. Combines selective recall and token-budget prompt injection in a single roundtrip",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"task": map[string]interface{}{
							"type":        "string",
							"description": "High-level description of the engineering task or planned changes",
						},
						"files": map[string]interface{}{
							"type":        "array",
							"items":       map[string]string{"type": "string"},
							"description": "Target repository files to inspect",
						},
						"symbols": map[string]interface{}{
							"type":        "array",
							"items":       map[string]string{"type": "string"},
							"description": "Target symbols or methods",
						},
						"budget_tokens": map[string]interface{}{
							"type":        "integer",
							"description": "Maximum token budget for prompt injection (default: 400)",
						},
					},
					"required": []string{"task"},
				},
			},
			{
				"name":        "post_failure_learn",
				"description": "Composite failure diagnosis and learning tool. Triages error logs against known failure memories and returns prevention guidance",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"error_output": map[string]interface{}{
							"type":        "string",
							"description": "Compiler error, panic stack trace, or failing test output",
						},
						"files": map[string]interface{}{
							"type":        "array",
							"items":       map[string]string{"type": "string"},
							"description": "Optional repository files involved in the failure",
						},
					},
					"required": []string{"error_output"},
				},
			},
		}
		sendRPCResult(out, req.ID, map[string]interface{}{"tools": tools})

	case "tools/call":
		var params MCPToolCallParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			sendRPCError(out, req.ID, -32602, "Invalid params")
			return
		}

		result := executeMCPTool(store, params)
		sendRPCResult(out, req.ID, result)

	default:
		sendRPCError(out, req.ID, -32601, fmt.Sprintf("Method %q not found", req.Method))
	}
}

func executeMCPTool(store *Store, params MCPToolCallParams) MCPToolResult {
	switch params.Name {
	case "list_scars":
		summaries, err := store.ListScars()
		if err != nil {
			return MCPToolResult{
				IsError: true,
				Content: []MCPContent{{Type: "text", Text: fmt.Sprintf("Error listing scars: %v", err)}},
			}
		}

		type ScarItem struct {
			ID        string `json:"id"`
			Title     string `json:"title"`
			State     string `json:"state"`
			Review    string `json:"review"`
			Verifier  string `json:"verifier"`
			Freshness string `json:"freshness"`
		}
		var items []ScarItem
		for _, s := range summaries {
			items = append(items, ScarItem{
				ID:        s.Scar.ID,
				Title:     s.Scar.Title,
				State:     s.Scar.Lifecycle.State,
				Review:    s.Scar.Review.Status,
				Verifier:  s.Scar.Verifier.Result,
				Freshness: s.Freshness,
			})
		}
		data, _ := json.MarshalIndent(items, "", "  ")
		return MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: string(data)}},
		}

	case "get_scar":
		idVal, ok := params.Arguments["id"].(string)
		if !ok || idVal == "" {
			return MCPToolResult{
				IsError: true,
				Content: []MCPContent{{Type: "text", Text: "Missing required argument 'id'"}},
			}
		}

		s, path, err := store.LoadScar(idVal)
		if err != nil {
			return MCPToolResult{
				IsError: true,
				Content: []MCPContent{{Type: "text", Text: fmt.Sprintf("Scar not found: %v", err)}},
			}
		}

		freshness := store.ComputeFreshness(s)
		info := map[string]interface{}{
			"id":        s.ID,
			"path":      path,
			"title":     s.Title,
			"lesson":    s.Lesson,
			"state":     s.Lifecycle.State,
			"review":    s.Review,
			"verifier":  s.Verifier,
			"freshness": freshness,
			"anchors":   s.Anchors,
		}
		data, _ := json.MarshalIndent(info, "", "  ")
		return MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: string(data)}},
		}

	case "recall_scars":
		opts := QueryOptions{
			Threshold: 3,
			TopK:      3,
		}

		if filesVal, ok := params.Arguments["files"].([]interface{}); ok {
			for _, f := range filesVal {
				if str, ok := f.(string); ok && str != "" {
					opts.Files = append(opts.Files, str)
				}
			}
		}
		if symVal, ok := params.Arguments["symbols"].([]interface{}); ok {
			for _, s := range symVal {
				if str, ok := s.(string); ok && str != "" {
					opts.Symbols = append(opts.Symbols, str)
				}
			}
		}
		if errSig, ok := params.Arguments["error_signature"].(string); ok {
			opts.ErrorSignature = errSig
		}
		if task, ok := params.Arguments["task"].(string); ok {
			opts.Task = task
		}

		matches, err := store.Recall(opts)
		if err != nil {
			return MCPToolResult{
				IsError: true,
				Content: []MCPContent{{Type: "text", Text: fmt.Sprintf("Recall failed: %v", err)}},
			}
		}

		jsonData, _ := FormatRecallJSON(opts, matches)
		return MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: string(jsonData)}},
		}

	case "inject_guardrails":
		var files []string
		if filesVal, ok := params.Arguments["files"].([]interface{}); ok {
			for _, f := range filesVal {
				if str, ok := f.(string); ok && str != "" {
					files = append(files, str)
				}
			}
		}
		budget := 400
		if bVal, ok := params.Arguments["budget_tokens"].(float64); ok && bVal > 0 {
			budget = int(bVal)
		}
		format := "xml"
		if fVal, ok := params.Arguments["format"].(string); ok && fVal != "" {
			format = fVal
		}

		snippet, err := store.InjectGuardrails(InjectOptions{
			Files:        files,
			BudgetTokens: budget,
			Format:       format,
		})
		if err != nil {
			return MCPToolResult{
				IsError: true,
				Content: []MCPContent{{Type: "text", Text: fmt.Sprintf("Inject failed: %v", err)}},
			}
		}
		return MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: snippet}},
		}

	case "triage_error":
		errOut, ok := params.Arguments["error_output"].(string)
		if !ok || errOut == "" {
			return MCPToolResult{
				IsError: true,
				Content: []MCPContent{{Type: "text", Text: "Missing required argument 'error_output'"}},
			}
		}

		res, err := store.TriageError(errOut)
		if err != nil {
			return MCPToolResult{
				IsError: true,
				Content: []MCPContent{{Type: "text", Text: fmt.Sprintf("Triage failed: %v", err)}},
			}
		}

		data, _ := json.MarshalIndent(res, "", "  ")
		return MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: string(data)}},
		}

	case "draft_scar":
		title, ok := params.Arguments["title"].(string)
		if !ok || title == "" {
			return MCPToolResult{
				IsError: true,
				Content: []MCPContent{{Type: "text", Text: "Missing required argument 'title'"}},
			}
		}
		lesson, ok := params.Arguments["lesson"].(string)
		if !ok || lesson == "" {
			return MCPToolResult{
				IsError: true,
				Content: []MCPContent{{Type: "text", Text: "Missing required argument 'lesson'"}},
			}
		}

		var files []string
		if filesVal, ok := params.Arguments["files"].([]interface{}); ok {
			for _, f := range filesVal {
				if str, ok := f.(string); ok && str != "" {
					files = append(files, str)
				}
			}
		}

		draft, path, err := store.CreateDraft(title, lesson, files)
		if err != nil {
			return MCPToolResult{
				IsError: true,
				Content: []MCPContent{{Type: "text", Text: fmt.Sprintf("Failed creating draft: %v", err)}},
			}
		}

		if verifierVal, ok := params.Arguments["verifier_argv"].([]interface{}); ok {
			for _, v := range verifierVal {
				if str, ok := v.(string); ok && str != "" {
					draft.Verifier.Argv = append(draft.Verifier.Argv, str)
				}
			}
			_, _ = store.SaveScar(draft)
		}

		res := map[string]string{
			"status": "draft_created",
			"id":     draft.ID,
			"path":   path,
		}
		data, _ := json.MarshalIndent(res, "", "  ")
		return MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: string(data)}},
		}

	case "pre_edit_check":
		var files []string
		if rawFiles, ok := params.Arguments["files"].([]interface{}); ok {
			for _, rf := range rawFiles {
				if fStr, ok := rf.(string); ok && strings.TrimSpace(fStr) != "" {
					files = append(files, strings.TrimSpace(fStr))
				}
			}
		}
		var symbols []string
		if rawSyms, ok := params.Arguments["symbols"].([]interface{}); ok {
			for _, rs := range rawSyms {
				if sStr, ok := rs.(string); ok && strings.TrimSpace(sStr) != "" {
					symbols = append(symbols, strings.TrimSpace(sStr))
				}
			}
		}
		failOn, _ := params.Arguments["fail_on"].(string)
		res, err := store.Enforce(EnforceOptions{
			Files:   files,
			Symbols: symbols,
			FailOn:  failOn,
		})
		if err != nil {
			return MCPToolResult{
				IsError: true,
				Content: []MCPContent{{Type: "text", Text: fmt.Sprintf("Enforce check failed: %v", err)}},
			}
		}
		data, _ := json.MarshalIndent(res, "", "  ")
		return MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: string(data)}},
		}

	case "plan_task":
		task, _ := params.Arguments["task"].(string)
		var files []string
		if rawFiles, ok := params.Arguments["files"].([]interface{}); ok {
			for _, rf := range rawFiles {
				if fStr, ok := rf.(string); ok && strings.TrimSpace(fStr) != "" {
					files = append(files, strings.TrimSpace(fStr))
				}
			}
		}
		var symbols []string
		if rawSyms, ok := params.Arguments["symbols"].([]interface{}); ok {
			for _, rs := range rawSyms {
				if sStr, ok := rs.(string); ok && strings.TrimSpace(sStr) != "" {
					symbols = append(symbols, strings.TrimSpace(sStr))
				}
			}
		}
		budget := 400
		if rawBudget, ok := params.Arguments["budget_tokens"].(float64); ok && rawBudget > 0 {
			budget = int(rawBudget)
		}

		guardrails, err := store.InjectGuardrails(InjectOptions{
			Files:        files,
			Symbols:      symbols,
			Task:         task,
			BudgetTokens: budget,
			Format:       "xml",
		})
		if err != nil {
			return MCPToolResult{
				IsError: true,
				Content: []MCPContent{{Type: "text", Text: fmt.Sprintf("Guardrail injection failed: %v", err)}},
			}
		}

		recalled, _ := store.Recall(QueryOptions{
			Files:     files,
			Symbols:   symbols,
			Task:      task,
			Threshold: 2,
			TopK:      10,
		})

		planRes := map[string]interface{}{
			"guardrails_xml": guardrails,
			"matches_count":  len(recalled),
			"matches":        recalled,
		}
		data, _ := json.MarshalIndent(planRes, "", "  ")
		return MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: string(data)}},
		}

	case "post_failure_learn":
		errorOutput, _ := params.Arguments["error_output"].(string)
		var files []string
		if rawFiles, ok := params.Arguments["files"].([]interface{}); ok {
			for _, rf := range rawFiles {
				if fStr, ok := rf.(string); ok && strings.TrimSpace(fStr) != "" {
					files = append(files, strings.TrimSpace(fStr))
				}
			}
		}
		triageRes, err := store.TriageErrorWithFiles(errorOutput, files)
		if err != nil {
			return MCPToolResult{
				IsError: true,
				Content: []MCPContent{{Type: "text", Text: fmt.Sprintf("Triage error failed: %v", err)}},
			}
		}

		data, _ := json.MarshalIndent(triageRes, "", "  ")
		return MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: string(data)}},
		}

	default:
		return MCPToolResult{
			IsError: true,
			Content: []MCPContent{{Type: "text", Text: fmt.Sprintf("Unknown tool %q", params.Name)}},
		}
	}
}

func sendRPCResult(out io.Writer, id interface{}, result interface{}) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	data, _ := json.Marshal(resp)
	data = append(data, '\n')
	_, _ = out.Write(data)
}

func sendRPCError(out io.Writer, id interface{}, code int, message string) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
		},
	}
	data, _ := json.Marshal(resp)
	data = append(data, '\n')
	_, _ = out.Write(data)
}
