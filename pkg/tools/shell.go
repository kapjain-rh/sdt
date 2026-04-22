package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/sdt-project/sdt/pkg/log"
)

// RegisterShellTools registers local shell and file reading tools.
func RegisterShellTools(registry *Registry, constraints *ToolConstraints) {
	registry.Register(&Tool{
		Name:        "shell",
		Description: "Execute a local shell command. Use ONLY for simple, short-running commands (file operations, text processing). Do NOT use for: oc wait (use wait_for_condition), polling loops with sleep (use wait_for_condition or wait_for_pods_ready), running Go tests, or searching for Go files. Max execution time: 60 seconds.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"command": {
					"type": "string",
					"description": "Shell command to execute (max 60s, no loops with sleep)"
				},
				"working_dir": {
					"type": "string",
					"description": "Working directory for the command (optional)"
				}
			},
			"required": ["command"]
		}`),
		Handler: func(ctx context.Context, input json.RawMessage) (*ToolResult, error) {
			var params struct {
				Command    string `json:"command"`
				WorkingDir string `json:"working_dir"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return nil, fmt.Errorf("parsing input: %w", err)
			}

			if err := constraints.CheckShell(params.Command); err != nil {
				return &ToolResult{Error: err}, nil
			}

			log.Infof("TOOL", "shell: %s", params.Command)

			shellCtx, shellCancel := constraints.ShellContext(ctx)
			defer shellCancel()

			cmd := exec.CommandContext(shellCtx, "sh", "-c", params.Command)
			if params.WorkingDir != "" {
				cmd.Dir = params.WorkingDir
			}

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()
			output := strings.TrimSpace(stdout.String())
			if stderr.Len() > 0 {
				output += "\nSTDERR: " + strings.TrimSpace(stderr.String())
			}
			if err != nil {
				if shellCtx.Err() != nil {
					log.Warnf("TOOL", "shell command timed out after %s: %s", constraints.ShellTimeout(), params.Command)
					return &ToolResult{Output: output, Error: fmt.Errorf("shell command timed out after %s — use wait_for_condition or wait_for_pods_ready for long-running waits", constraints.ShellTimeout())}, nil
				}
				log.Warnf("TOOL", "shell command failed: %v", err)
				return &ToolResult{Output: output, Error: fmt.Errorf("shell command failed: %w", err)}, nil
			}
			return &ToolResult{Output: output}, nil
		},
	})

	registry.Register(&Tool{
		Name:        "read_file",
		Description: "Read the contents of a local file. Useful for reading test data, templates, or configuration files.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "Path to the file to read"
				}
			},
			"required": ["path"]
		}`),
		Handler: func(ctx context.Context, input json.RawMessage) (*ToolResult, error) {
			var params struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return nil, fmt.Errorf("parsing input: %w", err)
			}

			log.Debugf("TOOL", "read_file: %s", params.Path)
			data, err := os.ReadFile(params.Path)
			if err != nil {
				log.Warnf("TOOL", "read_file failed: %s: %v", params.Path, err)
				return &ToolResult{Error: fmt.Errorf("reading file %s: %w", params.Path, err)}, nil
			}
			log.Debugf("TOOL", "read_file: %s (%d bytes)", params.Path, len(data))
			return &ToolResult{Output: string(data)}, nil
		},
	})

	registry.Register(&Tool{
		Name:        "write_file",
		Description: "Write content to a local file. Useful for creating rendered templates or temporary configuration files.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "Path to write the file to"
				},
				"content": {
					"type": "string",
					"description": "Content to write to the file"
				}
			},
			"required": ["path", "content"]
		}`),
		Handler: func(ctx context.Context, input json.RawMessage) (*ToolResult, error) {
			var params struct {
				Path    string `json:"path"`
				Content string `json:"content"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return nil, fmt.Errorf("parsing input: %w", err)
			}

			log.Infof("TOOL", "write_file: %s (%d bytes)", params.Path, len(params.Content))
			err := os.WriteFile(params.Path, []byte(params.Content), 0644)
			if err != nil {
				log.Warnf("TOOL", "write_file failed: %s: %v", params.Path, err)
				return &ToolResult{Error: fmt.Errorf("writing file %s: %w", params.Path, err)}, nil
			}
			return &ToolResult{Output: fmt.Sprintf("File written: %s (%d bytes)", params.Path, len(params.Content))}, nil
		},
	})
}

