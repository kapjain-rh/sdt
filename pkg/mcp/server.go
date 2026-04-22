package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sdt-project/sdt/pkg/tools"
)

// Server exposes MCP tools for use by LLM agents.
// It wraps the tool registry and provides execution context.
type Server struct {
	registry *tools.Registry
}

// NewServer creates a new MCP tool server.
func NewServer(registry *tools.Registry) *Server {
	return &Server{registry: registry}
}

// HandleToolCall processes a tool call from the LLM and returns the result.
func (s *Server) HandleToolCall(ctx context.Context, name string, input json.RawMessage) (string, error) {
	result, err := s.registry.Execute(ctx, name, input)
	if err != nil {
		return "", fmt.Errorf("tool %s failed: %w", name, err)
	}
	if result.Error != nil {
		return result.Output + "\nError: " + result.Error.Error(), nil
	}
	return result.Output, nil
}

// ListTools returns the names and descriptions of all available tools.
func (s *Server) ListTools() []ToolInfo {
	var infos []ToolInfo
	for _, name := range s.registry.List() {
		tool := s.registry.Get(name)
		if tool != nil {
			infos = append(infos, ToolInfo{
				Name:        tool.Name,
				Description: tool.Description,
			})
		}
	}
	return infos
}

// ToolInfo contains basic information about an available tool.
type ToolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}
