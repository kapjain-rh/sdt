package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/sdt-project/sdt/pkg/log"
)

// RuntimeInfo describes a detected runtime and its version.
type RuntimeInfo struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Version   string `json:"version"`
	Available bool   `json:"available"`
}

// DetectRuntimes checks which runtimes are available on the system.
func DetectRuntimes() []RuntimeInfo {
	runtimes := []struct {
		name       string
		binary     string
		versionArg string
	}{
		{"python", "python3", "--version"},
		{"node", "node", "--version"},
		{"npm", "npm", "--version"},
		{"npx", "npx", "--version"},
		{"go", "go", "version"},
		{"cypress", "npx", "cypress --version"},
	}

	var results []RuntimeInfo
	for _, rt := range runtimes {
		info := RuntimeInfo{Name: rt.name}
		path, err := exec.LookPath(rt.binary)
		if err != nil {
			info.Available = false
			results = append(results, info)
			continue
		}
		info.Path = path
		info.Available = true

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		args := strings.Fields(rt.versionArg)
		out, err := exec.CommandContext(ctx, rt.binary, args...).CombinedOutput()
		cancel()
		if err == nil {
			info.Version = strings.TrimSpace(string(out))
		}
		results = append(results, info)
	}
	return results
}

// requireBinary checks that a binary exists and returns its path, or a ToolResult error.
func requireBinary(name string) (string, *ToolResult) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", &ToolResult{Error: fmt.Errorf("%s is not installed or not in PATH. Install it first", name)}
	}
	return path, nil
}

// runCommand is the shared execution logic for all runtime tools.
func runCommand(ctx context.Context, constraints *ToolConstraints, binary string, args []string, workingDir string) *ToolResult {
	cmdStr := binary + " " + strings.Join(args, " ")
	log.Infof("TOOL", "runtime: %s", cmdStr)

	runCtx, cancel := constraints.ShellContext(ctx)
	defer cancel()

	cmd := exec.CommandContext(runCtx, binary, args...)
	if workingDir != "" {
		cmd.Dir = workingDir
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
		if runCtx.Err() != nil {
			log.Warnf("TOOL", "runtime command timed out after %s: %s", constraints.ShellTimeout(), cmdStr)
			return &ToolResult{Output: output, Error: fmt.Errorf("command timed out after %s: %s", constraints.ShellTimeout(), cmdStr)}
		}
		log.Warnf("TOOL", "runtime command failed: %v", err)
		return &ToolResult{Output: output, Error: fmt.Errorf("command failed: %w", err)}
	}
	return &ToolResult{Output: output}
}

// RegisterRuntimeTools registers language/runtime execution tools.
func RegisterRuntimeTools(registry *Registry, constraints *ToolConstraints) {
	registerPythonTool(registry, constraints)
	registerNodeTool(registry, constraints)
	registerNPMTool(registry, constraints)
	registerNPXTool(registry, constraints)
	registerGoTool(registry, constraints)
	registerCypressTool(registry, constraints)
	registerCheckRuntimesTool(registry)
}

func registerPythonTool(registry *Registry, constraints *ToolConstraints) {
	registry.Register(&Tool{
		Name:        "python",
		Description: "Run a Python 3 script or one-liner. Use for data processing, JSON manipulation, API calls, or any Python-based automation.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"script": {
					"type": "string",
					"description": "Python code to execute (inline) or path to a .py file"
				},
				"args": {
					"type": "array",
					"items": {"type": "string"},
					"description": "Arguments to pass to the script (optional)"
				},
				"working_dir": {
					"type": "string",
					"description": "Working directory (optional)"
				}
			},
			"required": ["script"]
		}`),
		Handler: func(ctx context.Context, input json.RawMessage) (*ToolResult, error) {
			var params struct {
				Script     string   `json:"script"`
				Args       []string `json:"args"`
				WorkingDir string   `json:"working_dir"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return nil, fmt.Errorf("parsing input: %w", err)
			}

			binary, errResult := requireBinary("python3")
			if errResult != nil {
				return errResult, nil
			}

			var args []string
			if strings.HasSuffix(params.Script, ".py") {
				args = append([]string{params.Script}, params.Args...)
			} else {
				args = append([]string{"-c", params.Script}, params.Args...)
			}

			return runCommand(ctx, constraints, binary, args, params.WorkingDir), nil
		},
	})
}

