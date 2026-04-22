package mcp

import (
	"encoding/json"

	"github.com/openshift/sdt/pkg/llm"
	"github.com/openshift/sdt/pkg/tools"
)

// ConvertToLLMTools converts the tool registry's tools to llm.ToolDefinition format
// for passing to the Claude API.
func ConvertToLLMTools(registry *tools.Registry) []llm.ToolDefinition {
	var defs []llm.ToolDefinition
	for _, name := range registry.List() {
		tool := registry.Get(name)
		if tool == nil {
			continue
		}
		defs = append(defs, llm.ToolDefinition{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		})
	}
	return defs
}

// CreateToolHandler creates a handler function that dispatches tool calls
// from the LLM agent loop to the tool registry.
func CreateToolHandler(registry *tools.Registry) func(name string, input json.RawMessage) (string, error) {
	return func(name string, input json.RawMessage) (string, error) {
		result, err := registry.Execute(nil, name, input)
		if err != nil {
			return "", err
		}
		if result.Error != nil {
			return result.Output + "\nError: " + result.Error.Error(), nil
		}
		return result.Output, nil
	}
}
