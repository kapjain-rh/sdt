"use client";

import { useState, useEffect } from "react";
import {
  FileText,
  Database,
  ChevronDown,
  ChevronRight,
  CheckCircle2,
  XCircle,
  AlertCircle,
  Wrench,
  Circle,
  Clock,
  Cpu,
  Package,
  Trash2,
} from "lucide-react";
import { EmptyState } from "./EmptyState";
import type {
  CaseCache,
  CachedPlan,
  CachedPlanPhase,
  CachedPlanStep,
  CachedResult,
  CachePhaseResult,
  CacheStepResult,
  CacheFixtureResult,
  CacheFixturePlan,
} from "@/lib/types";

function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`;
}

function formatDuration(ns: number): string {
  if (ns <= 0) return "-";
  const ms = ns / 1_000_000;
  if (ms < 1000) return `${ms.toFixed(0)}ms`;
  const s = ms / 1000;
  if (s < 60) return `${s.toFixed(1)}s`;
  const m = Math.floor(s / 60);
  const rem = s % 60;
  return `${m}m ${rem.toFixed(0)}s`;
}

function formatTime(iso: string): string {
  if (!iso) return "-";
  try {
    return new Date(iso).toLocaleString(undefined, {
      month: "short", day: "numeric", hour: "2-digit", minute: "2-digit",
    });
  } catch { return iso; }
}

function StatusIcon({ status }: { status: string }) {
  const s = status.toUpperCase();
  if (s === "PASSED" || s === "SUCCESS") return <CheckCircle2 size={14} className="text-emerald-400" />;
  if (s === "FAILED" || s === "ERROR") return <XCircle size={14} className="text-red-400" />;
  if (s === "SKIPPED") return <AlertCircle size={14} className="text-slate-400" />;
  return <AlertCircle size={14} className="text-amber-400" />;
}

function statusColor(status: string): string {
  const s = status.toUpperCase();
  if (s === "PASSED" || s === "SUCCESS") return "text-emerald-400";
  if (s === "FAILED" || s === "ERROR") return "text-red-400";
  if (s === "SKIPPED") return "text-slate-500";
  return "text-amber-400";
}

function PlanStepLine({ step }: { step: CachedPlanStep }) {
  const [open, setOpen] = useState(false);
  const hasParams = step.parameters && Object.keys(step.parameters).length > 0;
  return (
    <div>
      <div
        className="flex items-start gap-2 min-h-[24px] hover:bg-[#2a2d3a] transition-colors cursor-pointer pl-[72px] pr-3 py-0.5"
        onClick={() => setOpen(!open)}
      >
        <Circle size={5} className="text-pink-400 fill-current mt-1.5 shrink-0" />
        <div className="flex-1 min-w-0">
          <span className="text-slate-300 text-sm font-mono">{step.description}</span>
          <div className="flex items-center gap-3 mt-0.5">
            {step.tool_name && (
              <span className="flex items-center gap-1 text-[11px] text-violet-400">
                <Wrench size={10} /> {step.tool_name}
              </span>
            )}
            {step.on_failure && step.on_failure !== "fail" && (
              <span className="text-[11px] text-amber-400">on_failure: {step.on_failure}</span>
            )}
            {hasParams && <span className="text-[10px] text-slate-600">{open ? "[-]" : `[${Object.keys(step.parameters).length} params]`}</span>}
          </div>
        </div>
      </div>
      {open && (
        <div className="pl-[88px] pr-3 pb-1">
          {hasParams && (
            <div className="border-l-2 border-[#313244] pl-2 mb-1">
              {Object.entries(step.parameters).map(([k, v]) => (
                <div key={k} className="text-[11px] font-mono leading-relaxed">
                  <span className="text-yellow-300">{k}</span>
                  <span className="text-slate-500">: </span>
                  <span className="text-green-300 break-all whitespace-pre-wrap">{typeof v === "string" ? v : JSON.stringify(v)}</span>
                </div>
              ))}
            </div>
          )}
          {step.expected_result && (
            <div className="text-[11px] text-slate-500 italic">expect: {step.expected_result}</div>
          )}
        </div>
      )}
    </div>
  );
}

function CachedPlanView({ plan }: { plan: CachedPlan }) {
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});
  const phases: CachedPlanPhase[] = plan.phases || [];
  const fixtures: CacheFixturePlan[] = plan.fixtures || [];
  const toggle = (key: string) => setExpanded((p) => ({ ...p, [key]: !p[key] }));

  return (
    <div className="py-1">
      <div className="flex flex-wrap items-center gap-3 px-4 py-1.5 text-[11px]">
        <span className="text-cyan-400 font-mono"><Cpu size={10} className="inline mr-1" />{plan.model || "-"}</span>
        <span className="text-green-300 font-mono"><Clock size={10} className="inline mr-1" />{formatTime(plan.created_at)}</span>
        <span className="text-slate-500 font-mono">{formatBytes(plan.file_size)}</span>
        <span className="text-slate-600 font-mono truncate max-w-[200px]" title={plan.spec_hash}>hash: {plan.spec_hash.slice(0, 16)}...</span>
      </div>

      {phases.map((phase) => {
        const key = `p-${phase.name}`;
        const isOpen = expanded[key] ?? true;
        const steps = phase.steps || [];
        return (
          <div key={key}>
            <div
              className="flex items-center h-7 hover:bg-[#2a2d3a] transition-colors cursor-pointer select-none px-4"
              onClick={() => toggle(key)}
            >
              <span className="text-slate-500 mr-1">{isOpen ? <ChevronDown size={14} /> : <ChevronRight size={14} />}</span>
              <span className="text-pink-400 font-bold font-mono text-sm">## {phase.name}</span>
              <span className="text-[10px] text-slate-600 ml-3 tabular-nums">{steps.length} step{steps.length !== 1 ? "s" : ""}</span>
            </div>
            {isOpen
              ? steps.map((step, si) => <PlanStepLine key={si} step={step} />)
              : steps.length > 0 && (
                  <div className="h-6 text-xs text-slate-600 italic pl-[72px] flex items-center">{steps.length} step{steps.length !== 1 ? "s" : ""} (collapsed)</div>
                )}
          </div>
        );
      })}

      {fixtures.length > 0 && (
        <>
          <div className="flex items-center h-7 px-4 mt-1">
            <span className="text-purple-400 font-bold font-mono text-sm">## Fixtures</span>
          </div>
          {fixtures.map((fx) => {
            const key = `fx-${fx.name}`;
            const isOpen = expanded[key] ?? false;
            const allSteps = [...(fx.create || []), ...(fx.ready_check || []), ...(fx.cleanup || [])];
            return (
              <div key={key}>
                <div className="flex items-center h-7 hover:bg-[#2a2d3a] cursor-pointer px-4" onClick={() => toggle(key)}>
                  <span className="text-slate-500 mr-1">{isOpen ? <ChevronDown size={14} /> : <ChevronRight size={14} />}</span>
                  <Package size={12} className="text-violet-400 mr-1" />
                  <span className="text-violet-300 font-mono text-sm font-semibold">{fx.name}</span>
                  <span className="text-[10px] text-slate-600 ml-2">{allSteps.length} steps</span>
                </div>
                {isOpen && (
                  <div className="pl-[72px] pr-3 pb-1">
                    {(["create", "ready_check", "cleanup"] as const).map((phase) => {
                      const steps = fx[phase] || [];
                      if (steps.length === 0) return null;
                      return (
                        <div key={phase} className="mb-1">
                          <div className="text-[11px] text-cyan-400 font-mono font-semibold">{phase}:</div>
                          {steps.map((s, i) => (
                            <div key={i} className="text-[11px] font-mono text-slate-300 ml-2">
                              <Circle size={4} className="inline text-violet-400 fill-current mr-1" />
                              {s.description}
                              {s.tool_name && <span className="text-violet-400 ml-1">[{s.tool_name}]</span>}
                            </div>
                          ))}
                        </div>
                      );
                    })}
                  </div>
                )}
              </div>
            );
          })}
        </>
      )}
    </div>
  );
}

function ResultStepRow({ step }: { step: CacheStepResult }) {
  const [open, setOpen] = useState(false);
  return (
    <div>
      <div
        className="flex items-center gap-2 min-h-[24px] hover:bg-[#2a2d3a] transition-colors cursor-pointer pl-[72px] pr-3"
        onClick={() => setOpen(!open)}
      >
        <StatusIcon status={step.status} />
        <span className="text-slate-300 text-sm font-mono flex-1 truncate">{step.description}</span>
        {step.tool_name && <span className="flex items-center gap-1 text-[11px] text-violet-400 shrink-0"><Wrench size={10} /> {step.tool_name}</span>}
        <span className={`text-[11px] ${statusColor(step.status)} shrink-0`}>{step.status}</span>
        {step.duration > 0 && <span className="text-[10px] text-slate-600 shrink-0">{formatDuration(step.duration)}</span>}
      </div>
      {open && (step.output || step.error) && (
        <div className="pl-[88px] pr-3 pb-1">
          {step.error && <div className="text-[11px] text-red-300 font-mono whitespace-pre-wrap break-all">{step.error}</div>}
          {step.output && (
            <pre className="text-[11px] text-slate-400 font-mono whitespace-pre-wrap break-all max-h-48 overflow-auto bg-[#181825] rounded p-1 mt-0.5">
              {step.output.length > 2000 ? step.output.slice(0, 2000) + "\n... (truncated)" : step.output}
            </pre>
          )}
        </div>
      )}
    </div>
  );
}

function CachedResultView({ result }: { result: CachedResult }) {
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});
  const phases = result.phase_results || [];
  const fixtures = result.fixture_results || [];
  const toggle = (key: string) => setExpanded((p) => ({ ...p, [key]: !p[key] }));

  return (
    <div className="py-1">
      <div className="flex flex-wrap items-center gap-3 px-4 py-1.5 text-[11px]">
        <span className={`font-mono font-semibold ${statusColor(result.status)}`}>{result.status}</span>
        <span className="text-orange-300 font-mono">{formatDuration(result.duration)}</span>
        <span className="text-green-300 font-mono"><Clock size={10} className="inline mr-1" />{formatTime(result.start_time)}</span>
        <span className="text-slate-500 font-mono">{formatBytes(result.file_size)}</span>
      </div>

      {result.error && (
        <div className="mx-4 mb-2 p-2 bg-red-900/20 border border-red-900/40 rounded">
          <span className="text-red-400 font-mono text-[11px] font-semibold">error: </span>
          <span className="text-red-300 font-mono text-[11px] break-all">{result.error}</span>
        </div>
      )}

      {phases.map((phase: CachePhaseResult, pi: number) => {
        const key = `rp-${pi}`;
        const isOpen = expanded[key] ?? true;
        const steps = phase.step_results || [];
        return (
          <div key={key}>
            <div className="flex items-center h-7 hover:bg-[#2a2d3a] cursor-pointer select-none px-4" onClick={() => toggle(key)}>
              <span className="text-slate-500 mr-1">{isOpen ? <ChevronDown size={14} /> : <ChevronRight size={14} />}</span>
              <StatusIcon status={phase.status} />
              <span className="text-pink-400 font-bold font-mono text-sm ml-1">## {phase.phase}</span>
              <span className={`text-[11px] ml-2 ${statusColor(phase.status)}`}>{phase.status}</span>
              <span className="text-[10px] text-slate-600 ml-2 tabular-nums">{steps.length} step{steps.length !== 1 ? "s" : ""}</span>
            </div>
            {phase.error && <div className="pl-[72px] pr-3 text-[11px] text-red-300 font-mono">{phase.error}</div>}
            {isOpen
              ? steps.map((step: CacheStepResult, si: number) => <ResultStepRow key={si} step={step} />)
              : steps.length > 0 && (
                  <div className="h-6 text-xs text-slate-600 italic pl-[72px] flex items-center">{steps.length} step{steps.length !== 1 ? "s" : ""} (collapsed)</div>
                )}
          </div>
        );
      })}

      {fixtures.map((fx: CacheFixtureResult, fi: number) => {
        const key = `rfx-${fi}`;
        const isOpen = expanded[key] ?? false;
        return (
          <div key={key}>
            <div className="flex items-center h-7 hover:bg-[#2a2d3a] cursor-pointer px-4" onClick={() => toggle(key)}>
              <span className="text-slate-500 mr-1">{isOpen ? <ChevronDown size={14} /> : <ChevronRight size={14} />}</span>
              <StatusIcon status={fx.status} />
              <Package size={12} className="text-violet-400 ml-1 mr-1" />
              <span className="text-violet-300 font-mono text-sm font-semibold">{fx.name}</span>
              <span className={`text-[11px] ml-2 ${statusColor(fx.status)}`}>{fx.status}</span>
            </div>
            {fx.error && <div className="pl-[72px] pr-3 text-[11px] text-red-300 font-mono">{fx.error}</div>}
            {isOpen && (
              <div className="pl-[72px] pr-3 pb-1">
                {(["create", "ready_check", "cleanup"] as const).map((phase) => {
                  const steps = fx[phase] || [];
                  if (steps.length === 0) return null;
                  return (
                    <div key={phase} className="mb-1">
                      <div className="text-[11px] text-cyan-400 font-mono font-semibold">{phase}:</div>
                      {steps.map((s: CacheStepResult, i: number) => (
                        <div key={i} className="flex items-center gap-2 text-[11px] font-mono ml-2">
                          <StatusIcon status={s.status} />
                          <span className="text-slate-300">{s.description}</span>
                          <span className={statusColor(s.status)}>{s.status}</span>
                        </div>
                      ))}
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}

export function CachePanel({ cache, loading, emptyMessage, onClear }: {
  cache: CaseCache | null;
  loading: boolean;
  emptyMessage?: string;
  onClear?: () => void;
}) {
  const [cacheTab, setCacheTab] = useState<"plan" | "results">("plan");
  const [selectedResultIdx, setSelectedResultIdx] = useState(0);
  const [clearing, setClearing] = useState(false);

  if (loading) {
    return <div className="flex-1 flex items-center justify-center text-sm text-slate-600">Loading cache...</div>;
  }

  const plans = cache?.plans || [];
  const results = cache?.results || [];
  const hasPlan = plans.length > 0;
  const hasResults = results.length > 0;

  if (!hasPlan && !hasResults) {
    return (
      <div className="flex-1 flex items-center justify-center bg-[#1e1e2e]">
        <EmptyState
          icon={<Database size={40} className="text-slate-600" />}
          title="No Cache"
          description={emptyMessage || "Run this spec to generate a cached execution plan and results."}
        />
      </div>
    );
  }

  return (
    <div className="flex-1 flex flex-col bg-[#1e1e2e] overflow-hidden">
      <div className="flex items-center border-b border-[#313244] px-4">
        {hasPlan && (
          <button
            onClick={() => setCacheTab("plan")}
            className={`px-3 py-1.5 text-xs font-semibold transition-colors ${
              cacheTab === "plan" ? "text-blue-400 border-b-2 border-blue-400" : "text-slate-500 hover:text-slate-300"
            }`}
          >
            <FileText size={12} className="inline mr-1" />
            Execution Plan{plans.length > 1 ? ` (${plans.length})` : ""}
          </button>
        )}
        {hasResults && (
          <button
            onClick={() => setCacheTab("results")}
            className={`px-3 py-1.5 text-xs font-semibold transition-colors ${
              cacheTab === "results" ? "text-blue-400 border-b-2 border-blue-400" : "text-slate-500 hover:text-slate-300"
            }`}
          >
            <Database size={12} className="inline mr-1" />
            Results ({results.length})
          </button>
        )}
        {onClear && hasPlan && (
          <button
            onClick={async () => {
              setClearing(true);
              try { await onClear(); } finally { setClearing(false); }
            }}
            disabled={clearing}
            className="ml-auto px-2 py-1 text-[11px] text-red-400 hover:text-red-300 hover:bg-red-400/10 rounded transition-colors disabled:opacity-50"
          >
            <Trash2 size={12} className="inline mr-1" />
            {clearing ? "Clearing..." : "Clear Cache"}
          </button>
        )}
      </div>

      <div className="flex-1 overflow-auto">
        {cacheTab === "plan" && hasPlan && (
          plans.length === 1 ? (
            <CachedPlanView plan={plans[0]} />
          ) : (
            <div>
              {plans.map((plan, i) => (
                <div key={i} className="border-b border-[#313244]">
                  <div className="px-4 py-1 text-[11px] text-slate-500 font-mono bg-[#232336]">
                    Plan: {plan.spec_name || plan.spec_hash.slice(0, 16)}
                  </div>
                  <CachedPlanView plan={plan} />
                </div>
              ))}
            </div>
          )
        )}
        {cacheTab === "results" && hasResults && (
          <div>
            {results.length > 1 && (
              <div className="flex items-center gap-2 px-4 py-1.5 border-b border-[#313244]">
                <span className="text-[11px] text-slate-500">Run:</span>
                {results.map((r, i) => (
                  <button
                    key={i}
                    onClick={() => setSelectedResultIdx(i)}
                    className={`flex items-center gap-1 px-2 py-0.5 text-[11px] rounded transition-colors ${
                      i === selectedResultIdx
                        ? "bg-[#313244] text-slate-200"
                        : "text-slate-500 hover:text-slate-300"
                    }`}
                  >
                    <StatusIcon status={r.status} />
                    {formatTime(r.timestamp || r.start_time)}
                  </button>
                ))}
              </div>
            )}
            <CachedResultView result={results[selectedResultIdx]} />
          </div>
        )}
      </div>
    </div>
  );
}
