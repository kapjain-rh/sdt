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
	Name        string
	Description string // human-readable, used by prompt builder
	ToolName    string // empty = applies to all tools, specific name = only that tool
	Check       func(toolName string, command string) string
}

// ToolConstraints holds the global set of constraints and tool-specific settings.
type ToolConstraints struct {
	constraints  []Constraint
	shellTimeout time.Duration
}

// NewToolConstraints creates an empty constraint set with default shell timeout.
func NewToolConstraints() *ToolConstraints {
	return &ToolConstraints{
		shellTimeout: 60 * time.Second,
	}
}

// DefaultConstraints returns the framework-level constraint set (product-agnostic).
// Project-specific constraints should be added via AddConstraint().
func DefaultConstraints() *ToolConstraints {
	tc := NewToolConstraints()

	tc.constraints = []Constraint{
		{
			Name:        "block_find_root",
			Description: "Do not search from filesystem root — use relative paths",
			ToolName:    "shell",
			Check: func(_ string, cmd string) string {
				lower := strings.ToLower(strings.TrimSpace(cmd))
				if strings.HasPrefix(lower, "find /") && !strings.HasPrefix(lower, "find /tmp") {
					return "blocked: 'find /' searches the entire filesystem and will hang. Use relative paths or search in specific directories"
				}
				return ""
			},
		},
		{
			Name:        "block_sleep_loops",
			Description: "Do not use shell loops with sleep — use a dedicated polling/waiting tool",
			ToolName:    "shell",
			Check: func(_ string, cmd string) string {
				lower := strings.ToLower(cmd)
				hasSleep := strings.Contains(lower, "sleep")
				hasLoop := strings.Contains(lower, "for ") || strings.Contains(lower, "while ")
				hasSeqSleep := strings.Contains(lower, "seq ") && hasSleep
				if (hasSleep && hasLoop) || hasSeqSleep {
					return "blocked: shell loops with sleep are not allowed — use a dedicated polling/waiting tool instead"
				}
				return ""
			},
		},
		{
			Name:        "block_find_go_files",
			Description: "Do not search for Go source files — this framework uses Markdown specs",
			ToolName:    "shell",
			Check: func(_ string, cmd string) string {
				lower := strings.ToLower(cmd)
				if strings.Contains(lower, "find") && strings.Contains(lower, ".go") {
					return "blocked: do not search for Go source files — this framework uses Markdown specs, not Go tests"
				}
				return ""
			},
		},
	}

	return tc
}

// AddConstraint adds a constraint to the set.
func (tc *ToolConstraints) AddConstraint(c Constraint) {
	tc.constraints = append(tc.constraints, c)
}

// SetShellTimeout changes the maximum shell command duration.
func (tc *ToolConstraints) SetShellTimeout(d time.Duration) {
	tc.shellTimeout = d
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
	return tc.Check("shell", command)
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

// Constraints returns all registered constraints (for prompt generation).
func (tc *ToolConstraints) Constraints() []Constraint {
	result := make([]Constraint, len(tc.constraints))
	copy(result, tc.constraints)
	return result
}

// BlockShellCommand creates a constraint that blocks running a command prefix
// via shell and redirects to the named tool instead.
func BlockShellCommand(prefix string, redirectTool string) Constraint {
	return Constraint{
		Name:        fmt.Sprintf("block_%s_in_shell", strings.ReplaceAll(strings.TrimSpace(prefix), " ", "_")),
		Description: fmt.Sprintf("Do not use '%s' via shell — use the %s tool instead", strings.TrimSpace(prefix), redirectTool),
		ToolName:    "shell",
		Check: func(_ string, cmd string) string {
			lower := strings.ToLower(strings.TrimSpace(cmd))
			if strings.HasPrefix(lower, strings.ToLower(prefix)) {
				return fmt.Sprintf("blocked: do not use '%s' via shell — use the %s tool instead", strings.TrimSpace(prefix), redirectTool)
			}
			return ""
		},
	}
}

// RedirectTool creates a constraint that blocks a specific subcommand on a tool
// and redirects to another tool.
func RedirectTool(fromTool string, matchSubstring string, toTool string) Constraint {
	return Constraint{
		Name:        fmt.Sprintf("redirect_%s_%s", fromTool, strings.ReplaceAll(matchSubstring, " ", "_")),
		Description: fmt.Sprintf("Do not use %s for '%s' — use the %s tool instead", fromTool, matchSubstring, toTool),
		ToolName:    fromTool,
		Check: func(_ string, cmd string) string {
			if strings.Contains(strings.ToLower(cmd), strings.ToLower(matchSubstring)) {
				return fmt.Sprintf("blocked: do not use %s for '%s' — use the %s tool instead, it has built-in support", fromTool, matchSubstring, toTool)
			}
			return ""
		},
	}
}
