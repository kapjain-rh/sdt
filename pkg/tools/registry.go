package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/openshift/sdt/pkg/log"
)

// Tool represents a single MCP tool that can be called by the LLM agent.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
	Handler     ToolHandler     `json:"-"`
}

// ToolHandler is the function that executes a tool call.
type ToolHandler func(ctx context.Context, input json.RawMessage) (*ToolResult, error)

// ToolResult captures the outcome of a tool execution.
type ToolResult struct {
	Output string // Human-readable output
	Error  error  // Non-nil if the tool call failed
}

// ToolCallRecord tracks a single tool invocation for reporting.
type ToolCallRecord struct {
	ToolName   string
	Input      json.RawMessage
	Output     string
	Error      string
	Duration   time.Duration
}

// Registry holds all registered tools.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]*Tool
}

// NewRegistry creates an empty tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]*Tool),
	}
}

// Register adds a tool to the registry.
func (r *Registry) Register(tool *Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name] = tool
}

// Get returns a tool by name, or nil if not found.
func (r *Registry) Get(name string) *Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tools[name]
}

// Execute runs a tool by name with the given input.
func (r *Registry) Execute(ctx context.Context, name string, input json.RawMessage) (*ToolResult, error) {
	tool := r.Get(name)
	if tool == nil {
		return nil, fmt.Errorf("tool %q not found", name)
	}
	return tool.Handler(ctx, input)
}

// List returns all registered tool names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// ToolDefinitions returns tool definitions in the format expected by the Claude API.
func (r *Registry) ToolDefinitions() []json.RawMessage {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var defs []json.RawMessage
	for _, tool := range r.tools {
		def, err := json.Marshal(map[string]interface{}{
			"name":         tool.Name,
			"description":  tool.Description,
			"input_schema": json.RawMessage(tool.InputSchema),
		})
		if err != nil {
			continue
		}
		defs = append(defs, def)
	}
	return defs
}

// LLMToolDefinitions returns tool definitions as llm.ToolDefinition structs.
func (r *Registry) LLMToolDefinitions() []struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	type toolDef struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		InputSchema json.RawMessage `json:"input_schema"`
	}
	var defs []toolDef
	for _, tool := range r.tools {
		defs = append(defs, toolDef{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		})
	}
	// Return as the anonymous struct type matching the return signature
	result := make([]struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		InputSchema json.RawMessage `json:"input_schema"`
	}, len(defs))
	for i, d := range defs {
		result[i].Name = d.Name
		result[i].Description = d.Description
		result[i].InputSchema = d.InputSchema
	}
	return result
}

// HandleToolCall creates a tool handler function for use with the LLM agent loop.
// It dispatches tool calls to the appropriate registered handler.
func (r *Registry) HandleToolCall(ctx context.Context) func(name string, input json.RawMessage) (string, error) {
	return func(name string, input json.RawMessage) (string, error) {
		log.Debugf("TOOL", "Dispatching tool call: %s", name)
		start := time.Now()
		result, err := r.Execute(ctx, name, input)
		elapsed := time.Since(start)
		if err != nil {
			log.Errorf("TOOL", "Tool %s internal error after %s: %v", name, elapsed, err)
			return "", err
		}
		if result.Error != nil {
			log.Warnf("TOOL", "Tool %s returned error after %s: %v", name, elapsed, result.Error)
			return "", result.Error
		}
		log.Debugf("TOOL", "Tool %s completed in %s", name, elapsed)
		return result.Output, nil
	}
}
