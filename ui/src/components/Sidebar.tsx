"use client";

import { useState, useEffect } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  LayoutDashboard,
  FolderKanban,
  FolderOpen,
  FileText,
  ClipboardList,
  Play,
  Wrench,
  Server,
  FlaskConical,
  Layers,
  Package,
  Globe,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { useProjectContext } from "./ProjectContext";
import { getBackendUrl, setBackendUrl } from "@/lib/api";

const nav = [
  { href: "/", label: "Dashboard", icon: LayoutDashboard },
  { href: "/suite", label: "Suite", icon: Layers },
  { href: "/groups", label: "Groups", icon: FolderOpen },
  { href: "/cases", label: "Test Cases", icon: FileText },
  { href: "/fixtures", label: "Fixtures", icon: Package },
  { href: "/plans", label: "Test Plans", icon: ClipboardList },
  { href: "/runs", label: "Test Runs", icon: Play },
  { href: "/tools", label: "Tools", icon: Wrench },
  { href: "/mcp-servers", label: "MCP Servers", icon: Server },
];

export function Sidebar() {
  const pathname = usePathname();
  const { projects, selectedProject, selectProject, refreshProjects } = useProjectContext();
  const [backendInput, setBackendInput] = useState("");
  const [backendSaved, setBackendSaved] = useState(false);

  useEffect(() => {
    setBackendInput(getBackendUrl());
  }, []);

  const applyBackend = () => {
    setBackendUrl(backendInput);
    setBackendSaved(true);
    setTimeout(() => setBackendSaved(false), 1500);
    refreshProjects();
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter") applyBackend();
  };

  return (
    <aside className="fixed top-0 left-0 bottom-0 w-60 bg-sidebar text-sidebar-text flex flex-col z-20">
      <div className="px-6 py-5 border-b border-white/8 flex items-center gap-2">
        <FlaskConical size={22} className="text-primary" />
        <span className="text-white font-bold text-lg tracking-tight">
          SDT<span className="text-primary">-TCMS</span>
        </span>
      </div>

      <nav className="flex-1 py-3">
        {nav.map(({ href, label, icon: Icon }) => {
          const active = href === "/" ? pathname === "/" : pathname.startsWith(href);
          return (
            <Link
              key={href}
              href={href}
              className={cn(
                "flex items-center gap-3 px-6 py-2.5 text-[0.9rem] transition-colors hover:bg-white/6 hover:text-sidebar-active",
                active && "bg-white/6 text-sidebar-active border-r-[3px] border-primary"
              )}
            >
              <Icon size={18} />
              <span>{label}</span>
            </Link>
          );
        })}
      </nav>

      {/* Backend URL */}
      <div className="px-4 py-3 border-t border-white/8">
        <label className="text-[0.7rem] uppercase tracking-wider text-sidebar-text/60 mb-1 block">
          Backend
        </label>
        <div className="flex items-center gap-1">
          <Globe size={14} className="text-sidebar-text/60 shrink-0" />
          <input
            value={backendInput}
            onChange={(e) => setBackendInput(e.target.value)}
            onKeyDown={handleKeyDown}
            onBlur={applyBackend}
            placeholder="default (proxy :8090)"
            className="w-full bg-white/8 border border-white/10 rounded-md px-2 py-1.5 text-xs text-white placeholder:text-sidebar-text/30 focus:outline-none focus:ring-1 focus:ring-primary font-mono"
          />
        </div>
        {backendSaved && (
          <span className="text-[0.65rem] text-emerald-400 mt-0.5 block">Connected</span>
        )}
        <p className="text-[0.6rem] text-sidebar-text/40 mt-1">
          e.g. http://localhost:9090/api
        </p>
      </div>

      {/* Project selector */}
      <div className="px-4 py-3 border-t border-white/8">
        <label className="text-[0.7rem] uppercase tracking-wider text-sidebar-text/60 mb-1 block">
          Project
        </label>
        <div className="flex items-center gap-2">
          <FolderKanban size={14} className="text-sidebar-text/60 shrink-0" />
          <select
            value={selectedProject?.id || ""}
            onChange={(e) => selectProject(Number(e.target.value))}
            className="w-full bg-white/8 border border-white/10 rounded-md px-2 py-1.5 text-sm text-white appearance-none cursor-pointer focus:outline-none focus:ring-1 focus:ring-primary"
          >
            {projects.length === 0 && <option value="">No projects</option>}
            {projects.map((p) => (
              <option key={p.id} value={p.id} className="bg-sidebar text-white">
                {p.name}
              </option>
            ))}
          </select>
        </div>
      </div>
    </aside>
  );
}