func registerNodeTool(registry *Registry, constraints *ToolConstraints) {
	registry.Register(&Tool{
		Name:        "node",
		Description: "Run a Node.js script or one-liner. Use for JavaScript-based automation, API testing, or data processing.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"script": {
					"type": "string",
					"description": "JavaScript code to execute (inline) or path to a .js file"
				},
				"args": {
					"type": "array",
					"items": {"type": "string"},
					"description": "Arguments to pass to the script (optional)"
				},
				"working_dir": {
					"type": "string",
					"description": "Working directory (optional)"
				}
			},
			"required": ["script"]
		}`),
		Handler: func(ctx context.Context, input json.RawMessage) (*ToolResult, error) {
			var params struct {
				Script     string   `json:"script"`
				Args       []string `json:"args"`
				WorkingDir string   `json:"working_dir"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return nil, fmt.Errorf("parsing input: %w", err)
			}

			binary, errResult := requireBinary("node")
			if errResult != nil {
				return errResult, nil
			}

			var args []string
			if strings.HasSuffix(params.Script, ".js") || strings.HasSuffix(params.Script, ".mjs") {
				args = append([]string{params.Script}, params.Args...)
			} else {
				args = append([]string{"-e", params.Script}, params.Args...)
			}

			return runCommand(ctx, constraints, binary, args, params.WorkingDir), nil
		},
	})
}

func registerNPMTool(registry *Registry, constraints *ToolConstraints) {
	registry.Register(&Tool{
		Name:        "npm",
		Description: "Run an npm command (install, run, test, etc.). Use for managing Node.js project dependencies and running package scripts.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"command": {
					"type": "string",
					"description": "npm command to run (e.g., install, run test, run build)"
				},
				"working_dir": {
					"type": "string",
					"description": "Working directory containing package.json (optional)"
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

			binary, errResult := requireBinary("npm")
			if errResult != nil {
				return errResult, nil
			}

			args := strings.Fields(params.Command)
			return runCommand(ctx, constraints, binary, args, params.WorkingDir), nil
		},
	})
}

func registerNPXTool(registry *Registry, constraints *ToolConstraints) {
	registry.Register(&Tool{
		Name:        "npx",
		Description: "Run an npm package via npx without installing it globally. Use for one-off package execution (e.g., npx create-react-app, npx jest).",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"command": {
					"type": "string",
					"description": "Package and arguments to run (e.g., jest --coverage, playwright test)"
				},
				"working_dir": {
					"type": "string",
					"description": "Working directory (optional)"
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

			binary, errResult := requireBinary("npx")
			if errResult != nil {
				return errResult, nil
			}

			args := append([]string{"--yes"}, strings.Fields(params.Command)...)
			return runCommand(ctx, constraints, binary, args, params.WorkingDir), nil
		},
	})
}

func registerGoTool(registry *Registry, constraints *ToolConstraints) {
	registry.Register(&Tool{
		Name:        "go_run",
		Description: "Run a Go command (run, build, test, vet, etc.). Use for Go-based automation, building binaries, or running Go scripts.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"command": {
					"type": "string",
					"description": "Go subcommand and arguments (e.g., run main.go, build ./..., test ./pkg/...)"
				},
				"working_dir": {
					"type": "string",
					"description": "Working directory containing go.mod (optional)"
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

			binary, errResult := requireBinary("go")
			if errResult != nil {
				return errResult, nil
			}

			args := strings.Fields(params.Command)
			return runCommand(ctx, constraints, binary, args, params.WorkingDir), nil
		},
	})
}

