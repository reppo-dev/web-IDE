package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
)

// JSONRPCRequest represents a standard MCP request.
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// JSONRPCResponse represents a standard MCP response.
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents standard RPC error.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// StdioProcessClient manages an active OS process communicating via stdio JSON-RPC.
type StdioProcessClient struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    *bufio.Scanner
	reqID     int64
	pending   map[int64]chan JSONRPCResponse
	pendingMu sync.Mutex
	isClosed  bool
}

// StartProcess launches an external MCP server command and handles JSON-RPC stdio transport.
func StartProcess(ctx context.Context, command string, args []string, env map[string]string) (*StdioProcessClient, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	for k, v := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to open stdin pipe: %w", err)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("failed to open stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdoutPipe.Close()
		return nil, fmt.Errorf("failed to start mcp process %s: %w", command, err)
	}

	client := &StdioProcessClient{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  bufio.NewScanner(stdoutPipe),
		pending: make(map[int64]chan JSONRPCResponse),
	}

	go client.readLoop()

	return client, nil
}

func (c *StdioProcessClient) readLoop() {
	for c.stdout.Scan() {
		line := c.stdout.Bytes()
		var resp JSONRPCResponse
		if err := json.Unmarshal(line, &resp); err == nil && resp.ID > 0 {
			c.pendingMu.Lock()
			ch, exists := c.pending[resp.ID]
			if exists {
				delete(c.pending, resp.ID)
				ch <- resp
				close(ch)
			}
			c.pendingMu.Unlock()
		}
	}
}

// Call sends a JSON-RPC method request and waits for response.
func (c *StdioProcessClient) Call(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	id := atomic.AddInt64(&c.reqID, 1)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	bytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	respChan := make(chan JSONRPCResponse, 1)
	c.pendingMu.Lock()
	c.pending[id] = respChan
	c.pendingMu.Unlock()

	c.pendingMu.Lock()
	_, err = fmt.Fprintf(c.stdin, "%s\n", string(bytes))
	c.pendingMu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("failed to write to process stdin: %w", err)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-respChan:
		if resp.Error != nil {
			return nil, fmt.Errorf("mcp error (%d): %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	}
}

// Close gracefully terminates the process.
func (c *StdioProcessClient) Close() error {
	c.pendingMu.Lock()
	c.isClosed = true
	c.pendingMu.Unlock()

	_ = c.stdin.Close()
	if c.cmd.Process != nil {
		return c.cmd.Process.Kill()
	}
	return nil
}
