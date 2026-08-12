"use client";

import { useState, useEffect, useRef, useCallback } from "react";
import {
  Play,
  CheckCircle,
  XCircle,
  Clock,
  Terminal,
  Loader2,
} from "lucide-react";
import { Modal } from "./Modal";
import { api } from "@/lib/api";
import { formatDuration } from "@/lib/utils";
import type { Tool, ToolRunLog } from "@/lib/types";

const streamColors: Record<string, string> = {
  stdout: "text-green-400",
  stderr: "text-red-400",
  system: "text-blue-400",
};

const streamIcons: Record<string, string> = {
  stdout: "→",
  stderr: "✗",
  system: "●",
};

const CATEGORIES: Record<string, { color: string; label: string }> = {
  Go: { color: "text-cyan-400", label: "Go" },
  Python: { color: "text-yellow-400", label: "Python" },
  Shell: { color: "text-slate-300", label: "Shell" },
};

export function ToolTestModal({
  tool,
  open,
  onClose,
}: {
  tool: Tool | null;
  open: boolean;
  onClose: () => void;
}) {
  const [phase, setPhase] = useState<"idle" | "running" | "done">("idle");
  const [logs, setLogs] = useState<ToolRunLog[]>([]);
  const [status, setStatus] = useState<string | null>(null);
  const [exitCode, setExitCode] = useState<number>(0);
  const [durationMs, setDurationMs] = useState(0);
  const [elapsed, setElapsed] = useState(0);
  const [paramValues, setParamValues] = useState<Record<string, string>>({});
  const logEndRef = useRef<HTMLDivElement>(null);
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const startRef = useRef(0);
  const eventSourceRef = useRef<EventSource | null>(null);

  const paramEntries = tool?.input_params ? Object.entries(tool.input_params) : [];
  const hasParams = paramEntries.length > 0;

  useEffect(() => {
    if (!open || !tool) return;
    setPhase("idle");
    setLogs([]);
    setStatus(null);
    setExitCode(0);
    setDurationMs(0);
    setElapsed(0);
    // Initialize param values with defaults
    const defaults: Record<string, string> = {};
    if (tool.input_params) {
      for (const [key, param] of Object.entries(tool.input_params)) {
        defaults[key] = param.default || "";
      }
    }
    setParamValues(defaults);
    return () => {
      if (eventSourceRef.current) eventSourceRef.current.close();
      if (timerRef.current) clearInterval(timerRef.current);
    };
  }, [open, tool]);

  const scrollToBottom = useCallback(() => {
    logEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, []);

  useEffect(() => {
    scrollToBottom();
  }, [logs, scrollToBottom]);

  const canRun = () => {
    if (!tool?.input_params) return true;
    for (const [key, param] of Object.entries(tool.input_params)) {
      if (param.required && !paramValues[key]?.trim()) return false;
    }
    return true;
  };

  const getResolvedCommand = () => {
    if (!tool) return "";
    let args = (tool.args || []).join(" ");
    for (const [key, val] of Object.entries(paramValues)) {
      args = args.replaceAll(`{{${key}}}`, val || `{{${key}}}`);
    }
    return `${tool.command} ${args}`.trim();
  };

  const startTest = async () => {
    if (!tool) return;

    setPhase("running");
    setLogs([]);
    setStatus(null);

    startRef.current = Date.now();
    timerRef.current = setInterval(() => setElapsed(Date.now() - startRef.current), 200);

    try {
      const { run_id } = await api.tools.test(tool.id, paramValues);

      const es = new EventSource(api.tools.streamUrl(run_id));
      eventSourceRef.current = es;

      es.onmessage = (event) => {
        const data = JSON.parse(event.data);

        if (data.type === "done") {
          setStatus(data.status);
          setExitCode(data.exit_code);
          setDurationMs(data.duration);
          setPhase("done");
          if (timerRef.current) clearInterval(timerRef.current);
          es.close();
          return;
        }

        setLogs((prev) => [...prev, data as ToolRunLog]);
      };

      es.onerror = () => {
        es.close();
        if (phase !== "done") {
          setPhase("done");
          setStatus("error");
          if (timerRef.current) clearInterval(timerRef.current);
        }
      };
    } catch {
      setPhase("done");
      setStatus("error");
      if (timerRef.current) clearInterval(timerRef.current);
    }
  };

  if (!tool) return null;

  const catMeta = CATEGORIES[tool.category] || { color: "text-slate-400", label: tool.category };
  const cmdPreview = [tool.command, ...(tool.args || [])].join(" ");

  return (
    <Modal open={open} onClose={onClose} title="" wide>
      <div>
        {/* Header */}
        <div className="flex items-start justify-between mb-4">
          <div className="flex-1">
            <div className="flex items-center gap-2 mb-1">
              <Terminal size={18} className="text-primary" />
              <span className="text-xs font-semibold text-muted uppercase tracking-wider">
                Tool Test
              </span>
              {phase === "running" && (
                <span className="flex items-center gap-1 text-xs text-blue-600 bg-blue-50 px-2 py-0.5 rounded-full">
                  <Loader2 size={12} className="animate-spin" /> Running
                </span>
              )}
              {phase === "done" && status && (
                <span
                  className={`flex items-center gap-1 text-xs px-2 py-0.5 rounded-full font-semibold uppercase ${
                    status === "passed"
                      ? "bg-emerald-100 text-emerald-800"
                      : "bg-red-100 text-red-800"
                  }`}
                >
                  {status === "passed" ? <CheckCircle size={12} /> : <XCircle size={12} />}
                  {status}
                </span>
              )}
            </div>
            <h2 className="text-lg font-semibold text-slate-800">{tool.name}</h2>
            <div className="flex items-center gap-3 mt-1">
              <span className={`text-xs font-bold uppercase ${catMeta.color}`}>
                {catMeta.label}
              </span>
              <code className="text-xs bg-slate-100 px-2 py-0.5 rounded font-mono text-slate-600">
                {cmdPreview}
              </code>
            </div>
          </div>
          <div className="flex items-center gap-2 text-muted">
            <Clock size={16} />
            <span className="font-mono text-lg tabular-nums">
              {phase === "done" ? formatDuration(durationMs) : formatDuration(elapsed)}
            </span>
          </div>
        </div>

        {/* Env vars preview */}
        {tool.env && Object.keys(tool.env).length > 0 && (
          <div className="mb-4 bg-slate-50 border border-border rounded-lg p-3">
            <div className="text-xs font-semibold text-muted uppercase mb-1">
              Environment
            </div>
            <div className="flex flex-wrap gap-2">
              {Object.entries(tool.env).map(([k, v]) => (
                <code key={k} className="text-xs bg-white border border-border rounded px-1.5 py-0.5 font-mono">
                  <span className="text-indigo-600">{k}</span>
                  <span className="text-muted">=</span>
                  <span className="text-slate-600">{v}</span>
                </code>
              ))}
            </div>
          </div>
        )}

        {/* Idle: parameter form + run button */}
        {phase === "idle" && (
          <div className="bg-slate-50 border border-border rounded-lg p-4">
            {hasParams && (
              <div className="mb-4">
                <div className="text-xs font-semibold text-muted uppercase tracking-wider mb-3">
                  Input Parameters
                </div>
                <div className="space-y-3">
                  {paramEntries.map(([key, param]) => (
                    <div key={key}>
                      <label className="flex items-center gap-2 text-sm font-medium text-slate-700 mb-1">
                        <code className="font-mono text-indigo-700 bg-indigo-50 px-1.5 py-0.5 rounded text-xs">
                          {key}
                        </code>
                        <span className="text-xs bg-slate-200 px-1.5 py-0.5 rounded text-muted">
                          {param.type}
                        </span>
                        {param.required && (
                          <span className="text-xs text-red-500 font-semibold">*</span>
                        )}
                      </label>
                      {param.description && (
                        <p className="text-xs text-muted mb-1">{param.description}</p>
                      )}
                      {param.type === "boolean" ? (
                        <select
                          value={paramValues[key] || ""}
                          onChange={(e) =>
                            setParamValues((prev) => ({ ...prev, [key]: e.target.value }))
                          }
                          className="w-full border border-border rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
                        >
                          <option value="">— select —</option>
                          <option value="true">true</option>
                          <option value="false">false</option>
                        </select>
                      ) : (
                        <input
                          type={param.type === "number" ? "number" : "text"}
                          value={paramValues[key] || ""}
                          onChange={(e) =>
                            setParamValues((prev) => ({ ...prev, [key]: e.target.value }))
                          }
                          placeholder={param.default ? `Default: ${param.default}` : `Enter ${key}...`}
                          className="w-full border border-border rounded-md px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-primary"
                        />
                      )}
                    </div>
                  ))}
                </div>

                {/* Resolved command preview */}
                <div className="mt-4 p-3 bg-slate-900 rounded-md">
                  <div className="text-[10px] text-slate-500 uppercase font-semibold mb-1">
                    Resolved Command
                  </div>
                  <code className="text-sm font-mono text-green-400">
                    {getResolvedCommand()}
                  </code>
                </div>
              </div>
            )}

            <div className="flex items-center justify-between">
              <div>
                {!hasParams && (
                  <>
                    <p className="text-sm text-slate-700">
                      Run <code className="bg-slate-200 px-1.5 py-0.5 rounded font-mono text-xs">{cmdPreview}</code> and
                      capture output.
                    </p>
                    {tool.description && (
                      <p className="text-xs text-muted mt-1">{tool.description}</p>
                    )}
                  </>
                )}
                {hasParams && !canRun() && (
                  <p className="text-xs text-amber-600">
                    Fill in all required parameters to run.
                  </p>
                )}
              </div>
              <button
                onClick={startTest}
                disabled={!canRun()}
                className="flex items-center gap-2 px-6 py-2.5 rounded-lg font-semibold text-sm text-white bg-emerald-500 hover:bg-emerald-600 transition-colors shadow-sm hover:shadow-md disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <Play size={18} /> Run Test
              </button>
            </div>
          </div>
        )}

        {/* Running / Done: live terminal output */}
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
                {tool.name} — {tool.category} tool
              </span>
              {phase === "running" && (
                <Loader2 size={12} className="animate-spin text-blue-400 ml-auto" />
              )}
            </div>

            {/* Log output */}
            <div className="px-4 py-3 max-h-[400px] overflow-y-auto font-mono text-sm">
              {logs.map((log) => {
                const color = streamColors[log.stream] || "text-slate-400";
                const icon = streamIcons[log.stream] || "·";
                const isSep = log.message.startsWith("──");

                if (isSep) {
                  return (
                    <div key={log.id} className="text-slate-600 py-0.5 text-xs">
                      {log.message}
                    </div>
                  );
                }

                return (
                  <div key={log.id} className="flex gap-2 py-0.5">
                    <span className="text-slate-600 text-xs w-16 shrink-0 text-right tabular-nums">
                      {new Date(log.timestamp).toLocaleTimeString("en-US", {
                        hour12: false,
                        hour: "2-digit",
                        minute: "2-digit",
                        second: "2-digit",
                      })}
                    </span>
                    <span className={`w-4 text-center shrink-0 ${color}`}>{icon}</span>
                    <span
                      className={`break-words ${
                        log.stream === "system" ? "text-blue-400 font-semibold" : color
                      }`}
                    >
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
                  status === "passed"
                    ? "bg-emerald-900/30 border-emerald-800"
                    : "bg-red-900/30 border-red-800"
                }`}
              >
                <div className="flex items-center gap-3">
                  {status === "passed" ? (
                    <CheckCircle size={22} className="text-emerald-400" />
                  ) : (
                    <XCircle size={22} className="text-red-400" />
                  )}
                  <div>
                    <div
                      className={`font-bold uppercase ${
                        status === "passed" ? "text-emerald-400" : "text-red-400"
                      }`}
                    >
                      {status}
                    </div>
                    <div className="text-xs text-slate-400">
                      Exit code: {exitCode} · Duration: {formatDuration(durationMs)}
                    </div>
                  </div>
                </div>
                <div className="flex gap-2">
                  <button
                    onClick={() => setPhase("idle")}
                    className="px-4 py-1.5 text-sm bg-slate-700 text-slate-200 rounded-md hover:bg-slate-600 transition-colors"
                  >
                    Re-run
                  </button>
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
