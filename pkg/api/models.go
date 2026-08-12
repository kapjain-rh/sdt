package api

import "time"

type Project struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type TestCase struct {
	ID            int64     `json:"id"`
	ProjectID     int64     `json:"project_id"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	Preconditions string    `json:"preconditions"`
	Setup         []string  `json:"setup"`
	Steps         []string  `json:"steps"`
	Verify        []string  `json:"verify"`
	Cleanup       []string  `json:"cleanup"`
	Priority      string    `json:"priority"`
	Status        string    `json:"status"`
	Author        string    `json:"author"`
	Labels        string    `json:"labels"`
	Group         string    `json:"group,omitempty"`
	Fixtures      []string  `json:"fixtures,omitempty"`
	DependsOn     []string  `json:"depends_on,omitempty"`
	Timeout       string    `json:"timeout,omitempty"`
	CaseID        string    `json:"case_id,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type SuiteInfo struct {
	Name               string   `json:"name"`
	FilePath           string   `json:"file_path"`
	Timeout            string   `json:"timeout,omitempty"`
	PreSuite           []string `json:"pre_suite"`
	PreSuiteValidation []string `json:"pre_suite_validation"`
	PreTest            []string `json:"pre_test"`
	PreTestValidation  []string `json:"pre_test_validation"`
	PostTest           []string `json:"post_test"`
	PostSuite          []string `json:"post_suite"`
}

type GroupInfo struct {
	Name              string   `json:"name"`
	FilePath          string   `json:"file_path"`
	Timeout           string   `json:"timeout,omitempty"`
	PreTest           []string `json:"pre_test"`
	PreTestValidation []string `json:"pre_test_validation"`
	PostTest          []string `json:"post_test"`
	Specs             []string `json:"specs"`
}

type Fixture struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Status      string            `json:"status"`
	Templates   []string          `json:"templates"`
	Parameters  map[string]string `json:"parameters"`
	Lifecycle   FixtureLifecycle  `json:"lifecycle"`
}

type FixtureLifecycle struct {
	Create  string `json:"create"`
	Ready   string `json:"ready"`
	Cleanup string `json:"cleanup"`
}

type TestPlan struct {
	ID          int64     `json:"id"`
	ProjectID   int64     `json:"project_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Milestone   string    `json:"milestone"`
	Status      string    `json:"status"`
	CaseCount   int       `json:"case_count,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type TestRun struct {
	ID          int64             `json:"id"`
	PlanID      int64             `json:"plan_id"`
	PlanName    string            `json:"plan_name,omitempty"`
	Name        string            `json:"name"`
	Build       string            `json:"build"`
	Environment string            `json:"environment"`
	EnvVars     map[string]string `json:"env_vars,omitempty"`
	Status      string            `json:"status"`
	StartedAt   *time.Time        `json:"started_at"`
	CompletedAt *time.Time        `json:"completed_at"`
	CreatedAt   time.Time         `json:"created_at"`
	Total       int               `json:"total"`
	Passed      int               `json:"passed"`
	Failed      int               `json:"failed"`
	Blocked     int               `json:"blocked"`
	Skipped     int               `json:"skipped"`
}

type TestStepResult struct {
	ID           int64  `json:"id"`
	ResultID     int64  `json:"result_id"`
	StepIndex    int    `json:"step_index"`
	Status       string `json:"status"`
	ActualResult string `json:"actual_result"`
	Comment      string `json:"comment"`
}

type TestResult struct {
	ID          int64            `json:"id"`
	RunID       int64            `json:"run_id"`
	CaseID      int64            `json:"case_id"`
	Status      string           `json:"status"`
	Comment     string           `json:"comment"`
	ExecutedBy  string           `json:"executed_by"`
	DurationMs  int64            `json:"duration_ms"`
	ExecutedAt  *time.Time       `json:"executed_at"`
	StepResults []TestStepResult `json:"step_results,omitempty"`
	Case        *TestCase        `json:"case,omitempty"`
}

type Execution struct {
	ID         int64             `json:"id"`
	CaseID     int64             `json:"case_id"`
	Status     string            `json:"status"`
	ExecutedBy string            `json:"executed_by"`
	DurationMs int64             `json:"duration_ms"`
	StartedAt  *time.Time        `json:"started_at"`
	FinishedAt *time.Time        `json:"finished_at"`
	Verdict    string            `json:"verdict"`
	EnvVars    map[string]string `json:"env_vars,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
}

type ExecutionLog struct {
	ID          int64     `json:"id"`
	ExecutionID int64     `json:"execution_id"`
	StepIndex   int       `json:"step_index"`
	LogType     string    `json:"log_type"`
	Message     string    `json:"message"`
	Timestamp   time.Time `json:"timestamp"`
}

type ToolParam struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Default     string `json:"default,omitempty"`
}

type Tool struct {
	ID          int64                `json:"id"`
	ProjectID   int64                `json:"project_id"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Command     string               `json:"command"`
	Args        []string             `json:"args"`
	Env         map[string]string    `json:"env"`
	InputParams map[string]ToolParam `json:"input_params"`
	Category    string               `json:"category"`
	Status      string               `json:"status"`
	Author      string               `json:"author"`
	CreatedAt   time.Time            `json:"created_at"`
	UpdatedAt   time.Time            `json:"updated_at"`
}

type ToolRun struct {
	ID          int64      `json:"id"`
	ToolID      int64      `json:"tool_id"`
	MCPServerID int64      `json:"mcp_server_id,omitempty"`
	MCPToolName string     `json:"mcp_tool_name,omitempty"`
	Status      string     `json:"status"`
	ExitCode    int        `json:"exit_code"`
	DurationMs  int64      `json:"duration_ms"`
	StartedAt   *time.Time `json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

type ToolRunLog struct {
	ID        int64     `json:"id"`
	RunID     int64     `json:"run_id"`
	Stream    string    `json:"stream"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

type MCPServer struct {
	ID        int64             `json:"id"`
	ProjectID int64             `json:"project_id"`
	Name      string            `json:"name"`
	Command   string            `json:"command"`
	Args      []string          `json:"args"`
	Env       map[string]string `json:"env"`
	Status    string            `json:"status"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

type DashboardStats struct {
	Projects   int `json:"projects"`
	TotalCases int `json:"total_cases"`
	TotalPlans int `json:"total_plans"`
	TotalRuns  int `json:"total_runs"`
	ActiveRuns int `json:"active_runs"`
}

type TrendPoint struct {
	Date    string `json:"date"`
	Passed  int    `json:"passed"`
	Failed  int    `json:"failed"`
	Blocked int    `json:"blocked"`
	Skipped int    `json:"skipped"`
	Total   int    `json:"total"`
}
