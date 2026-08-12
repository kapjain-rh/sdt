"use client";

import { cn } from "@/lib/utils";

const statusColors: Record<string, string> = {
  passed: "bg-emerald-100 text-emerald-800",
  active: "bg-emerald-100 text-emerald-800",
  completed: "bg-emerald-100 text-emerald-800",
  failed: "bg-red-100 text-red-800",
  deprecated: "bg-red-100 text-red-800",
  blocked: "bg-amber-100 text-amber-800",
  skipped: "bg-indigo-100 text-indigo-800",
  draft: "bg-indigo-100 text-indigo-800",
  verify: "bg-amber-100 text-amber-800",
  approved: "bg-emerald-100 text-emerald-800",
  configured: "bg-slate-100 text-slate-600",
  connected: "bg-emerald-100 text-emerald-800",
  disconnected: "bg-slate-100 text-slate-600",
  untested: "bg-slate-100 text-slate-600",
  not_started: "bg-slate-100 text-slate-600",
  in_progress: "bg-blue-100 text-blue-800",
};

const priorityColors: Record<string, string> = {
  Critical: "bg-red-100 text-red-800",
  High: "bg-orange-100 text-orange-800",
  Medium: "bg-yellow-100 text-yellow-800",
  Low: "bg-green-100 text-green-800",
};

export function Badge({ value, type = "status" }: { value: string; type?: "status" | "priority" }) {
  const colors = type === "priority" ? priorityColors : statusColors;
  return (
    <span
      className={cn(
        "inline-flex items-center px-2 py-0.5 rounded text-xs font-semibold uppercase tracking-wide",
        colors[value] || "bg-slate-100 text-slate-600"
      )}
    >
      {value.replace(/_/g, " ")}
    </span>
  );
}
