package scar

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestSARIFDriverUsesPackageVersion(t *testing.T) {
	data, err := FormatEnforceSARIF(&EnforceResult{})
	if err != nil {
		t.Fatalf("unexpected SARIF error: %v", err)
	}
	var report SARIFReport
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("failed decoding SARIF output: %v", err)
	}
	if len(report.Runs) == 0 {
		t.Fatal("expected at least one SARIF run")
	}
	if got := report.Runs[0].Tool.Driver.Version; got != Version {
		t.Errorf("SARIF driver version %q does not match package version %q", got, Version)
	}
}

func TestMCPServerInfoUsesPackageVersion(t *testing.T) {
	store := NewStore(t.TempDir())
	var out bytes.Buffer
	handleMCPRequest(store, JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
	}, &out)

	var resp struct {
		Result struct {
			ServerInfo struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"serverInfo"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("failed decoding MCP response: %v", err)
	}
	if got := resp.Result.ServerInfo.Version; got != Version {
		t.Errorf("MCP server version %q does not match package version %q", got, Version)
	}
}
