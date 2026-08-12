import type {
  Project,
  TestCase,
  TestPlan,
  TestRun,
  TestResult,
  DashboardStats,
  TrendPoint,
  Execution,
  Tool,
  ToolRun,
  MCPServer,
  MCPDiscoveredTool,
  SuiteInfo,
  GroupInfo,
  Fixture,
  CacheSummary,
  CaseCache,
} from "./types";

const STORAGE_KEY = "sdt_backend_url";

function getBaseUrl(): string {
  if (typeof window === "undefined") return "/api";
  return localStorage.getItem(STORAGE_KEY) || "/api";
}

export function getBackendUrl(): string {
  if (typeof window === "undefined") return "";
  return localStorage.getItem(STORAGE_KEY) || "";
}

export function setBackendUrl(url: string) {
  if (url) {
    localStorage.setItem(STORAGE_KEY, url.replace(/\/+$/, ""));
  } else {
    localStorage.removeItem(STORAGE_KEY);
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${getBaseUrl()}${path}`, {
    headers: { "Content-Type": "application/json" },
    ...init,
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `HTTP ${res.status}`);
  }
  return res.json();
}

export const api = {
  dashboard: {
    stats: () => request<DashboardStats>("/dashboard"),
    trends: (projectId?: number, days = 30) => {
      const params = new URLSearchParams({ days: String(days) });
      if (projectId) params.set("project_id", String(projectId));
      return request<TrendPoint[]>(`/dashboard/trends?${params}`);
    },
  },

  projects: {
    list: () => request<Project[]>("/projects"),
    get: (id: number) => request<Project>(`/projects/${id}`),
    create: (data: Partial<Project>) =>
      request<Project>("/projects", { method: "POST", body: JSON.stringify(data) }),
    update: (id: number, data: Partial<Project>) =>
      request<Project>(`/projects/${id}`, { method: "PUT", body: JSON.stringify(data) }),
    delete: (id: number) =>
      request(`/projects/${id}`, { method: "DELETE" }),
  },

  cases: {
    list: (projectId: number, search?: string) => {
      const params = search ? `?search=${encodeURIComponent(search)}` : "";
      return request<TestCase[]>(`/projects/${projectId}/cases${params}`);
    },
    get: (id: number) => request<TestCase>(`/cases/${id}`),
    create: (projectId: number, data: Partial<TestCase>) =>
      request<TestCase>(`/projects/${projectId}/cases`, {
        method: "POST",
        body: JSON.stringify(data),
      }),
    update: (id: number, data: Partial<TestCase>) =>
      request<TestCase>(`/cases/${id}`, { method: "PUT", body: JSON.stringify(data) }),
    delete: (id: number) =>
      request(`/cases/${id}`, { method: "DELETE" }),
    cache: (id: number) => request<CaseCache>(`/cases/${id}/cache`),
    saveCache: (id: number) =>
      request(`/cases/${id}/cache`, { method: "POST" }),
    deleteCache: (id: number) =>
      request(`/cases/${id}/cache`, { method: "DELETE" }),
  },

  plans: {
    list: (projectId: number) => request<TestPlan[]>(`/projects/${projectId}/plans`),
    get: (id: number) => request<TestPlan>(`/plans/${id}`),
    create: (projectId: number, data: Partial<TestPlan>) =>
      request<TestPlan>(`/projects/${projectId}/plans`, {
        method: "POST",
        body: JSON.stringify(data),
      }),
    update: (id: number, data: Partial<TestPlan>) =>
      request<TestPlan>(`/plans/${id}`, { method: "PUT", body: JSON.stringify(data) }),
    delete: (id: number) =>
      request(`/plans/${id}`, { method: "DELETE" }),
    getCases: (planId: number) => request<TestCase[]>(`/plans/${planId}/cases`),
    setCases: (planId: number, caseIds: number[]) =>
      request(`/plans/${planId}/cases`, {
        method: "POST",
        body: JSON.stringify({ case_ids: caseIds }),
      }),
    removeCaseFromPlan: (planId: number, caseId: number) =>
      request(`/plans/${planId}/cases/${caseId}`, { method: "DELETE" }),
  },

  runs: {
    list: (projectId: number) => request<TestRun[]>(`/projects/${projectId}/runs`),
    listForPlan: (planId: number) => request<TestRun[]>(`/plans/${planId}/runs`),
    get: (id: number) => request<TestRun>(`/runs/${id}`),
    create: (planId: number, data: Partial<TestRun>) =>
      request<TestRun>(`/plans/${planId}/runs`, {
        method: "POST",
        body: JSON.stringify(data),
      }),
    delete: (id: number) =>
      request(`/runs/${id}`, { method: "DELETE" }),
    start: (id: number) =>
      request(`/runs/${id}/start`, { method: "POST" }),
    complete: (id: number) =>
      request(`/runs/${id}/complete`, { method: "POST" }),
    results: (runId: number) => request<TestResult[]>(`/runs/${runId}/results`),
    execute: (id: number, executedBy: string, envVars?: Record<string, string>) =>
      request<{ status: string }>(`/runs/${id}/execute`, {
        method: "POST",
        body: JSON.stringify({ executed_by: executedBy, env_vars: envVars }),
      }),
    streamUrl: (id: number) => `${getBaseUrl()}/runs/${id}/stream`,
    activeExecution: (id: number) =>
      request<{ execution_id: number | null }>(`/runs/${id}/active-execution`),
  },

  results: {
    get: (id: number) => request<TestResult>(`/results/${id}`),
    update: (
      id: number,
      data: {
        status: string;
        comment: string;
        executed_by: string;
        duration_ms: number;
        step_results?: { step_index: number; status: string; actual_result: string; comment: string }[];
      }
    ) => request<TestResult>(`/results/${id}`, { method: "PUT", body: JSON.stringify(data) }),
  },

  tools: {
    list: (projectId: number, search?: string) => {
      const params = search ? `?search=${encodeURIComponent(search)}` : "";
      return request<Tool[]>(`/projects/${projectId}/tools${params}`);
    },
    get: (id: number) => request<Tool>(`/tools/${id}`),
    create: (projectId: number, data: Partial<Tool>) =>
      request<Tool>(`/projects/${projectId}/tools`, {
        method: "POST",
        body: JSON.stringify(data),
      }),
    update: (id: number, data: Partial<Tool>) =>
      request<Tool>(`/tools/${id}`, { method: "PUT", body: JSON.stringify(data) }),
    delete: (id: number) =>
      request(`/tools/${id}`, { method: "DELETE" }),
    setStatus: (id: number, status: string) =>
      request<Tool>(`/tools/${id}/status`, {
        method: "POST",
        body: JSON.stringify({ status }),
      }),
    test: (id: number, params?: Record<string, string>) =>
      request<{ run_id: number }>(`/tools/${id}/test`, {
        method: "POST",
        body: JSON.stringify({ params: params || {} }),
      }),
    listRuns: (id: number) => request<ToolRun[]>(`/tools/${id}/runs`),
    streamUrl: (runId: number) => `${getBaseUrl()}/tool-runs/${runId}/stream`,
  },

  mcpServers: {
    list: (projectId: number) =>
      request<MCPServer[]>(`/projects/${projectId}/mcp-servers`),
    get: (id: number) => request<MCPServer>(`/mcp-servers/${id}`),
    create: (projectId: number, data: Partial<MCPServer>) =>
      request<MCPServer>(`/projects/${projectId}/mcp-servers`, {
        method: "POST",
        body: JSON.stringify(data),
      }),
    update: (id: number, data: Partial<MCPServer>) =>
      request<MCPServer>(`/mcp-servers/${id}`, {
        method: "PUT",
        body: JSON.stringify(data),
      }),
    delete: (id: number) =>
      request(`/mcp-servers/${id}`, { method: "DELETE" }),
    connect: (id: number) =>
      request<{ status: string; tools: MCPDiscoveredTool[]; error?: string }>(
        `/mcp-servers/${id}/connect`,
        { method: "POST" }
      ),
    disconnect: (id: number) =>
      request(`/mcp-servers/${id}/disconnect`, { method: "POST" }),
    tools: (id: number) =>
      request<MCPDiscoveredTool[]>(`/mcp-servers/${id}/tools`),
    callTool: (id: number, name: string, args: Record<string, unknown>) =>
      request<{ run_id: number }>(
        `/mcp-servers/${id}/tools/call`,
        { method: "POST", body: JSON.stringify({ name, arguments: args }) }
      ),
    refresh: (id: number) =>
      request<MCPDiscoveredTool[]>(`/mcp-servers/${id}/refresh`, { method: "POST" }),
  },

  suite: {
    get: (projectId: number) => request<SuiteInfo>(`/projects/${projectId}/suite`),
    update: (projectId: number, data: Partial<SuiteInfo>) =>
      request<SuiteInfo>(`/projects/${projectId}/suite`, {
        method: "PUT",
        body: JSON.stringify(data),
      }),
    cache: (projectId: number) => request<CaseCache>(`/projects/${projectId}/suite/cache`),
  },

  groups: {
    list: (projectId: number) => request<GroupInfo[]>(`/projects/${projectId}/groups`),
    get: (projectId: number, name: string) =>
      request<GroupInfo>(`/projects/${projectId}/groups/${encodeURIComponent(name)}`),
    create: (projectId: number, data: Partial<GroupInfo>) =>
      request<GroupInfo>(`/projects/${projectId}/groups`, {
        method: "POST",
        body: JSON.stringify(data),
      }),
    update: (projectId: number, name: string, data: Partial<GroupInfo>) =>
      request<GroupInfo>(`/projects/${projectId}/groups/${encodeURIComponent(name)}`, {
        method: "PUT",
        body: JSON.stringify(data),
      }),
    delete: (projectId: number, name: string) =>
      request(`/projects/${projectId}/groups/${encodeURIComponent(name)}`, {
        method: "DELETE",
      }),
    cache: (projectId: number, name: string) =>
      request<CaseCache>(`/projects/${projectId}/groups/${encodeURIComponent(name)}/cache`),
  },

  fixtures: {
    list: (projectId: number) => request<Fixture[]>(`/projects/${projectId}/fixtures`),
    get: (projectId: number, name: string) =>
      request<Fixture>(`/projects/${projectId}/fixtures/${encodeURIComponent(name)}`),
    create: (projectId: number, data: Partial<Fixture>) =>
      request<Fixture>(`/projects/${projectId}/fixtures`, {
        method: "POST",
        body: JSON.stringify(data),
      }),
    update: (projectId: number, name: string, data: Partial<Fixture>) =>
      request<Fixture>(`/projects/${projectId}/fixtures/${encodeURIComponent(name)}`, {
        method: "PUT",
        body: JSON.stringify(data),
      }),
    delete: (projectId: number, name: string) =>
      request(`/projects/${projectId}/fixtures/${encodeURIComponent(name)}`, {
        method: "DELETE",
      }),
  },

  cache: {
    get: (projectId: number) => request<CacheSummary>(`/projects/${projectId}/cache`),
    clear: (projectId: number) =>
      request(`/projects/${projectId}/cache`, { method: "DELETE" }),
  },

  executions: {
    run: (caseId: number, executedBy: string, opts?: { title?: string; steps?: string[]; env_vars?: Record<string, string> }) =>
      request<{ execution_id: number }>(`/cases/${caseId}/execute`, {
        method: "POST",
        body: JSON.stringify({ executed_by: executedBy, ...opts }),
      }),
    get: (id: number) => request<Execution>(`/executions/${id}`),
    list: (caseId: number) => request<Execution[]>(`/cases/${caseId}/executions`),
    streamUrl: (id: number) => `${getBaseUrl()}/executions/${id}/stream`,
  },
};
