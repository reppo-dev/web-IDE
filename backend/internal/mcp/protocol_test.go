package mcp

import (
	"encoding/json"
	"testing"
)

func TestJSONRPCSerialization(t *testing.T) {
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
	}

	bytes, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	var parsed JSONRPCRequest
	if err := json.Unmarshal(bytes, &parsed); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	if parsed.Method != "tools/list" || parsed.ID != 1 {
		t.Errorf("mismatched parsed jsonrpc request: %+v", parsed)
	}
}
