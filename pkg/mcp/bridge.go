package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sdt-project/sdt/pkg/log"
	"github.com/sdt-project/sdt/pkg/tools"
)

// RegisterMCPTools discovers tools from an MCP server and registers them
// into the SDT tool registry. Each MCP tool becomes a regular SDT tool
// whose handler calls back to the MCP server.
func RegisterMCPTools(client *Client, registry *tools.Registry, category string) error {
	mcpTools, err := client.ListTools()
	if err != nil {
		return fmt.Errorf("listing tools from MCP server %q: %w", client.name, err)
	}

	for _, mt := range mcpTools {
		tool := mcpToolToSDT(client, mt, category)
		registry.Register(tool)
		log.Debugf("MCP", "Registered tool %q from server %q", mt.Name, client.name)
	}

	log.Infof("MCP", "Registered %d tools from server %q", len(mcpTools), client.name)
	return nil
}

func mcpToolToSDT(client *Client, mt MCPTool, category string) *tools.Tool {
	toolName := mt.Name
	return &tools.Tool{
		Name:        toolName,
		Description: mt.Description,
		InputSchema: mt.InputSchema,
		Category:    category,
		Handler: func(ctx context.Context, input json.RawMessage) (*tools.ToolResult, error) {
			output, err := client.CallTool(toolName, input)
			if err != nil {
				return &tools.ToolResult{Error: err}, nil
			}
			return &tools.ToolResult{Output: output}, nil
		},
	}
}
