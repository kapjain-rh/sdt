"use client";

import { useState, useEffect, useCallback } from "react";
import {
  FileText,
  Plus,
  Search,
  Play,
  Database,
} from "lucide-react";
import { api } from "@/lib/api";
import { useProjectContext } from "@/components/ProjectContext";
import { Badge } from "@/components/Badge";
import { EmptyState } from "@/components/EmptyState";
import { LiveRunModal } from "@/components/LiveRunModal";
import { CachePanel } from "@/components/CachePanel";
import {
  IDEBody, IDEToolbar, IDEStatusBar,
  IDETitleLine, IDEBlankLine, IDESeparator,
  IDEMetaLine, IDEMetaSelect, IDESectionEditor,
} from "@/components/IDEEditor";
import type { TestCase, CaseCache } from "@/lib/types";
import type { SectionDef } from "@/components/IDEEditor";

// ── Constants & helpers ──

const CASE_SECTIONS: SectionDef[] = [
  { key: "setup", label: "Setup", mdHeader: "## Setup", color: "text-amber-400", dotColor: "bg-amber-400", hint: "Prepare the environment" },
  { key: "steps", label: "Steps", mdHeader: "## Steps", color: "text-blue-400", dotColor: "bg-blue-400", hint: "Main test actions" },
  { key: "verify", label: "Verify", mdHeader: "## Verify", color: "text-emerald-400", dotColor: "bg-emerald-400", hint: "Expected outcomes" },
  { key: "cleanup", label: "Cleanup", mdHeader: "## Cleanup", color: "text-slate-400", dotColor: "bg-slate-400", hint: "Teardown and restore" },
];

interface CaseFormData {
  title: string;
  priority: string;
  status: string;
  author: string;
  labels: string;
  group: string;
  fixtures: string;
  timeout: string;
  [key: string]: string | string[];
  setup: string[];
  steps: string[];
  verify: string[];
  cleanup: string[];
}

const emptyForm: CaseFormData = {
  title: "", priority: "Medium", status: "draft", author: "", labels: "",
  group: "", fixtures: "", timeout: "",
  setup: [], steps: [], verify: [], cleanup: [],
};

function caseToForm(c: TestCase): CaseFormData {
  return {
    title: c.title, priority: c.priority, status: c.status,
    author: c.author || "", labels: c.labels || "",
    group: c.group || "", fixtures: c.fixtures?.join(", ") || "",
    timeout: c.timeout || "",
    setup: c.setup || [], steps: c.steps || [],
    verify: c.verify || [], cleanup: c.cleanup || [],
  };
}

function stepCount(c: TestCase) {
  return (c.setup?.length || 0) + (c.steps?.length || 0) + (c.verify?.length || 0) + (c.cleanup?.length || 0);
}

const priorityColors: Record<string, string> = {
  Critical: "text-red-400", High: "text-orange-400", Medium: "text-yellow-400", Low: "text-slate-400",
};

function CaseCachePanel({ caseId }: { caseId: number }) {
  const [cache, setCache] = useState<CaseCache | null>(null);
  const [loading, setLoading] = useState(true);

  const loadCache = useCallback(() => {
    setLoading(true);
    api.cases.cache(caseId)
      .then(setCache)
      .catch(() => setCache(null))
      .finally(() => setLoading(false));
  }, [caseId]);

  useEffect(() => { loadCache(); }, [loadCache]);

  const handleClear = async () => {
    await api.cases.deleteCache(caseId);
    loadCache();
  };

  return <CachePanel cache={cache} loading={loading} onClear={handleClear} />;
}

// ── Main page ──

type EditorTab = "spec" | "cache";

