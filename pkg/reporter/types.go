package reporter

import "time"

// StepStatus represents the outcome of a single step.
type StepStatus string

const (
	StepPassed  StepStatus = "PASSED"
	StepFailed  StepStatus = "FAILED"
	StepSkipped StepStatus = "SKIPPED"
)

// TestStatus represents the overall outcome of a test spec.
type TestStatus string

const (
	TestPassed  TestStatus = "passed"
	TestFailed  TestStatus = "failed"
	TestSkipped TestStatus = "skipped"
)

// StepOutcome captures the result of a single step execution.
type StepOutcome struct {
	Status    StepStatus
	Duration  time.Duration
	Output    string // Human-readable output from the step
	Error     string // Error message if failed
	Thinking  string // LLM reasoning trace (if available)
	ToolCalls []ToolCallRecord
}

// ToolCallRecord tracks a single MCP tool invocation within a step.
type ToolCallRecord struct {
	ToolName   string
	Parameters map[string]string
	Output     string
	Error      string
	Duration   time.Duration
}

// StepReport holds the full report for a single step.
type StepReport struct {
	Phase   string // e.g., "setup", "steps", "verify", "cleanup"
	Index   int
	Text    string // Original step text from the spec
	Outcome StepOutcome
}

// TestReport holds the full report for a single test spec execution.
type TestReport struct {
	Name      string
	FilePath  string
	Status    TestStatus
	Duration  time.Duration
	Steps     []StepReport
	Error     string            // Top-level error message if failed
	Output    string            // Aggregated output
	Diagnosis string            // Auto-debug analysis when test fails
	Metadata  map[string]string // Spec metadata (author, priority, caseID, etc.)
	CaseID    string            // Kiwi TCMS case ID for reporting
	Retries   int               // Number of retries needed (0 = passed first try)
	Flaky     bool              // True if spec failed initially but passed on retry
}

// TokenUsage tracks LLM token consumption and estimated cost.
type TokenUsage struct {
	InputTokens   int     `json:"input_tokens"`
	OutputTokens  int     `json:"output_tokens"`
	TotalTokens   int     `json:"total_tokens"`
	EstimatedCost float64 `json:"estimated_cost"`
	Requests      int     `json:"requests"`
}

// SuiteReport holds the full report for an entire suite execution.
type SuiteReport struct {
	Name       string
	Dir        string
	Tests      []*TestReport
	Duration   time.Duration
	Passed     int
	Failed     int
	Skipped    int
	TokenUsage *TokenUsage
}

// Reporter is the interface for outputting test results.
type Reporter interface {
	// StartSuite is called before a suite begins execution.
	StartSuite(name string)

	// StartSpec is called before a spec begins execution.
	StartSpec(name string)

	// StepResult is called after each step completes.
	StepResult(phase string, stepIndex int, stepText string, outcome StepOutcome)

	// EndSpec is called after a spec completes (including cleanup).
	EndSpec(report *TestReport)

	// EndSuite is called after all specs complete.
	EndSuite(report *SuiteReport)

	// Finalize writes final output (e.g., JUnit XML files) and returns any error.
	Finalize(report *SuiteReport) error
}

// MultiReporter fans out to multiple reporters.
type MultiReporter struct {
	reporters []Reporter
}

// NewMultiReporter creates a reporter that delegates to all provided reporters.
func NewMultiReporter(reporters ...Reporter) *MultiReporter {
	return &MultiReporter{reporters: reporters}
}

func (m *MultiReporter) StartSuite(name string) {
	for _, r := range m.reporters {
		r.StartSuite(name)
	}
}

func (m *MultiReporter) StartSpec(name string) {
	for _, r := range m.reporters {
		r.StartSpec(name)
	}
}

func (m *MultiReporter) StepResult(phase string, stepIndex int, stepText string, outcome StepOutcome) {
	for _, r := range m.reporters {
		r.StepResult(phase, stepIndex, stepText, outcome)
	}
}

func (m *MultiReporter) EndSpec(report *TestReport) {
	for _, r := range m.reporters {
		r.EndSpec(report)
	}
}

func (m *MultiReporter) EndSuite(report *SuiteReport) {
	for _, r := range m.reporters {
		r.EndSuite(report)
	}
}

func (m *MultiReporter) Finalize(report *SuiteReport) error {
	for _, r := range m.reporters {
		if err := r.Finalize(report); err != nil {
			return err
		}
	}
	return nil
}
