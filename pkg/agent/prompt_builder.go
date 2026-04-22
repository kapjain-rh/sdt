package agent

import (
	"fmt"
	"strings"

	"github.com/sdt-project/sdt/pkg/tools"
)

// PromptContext holds the information needed to build dynamic prompts.
type PromptContext struct {
	Registry       *tools.Registry
	Constraints    *tools.ToolConstraints
	ProjectContext string // project-specific prompt fragment (e.g., OpenShift tool rules)
}

// BuildToolCatalog generates a tool listing for system prompts.
func BuildToolCatalog(registry *tools.Registry) string {
	defs := registry.LLMToolDefinitions()
	if len(defs) == 0 {
		return "No tools registered."
	}
	var lines []string
	for _, d := range defs {
		lines = append(lines, fmt.Sprintf("- %s: %s", d.Name, d.Description))
	}
	return strings.Join(lines, "\n")
}

// BuildToolRules generates constraint-based rules for system prompts.
func BuildToolRules(constraints *tools.ToolConstraints) string {
	if constraints == nil {
		return ""
	}
	all := constraints.Constraints()
	if len(all) == 0 {
		return ""
	}
	var lines []string
	for _, c := range all {
		if c.Description != "" {
			lines = append(lines, fmt.Sprintf("- %s", c.Description))
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
}

// BuildSystemPrompt combines a base prompt template with dynamic tool info.
func BuildSystemPrompt(basePrompt string, pctx *PromptContext) string {
	if pctx == nil {
		return basePrompt
	}

	prompt := basePrompt

	if pctx.Registry != nil {
		catalog := BuildToolCatalog(pctx.Registry)
		if catalog != "" {
			prompt += "\n\nAvailable tools:\n" + catalog
		}
	}

	if pctx.Constraints != nil {
		rules := BuildToolRules(pctx.Constraints)
		if rules != "" {
			prompt += "\n\nTool selection rules (enforced at runtime — violations will be blocked):\n" + rules
		}
	}

	if pctx.ProjectContext != "" {
		prompt += "\n\n" + pctx.ProjectContext
	}

	return prompt
}
