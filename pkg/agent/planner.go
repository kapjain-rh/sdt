package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sdt-project/sdt/pkg/cache"
	"github.com/sdt-project/sdt/pkg/fixture"
	"github.com/sdt-project/sdt/pkg/llm"
	"github.com/sdt-project/sdt/pkg/log"
	"github.com/sdt-project/sdt/pkg/spec"
)

// PlannerAgent creates execution plans from test specifications.
type PlannerAgent struct {
	llmClient    *llm.Client
	store        *cache.Store
	promptContext *PromptContext
}

// ExecutionPlan is the output of the planner: a detailed execution plan for a test.
type ExecutionPlan struct {
	SpecHash  string      `json:"spec_hash"`
	SpecName  string      `json:"spec_name"`
	CreatedAt time.Time   `json:"created_at"`
	Model     string      `json:"model"`
	Phases    []PlanPhase `json:"phases"`
	Fixtures  []FixturePlan `json:"fixtures"`
}

// PlanPhase represents a phase (setup, steps, verify, cleanup) in the execution plan.
type PlanPhase struct {
	Name  string     `json:"name"`
	Steps []PlanStep `json:"steps"`
}

// PlanStep represents a single step in the plan.
type PlanStep struct {
	Description    string                 `json:"description"`
	ToolName       string                 `json:"tool_name"`
	Parameters     map[string]interface{} `json:"parameters"`
	ExpectedResult string                 `json:"expected_result"`
	Validation     interface{}            `json:"validation"`
	OnFailure      string                 `json:"on_failure"` // "fail", "retry", "skip"
}

// UnmarshalJSON implements custom unmarshaling for PlanStep to handle
// LLM responses where fields may be strings, objects, or arrays, and
// field names may differ from our Go struct tags.
func (ps *PlanStep) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Helper: case-insensitive field lookup — LLM may use camelCase, PascalCase, or snake_case
	getField := func(keys ...string) json.RawMessage {
		for _, k := range keys {
			if v, ok := raw[k]; ok && string(v) != "null" {
				return v
			}
		}
		return nil
	}

	// Helper: unmarshal a field as a string, falling back to raw JSON representation
	toString := func(keys ...string) string {
		v := getField(keys...)
		if v == nil {
			return ""
		}
		var s string
		if err := json.Unmarshal(v, &s); err == nil {
			return s
		}
		return string(v)
	}

	ps.Description = toString("description", "Description", "desc", "name")
	ps.ToolName = toString("tool_name", "Tool", "tool", "toolName", "ToolName")
	ps.ExpectedResult = toString("expected_result", "ExpectedResult", "expected", "expectedResult", "Expected")
	ps.Validation = toString("validation", "Validation", "validate", "check")
	ps.OnFailure = toString("on_failure", "OnFailure", "failureStrategy", "FailureStrategy",
		"onFailure", "failure_strategy", "FailureHandling", "failureHandling")

	// Parameters: accept map[string]interface{}
	paramField := getField("parameters", "Parameters", "params", "Params")
	if paramField != nil {
		var params map[string]interface{}
		if err := json.Unmarshal(paramField, &params); err != nil {
			ps.Parameters = map[string]interface{}{"_raw": string(paramField)}
		} else {
			ps.Parameters = params
		}
	}

	return nil
}

// FixturePlan represents fixture setup/teardown steps in the plan.
type FixturePlan struct {
	Name       string                 `json:"name"`
	Template   string                 `json:"template"`
	Parameters map[string]interface{} `json:"parameters"`
	Create     []PlanStep             `json:"create"`
	ReadyCheck []PlanStep             `json:"ready_check"`
	Cleanup    []PlanStep             `json:"cleanup"`
}

// NewPlannerAgent creates a new planner agent.
func NewPlannerAgent(llmClient *llm.Client, store *cache.Store) *PlannerAgent {
	return &PlannerAgent{
		llmClient: llmClient,
		store:     store,
	}
}

// WithPromptContext sets dynamic prompt context for the planner.
func (p *PlannerAgent) WithPromptContext(pctx *PromptContext) *PlannerAgent {
	p.promptContext = pctx
	return p
}

