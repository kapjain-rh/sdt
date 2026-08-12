"use client";

import { useState, useEffect, useCallback } from "react";
import { Play, Trash2, Eye, CheckCircle, BarChart3, Variable } from "lucide-react";
import { api } from "@/lib/api";
import { useProjectContext } from "@/components/ProjectContext";
import { Badge } from "@/components/Badge";
import { EmptyState } from "@/components/EmptyState";
import type { TestRun } from "@/lib/types";
import { formatDate, passRate } from "@/lib/utils";
import Link from "next/link";

function ProgressBar({ run }: { run: TestRun }) {
  const { total, passed, failed, blocked, skipped } = run;
  if (total === 0) return <div className="text-xs text-muted">No cases</div>;
  const executed = passed + failed + blocked + skipped;
  const pct = Math.round((executed / total) * 100);

  return (
    <div className="w-full">
      <div className="flex justify-between text-xs text-muted mb-1">
        <span>{executed}/{total} executed</span>
        <span>{pct}%</span>
      </div>
      <div className="h-2 bg-slate-100 rounded-full overflow-hidden flex">
        {passed > 0 && <div className="bg-emerald-500 h-full" style={{ width: `${(passed / total) * 100}%` }} />}
        {failed > 0 && <div className="bg-red-500 h-full" style={{ width: `${(failed / total) * 100}%` }} />}
        {blocked > 0 && <div className="bg-amber-500 h-full" style={{ width: `${(blocked / total) * 100}%` }} />}
        {skipped > 0 && <div className="bg-indigo-400 h-full" style={{ width: `${(skipped / total) * 100}%` }} />}
      </div>
    </div>
  );
}

