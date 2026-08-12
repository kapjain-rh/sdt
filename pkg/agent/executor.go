package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sdt-project/sdt/pkg/llm"
	"github.com/sdt-project/sdt/pkg/log"
	"github.com/sdt-project/sdt/pkg/tools"
)

// ExecutorAgent executes an execution plan using the LLM as an autonomous agent.
type ExecutorAgent struct {
	llmClient    *llm.Client
	registry     *tools.Registry
	promptContext *PromptContext
}

// ExecutionResult captures the outcome of executing a plan.
type ExecutionResult struct {
	SpecName      string                   `json:"spec_name"`
	Status        string                   `json:"status"`         // "PASSED", "FAILED", "SKIPPED"
	StartTime     time.Time                `json:"start_time"`
	EndTime       time.Time                `json:"end_time"`
	Duration      time.Duration            `json:"duration"`
	PhaseResults  []PhaseResult            `json:"phase_results"`
	FixtureResults []FixtureResult         `json:"fixture_results"`
	Error         string                   `json:"error,omitempty"`
	ToolCalls     []llm.ToolCallLog        `json:"tool_calls"`
	CleanupRun    bool                     `json:"cleanup_run"`
}

// PhaseResult captures the outcome of a phase (setup, steps, verify, cleanup).
type PhaseResult struct {
	Phase      string       `json:"phase"`
	Status     string       `json:"status"` // "PASSED", "FAILED", "SKIPPED"
	StepResults []StepResult `json:"step_results"`
	Error      string       `json:"error,omitempty"`
}

// StepResult captures the outcome of a single step.
type StepResult struct {
	Description string `json:"description"`
	ToolName    string `json:"tool_name"`
	Status      string `json:"status"` // "PASSED", "FAILED", "SKIPPED"
	Output      string `json:"output"`
	Error       string `json:"error,omitempty"`
	Duration    time.Duration `json:"duration"`
}

// FixtureResult captures the outcome of fixture setup/cleanup.
type FixtureResult struct {
	Name          string       `json:"name"`
	Status        string       `json:"status"` // "PASSED", "FAILED", "SKIPPED"
	CreateResult  []StepResult `json:"create"`
	ReadyResult   []StepResult `json:"ready_check"`
	CleanupResult []StepResult `json:"cleanup"`
	Error         string       `json:"error,omitempty"`
}

// NewExecutorAgent creates a new executor agent.
func NewExecutorAgent(llmClient *llm.Client, registry *tools.Registry) *ExecutorAgent {
	return &ExecutorAgent{
		llmClient: llmClient,
		registry:  registry,
	}
}

// WithPromptContext sets dynamic prompt context for the executor.
func (e *ExecutorAgent) WithPromptContext(pctx *PromptContext) *ExecutorAgent {
	e.promptContext = pctx
	return e
}

// systemPrompt returns the executor system prompt with dynamic tool info.
func (e *ExecutorAgent) systemPrompt() string {
	return BuildSystemPrompt(ExecutorSystemPrompt, e.promptContext)
}

