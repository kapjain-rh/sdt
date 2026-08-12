package api

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type RunEvent struct {
	Type        string `json:"type"`
	CaseID      int64  `json:"case_id,omitempty"`
	ResultID    int64  `json:"result_id,omitempty"`
	ExecutionID int64  `json:"execution_id,omitempty"`
	Title       string `json:"title,omitempty"`
	Verdict     string `json:"verdict,omitempty"`
	DurationMs  int64  `json:"duration_ms,omitempty"`
	Passed      int    `json:"passed,omitempty"`
	Failed      int    `json:"failed,omitempty"`
	Blocked     int    `json:"blocked,omitempty"`
	Skipped     int    `json:"skipped,omitempty"`
}

type SpecExecutor struct {
	store      *ProjectStore
	mu         sync.Mutex
	runs       map[int64]bool
	sdtBinary  string
	projectDir string

	runMu         sync.RWMutex
	runExecutions map[int64]int64      // runID → current executionID
	runEvents     map[int64][]RunEvent // runID → buffered events
	activeRuns    map[int64]bool       // runID → true while batch is running
}

func NewSpecExecutor(s *ProjectStore, projectDir string) *SpecExecutor {
	sdtBinary, _ := exec.LookPath("sdt")
	if sdtBinary == "" {
		sdtBinary = filepath.Join(filepath.Dir(os.Args[0]), "sdt")
	}

	se := &SpecExecutor{
		store:         s,
		runs:          make(map[int64]bool),
		sdtBinary:     sdtBinary,
		projectDir:    projectDir,
		runExecutions: make(map[int64]int64),
		runEvents:     make(map[int64][]RunEvent),
		activeRuns:    make(map[int64]bool),
	}
	se.cleanOrphanedExecutions()
	return se
}

func (se *SpecExecutor) cleanOrphanedExecutions() {
	ids := se.store.listJSONIDs("executions")
	for _, id := range ids {
		var e Execution
		if err := se.store.loadJSON("executions", id, &e); err != nil {
			continue
		}
		if e.Status == "pending" || e.Status == "running" {
			now := time.Now()
			e.Status = "error"
			e.Verdict = "error"
			e.FinishedAt = &now
			if e.StartedAt != nil {
				e.DurationMs = now.Sub(*e.StartedAt).Milliseconds()
			}
			se.store.saveJSON("executions", id, &e)
			se.store.AppendLog(&ExecutionLog{
				ExecutionID: id,
				StepIndex:   -1,
				LogType:     "error",
				Message:     fmt.Sprintf("Execution was orphaned (status was %q at server restart) — marked as error", e.Status),
			})
		}
	}
}

func (se *SpecExecutor) RunCase(execution *Execution) {
	go se.executeCase(execution)
}

func (se *SpecExecutor) RunSteps(execution *Execution, title string, steps []string) {
	go se.executeSteps(execution, title, steps)
}

func (se *SpecExecutor) executeCase(execution *Execution) {
	se.mu.Lock()
	se.runs[execution.ID] = true
	se.mu.Unlock()
	defer func() {
		se.mu.Lock()
		delete(se.runs, execution.ID)
		se.mu.Unlock()
	}()

	now := time.Now()
	execution.StartedAt = &now
	execution.Status = "running"
	se.store.UpdateExecution(execution)

	tc, err := se.store.GetCase(execution.CaseID)
	if err != nil || tc == nil {
		se.logExec(execution.ID, -1, "error", "Test case not found")
		se.finishExec(execution, "error", "error", now)
		return
	}

	se.logExec(execution.ID, -1, "info", fmt.Sprintf("Starting execution: %s", tc.Title))

	cases, _ := se.store.listCasesInternal("")
	var specPath string
	for _, c := range cases {
		if c.ID == execution.CaseID {
			specPath = c.filePath
			break
		}
	}

	if specPath == "" {
		se.logExec(execution.ID, -1, "error", "Spec file not found for this case")
		se.finishExec(execution, "error", "error", now)
		return
	}

	absSpecPath, _ := filepath.Abs(specPath)
	se.logExec(execution.ID, -1, "info", fmt.Sprintf("Spec file: %s", filepath.Base(absSpecPath)))
	se.logExec(execution.ID, -1, "separator", "────────────────────────────────────")

	args := []string{"specs", "run", absSpecPath}
	se.runSDT(execution, args, now)
}

