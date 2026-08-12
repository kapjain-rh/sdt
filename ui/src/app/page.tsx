"use client";

import { useState, useEffect } from "react";
import {
  FolderKanban,
  FileText,
  ClipboardList,
  Play,
  Activity,
  Settings,
  TrendingUp,
  BarChart3,
} from "lucide-react";
import { api } from "@/lib/api";
import { useProjectContext } from "@/components/ProjectContext";
import { ProjectManager } from "@/components/ProjectManager";
import type { DashboardStats, TrendPoint } from "@/lib/types";

function StatCard({
  icon: Icon,
  label,
  value,
  color,
}: {
  icon: React.ElementType;
  label: string;
  value: number;
  color: string;
}) {
  return (
    <div className="bg-white border border-border rounded-lg p-5 shadow-sm">
      <div className="flex items-center justify-between mb-3">
        <div className={`p-2 rounded-lg ${color}`}>
          <Icon size={20} />
        </div>
      </div>
      <div className="text-2xl font-bold text-slate-800">{value}</div>
      <div className="text-xs text-muted uppercase tracking-wider mt-1">{label}</div>
    </div>
  );
}

function TrendChart({ data }: { data: TrendPoint[] }) {
  if (data.length === 0) {
    return (
      <div className="flex items-center justify-center h-48 text-muted text-sm">
        No execution data yet. Run some tests to see trends.
      </div>
    );
  }

  const maxVal = Math.max(...data.map((d) => d.total), 1);

  return (
    <div className="h-48 flex items-end gap-1 px-2">
      {data.map((d, i) => {
        const passH = (d.passed / maxVal) * 100;
        const failH = (d.failed / maxVal) * 100;
        const blockH = (d.blocked / maxVal) * 100;
        const skipH = (d.skipped / maxVal) * 100;
        return (
          <div key={i} className="flex-1 flex flex-col-reverse gap-px group relative" title={d.date}>
            <div className="bg-emerald-500 rounded-t-sm transition-all" style={{ height: `${passH}%` }} />
            <div className="bg-red-500 rounded-t-sm transition-all" style={{ height: `${failH}%` }} />
            <div className="bg-amber-500 rounded-t-sm transition-all" style={{ height: `${blockH}%` }} />
            <div className="bg-indigo-400 rounded-t-sm transition-all" style={{ height: `${skipH}%` }} />
            <div className="absolute -top-8 left-1/2 -translate-x-1/2 bg-slate-800 text-white text-[10px] px-1.5 py-0.5 rounded opacity-0 group-hover:opacity-100 transition-opacity whitespace-nowrap pointer-events-none">
              {d.date}: {d.passed}P {d.failed}F
            </div>
          </div>
        );
      })}
    </div>
  );
}

export default function DashboardPage() {
  const { selectedProject } = useProjectContext();
  const [stats, setStats] = useState<DashboardStats | null>(null);
  const [trends, setTrends] = useState<TrendPoint[]>([]);
  const [showProjects, setShowProjects] = useState(false);

  useEffect(() => {
    api.dashboard.stats().then(setStats).catch(() => {});
    api.dashboard
      .trends(selectedProject?.id, 30)
      .then(setTrends)
      .catch(() => {});
  }, [selectedProject]);

  return (
    <>
      <div className="border-b border-border bg-white px-8 py-4 flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold text-slate-800">Dashboard</h1>
          {selectedProject && (
            <p className="text-sm text-muted mt-0.5">{selectedProject.name}</p>
          )}
        </div>
        <button
          onClick={() => setShowProjects(true)}
          className="flex items-center gap-1.5 px-3 py-1.5 text-sm border border-border rounded-md hover:bg-slate-50 transition-colors text-slate-600"
        >
          <Settings size={15} /> Manage Projects
        </button>
      </div>

      <div className="p-8">
        {stats && (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-4 mb-8">
            <StatCard icon={FolderKanban} label="Projects" value={stats.projects} color="bg-blue-50 text-blue-600" />
            <StatCard icon={FileText} label="Test Cases" value={stats.total_cases} color="bg-emerald-50 text-emerald-600" />
            <StatCard icon={ClipboardList} label="Test Plans" value={stats.total_plans} color="bg-purple-50 text-purple-600" />
            <StatCard icon={Play} label="Total Runs" value={stats.total_runs} color="bg-orange-50 text-orange-600" />
            <StatCard icon={Activity} label="Active Runs" value={stats.active_runs} color="bg-cyan-50 text-cyan-600" />
          </div>
        )}

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          <div className="bg-white border border-border rounded-lg shadow-sm">
            <div className="flex items-center justify-between px-5 py-4 border-b border-border">
              <h2 className="font-semibold text-slate-800 flex items-center gap-2">
                <TrendingUp size={18} className="text-emerald-500" />
                Execution Trends (30 days)
              </h2>
            </div>
            <div className="p-5">
              <TrendChart data={trends} />
              <div className="flex gap-4 mt-4 justify-center">
                {[
                  { label: "Passed", color: "bg-emerald-500" },
                  { label: "Failed", color: "bg-red-500" },
                  { label: "Blocked", color: "bg-amber-500" },
                  { label: "Skipped", color: "bg-indigo-400" },
                ].map(({ label, color }) => (
                  <div key={label} className="flex items-center gap-1.5 text-xs text-muted">
                    <div className={`w-2.5 h-2.5 rounded-sm ${color}`} />
                    {label}
                  </div>
                ))}
              </div>
            </div>
          </div>

          <div className="bg-white border border-border rounded-lg shadow-sm">
            <div className="flex items-center justify-between px-5 py-4 border-b border-border">
              <h2 className="font-semibold text-slate-800 flex items-center gap-2">
                <BarChart3 size={18} className="text-blue-500" />
                Quick Stats
              </h2>
            </div>
            <div className="p-5 space-y-4">
              {stats ? (
                <>
                  <div className="flex items-center justify-between p-3 bg-slate-50 rounded-lg">
                    <span className="text-sm text-slate-600">Total Test Cases</span>
                    <span className="text-lg font-bold text-slate-800">{stats.total_cases}</span>
                  </div>
                  <div className="flex items-center justify-between p-3 bg-slate-50 rounded-lg">
                    <span className="text-sm text-slate-600">Active Test Plans</span>
                    <span className="text-lg font-bold text-slate-800">{stats.total_plans}</span>
                  </div>
                  <div className="flex items-center justify-between p-3 bg-slate-50 rounded-lg">
                    <span className="text-sm text-slate-600">Test Runs Executed</span>
                    <span className="text-lg font-bold text-slate-800">{stats.total_runs}</span>
                  </div>
                  <div className="flex items-center justify-between p-3 bg-emerald-50 rounded-lg">
                    <span className="text-sm text-emerald-700">Currently Active Runs</span>
                    <span className="text-lg font-bold text-emerald-700">{stats.active_runs}</span>
                  </div>
                </>
              ) : (
                <div className="text-center text-muted text-sm py-8">Loading...</div>
              )}
            </div>
          </div>
        </div>
      </div>

      <ProjectManager open={showProjects} onClose={() => setShowProjects(false)} />
    </>
  );
}
