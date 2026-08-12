"use client";

import { useState, useEffect, useCallback } from "react";
import {
  Plus,
  Pencil,
  Trash2,
  ClipboardList,
  FileText,
  Play,
  CheckSquare,
  Square,
  X,
  Variable,
} from "lucide-react";
import { api } from "@/lib/api";
import { useProjectContext } from "@/components/ProjectContext";
import { Badge } from "@/components/Badge";
import { Modal } from "@/components/Modal";
import { EmptyState } from "@/components/EmptyState";
import type { TestPlan, TestCase } from "@/lib/types";
import { formatDate } from "@/lib/utils";
import { useRouter } from "next/navigation";

export default function PlansPage() {
  const { selectedProject } = useProjectContext();
  const router = useRouter();
  const [plans, setPlans] = useState<TestPlan[]>([]);
  const [loading, setLoading] = useState(true);
  const [modalOpen, setModalOpen] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [saving, setSaving] = useState(false);
  const [formData, setFormData] = useState<{ name: string; description: string; milestone: string; status: "active" | "draft" | "completed" }>({ name: "", description: "", milestone: "", status: "active" });

  const [casesModalOpen, setCasesModalOpen] = useState(false);
  const [casesForPlan, setCasesForPlan] = useState<number | null>(null);
  const [allCases, setAllCases] = useState<TestCase[]>([]);
  const [selectedCaseIds, setSelectedCaseIds] = useState<Set<number>>(new Set());
  const [assignedCaseIds, setAssignedCaseIds] = useState<Set<number>>(new Set());

  const [runModalOpen, setRunModalOpen] = useState(false);
  const [runPlanId, setRunPlanId] = useState<number | null>(null);
  const [runForm, setRunForm] = useState<{ name: string; build: string; environment: string; env_vars: Record<string, string> }>({ name: "", build: "", environment: "", env_vars: {} });

  const loadPlans = useCallback(async () => {
    if (!selectedProject) return;
    setLoading(true);
    try {
      const data = await api.plans.list(selectedProject.id);
      setPlans(data);
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  }, [selectedProject]);

  useEffect(() => {
    loadPlans();
  }, [loadPlans]);

  const openNew = () => {
    setEditingId(null);
    setFormData({ name: "", description: "", milestone: "", status: "active" as const });
    setModalOpen(true);
  };

  const openEdit = (p: TestPlan) => {
    setEditingId(p.id);
    setFormData({ name: p.name, description: p.description, milestone: p.milestone, status: p.status });
    setModalOpen(true);
  };

  const handleSave = async () => {
    if (!selectedProject || !formData.name.trim()) return;
    setSaving(true);
    try {
      if (editingId) {
        await api.plans.update(editingId, formData);
      } else {
        await api.plans.create(selectedProject.id, formData);
      }
      setModalOpen(false);
      await loadPlans();
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (id: number) => {
    if (!confirm("Delete this test plan and all associated runs?")) return;
    await api.plans.delete(id);
    await loadPlans();
  };

  const openCasesModal = async (planId: number) => {
    if (!selectedProject) return;
    setCasesForPlan(planId);
    const [allProjectCases, planCases] = await Promise.all([
      api.cases.list(selectedProject.id),
      api.plans.getCases(planId),
    ]);
    setAllCases(allProjectCases);
    const assigned = new Set(planCases.map((c) => c.id));
    setAssignedCaseIds(assigned);
    setSelectedCaseIds(new Set(assigned));
    setCasesModalOpen(true);
  };

  const toggleCase = (id: number) => {
    setSelectedCaseIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const saveCases = async () => {
    if (casesForPlan === null) return;
    await api.plans.setCases(casesForPlan, Array.from(selectedCaseIds));
    setCasesModalOpen(false);
    await loadPlans();
  };

  const openRunModal = (planId: number) => {
    setRunPlanId(planId);
    setRunForm({ name: `Run ${new Date().toLocaleDateString()}`, build: "", environment: "", env_vars: {} });
    setRunModalOpen(true);
  };

  const createRun = async () => {
    if (!runPlanId || !runForm.name.trim()) return;
    setSaving(true);
    try {
      await api.runs.create(runPlanId, runForm);
      setRunModalOpen(false);
      router.push("/runs");
    } finally {
      setSaving(false);
    }
  };

  if (!selectedProject) {
    return (
      <div className="p-8">
        <EmptyState
          icon={<ClipboardList size={48} />}
          title="No Project Selected"
          description="Select or create a project from the sidebar to manage test plans."
        />
      </div>
    );
  }

  return (
    <>
      <div className="border-b border-border bg-white px-8 py-4 flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold text-slate-800">Test Plans</h1>
          <p className="text-sm text-muted mt-0.5">
            {selectedProject.name} &middot; {plans.length} plan{plans.length !== 1 ? "s" : ""}
          </p>
        </div>
        <button
          onClick={openNew}
          className="flex items-center gap-1.5 px-4 py-2 bg-primary text-white text-sm rounded-md hover:bg-primary-hover transition-colors"
        >
          <Plus size={16} /> New Plan
        </button>
      </div>

      <div className="p-8">
        {loading ? (
          <div className="text-center py-12 text-muted text-sm">Loading test plans...</div>
        ) : plans.length === 0 ? (
          <EmptyState
            icon={<ClipboardList size={48} />}
            title="No Test Plans"
            description="Create a test plan to organize your test cases into executable groups."
            action={
              <button
                onClick={openNew}
                className="flex items-center gap-1.5 px-4 py-2 bg-primary text-white text-sm rounded-md hover:bg-primary-hover transition-colors"
              >
                <Plus size={16} /> Create Test Plan
              </button>
            }
          />
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {plans.map((p) => (
              <div key={p.id} className="bg-white border border-border rounded-lg shadow-sm hover:shadow-md transition-shadow">
                <div className="p-5">
                  <div className="flex items-start justify-between mb-3">
                    <div className="flex-1">
                      <h3 className="font-semibold text-slate-800 text-sm">{p.name}</h3>
                      {p.description && (
                        <p className="text-xs text-muted mt-1 line-clamp-2">{p.description}</p>
                      )}
                    </div>
                    <Badge value={p.status} />
                  </div>
                  <div className="flex items-center gap-4 text-xs text-muted mb-4">
                    <span className="flex items-center gap-1">
                      <FileText size={12} /> {p.case_count} cases
                    </span>
                    {p.milestone && <span>Milestone: {p.milestone}</span>}
                    <span>{formatDate(p.updated_at)}</span>
                  </div>
                  <div className="flex gap-2 border-t border-border pt-3">
                    <button
                      onClick={() => openCasesModal(p.id)}
                      className="flex items-center gap-1 px-2.5 py-1.5 text-xs border border-border rounded-md hover:bg-slate-50 transition-colors text-slate-600"
                    >
                      <CheckSquare size={12} /> Manage Cases
                    </button>
                    <button
                      onClick={() => openRunModal(p.id)}
                      className="flex items-center gap-1 px-2.5 py-1.5 text-xs bg-success text-white rounded-md hover:opacity-90 transition-opacity"
                      title={p.case_count === 0 ? "Add cases first" : ""}
                    >
                      <Play size={12} /> New Run
                    </button>
                    <div className="ml-auto flex gap-1">
                      <button
                        onClick={() => openEdit(p)}
                        className="p-1.5 text-muted hover:text-primary transition-colors rounded hover:bg-blue-50"
                      >
                        <Pencil size={13} />
                      </button>
                      <button
                        onClick={() => handleDelete(p.id)}
                        className="p-1.5 text-muted hover:text-danger transition-colors rounded hover:bg-red-50"
                      >
                        <Trash2 size={13} />
                      </button>
                    </div>
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Plan Form Modal */}
      <Modal
        open={modalOpen}
        onClose={() => setModalOpen(false)}
        title={editingId ? "Edit Test Plan" : "New Test Plan"}
        footer={
          <>
            <button
              onClick={() => setModalOpen(false)}
              className="px-4 py-2 text-sm border border-border rounded-md hover:bg-slate-50 transition-colors"
            >
              Cancel
            </button>
            <button
              onClick={handleSave}
              disabled={saving || !formData.name.trim()}
              className="px-4 py-2 text-sm bg-primary text-white rounded-md hover:bg-primary-hover transition-colors disabled:opacity-50"
            >
              {saving ? "Saving..." : editingId ? "Update" : "Create"}
            </button>
          </>
        }
      >
        <div className="space-y-4">
          <div>
            <label className="text-sm font-medium text-slate-700 mb-1 block">Name *</label>
            <input
              autoFocus
              value={formData.name}
              onChange={(e) => setFormData({ ...formData, name: e.target.value })}
              className="w-full border border-border rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
              placeholder="Test plan name"
            />
          </div>
          <div>
            <label className="text-sm font-medium text-slate-700 mb-1 block">Description</label>
            <textarea
              value={formData.description}
              onChange={(e) => setFormData({ ...formData, description: e.target.value })}
              className="w-full border border-border rounded-md px-3 py-2 text-sm min-h-[60px] resize-y focus:outline-none focus:ring-2 focus:ring-primary"
              placeholder="What does this plan cover?"
            />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="text-sm font-medium text-slate-700 mb-1 block">Milestone</label>
              <input
                value={formData.milestone}
                onChange={(e) => setFormData({ ...formData, milestone: e.target.value })}
                className="w-full border border-border rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
                placeholder="e.g. Sprint 12"
              />
            </div>
            <div>
              <label className="text-sm font-medium text-slate-700 mb-1 block">Status</label>
              <select
                value={formData.status}
                onChange={(e) => setFormData({ ...formData, status: e.target.value as "active" | "draft" | "completed" })}
                className="w-full border border-border rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
              >
                <option value="active">Active</option>
                <option value="draft">Draft</option>
                <option value="completed">Completed</option>
              </select>
            </div>
          </div>
        </div>
      </Modal>

      {/* Cases Assignment Modal */}
      <Modal
        open={casesModalOpen}
        onClose={() => setCasesModalOpen(false)}
        title="Manage Plan Cases"
        wide
        footer={
          <>
            <button
              onClick={() => setCasesModalOpen(false)}
              className="px-4 py-2 text-sm border border-border rounded-md hover:bg-slate-50 transition-colors"
            >
              Cancel
            </button>
            <button
              onClick={saveCases}
              className="px-4 py-2 text-sm bg-primary text-white rounded-md hover:bg-primary-hover transition-colors"
            >
              Save ({selectedCaseIds.size} selected)
            </button>
          </>
        }
      >
        <p className="text-sm text-muted mb-4">
          Select test cases to include in this plan. {selectedCaseIds.size} of {allCases.length} selected.
        </p>
        {allCases.length === 0 ? (
          <div className="text-center py-8 text-muted text-sm">
            No test cases in this project. Create test cases first.
          </div>
        ) : (
          <div className="space-y-1 max-h-[400px] overflow-y-auto">
            {allCases.map((c) => (
              <label
                key={c.id}
                className="flex items-center gap-3 px-3 py-2.5 rounded-md hover:bg-slate-50 cursor-pointer transition-colors"
              >
                <button
                  onClick={() => toggleCase(c.id)}
                  className="text-primary"
                >
                  {selectedCaseIds.has(c.id) ? <CheckSquare size={18} /> : <Square size={18} className="text-slate-300" />}
                </button>
                <div className="flex-1 min-w-0">
                  <div className="text-sm font-medium text-slate-800 truncate">{c.title}</div>
                  <div className="text-xs text-muted flex gap-2">
                    <span>TC-{c.id}</span>
                    <Badge value={c.priority} type="priority" />
                    {(c.steps?.length ?? 0) > 0 && <span>{c.steps!.length} steps</span>}
                  </div>
                </div>
                {assignedCaseIds.has(c.id) && !selectedCaseIds.has(c.id) && (
                  <span className="text-xs text-danger">will be removed</span>
                )}
                {!assignedCaseIds.has(c.id) && selectedCaseIds.has(c.id) && (
                  <span className="text-xs text-success">will be added</span>
                )}
              </label>
            ))}
          </div>
        )}
      </Modal>

      {/* Create Run Modal */}
      <Modal
        open={runModalOpen}
        onClose={() => setRunModalOpen(false)}
        title="Create Test Run"
        footer={
          <>
            <button
              onClick={() => setRunModalOpen(false)}
              className="px-4 py-2 text-sm border border-border rounded-md hover:bg-slate-50 transition-colors"
            >
              Cancel
            </button>
            <button
              onClick={createRun}
              disabled={saving || !runForm.name.trim()}
              className="px-4 py-2 text-sm bg-success text-white rounded-md hover:opacity-90 transition-opacity disabled:opacity-50"
            >
              {saving ? "Creating..." : "Create Run"}
            </button>
          </>
        }
      >
        <div className="space-y-4">
          <div>
            <label className="text-sm font-medium text-slate-700 mb-1 block">Run Name *</label>
            <input
              autoFocus
              value={runForm.name}
              onChange={(e) => setRunForm({ ...runForm, name: e.target.value })}
              className="w-full border border-border rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
            />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="text-sm font-medium text-slate-700 mb-1 block">Build</label>
              <input
                value={runForm.build}
                onChange={(e) => setRunForm({ ...runForm, build: e.target.value })}
                className="w-full border border-border rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
                placeholder="e.g. v2.1.0"
              />
            </div>
            <div>
              <label className="text-sm font-medium text-slate-700 mb-1 block">Environment</label>
              <input
                value={runForm.environment}
                onChange={(e) => setRunForm({ ...runForm, environment: e.target.value })}
                className="w-full border border-border rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
                placeholder="e.g. staging"
              />
            </div>
          </div>
          <div>
            <div className="flex items-center justify-between mb-2">
              <label className="text-sm font-medium text-slate-700 flex items-center gap-1.5">
                <Variable size={14} /> Environment Variables
              </label>
              <button
                type="button"
                onClick={() => {
                  const key = `VAR_${Object.keys(runForm.env_vars).length + 1}`;
                  setRunForm({ ...runForm, env_vars: { ...runForm.env_vars, [key]: "" } });
                }}
                className="text-xs text-primary hover:text-primary-hover transition-colors"
              >
                + Add Variable
              </button>
            </div>
            {Object.keys(runForm.env_vars).length === 0 ? (
              <p className="text-xs text-muted italic">No environment variables. These will be passed to the SDT executor process.</p>
            ) : (
              <div className="space-y-2">
                {Object.entries(runForm.env_vars).map(([key, val]) => (
                  <div key={key} className="flex items-center gap-2">
                    <input
                      value={key}
                      onChange={(e) => {
                        const next = { ...runForm.env_vars };
                        const v = next[key];
                        delete next[key];
                        next[e.target.value] = v;
                        setRunForm({ ...runForm, env_vars: next });
                      }}
                      className="w-2/5 border border-border rounded-md px-2 py-1.5 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-primary"
                      placeholder="KEY"
                    />
                    <span className="text-slate-400">=</span>
                    <input
                      value={val}
                      onChange={(e) => setRunForm({ ...runForm, env_vars: { ...runForm.env_vars, [key]: e.target.value } })}
                      className="flex-1 border border-border rounded-md px-2 py-1.5 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-primary"
                      placeholder="value"
                    />
                    <button
                      type="button"
                      onClick={() => {
                        const next = { ...runForm.env_vars };
                        delete next[key];
                        setRunForm({ ...runForm, env_vars: next });
                      }}
                      className="p-1 text-muted hover:text-danger transition-colors"
                    >
                      <Trash2 size={14} />
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      </Modal>
    </>
  );
}
