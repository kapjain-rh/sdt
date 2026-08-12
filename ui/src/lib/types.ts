export interface Project {
  id: number;
  name: string;
  description: string;
  created_at: string;
  updated_at: string;
}

export interface TestCase {
  id: number;
  project_id: number;
  title: string;
  description: string;
  preconditions: string;
  setup: string[] | null;
  steps: string[] | null;
  verify: string[] | null;
  cleanup: string[] | null;
  priority: "Critical" | "High" | "Medium" | "Low";
  status: "active" | "draft" | "deprecated" | "approved";
  author: string;
  labels: string;
  group?: string;
  fixtures?: string[];
  depends_on?: string[];
  timeout?: string;
  case_id?: string;
  created_at: string;
  updated_at: string;
}

export interface SuiteInfo {
  name: string;
  file_path: string;
  timeout?: string;
  pre_suite: string[] | null;
  pre_suite_validation: string[] | null;
  pre_test: string[] | null;
  pre_test_validation: string[] | null;
  post_test: string[] | null;
  post_suite: string[] | null;
}

export interface GroupInfo {
  name: string;
  file_path: string;
  timeout?: string;
  pre_test: string[] | null;
  pre_test_validation: string[] | null;
  post_test: string[] | null;
  specs: string[] | null;
}

export interface Fixture {
  name: string;
  description: string;
  status: string;
  templates: string[] | null;
  parameters: Record<string, string>;
  lifecycle: {
    create: string;
    ready: string;
    cleanup: string;
  };
}

export interface TestPlan {
  id: number;
  project_id: number;
  name: string;
  description: string;
  milestone: string;
  status: "active" | "draft" | "completed";
  case_count: number;
  created_at: string;
  updated_at: string;
}

export interface TestRun {
  id: number;
  plan_id: number;
  plan_name: string;
  name: string;
  build: string;
  environment: string;
  env_vars?: Record<string, string>;
  status: "not_started" | "in_progress" | "completed";
  started_at: string | null;
  completed_at: string | null;
  created_at: string;
  total: number;
  passed: number;
  failed: number;
  blocked: number;
  skipped: number;
}

export interface TestStepResult {
  id: number;
  result_id: number;
  step_index: number;
  status: "untested" | "passed" | "failed" | "skipped";
  actual_result: string;
  comment: string;
}

export interface TestResult {
  id: number;
  run_id: number;
  case_id: number;
  status: "untested" | "passed" | "failed" | "blocked" | "skipped";
  comment: string;
  executed_by: string;
  duration_ms: number;
  executed_at: string | null;
  case: TestCase;
  step_results: TestStepResult[] | null;
}

export interface DashboardStats {
  projects: number;
  total_cases: number;
  total_plans: number;
  total_runs: number;
  active_runs: number;
}

export interface TrendPoint {
  date: string;
  passed: number;
  failed: number;
  blocked: number;
  skipped: number;
  total: number;
}

export interface ToolParam {
  type: "string" | "number" | "boolean";
  description: string;
  required: boolean;
  default?: string;
}

export interface Tool {
  id: number;
  project_id: number;
  name: string;
  description: string;
  command: string;
  args: string[];
  env: Record<string, string>;
  input_params: Record<string, ToolParam>;
  category: string;
  status: "draft" | "verify" | "approved";
  author: string;
  created_at: string;
  updated_at: string;
}

export interface ToolRun {
  id: number;
  tool_id: number;
  mcp_server_id?: number;
  mcp_tool_name?: string;
  status: "pending" | "running" | "passed" | "failed" | "error";
  exit_code: number;
  duration_ms: number;
  started_at: string | null;
  finished_at: string | null;
  created_at: string;
}

export interface ToolRunLog {
  id: number;
  run_id: number;
  stream: "stdout" | "stderr" | "system";
  message: string;
  timestamp: string;
}

export interface MCPServer {
  id: number;
  project_id: number;
  name: string;
  command: string;
  args: string[];
  env: Record<string, string>;
  status: "configured" | "connected" | "error";
  created_at: string;
  updated_at: string;
}

export interface MCPDiscoveredTool {
  name: string;
  description: string;
  inputSchema: Record<string, unknown>;
}

export interface Execution {
  id: number;
  case_id: number;
  status: "pending" | "running" | "passed" | "failed" | "error";
  executed_by: string;
  duration_ms: number;
  verdict: string;
  env_vars?: Record<string, string>;
  started_at: string | null;
  finished_at: string | null;
  created_at: string;
}

export interface ExecutionLog {
  id: number;
  execution_id: number;
  step_index: number;
  log_type: string;
  message: string;
  timestamp: string;
}

export interface CachedPlanStep {
  description: string;
  tool_name: string;
  parameters: Record<string, unknown>;
  expected_result: string;
  validation: unknown;
  on_failure: string;
}

export interface CachedPlanPhase {
  name: string;
  steps: CachedPlanStep[];
}

export interface CacheFixturePlan {
  name: string;
  template: string;
  parameters: Record<string, unknown>;
  create: CachedPlanStep[];
  ready_check: CachedPlanStep[];
  cleanup: CachedPlanStep[];
}

export interface CachedPlan {
  spec_hash: string;
  spec_name: string;
  created_at: string;
  model: string;
  phases: CachedPlanPhase[];
  fixtures: CacheFixturePlan[];
  file_size: number;
  file_name: string;
}

export interface CacheStepResult {
  description: string;
  tool_name: string;
  status: string;
  output: string;
  error?: string;
  duration: number;
}

export interface CachePhaseResult {
  phase: string;
  status: string;
  step_results: CacheStepResult[];
  error?: string;
}

export interface CacheFixtureResult {
  name: string;
  status: string;
  create: CacheStepResult[];
  ready_check: CacheStepResult[];
  cleanup: CacheStepResult[];
  error?: string;
}

export interface CachedResult {
  spec_hash: string;
  spec_name: string;
  run_id: string;
  status: string;
  start_time: string;
  end_time: string;
  duration: number;
  error: string;
  timestamp: string;
  phase_results: CachePhaseResult[];
  fixture_results: CacheFixtureResult[];
  cleanup_run: boolean;
  file_size: number;
  file_name: string;
}

export interface CacheSummary {
  plans: CachedPlan[];
  results: CachedResult[];
  total_size: number;
  plan_count: number;
  result_count: number;
}

export interface CaseCache {
  spec_hashes: string[];
  plans: CachedPlan[];
  results: CachedResult[];
}

export interface RunEvent {
  type: "case_start" | "case_done" | "done";
  case_id?: number;
  result_id?: number;
  execution_id?: number;
  title?: string;
  verdict?: string;
  duration_ms?: number;
  passed?: number;
  failed?: number;
  blocked?: number;
  skipped?: number;
}
