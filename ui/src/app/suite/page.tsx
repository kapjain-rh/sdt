"use client";

import { useState, useEffect, useCallback } from "react";
import { Layers, FileText, Database } from "lucide-react";
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
import type { SuiteInfo, TestCase, CaseCache } from "@/lib/types";
import type { SectionDef } from "@/components/IDEEditor";

const SECTIONS: SectionDef[] = [
  { key: "pre_suite", label: "Pre-Suite", mdHeader: "## Pre-Suite", color: "text-amber-400", dotColor: "bg-amber-400", hint: "Steps run once before all tests" },
  { key: "pre_suite_validation", label: "Pre-Suite Validation", mdHeader: "## Pre-Suite Validation", color: "text-emerald-400", dotColor: "bg-emerald-400", hint: "Conditions verified after pre-suite" },
  { key: "pre_test", label: "Pre-Test", mdHeader: "## Pre-Test", color: "text-blue-400", dotColor: "bg-blue-400", hint: "Steps run before each test" },
  { key: "pre_test_validation", label: "Pre-Test Validation", mdHeader: "## Pre-Test Validation", color: "text-violet-400", dotColor: "bg-violet-400", hint: "Conditions verified after pre-test" },
  { key: "post_test", label: "Post-Test", mdHeader: "## Post-Test", color: "text-rose-400", dotColor: "bg-rose-400", hint: "Steps run after each test" },
  { key: "post_suite", label: "Post-Suite", mdHeader: "## Post-Suite", color: "text-slate-400", dotColor: "bg-slate-400", hint: "Steps run once after all tests" },
];

interface SuiteFormData {
  name: string;
  timeout: string;
  [key: string]: string | string[];
  pre_suite: string[];
  pre_suite_validation: string[];
  pre_test: string[];
  pre_test_validation: string[];
  post_test: string[];
  post_suite: string[];
}

const emptySuiteForm: SuiteFormData = {
  name: "", timeout: "",
  pre_suite: [], pre_suite_validation: [], pre_test: [],
  pre_test_validation: [], post_test: [], post_suite: [],
};

function toTestCase(name: string, label: string, steps: string[]): TestCase {
  return {
    id: 0, project_id: 1, title: `${name} — ${label}`,
    description: "", preconditions: "",
    setup: null, steps, verify: null, cleanup: null,
    priority: "Medium", status: "active", author: "", labels: "",
    created_at: new Date().toISOString(), updated_at: new Date().toISOString(),
  };
}

