"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { useParams, useRouter } from "next/navigation";
import {
  ArrowLeft,
  ArrowRight,
  CheckCircle,
  XCircle,
  Ban,
  SkipForward,
  Clock,
  AlertTriangle,
  ChevronLeft,
  ListChecks,
} from "lucide-react";
import { api } from "@/lib/api";
import { Badge } from "@/components/Badge";
import type { TestRun, TestResult } from "@/lib/types";
import { formatDuration, passRate } from "@/lib/utils";

type Verdict = "passed" | "failed" | "blocked" | "skipped";
type StepStatus = "untested" | "passed" | "failed" | "skipped";

const verdictConfig: Record<Verdict, { icon: React.ElementType; color: string; bg: string; label: string }> = {
  passed: { icon: CheckCircle, color: "text-white", bg: "bg-emerald-500 hover:bg-emerald-600", label: "Pass" },
  failed: { icon: XCircle, color: "text-white", bg: "bg-red-500 hover:bg-red-600", label: "Fail" },
  blocked: { icon: Ban, color: "text-white", bg: "bg-amber-500 hover:bg-amber-600", label: "Blocked" },
  skipped: { icon: SkipForward, color: "text-white", bg: "bg-indigo-500 hover:bg-indigo-600", label: "Skip" },
};

const stepStatusConfig: Record<StepStatus, { bg: string; text: string }> = {
  untested: { bg: "bg-slate-100 border-slate-200", text: "text-slate-500" },
  passed: { bg: "bg-emerald-50 border-emerald-200", text: "text-emerald-700" },
  failed: { bg: "bg-red-50 border-red-200", text: "text-red-700" },
  skipped: { bg: "bg-indigo-50 border-indigo-200", text: "text-indigo-700" },
};

