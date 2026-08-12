"use client";

import { useState, useEffect, useCallback } from "react";
import {
  FolderOpen,
  Plus,
  FileText,
  Database,
} from "lucide-react";
import { api } from "@/lib/api";
import { useProjectContext } from "@/components/ProjectContext";
import { EmptyState } from "@/components/EmptyState";
import { LiveRunModal } from "@/components/LiveRunModal";
import { CachePanel } from "@/components/CachePanel";
import {
  IDEShell, IDEBody, IDEToolbar, IDEStatusBar,
  IDETitleLine, IDEBlankLine, IDESeparator, IDEMetaLine,
  IDESectionEditor,
} from "@/components/IDEEditor";
import type { GroupInfo, TestCase, CaseCache } from "@/lib/types";
import type { SectionDef } from "@/components/IDEEditor";

const GROUP_SECTIONS: SectionDef[] = [
  { key: "pre_test", label: "Pre-Test", mdHeader: "## Pre-Test", color: "text-blue-400", dotColor: "bg-blue-400", hint: "Steps run before each test in this group" },
  { key: "pre_test_validation", label: "Pre-Test Validation", mdHeader: "## Pre-Test Validation", color: "text-violet-400", dotColor: "bg-violet-400", hint: "Conditions verified after pre-test" },
  { key: "post_test", label: "Post-Test", mdHeader: "## Post-Test", color: "text-rose-400", dotColor: "bg-rose-400", hint: "Steps run after each test in this group" },
];

interface GroupFormData {
  name: string;
  timeout: string;
  [key: string]: string | string[];
  pre_test: string[];
  pre_test_validation: string[];
  post_test: string[];
}

const emptyForm: GroupFormData = {
  name: "", timeout: "",
  pre_test: [], pre_test_validation: [], post_test: [],
};

function toTestCase(groupName: string, label: string, steps: string[]): TestCase {
  return {
    id: 0, project_id: 1, title: `Group "${groupName}" — ${label}`,
    description: "", preconditions: "",
    setup: null, steps, verify: null, cleanup: null,
    priority: "Medium", status: "active", author: "", labels: "",
    created_at: new Date().toISOString(), updated_at: new Date().toISOString(),
  };
}

function groupToForm(g: GroupInfo): GroupFormData {
  return {
    name: g.name, timeout: g.timeout || "",
    pre_test: g.pre_test || [], pre_test_validation: g.pre_test_validation || [],
    post_test: g.post_test || [],
  };
}

