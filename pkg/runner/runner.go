package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/sdt-project/sdt/pkg/agent"
	"github.com/sdt-project/sdt/pkg/cache"
	"github.com/sdt-project/sdt/pkg/fixture"
	"github.com/sdt-project/sdt/pkg/llm"
	"github.com/sdt-project/sdt/pkg/log"
	"github.com/sdt-project/sdt/pkg/reporter"
	"github.com/sdt-project/sdt/pkg/spec"
	"github.com/sdt-project/sdt/pkg/tools"
)

// Runner orchestrates the full test lifecycle:
// suite hooks → group hooks → spec setup → fixtures → steps → verify → cleanup → hooks
type Runner struct {
	llmClient      *llm.Client
	toolRegistry   *tools.Registry
	fixtureManager *fixture.Manager
	cacheStore     *cache.Store
	reporter       reporter.Reporter
	defaultTimeout time.Duration
	noCache           bool
	dryRun            bool
	extraContext      string
	skipCleanup       bool
	skipPhases        map[string]bool
	onlyPhases        map[string]bool
	constraints       *tools.ToolConstraints
	systemDescription string
	promptContext     *agent.PromptContext
}

// Config holds runner configuration.
type Config struct {
	LLMClient         *llm.Client
	ToolRegistry      *tools.Registry
	Constraints       *tools.ToolConstraints
	FixtureManager    *fixture.Manager
	CacheStore        *cache.Store
	Reporter          reporter.Reporter
	DefaultTimeout    time.Duration
	NoCache           bool
	DryRun            bool
	ExtraContext      string
	SkipCleanup       bool
	SkipPhases        []string
	OnlyPhases        []string
	SystemDescription string // e.g., "OpenShift cluster", "Kafka cluster", "web application"
	ProjectContext    string // project-specific prompt fragment appended to system prompts
}

// NewRunner creates a runner with the given configuration.
func NewRunner(cfg Config) *Runner {
	timeout := cfg.DefaultTimeout
	if timeout == 0 {
		timeout = 30 * time.Minute
	}
	skip := make(map[string]bool)
	for _, p := range cfg.SkipPhases {
		skip[p] = true
	}
	if cfg.SkipCleanup {
		skip[string(spec.PhaseCleanup)] = true
	}
	only := make(map[string]bool)
	for _, p := range cfg.OnlyPhases {
		only[p] = true
	}
	sysDesc := cfg.SystemDescription
	if sysDesc == "" {
		sysDesc = "target system"
	}

	var pctx *agent.PromptContext
	if cfg.ToolRegistry != nil || cfg.Constraints != nil {
		pctx = &agent.PromptContext{
			Registry:       cfg.ToolRegistry,
			Constraints:    cfg.Constraints,
			ProjectContext: cfg.ProjectContext,
		}
	}

	return &Runner{
		llmClient:         cfg.LLMClient,
		toolRegistry:      cfg.ToolRegistry,
		fixtureManager:    cfg.FixtureManager,
		cacheStore:        cfg.CacheStore,
		reporter:          cfg.Reporter,
		defaultTimeout:    timeout,
		noCache:           cfg.NoCache,
		dryRun:            cfg.DryRun,
		extraContext:      cfg.ExtraContext,
		skipCleanup:       cfg.SkipCleanup,
		skipPhases:        skip,
		onlyPhases:        only,
		constraints:       cfg.Constraints,
		systemDescription: sysDesc,
		promptContext:     pctx,
	}
}

// shouldRunPhase returns true if the given phase should be executed
// based on --skip-phases and --only-phases configuration.
func (r *Runner) shouldRunPhase(phase string) bool {
	if r.skipPhases[phase] {
		log.Infof("RUN", "Skipping phase %q (--skip-phases)", phase)
		return false
	}
	if len(r.onlyPhases) > 0 && !r.onlyPhases[phase] {
		log.Infof("RUN", "Skipping phase %q (not in --only-phases)", phase)
		return false
	}
	return true
}