func (se *SpecExecutor) executeSteps(execution *Execution, title string, steps []string) {
	se.mu.Lock()
	se.runs[execution.ID] = true
	se.mu.Unlock()
	defer func() {
		se.mu.Lock()
		delete(se.runs, execution.ID)
		se.mu.Unlock()
	}()

	now := time.Now()
	execution.StartedAt = &now
	execution.Status = "running"
	se.store.UpdateExecution(execution)

	se.logExec(execution.ID, -1, "info", fmt.Sprintf("Running: %s", title))

	absProjDir, _ := filepath.Abs(se.projectDir)
	tmpDir := filepath.Join(absProjDir, ".sdt", "tmp")
	os.MkdirAll(tmpDir, 0755)

	safeName := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, title)
	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("exec_%d_%s.md", execution.ID, safeName))

	var content strings.Builder
	content.WriteString(fmt.Sprintf("# Test: %s\n\n", title))
	content.WriteString("## Metadata\n")
	content.WriteString("- Status: draft\n")
	content.WriteString("- Priority: Medium\n\n")
	content.WriteString("## Steps\n")
	for i, step := range steps {
		content.WriteString(fmt.Sprintf("%d. %s\n", i+1, step))
	}

	if err := os.WriteFile(tmpFile, []byte(content.String()), 0644); err != nil {
		se.logExec(execution.ID, -1, "error", fmt.Sprintf("Failed to create temp spec: %s", err))
		se.finishExec(execution, "error", "error", now)
		return
	}
	defer os.Remove(tmpFile)

	for i, step := range steps {
		se.logExec(execution.ID, i, "step_start", fmt.Sprintf("Step %d: %s", i+1, step))
	}
	se.logExec(execution.ID, -1, "separator", "────────────────────────────────────")

	args := []string{"specs", "run", tmpFile}
	se.runSDT(execution, args, now)
}

func (se *SpecExecutor) runSDT(execution *Execution, args []string, startTime time.Time) {
	if _, err := os.Stat(se.sdtBinary); os.IsNotExist(err) {
		se.logExec(execution.ID, -1, "error", fmt.Sprintf("SDT binary not found at: %s", se.sdtBinary))
		se.logExec(execution.ID, -1, "error", "Install SDT: cd /path/to/sdt && make install")
		se.finishExec(execution, "error", "error", startTime)
		return
	}

	se.logExec(execution.ID, -1, "running", fmt.Sprintf("Executing: sdt %s", strings.Join(args, " ")))

	cmd := exec.Command(se.sdtBinary, args...)
	cmd.Dir = se.projectDir
	cmd.Env = append(os.Environ(), "SDT_LOG_LEVEL=info")
	for k, v := range execution.EnvVars {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		se.logExec(execution.ID, -1, "error", fmt.Sprintf("stdout pipe: %s", err))
		se.finishExec(execution, "error", "error", startTime)
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		se.logExec(execution.ID, -1, "error", fmt.Sprintf("stderr pipe: %s", err))
		se.finishExec(execution, "error", "error", startTime)
		return
	}

	if err := cmd.Start(); err != nil {
		se.logExec(execution.ID, -1, "error", fmt.Sprintf("Failed to start: %s", err))
		se.finishExec(execution, "error", "error", startTime)
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 64*1024), 256*1024)
		for scanner.Scan() {
			line := scanner.Text()
			logType := classifyLogLine(line)
			se.logExec(execution.ID, -1, logType, line)
		}
	}()

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		scanner.Buffer(make([]byte, 64*1024), 256*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.TrimSpace(line) == "" {
				continue
			}
			se.logExec(execution.ID, -1, "info", line)
		}
	}()

	wg.Wait()

	err = cmd.Wait()
	se.logExec(execution.ID, -1, "separator", "────────────────────────────────────")

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			se.logExec(execution.ID, -1, "error", fmt.Sprintf("Exit code: %d", exitErr.ExitCode()))
		} else {
			se.logExec(execution.ID, -1, "error", fmt.Sprintf("Error: %s", err))
		}
		se.logExec(execution.ID, -1, "verdict", "failed")
		se.finishExec(execution, "failed", "failed", startTime)
	} else {
		se.logExec(execution.ID, -1, "verdict", "passed")
		se.finishExec(execution, "passed", "passed", startTime)
	}
}

func (se *SpecExecutor) finishExec(execution *Execution, status, verdict string, startTime time.Time) {
	now := time.Now()
	execution.FinishedAt = &now
	execution.Status = status
	execution.Verdict = verdict
	execution.DurationMs = now.Sub(startTime).Milliseconds()
	se.store.UpdateExecution(execution)

	se.logExec(execution.ID, -1, "finished", fmt.Sprintf("Completed in %dms — %s", execution.DurationMs, verdict))
}

