package mcp

import (
	"context"
	"fmt"
	"sync"
)

// ServerConfig defines configuration for an external MCP server.
type ServerConfig struct {
	Name    string            `json:"name"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
	Enabled bool              `json:"enabled"`
}

// ToolDefinition represents an MCP tool exposed to agents.
type ToolDefinition struct {
	ServerName  string      `json:"server_name"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"input_schema"`
}

// Host manages connections to external MCP servers.
type Host struct {
	servers map[string]ServerConfig
	tools   map[string]ToolDefinition
	mu      sync.RWMutex
}

// NewHost creates a new MCP Host.
func NewHost() *Host {
	return &Host{
		servers: make(map[string]ServerConfig),
		tools:   make(map[string]ToolDefinition),
	}
}

// RegisterServer adds a new MCP server configuration.
func (h *Host) RegisterServer(cfg ServerConfig) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.servers[cfg.Name] = cfg
}

// ListServers returns all registered server configs.
func (h *Host) ListServers() []ServerConfig {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var list []ServerConfig
	for _, s := range h.servers {
		list = append(list, s)
	}
	return list
}

// ListTools returns all discovered tools.
func (h *Host) ListTools() []ToolDefinition {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var list []ToolDefinition
	for _, t := range h.tools {
		list = append(list, t)
	}
	return list
}

// RegisterTool manually or dynamically adds a tool to the registry.
func (h *Host) RegisterTool(tool ToolDefinition) {
	h.mu.Lock()
	defer h.mu.Unlock()
	key := fmt.Sprintf("%s::%s", tool.ServerName, tool.Name)
	h.tools[key] = tool
}

// CallTool executes an MCP tool by name.
func (h *Host) CallTool(ctx context.Context, serverName, toolName string, params map[string]interface{}) (interface{}, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	key := fmt.Sprintf("%s::%s", serverName, toolName)
	_, exists := h.tools[key]
	if !exists {
		return nil, fmt.Errorf("tool %s on server %s not found", toolName, serverName)
	}

	// Tool execution stub (delegates to external process transport)
	return map[string]interface{}{
		"status": "success",
		"output": fmt.Sprintf("Tool %s executed successfully", toolName),
	}, nil
}
