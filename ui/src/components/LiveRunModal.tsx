"use client";

import { useState, useEffect, useRef, useCallback } from "react";
import {
  Play,
  CheckCircle,
  XCircle,
  Clock,
  AlertTriangle,
  Terminal,
  Loader2,
  Database,
} from "lucide-react";
import { Modal } from "./Modal";
import { Badge } from "./Badge";
import { api } from "@/lib/api";
import { formatDuration } from "@/lib/utils";
import type { TestCase, ExecutionLog } from "@/lib/types";

const logTypeStyles: Record<string, { icon?: string; color: string; bold?: boolean }> = {
  info: { icon: "ℹ", color: "text-slate-500" },
  action: { icon: "▶", color: "text-blue-600" },
  running: { icon: "⏳", color: "text-blue-500" },
  expected: { icon: "📋", color: "text-slate-600" },
  actual: { icon: "📌", color: "text-slate-700" },
  verifying: { icon: "🔍", color: "text-purple-600" },
  precondition: { icon: "⚙", color: "text-amber-600" },
  precondition_ok: { icon: "✓", color: "text-emerald-600" },
  step_start: { icon: "→", color: "text-primary", bold: true },
  step_status: { icon: "●", color: "text-emerald-600", bold: true },
  step_done: { icon: "✓", color: "text-emerald-600" },
  summary: { icon: "📊", color: "text-slate-800", bold: true },
  verdict: { icon: "🏁", color: "text-slate-900", bold: true },
  separator: { color: "text-slate-300" },
  finished: { icon: "✅", color: "text-emerald-700", bold: true },
  error: { icon: "✗", color: "text-red-600", bold: true },
  result_data: { color: "hidden" },
};

function LogLine({ log }: { log: ExecutionLog }) {
  const style = logTypeStyles[log.log_type] || { color: "text-slate-500" };
  if (style.color === "hidden") return null;

  const isStep = log.step_index >= 0;

  return (
    <div className={`flex gap-2 py-0.5 ${isStep ? "pl-4" : ""} ${style.bold ? "font-semibold" : ""}`}>
      <span className="text-xs text-slate-400 font-mono w-16 shrink-0 text-right tabular-nums">
        {new Date(log.timestamp).toLocaleTimeString("en-US", {
          hour12: false,
          hour: "2-digit",
          minute: "2-digit",
          second: "2-digit",
        })}
      </span>
      {style.icon && <span className="w-4 text-center shrink-0">{style.icon}</span>}
      <span className={`text-sm ${style.color} break-words`}>{log.message}</span>
    </div>
  );
}