// Execute runs the execution plan using the LLM as an autonomous agent.
// The agent calls tools from the registry to perform the test steps.
// The extraContext parameter provides additional instructions or environment
// details that the LLM should consider during execution.
// stopOnError controls whether the agent loop stops on the first tool error.
func (e *ExecutorAgent) Execute(ctx context.Context, plan *ExecutionPlan, extraContext string, stopOnError bool) (*ExecutionResult, error) {
	result := &ExecutionResult{
		SpecName:   plan.SpecName,
		StartTime:  time.Now(),
		Status:     "PASSED",
		PhaseResults: make([]PhaseResult, 0),
		FixtureResults: make([]FixtureResult, 0),
	}
	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
	}()

	totalSteps := 0
	for _, phase := range plan.Phases {
		totalSteps += len(phase.Steps)
	}
	log.Infof("EXEC", "Starting execution of %q — %d phases, %d steps, %d fixtures",
		plan.SpecName, len(plan.Phases), totalSteps, len(plan.Fixtures))

	for i, phase := range plan.Phases {
		log.Debugf("EXEC", "  Phase %d/%d: %s (%d steps)", i+1, len(plan.Phases), phase.Name, len(phase.Steps))
		for j, step := range phase.Steps {
			log.Debugf("EXEC", "    Step %d/%d: %s (tool=%s, on_failure=%s)",
				j+1, len(phase.Steps), step.Description, step.ToolName, step.OnFailure)
		}
	}

	// Clone registry and add execution-scoped tools
	execRegistry := e.registry.Clone()
	execRegistry.Register(&tools.Tool{
		Name:        "report_step_result",
		Description: "Report the result of a test step. You MUST call this after completing each step. FAIL stops execution immediately.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"step_description": {"type": "string", "description": "Brief description of the step that was executed"},
				"status": {"type": "string", "enum": ["PASS", "FAIL", "SKIP"], "description": "The outcome of the step"},
				"error_message": {"type": "string", "description": "Error details when status is FAIL"}
			},
			"required": ["step_description", "status"]
		}`),
		Handler: func(ctx context.Context, input json.RawMessage) (*tools.ToolResult, error) {
			var params struct {
				StepDescription string `json:"step_description"`
				Status          string `json:"status"`
				ErrorMessage    string `json:"error_message"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return nil, fmt.Errorf("parsing report_step_result: %w", err)
			}
			log.Infof("EXEC", "Step result: %s — %s", params.StepDescription, params.Status)
			switch params.Status {
			case "FAIL":
				errMsg := params.ErrorMessage
				if errMsg == "" {
					errMsg = "step failed"
				}
				return nil, fmt.Errorf("step failed: %s: %s: %w", params.StepDescription, errMsg, llm.ErrStopExecution)
			case "SKIP":
				return &tools.ToolResult{Output: fmt.Sprintf("Step skipped: %s", params.StepDescription)}, nil
			default:
				return &tools.ToolResult{Output: fmt.Sprintf("Step passed: %s", params.StepDescription)}, nil
			}
		},
		Category: "framework",
	})

	// Set up the agent loop request
	toolDefs := execRegistry.LLMToolDefinitions()
	var llmTools []llm.ToolDefinition
	for _, td := range toolDefs {
		llmTools = append(llmTools, llm.ToolDefinition{
			Name:        td.Name,
			Description: td.Description,
			InputSchema: td.InputSchema,
		})
	}
	log.Debugf("EXEC", "Registered %d tools for agent loop (includes report_step_result)", len(llmTools))

	prompt := buildExecutorPrompt(plan)
	if extraContext != "" {
		prompt += fmt.Sprintf("\n\nAdditional Context:\n%s\n\nTake the above context into account during execution.", extraContext)
	}

	// Create the initial request
	req := &llm.Request{
		Model:     "",
		MaxTokens: 0,
		System:    e.systemPrompt(),
		Messages: []llm.Message{
			{
				Role: llm.RoleUser,
				Content: []llm.ContentBlock{
					{Type: "text", Text: prompt},
				},
			},
		},
		Tools: llmTools,
	}

	// Use stopOnError=false so tool errors go back to the LLM for interpretation.
	// The LLM decides PASS/FAIL/SKIP via report_step_result. Only report_step_result(FAIL)
	// stops the loop (via ErrStopExecution).
	log.Infof("EXEC", "Launching LLM agent loop for %q", plan.SpecName)
	toolHandler := execRegistry.HandleToolCall(ctx)
	finalResp, toolLogs, loopErr := e.llmClient.RunAgentLoop(ctx, req, toolHandler, false)

	// Always process tool logs — they contain the steps that ran before any failure
	result.ToolCalls = toolLogs

	phaseResult := PhaseResult{
		Phase:       "execution",
		Status:      "PASSED",
		StepResults: make([]StepResult, 0),
	}

	passed, failed, skipped := 0, 0, 0
	for _, tl := range toolLogs {
		// report_step_result calls are step-level verdicts, not tool executions
		if tl.ToolName == "report_step_result" {
			var params struct {
				StepDescription string `json:"step_description"`
				Status          string `json:"status"`
				ErrorMessage    string `json:"error_message"`
			}
			_ = json.Unmarshal(tl.Input, &params)
			stepResult := StepResult{
				ToolName:    "report_step_result",
				Description: params.StepDescription,
				Duration:    tl.Duration,
			}
			switch params.Status {
			case "FAIL":
				stepResult.Status = "FAILED"
				stepResult.Error = params.ErrorMessage
				phaseResult.Status = "FAILED"
				result.Status = "FAILED"
				failed++
			case "SKIP":
				stepResult.Status = "SKIPPED"
				skipped++
			default:
				stepResult.Status = "PASSED"
				passed++
			}
			phaseResult.StepResults = append(phaseResult.StepResults, stepResult)
			continue
		}

		// Skip raw tool calls — they're intermediate; report_step_result is the verdict
		if len(phaseResult.StepResults) > 0 || tl.Error == "" {
			continue
		}
		// If we have a tool error with no report_step_result yet, record it
		stepResult := StepResult{
			ToolName:    tl.ToolName,
			Description: tl.ToolName,
			Output:      tl.Output,
			Duration:    tl.Duration,
			Status:      "FAILED",
			Error:       tl.Error,
		}
		phaseResult.Status = "FAILED"
		result.Status = "FAILED"
		failed++
		phaseResult.StepResults = append(phaseResult.StepResults, stepResult)
	}

	result.PhaseResults = append(result.PhaseResults, phaseResult)

	if loopErr != nil {
		result.Status = "FAILED"
		result.Error = fmt.Sprintf("agent loop error: %v", loopErr)
		log.Errorf("EXEC", "Execution FAILED for %q: %v", plan.SpecName, loopErr)
		return result, loopErr
	}

	if finalResp != nil {
		thinking := finalResp.ThinkingContent()
		if thinking != "" {
			result.ToolCalls = append(result.ToolCalls, llm.ToolCallLog{
				ToolName: "reasoning",
				Output:   thinking,
			})
		}
	}

	result.CleanupRun = true

	log.Infof("EXEC", "Execution completed for %q — status=%s, tool_calls=%d (passed=%d, failed=%d), duration=%s",
		plan.SpecName, result.Status, len(toolLogs), passed, failed, time.Since(result.StartTime))

	return result, nil
}

