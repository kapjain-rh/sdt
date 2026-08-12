"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { useRouter, usePathname } from "next/navigation";
import {
  Play,
  CheckCircle,
  XCircle,
  Clock,
  ChevronLeft,
  Loader2,
  Terminal,
  ListChecks,
  Database,
} from "lucide-react";
import { api } from "@/lib/api";
import { Badge } from "@/components/Badge";
import { LogTerminal, useExecutionStream } from "@/components/LogViewer";
import type { TestRun, TestResult, RunEvent } from "@/lib/types";
import { formatDuration } from "@/lib/utils";

type CaseState = {
  resultId: number;
  caseId: number;
  title: string;
  group?: string;
  status: string;
  executionId?: number;
  durationMs?: number;
};

export default function RunProgressClient() {
  const router = useRouter();
  const pathname = usePathname();
  const runId = Number(pathname.split("/")[2]);

  const [run, setRun] = useState<TestRun | null>(null);
  const [cases, setCases] = useState<CaseState[]>([]);
  const [loading, setLoading] = useState(true);
  const [executing, setExecuting] = useState(false);
  const [selectedExecId, setSelectedExecId] = useState<number | null>(null);
  const [executedBy, setExecutedBy] = useState("");
  const [elapsed, setElapsed] = useState(0);
  const [cacheStatus, setCacheStatus] = useState<Record<number, "checking" | "cached" | "none">>({});

  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const startRef = useRef(0);
  const eventSourceRef = useRef<EventSource | null>(null);

  const { logs, status: logStatus } = useExecutionStream(selectedExecId);

  const connectToRunStream = useCallback(() => {
    if (eventSourceRef.current) eventSourceRef.current.close();

    const es = new EventSource(api.runs.streamUrl(runId));
    eventSourceRef.current = es;

    es.onmessage = (event) => {
      const data: RunEvent = JSON.parse(event.data);

      if (data.type === "case_start") {
        setCases((prev) =>
          prev.map((c) =>
            c.caseId === data.case_id
              ? { ...c, status: "running", executionId: data.execution_id }
              : c
          )
        );
        if (data.execution_id) {
          setSelectedExecId(data.execution_id);
        }
      }

      if (data.type === "case_done") {
        setCases((prev) =>
          prev.map((c) =>
            c.caseId === data.case_id
              ? {
                  ...c,
                  status: data.verdict || "failed",
                  durationMs: data.duration_ms,
                }
              : c
          )
        );
        if (data.verdict === "passed" && data.case_id) {
          setCacheStatus((prev) => ({ ...prev, [data.case_id!]: "checking" }));
          api.cases.cache(data.case_id!).then((cc) => {
            setCacheStatus((prev) => ({
              ...prev,
              [data.case_id!]: cc.plans && cc.plans.length > 0 ? "cached" : "none",
            }));
          }).catch(() => {
            setCacheStatus((prev) => ({ ...prev, [data.case_id!]: "none" }));
          });
        }
      }

      if (data.type === "done") {
        setExecuting(false);
        if (timerRef.current) clearInterval(timerRef.current);
        es.close();
        loadDataQuiet();
      }
    };

    es.onerror = () => {
      es.close();
      setExecuting(false);
      if (timerRef.current) clearInterval(timerRef.current);
      loadDataQuiet();
    };
  }, [runId]);

  const loadDataQuiet = useCallback(async () => {
    try {
      const [runData, resultsData] = await Promise.all([
        api.runs.get(runId),
        api.runs.results(runId),
      ]);
      setRun(runData);
      setCases(
        resultsData.map((r: TestResult) => ({
          resultId: r.id,
          caseId: r.case_id,
          title: r.case?.title || `Case ${r.case_id}`,
          group: r.case?.group,
          status: r.status,
          durationMs: r.duration_ms,
        }))
      );
    } catch {
      // ignore
    }
  }, [runId]);

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const [runData, resultsData] = await Promise.all([
        api.runs.get(runId),
        api.runs.results(runId),
      ]);
      setRun(runData);
      const caseStates = resultsData.map((r: TestResult) => ({
        resultId: r.id,
        caseId: r.case_id,
        title: r.case?.title || `Case ${r.case_id}`,
        group: r.case?.group,
        status: r.status,
        durationMs: r.duration_ms,
      }));
      setCases(caseStates);

      const passedCases = caseStates.filter((c: CaseState) => c.status === "passed");
      for (const c of passedCases) {
        setCacheStatus((prev) => ({ ...prev, [c.caseId]: "checking" }));
        api.cases.cache(c.caseId).then((cc) => {
          setCacheStatus((prev) => ({
            ...prev,
            [c.caseId]: cc.plans && cc.plans.length > 0 ? "cached" : "none",
          }));
        }).catch(() => {
          setCacheStatus((prev) => ({ ...prev, [c.caseId]: "none" }));
        });
      }

      if (runData.status === "in_progress") {
        setExecuting(true);
        startRef.current = runData.started_at
          ? new Date(runData.started_at).getTime()
          : Date.now();
        timerRef.current = setInterval(
          () => setElapsed(Date.now() - startRef.current),
          500
        );

        try {
          const active = await api.runs.activeExecution(runId);
          if (active.execution_id) {
            setSelectedExecId(active.execution_id);
            setCases((prev) =>
              prev.map((c) => {
                const exec = resultsData.find(
                  (r: TestResult) => r.case_id === c.caseId && r.status === "untested"
                );
                if (exec && active.execution_id) {
                  return { ...c, status: "running", executionId: active.execution_id };
                }
                return c;
              })
            );
          }
        } catch {
          // ignore
        }

        connectToRunStream();
      }
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  }, [runId, connectToRunStream]);

  useEffect(() => {
    loadData();
    const stored = localStorage.getItem("tcms_executor");
    if (stored) setExecutedBy(stored);
  }, [loadData]);

  useEffect(() => {
    if (!executing || run?.status === "completed") return;
    const poll = setInterval(async () => {
      try {
        const updated = await api.runs.get(runId);
        setRun(updated);
        if (updated.status === "completed") {
          setExecuting(false);
          if (timerRef.current) clearInterval(timerRef.current);
          loadDataQuiet();
        }
      } catch {
        // ignore
      }
    }, 3000);
    return () => clearInterval(poll);
  }, [executing, runId, run?.status, loadDataQuiet]);

  const handleRunAll = async () => {
    if (executedBy) localStorage.setItem("tcms_executor", executedBy);
    setExecuting(true);
    startRef.current = Date.now();
    timerRef.current = setInterval(
      () => setElapsed(Date.now() - startRef.current),
      500
    );

    try {
      await api.runs.execute(runId, executedBy);
      connectToRunStream();
    } catch {
      setExecuting(false);
      if (timerRef.current) clearInterval(timerRef.current);
    }
  };

  useEffect(() => {
    return () => {
      if (eventSourceRef.current) eventSourceRef.current.close();
      if (timerRef.current) clearInterval(timerRef.current);
    };
  }, []);

  if (loading) {
    return (
      <div className="p-8 text-center text-muted">Loading run...</div>
    );
  }

  if (!run) {
    return (
      <div className="p-8 text-center text-muted">
        <p>Run not found.</p>
        <button
          onClick={() => router.push("/runs")}
          className="mt-4 text-primary hover:underline"
        >
          Back to Runs
        </button>
      </div>
    );
  }

  const passed = cases.filter((c) => c.status === "passed").length;
  const failed = cases.filter((c) => c.status === "failed").length;
  const running = cases.filter((c) => c.status === "running").length;
  const untested = cases.filter(
    (c) => c.status === "untested" || c.status === "pending"
  ).length;
  const total = cases.length;
  const executed = passed + failed;

  const selectedCase = cases.find((c) => c.executionId === selectedExecId);

  return (
    <div className="flex flex-col h-screen">
      {/* Header */}
      <div className="border-b border-border bg-white px-6 py-4">
        <div className="flex items-center justify-between">
          <div>
            <button
              onClick={() => router.push("/runs")}
              className="flex items-center gap-1 text-sm text-muted hover:text-slate-700 mb-2 transition-colors"
            >
              <ChevronLeft size={14} /> Back to Runs
            </button>
            <div className="flex items-center gap-3">
              <h1 className="text-xl font-semibold text-slate-800">
                {run.name}
              </h1>
              <Badge value={executing ? "in_progress" : run.status} />
            </div>
            <p className="text-sm text-muted mt-0.5">
              Plan: {run.plan_name || "—"} &middot; {total} cases
            </p>
          </div>
          <div className="flex items-center gap-4">
            {executing && (
              <div className="flex items-center gap-1.5 text-muted">
                <Clock size={14} />
                <span className="font-mono text-sm tabular-nums">
                  {formatDuration(elapsed)}
                </span>
              </div>
            )}
            {!executing && run.status !== "completed" && (
              <div className="flex items-center gap-3">
                <input
                  value={executedBy}
                  onChange={(e) => setExecutedBy(e.target.value)}
                  placeholder="Your name"
                  className="w-36 border border-border rounded px-2 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-primary"
                />
                <button
                  onClick={handleRunAll}
                  className="flex items-center gap-2 px-5 py-2 rounded-lg font-semibold text-sm text-white bg-emerald-500 hover:bg-emerald-600 transition-colors shadow-sm"
                >
                  <Play size={16} /> Run All
                </button>
              </div>
            )}
          </div>
        </div>

        {/* Progress bar */}
        <div className="mt-4">
          <div className="flex justify-between text-xs text-muted mb-1">
            <span>
              {executed}/{total} completed
              {running > 0 && ` (${running} running)`}
            </span>
            <span>{total > 0 ? Math.round((executed / total) * 100) : 0}%</span>
          </div>
          <div className="h-2.5 bg-slate-100 rounded-full overflow-hidden flex">
            {passed > 0 && (
              <div
                className="bg-emerald-500 h-full transition-all duration-300"
                style={{ width: `${(passed / total) * 100}%` }}
              />
            )}
            {failed > 0 && (
              <div
                className="bg-red-500 h-full transition-all duration-300"
                style={{ width: `${(failed / total) * 100}%` }}
              />
            )}
            {running > 0 && (
              <div
                className="bg-blue-500 h-full transition-all duration-300 animate-pulse"
                style={{ width: `${(running / total) * 100}%` }}
              />
            )}
          </div>
          <div className="flex gap-4 mt-2">
            {[
              { label: "Passed", val: passed, color: "text-emerald-600", bg: "bg-emerald-50" },
              { label: "Failed", val: failed, color: "text-red-600", bg: "bg-red-50" },
              { label: "Running", val: running, color: "text-blue-600", bg: "bg-blue-50" },
              { label: "Untested", val: untested, color: "text-slate-500", bg: "bg-slate-50" },
            ].map(({ label, val, color, bg }) => (
              <div
                key={label}
                className={`flex items-center gap-1.5 px-2 py-1 rounded text-xs ${bg} ${color}`}
              >
                <span className="font-bold">{val}</span>
                <span>{label}</span>
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* Main content: case list + log viewer */}
      <div className="flex flex-1 overflow-hidden">
        {/* Case list */}
        <div className="w-80 border-r border-border bg-white flex flex-col shrink-0">
          <div className="p-3 border-b border-border bg-slate-50">
            <div className="flex items-center gap-2 text-sm font-semibold text-slate-700">
              <ListChecks size={16} />
              Test Cases
            </div>
          </div>
          <div className="flex-1 overflow-y-auto">
            {cases.map((c) => {
              const isSelected = c.executionId === selectedExecId && selectedExecId !== null;
              const isRunning = c.status === "running";

              return (
                <button
                  key={c.resultId}
                  onClick={() => {
                    if (c.executionId) setSelectedExecId(c.executionId);
                  }}
                  className={`w-full text-left px-4 py-3 border-b border-border/50 transition-colors ${
                    isSelected
                      ? "bg-primary/5 border-l-[3px] border-l-primary"
                      : "hover:bg-slate-50"
                  }`}
                >
                  <div className="flex items-center gap-2 mb-0.5">
                    {isRunning ? (
                      <Loader2
                        size={14}
                        className="animate-spin text-blue-500"
                      />
                    ) : c.status === "passed" ? (
                      <CheckCircle size={14} className="text-emerald-500" />
                    ) : c.status === "failed" || c.status === "error" ? (
                      <XCircle size={14} className="text-red-500" />
                    ) : (
                      <div className="w-3.5 h-3.5 rounded-full border-2 border-slate-300" />
                    )}
                    <span className="text-xs text-muted font-mono">
                      TC-{c.caseId}
                    </span>
                    {c.group && (
                      <span className="text-[10px] text-violet-600 bg-violet-50 px-1 py-0.5 rounded">
                        {c.group}
                      </span>
                    )}
                  </div>
                  <div className="text-sm text-slate-700 truncate pl-5">
                    {c.title}
                  </div>
                  <div className="flex items-center gap-2 pl-5 mt-0.5">
                    {c.durationMs != null && c.durationMs > 0 && (
                      <span className="text-xs text-muted">
                        {formatDuration(c.durationMs)}
                      </span>
                    )}
                    {c.status === "passed" && cacheStatus[c.caseId] === "cached" && (
                      <span className="flex items-center gap-1 text-[10px] text-emerald-600 bg-emerald-50 px-1.5 py-0.5 rounded">
                        <Database size={10} /> Cached
                      </span>
                    )}
                  </div>
                </button>
              );
            })}
          </div>
        </div>

        {/* Log viewer */}
        <div className="flex-1 flex flex-col overflow-hidden bg-slate-50">
          {selectedExecId ? (
            <div className="flex-1 overflow-hidden flex flex-col">
              <div className="px-4 py-3 bg-white border-b border-border flex items-center gap-3">
                <Terminal size={16} className="text-primary" />
                <span className="text-sm font-semibold text-slate-700">
                  {selectedCase?.title || "Execution Logs"}
                </span>
                {logStatus === "running" && (
                  <span className="flex items-center gap-1 text-xs text-blue-600 bg-blue-50 px-2 py-0.5 rounded-full">
                    <Loader2 size={10} className="animate-spin" /> Live
                  </span>
                )}
                {selectedCase?.status === "passed" && selectedCase.caseId && (
                  <div className="ml-auto">
                    {cacheStatus[selectedCase.caseId] === "cached" ? (
                      <span className="flex items-center gap-1.5 px-3 py-1 text-xs text-emerald-600 bg-emerald-50 rounded-md border border-emerald-200">
                        <Database size={12} /> Cached
                      </span>
                    ) : cacheStatus[selectedCase.caseId] === "checking" ? (
                      <span className="flex items-center gap-1.5 px-3 py-1 text-xs text-slate-500">
                        <Loader2 size={12} className="animate-spin" /> Checking cache...
                      </span>
                    ) : cacheStatus[selectedCase.caseId] === "none" ? (
                      <button
                        onClick={async () => {
                          const cid = selectedCase.caseId;
                          setCacheStatus((prev) => ({ ...prev, [cid]: "checking" }));
                          try {
                            await api.cases.saveCache(cid);
                            setCacheStatus((prev) => ({ ...prev, [cid]: "cached" }));
                          } catch {
                            setCacheStatus((prev) => ({ ...prev, [cid]: "none" }));
                          }
                        }}
                        className="flex items-center gap-1.5 px-3 py-1 text-xs text-white bg-blue-500 hover:bg-blue-600 rounded-md transition-colors"
                      >
                        <Database size={12} /> Save to Cache
                      </button>
                    ) : null}
                  </div>
                )}
              </div>
              <div className="flex-1 overflow-hidden p-4">
                <LogTerminal
                  logs={logs}
                  running={logStatus === "running"}
                  maxHeight="calc(100vh - 320px)"
                />
              </div>
            </div>
          ) : (
            <div className="flex-1 flex items-center justify-center text-muted">
              <div className="text-center">
                <Terminal size={48} className="mx-auto mb-3 text-slate-300" />
                <p className="text-sm">
                  {executing
                    ? "Waiting for test execution to start..."
                    : "Click a test case to view its execution logs"}
                </p>
                {!executing && run.status !== "completed" && (
                  <p className="text-xs mt-2">
                    Or click &ldquo;Run All&rdquo; to execute all test cases
                  </p>
                )}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