export default function CasesPage() {
  const { selectedProject } = useProjectContext();
  const [cases, setCases] = useState<TestCase[]>([]);
  const [search, setSearch] = useState("");
  const [loading, setLoading] = useState(true);
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [isNew, setIsNew] = useState(false);
  const [formData, setFormData] = useState<CaseFormData>(emptyForm);
  const [saving, setSaving] = useState(false);
  const [dirty, setDirty] = useState(false);
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({});
  const [runCase, setRunCase] = useState<TestCase | null>(null);
  const [editorTab, setEditorTab] = useState<EditorTab>("spec");

  const loadCases = useCallback(async () => {
    if (!selectedProject) return;
    setLoading(true);
    try { setCases(await api.cases.list(selectedProject.id, search)); }
    catch { setCases([]); }
    finally { setLoading(false); }
  }, [selectedProject, search]);

  useEffect(() => { loadCases(); }, [loadCases]);

  const selectCase = (c: TestCase) => {
    setSelectedId(c.id);
    setFormData(caseToForm(c));
    setIsNew(false);
    setDirty(false);
    setCollapsed({});
    setEditorTab("spec");
  };

  const startNew = () => {
    setSelectedId(null);
    setFormData({ ...emptyForm });
    setIsNew(true);
    setDirty(true);
    setCollapsed({});
    setEditorTab("spec");
  };

  const update = <K extends keyof CaseFormData>(k: K, v: CaseFormData[K]) => {
    setFormData((p) => ({ ...p, [k]: v }));
    setDirty(true);
  };

  const handleSave = async () => {
    if (!selectedProject || !formData.title.trim()) return;
    setSaving(true);
    try {
      const payload = {
        ...formData,
        priority: formData.priority as TestCase["priority"],
        status: formData.status as TestCase["status"],
        fixtures: formData.fixtures ? (formData.fixtures as string).split(",").map((s: string) => s.trim()).filter(Boolean) : [],
      };
      if (isNew) {
        const created = await api.cases.create(selectedProject.id, payload);
        setSelectedId(created.id);
        setIsNew(false);
      } else if (selectedId) {
        await api.cases.update(selectedId, payload);
      }
      setDirty(false);
      await loadCases();
    } finally { setSaving(false); }
  };

  const handleDelete = async () => {
    if (!selectedId || !confirm("Delete this test case?")) return;
    await api.cases.delete(selectedId);
    setSelectedId(null);
    setIsNew(false);
    setFormData(emptyForm);
    setDirty(false);
    await loadCases();
  };

  const handleReset = () => {
    if (isNew) { setFormData({ ...emptyForm }); }
    else {
      const c = cases.find((x) => x.id === selectedId);
      if (c) setFormData(caseToForm(c));
    }
    setDirty(false);
  };

  const runSelected = () => {
    if (selectedId) {
      const c = cases.find((x) => x.id === selectedId);
      if (c) setRunCase(c);
    }
  };

  const showEditor = isNew || selectedId !== null;
  const selectedCase = cases.find((c) => c.id === selectedId);
  const caseIdStr = selectedCase?.case_id || (selectedId ? `TC-${selectedId}` : "new");
  const fileName = isNew ? "new_spec.md" : `${(formData.title || "untitled").replace(/\s+/g, "_").toLowerCase().substring(0, 30)}.md`;
  const approved = formData.status === "approved";

  const totalSteps = CASE_SECTIONS.reduce((s, sec) => s + (formData[sec.key] as string[]).length, 0);

  // Line numbering
  let line = 1;
  const metaStart = line;
  line += 10;
  const secStarts: Record<string, number> = {};
  for (const s of CASE_SECTIONS) {
    secStarts[s.key] = line;
    line += 1 + (collapsed[s.key] ? 1 : Math.max((formData[s.key] as string[]).length, 1)) + 1;
  }

  if (!selectedProject) return <div className="p-8"><EmptyState icon={<FileText size={48} />} title="No Project Selected" description="Select a project from the sidebar." /></div>;

  return (
    <>
      <div className="flex h-[calc(100vh-64px)]">
        {/* File explorer sidebar */}
        <div className="w-64 bg-[#181825] border-r border-[#313244] flex flex-col shrink-0">
          <div className="flex items-center justify-between px-3 py-2 border-b border-[#313244]">
            <span className="text-[11px] text-slate-400 uppercase font-semibold tracking-wider">Test Cases</span>
            <button onClick={startNew} className="p-1 text-slate-400 hover:text-blue-400 transition-colors rounded hover:bg-[#313244]" title="New Case">
              <Plus size={14} />
            </button>
          </div>

          {/* Search */}
          <div className="px-2 py-2 border-b border-[#313244]">
            <div className="relative">
              <Search size={12} className="absolute left-2 top-1/2 -translate-y-1/2 text-slate-600" />
              <input
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                className="w-full pl-7 pr-2 py-1 bg-[#252535] border border-[#313244] rounded text-xs text-slate-300 outline-none focus:border-blue-500 placeholder:text-slate-600 font-mono"
                placeholder="Filter..."
              />
            </div>
          </div>

          <div className="flex-1 overflow-auto py-1">
            {loading ? (
              <div className="px-3 py-4 text-xs text-slate-600">Loading...</div>
            ) : cases.length === 0 && !isNew ? (
              <div className="px-3 py-4 text-xs text-slate-600 italic">No test cases</div>
            ) : (
              <>
                {isNew && (
                  <button className="w-full flex items-center gap-2 px-3 py-1.5 text-left bg-[#313244] text-blue-400">
                    <FileText size={12} className="shrink-0 text-blue-400" />
                    <span className="truncate text-xs italic">new case</span>
                  </button>
                )}
                {cases.map((c) => {
                  const active = !isNew && selectedId === c.id;
                  return (
                    <button
                      key={c.id}
                      onClick={() => selectCase(c)}
                      className={`w-full flex items-center gap-2 px-3 py-1.5 text-left transition-colors group ${active ? "bg-[#313244] text-slate-200" : "text-slate-400 hover:bg-[#252535] hover:text-slate-300"}`}
                    >
                      <FileText size={12} className={`shrink-0 ${active ? "text-blue-400" : "text-slate-600"}`} />
                      <div className="flex-1 min-w-0">
                        <div className="text-xs truncate">{c.title}</div>
                        <div className="flex items-center gap-2 mt-0.5">
                          <span className="text-[10px] text-slate-600 font-mono">{c.case_id || c.id}</span>
                          <span className={`text-[10px] ${priorityColors[c.priority] || "text-slate-500"}`}>{c.priority[0]}</span>
                          <span className="text-[10px] text-slate-600">{stepCount(c)}s</span>
                        </div>
                      </div>
                    </button>
                  );
                })}
              </>
            )}
          </div>
          <div className="px-3 py-2 border-t border-[#313244] text-[10px] text-slate-600">
            {cases.length} case{cases.length !== 1 ? "s" : ""}
          </div>
        </div>

        {/* Editor area */}
        <div className="flex-1 flex flex-col min-w-0">
          {!showEditor ? (
            <div className="flex-1 flex items-center justify-center bg-[#1e1e2e]">
              <EmptyState
                icon={<FileText size={48} className="text-slate-600" />}
                title="Select a Test Case"
                description="Choose a case from the sidebar or create a new one."
                action={<button onClick={startNew} className="flex items-center gap-1.5 px-4 py-2 bg-primary text-white text-sm rounded-md hover:bg-primary-hover transition-colors"><Plus size={16} /> New Case</button>}
              />
            </div>
          ) : (
            <>
              <IDEToolbar
                fileName={fileName}
                dirty={dirty}
                saving={saving}
                canSave={!!formData.title.trim()}
                onSave={handleSave}
                onReset={handleReset}
                onDelete={!isNew && !approved ? handleDelete : undefined}
                stats={`${caseIdStr} · ${totalSteps} steps`}
              />

              {/* Run bar + tab switcher */}
              {!isNew && selectedId && (
                <div className="flex items-center justify-between px-4 py-1.5 bg-[#232336] border-b border-[#313244]">
                  <div className="flex items-center gap-2">
                    {/* Tab buttons */}
                    <button
                      onClick={() => setEditorTab("spec")}
                      className={`flex items-center gap-1 px-2 py-0.5 text-xs rounded transition-colors ${
                        editorTab === "spec"
                          ? "bg-[#313244] text-slate-200"
                          : "text-slate-500 hover:text-slate-300"
                      }`}
                    >
                      <FileText size={12} /> Spec
                    </button>
                    <button
                      onClick={() => setEditorTab("cache")}
                      className={`flex items-center gap-1 px-2 py-0.5 text-xs rounded transition-colors ${
                        editorTab === "cache"
                          ? "bg-[#313244] text-slate-200"
                          : "text-slate-500 hover:text-slate-300"
                      }`}
                    >
                      <Database size={12} /> Cache
                    </button>
                    <span className="w-px h-4 bg-[#313244] mx-1" />
                    <Badge value={formData.priority as string} type="priority" />
                    <Badge value={formData.status as string} />
                    {formData.group && <span className="text-[10px] text-violet-400 bg-violet-400/10 px-1.5 py-0.5 rounded font-mono">{formData.group}</span>}
                  </div>
                  <button
                    onClick={runSelected}
                    className="flex items-center gap-1.5 px-3 py-1 text-xs bg-emerald-600/80 text-emerald-100 rounded hover:bg-emerald-500 transition-colors"
                  >
                    <Play size={12} /> Run Test
                  </button>
                </div>
              )}

              {/* Spec editor or Cache view */}
              {editorTab === "spec" ? (
                <>
                  <IDEBody>
                    <IDETitleLine lineNum={metaStart} value={formData.title as string} onChange={(v) => update("title", v)} placeholder="Test Case Title" />
                    <IDEBlankLine lineNum={metaStart + 1} />
                    <IDEMetaSelect lineNum={metaStart + 2} label="Priority" value={formData.priority as string} onChange={(v) => update("priority", v)}
                      options={[{ value: "Critical", label: "Critical" }, { value: "High", label: "High" }, { value: "Medium", label: "Medium" }, { value: "Low", label: "Low" }]} />
                    <IDEMetaSelect lineNum={metaStart + 3} label="Status" value={formData.status as string} onChange={(v) => update("status", v)}
                      options={[{ value: "draft", label: "draft" }, { value: "active", label: "active" }, { value: "deprecated", label: "deprecated" }]} />
                    <IDEMetaLine lineNum={metaStart + 4} label="Author" value={formData.author as string} onChange={(v) => update("author", v)} placeholder="name" color="text-green-300" />
                    <IDEMetaLine lineNum={metaStart + 5} label="Group" value={formData.group as string} onChange={(v) => update("group", v)} placeholder="with-loki" color="text-violet-300" />
                    <IDEMetaLine lineNum={metaStart + 6} label="Fixtures" value={formData.fixtures as string} onChange={(v) => update("fixtures", v)} placeholder="fixture1, fixture2" color="text-pink-300" />
                    <IDEMetaLine lineNum={metaStart + 7} label="Timeout" value={formData.timeout as string} onChange={(v) => update("timeout", v)} placeholder="15m" />
                    <IDEMetaLine lineNum={metaStart + 8} label="Labels" value={formData.labels as string} onChange={(v) => update("labels", v)} placeholder="Serial, Disruptive" color="text-yellow-300" />
                    <IDESeparator lineNum={metaStart + 9} />
                    {CASE_SECTIONS.map((sec) => (
                      <div key={sec.key}>
                        <IDEBlankLine />
                        <IDESectionEditor section={sec} steps={formData[sec.key] as string[]} onChange={(v) => update(sec.key, v)} lineStart={secStarts[sec.key]}
                          collapsed={!!collapsed[sec.key]} onToggle={() => setCollapsed((p) => ({ ...p, [sec.key]: !p[sec.key] }))} />
                      </div>
                    ))}
                  </IDEBody>
                  <IDEStatusBar lineCount={line - 1} fileName={fileName} dirty={dirty} />
                </>
              ) : (
                selectedId && <CaseCachePanel caseId={selectedId} />
              )}
            </>
          )}
        </div>
      </div>

      <LiveRunModal testCase={runCase} open={runCase !== null} onClose={() => setRunCase(null)} />
    </>
  );
}
