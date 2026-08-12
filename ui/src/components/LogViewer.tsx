"use client";

import { useState, useEffect, useRef, useCallback } from "react";
import { Loader2 } from "lucide-react";
import { api } from "@/lib/api";
import type { ExecutionLog } from "@/lib/types";

export const logTypeStyles: Record<
  string,
  { icon?: string; color: string; termColor: string; bold?: boolean }
> = {
  info: { icon: "ℹ", color: "text-slate-500", termColor: "text-slate-400" },
  action: { icon: "▶", color: "text-blue-600", termColor: "text-blue-400" },
  running: { icon: "⏳", color: "text-blue-500", termColor: "text-blue-300" },
  expected: { icon: "📋", color: "text-slate-600", termColor: "text-slate-500" },
  actual: { icon: "📌", color: "text-slate-700", termColor: "text-cyan-400" },
  verifying: { icon: "🔍", color: "text-purple-600", termColor: "text-purple-400" },
  precondition: { icon: "⚙", color: "text-amber-600", termColor: "text-amber-400" },
  precondition_ok: { icon: "✓", color: "text-emerald-600", termColor: "text-emerald-400" },
  step_start: { icon: "→", color: "text-primary", termColor: "text-white", bold: true },
  step_status: { icon: "●", color: "text-emerald-600", termColor: "text-emerald-400", bold: true },
  step_done: { icon: "✓", color: "text-emerald-600", termColor: "text-emerald-400" },
  summary: { icon: "📊", color: "text-slate-800", termColor: "text-yellow-300", bold: true },
  verdict: { icon: "🏁", color: "text-slate-900", termColor: "text-slate-900", bold: true },
  separator: { color: "text-slate-300", termColor: "text-slate-600" },
  finished: { icon: "✅", color: "text-emerald-700", termColor: "text-emerald-400", bold: true },
  error: { icon: "✗", color: "text-red-600", termColor: "text-red-400", bold: true },
  result_data: { color: "hidden", termColor: "hidden" },
};

export function TerminalLogLine({ log }: { log: ExecutionLog }) {
  const style = logTypeStyles[log.log_type] || { termColor: "text-slate-400" };
  if (style.termColor === "hidden") return null;

  const isStep = log.step_index >= 0;
  const isSep = log.log_type === "separator";
  const isVerdict = log.log_type === "verdict";
  const isFinished = log.log_type === "finished";

  if (isSep) {
    return (
      <div className="text-slate-600 py-0.5 text-xs">{log.message}</div>
    );
  }
  if (isVerdict) {
    const vColor = log.message === "passed" ? "text-emerald-400" : "text-red-400";
    return (
      <div className={`py-1 text-lg font-bold ${vColor} uppercase`}>
        VERDICT: {log.message}
      </div>
    );
  }
  if (isFinished) {
    return (
      <div className="py-1 text-emerald-400 font-semibold">{log.message}</div>
    );
  }

  return (
    <div className={`flex gap-2 py-0.5 ${isStep ? "pl-4" : ""} ${style.bold ? "font-semibold" : ""}`}>
      <span className="text-slate-600 text-xs w-16 shrink-0 text-right tabular-nums">
        {new Date(log.timestamp).toLocaleTimeString("en-US", {
          hour12: false,
          hour: "2-digit",
          minute: "2-digit",
          second: "2-digit",
        })}
      </span>
      {style.icon && <span className="w-4 text-center shrink-0 text-slate-500">{style.icon}</span>}
      <span className={`${style.termColor} break-words`}>{log.message}</span>
    </div>
  );
}

export function useExecutionStream(executionId: number | null) {
  const [logs, setLogs] = useState<ExecutionLog[]>([]);
  const [status, setStatus] = useState<"idle" | "running" | "done">("idle");
  const [verdict, setVerdict] = useState<string | null>(null);
  const [durationMs, setDurationMs] = useState(0);
  const eventSourceRef = useRef<EventSource | null>(null);

  const reset = useCallback(() => {
    if (eventSourceRef.current) eventSourceRef.current.close();
    setLogs([]);
    setStatus("idle");
    setVerdict(null);
    setDurationMs(0);
  }, []);

  useEffect(() => {
    if (!executionId) {
      reset();
      return;
    }

    setLogs([]);
    setStatus("running");
    setVerdict(null);

    const es = new EventSource(api.executions.streamUrl(executionId));
    eventSourceRef.current = es;
    let done = false;

    es.onmessage = (event) => {
      const data = JSON.parse(event.data);
      if (data.type === "done") {
        done = true;
        setVerdict(data.verdict);
        setDurationMs(data.duration);
        setStatus("done");
        es.close();
        return;
      }
      setLogs((prev) => [...prev, data as ExecutionLog]);
    };

    es.onerror = () => {
      es.close();
      if (!done) {
        pollExecution(executionId);
      }
    };

    const pollExecution = async (execId: number) => {
      let lastLogId = 0;
      const poll = async () => {
        try {
          const newLogs = await fetch(
            `${api.executions.streamUrl(execId).replace("/stream", `/logs?after=${lastLogId}`)}`
          ).then((r) => r.json());
          if (newLogs.length > 0) {
            setLogs((prev) => [...prev, ...newLogs]);
            lastLogId = newLogs[newLogs.length - 1].id;
          }
          const exec = await api.executions.get(execId);
          if (exec.status !== "pending" && exec.status !== "running") {
            setVerdict(exec.verdict || exec.status);
            setDurationMs(exec.duration_ms);
            setStatus("done");
            return;
          }
          setTimeout(poll, 1000);
        } catch {
          setStatus("done");
          setVerdict("error");
        }
      };
      poll();
    };

    return () => {
      es.close();
    };
  }, [executionId, reset]);

  return { logs, status, verdict, durationMs, reset };
}

export function LogTerminal({
  logs,
  running,
  maxHeight = "400px",
}: {
  logs: ExecutionLog[];
  running: boolean;
  maxHeight?: string;
}) {
  const logEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    logEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [logs]);

  return (
    <div className="bg-slate-900 rounded-lg overflow-hidden">
      <div className="flex items-center gap-2 px-4 py-2 bg-slate-800 border-b border-slate-700">
        <div className="flex gap-1.5">
          <div className="w-3 h-3 rounded-full bg-red-500/80" />
          <div className="w-3 h-3 rounded-full bg-yellow-500/80" />
          <div className="w-3 h-3 rounded-full bg-green-500/80" />
        </div>
        <span className="text-xs text-slate-400 font-mono ml-2">
          sdt — execution logs
        </span>
        {running && <Loader2 size={12} className="animate-spin text-blue-400 ml-auto" />}
      </div>
      <div className="px-4 py-3 overflow-y-auto font-mono text-sm" style={{ maxHeight }}>
        {logs.map((log) => (
          <TerminalLogLine key={log.id} log={log} />
        ))}
        {running && (
          <div className="flex items-center gap-2 py-1 text-blue-400">
            <Loader2 size={14} className="animate-spin" />
            <span className="text-sm animate-pulse">Executing...</span>
          </div>
        )}
        <div ref={logEndRef} />
      </div>
    </div>
  );
}
