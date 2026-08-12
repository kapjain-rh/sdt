"use client";

import { useState, useEffect, useCallback } from "react";
import {
  Package,
  Plus,
  FileText,
} from "lucide-react";
import { api } from "@/lib/api";
import { useProjectContext } from "@/components/ProjectContext";
import { Badge } from "@/components/Badge";
import { EmptyState } from "@/components/EmptyState";
import { LiveRunModal } from "@/components/LiveRunModal";
import {
  IDEBody, IDEToolbar, IDEStatusBar,
  IDETitleLine, IDEBlankLine, IDESeparator,
  IDEMetaLine, IDEMetaSelect, IDEMetaTextarea,
  IDEListEditor, IDEKeyValueEditor,
} from "@/components/IDEEditor";
import type { Fixture, TestCase } from "@/lib/types";

interface FixtureFormData {
  name: string;
  description: string;
  status: string;
  templates: string[];
  parameters: Record<string, string>;
  lifecycle: { create: string; ready: string; cleanup: string };
}

const emptyForm: FixtureFormData = {
  name: "", description: "", status: "draft",
  templates: [], parameters: {},
  lifecycle: { create: "", ready: "", cleanup: "" },
};

function fixtureToForm(f: Fixture): FixtureFormData {
  return {
    name: f.name, description: f.description || "", status: f.status || "draft",
    templates: f.templates || [], parameters: f.parameters ? { ...f.parameters } : {},
    lifecycle: {
      create: f.lifecycle?.create || "",
      ready: f.lifecycle?.ready || "",
      cleanup: f.lifecycle?.cleanup || "",
    },
  };
}

function toTestCase(fixtureName: string, phase: string, instruction: string): TestCase {
  return {
    id: 0, project_id: 1, title: `Fixture "${fixtureName}" — ${phase}`,
    description: "", preconditions: "",
    setup: null, steps: [instruction], verify: null, cleanup: null,
    priority: "Medium", status: "active", author: "", labels: "",
    created_at: new Date().toISOString(), updated_at: new Date().toISOString(),
  };
}