// systemPrompt returns the planner system prompt with dynamic tool info.
func (p *PlannerAgent) systemPrompt() string {
	return BuildSystemPrompt(PlannerSystemPrompt, p.promptContext)
}

// Plan generates an execution plan for a test spec, using cache when available.
// It checks the cache first by computing a spec hash. If a plan exists, it returns it.
// Otherwise, it generates a new plan via LLM and saves it to cache.
// The extraContext parameter provides additional instructions or environment details
// that the LLM should consider when planning (e.g., cluster-specific conditions).
func (p *PlannerAgent) Plan(ctx context.Context, specHash string, testSpec *spec.TestSpec, suiteSpec *spec.SuiteSpec, groupSpec *spec.GroupSpec, fixtures []*fixture.Definition, extraContext string) (*ExecutionPlan, error) {
	// Check cache first (skip if no hash provided, e.g. --no-cache mode)
	if specHash != "" {
		if cached, ok := p.store.GetPlan(specHash); ok {
			var plan ExecutionPlan
			if err := json.Unmarshal(cached, &plan); err == nil {
				return &plan, nil
			}
		}
	}

	// Generate new plan via LLM
	plan, err := p.generatePlan(ctx, testSpec, suiteSpec, groupSpec, fixtures, extraContext)
	if err != nil {
		return nil, err
	}

	plan.SpecHash = specHash
	plan.Model = p.llmClient.Model()

	// Save to cache
	if specHash != "" {
		if planBytes, err := json.Marshal(plan); err == nil {
			p.store.SavePlan(specHash, planBytes)
		}
	}

	return plan, nil
}

// PlanHooks generates an execution plan for hook steps (pre-suite, pre-test, etc.),
// using cache when available. This ensures hooks produce consistent plans across runs.
func (p *PlannerAgent) PlanHooks(ctx context.Context, phaseName string, steps []spec.StepDef, extraContext string) (*ExecutionPlan, error) {
	stepTexts := make([]string, len(steps))
	for i, s := range steps {
		stepTexts[i] = s.RawText
	}
	specHash := cache.ComputeHookHash(phaseName, stepTexts)

	// Check cache first
	if p.store != nil {
		if cached, ok := p.store.GetPlan(specHash); ok {
			var plan ExecutionPlan
			if err := json.Unmarshal(cached, &plan); err == nil {
				log.Infof("PLAN", "Using cached hook plan for %q (%d phases)", phaseName, len(plan.Phases))
				return &plan, nil
			}
		}
	}

	// Generate new plan via LLM
	prompt := fmt.Sprintf("Hook Phase: %s\n\nSteps:\n", phaseName)
	for i, step := range steps {
		prompt += fmt.Sprintf("%d. %s\n", i+1, step.RawText)
	}
	prompt += "\nGenerate a detailed execution plan as JSON. For each step, specify the tool to use, parameters, expected results, and validation criteria."

	if extraContext != "" {
		prompt += fmt.Sprintf("\n\nAdditional Context:\n%s\n\nTake the above context into account when generating the execution plan.", extraContext)
	}

	log.Infof("PLAN", "Requesting hook plan from LLM for %q (%d steps)", phaseName, len(steps))

	resp, err := p.llmClient.Chat(ctx, p.systemPrompt(), prompt, nil)
	if err != nil {
		return nil, fmt.Errorf("llm call for hook plan failed: %w", err)
	}

	content := resp.TextContent()
	log.Infof("PLAN", "Hook plan response received for %q (%d chars)", phaseName, len(content))

	var plan ExecutionPlan
	if err := json.Unmarshal([]byte(content), &plan); err != nil {
		jsonStart := jsonStartIndex(content)
		if jsonStart >= 0 {
			jsonEnd := jsonEndIndex(content, jsonStart)
			if jsonEnd > jsonStart {
				jsonStr := content[jsonStart : jsonEnd+1]
				if err := json.Unmarshal([]byte(jsonStr), &plan); err != nil {
					return nil, fmt.Errorf("parsing hook plan JSON: %w", err)
				}
			}
		} else {
			return nil, fmt.Errorf("parsing hook plan: %w", err)
		}
	}

	plan.SpecName = phaseName
	plan.SpecHash = specHash
	plan.Model = p.llmClient.Model()
	plan.CreatedAt = time.Now()

	totalSteps := 0
	for _, phase := range plan.Phases {
		totalSteps += len(phase.Steps)
	}
	log.Infof("PLAN", "Hook plan parsed for %q — %d phases, %d total steps", phaseName, len(plan.Phases), totalSteps)

	// Save to cache
	if p.store != nil {
		if planBytes, err := json.Marshal(plan); err == nil {
			p.store.SavePlan(specHash, planBytes)
		}
	}

	return &plan, nil
}

