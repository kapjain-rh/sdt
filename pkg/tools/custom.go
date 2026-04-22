package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/sdt-project/sdt/pkg/log"
	"gopkg.in/yaml.v3"
)

// CustomToolDef is a YAML-defined tool for the default MCP server.
type CustomToolDef struct {
	Name        string                     `yaml:"name" json:"name"`
	Description string                     `yaml:"description" json:"description"`
	Category    string                     `yaml:"category,omitempty" json:"category,omitempty"`
	Status      string                     `yaml:"status" json:"status"` // "draft" or "approved"
	Input       map[string]CustomToolParam `yaml:"input,omitempty" json:"input,omitempty"`
	Command     string                     `yaml:"command" json:"command"`
}

// CustomToolParam defines a single parameter for a custom tool.
type CustomToolParam struct {
	Type        string `yaml:"type" json:"type"`
	Description string `yaml:"description" json:"description"`
	Required    bool   `yaml:"required,omitempty" json:"required,omitempty"`
	Default     string `yaml:"default,omitempty" json:"default,omitempty"`
}

// LoadCustomTools loads YAML tool definitions from a directory and registers
// approved tools into the registry. Returns all loaded definitions (including drafts).
func LoadCustomTools(dir string, registry *Registry, approvedOnly bool) ([]*CustomToolDef, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading tools dir %s: %w", dir, err)
	}

	var allDefs []*CustomToolDef
	for _, entry := range entries {
		if entry.IsDir() || (!strings.HasSuffix(entry.Name(), ".yaml") && !strings.HasSuffix(entry.Name(), ".yml")) {
			continue
		}
		def, err := LoadCustomToolDef(filepath.Join(dir, entry.Name()))
		if err != nil {
			log.Warnf("TOOLS", "Skipping %s: %v", entry.Name(), err)
			continue
		}
		allDefs = append(allDefs, def)

		if approvedOnly && def.Status != "approved" {
			log.Debugf("TOOLS", "Skipping draft tool %q", def.Name)
			continue
		}

		if registry != nil {
			tool := customDefToTool(def)
			registry.Register(tool)
			log.Debugf("TOOLS", "Registered custom tool %q (status: %s)", def.Name, def.Status)
		}
	}

	return allDefs, nil
}

// LoadCustomToolDef loads a single YAML tool definition.
func LoadCustomToolDef(path string) (*CustomToolDef, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var def CustomToolDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if def.Name == "" {
		return nil, fmt.Errorf("%s: tool name is required", path)
	}
	if def.Command == "" {
		return nil, fmt.Errorf("%s: command is required", path)
	}
	if def.Status == "" {
		def.Status = "draft"
	}
	return &def, nil
}

// SaveCustomToolDef writes a tool definition to a YAML file.
func SaveCustomToolDef(path string, def *CustomToolDef) error {
	data, err := yaml.Marshal(def)
	if err != nil {
		return fmt.Errorf("marshaling tool definition: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

func customDefToTool(def *CustomToolDef) *Tool {
	schema := buildInputSchema(def.Input)
	cmdTemplate := def.Command

	return &Tool{
		Name:        def.Name,
		Description: def.Description,
		InputSchema: schema,
		Category:    def.Category,
		Handler: func(ctx context.Context, input json.RawMessage) (*ToolResult, error) {
			var params map[string]interface{}
			if err := json.Unmarshal(input, &params); err != nil {
				return nil, fmt.Errorf("parsing input: %w", err)
			}

			cmd, err := renderCommand(cmdTemplate, params)
			if err != nil {
				return nil, fmt.Errorf("rendering command: %w", err)
			}

			log.Debugf("TOOLS", "Executing custom tool %q: %s", def.Name, cmd)

			out, err := exec.CommandContext(ctx, "sh", "-c", cmd).CombinedOutput()
			output := strings.TrimSpace(string(out))
			if err != nil {
				if output != "" {
					return &ToolResult{Output: output, Error: fmt.Errorf("command failed: %w", err)}, nil
				}
				return nil, fmt.Errorf("command failed: %w", err)
			}
			return &ToolResult{Output: output}, nil
		},
	}
}

func buildInputSchema(params map[string]CustomToolParam) json.RawMessage {
	properties := make(map[string]interface{})
	var required []string

	for name, param := range params {
		prop := map[string]interface{}{
			"type":        param.Type,
			"description": param.Description,
		}
		properties[name] = prop
		if param.Required {
			required = append(required, name)
		}
	}

	schema := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}

	data, _ := json.Marshal(schema)
	return data
}

func renderCommand(tmpl string, params map[string]interface{}) (string, error) {
	t, err := template.New("cmd").Option("missingkey=zero").Parse(tmpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, params); err != nil {
		return "", err
	}
	return buf.String(), nil
}