export default function FixturesPage() {
  const { selectedProject } = useProjectContext();
  const [fixtures, setFixtures] = useState<Fixture[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedName, setSelectedName] = useState<string | null>(null);
  const [isNew, setIsNew] = useState(false);
  const [formData, setFormData] = useState<FixtureFormData>(emptyForm);
  const [originalName, setOriginalName] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [dirty, setDirty] = useState(false);
  const [runCase, setRunCase] = useState<TestCase | null>(null);

  const loadFixtures = useCallback(async () => {
    if (!selectedProject) return;
    setLoading(true);
    try { setFixtures(await api.fixtures.list(selectedProject.id)); }
    catch { setFixtures([]); }
    finally { setLoading(false); }
  }, [selectedProject]);

  useEffect(() => { loadFixtures(); }, [loadFixtures]);

  const selectFixture = (f: Fixture) => {
    setSelectedName(f.name);
    setOriginalName(f.name);
    setFormData(fixtureToForm(f));
    setIsNew(false);
    setDirty(false);
  };

  const startNew = () => {
    setSelectedName(null);
    setOriginalName(null);
    setFormData({ ...emptyForm, parameters: {}, lifecycle: { create: "", ready: "", cleanup: "" } });
    setIsNew(true);
    setDirty(true);
  };

  const update = <K extends keyof FixtureFormData>(k: K, v: FixtureFormData[K]) => {
    setFormData((p) => ({ ...p, [k]: v }));
    setDirty(true);
  };

  const updateLifecycle = (phase: "create" | "ready" | "cleanup", v: string) => {
    setFormData((p) => ({ ...p, lifecycle: { ...p.lifecycle, [phase]: v } }));
    setDirty(true);
  };

  const handleSave = async () => {
    if (!selectedProject || !formData.name.trim()) return;
    setSaving(true);
    try {
      if (isNew) {
        await api.fixtures.create(selectedProject.id, formData);
      } else if (originalName) {
        await api.fixtures.update(selectedProject.id, originalName, formData);
      }
      setDirty(false);
      setIsNew(false);
      setOriginalName(formData.name);
      setSelectedName(formData.name);
      await loadFixtures();
    } finally { setSaving(false); }
  };

  const handleDelete = async () => {
    if (!selectedProject || !originalName) return;
    if (!confirm(`Delete fixture "${originalName}"?`)) return;
    await api.fixtures.delete(selectedProject.id, originalName);
    setSelectedName(null);
    setFormData(emptyForm);
    setIsNew(false);
    setDirty(false);
    await loadFixtures();
  };

  const handleReset = () => {
    if (isNew) { setFormData({ ...emptyForm, parameters: {}, lifecycle: { create: "", ready: "", cleanup: "" } }); }
    else {
      const f = fixtures.find((x) => x.name === originalName);
      if (f) setFormData(fixtureToForm(f));
    }
    setDirty(false);
  };

  const showEditor = isNew || selectedName !== null;
  const fileName = isNew ? "new_fixture.yaml" : `${(formData.name || "fixture").replace(/\s+/g, "-").toLowerCase()}.yaml`;
  const approved = formData.status === "approved";

  // Line numbering
  let line = 1;
  const metaStart = line;
  line += 5; // name, blank, description, status, separator

  const templatesLine = line;
  line += 1 + formData.templates.length + 1;

  const paramsLine = line;
  line += 1 + Object.keys(formData.parameters).length + 1;

  const lifecycleLine = line;
  const createLines = formData.lifecycle.create ? formData.lifecycle.create.split("\n").length : 1;
  const readyLines = formData.lifecycle.ready ? formData.lifecycle.ready.split("\n").length : 1;
  const cleanupLines = formData.lifecycle.cleanup ? formData.lifecycle.cleanup.split("\n").length : 1;
  const createLine = lifecycleLine + 1;
  const readyLine = createLine + 1 + createLines + 1;
  const cleanupLine = readyLine + 1 + readyLines + 1;
  line = cleanupLine + 1 + cleanupLines;

  if (!selectedProject) return <div className="p-8"><EmptyState icon={<Package size={48} />} title="No Project Selected" description="Select a project from the sidebar." /></div>;

  return (
    <>
      <div className="flex h-[calc(100vh-64px)]">
        {/* File explorer sidebar */}
        <div className="w-60 bg-[#181825] border-r border-[#313244] flex flex-col shrink-0">
          <div className="flex items-center justify-between px-3 py-2 border-b border-[#313244]">
            <span className="text-[11px] text-slate-400 uppercase font-semibold tracking-wider">Fixtures</span>
            <button onClick={startNew} className="p-1 text-slate-400 hover:text-blue-400 transition-colors rounded hover:bg-[#313244]" title="New Fixture">
              <Plus size={14} />
            </button>
          </div>
          <div className="flex-1 overflow-auto py-1">
            {loading ? (
              <div className="px-3 py-4 text-xs text-slate-600">Loading...</div>
            ) : fixtures.length === 0 && !isNew ? (
              <div className="px-3 py-4 text-xs text-slate-600 italic">No fixtures yet</div>
            ) : (
              <>
                {isNew && (
                  <button className="w-full flex items-center gap-2 px-3 py-1.5 text-sm text-left bg-[#313244] text-blue-400 font-mono">
                    <FileText size={13} className="shrink-0 text-blue-400" />
                    <span className="truncate italic text-xs">new fixture</span>
                  </button>
                )}
                {fixtures.map((f) => {
                  const active = !isNew && selectedName === f.name;
                  const paramCount = Object.keys(f.parameters || {}).length;
                  const tplCount = f.templates?.length ?? 0;
                  return (
                    <button
                      key={f.name}
                      onClick={() => selectFixture(f)}
                      className={`w-full flex items-center gap-2 px-3 py-1.5 text-left transition-colors ${active ? "bg-[#313244] text-slate-200" : "text-slate-400 hover:bg-[#252535] hover:text-slate-300"}`}
                    >
                      <Package size={13} className={`shrink-0 ${active ? "text-violet-400" : "text-slate-600"}`} />
                      <div className="flex-1 min-w-0">
                        <div className="text-xs font-mono truncate">{f.name}</div>
                        <div className="flex items-center gap-2 mt-0.5">
                          <span className={`text-[10px] ${f.status === "active" ? "text-emerald-500" : "text-slate-600"}`}>{f.status}</span>
                          {tplCount > 0 && <span className="text-[10px] text-slate-600">{tplCount}t</span>}
                          {paramCount > 0 && <span className="text-[10px] text-slate-600">{paramCount}p</span>}
                        </div>
                      </div>
                    </button>
                  );
                })}
              </>
            )}
          </div>
          <div className="px-3 py-2 border-t border-[#313244] text-[10px] text-slate-600">
            {fixtures.length} fixture{fixtures.length !== 1 ? "s" : ""}
          </div>
        </div>

        {/* Editor area */}
        <div className="flex-1 flex flex-col min-w-0">
          {!showEditor ? (
            <div className="flex-1 flex items-center justify-center bg-[#1e1e2e]">
              <EmptyState
                icon={<Package size={48} className="text-slate-600" />}
                title="Select a Fixture"
                description="Choose a fixture from the sidebar or create a new one."
                action={<button onClick={startNew} className="flex items-center gap-1.5 px-4 py-2 bg-primary text-white text-sm rounded-md hover:bg-primary-hover transition-colors"><Plus size={16} /> New Fixture</button>}
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
                onDelete={!isNew && !approved ? handleDelete : undefined}
                stats={`${formData.templates.length} templates · ${Object.keys(formData.parameters).length} params`}
              />

              {/* Status bar under toolbar */}
              {!isNew && selectedName && (
                <div className="flex items-center gap-2 px-4 py-1.5 bg-[#232336] border-b border-[#313244]">
                  <Badge value={formData.status} />
                  {formData.description && <span className="text-xs text-slate-500 truncate">{formData.description}</span>}
                </div>
              )}

              <IDEBody>
                {/* # name */}
                <IDETitleLine lineNum={metaStart} value={formData.name} onChange={(v) => update("name", v)} placeholder="fixture-name (e.g. flowcollector-default)" />
                <IDEBlankLine lineNum={metaStart + 1} />

                {/* Metadata */}
                <IDEMetaLine lineNum={metaStart + 2} label="Description" value={formData.description} onChange={(v) => update("description", v)} placeholder="What does this fixture provide?" color="text-green-300" />
                <IDEMetaSelect lineNum={metaStart + 3} label="Status" value={formData.status} onChange={(v) => update("status", v)}
                  options={[{ value: "draft", label: "draft" }, { value: "active", label: "active" }]} />
                <IDESeparator lineNum={metaStart + 4} />

                {/* Templates */}
                <IDEBlankLine />
                <IDEListEditor
                  lineStart={templatesLine}
                  label="templates"
                  items={formData.templates}
                  onChange={(v) => update("templates", v)}
                  placeholder="templates/path/to/resource.yaml"
                  color="text-pink-300"
                />

                {/* Parameters */}
                <IDEBlankLine />
                <IDEKeyValueEditor
                  lineStart={paramsLine}
                  label="parameters"
                  entries={formData.parameters}
                  onChange={(v) => update("parameters", v)}
                />

                {/* Lifecycle */}
                <IDEBlankLine />
                <div className="flex items-center h-7">
                  <span className="w-12 text-right pr-3 text-xs text-slate-600 font-mono shrink-0 tabular-nums">{lifecycleLine}</span>
                  <div className="w-4 shrink-0 ml-1" />
                  <span className="text-purple-400 font-bold font-mono text-sm">## Lifecycle</span>
                </div>

                <IDEBlankLine />
                <IDEMetaTextarea
                  lineStart={createLine}
                  label="create"
                  value={formData.lifecycle.create}
                  onChange={(v) => updateLifecycle("create", v)}
                  placeholder="How to create this fixture..."
                  color="text-emerald-300"
                  onRun={formData.lifecycle.create.trim() ? () => setRunCase(toTestCase(formData.name, "Create", formData.lifecycle.create)) : undefined}
                />

                <IDEBlankLine />
                <IDEMetaTextarea
                  lineStart={readyLine}
                  label="ready"
                  value={formData.lifecycle.ready}
                  onChange={(v) => updateLifecycle("ready", v)}
                  placeholder="How to verify the fixture is ready..."
                  color="text-blue-300"
                  onRun={formData.lifecycle.ready.trim() ? () => setRunCase(toTestCase(formData.name, "Ready", formData.lifecycle.ready)) : undefined}
                />

                <IDEBlankLine />
                <IDEMetaTextarea
                  lineStart={cleanupLine}
                  label="cleanup"
                  value={formData.lifecycle.cleanup}
                  onChange={(v) => updateLifecycle("cleanup", v)}
                  placeholder="How to clean up this fixture..."
                  color="text-rose-300"
                  onRun={formData.lifecycle.cleanup.trim() ? () => setRunCase(toTestCase(formData.name, "Cleanup", formData.lifecycle.cleanup)) : undefined}
                />
              </IDEBody>
              <IDEStatusBar lineCount={line} fileName={fileName} dirty={dirty} lang="YAML" />
            </>
          )}
        </div>
      </div>

      <LiveRunModal testCase={runCase} open={runCase !== null} onClose={() => setRunCase(null)} />
    </>
  );
}
