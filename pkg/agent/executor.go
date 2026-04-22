package agent

import (
	"context"
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

	// Set up the agent loop request
	toolDefs := e.registry.LLMToolDefinitions()
	var llmTools []llm.ToolDefinition
	for _, td := range toolDefs {
		llmTools = append(llmTools, llm.ToolDefinition{
			Name:        td.Name,
			Description: td.Description,
			InputSchema: td.InputSchema,
		})
	}
	log.Debugf("EXEC", "Registered %d tools for agent loop", len(llmTools))

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

	log.Infof("EXEC", "Launching LLM agent loop for %q (stopOnError=%v)", plan.SpecName, stopOnError)
	toolHandler := e.registry.HandleToolCall(ctx)
	finalResp, toolLogs, err := e.llmClient.RunAgentLoop(ctx, req, toolHandler, stopOnError)
	if err != nil {
		result.Status = "FAILED"
		result.Error = fmt.Sprintf("agent loop error: %v", err)
		log.Errorf("EXEC", "Execution FAILED for %q: %v", plan.SpecName, err)
		return result, err
	}

	// Store tool call logs
	result.ToolCalls = toolLogs

	// Parse the final response to extract execution results
	// For now, we create a simple result based on tool call success
	phaseResult := PhaseResult{
		Phase:       "execution",
		Status:      "PASSED",
		StepResults: make([]StepResult, 0),
	}

	passed, failed := 0, 0
	for _, log := range toolLogs {
		stepResult := StepResult{
			ToolName:    log.ToolName,
			Output:      log.Output,
			Duration:    log.Duration,
		}

		if log.Error != "" {
			stepResult.Status = "FAILED"
			stepResult.Error = log.Error
			phaseResult.Status = "FAILED"
			result.Status = "FAILED"
			failed++
		} else {
			stepResult.Status = "PASSED"
			passed++
		}

		phaseResult.StepResults = append(phaseResult.StepResults, stepResult)
	}

	result.PhaseResults = append(result.PhaseResults, phaseResult)

	// If there was reasoning in the response, include it (extended thinking)
	thinking := finalResp.ThinkingContent()
	if thinking != "" {
		result.ToolCalls = append(result.ToolCalls, llm.ToolCallLog{
			ToolName: "reasoning",
			Output:   thinking,
		})
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
2. Check that the output matches the expected result
3. If validation passes, continue to the next step
4. If validation fails and on_failure is "fail", stop and report error
5. If on_failure is "retry", try again
6. If on_failure is "skip", skip to the next step

Always run cleanup steps at the end, even if earlier steps fail.

Report tool calls and results as you progress.`

	return prompt
}