export default function GroupsPage() {
  const { selectedProject } = useProjectContext();
  const [groups, setGroups] = useState<GroupInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedName, setSelectedName] = useState<string | null>(null);
  const [isNew, setIsNew] = useState(false);
  const [formData, setFormData] = useState<GroupFormData>(emptyForm);
  const [originalName, setOriginalName] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [dirty, setDirty] = useState(false);
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({});
  const [runCase, setRunCase] = useState<TestCase | null>(null);
  const [editorTab, setEditorTab] = useState<"spec" | "cache">("spec");
  const [cache, setCache] = useState<CaseCache | null>(null);
  const [cacheLoading, setCacheLoading] = useState(false);

  const loadGroups = useCallback(async () => {
    if (!selectedProject) return;
    setLoading(true);
    try { setGroups(await api.groups.list(selectedProject.id)); }
    catch { setGroups([]); }
    finally { setLoading(false); }
  }, [selectedProject]);

  useEffect(() => { loadGroups(); }, [loadGroups]);

  useEffect(() => {
    if (editorTab !== "cache" || !selectedProject || !selectedName) return;
    setCacheLoading(true);
    api.groups.cache(selectedProject.id, selectedName)
      .then(setCache)
      .catch(() => setCache(null))
      .finally(() => setCacheLoading(false));
  }, [editorTab, selectedProject, selectedName]);

  const selectGroup = (g: GroupInfo) => {
    setSelectedName(g.name);
    setOriginalName(g.name);
    setFormData(groupToForm(g));
    setIsNew(false);
    setDirty(false);
    setCollapsed({});
    setEditorTab("spec");
  };

  const startNew = () => {
    setSelectedName(null);
    setOriginalName(null);
    setFormData({ ...emptyForm });
    setIsNew(true);
    setDirty(true);
    setCollapsed({});
    setEditorTab("spec");
  };

  const update = <K extends keyof GroupFormData>(k: K, v: GroupFormData[K]) => {
    setFormData((p) => ({ ...p, [k]: v }));
    setDirty(true);
  };

  const handleSave = async () => {
    if (!selectedProject || !formData.name.trim()) return;
    setSaving(true);
    try {
      if (isNew) {
        await api.groups.create(selectedProject.id, formData);
      } else if (originalName) {
        await api.groups.update(selectedProject.id, originalName, formData);
      }
      setDirty(false);
      setIsNew(false);
      setOriginalName(formData.name);
      setSelectedName(formData.name);
      await loadGroups();
    } finally { setSaving(false); }
  };

  const handleDelete = async () => {
    if (!selectedProject || !originalName) return;
    if (!confirm(`Delete group "${originalName}" and its hook definitions?`)) return;
    await api.groups.delete(selectedProject.id, originalName);
    setSelectedName(null);
    setFormData(emptyForm);
    setIsNew(false);
    setDirty(false);
    await loadGroups();
  };

  const handleReset = () => {
    if (isNew) { setFormData({ ...emptyForm }); }
    else {
      const g = groups.find((x) => x.name === originalName);
      if (g) setFormData(groupToForm(g));
    }
    setDirty(false);
  };

  const showEditor = isNew || selectedName !== null;
  const fileName = isNew ? "_group_new.md" : `_group_${(formData.name || "").replace(/\s+/g, "-").toLowerCase()}.md`;

  const totalSteps = GROUP_SECTIONS.reduce((s, sec) => s + (formData[sec.key] as string[]).length, 0);

  let line = 1;
  const metaStart = line; line += 4;
  const secStarts: Record<string, number> = {};
  for (const s of GROUP_SECTIONS) {
    secStarts[s.key] = line;
    line += 1 + (collapsed[s.key] ? 1 : Math.max((formData[s.key] as string[]).length, 1)) + 1;
  }

  // Specs section line
  const specsLine = line;
  const currentGroup = groups.find((g) => g.name === selectedName);
  const specs = currentGroup?.specs || [];
  line += 1 + Math.max(specs.length, 0) + 1;

  if (!selectedProject) return <div className="p-8"><EmptyState icon={<FolderOpen size={48} />} title="No Project Selected" description="Select a project from the sidebar." /></div>;

  return (
    <>
      <div className="flex h-[calc(100vh-64px)]">
        {/* File explorer sidebar */}
        <div className="w-60 bg-[#181825] border-r border-[#313244] flex flex-col shrink-0">
          <div className="flex items-center justify-between px-3 py-2 border-b border-[#313244]">
            <span className="text-[11px] text-slate-400 uppercase font-semibold tracking-wider">Groups</span>
            <button onClick={startNew} className="p-1 text-slate-400 hover:text-blue-400 transition-colors rounded hover:bg-[#313244]" title="New Group">
              <Plus size={14} />
            </button>
          </div>
          <div className="flex-1 overflow-auto py-1">
            {loading ? (
              <div className="px-3 py-4 text-xs text-slate-600">Loading...</div>
            ) : groups.length === 0 && !isNew ? (
              <div className="px-3 py-4 text-xs text-slate-600 italic">No groups yet</div>
            ) : (
              <>
                {isNew && (
                  <button className="w-full flex items-center gap-2 px-3 py-1.5 text-sm text-left bg-[#313244] text-blue-400 font-mono">
                    <FileText size={13} className="shrink-0 text-blue-400" />
                    <span className="truncate italic">new group</span>
                  </button>
                )}
                {groups.map((g) => {
                  const active = !isNew && selectedName === g.name;
                  const hookCount = (g.pre_test?.length ?? 0) + (g.pre_test_validation?.length ?? 0) + (g.post_test?.length ?? 0);
                  return (
                    <button
                      key={g.name}
                      onClick={() => selectGroup(g)}
                      className={`w-full flex items-center gap-2 px-3 py-1.5 text-sm text-left transition-colors font-mono ${active ? "bg-[#313244] text-slate-200" : "text-slate-400 hover:bg-[#252535] hover:text-slate-300"}`}
                    >
                      <FileText size={13} className={`shrink-0 ${active ? "text-violet-400" : "text-slate-600"}`} />
                      <span className="truncate">{g.name}</span>
                      <span className="ml-auto text-[10px] text-slate-600 tabular-nums">{hookCount}</span>
                    </button>
                  );
                })}
              </>
            )}
          </div>
          <div className="px-3 py-2 border-t border-[#313244] text-[10px] text-slate-600">
            {groups.length} group{groups.length !== 1 ? "s" : ""}
          </div>
        </div>

        {/* Editor area */}
        <div className="flex-1 flex flex-col min-w-0">
          {!showEditor ? (
            <div className="flex-1 flex items-center justify-center bg-[#1e1e2e]">
              <EmptyState
                icon={<FolderOpen size={48} className="text-slate-600" />}
                title="Select a Group"
                description="Choose a group from the sidebar or create a new one."
                action={<button onClick={startNew} className="flex items-center gap-1.5 px-4 py-2 bg-primary text-white text-sm rounded-md hover:bg-primary-hover transition-colors"><Plus size={16} /> New Group</button>}
              />
            </div>
          ) : (
            <>
              <IDEToolbar
                fileName={fileName}
                dirty={dirty}
                saving={saving}
                canSave={!!formData.name.trim()}
                onSave={handleSave}
                onReset={handleReset}
                onDelete={!isNew ? handleDelete : undefined}
                stats={`${totalSteps} hook steps`}
              />

              {!isNew && selectedName && (
                <div className="flex items-center px-4 py-1.5 bg-[#232336] border-b border-[#313244]">
                  <button
                    onClick={() => setEditorTab("spec")}
                    className={`flex items-center gap-1 px-2 py-0.5 text-xs rounded transition-colors ${
                      editorTab === "spec" ? "bg-[#313244] text-slate-200" : "text-slate-500 hover:text-slate-300"
                    }`}
                  >
                    <FileText size={12} /> Spec
                  </button>
                  <button
                    onClick={() => setEditorTab("cache")}
                    className={`flex items-center gap-1 px-2 py-0.5 text-xs rounded transition-colors ${
                      editorTab === "cache" ? "bg-[#313244] text-slate-200" : "text-slate-500 hover:text-slate-300"
                    }`}
                  >
                    <Database size={12} /> Cache
                  </button>
                </div>
              )}

              {editorTab === "spec" ? (
              <>
              <IDEBody>
                <IDETitleLine lineNum={metaStart} value={formData.name} onChange={(v) => update("name", v)} placeholder="Group Name (e.g. with-loki)" />
                <IDEBlankLine lineNum={metaStart + 1} />
                <IDEMetaLine lineNum={metaStart + 2} label="Timeout" value={formData.timeout} onChange={(v) => update("timeout", v)} placeholder="45m" />
                <IDESeparator lineNum={metaStart + 3} />

                {GROUP_SECTIONS.map((sec) => (
                  <div key={sec.key}>
                    <IDEBlankLine />
                    <IDESectionEditor section={sec} steps={formData[sec.key] as string[]} onChange={(v) => update(sec.key, v)} lineStart={secStarts[sec.key]}
                      collapsed={!!collapsed[sec.key]} onToggle={() => setCollapsed((p) => ({ ...p, [sec.key]: !p[sec.key] }))}
                      onRun={() => { const st = formData[sec.key] as string[]; if (st.length) setRunCase(toTestCase(formData.name, sec.label, st)); }} />
                  </div>
                ))}

                {/* Specs using this group (read-only) */}
                {specs.length > 0 && (
                  <>
                    <IDEBlankLine />
                    <div className="flex items-center h-7 select-none">
                      <span className="w-12 text-right pr-3 text-xs text-slate-600 font-mono shrink-0 tabular-nums">{specsLine}</span>
                      <div className="w-4 shrink-0 ml-1" />
                      <span className="font-bold font-mono text-sm text-slate-500 ml-1">## Specs</span>
                      <span className="text-slate-600 text-xs ml-3 italic">read-only — specs referencing this group</span>
                    </div>
                    {specs.map((s, i) => (
                      <div key={s} className="flex items-center h-[22px]">
                        <span className="w-12 text-right pr-3 text-xs text-slate-600 font-mono shrink-0 tabular-nums">{specsLine + 1 + i}</span>
                        <div className="w-4 shrink-0 ml-1 flex items-center justify-center">
                          <FileText size={10} className="text-slate-600" />
                        </div>
                        <span className="text-sm text-slate-500 font-mono ml-1">{s}</span>
                      </div>
                    ))}
                  </>
                )}
              </IDEBody>
              <IDEStatusBar lineCount={line - 1} fileName={fileName} dirty={dirty} />
              </>
              ) : (
                <CachePanel cache={cache} loading={cacheLoading} emptyMessage="Run tests in this group to generate cached hook plans." />
              )}
            </>
          )}
        </div>
      </div>

      <LiveRunModal testCase={runCase} open={runCase !== null} onClose={() => setRunCase(null)} />
    </>
  );
}
