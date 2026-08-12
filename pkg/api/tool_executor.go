package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type ToolExecutor struct {
	store *ProjectStore
	mu    sync.Mutex
	runs  map[int64]bool
}

func NewToolExecutor(s *ProjectStore) *ToolExecutor {
	return &ToolExecutor{store: s, runs: make(map[int64]bool)}
}

func (te *ToolExecutor) IsRunning(runID int64) bool {
	te.mu.Lock()
	defer te.mu.Unlock()
	return te.runs[runID]
}

func (te *ToolExecutor) Run(tool *Tool, params map[string]string) (int64, error) {
	run := &ToolRun{
		ToolID: tool.ID,
		Status: "pending",
	}
	if err := te.store.CreateToolRun(run); err != nil {
		return 0, fmt.Errorf("creating tool run: %w", err)
	}

	go te.execute(run, tool, params)

	return run.ID, nil
}

func (te *ToolExecutor) RunMCPTool(mgr *MCPManager, serverID int64, serverName, toolName string, arguments json.RawMessage) (int64, error) {
	run := &ToolRun{
		MCPServerID: serverID,
		MCPToolName: toolName,
		Status:      "pending",
	}
	if err := te.store.CreateToolRun(run); err != nil {
		return 0, fmt.Errorf("creating tool run: %w", err)
	}

	go te.executeMCP(run, mgr, serverID, serverName, toolName, arguments)

	return run.ID, nil
}

func (te *ToolExecutor) executeMCP(run *ToolRun, mgr *MCPManager, serverID int64, serverName, toolName string, arguments json.RawMessage) {
	te.mu.Lock()
	te.runs[run.ID] = true
	te.mu.Unlock()
	defer func() {
		te.mu.Lock()
		delete(te.runs, run.ID)
		te.mu.Unlock()
	}()

	now := time.Now()
	run.StartedAt = &now
	run.Status = "running"
	te.store.UpdateToolRun(run)

	te.log(run.ID, "system", "MCP Server: %s", serverName)
	te.log(run.ID, "system", "Tool: %s", toolName)
	if len(arguments) > 0 && string(arguments) != "{}" {
		te.log(run.ID, "system", "Arguments: %s", string(arguments))
	}
	te.log(run.ID, "system", "────────────────────────────────────")

	output, err := mgr.CallTool(serverID, toolName, arguments)

	te.log(run.ID, "system", "────────────────────────────────────")

	if err != nil {
		te.log(run.ID, "stderr", "%s", err.Error())
		te.finish(run, "failed", 1, now)
		return
	}

	for _, line := range strings.Split(output, "\n") {
		te.log(run.ID, "stdout", "%s", line)
	}

	te.finish(run, "passed", 0, now)
}

func (te *ToolExecutor) execute(run *ToolRun, tool *Tool, params map[string]string) {
	te.mu.Lock()
	te.runs[run.ID] = true
	te.mu.Unlock()
	defer func() {
		te.mu.Lock()
		delete(te.runs, run.ID)
		te.mu.Unlock()
	}()

	now := time.Now()
	run.StartedAt = &now
	run.Status = "running"
	te.store.UpdateToolRun(run)

	te.log(run.ID, "system", "Starting tool: %s", tool.Name)
	te.log(run.ID, "system", "Type: %s | Command: %s", tool.Category, tool.Command)

	merged := make(map[string]string)
	for name, p := range tool.InputParams {
		if p.Default != "" {
			merged[name] = p.Default
		}
	}
	for k, v := range params {
		merged[k] = v
	}

	cmdCommand := tool.Command
	for k, v := range merged {
		cmdCommand = strings.ReplaceAll(cmdCommand, "{{"+k+"}}", v)
	}

	cmdArgs := make([]string, len(tool.Args))
	for i, arg := range tool.Args {
		rendered := arg
		for k, v := range merged {
			rendered = strings.ReplaceAll(rendered, "{{"+k+"}}", v)
		}
		cmdArgs[i] = rendered
	}

	if len(merged) > 0 {
		te.log(run.ID, "system", "Parameters: %s", formatParams(merged))
	}

	te.log(run.ID, "system", "Running %s tool: %s %s", tool.Category, cmdCommand, joinArgs(cmdArgs))

	cmd := exec.Command(cmdCommand, cmdArgs...)

	for k, v := range tool.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = append(cmd.Env, "PATH="+getPath())

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		te.log(run.ID, "system", "Error setting up stdout: %s", err)
		te.finish(run, "error", -1, now)
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		te.log(run.ID, "system", "Error setting up stderr: %s", err)
		te.finish(run, "error", -1, now)
		return
	}

	te.log(run.ID, "system", "────────────────────────────────────")

	if err := cmd.Start(); err != nil {
		te.log(run.ID, "stderr", "Failed to start: %s", err)
		te.finish(run, "error", -1, now)
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			te.log(run.ID, "stdout", "%s", scanner.Text())
		}
	}()

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			te.log(run.ID, "stderr", "%s", scanner.Text())
		}
	}()

	wg.Wait()

	exitCode := 0
	status := "passed"
	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
		status = "failed"
	}

	te.log(run.ID, "system", "────────────────────────────────────")
	te.log(run.ID, "system", "Exit code: %d", exitCode)
	te.finish(run, status, exitCode, now)
}

func (te *ToolExecutor) finish(run *ToolRun, status string, exitCode int, startTime time.Time) {
	now := time.Now()
	run.FinishedAt = &now
	run.Status = status
	run.ExitCode = exitCode
	run.DurationMs = now.Sub(startTime).Milliseconds()
	te.store.UpdateToolRun(run)

	te.log(run.ID, "system", "Completed in %dms — %s", run.DurationMs, status)
}

func (te *ToolExecutor) log(runID int64, stream, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	te.store.AppendToolLog(&ToolRunLog{
		RunID:   runID,
		Stream:  stream,
		Message: msg,
	})
}

func joinArgs(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return strings.Join(args, " ")
}

func formatParams(params map[string]string) string {
	parts := make([]string, 0, len(params))
	for k, v := range params {
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, ", ")
}

func getPath() string {
	return strings.Join([]string{
		"/usr/local/bin",
		"/usr/bin",
		"/bin",
		"/usr/sbin",
		"/sbin",
		"/usr/local/go/bin",
		"/opt/homebrew/bin",
	}, ":")
}