// generatePlan uses the LLM to create a plan from the spec.
func (p *PlannerAgent) generatePlan(ctx context.Context, testSpec *spec.TestSpec, suiteSpec *spec.SuiteSpec, groupSpec *spec.GroupSpec, fixtures []*fixture.Definition, extraContext string) (*ExecutionPlan, error) {
	// Build a detailed prompt for the planner
	prompt := buildPlannerPrompt(testSpec, suiteSpec, groupSpec, fixtures)

	if extraContext != "" {
		prompt += fmt.Sprintf("\n\nAdditional Context:\n%s\n\nTake the above context into account when generating the execution plan.", extraContext)
	}

	log.Infof("PLAN", "Requesting plan from LLM for %q (setup=%d, steps=%d, verify=%d, cleanup=%d)",
		testSpec.Name, len(testSpec.Setup), len(testSpec.Steps), len(testSpec.Verify), len(testSpec.Cleanup))

	// Call the LLM
	resp, err := p.llmClient.Chat(ctx, p.systemPrompt(), prompt, nil)
	if err != nil {
		log.Errorf("PLAN", "LLM call failed for %q: %v", testSpec.Name, err)
		return nil, fmt.Errorf("llm call failed: %w", err)
	}

	// Extract JSON from response
	content := resp.TextContent()
	log.Infof("PLAN", "LLM response received (%d chars, stop_reason=%s)", len(content), resp.StopReason)

	// Parse the plan from JSON
	var plan ExecutionPlan
	if err := json.Unmarshal([]byte(content), &plan); err != nil {
		log.Debugf("PLAN", "Direct JSON parse failed, attempting extraction: %v", err)
		// Try to extract JSON from the response if it's wrapped in text
		jsonStart := jsonStartIndex(content)
		if jsonStart >= 0 {
			jsonEnd := jsonEndIndex(content, jsonStart)
			if jsonEnd > jsonStart {
				jsonStr := content[jsonStart : jsonEnd+1]
				log.Debugf("PLAN", "Extracted JSON block (%d chars, offset %d..%d)", len(jsonStr), jsonStart, jsonEnd)
				if err := json.Unmarshal([]byte(jsonStr), &plan); err != nil {
					log.Errorf("PLAN", "JSON extraction parse failed: %v", err)
					log.Debugf("PLAN", "Raw LLM response:\n%s", content)
					return nil, fmt.Errorf("parsing plan JSON: %w", err)
				}
			}
		} else {
			log.Errorf("PLAN", "No JSON found in LLM response. Raw response:\n%s", content)
			return nil, fmt.Errorf("parsing plan: %w", err)
		}
	}

	plan.SpecName = testSpec.Name
	plan.CreatedAt = time.Now()

	totalSteps := 0
	for _, phase := range plan.Phases {
		totalSteps += len(phase.Steps)
	}
	log.Infof("PLAN", "Plan parsed — %d phases, %d total steps, %d fixtures",
		len(plan.Phases), totalSteps, len(plan.Fixtures))
	for _, phase := range plan.Phases {
		log.Debugf("PLAN", "  Phase %q: %d steps", phase.Name, len(phase.Steps))
		for j, step := range phase.Steps {
			log.Debugf("PLAN", "    %d. %s (tool=%s)", j+1, step.Description, step.ToolName)
		}
	}

	return &plan, nil
}