func (se *SpecExecutor) logExec(execID int64, stepIndex int, logType, message string) {
	se.store.AppendLog(&ExecutionLog{
		ExecutionID: execID,
		StepIndex:   stepIndex,
		LogType:     logType,
		Message:     message,
	})
}

// SaveCacheForCase runs a dry-run planning pass to generate and cache the plan for a spec.
func (se *SpecExecutor) SaveCacheForCase(caseID int64) error {
	cases, _ := se.store.listCasesInternal("")
	var specPath string
	for _, c := range cases {
		if c.ID == caseID {
			specPath = c.filePath
			break
		}
	}
	if specPath == "" {
		return fmt.Errorf("spec file not found for case %d", caseID)
	}

	absSpecPath, _ := filepath.Abs(specPath)
	cmd := exec.Command(se.sdtBinary, "run", absSpecPath, "--dry-run")
	cmd.Dir = se.projectDir
	cmd.Env = append(os.Environ(), "SDT_LOG_LEVEL=warn")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("dry-run failed: %s\n%s", err, string(out))
	}
	return nil
}

func (se *SpecExecutor) RunBatch(runID int64, executedBy string, envVars map[string]string) {
	go se.executeBatch(runID, executedBy, envVars)
}

func (se *SpecExecutor) executeBatch(runID int64, executedBy string, envVars map[string]string) {
	se.runMu.Lock()
	se.activeRuns[runID] = true
	se.runEvents[runID] = nil
	se.runMu.Unlock()
	defer func() {
		se.runMu.Lock()
		se.activeRuns[runID] = false
		delete(se.runExecutions, runID)
		se.runMu.Unlock()
	}()

	se.store.StartRun(runID)

	results, err := se.store.ListResults(runID)
	if err != nil {
		se.emitRunEvent(runID, RunEvent{Type: "done"})
		return
	}

	// Build case title → result mapping for matching console output
	type caseInfo struct {
		result    TestResult
		caseTitle string
	}
	casesByTitle := map[string]*caseInfo{}
	for i := range results {
		r := &results[i]
		if r.Case == nil || r.Status != "untested" {
			continue
		}
		casesByTitle[r.Case.Title] = &caseInfo{result: *r, caseTitle: r.Case.Title}
	}

	// Create a single execution record for the suite run
	execution := &Execution{
		CaseID:     0,
		Status:     "pending",
		ExecutedBy: executedBy,
		EnvVars:    envVars,
	}
	if err := se.store.CreateExecution(execution); err != nil {
		se.emitRunEvent(runID, RunEvent{Type: "done"})
		return
	}

	se.runMu.Lock()
	se.runExecutions[runID] = execution.ID
	se.runMu.Unlock()

	// Emit case_start for all cases upfront so UI shows them
	for _, r := range results {
		if r.Case == nil || r.Status != "untested" {
			continue
		}
		se.emitRunEvent(runID, RunEvent{
			Type:        "case_start",
			CaseID:      r.CaseID,
			ResultID:    r.ID,
			ExecutionID: execution.ID,
			Title:       r.Case.Title,
		})
	}

	now := time.Now()
	execution.StartedAt = &now
	execution.Status = "running"
	se.store.UpdateExecution(execution)

	// Run `sdt run <specsDir>` as a single subprocess for proper suite/group hook execution
	specsDir, _ := filepath.Abs(se.store.SpecsDir())
	args := []string{"run", specsDir, "--include-drafts"}
	se.logExec(execution.ID, -1, "running", fmt.Sprintf("Executing: sdt %s", strings.Join(args, " ")))

	cmd := exec.Command(se.sdtBinary, args...)
	cmd.Dir = se.projectDir
	cmd.Env = append(os.Environ(), "SDT_LOG_LEVEL=info")
	for k, v := range envVars {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		se.logExec(execution.ID, -1, "error", fmt.Sprintf("stdout pipe: %s", err))
		se.finishExec(execution, "error", "error", now)
		se.emitRunEvent(runID, RunEvent{Type: "done"})
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		se.logExec(execution.ID, -1, "error", fmt.Sprintf("stderr pipe: %s", err))
		se.finishExec(execution, "error", "error", now)
		se.emitRunEvent(runID, RunEvent{Type: "done"})
		return
	}

	if err := cmd.Start(); err != nil {
		se.logExec(execution.ID, -1, "error", fmt.Sprintf("Failed to start: %s", err))
		se.finishExec(execution, "error", "error", now)
		se.emitRunEvent(runID, RunEvent{Type: "done"})
		return
	}

	passed, failed := 0, 0
	var currentSpec string

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 64*1024), 256*1024)
		for scanner.Scan() {
			line := scanner.Text()
			logType := classifyLogLine(line)
			se.logExec(execution.ID, -1, logType, line)

			// Track spec start: "=== RUN  <name>"
			if strings.HasPrefix(line, "=== RUN  ") {
				currentSpec = strings.TrimPrefix(line, "=== RUN  ")
			}

			// Track spec end: "--- PASS: <name>" or "--- FAIL: <name>"
			if strings.HasPrefix(line, "--- PASS: ") || strings.HasPrefix(line, "--- FAIL: ") || strings.HasPrefix(line, "--- SKIP: ") {
				verdict := "failed"
				var specName string
				if strings.HasPrefix(line, "--- PASS: ") {
					verdict = "passed"
					specName = strings.TrimPrefix(line, "--- PASS: ")
				} else if strings.HasPrefix(line, "--- FAIL: ") {
					verdict = "failed"
					specName = strings.TrimPrefix(line, "--- FAIL: ")
				} else {
					verdict = "skipped"
					specName = strings.TrimPrefix(line, "--- SKIP: ")
				}
				// Strip duration suffix: "name (1.2s)" → "name"
				if idx := strings.LastIndex(specName, " ("); idx > 0 {
					specName = specName[:idx]
				}

				if ci, ok := casesByTitle[specName]; ok {
					ci.result.Status = verdict
					ci.result.ExecutedBy = executedBy
					se.store.UpdateResult(&ci.result)

					se.emitRunEvent(runID, RunEvent{
						Type:    "case_done",
						CaseID:  ci.result.CaseID,
						ResultID: ci.result.ID,
						Verdict: verdict,
					})

					if verdict == "passed" {
						passed++
					} else {
						failed++
					}
				}
				currentSpec = ""
			}
		}
	}()

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		scanner.Buffer(make([]byte, 64*1024), 256*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.TrimSpace(line) == "" {
				continue
			}
			se.logExec(execution.ID, -1, "info", line)
		}
	}()

	wg.Wait()

	err = cmd.Wait()
	se.logExec(execution.ID, -1, "separator", "────────────────────────────────────")

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			se.logExec(execution.ID, -1, "error", fmt.Sprintf("Exit code: %d", exitErr.ExitCode()))
		}
		se.finishExec(execution, "completed", "failed", now)
	} else {
		se.finishExec(execution, "completed", "passed", now)
	}

	_ = currentSpec

	se.store.CompleteRun(runID)

	se.emitRunEvent(runID, RunEvent{
		Type:   "done",
		Passed: passed,
		Failed: failed,
	})
}