export default function SuitePage() {
  const { selectedProject } = useProjectContext();
  const [suite, setSuite] = useState<SuiteInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [editing, setEditing] = useState(false);
  const [saving, setSaving] = useState(false);
  const [dirty, setDirty] = useState(false);
  const [formData, setFormData] = useState<SuiteFormData>(emptySuiteForm);
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({});
  const [runCase, setRunCase] = useState<TestCase | null>(null);
  const [editorTab, setEditorTab] = useState<"spec" | "cache">("spec");
  const [cache, setCache] = useState<CaseCache | null>(null);
  const [cacheLoading, setCacheLoading] = useState(false);

  const loadData = useCallback(async () => {
    if (!selectedProject) return;
    setLoading(true);
    try {
      const d = await api.suite.get(selectedProject.id);
      const valid = d && d.name;
      setSuite(valid ? d : null);
      if (valid) {
        setFormData({
          name: d.name || "", timeout: d.timeout || "",
          pre_suite: d.pre_suite || [], pre_suite_validation: d.pre_suite_validation || [],
          pre_test: d.pre_test || [], pre_test_validation: d.pre_test_validation || [],
          post_test: d.post_test || [], post_suite: d.post_suite || [],
        });
        setEditing(true);
      }
    } catch { setSuite(null); }
    finally { setLoading(false); }
  }, [selectedProject]);

  useEffect(() => { loadData(); }, [loadData]);

  useEffect(() => {
    if (editorTab !== "cache" || !selectedProject) return;
    setCacheLoading(true);
    api.suite.cache(selectedProject.id)
      .then(setCache)
      .catch(() => setCache(null))
      .finally(() => setCacheLoading(false));
  }, [editorTab, selectedProject]);

  const update = <K extends keyof SuiteFormData>(k: K, v: SuiteFormData[K]) => {
    setFormData((p) => ({ ...p, [k]: v }));
    setDirty(true);
  };

  const handleSave = async () => {
    if (!selectedProject || !formData.name.trim()) return;
    setSaving(true);
    try { await api.suite.update(selectedProject.id, formData); setDirty(false); await loadData(); }
    finally { setSaving(false); }
  };

  const handleReset = () => {
    if (suite) {
      setFormData({
        name: suite.name || "", timeout: suite.timeout || "",
        pre_suite: suite.pre_suite || [], pre_suite_validation: suite.pre_suite_validation || [],
        pre_test: suite.pre_test || [], pre_test_validation: suite.pre_test_validation || [],
        post_test: suite.post_test || [], post_suite: suite.post_suite || [],
      });
    } else { setFormData(emptySuiteForm); }
    setDirty(false);
  };

  const totalSteps = SECTIONS.reduce((s, sec) => s + (formData[sec.key] as string[]).length, 0);

  let line = 1;
  const metaStart = line; line += 4;
  const secStarts: Record<string, number> = {};
  for (const s of SECTIONS) {
    secStarts[s.key] = line;
    line += 1 + (collapsed[s.key] ? 1 : Math.max((formData[s.key] as string[]).length, 1)) + 1;
  }

  if (!selectedProject) return <div className="p-8"><EmptyState icon={<Layers size={48} />} title="No Project Selected" description="Select a project from the sidebar." /></div>;
  if (loading) return <div className="text-center py-12 text-muted text-sm">Loading suite...</div>;
  if (!editing) return (
    <div className="p-8">
      <EmptyState icon={<Layers size={48} />} title="No Suite Configuration" description="Create a suite to define hooks that run before and after your tests."
        action={<button onClick={() => { setFormData({ ...emptySuiteForm, name: (selectedProject?.name || "") + " Tests" }); setEditing(true); setDirty(true); }} className="flex items-center gap-1.5 px-4 py-2 bg-primary text-white text-sm rounded-md hover:bg-primary-hover transition-colors"><FileText size={16} /> Create Suite</button>}
      />
    </div>
  );

  return (
    <>
      <IDEShell>
        <IDEToolbar fileName="_suite.md" dirty={dirty} saving={saving} canSave={!!formData.name.trim()} onSave={handleSave} onReset={handleReset}
          stats={`${totalSteps} steps across ${SECTIONS.filter(s => (formData[s.key] as string[]).length > 0).length} sections`} />

        {suite && (
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
              <IDETitleLine lineNum={metaStart} value={formData.name} onChange={(v) => update("name", v)} placeholder="Suite Name" />
              <IDEBlankLine lineNum={metaStart + 1} />
              <IDEMetaLine lineNum={metaStart + 2} label="Timeout" value={formData.timeout} onChange={(v) => update("timeout", v)} placeholder="30m" />
              <IDESeparator lineNum={metaStart + 3} />
              {SECTIONS.map((sec) => (
                <div key={sec.key}>
                  <IDEBlankLine />
                  <IDESectionEditor section={sec} steps={formData[sec.key] as string[]} onChange={(v) => update(sec.key, v)} lineStart={secStarts[sec.key]}
                    collapsed={!!collapsed[sec.key]} onToggle={() => setCollapsed((p) => ({ ...p, [sec.key]: !p[sec.key] }))}
                    onRun={() => { const st = formData[sec.key] as string[]; if (st.length) setRunCase(toTestCase(formData.name || "Suite", sec.label, st)); }} />
                </div>
              ))}
            </IDEBody>
            <IDEStatusBar lineCount={line - 1} fileName={suite?.file_path?.split("/").pop()} dirty={dirty} />
          </>
        ) : (
          <CachePanel cache={cache} loading={cacheLoading} emptyMessage="Run the suite to generate cached hook plans." />
        )}
      </IDEShell>
      <LiveRunModal testCase={runCase} open={runCase !== null} onClose={() => setRunCase(null)} />
    </>
  );
}
