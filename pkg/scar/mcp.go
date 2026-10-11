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

// ServeMCP runs a read-only Model Context Protocol (MCP) server over stdin/stdout.
// Invariant: This server is strictly read-only and never mutates repository files or runs verifiers.
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
				"version": "0.2.0",
			},
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
		}
		sendRPCResult(out, req.ID, res)

	case "notifications/initialized":
		// No response required for notifications

	case "ping":
		sendRPCResult(out, req.ID, map[string]interface{}{})

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