// RunSuite executes a full suite: suite hooks, group hooks, and all test specs.
func (r *Runner) RunSuite(ctx context.Context, suite *spec.Suite) *reporter.SuiteReport {
	suiteStart := time.Now()
	suiteName := "default"
	if suite.SuiteSpec != nil {
		suiteName = suite.SuiteSpec.Name
	}

	r.reporter.StartSuite(suiteName)

	suiteReport := &reporter.SuiteReport{
		Name: suiteName,
		Dir:  suite.Dir,
	}

	// Phase: Pre-Suite (once) — run hooks then validate
	preSuiteFailed := false
	if suite.SuiteSpec != nil && len(suite.SuiteSpec.PreSuite) > 0 && r.shouldRunPhase("pre-suite") {
		_ = r.runHookPhase(ctx, "pre-suite", suite.SuiteSpec.PreSuite)

		// Validate pre-suite: if validation steps exist, they determine success/failure
		if len(suite.SuiteSpec.PreSuiteValidation) > 0 && r.shouldRunPhase("pre-suite-validation") {
			if err := r.runValidation(ctx, "pre-suite-validation", suite.SuiteSpec.PreSuiteValidation); err != nil {
				log.Errorf("RUN", "Pre-suite validation failed — skipping all tests: %v", err)
				preSuiteFailed = true
			}
		}
	}

	// Execute each test spec
	for _, testSpec := range suite.Tests {
		if preSuiteFailed {
			testReport := &reporter.TestReport{
				Name:     testSpec.TestName(),
				FilePath: testSpec.FilePath,
				Status:   reporter.TestSkipped,
				Error:    "skipped: pre-suite hook failed",
			}
			suiteReport.Tests = append(suiteReport.Tests, testReport)
			suiteReport.Skipped++
			r.reporter.StartSpec(testSpec.TestName())
			r.reporter.EndSpec(testReport)
			continue
		}
		testReport := r.RunSpec(ctx, testSpec, suite.SuiteSpec, suite.Groups)
		suiteReport.Tests = append(suiteReport.Tests, testReport)

		switch testReport.Status {
		case reporter.TestPassed:
			suiteReport.Passed++
		case reporter.TestFailed:
			suiteReport.Failed++
		case reporter.TestSkipped:
			suiteReport.Skipped++
		}
	}

	// Phase: Post-Suite (once)
	if suite.SuiteSpec != nil && len(suite.SuiteSpec.PostSuite) > 0 && r.shouldRunPhase("post-suite") {
		_ = r.runHookPhase(ctx, "post-suite", suite.SuiteSpec.PostSuite)
	}

	suiteReport.Duration = time.Since(suiteStart)
	r.reporter.EndSuite(suiteReport)
	return suiteReport
}