export default function ExecuteClient() {
  const params = useParams();
  const router = useRouter();
  const runId = Number(params.id);

  const [run, setRun] = useState<TestRun | null>(null);
  const [results, setResults] = useState<TestResult[]>([]);
  const [currentIndex, setCurrentIndex] = useState(0);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [sidebarOpen, setSidebarOpen] = useState(true);

  const [stepStatuses, setStepStatuses] = useState<StepStatus[]>([]);
  const [stepActuals, setStepActuals] = useState<string[]>([]);
  const [comment, setComment] = useState("");
  const [executedBy, setExecutedBy] = useState("");

  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const startTimeRef = useRef<number>(Date.now());
  const [elapsed, setElapsed] = useState(0);

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const [runData, resultsData] = await Promise.all([
        api.runs.get(runId),
        api.runs.results(runId),
      ]);
      setRun(runData);
      setResults(resultsData);
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  }, [runId]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  useEffect(() => {
    const stored = localStorage.getItem("tcms_executor");
    if (stored) setExecutedBy(stored);
  }, []);

  useEffect(() => {
    if (results.length === 0) return;
    const r = results[currentIndex];
    if (!r) return;

    const numSteps = r.case?.steps?.length || 0;
    setStepStatuses(
      r.step_results
        ? Array.from({ length: numSteps }, (_, i) => {
            const sr = r.step_results?.find((s) => s.step_index === i);
            return (sr?.status as StepStatus) || "untested";
          })
        : Array(numSteps).fill("untested")
    );
    setStepActuals(
      r.step_results
        ? Array.from({ length: numSteps }, (_, i) => {
            const sr = r.step_results?.find((s) => s.step_index === i);
            return sr?.actual_result || "";
          })
        : Array(numSteps).fill("")
    );
    setComment(r.comment || "");

    startTimeRef.current = Date.now();
    setElapsed(0);
    if (timerRef.current) clearInterval(timerRef.current);
    timerRef.current = setInterval(() => {
      setElapsed(Date.now() - startTimeRef.current);
    }, 1000);

    return () => {
      if (timerRef.current) clearInterval(timerRef.current);
    };
  }, [currentIndex, results]);

  const setStepStatus = (idx: number, status: StepStatus) => {
    setStepStatuses((prev) => {
      const next = [...prev];
      next[idx] = next[idx] === status ? "untested" : status;
      return next;
    });
  };

  const setStepActual = (idx: number, value: string) => {
    setStepActuals((prev) => {
      const next = [...prev];
      next[idx] = value;
      return next;
    });
  };

  const submitVerdict = async (verdict: Verdict) => {
    const r = results[currentIndex];
    if (!r) return;

    if (executedBy) {
      localStorage.setItem("tcms_executor", executedBy);
    }

    setSaving(true);
    try {
      await api.results.update(r.id, {
        status: verdict,
        comment,
        executed_by: executedBy,
        duration_ms: Date.now() - startTimeRef.current,
        step_results: stepStatuses.map((status, i) => ({
          step_index: i,
          status,
          actual_result: stepActuals[i] || "",
          comment: "",
        })),
      });

      const updatedResults = [...results];
      updatedResults[currentIndex] = {
        ...r,
        status: verdict,
        comment,
        executed_by: executedBy,
      };
      setResults(updatedResults);

      if (currentIndex < results.length - 1) {
        setCurrentIndex(currentIndex + 1);
      }
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return <div className="p-8 text-center text-muted">Loading test execution...</div>;
  }

  if (!run || results.length === 0) {
    return (
      <div className="p-8 text-center text-muted">
        <p>No test cases found for this run.</p>
        <button
          onClick={() => router.push("/runs")}
          className="mt-4 text-primary hover:underline"
        >
          Back to Runs
        </button>
      </div>
    );
  }

  const currentResult = results[currentIndex];
  const testCase = currentResult.case;
  const steps = testCase?.steps || [];
  const executed = results.filter((r) => r.status !== "untested").length;

  return (
    <div className="flex h-screen ml-[-240px] md:ml-0">
      {/* Case Navigation Sidebar */}
      {sidebarOpen && (
        <div className="w-72 bg-white border-r border-border flex flex-col shrink-0">
          <div className="p-4 border-b border-border">
            <button
              onClick={() => router.push("/runs")}
              className="flex items-center gap-1 text-sm text-muted hover:text-slate-700 mb-3 transition-colors"
            >
              <ChevronLeft size={14} /> Back to Runs
            </button>
            <h2 className="font-semibold text-slate-800 text-sm truncate">{run.name}</h2>
            <div className="flex items-center gap-2 mt-1">
              <Badge value={run.status} />
              <span className="text-xs text-muted">{executed}/{results.length} done</span>
            </div>
            <div className="mt-3 h-1.5 bg-slate-100 rounded-full overflow-hidden flex">
              {results.filter((r) => r.status === "passed").length > 0 && (
                <div className="bg-emerald-500 h-full" style={{ width: `${(results.filter((r) => r.status === "passed").length / results.length) * 100}%` }} />
              )}
              {results.filter((r) => r.status === "failed").length > 0 && (
                <div className="bg-red-500 h-full" style={{ width: `${(results.filter((r) => r.status === "failed").length / results.length) * 100}%` }} />
              )}
              {results.filter((r) => r.status === "blocked").length > 0 && (
                <div className="bg-amber-500 h-full" style={{ width: `${(results.filter((r) => r.status === "blocked").length / results.length) * 100}%` }} />
              )}
              {results.filter((r) => r.status === "skipped").length > 0 && (
                <div className="bg-indigo-400 h-full" style={{ width: `${(results.filter((r) => r.status === "skipped").length / results.length) * 100}%` }} />
              )}
            </div>
          </div>
          <div className="flex-1 overflow-y-auto">
            {results.map((r, i) => (
              <button
                key={r.id}
                onClick={() => setCurrentIndex(i)}
                className={`w-full text-left px-4 py-3 border-b border-border/50 transition-colors ${
                  i === currentIndex
                    ? "bg-primary/5 border-l-[3px] border-l-primary"
                    : "hover:bg-slate-50"
                }`}
              >
                <div className="flex items-center gap-2 mb-0.5">
                  <span className="text-xs text-muted font-mono">TC-{r.case_id}</span>
                  <Badge value={r.status} />
                </div>
                <div className="text-sm text-slate-700 truncate">{r.case?.title}</div>
              </button>
            ))}
          </div>
          <div className="p-3 border-t border-border bg-slate-50 text-xs text-center text-muted">
            Pass rate: {passRate(results.filter((r) => r.status === "passed").length, results.length)}%
          </div>
        </div>
      )}

      {/* Main Execution Area */}
      <div className="flex-1 flex flex-col overflow-hidden">
        {/* Top Bar */}
        <div className="flex items-center justify-between px-6 py-3 bg-white border-b border-border">
          <div className="flex items-center gap-4">
            <button
              onClick={() => setSidebarOpen(!sidebarOpen)}
              className="p-1.5 text-muted hover:text-slate-700 hover:bg-slate-100 rounded transition-colors"
            >
              <ListChecks size={18} />
            </button>
            <div>
              <span className="text-sm font-semibold text-slate-800">
                {currentIndex + 1} / {results.length}
              </span>
              <span className="text-xs text-muted ml-2">TC-{currentResult.case_id}</span>
            </div>
          </div>
          <div className="flex items-center gap-4">
            <div className="flex items-center gap-1.5 text-muted">
              <Clock size={14} />
              <span className="font-mono text-sm">{formatDuration(elapsed)}</span>
            </div>
            <input
              value={executedBy}
              onChange={(e) => setExecutedBy(e.target.value)}
              placeholder="Your name"
              className="w-32 border border-border rounded px-2 py-1 text-xs focus:outline-none focus:ring-1 focus:ring-primary"
            />
          </div>
        </div>

        {/* Test Case Content */}
        <div className="flex-1 overflow-y-auto p-6">
          <div className="max-w-4xl mx-auto">
            {/* Case Header */}
            <div className="mb-6">
              <div className="flex items-center gap-2 mb-2">
                <Badge value={testCase.priority} type="priority" />
                <Badge value={testCase.status} />
              </div>
              <h2 className="text-lg font-semibold text-slate-800">{testCase.title}</h2>
              {testCase.description && (
                <p className="text-sm text-muted mt-1">{testCase.description}</p>
              )}
            </div>

            {/* Preconditions */}
            {testCase.preconditions && (
              <div className="mb-6 bg-amber-50 border border-amber-200 rounded-lg p-4">
                <div className="flex items-center gap-1.5 text-xs font-semibold text-amber-700 uppercase mb-1">
                  <AlertTriangle size={12} /> Preconditions
                </div>
                <p className="text-sm text-amber-900 whitespace-pre-wrap">{testCase.preconditions}</p>
              </div>
            )}

            {/* Steps */}
            {steps.length > 0 && (
              <div className="mb-6">
                <h3 className="text-sm font-semibold text-slate-700 mb-3">Test Steps</h3>
                <div className="space-y-3">
                  {steps.map((step, i) => {
                    const status = stepStatuses[i] || "untested";
                    const config = stepStatusConfig[status];
                    return (
                      <div key={i} className={`border rounded-lg p-4 transition-colors ${config.bg}`}>
                        <div className="flex items-start justify-between mb-2">
                          <div className="flex items-center gap-2">
                            <span className={`text-xs font-bold px-1.5 py-0.5 rounded ${config.text} bg-white/80`}>
                              Step {i + 1}
                            </span>
                          </div>
                          <div className="flex gap-1">
                            {(["passed", "failed", "skipped"] as StepStatus[]).map((s) => {
                              const active = status === s;
                              const colors: Record<string, string> = {
                                passed: active ? "bg-emerald-500 text-white border-emerald-500" : "border-slate-300 text-slate-500 hover:border-emerald-400 hover:text-emerald-600",
                                failed: active ? "bg-red-500 text-white border-red-500" : "border-slate-300 text-slate-500 hover:border-red-400 hover:text-red-600",
                                skipped: active ? "bg-indigo-500 text-white border-indigo-500" : "border-slate-300 text-slate-500 hover:border-indigo-400 hover:text-indigo-600",
                              };
                              return (
                                <button
                                  key={s}
                                  onClick={() => setStepStatus(i, s)}
                                  className={`px-2 py-0.5 text-[11px] font-semibold uppercase rounded border transition-colors ${colors[s]}`}
                                >
                                  {s}
                                </button>
                              );
                            })}
                          </div>
                        </div>
                        <div>
                          <p className="text-sm text-slate-800 whitespace-pre-wrap">{step}</p>
                        </div>
                        {(status === "failed" || stepActuals[i]) && (
                          <div className="mt-2">
                            <div className="text-[10px] text-muted uppercase font-semibold mb-0.5">Actual Result</div>
                            <input
                              value={stepActuals[i] || ""}
                              onChange={(e) => setStepActual(i, e.target.value)}
                              className="w-full border border-border/50 rounded px-2 py-1 text-sm bg-white focus:outline-none focus:ring-1 focus:ring-primary"
                              placeholder="What actually happened..."
                            />
                          </div>
                        )}
                      </div>
                    );
                  })}
                </div>
              </div>
            )}

            {/* Comment */}
            <div className="mb-6">
              <label className="text-sm font-semibold text-slate-700 mb-1 block">Comment</label>
              <textarea
                value={comment}
                onChange={(e) => setComment(e.target.value)}
                className="w-full border border-border rounded-lg px-3 py-2 text-sm min-h-[60px] resize-y focus:outline-none focus:ring-2 focus:ring-primary"
                placeholder="Add any notes about this test execution..."
              />
            </div>

            {/* Verdict Buttons */}
            <div className="bg-slate-50 border border-border rounded-lg p-4">
              <div className="text-xs font-semibold text-muted uppercase mb-3">Set Verdict</div>
              <div className="flex gap-3">
                {(Object.entries(verdictConfig) as [Verdict, typeof verdictConfig.passed][]).map(
                  ([verdict, { icon: Icon, color, bg, label }]) => (
                    <button
                      key={verdict}
                      onClick={() => submitVerdict(verdict)}
                      disabled={saving}
                      className={`flex items-center gap-2 px-6 py-3 rounded-lg font-semibold text-sm transition-all ${bg} ${color} disabled:opacity-50 shadow-sm hover:shadow-md`}
                    >
                      <Icon size={18} />
                      {label}
                    </button>
                  )
                )}
              </div>
            </div>
          </div>
        </div>

        {/* Navigation Footer */}
        <div className="flex items-center justify-between px-6 py-3 bg-white border-t border-border">
          <button
            onClick={() => setCurrentIndex(Math.max(0, currentIndex - 1))}
            disabled={currentIndex === 0}
            className="flex items-center gap-1 px-3 py-1.5 text-sm border border-border rounded-md hover:bg-slate-50 disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
          >
            <ArrowLeft size={14} /> Previous
          </button>
          <div className="flex gap-1">
            {results.map((r, i) => {
              const colors: Record<string, string> = {
                passed: "bg-emerald-500",
                failed: "bg-red-500",
                blocked: "bg-amber-500",
                skipped: "bg-indigo-400",
                untested: "bg-slate-200",
              };
              return (
                <button
                  key={i}
                  onClick={() => setCurrentIndex(i)}
                  className={`w-3 h-3 rounded-full transition-all ${colors[r.status] || colors.untested} ${
                    i === currentIndex ? "ring-2 ring-primary ring-offset-1 scale-125" : "hover:scale-110"
                  }`}
                  title={`TC-${r.case_id}: ${r.case?.title} (${r.status})`}
                />
              );
            })}
          </div>
          <button
            onClick={() => setCurrentIndex(Math.min(results.length - 1, currentIndex + 1))}
            disabled={currentIndex === results.length - 1}
            className="flex items-center gap-1 px-3 py-1.5 text-sm border border-border rounded-md hover:bg-slate-50 disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
          >
            Next <ArrowRight size={14} />
          </button>
        </div>
      </div>
    </div>
  );
}