func (se *SpecExecutor) emitRunEvent(runID int64, event RunEvent) {
	se.runMu.Lock()
	defer se.runMu.Unlock()
	se.runEvents[runID] = append(se.runEvents[runID], event)
}

func (se *SpecExecutor) GetRunEvents(runID int64, afterIndex int) []RunEvent {
	se.runMu.RLock()
	defer se.runMu.RUnlock()
	events := se.runEvents[runID]
	if afterIndex >= len(events) {
		return nil
	}
	return events[afterIndex:]
}

func (se *SpecExecutor) IsRunActive(runID int64) bool {
	se.runMu.RLock()
	defer se.runMu.RUnlock()
	return se.activeRuns[runID]
}

func (se *SpecExecutor) ActiveExecution(runID int64) int64 {
	se.runMu.RLock()
	defer se.runMu.RUnlock()
	return se.runExecutions[runID]
}

func classifyLogLine(line string) string {
	lower := strings.ToLower(line)
	switch {
	case strings.Contains(lower, "pass") && (strings.Contains(lower, "verdict") || strings.Contains(lower, "result")):
		return "step_status"
	case strings.Contains(lower, "fail"):
		return "error"
	case strings.Contains(lower, "running") || strings.Contains(lower, "executing"):
		return "running"
	case strings.Contains(lower, "step") && (strings.Contains(lower, "start") || strings.Contains(line, "→")):
		return "step_start"
	case strings.Contains(lower, "✓") || strings.Contains(lower, "done") || strings.Contains(lower, "complete"):
		return "step_done"
	case strings.HasPrefix(line, "─"):
		return "separator"
	default:
		return "info"
	}
}
