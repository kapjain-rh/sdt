package tools

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Constraint defines a rule that checks whether a tool call should be allowed.
// If the rule matches, it returns a non-empty rejection message with guidance.
type Constraint struct {
	Name       string
	ToolName   string // empty = applies to all tools, specific name = only that tool
	Check      func(toolName string, command string) string
}

// ToolConstraints holds the global set of constraints and tool-specific settings.
type ToolConstraints struct {
	constraints    []Constraint
	shellTimeout   time.Duration
}

// DefaultConstraints returns the standard constraint set for all tools.
func DefaultConstraints() *ToolConstraints {
	tc := &ToolConstraints{
		shellTimeout: 60 * time.Second,
	}

	tc.constraints = []Constraint{
		// --- Shell constraints ---
		{
			Name:     "block_find_root",
			ToolName: "shell",
			Check: func(_ string, cmd string) string {
				lower := strings.ToLower(strings.TrimSpace(cmd))
				if strings.HasPrefix(lower, "find /") && !strings.HasPrefix(lower, "find /tmp") {
					return "blocked: 'find /' searches the entire filesystem and will hang. Use relative paths or search in specific directories"
				}
				return ""
			},
		},
		{
			Name:     "block_sleep_loops",
			ToolName: "shell",
			Check: func(_ string, cmd string) string {
				lower := strings.ToLower(cmd)
				hasSleep := strings.Contains(lower, "sleep")
				hasLoop := strings.Contains(lower, "for ") || strings.Contains(lower, "while ")
				hasSeqSleep := strings.Contains(lower, "seq ") && hasSleep
				if (hasSleep && hasLoop) || hasSeqSleep {
					return "blocked: shell loops with sleep are not allowed — use the wait_for_condition or wait_for_pods_ready tools instead"
				}
				return ""
			},
		},
		{
			Name:     "block_oc_wait_in_shell",
			ToolName: "shell",
			Check: func(_ string, cmd string) string {
				lower := strings.ToLower(strings.TrimSpace(cmd))
				if strings.HasPrefix(lower, "oc wait ") {
					return "blocked: do not use 'oc wait' via shell — use the wait_for_condition tool instead"
				}
				return ""
			},
		},
		{
			Name:     "block_oc_get_in_shell",
			ToolName: "shell",
			Check: func(_ string, cmd string) string {
				lower := strings.ToLower(strings.TrimSpace(cmd))
				if strings.HasPrefix(lower, "oc get ") {
					return "blocked: do not use 'oc get' via shell — use the oc_get tool instead"
				}
				return ""
			},
		},
		{
			Name:     "block_oc_apply_in_shell",
			ToolName: "shell",
			Check: func(_ string, cmd string) string {
				lower := strings.ToLower(strings.TrimSpace(cmd))
				if strings.HasPrefix(lower, "oc apply ") {
					return "blocked: do not use 'oc apply' via shell — use the oc_apply tool instead"
				}
				return ""
			},
		},
		{
			Name:     "block_oc_delete_in_shell",
			ToolName: "shell",
			Check: func(_ string, cmd string) string {
				lower := strings.ToLower(strings.TrimSpace(cmd))
				if strings.HasPrefix(lower, "oc delete ") {
					return "blocked: do not use 'oc delete' via shell — use the oc_delete tool instead"
				}
				return ""
			},
		},
		{
			Name:     "block_oc_patch_in_shell",
			ToolName: "shell",
			Check: func(_ string, cmd string) string {
				lower := strings.ToLower(strings.TrimSpace(cmd))
				if strings.HasPrefix(lower, "oc patch ") {
					return "blocked: do not use 'oc patch' via shell — use the oc_patch tool instead"
				}
				return ""
			},
		},
		{
			Name:     "block_oc_logs_in_shell",
			ToolName: "shell",
			Check: func(_ string, cmd string) string {
				lower := strings.ToLower(strings.TrimSpace(cmd))
				if strings.HasPrefix(lower, "oc logs ") {
					return "blocked: do not use 'oc logs' via shell — use the oc_logs tool instead"
				}
				return ""
			},
		},
		{
			Name:     "block_oc_exec_in_shell",
			ToolName: "shell",
			Check: func(_ string, cmd string) string {
				lower := strings.ToLower(strings.TrimSpace(cmd))
				if strings.HasPrefix(lower, "oc exec ") {
					return "blocked: do not use 'oc exec' via shell — use the oc_exec tool instead"
				}
				return ""
			},
		},
		{
			Name:     "block_go_test_in_shell",
			ToolName: "shell",
			Check: func(_ string, cmd string) string {
				lower := strings.ToLower(strings.TrimSpace(cmd))
				if strings.HasPrefix(lower, "go test") {
					return "blocked: this is a spec-driven testing framework — do not run Go tests. Execute test steps using MCP tools"
				}
				return ""
			},
		},
		{
			Name:     "block_find_go_files",
			ToolName: "shell",
			Check: func(_ string, cmd string) string {
				lower := strings.ToLower(cmd)
				if strings.Contains(lower, "find") && strings.Contains(lower, ".go") {
					return "blocked: do not search for Go source files — this framework uses Markdown specs, not Go tests"
				}
				return ""
			},
		},
		// --- oc_run constraints: redirect to specialized tools ---
		{
			Name:     "redirect_oc_run_wait",
			ToolName: "oc_run",
			Check: func(_ string, cmd string) string {
				lower := strings.ToLower(cmd)
				if strings.Contains(lower, "wait") {
					return "blocked: do not use oc_run for 'oc wait' — use the wait_for_condition tool instead, it has built-in polling and timeout"
				}
				return ""
			},
		},
	}

	return tc
}

// ShellTimeout returns the maximum allowed duration for shell commands.
func (tc *ToolConstraints) ShellTimeout() time.Duration {
	return tc.shellTimeout
}

// ShellContext returns a child context with the shell timeout applied.
func (tc *ToolConstraints) ShellContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, tc.shellTimeout)
}

// CheckShell validates a shell command against all shell constraints.
// Returns nil if allowed, or an error with guidance if blocked.
func (tc *ToolConstraints) CheckShell(command string) error {
	for _, c := range tc.constraints {
		if c.ToolName != "" && c.ToolName != "shell" {
			continue
		}
		if msg := c.Check("shell", command); msg != "" {
			return fmt.Errorf("%s", msg)
		}
	}
	return nil
}

// Check validates a tool call against all applicable constraints.
// Returns nil if allowed, or an error with guidance if blocked.
func (tc *ToolConstraints) Check(toolName string, command string) error {
	for _, c := range tc.constraints {
		if c.ToolName != "" && c.ToolName != toolName {
			continue
		}
		if msg := c.Check(toolName, command); msg != "" {
			return fmt.Errorf("%s", msg)
		}
	}
	return nil
}