// RunSpec executes a single test spec through its full lifecycle.
func (r *Runner) RunSpec(ctx context.Context, testSpec *spec.TestSpec, suiteSpec *spec.SuiteSpec, groups map[string]*spec.GroupSpec) *reporter.TestReport {
	specStart := time.Now()
	testName := testSpec.TestName()

	r.reporter.StartSpec(testName)

	// Determine timeout for the test itself (hooks get their own timeout)
	timeout := r.defaultTimeout
	if testSpec.Metadata.Timeout > 0 {
		timeout = testSpec.Metadata.Timeout
	}

	testReport := &reporter.TestReport{
		Name:     testName,
		FilePath: testSpec.FilePath,
		Status:   reporter.TestPassed, // Assume pass until failure
		Metadata: map[string]string{
			"author":   testSpec.Metadata.Author,
			"priority": testSpec.Metadata.Priority,
			"caseID":   testSpec.Metadata.CaseID,
		},
		CaseID: testSpec.Metadata.CaseID,
	}

	log.Infof("RUN", "Spec %q — timeout=%s, group=%q, fixtures=%v",
		testName, timeout, testSpec.Metadata.Group, testSpec.Metadata.Fixtures)

	// --- Pre-test hooks run with their own timeout (not the spec timeout) ---
	hookCtx, hookCancel := context.WithTimeout(ctx, 15*time.Minute)
	defer hookCancel()

	// --- Suite Pre-Test hooks ---
	hookFailed := false
	if suiteSpec != nil && len(suiteSpec.PreTest) > 0 && r.shouldRunPhase("suite-pre-test") {
		_ = r.runHookPhase(hookCtx, "suite-pre-test", suiteSpec.PreTest)

		// Validate suite pre-test
		if len(suiteSpec.PreTestValidation) > 0 && r.shouldRunPhase("suite-pre-test-validation") {
			if err := r.runValidation(hookCtx, "suite-pre-test-validation", suiteSpec.PreTestValidation); err != nil {
				testReport.Status = reporter.TestFailed
				testReport.Error = fmt.Sprintf("suite-pre-test validation: %s", err)
				hookFailed = true
			}
		}
	}

	// --- Group Pre-Test hooks ---
	if !hookFailed && testSpec.Metadata.Group != "" {
		if groupSpec, ok := groups[testSpec.Metadata.Group]; ok && len(groupSpec.PreTest) > 0 && r.shouldRunPhase("group-pre-test") {
			_ = r.runHookPhase(hookCtx, "group-pre-test", groupSpec.PreTest)

			// Validate group pre-test
			if len(groupSpec.PreTestValidation) > 0 && r.shouldRunPhase("group-pre-test-validation") {
				if err := r.runValidation(hookCtx, "group-pre-test-validation", groupSpec.PreTestValidation); err != nil {
					testReport.Status = reporter.TestFailed
					testReport.Error = fmt.Sprintf("group-pre-test validation: %s", err)
					hookFailed = true
				}
			}
		}
	}
	hookCancel()

	// If pre-test validation failed, diagnose and skip planning/execution
	if hookFailed {
		log.Errorf("RUN", "Skipping spec %q — pre-test validation failed: %s", testName, testReport.Error)
		testReport.Diagnosis = r.diagnoseFailure(ctx, testName, testReport.Error)
	}

	if !hookFailed {
		// Create spec timeout for planning + execution only (hooks already ran with their own timeout)
		specCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		// Create agents
		memoryAgent := agent.NewMemoryAgent(r.cacheStore)
		plannerAgent := agent.NewPlannerAgent(r.llmClient, r.cacheStore).WithPromptContext(r.promptContext)
		executorAgent := agent.NewExecutorAgent(r.llmClient, r.toolRegistry).WithPromptContext(r.promptContext)

		// --- Resolve fixtures ---
		var fixtureDefs []*fixture.Definition
		if len(testSpec.Metadata.Fixtures) > 0 && r.fixtureManager != nil {
			var err error
			fixtureDefs, err = r.fixtureManager.Resolve(testSpec.Metadata.Fixtures)
			if err != nil {
				testReport.Status = reporter.TestFailed
				testReport.Error = fmt.Sprintf("resolving fixtures: %s", err)
				r.reporter.EndSpec(testReport)
				return testReport
			}
		}

		// --- Plan Mode ---
		var plan *agent.ExecutionPlan
		var groupSpec *spec.GroupSpec
		if testSpec.Metadata.Group != "" {
			groupSpec = groups[testSpec.Metadata.Group]
		}

		// Check cache
		if !r.noCache {
			if cached, ok := memoryAgent.GetCachedPlan(testSpec); ok {
				plan = cached
				log.Infof("RUN", "Using cached plan for %q", testName)
			}
		}

		// Generate plan if not cached
		if plan == nil {
			log.Infof("RUN", "Generating new plan for %q", testName)
			var err error
			plan, err = plannerAgent.Plan(specCtx, testSpec, suiteSpec, groupSpec, fixtureDefs, r.extraContext)
			if err != nil {
				log.Errorf("RUN", "Planning FAILED for %q: %v", testName, err)
				testReport.Status = reporter.TestFailed
				testReport.Error = fmt.Sprintf("planning: %s", err)
				r.reporter.EndSpec(testReport)
				return testReport
			}
			log.Infof("RUN", "Plan generated for %q — %d phases", testName, len(plan.Phases))
			// Cache the plan
			memoryAgent.SavePlan(testSpec, plan)
		}

		// Report plan
		for _, phase := range plan.Phases {
			for i, step := range phase.Steps {
				r.reporter.StepResult(phase.Name, i, step.Description, reporter.StepOutcome{
					Status: reporter.StepPassed,
					Output: fmt.Sprintf("[PLAN] %s → %s", step.Description, step.ToolName),
				})
			}
		}

		// Filter plan phases based on --skip-phases / --only-phases
		if len(r.skipPhases) > 0 || len(r.onlyPhases) > 0 {
			var filtered []agent.PlanPhase
			for _, phase := range plan.Phases {
				if r.shouldRunPhase(phase.Name) {
					filtered = append(filtered, phase)
				}
			}
			plan.Phases = filtered
		}

		// --- Dry Run: stop here ---
		if r.dryRun {
			testReport.Status = reporter.TestSkipped
			testReport.Output = "Dry run — plan generated, execution skipped"
			testReport.Duration = time.Since(specStart)
			r.reporter.EndSpec(testReport)
			return testReport
		}

		// --- Auto-Pilot Execution ---
		log.Infof("RUN", "Starting auto-pilot execution for %q", testName)
		result, err := executorAgent.Execute(specCtx, plan, r.extraContext, true)
		if err != nil {
			log.Errorf("RUN", "Execution error for %q: %v", testName, err)
			testReport.Status = reporter.TestFailed
			testReport.Error = fmt.Sprintf("execution: %s", err)
			testReport.Diagnosis = r.diagnoseFailure(specCtx, testName, testReport.Error)
		} else if result != nil {
			// Convert execution result to step reports
			for _, pr := range result.PhaseResults {
				for i, sr := range pr.StepResults {
					outcome := reporter.StepOutcome{
						Duration: sr.Duration,
						Output:   sr.Output,
					}
					if sr.Error != "" {
						outcome.Status = reporter.StepFailed
						outcome.Error = sr.Error
						testReport.Status = reporter.TestFailed
						if testReport.Error == "" {
							testReport.Error = sr.Error
						}
					} else {
						outcome.Status = reporter.StepPassed
					}
					r.reporter.StepResult(pr.Phase, i, sr.Description, outcome)
					testReport.Steps = append(testReport.Steps, reporter.StepReport{
						Phase:   pr.Phase,
						Index:   i,
						Text:    sr.Description,
						Outcome: outcome,
					})
				}
			}
		}

		// Diagnose if execution completed but had step failures
		if testReport.Status == reporter.TestFailed && testReport.Diagnosis == "" {
			testReport.Diagnosis = r.diagnoseFailure(ctx, testName, testReport.Error)
		}

		// Save result to history
		if r.cacheStore != nil {
			memoryAgent.SaveResult(testSpec, result)
		}
	}
	// --- Group Post-Test hooks (use parent context — specCtx may be expired) ---
	cleanupCtx, cleanupCancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cleanupCancel()

	if testSpec.Metadata.Group != "" {
		if gs, ok := groups[testSpec.Metadata.Group]; ok && len(gs.PostTest) > 0 && r.shouldRunPhase("group-post-test") {
			_ = r.runHookPhase(cleanupCtx, "group-post-test", gs.PostTest)
		}
	}

	// --- Suite Post-Test hooks ---
	if suiteSpec != nil && len(suiteSpec.PostTest) > 0 && r.shouldRunPhase("suite-post-test") {
		_ = r.runHookPhase(cleanupCtx, "suite-post-test", suiteSpec.PostTest)
	}

	testReport.Duration = time.Since(specStart)
	r.reporter.EndSpec(testReport)
	return testReport
}