// buildPlannerPrompt constructs the prompt for the planner agent.
func buildPlannerPrompt(testSpec *spec.TestSpec, suiteSpec *spec.SuiteSpec, groupSpec *spec.GroupSpec, fixtures []*fixture.Definition) string {
	prompt := fmt.Sprintf(`Test Specification:
Name: %s
File: %s
Author: %s
Priority: %s
CaseID: %s
Labels: %v
Timeout: %v

Setup Steps:
`, testSpec.Name, testSpec.FilePath, testSpec.Metadata.Author, testSpec.Metadata.Priority, testSpec.Metadata.CaseID, testSpec.Metadata.Labels, testSpec.Metadata.Timeout)

	for i, step := range testSpec.Setup {
		prompt += fmt.Sprintf("%d. %s\n", i+1, step.RawText)
	}

	prompt += "\nTest Steps:\n"
	for i, step := range testSpec.Steps {
		prompt += fmt.Sprintf("%d. %s\n", i+1, step.RawText)
	}

	prompt += "\nVerification Steps:\n"
	for i, step := range testSpec.Verify {
		prompt += fmt.Sprintf("%d. %s\n", i+1, step.RawText)
	}

	prompt += "\nCleanup Steps:\n"
	for i, step := range testSpec.Cleanup {
		prompt += fmt.Sprintf("%d. %s\n", i+1, step.RawText)
	}

	if suiteSpec != nil || groupSpec != nil {
		prompt += "\nNOTE: The following suite/group hooks are executed SEPARATELY by the runner before and after this plan."
		prompt += "\nDo NOT include these hook steps in your plan — they are already handled.\n"
		if suiteSpec != nil {
			prompt += fmt.Sprintf("Suite: %s\n", suiteSpec.Name)
		}
		if groupSpec != nil {
			prompt += fmt.Sprintf("Group: %s\n", groupSpec.Name)
		}
	}

	if len(fixtures) > 0 {
		prompt += "\nRequired Fixtures:\n"
		for _, fix := range fixtures {
			prompt += fmt.Sprintf("\n### Fixture: %s\n", fix.Name)
			prompt += fmt.Sprintf("Description: %s\n", fix.Description)

			templates := fix.TemplatePaths()
			if len(templates) > 0 {
				prompt += "Templates:\n"
				for _, t := range templates {
					prompt += fmt.Sprintf("  - %s\n", t)
				}
			}

			if len(fix.Parameters) > 0 {
				prompt += "Parameters:\n"
				for k, v := range fix.Parameters {
					prompt += fmt.Sprintf("  %s: %s\n", k, v)
				}
			}

			prompt += fmt.Sprintf("Create: %s\n", fix.Lifecycle.Create)
			prompt += fmt.Sprintf("Ready check: %s\n", fix.Lifecycle.Ready)
			prompt += fmt.Sprintf("Cleanup: %s\n", fix.Lifecycle.Cleanup)
		}
	}

	prompt += "\nGenerate a detailed execution plan as JSON. For each step, specify the tool to use, parameters, expected results, and validation criteria."

	return prompt
}

// ComputeSpecHash computes a SHA256 hash of the spec file content for caching.
func ComputeSpecHash(ts *spec.TestSpec) string {
	return cache.ComputeSpecHashFromFile(ts.FilePath,
		ts.Name, ts.FilePath, ts.Metadata.Author, ts.Metadata.Priority,
		ts.Metadata.CaseID, fmt.Sprintf("%d|%d|%d|%d",
			len(ts.Setup), len(ts.Steps), len(ts.Verify), len(ts.Cleanup)))
}

// jsonStartIndex finds the start of a JSON object or array in a string.
func jsonStartIndex(s string) int {
	for i, c := range s {
		if c == '{' || c == '[' {
			return i
		}
	}
	return -1
}

// jsonEndIndex finds the end of a JSON object or array starting at startIdx.
func jsonEndIndex(s string, startIdx int) int {
	if startIdx >= len(s) {
		return -1
	}

	depth := 0
	inString := false
	escape := false

	for i := startIdx; i < len(s); i++ {
		c := s[i]

		if escape {
			escape = false
			continue
		}

		if c == '\\' {
			escape = true
			continue
		}

		if c == '"' {
			inString = !inString
			continue
		}

		if inString {
			continue
		}

		if c == '{' || c == '[' {
			depth++
		} else if c == '}' || c == ']' {
			depth--
			if depth == 0 {
				return i
			}
		}
	}

	return -1
}
