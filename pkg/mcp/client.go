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

	"github.com/sdt-project/sdt/pkg/log"
)

const protocolVersion = "2024-11-05"

type jsonRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      *int64      `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type initializeParams struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities"`
	ClientInfo      clientInfo     `json:"clientInfo"`
}

type clientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type initializeResult struct {
	ProtocolVersion string     `json:"protocolVersion"`
	ServerInfo      serverInfo `json:"serverInfo"`
}

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// MCPTool represents a tool discovered from an MCP server.
type MCPTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type toolsListResult struct {
	Tools []MCPTool `json:"tools"`
}

type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type toolCallResult struct {
	Content []toolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type toolContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// Client connects to an MCP server over stdio.
type Client struct {
	name    string
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	scanner *bufio.Scanner
	mu      sync.Mutex
	nextID  atomic.Int64
}

// MCPServerConfig describes how to launch an MCP server.
type MCPServerConfig struct {
	Command string            `yaml:"command"`
	Args    []string          `yaml:"args,omitempty"`
	Env     map[string]string `yaml:"env,omitempty"`
}

// NewClient starts an MCP server subprocess and performs the initialize handshake.
func NewClient(ctx context.Context, name string, cfg MCPServerConfig) (*Client, error) {
	cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)

	for k, v := range cfg.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting MCP server %q (%s): %w", name, cfg.Command, err)
	}

	c := &Client{
		name:    name,
		cmd:     cmd,
		stdin:   stdin,
		scanner: bufio.NewScanner(stdout),
	}
	c.scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	if err := c.initialize(); err != nil {
		c.Close()
		return nil, fmt.Errorf("MCP initialize handshake with %q failed: %w", name, err)
	}

	log.Infof("MCP", "Connected to server %q (%s)", name, cfg.Command)
	return c, nil
}

func (c *Client) initialize() error {
	result, err := c.call("initialize", initializeParams{
		ProtocolVersion: protocolVersion,
		Capabilities:    map[string]any{},
		ClientInfo:      clientInfo{Name: "sdt", Version: "1.0.0"},
	})
	if err != nil {
		return err
	}

	var initResult initializeResult
	if err := json.Unmarshal(result, &initResult); err != nil {
		return fmt.Errorf("parsing initialize result: %w", err)
	}

	log.Debugf("MCP", "Server %q: %s v%s (protocol %s)",
		c.name, initResult.ServerInfo.Name, initResult.ServerInfo.Version, initResult.ProtocolVersion)

	return c.notify("notifications/initialized", nil)
}

// ListTools returns all tools exposed by the MCP server.
func (c *Client) ListTools() ([]MCPTool, error) {
	result, err := c.call("tools/list", map[string]any{})
	if err != nil {
		return nil, err
	}
	var listResult toolsListResult
	if err := json.Unmarshal(result, &listResult); err != nil {
		return nil, fmt.Errorf("parsing tools/list result: %w", err)
	}
	return listResult.Tools, nil
}

// CallTool invokes a tool on the MCP server and returns the text output.
func (c *Client) CallTool(name string, arguments json.RawMessage) (string, error) {
	result, err := c.call("tools/call", toolCallParams{
		Name:      name,
		Arguments: arguments,
	})
	if err != nil {
		return "", err
	}

	var callResult toolCallResult
	if err := json.Unmarshal(result, &callResult); err != nil {
		return "", fmt.Errorf("parsing tools/call result: %w", err)
	}

	var output string
	for _, content := range callResult.Content {
		if content.Type == "text" {
			if output != "" {
				output += "\n"
			}
			output += content.Text
		}
	}

	if callResult.IsError {
		return "", fmt.Errorf("tool %q error: %s", name, output)
	}

	return output, nil
}

// Close shuts down the MCP server subprocess.
func (c *Client) Close() error {
	c.stdin.Close()
	return c.cmd.Wait()
}

func (c *Client) call(method string, params interface{}) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := c.nextID.Add(1)
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	if _, err := fmt.Fprintf(c.stdin, "%s\n", data); err != nil {
		return nil, fmt.Errorf("writing to MCP server: %w", err)
	}

	for c.scanner.Scan() {
		line := c.scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var resp jsonRPCResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			continue
		}

		// Skip notifications (no id)
		if resp.ID == nil {
			continue
		}

		if *resp.ID != id {
			continue
		}

		if resp.Error != nil {
			return nil, fmt.Errorf("MCP error %d: %s", resp.Error.Code, resp.Error.Message)
		}

		return resp.Result, nil
	}

	if err := c.scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading from MCP server: %w", err)
	}
	return nil, fmt.Errorf("MCP server closed connection")
}

func (c *Client) notify(method string, params interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	req := jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshaling notification: %w", err)
	}

	if _, err := fmt.Fprintf(c.stdin, "%s\n", data); err != nil {
		return fmt.Errorf("writing notification to MCP server: %w", err)
	}

	return nil
}