func registerCypressTool(registry *Registry, constraints *ToolConstraints) {
	registry.Register(&Tool{
		Name:        "cypress",
		Description: "Run Cypress end-to-end tests via npx (no global install needed). Use for browser-based UI testing, component testing, and E2E test suites.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"command": {
					"type": "string",
					"description": "Cypress subcommand and arguments (e.g., run, run --spec cypress/e2e/login.cy.js, run --browser chrome, open)"
				},
				"spec": {
					"type": "string",
					"description": "Specific spec file or pattern to run (optional, shorthand for --spec)"
				},
				"browser": {
					"type": "string",
					"description": "Browser to use: chrome, firefox, electron (optional, default: electron)"
				},
				"config": {
					"type": "string",
					"description": "Path to cypress.config.js (optional)"
				},
				"env": {
					"type": "object",
					"additionalProperties": {"type": "string"},
					"description": "Environment variables to pass to Cypress (optional)"
				},
				"working_dir": {
					"type": "string",
					"description": "Working directory containing cypress.config.js (optional)"
				}
			},
			"required": ["command"]
		}`),
		Handler: func(ctx context.Context, input json.RawMessage) (*ToolResult, error) {
			var params struct {
				Command    string            `json:"command"`
				Spec       string            `json:"spec"`
				Browser    string            `json:"browser"`
				Config     string            `json:"config"`
				Env        map[string]string `json:"env"`
				WorkingDir string            `json:"working_dir"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return nil, fmt.Errorf("parsing input: %w", err)
			}

			// Cypress runs via npx — no global install needed
			binary, errResult := requireBinary("npx")
			if errResult != nil {
				return errResult, nil
			}

			args := []string{"--yes", "cypress"}
			args = append(args, strings.Fields(params.Command)...)

			if params.Spec != "" && !strings.Contains(params.Command, "--spec") {
				args = append(args, "--spec", params.Spec)
			}
			if params.Browser != "" && !strings.Contains(params.Command, "--browser") {
				args = append(args, "--browser", params.Browser)
			}
			if params.Config != "" && !strings.Contains(params.Command, "--config-file") {
				args = append(args, "--config-file", params.Config)
			}
			if len(params.Env) > 0 {
				var envPairs []string
				for k, v := range params.Env {
					envPairs = append(envPairs, fmt.Sprintf("%s=%s", k, v))
				}
				args = append(args, "--env", strings.Join(envPairs, ","))
			}

			// Cypress tests can run longer than shell commands — use 5 min timeout
			cypressCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			defer cancel()

			cmdStr := binary + " " + strings.Join(args, " ")
			log.Infof("TOOL", "cypress: %s", cmdStr)

			cmd := exec.CommandContext(cypressCtx, binary, args...)
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
				if cypressCtx.Err() != nil {
					return &ToolResult{Output: output, Error: fmt.Errorf("cypress timed out after 5m")}, nil
				}
				return &ToolResult{Output: output, Error: fmt.Errorf("cypress failed: %w", err)}, nil
			}
			return &ToolResult{Output: output}, nil
		},
	})
}

func registerCheckRuntimesTool(registry *Registry) {
	registry.Register(&Tool{
		Name:        "check_runtimes",
		Description: "Check which language runtimes are available on this system. Returns availability and version for: python3, node, npm, npx, go, cypress.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {}
		}`),
		Handler: func(ctx context.Context, input json.RawMessage) (*ToolResult, error) {
			runtimes := DetectRuntimes()
			var lines []string
			for _, rt := range runtimes {
				if rt.Available {
					lines = append(lines, fmt.Sprintf("✓ %s: %s (%s)", rt.Name, rt.Version, rt.Path))
				} else {
					lines = append(lines, fmt.Sprintf("✗ %s: not installed", rt.Name))
				}
			}
			return &ToolResult{Output: strings.Join(lines, "\n")}, nil
		},
	})
}
