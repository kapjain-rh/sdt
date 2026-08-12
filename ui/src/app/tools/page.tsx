"use client";

import { useState, useEffect, useCallback } from "react";
import {
  Plus,
  Search,
  Wrench,
  Pencil,
  Trash2,
  ChevronRight,
  ChevronDown,
  ArrowRight,
  CheckCircle,
  Clock,
  FileEdit,
  X,
  Terminal,
  Play,
} from "lucide-react";
import { useProjectContext } from "@/components/ProjectContext";
import { Modal } from "@/components/Modal";
import { Badge } from "@/components/Badge";
import { EmptyState } from "@/components/EmptyState";
import { ToolTestModal } from "@/components/ToolTestModal";
import { api } from "@/lib/api";
import { formatDate } from "@/lib/utils";
import type { Tool, ToolParam } from "@/lib/types";

const CATEGORIES = ["Go", "Python", "Shell"] as const;
type Category = (typeof CATEGORIES)[number];

const categoryMeta: Record<Category, { color: string; bg: string; icon: string }> = {
  Go:     { color: "text-cyan-700",   bg: "bg-cyan-100",   icon: "Go" },
  Python: { color: "text-yellow-700", bg: "bg-yellow-100", icon: "Py" },
  Shell:  { color: "text-slate-700",  bg: "bg-slate-200",  icon: "$" },
};

interface ToolFormData {
  name: string;
  description: string;
  command: string;
  args: string[];
  env: Record<string, string>;
  input_params: Record<string, ToolParam>;
  category: string;
  status: "draft" | "verify" | "approved";
  author: string;
}

const statusFlow: Record<string, { next: string; label: string; color: string }> = {
  draft: { next: "verify", label: "Send to Verify", color: "bg-amber-500 hover:bg-amber-600" },
  verify: { next: "approved", label: "Approve", color: "bg-emerald-500 hover:bg-emerald-600" },
};