// diagnoseFailure uses the LLM with cluster tools to investigate a test failure.
// It returns a diagnosis string with root cause analysis and suggestions.
func (r *Runner) diagnoseFailure(ctx context.Context, testName string, failureError string) string {
	if r.dryRun {
		return ""
	}

	log.Infof("DEBUG", "Starting auto-debug for %q — error: %s", testName, failureError)

	diagCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	prompt := fmt.Sprintf(`A test step has failed. Investigate the root cause.

Test: %s
Error: %s

Use the available tools to check cluster state and identify why this failed.
After investigating, provide your diagnosis as plain text (not JSON).`, testName, failureError)

	if r.extraContext != "" {
		prompt += fmt.Sprintf("\n\nAdditional Context:\n%s", r.extraContext)
	}

	toolDefs := r.toolRegistry.LLMToolDefinitions()
	var llmTools []llm.ToolDefinition
	for _, td := range toolDefs {
		llmTools = append(llmTools, llm.ToolDefinition{
			Name:        td.Name,
			Description: td.Description,
			InputSchema: td.InputSchema,
		})
	}

	systemPrompt := agent.BuildSystemPrompt(agent.DiagnosticSystemPrompt, r.promptContext)

	req := &llm.Request{
		System: systemPrompt,
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

	toolHandler := r.toolRegistry.HandleToolCall(diagCtx)
	resp, _, err := r.llmClient.RunAgentLoop(diagCtx, req, toolHandler, false)
	if err != nil {
		log.Warnf("DEBUG", "Auto-debug failed for %q: %v", testName, err)
		return fmt.Sprintf("Auto-debug failed: %s", err)
	}

	diagnosis := ""
	if resp != nil {
		diagnosis = resp.TextContent()
	}

	log.Infof("DEBUG", "Auto-debug completed for %q (%d chars)", testName, len(diagnosis))
	return diagnosis
}

// runHookPhase executes hook steps (pre-suite, pre-test, post-test, post-suite).
// It first generates (or retrieves from cache) an execution plan for the hook steps,
// then executes the plan via the executor agent. This ensures hooks produce consistent
// plans across runs for the same spec.
// Returns an error if the hook phase failed.
func (r *Runner) runHookPhase(ctx context.Context, phaseName string, steps []spec.StepDef) error {
	log.Infof("HOOK", "Starting %s phase (%d steps)", phaseName, len(steps))
	for i, step := range steps {
		log.Debugf("HOOK", "  Step %d/%d: %s", i+1, len(steps), step.RawText)
	}

	if r.dryRun {
		log.Infof("HOOK", "Dry-run mode — skipping execution of %s", phaseName)
		for _, step := range steps {
			r.reporter.StepResult(phaseName, step.Index, step.RawText, reporter.StepOutcome{
				Status: reporter.StepPassed,
				Output: fmt.Sprintf("[DRY-RUN] %s: %s", phaseName, step.RawText),
			})
		}
		return nil
	}

	hookStart := time.Now()

	// Phase 1: Plan (with caching)
	plannerAgent := agent.NewPlannerAgent(r.llmClient, r.cacheStore).WithPromptContext(r.promptContext)
	plan, err := plannerAgent.PlanHooks(ctx, phaseName, steps, r.extraContext)
	if err != nil {
		log.Errorf("HOOK", "%s planning failed after %s: %v", phaseName, time.Since(hookStart), err)
		r.reporter.StepResult(phaseName, 0, fmt.Sprintf("%s hook", phaseName), reporter.StepOutcome{
			Status: reporter.StepFailed,
			Error:  fmt.Sprintf("hook planning failed: %v", err),
		})
		return fmt.Errorf("%s planning failed: %w", phaseName, err)
	}

	// Report planned steps
	totalSteps := 0
	for _, phase := range plan.Phases {
		totalSteps += len(phase.Steps)
	}
	log.Infof("HOOK", "%s plan ready — %d phases, %d steps", phaseName, len(plan.Phases), totalSteps)

	// Phase 2: Execute the plan
	executorAgent := agent.NewExecutorAgent(r.llmClient, r.toolRegistry).WithPromptContext(r.promptContext)
	result, err := executorAgent.Execute(ctx, plan, r.extraContext, false)
	if err != nil {
		log.Errorf("HOOK", "%s execution FAILED after %s: %v", phaseName, time.Since(hookStart), err)
		r.reporter.StepResult(phaseName, 0, fmt.Sprintf("%s hook", phaseName), reporter.StepOutcome{
			Status: reporter.StepFailed,
			Error:  fmt.Sprintf("hook execution failed: %v", err),
		})
		return fmt.Errorf("%s failed: %w", phaseName, err)
	}

	// Report results
	passed, failed := 0, 0
	if result != nil {
		for _, pr := range result.PhaseResults {
			for i, sr := range pr.StepResults {
				outcome := reporter.StepOutcome{
					Duration: sr.Duration,
					Output:   sr.Output,
				}
				desc := sr.Description
				if desc == "" {
					desc = sr.ToolName
				}
				if sr.Error != "" {
					outcome.Status = reporter.StepFailed
					outcome.Error = sr.Error
					failed++
				} else {
					outcome.Status = reporter.StepPassed
					passed++
				}
				r.reporter.StepResult(phaseName, i, desc, outcome)
			}
		}
	}

	log.Infof("HOOK", "%s phase completed in %s — %d tool calls (passed=%d, failed=%d)",
		phaseName, time.Since(hookStart), passed+failed, passed, failed)

	return nil
}

// runValidation executes validation checks after a hook phase.
// Each validation step is a condition that must be true (e.g., "namespace X exists").
// The LLM checks each condition using cluster tools. If any check fails, validation fails.
// Unlike hooks, validation uses stopOnError=true — a failed check means the setup is broken.
func (r *Runner) runValidation(ctx context.Context, phaseName string, steps []spec.StepDef) error {
	log.Infof("VALIDATE", "Starting %s (%d checks)", phaseName, len(steps))
	for i, step := range steps {
		log.Debugf("VALIDATE", "  Check %d/%d: %s", i+1, len(steps), step.RawText)
	}

	if r.dryRun {
		log.Infof("VALIDATE", "Dry-run mode — skipping %s", phaseName)
		for _, step := range steps {
			r.reporter.StepResult(phaseName, step.Index, step.RawText, reporter.StepOutcome{
				Status: reporter.StepPassed,
				Output: fmt.Sprintf("[DRY-RUN] %s: %s", phaseName, step.RawText),
			})
		}
		return nil
	}

	validationStart := time.Now()

	var checkList string
	for i, step := range steps {
		checkList += fmt.Sprintf("%d. %s\n", i+1, step.RawText)
	}

	prompt := fmt.Sprintf(`Validate the following conditions against the %s.
Each condition MUST be true. Check each one and report PASS or FAIL.

%s

For each condition, use the available tools to verify it. If ANY condition fails, clearly state which one failed and why.`, r.systemDescription, checkList)

	if r.extraContext != "" {
		prompt += fmt.Sprintf("\n\nAdditional Context:\n%s", r.extraContext)
	}

	toolDefs := r.toolRegistry.LLMToolDefinitions()
	var llmTools []llm.ToolDefinition
	for _, td := range toolDefs {
		llmTools = append(llmTools, llm.ToolDefinition{
			Name:        td.Name,
			Description: td.Description,
			InputSchema: td.InputSchema,
		})
	}

	systemPrompt := agent.BuildSystemPrompt(agent.ExecutorSystemPrompt, r.promptContext)

	req := &llm.Request{
		System: systemPrompt,
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

	toolHandler := r.toolRegistry.HandleToolCall(ctx)
	_, toolLogs, err := r.llmClient.RunAgentLoop(ctx, req, toolHandler, true)
	if err != nil {
		log.Errorf("VALIDATE", "%s FAILED after %s: %v", phaseName, time.Since(validationStart), err)
		r.reporter.StepResult(phaseName, 0, fmt.Sprintf("%s check", phaseName), reporter.StepOutcome{
			Status: reporter.StepFailed,
			Error:  fmt.Sprintf("validation failed: %v", err),
		})
		return fmt.Errorf("%s failed: %w", phaseName, err)
	}

	passed, failed := 0, 0
	for i, tl := range toolLogs {
		outcome := reporter.StepOutcome{
			Duration: tl.Duration,
			Output:   tl.Output,
		}
		desc := tl.ToolName
		if tl.Error != "" {
			outcome.Status = reporter.StepFailed
			outcome.Error = tl.Error
			failed++
		} else {
			outcome.Status = reporter.StepPassed
			passed++
		}
		r.reporter.StepResult(phaseName, i, desc, outcome)
	}

	log.Infof("VALIDATE", "%s completed in %s — %d checks (passed=%d, failed=%d)",
		phaseName, time.Since(validationStart), passed+failed, passed, failed)

	if failed > 0 {
		return fmt.Errorf("%s: %d check(s) failed", phaseName, failed)
	}
	return nil
}