export default function RunsPage() {
  const { selectedProject } = useProjectContext();
  const [runs, setRuns] = useState<TestRun[]>([]);
  const [loading, setLoading] = useState(true);

  const loadRuns = useCallback(async () => {
    if (!selectedProject) return;
    setLoading(true);
    try {
      const data = await api.runs.list(selectedProject.id);
      setRuns(data);
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  }, [selectedProject]);

  useEffect(() => {
    loadRuns();
  }, [loadRuns]);

  useEffect(() => {
    const hasActive = runs.some((r) => r.status === "in_progress");
    if (!hasActive) return;
    const poll = setInterval(() => {
      if (!selectedProject) return;
      api.runs.list(selectedProject.id).then(setRuns).catch(() => {});
    }, 5000);
    return () => clearInterval(poll);
  }, [runs, selectedProject]);

  const handleComplete = async (id: number) => {
    if (!confirm("Mark this run as completed?")) return;
    await api.runs.complete(id);
    await loadRuns();
  };

  const handleDelete = async (id: number) => {
    if (!confirm("Delete this test run?")) return;
    await api.runs.delete(id);
    await loadRuns();
  };

  if (!selectedProject) {
    return (
      <div className="p-8">
        <EmptyState
          icon={<Play size={48} />}
          title="No Project Selected"
          description="Select or create a project from the sidebar to view test runs."
        />
      </div>
    );
  }

  return (
    <>
      <div className="border-b border-border bg-white px-8 py-4 flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold text-slate-800">Test Runs</h1>
          <p className="text-sm text-muted mt-0.5">
            {selectedProject.name} &middot; {runs.length} run{runs.length !== 1 ? "s" : ""}
          </p>
        </div>
        <Link
          href="/plans"
          className="flex items-center gap-1.5 px-4 py-2 bg-primary text-white text-sm rounded-md hover:bg-primary-hover transition-colors"
        >
          <Play size={16} /> Create from Plan
        </Link>
      </div>

      <div className="p-8">
        {loading ? (
          <div className="text-center py-12 text-muted text-sm">Loading test runs...</div>
        ) : runs.length === 0 ? (
          <EmptyState
            icon={<Play size={48} />}
            title="No Test Runs"
            description="Create a test run from a test plan to start executing tests."
            action={
              <Link
                href="/plans"
                className="flex items-center gap-1.5 px-4 py-2 bg-primary text-white text-sm rounded-md hover:bg-primary-hover transition-colors"
              >
                Go to Test Plans
              </Link>
            }
          />
        ) : (
          <div className="space-y-4">
            {runs.map((run) => (
              <div key={run.id} className="bg-white border border-border rounded-lg shadow-sm p-5">
                <div className="flex items-start justify-between mb-4">
                  <div className="flex-1">
                    <div className="flex items-center gap-3 mb-1">
                      <h3 className="font-semibold text-slate-800">{run.name}</h3>
                      <Badge value={run.status} />
                    </div>
                    <div className="flex items-center gap-4 text-xs text-muted">
                      <span>Plan: {run.plan_name || "—"}</span>
                      {run.build && <span>Build: {run.build}</span>}
                      {run.environment && <span>Env: {run.environment}</span>}
                      <span>Created: {formatDate(run.created_at)}</span>
                    </div>
                    {run.env_vars && Object.keys(run.env_vars).length > 0 && (
                      <div className="flex items-center gap-1.5 mt-1.5 flex-wrap">
                        <Variable size={12} className="text-slate-400 shrink-0" />
                        {Object.entries(run.env_vars).map(([k, v]) => (
                          <span key={k} className="text-[11px] font-mono bg-slate-100 text-slate-600 px-1.5 py-0.5 rounded border border-slate-200">
                            {k}={v}
                          </span>
                        ))}
                      </div>
                    )}
                  </div>
                  <div className="flex items-center gap-2">
                    {run.status === "not_started" && (
                      <a
                        href={`/runs/${run.id}`}
                        className="flex items-center gap-1 px-3 py-1.5 text-xs bg-success text-white rounded-md hover:opacity-90 transition-opacity"
                      >
                        <Play size={12} /> Run
                      </a>
                    )}
                    {run.status === "in_progress" && (
                      <>
                        <a
                          href={`/runs/${run.id}`}
                          className="flex items-center gap-1 px-3 py-1.5 text-xs bg-primary text-white rounded-md hover:bg-primary-hover transition-colors"
                        >
                          <Eye size={12} /> Progress
                        </a>
                        <a
                          href={`/runs/${run.id}/execute`}
                          className="flex items-center gap-1 px-3 py-1.5 text-xs border border-border rounded-md hover:bg-slate-50 transition-colors text-slate-600"
                        >
                          <Play size={12} /> Manual
                        </a>
                        <button
                          onClick={() => handleComplete(run.id)}
                          className="flex items-center gap-1 px-3 py-1.5 text-xs border border-border rounded-md hover:bg-slate-50 transition-colors text-slate-600"
                        >
                          <CheckCircle size={12} /> Complete
                        </button>
                      </>
                    )}
                    {run.status === "completed" && (
                      <a
                        href={`/runs/${run.id}`}
                        className="flex items-center gap-1 px-3 py-1.5 text-xs border border-border rounded-md hover:bg-slate-50 transition-colors text-slate-600"
                      >
                        <Eye size={12} /> View Results
                      </a>
                    )}
                    <button
                      onClick={() => handleDelete(run.id)}
                      className="p-1.5 text-muted hover:text-danger transition-colors rounded hover:bg-red-50"
                    >
                      <Trash2 size={14} />
                    </button>
                  </div>
                </div>

                <ProgressBar run={run} />

                <div className="flex gap-4 mt-3">
                  {[
                    { label: "Passed", val: run.passed, color: "text-emerald-600", bg: "bg-emerald-50" },
                    { label: "Failed", val: run.failed, color: "text-red-600", bg: "bg-red-50" },
                    { label: "Blocked", val: run.blocked, color: "text-amber-600", bg: "bg-amber-50" },
                    { label: "Skipped", val: run.skipped, color: "text-indigo-600", bg: "bg-indigo-50" },
                    { label: "Untested", val: run.total - run.passed - run.failed - run.blocked - run.skipped, color: "text-slate-500", bg: "bg-slate-50" },
                  ].map(({ label, val, color, bg }) => (
                    <div key={label} className={`flex items-center gap-1.5 px-2 py-1 rounded text-xs ${bg} ${color}`}>
                      <span className="font-bold">{val}</span>
                      <span>{label}</span>
                    </div>
                  ))}
                  <div className="ml-auto flex items-center gap-1 text-xs text-muted">
                    <BarChart3 size={12} />
                    <span>Pass rate: {passRate(run.passed, run.total)}%</span>
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </>
  );
}