function CategoryBadge({ category }: { category: string }) {
  const meta = categoryMeta[category as Category];
  if (!meta) {
    return <span className="text-sm text-muted">{category || "—"}</span>;
  }
  return (
    <span
      className={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-bold uppercase tracking-wide ${meta.bg} ${meta.color}`}
    >
      <span className="font-mono text-[10px]">{meta.icon}</span>
      {category}
    </span>
  );
}

function EnvEditor({
  env,
  onChange,
}: {
  env: Record<string, string>;
  onChange: (env: Record<string, string>) => void;
}) {
  const entries = Object.entries(env);

  const addEntry = () => onChange({ ...env, "": "" });
  const removeEntry = (key: string) => {
    const next = { ...env };
    delete next[key];
    onChange(next);
  };
  const updateKey = (oldKey: string, newKey: string) => {
    const next: Record<string, string> = {};
    for (const [k, v] of Object.entries(env)) {
      next[k === oldKey ? newKey : k] = v;
    }
    onChange(next);
  };
  const updateValue = (key: string, value: string) => {
    onChange({ ...env, [key]: value });
  };

  return (
    <div>
      <label className="text-sm font-medium text-slate-700 mb-2 block">
        Environment Variables
      </label>
      <div className="space-y-2 mb-3">
        {entries.map(([key, val], i) => (
          <div key={i} className="flex gap-2 items-center">
            <input
              value={key}
              onChange={(e) => updateKey(key, e.target.value)}
              className="flex-1 border border-border rounded px-2 py-1.5 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-primary"
              placeholder="KEY"
            />
            <span className="text-muted">=</span>
            <input
              value={val}
              onChange={(e) => updateValue(key, e.target.value)}
              className="flex-1 border border-border rounded px-2 py-1.5 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-primary"
              placeholder="value"
            />
            <button
              onClick={() => removeEntry(key)}
              className="p-1 text-muted hover:text-danger"
            >
              <X size={14} />
            </button>
          </div>
        ))}
      </div>
      <button
        onClick={addEntry}
        className="flex items-center gap-1 text-sm text-primary hover:text-primary/80"
      >
        <Plus size={14} /> Add Variable
      </button>
    </div>
  );
}

function ArgsEditor({
  args,
  onChange,
}: {
  args: string[];
  onChange: (args: string[]) => void;
}) {
  const addArg = () => onChange([...args, ""]);
  const removeArg = (i: number) => onChange(args.filter((_, idx) => idx !== i));
  const updateArg = (i: number, value: string) => {
    const updated = [...args];
    updated[i] = value;
    onChange(updated);
  };

  return (
    <div>
      <label className="text-sm font-medium text-slate-700 mb-2 block">Arguments</label>
      <div className="space-y-2 mb-3">
        {args.map((arg, i) => (
          <div key={i} className="flex gap-2 items-center">
            <span className="text-xs font-semibold text-muted w-5">{i + 1}</span>
            <input
              value={arg}
              onChange={(e) => updateArg(i, e.target.value)}
              className="flex-1 border border-border rounded px-2 py-1.5 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-primary"
              placeholder="argument"
            />
            <button
              onClick={() => removeArg(i)}
              className="p-1 text-muted hover:text-danger"
            >
              <X size={14} />
            </button>
          </div>
        ))}
      </div>
      <button
        onClick={addArg}
        className="flex items-center gap-1 text-sm text-primary hover:text-primary/80"
      >
        <Plus size={14} /> Add Argument
      </button>
    </div>
  );
}

function InputParamsEditor({
  params,
  onChange,
}: {
  params: Record<string, ToolParam>;
  onChange: (params: Record<string, ToolParam>) => void;
}) {
  const entries = Object.entries(params);

  const addParam = () => {
    onChange({
      ...params,
      "": { type: "string", description: "", required: false },
    });
  };

  const removeParam = (key: string) => {
    const next = { ...params };
    delete next[key];
    onChange(next);
  };

  const updateKey = (oldKey: string, newKey: string) => {
    const next: Record<string, ToolParam> = {};
    for (const [k, v] of Object.entries(params)) {
      next[k === oldKey ? newKey : k] = v;
    }
    onChange(next);
  };

  const updateParam = (key: string, field: string, value: string | boolean) => {
    onChange({
      ...params,
      [key]: { ...params[key], [field]: value },
    });
  };

  return (
    <div>
      <label className="text-sm font-medium text-slate-700 mb-1 block">
        Input Parameters
        <span className="text-xs text-muted font-normal ml-2">
          LLM-fillable parameters (use {"{{name}}"} in args)
        </span>
      </label>
      <div className="space-y-3 mb-3">
        {entries.map(([key, param], i) => (
          <div
            key={i}
            className="border border-border rounded-md p-3 bg-slate-50 space-y-2"
          >
            <div className="flex gap-2 items-center">
              <input
                value={key}
                onChange={(e) => updateKey(key, e.target.value)}
                className="w-40 border border-border rounded px-2 py-1.5 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-primary"
                placeholder="param_name"
              />
              <select
                value={param.type}
                onChange={(e) => updateParam(key, "type", e.target.value)}
                className="border border-border rounded px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
              >
                <option value="string">string</option>
                <option value="number">number</option>
                <option value="boolean">boolean</option>
              </select>
              <label className="flex items-center gap-1 text-sm text-slate-600">
                <input
                  type="checkbox"
                  checked={param.required}
                  onChange={(e) => updateParam(key, "required", e.target.checked)}
                  className="rounded border-border"
                />
                Required
              </label>
              <button
                onClick={() => removeParam(key)}
                className="ml-auto p-1 text-muted hover:text-danger"
              >
                <X size={14} />
              </button>
            </div>
            <input
              value={param.description}
              onChange={(e) => updateParam(key, "description", e.target.value)}
              className="w-full border border-border rounded px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
              placeholder="Parameter description"
            />
            <input
              value={param.default || ""}
              onChange={(e) => updateParam(key, "default", e.target.value)}
              className="w-full border border-border rounded px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
              placeholder="Default value (optional)"
            />
          </div>
        ))}
      </div>
      <button
        onClick={addParam}
        className="flex items-center gap-1 text-sm text-primary hover:text-primary/80"
      >
        <Plus size={14} /> Add Parameter
      </button>
    </div>
  );
}

function ToolForm({
  data,
  onChange,
}: {
  data: ToolFormData;
  onChange: (data: ToolFormData) => void;
}) {
  return (
    <div className="space-y-4">
      <div>
        <label className="text-sm font-medium text-slate-700 mb-1 block">Name *</label>
        <input
          autoFocus
          value={data.name}
          onChange={(e) => onChange({ ...data, name: e.target.value })}
          placeholder="Tool name (e.g. oc_get_pods)"
          className="w-full border border-border rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
        />
      </div>

      <div className="grid grid-cols-3 gap-3">
        <div>
          <label className="text-sm font-medium text-slate-700 mb-1 block">Type *</label>
          <select
            value={data.category}
            onChange={(e) => onChange({ ...data, category: e.target.value })}
            className="w-full border border-border rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
          >
            <option value="">Select type...</option>
            {CATEGORIES.map((c) => (
              <option key={c} value={c}>
                {c}
              </option>
            ))}
          </select>
        </div>
        <div>
          <label className="text-sm font-medium text-slate-700 mb-1 block">Status</label>
          <select
            value={data.status}
            onChange={(e) =>
              onChange({ ...data, status: e.target.value as "draft" | "verify" | "approved" })
            }
            className="w-full border border-border rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
          >
            <option value="draft">Draft</option>
            <option value="verify">Verify</option>
            <option value="approved">Approved</option>
          </select>
        </div>
        <div>
          <label className="text-sm font-medium text-slate-700 mb-1 block">Author</label>
          <input
            value={data.author}
            onChange={(e) => onChange({ ...data, author: e.target.value })}
            placeholder="Author name"
            className="w-full border border-border rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
          />
        </div>
      </div>

      <div>
        <label className="text-sm font-medium text-slate-700 mb-1 block">Description</label>
        <textarea
          value={data.description}
          onChange={(e) => onChange({ ...data, description: e.target.value })}
          placeholder="What does this tool do?"
          rows={2}
          className="w-full border border-border rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary resize-none"
        />
      </div>

      <div>
        <label className="text-sm font-medium text-slate-700 mb-1 block">Command *</label>
        <input
          value={data.command}
          onChange={(e) => onChange({ ...data, command: e.target.value })}
          placeholder="e.g. go run, python, bash"
          className="w-full border border-border rounded-md px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-primary"
        />
      </div>

      <ArgsEditor args={data.args} onChange={(args) => onChange({ ...data, args })} />

      <InputParamsEditor
        params={data.input_params}
        onChange={(input_params) => onChange({ ...data, input_params })}
      />

      <EnvEditor env={data.env} onChange={(env) => onChange({ ...data, env })} />
    </div>
  );
}

export default function ToolsPage() {
  const { selectedProject } = useProjectContext();
  const [tools, setTools] = useState<Tool[]>([]);
  const [search, setSearch] = useState("");
  const [loading, setLoading] = useState(true);
  const [modalOpen, setModalOpen] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [saving, setSaving] = useState(false);
  const [expandedId, setExpandedId] = useState<number | null>(null);
  const [categoryFilter, setCategoryFilter] = useState<string>("all");
  const [testTool, setTestTool] = useState<Tool | null>(null);
  const [formData, setFormData] = useState<ToolFormData>({
    name: "",
    description: "",
    command: "",
    args: [],
    env: {},
    input_params: {},
    category: "",
    status: "draft" as const,
    author: "",
  });

  const loadTools = useCallback(async () => {
    if (!selectedProject) return;
    setLoading(true);
    try {
      const data = await api.tools.list(selectedProject.id, search);
      setTools(data);
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  }, [selectedProject, search]);

  useEffect(() => {
    loadTools();
  }, [loadTools]);

  const openNew = () => {
    setEditingId(null);
    setFormData({
      name: "",
      description: "",
      command: "",
      args: [],
      env: {},
      input_params: {},
      category: categoryFilter !== "all" ? categoryFilter : "",
      status: "draft" as const,
      author: "",
    });
    setModalOpen(true);
  };

  const openEdit = (t: Tool) => {
    setEditingId(t.id);
    setFormData({
      name: t.name,
      description: t.description,
      command: t.command,
      args: t.args || [],
      env: t.env || {},
      input_params: t.input_params || {},
      category: t.category,
      status: t.status,
      author: t.author,
    });
    setModalOpen(true);
  };

  const handleSave = async () => {
    if (!selectedProject || !formData.name.trim()) return;
    setSaving(true);
    try {
      if (editingId) {
        await api.tools.update(editingId, formData);
      } else {
        await api.tools.create(selectedProject.id, formData);
      }
      setModalOpen(false);
      await loadTools();
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (id: number) => {
    if (!confirm("Delete this tool?")) return;
    await api.tools.delete(id);
    await loadTools();
  };

  const handleStatusChange = async (id: number, newStatus: string) => {
    await api.tools.setStatus(id, newStatus);
    await loadTools();
  };

  if (!selectedProject) {
    return (
      <div className="p-8">
        <EmptyState
          icon={<Wrench size={48} />}
          title="No Project Selected"
          description="Select a project from the sidebar to manage tools."
        />
      </div>
    );
  }

  const categoryCounts: Record<string, number> = { all: tools.length };
  for (const c of CATEGORIES) {
    categoryCounts[c] = tools.filter((t) => t.category === c).length;
  }

  const filteredTools =
    categoryFilter === "all"
      ? tools
      : tools.filter((t) => t.category === categoryFilter);

  const statusCounts = {
    draft: filteredTools.filter((t) => t.status === "draft").length,
    verify: filteredTools.filter((t) => t.status === "verify").length,
    approved: filteredTools.filter((t) => t.status === "approved").length,
  };

  return (
    <div className="p-8">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">Tools</h1>
          <p className="text-sm text-muted mt-1">
            Manage project tools — Go, Python, and Shell commands
          </p>
        </div>
        <button
          onClick={openNew}
          className="flex items-center gap-2 px-4 py-2 bg-primary text-white rounded-lg text-sm font-semibold hover:bg-primary/90 transition-colors shadow-sm"
        >
          <Plus size={16} /> Add Tool
        </button>
      </div>

      {/* Status Summary Cards */}
      <div className="grid grid-cols-3 gap-4 mb-6">
        <div className="bg-white border border-border rounded-lg p-4 flex items-center gap-3">
          <div className="w-10 h-10 rounded-lg bg-indigo-100 flex items-center justify-center">
            <FileEdit size={20} className="text-indigo-600" />
          </div>
          <div>
            <div className="text-2xl font-bold text-slate-900">{statusCounts.draft}</div>
            <div className="text-xs text-muted uppercase tracking-wider">Draft</div>
          </div>
        </div>
        <div className="bg-white border border-border rounded-lg p-4 flex items-center gap-3">
          <div className="w-10 h-10 rounded-lg bg-amber-100 flex items-center justify-center">
            <Clock size={20} className="text-amber-600" />
          </div>
          <div>
            <div className="text-2xl font-bold text-slate-900">{statusCounts.verify}</div>
            <div className="text-xs text-muted uppercase tracking-wider">In Verify</div>
          </div>
        </div>
        <div className="bg-white border border-border rounded-lg p-4 flex items-center gap-3">
          <div className="w-10 h-10 rounded-lg bg-emerald-100 flex items-center justify-center">
            <CheckCircle size={20} className="text-emerald-600" />
          </div>
          <div>
            <div className="text-2xl font-bold text-slate-900">{statusCounts.approved}</div>
            <div className="text-xs text-muted uppercase tracking-wider">Approved</div>
          </div>
        </div>
      </div>

      {/* Category Tabs + Search */}
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-1 bg-slate-100 rounded-lg p-1">
          <button
            onClick={() => setCategoryFilter("all")}
            className={`px-3 py-1.5 rounded-md text-sm font-medium transition-colors ${
              categoryFilter === "all"
                ? "bg-white text-slate-900 shadow-sm"
                : "text-muted hover:text-slate-700"
            }`}
          >
            All
            <span className="ml-1.5 text-xs text-muted">({categoryCounts.all})</span>
          </button>
          {CATEGORIES.map((cat) => {
            const meta = categoryMeta[cat];
            return (
              <button
                key={cat}
                onClick={() => setCategoryFilter(cat)}
                className={`flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm font-medium transition-colors ${
                  categoryFilter === cat
                    ? "bg-white text-slate-900 shadow-sm"
                    : "text-muted hover:text-slate-700"
                }`}
              >
                <span className={`font-mono text-[10px] font-bold ${meta.color}`}>
                  {meta.icon}
                </span>
                {cat}
                <span className="text-xs text-muted">({categoryCounts[cat]})</span>
              </button>
            );
          })}
        </div>

        <div className="relative w-64">
          <Search
            size={16}
            className="absolute left-3 top-1/2 -translate-y-1/2 text-muted"
          />
          <input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search tools..."
            className="w-full pl-9 pr-3 py-2 border border-border rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary"
          />
        </div>
      </div>

      {/* Tools Table */}
      {loading ? (
        <div className="text-center py-12 text-muted">Loading...</div>
      ) : filteredTools.length === 0 ? (
        <EmptyState
          icon={<Wrench size={48} />}
          title={categoryFilter !== "all" ? `No ${categoryFilter} Tools` : "No Tools Yet"}
          description={
            categoryFilter !== "all"
              ? `No ${categoryFilter} tools in this project. Add one to get started.`
              : "Add tools to your project — Go, Python, and Shell commands used in test execution."
          }
          action={
            <button
              onClick={openNew}
              className="flex items-center gap-2 px-4 py-2 bg-primary text-white rounded-lg text-sm font-semibold hover:bg-primary/90"
            >
              <Plus size={16} /> Add Tool
            </button>
          }
        />
      ) : (
        <div className="bg-white border border-border rounded-lg shadow-sm overflow-hidden">
          <table className="w-full">
            <thead>
              <tr className="bg-slate-50">
                <th className="text-left px-4 py-3 text-xs font-semibold text-muted uppercase tracking-wider w-8" />
                <th className="text-left px-4 py-3 text-xs font-semibold text-muted uppercase tracking-wider">
                  Name
                </th>
                <th className="text-left px-4 py-3 text-xs font-semibold text-muted uppercase tracking-wider">
                  Command
                </th>
                <th className="text-left px-4 py-3 text-xs font-semibold text-muted uppercase tracking-wider">
                  Type
                </th>
                <th className="text-left px-4 py-3 text-xs font-semibold text-muted uppercase tracking-wider">
                  Status
                </th>
                <th className="text-left px-4 py-3 text-xs font-semibold text-muted uppercase tracking-wider">
                  Author
                </th>
                <th className="text-left px-4 py-3 text-xs font-semibold text-muted uppercase tracking-wider">
                  Updated
                </th>
                <th className="text-left px-4 py-3 text-xs font-semibold text-muted uppercase tracking-wider w-36" />
              </tr>
            </thead>
            <tbody>
              {filteredTools.map((t) => {
                const flow = statusFlow[t.status];
                return (
                  <ToolRow
                    key={t.id}
                    tool={t}
                    expanded={expandedId === t.id}
                    onToggle={() => setExpandedId(expandedId === t.id ? null : t.id)}
                    onEdit={() => openEdit(t)}
                    onDelete={() => handleDelete(t.id)}
                    onTest={() => setTestTool(t)}
                    onStatusChange={handleStatusChange}
                    flow={flow}
                  />
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {/* Create/Edit Modal */}
      <Modal
        open={modalOpen}
        onClose={() => setModalOpen(false)}
        title={editingId ? "Edit Tool" : "New Tool"}
        wide
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
              className="px-4 py-2 text-sm bg-primary text-white rounded-md hover:bg-primary/90 transition-colors disabled:opacity-50"
            >
              {saving ? "Saving..." : editingId ? "Update" : "Create"}
            </button>
          </>
        }
      >
        <ToolForm data={formData} onChange={setFormData} />
      </Modal>

      {/* Test Modal */}
      <ToolTestModal
        tool={testTool}
        open={!!testTool}
        onClose={() => setTestTool(null)}
      />
    </div>
  );
}

function ToolRow({
  tool,
  expanded,
  onToggle,
  onEdit,
  onDelete,
  onTest,
  onStatusChange,
  flow,
}: {
  tool: Tool;
  expanded: boolean;
  onToggle: () => void;
  onEdit: () => void;
  onDelete: () => void;
  onTest: () => void;
  onStatusChange: (id: number, status: string) => void;
  flow: { next: string; label: string; color: string } | undefined;
}) {
  return (
    <>
      <tr
        className="border-t border-border hover:bg-slate-50/50 transition-colors cursor-pointer"
        onClick={onToggle}
      >
        <td className="px-4 py-3 text-muted">
          {expanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
        </td>
        <td className="px-4 py-3">
          <div className="flex items-center gap-2">
            <Wrench size={14} className="text-primary shrink-0" />
            <span className="text-sm font-medium text-slate-800">{tool.name}</span>
          </div>
        </td>
        <td className="px-4 py-3">
          <code className="text-xs bg-slate-100 px-2 py-0.5 rounded font-mono text-slate-700">
            {tool.command || "—"}
          </code>
        </td>
        <td className="px-4 py-3">
          <CategoryBadge category={tool.category} />
        </td>
        <td className="px-4 py-3">
          <Badge value={tool.status} />
        </td>
        <td className="px-4 py-3 text-sm text-muted">{tool.author || "—"}</td>
        <td className="px-4 py-3 text-sm text-muted">{formatDate(tool.updated_at)}</td>
        <td className="px-4 py-3">
          <div className="flex gap-1" onClick={(e) => e.stopPropagation()}>
            <button
              onClick={onTest}
              className="p-1.5 text-muted hover:text-emerald-600 transition-colors"
              title="Test Tool"
            >
              <Play size={14} />
            </button>
            {flow && (
              <button
                onClick={() => onStatusChange(tool.id, flow.next)}
                className={`flex items-center gap-1 px-2 py-1 text-xs text-white rounded ${flow.color} transition-colors`}
                title={flow.label}
              >
                <ArrowRight size={12} />
                <span className="hidden xl:inline">{flow.label}</span>
              </button>
            )}
            {tool.status !== "draft" && (
              <button
                onClick={() => onStatusChange(tool.id, "draft")}
                className="p-1.5 text-muted hover:text-indigo-600 transition-colors"
                title="Reset to Draft"
              >
                <FileEdit size={14} />
              </button>
            )}
            <button
              onClick={onEdit}
              className="p-1.5 text-muted hover:text-primary transition-colors"
              title="Edit"
            >
              <Pencil size={14} />
            </button>
            <button
              onClick={onDelete}
              className="p-1.5 text-muted hover:text-danger transition-colors"
              title="Delete"
            >
              <Trash2 size={14} />
            </button>
          </div>
        </td>
      </tr>

      {/* Expanded Detail */}
      {expanded && (
        <tr className="border-t border-border bg-slate-50/50">
          <td colSpan={8} className="px-8 py-4">
            <div className="grid grid-cols-2 gap-6">
              <div>
                {tool.description && (
                  <div className="mb-3">
                    <div className="text-[11px] text-muted uppercase font-semibold mb-1">
                      Description
                    </div>
                    <p className="text-sm text-slate-700 whitespace-pre-wrap">
                      {tool.description}
                    </p>
                  </div>
                )}
                <div className="mb-3">
                  <div className="text-[11px] text-muted uppercase font-semibold mb-1">
                    Command
                  </div>
                  <div className="flex items-center gap-2">
                    <Terminal size={14} className="text-muted" />
                    <code className="text-sm font-mono bg-slate-100 px-2 py-1 rounded">
                      {tool.command}
                      {(tool.args?.length || 0) > 0 && ` ${tool.args.join(" ")}`}
                    </code>
                  </div>
                </div>
                <div className="mb-3">
                  <div className="text-[11px] text-muted uppercase font-semibold mb-1">
                    Type
                  </div>
                  <CategoryBadge category={tool.category} />
                </div>
                <div className="mb-3">
                  <div className="text-[11px] text-muted uppercase font-semibold mb-1">
                    Status Flow
                  </div>
                  <div className="flex items-center gap-2 text-sm">
                    <span
                      className={`px-2 py-0.5 rounded text-xs font-semibold ${
                        tool.status === "draft"
                          ? "bg-indigo-200 text-indigo-800"
                          : "bg-indigo-50 text-indigo-400"
                      }`}
                    >
                      Draft
                    </span>
                    <ArrowRight size={12} className="text-muted" />
                    <span
                      className={`px-2 py-0.5 rounded text-xs font-semibold ${
                        tool.status === "verify"
                          ? "bg-amber-200 text-amber-800"
                          : "bg-amber-50 text-amber-400"
                      }`}
                    >
                      Verify
                    </span>
                    <ArrowRight size={12} className="text-muted" />
                    <span
                      className={`px-2 py-0.5 rounded text-xs font-semibold ${
                        tool.status === "approved"
                          ? "bg-emerald-200 text-emerald-800"
                          : "bg-emerald-50 text-emerald-400"
                      }`}
                    >
                      Approved
                    </span>
                  </div>
                </div>
              </div>
              <div>
                {(tool.args?.length || 0) > 0 && (
                  <div className="mb-3">
                    <div className="text-[11px] text-muted uppercase font-semibold mb-1">
                      Arguments ({tool.args.length})
                    </div>
                    <div className="space-y-1">
                      {tool.args.map((arg, i) => (
                        <div
                          key={i}
                          className="flex items-center gap-2 text-sm font-mono bg-white border border-border rounded px-2 py-1"
                        >
                          <span className="text-xs text-muted">{i + 1}.</span>
                          <span className="text-slate-700">{arg}</span>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
                {tool.input_params && Object.keys(tool.input_params).length > 0 && (
                  <div className="mb-3">
                    <div className="text-[11px] text-muted uppercase font-semibold mb-1">
                      Input Parameters ({Object.keys(tool.input_params).length})
                    </div>
                    <div className="space-y-1">
                      {Object.entries(tool.input_params).map(([key, param]) => (
                        <div
                          key={key}
                          className="text-sm bg-white border border-border rounded px-2 py-1.5"
                        >
                          <div className="flex items-center gap-2">
                            <span className="font-mono text-indigo-700">{key}</span>
                            <span className="text-xs bg-slate-100 px-1.5 py-0.5 rounded text-muted">
                              {param.type}
                            </span>
                            {param.required && (
                              <span className="text-xs bg-red-100 text-red-700 px-1.5 py-0.5 rounded">
                                required
                              </span>
                            )}
                            {param.default && (
                              <span className="text-xs text-muted">
                                default: <code>{param.default}</code>
                              </span>
                            )}
                          </div>
                          {param.description && (
                            <div className="text-xs text-muted mt-0.5">{param.description}</div>
                          )}
                        </div>
                      ))}
                    </div>
                  </div>
                )}
                {tool.env && Object.keys(tool.env).length > 0 && (
                  <div className="mb-3">
                    <div className="text-[11px] text-muted uppercase font-semibold mb-1">
                      Environment Variables ({Object.keys(tool.env).length})
                    </div>
                    <div className="space-y-1">
                      {Object.entries(tool.env).map(([key, val]) => (
                        <div
                          key={key}
                          className="flex items-center gap-1 text-sm font-mono bg-white border border-border rounded px-2 py-1"
                        >
                          <span className="text-indigo-700">{key}</span>
                          <span className="text-muted">=</span>
                          <span className="text-slate-600">{val}</span>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            </div>
          </td>
        </tr>
      )}
    </>
  );
}
