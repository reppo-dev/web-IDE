package mcp

import (
	"context"
	"testing"
)

func TestMCPHost(t *testing.T) {
	host := NewHost()

	host.RegisterServer(ServerConfig{
		Name:    "github",
		Command: "npx",
		Args:    []string{"@github/mcp-server"},
		Enabled: true,
	})

	if len(host.ListServers()) != 1 {
		t.Errorf("expected 1 registered server")
	}

	host.RegisterTool(ToolDefinition{
		ServerName:  "github",
		Name:        "create_issue",
		Description: "Create a GitHub issue",
	})

	tools := host.ListTools()
	if len(tools) != 1 {
		t.Errorf("expected 1 registered tool")
	}

	res, err := host.CallTool(context.Background(), "github", "create_issue", map[string]interface{}{"title": "Bug"})
	if err != nil {
		t.Fatalf("call tool failed: %v", err)
	}
	if res == nil {
		t.Errorf("expected response from call tool")
	}
}