// buildExecutorPrompt constructs the prompt for the executor agent.
func buildExecutorPrompt(plan *ExecutionPlan) string {
	prompt := fmt.Sprintf(`Execute the following test plan:

Test: %s
Created: %s

Phases:
`, plan.SpecName, plan.CreatedAt.Format(time.RFC3339))

	for _, phase := range plan.Phases {
		prompt += fmt.Sprintf("\n%s:\n", phase.Name)
		for i, step := range phase.Steps {
			prompt += fmt.Sprintf("  %d. %s\n", i+1, step.Description)
			prompt += fmt.Sprintf("     Tool: %s\n", step.ToolName)
			prompt += fmt.Sprintf("     Parameters: %v\n", step.Parameters)
			prompt += fmt.Sprintf("     Expected: %s\n", step.ExpectedResult)
			prompt += fmt.Sprintf("     Validation: %s\n", step.Validation)
		}
	}

	if len(plan.Fixtures) > 0 {
		prompt += "\nFixtures:\n"
		for _, fix := range plan.Fixtures {
			prompt += fmt.Sprintf("  %s (template: %s)\n", fix.Name, fix.Template)
		}
	}

	prompt += `
Execute each step in order. For each step:
1. Call the specified tool with the given parameters
2. Analyze the output to determine if the step succeeded or failed
3. Call report_step_result with the outcome:
   - PASS if the step succeeded
   - FAIL if the step failed and on_failure is "fail" (execution stops)
   - SKIP if the step failed but on_failure is "skip" (execution continues)
   - If on_failure is "retry", retry the step before reporting

IMPORTANT: You MUST call report_step_result after EVERY step. Never skip this call.
When you report FAIL, execution stops immediately — do not call any more tools after that.

Always run cleanup steps at the end, even if earlier steps fail.`

	return prompt
}