export function LiveRunModal({
  testCase,
  open,
  onClose,
  envVars,
}: {
  testCase: TestCase | null;
  open: boolean;
  onClose: () => void;
  envVars?: Record<string, string>;
}) {
  const [phase, setPhase] = useState<"idle" | "running" | "done">("idle");
  const [executedBy, setExecutedBy] = useState("");
  const [logs, setLogs] = useState<ExecutionLog[]>([]);
  const [verdict, setVerdict] = useState<string | null>(null);
  const [durationMs, setDurationMs] = useState(0);
  const [elapsed, setElapsed] = useState(0);
  const [cacheSaved, setCacheSaved] = useState(false);
  const [cacheChecked, setCacheChecked] = useState(false);
  const logEndRef = useRef<HTMLDivElement>(null);
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const startRef = useRef(0);
  const eventSourceRef = useRef<EventSource | null>(null);

  useEffect(() => {
    if (phase === "done" && verdict === "passed" && testCase) {
      api.cases.cache(testCase.id).then((cc) => {
        setCacheSaved(cc.plans && cc.plans.length > 0);
        setCacheChecked(true);
      }).catch(() => setCacheChecked(true));
    }
  }, [phase, verdict, testCase]);

  useEffect(() => {
    if (!open) return;
    setPhase("idle");
    setLogs([]);
    setVerdict(null);
    setDurationMs(0);
    setElapsed(0);
    setCacheSaved(false);
    setCacheChecked(false);
    const stored = localStorage.getItem("tcms_executor");
    if (stored) setExecutedBy(stored);
    return () => {
      if (eventSourceRef.current) eventSourceRef.current.close();
      if (timerRef.current) clearInterval(timerRef.current);
    };
  }, [open, testCase]);

  const scrollToBottom = useCallback(() => {
    logEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, []);

  useEffect(() => {
    scrollToBottom();
  }, [logs, scrollToBottom]);

  const pollExecution = useCallback(async (execId: number) => {
    let lastLogId = 0;
    const poll = async () => {
      try {
        const logs = await fetch(`/api/executions/${execId}/logs?after=${lastLogId}`).then(r => r.json());
        if (logs.length > 0) {
          setLogs((prev) => [...prev, ...logs]);
          lastLogId = logs[logs.length - 1].id;
        }
        const exec = await api.executions.get(execId);
        if (exec.status !== "pending" && exec.status !== "running") {
          setVerdict(exec.verdict || exec.status);
          setDurationMs(exec.duration_ms);
          setPhase("done");
          if (timerRef.current) clearInterval(timerRef.current);
          return;
        }
        setTimeout(poll, 1000);
      } catch {
        setPhase("done");
        setVerdict("error");
        if (timerRef.current) clearInterval(timerRef.current);
      }
    };
    poll();
  }, []);

  const startRun = async () => {
    if (!testCase) return;
    if (executedBy) localStorage.setItem("tcms_executor", executedBy);

    setPhase("running");
    setLogs([]);
    setVerdict(null);

    startRef.current = Date.now();
    timerRef.current = setInterval(() => setElapsed(Date.now() - startRef.current), 200);

    try {
      const isSynthetic = testCase.id === 0;
      const hasEnv = envVars && Object.keys(envVars).length > 0;
      const opts = isSynthetic
        ? { title: testCase.title, steps: [...(testCase.setup || []), ...(testCase.steps || []), ...(testCase.verify || []), ...(testCase.cleanup || [])], ...(hasEnv ? { env_vars: envVars } : {}) }
        : hasEnv ? { env_vars: envVars } : undefined;
      const caseId = isSynthetic ? 1 : testCase.id;
      const { execution_id } = await api.executions.run(caseId, executedBy, opts);

      const es = new EventSource(api.executions.streamUrl(execution_id));
      eventSourceRef.current = es;
      let done = false;

      es.onmessage = (event) => {
        const data = JSON.parse(event.data);

        if (data.type === "done") {
          done = true;
          setVerdict(data.verdict);
          setDurationMs(data.duration);
          setPhase("done");
          if (timerRef.current) clearInterval(timerRef.current);
          es.close();
          return;
        }

        setLogs((prev) => [...prev, data as ExecutionLog]);
      };

      es.onerror = () => {
        es.close();
        if (done) return;
        pollExecution(execution_id);
      };
    } catch {
      setPhase("done");
      setVerdict("error");
      if (timerRef.current) clearInterval(timerRef.current);
    }
  };

  if (!testCase) return null;

  const steps = testCase.steps || [];
  const allSteps = [
    ...(testCase.setup || []).map((s) => ({ text: s, section: "Setup" })),
    ...steps.map((s) => ({ text: s, section: "Steps" })),
    ...(testCase.verify || []).map((s) => ({ text: s, section: "Verify" })),
    ...(testCase.cleanup || []).map((s) => ({ text: s, section: "Cleanup" })),
  ];

  return (
    <Modal open={open} onClose={onClose} title="" wide>
      <div>
        {/* Header */}
        <div className="flex items-start justify-between mb-4">
          <div className="flex-1">
            <div className="flex items-center gap-2 mb-1">
              <Terminal size={18} className="text-primary" />
              <span className="text-xs font-semibold text-muted uppercase tracking-wider">
                Test Execution
              </span>
              {phase === "running" && (
                <span className="flex items-center gap-1 text-xs text-blue-600 bg-blue-50 px-2 py-0.5 rounded-full">
                  <Loader2 size={12} className="animate-spin" /> Running
                </span>
              )}
              {phase === "done" && verdict && (
                <Badge value={verdict} />
              )}
            </div>
            <h2 className="text-lg font-semibold text-slate-800">{testCase.title}</h2>
            <div className="flex items-center gap-2 mt-1">
              <Badge value={testCase.priority} type="priority" />
              <span className="text-xs text-muted">{testCase.case_id || testCase.id}</span>
              {testCase.group && (
                <span className="text-xs text-violet-600 bg-violet-50 px-1.5 py-0.5 rounded">
                  {testCase.group}
                </span>
              )}
              <span className="text-xs text-muted">&middot; {allSteps.length} steps</span>
            </div>
          </div>
          <div className="flex items-center gap-2 text-muted">
            <Clock size={16} />
            <span className="font-mono text-lg tabular-nums">
              {phase === "done" ? formatDuration(durationMs) : formatDuration(elapsed)}
            </span>
          </div>
        </div>

        {/* Fixtures */}
        {testCase.fixtures && testCase.fixtures.length > 0 && (
          <div className="mb-4 bg-violet-50 border border-violet-200 rounded-lg p-3">
            <div className="text-xs font-semibold text-violet-700 uppercase mb-1">Fixtures</div>
            <div className="flex gap-1 flex-wrap">
              {testCase.fixtures.map((f) => (
                <span key={f} className="bg-violet-100 text-violet-700 px-2 py-0.5 rounded text-xs font-mono">{f}</span>
              ))}
            </div>
          </div>
        )}

        {/* Idle state: show steps summary and start button */}
        {phase === "idle" && (
          <>
            {/* Steps Preview */}
            {allSteps.length > 0 && (
              <div className="mb-4">
                <h3 className="text-sm font-semibold text-slate-700 mb-2">Test Flow</h3>
                <div className="space-y-1.5">
                  {allSteps.map((step, i) => {
                    const sectionColors: Record<string, string> = {
                      Setup: "text-amber-700 bg-amber-100",
                      Steps: "text-primary bg-primary/10",
                      Verify: "text-emerald-700 bg-emerald-100",
                      Cleanup: "text-slate-600 bg-slate-200",
                    };
                    return (
                      <div key={i} className="flex gap-2 items-start border border-border rounded-lg px-3 py-2 bg-slate-50">
                        <span className={`text-[10px] font-bold px-1.5 py-0.5 rounded mt-0.5 shrink-0 ${sectionColors[step.section] || ""}`}>
                          {step.section}
                        </span>
                        <p className="text-sm text-slate-800">{step.text}</p>
                      </div>
                    );
                  })}
                </div>
              </div>
            )}

            {/* Executor + Run button */}
            <div className="bg-slate-50 border border-border rounded-lg p-4">
              <div className="flex items-center gap-4">
                <div className="flex items-center gap-2 flex-1">
                  <label className="text-sm font-medium text-slate-700 whitespace-nowrap">Run as</label>
                  <input
                    value={executedBy}
                    onChange={(e) => setExecutedBy(e.target.value)}
                    className="w-48 border border-border rounded-md px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
                    placeholder="Your name"
                  />
                </div>
                <button
                  onClick={startRun}
                  className="flex items-center gap-2 px-6 py-2.5 rounded-lg font-semibold text-sm text-white bg-emerald-500 hover:bg-emerald-600 transition-colors shadow-sm hover:shadow-md"
                >
                  <Play size={18} /> Run Test
                </button>
              </div>
            </div>
          </>
        )}

        {/* Running / Done: show live log output */}
        {(phase === "running" || phase === "done") && (
          <div className="bg-slate-900 rounded-lg overflow-hidden">
            {/* Terminal header */}
            <div className="flex items-center gap-2 px-4 py-2 bg-slate-800 border-b border-slate-700">
              <div className="flex gap-1.5">
                <div className="w-3 h-3 rounded-full bg-red-500/80" />
                <div className="w-3 h-3 rounded-full bg-yellow-500/80" />
                <div className="w-3 h-3 rounded-full bg-green-500/80" />
              </div>
              <span className="text-xs text-slate-400 font-mono ml-2">
                tcms — TC-{testCase.id} execution
              </span>
              {phase === "running" && (
                <Loader2 size={12} className="animate-spin text-blue-400 ml-auto" />
              )}
            </div>

            {/* Log output */}
            <div className="px-4 py-3 max-h-[400px] overflow-y-auto font-mono text-sm">
              {logs.map((log) => {
                const style = logTypeStyles[log.log_type] || { color: "text-slate-400" };
                if (style.color === "hidden") return null;

                const isStep = log.step_index >= 0;
                const isSep = log.log_type === "separator";
                const isVerdict = log.log_type === "verdict";
                const isFinished = log.log_type === "finished";

                if (isSep) {
                  return (
                    <div key={log.id} className="text-slate-600 py-0.5 text-xs">
                      {log.message}
                    </div>
                  );
                }

                if (isVerdict) {
                  const vColor = log.message === "passed" ? "text-emerald-400" : "text-red-400";
                  return (
                    <div key={log.id} className={`py-1 text-lg font-bold ${vColor} uppercase`}>
                      VERDICT: {log.message}
                    </div>
                  );
                }

                if (isFinished) {
                  return (
                    <div key={log.id} className="py-1 text-emerald-400 font-semibold">
                      {log.message}
                    </div>
                  );
                }

                const colorMap: Record<string, string> = {
                  info: "text-slate-400",
                  action: "text-blue-400",
                  running: "text-blue-300",
                  expected: "text-slate-500",
                  actual: "text-cyan-400",
                  verifying: "text-purple-400",
                  precondition: "text-amber-400",
                  precondition_ok: "text-emerald-400",
                  step_start: "text-white",
                  step_status: "text-emerald-400",
                  step_done: "text-emerald-400",
                  summary: "text-yellow-300",
                  error: "text-red-400",
                };

                return (
                  <div
                    key={log.id}
                    className={`flex gap-2 py-0.5 ${isStep ? "pl-4" : ""} ${
                      style.bold ? "font-semibold" : ""
                    }`}
                  >
                    <span className="text-slate-600 text-xs w-16 shrink-0 text-right tabular-nums">
                      {new Date(log.timestamp).toLocaleTimeString("en-US", {
                        hour12: false,
                        hour: "2-digit",
                        minute: "2-digit",
                        second: "2-digit",
                      })}
                    </span>
                    {style.icon && (
                      <span className="w-4 text-center shrink-0 text-slate-500">{style.icon}</span>
                    )}
                    <span className={`${colorMap[log.log_type] || "text-slate-400"} break-words`}>
                      {log.message}
                    </span>
                  </div>
                );
              })}

              {phase === "running" && (
                <div className="flex items-center gap-2 py-1 text-blue-400">
                  <Loader2 size={14} className="animate-spin" />
                  <span className="text-sm animate-pulse">Executing...</span>
                </div>
              )}
              <div ref={logEndRef} />
            </div>

            {/* Result summary bar */}
            {phase === "done" && (
              <div
                className={`px-4 py-3 border-t flex items-center justify-between ${
                  verdict === "passed"
                    ? "bg-emerald-900/30 border-emerald-800"
                    : verdict === "failed"
                    ? "bg-red-900/30 border-red-800"
                    : "bg-slate-800 border-slate-700"
                }`}
              >
                <div className="flex items-center gap-3">
                  {verdict === "passed" ? (
                    <CheckCircle size={22} className="text-emerald-400" />
                  ) : (
                    <XCircle size={22} className="text-red-400" />
                  )}
                  <div>
                    <div className={`font-bold uppercase ${verdict === "passed" ? "text-emerald-400" : "text-red-400"}`}>
                      {verdict}
                    </div>
                    <div className="text-xs text-slate-400">
                      {executedBy && `by ${executedBy} · `}
                      Duration: {formatDuration(durationMs)}
                    </div>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  {verdict === "passed" && cacheChecked && (
                    cacheSaved ? (
                      <span className="flex items-center gap-1.5 px-3 py-1.5 text-sm text-emerald-400 bg-emerald-900/30 rounded-md border border-emerald-800">
                        <Database size={14} />
                        Cached
                      </span>
                    ) : (
                      <button
                        onClick={async () => {
                          if (!testCase) return;
                          try {
                            await api.cases.saveCache(testCase.id);
                            setCacheSaved(true);
                          } catch {
                            // save failed
                          }
                        }}
                        className="flex items-center gap-1.5 px-3 py-1.5 text-sm bg-blue-600 text-white rounded-md hover:bg-blue-500 transition-colors"
                      >
                        <Database size={14} />
                        Save to Cache
                      </button>
                    )
                  )}
                  <button
                    onClick={onClose}
                    className="px-4 py-1.5 text-sm bg-slate-700 text-slate-200 rounded-md hover:bg-slate-600 transition-colors"
                  >
                    Close
                  </button>
                </div>
              </div>
            )}
          </div>
        )}
      </div>
    </Modal>
  );
}
