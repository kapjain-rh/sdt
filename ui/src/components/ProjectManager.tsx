"use client";

import { useState } from "react";
import { Plus, Pencil, Trash2, FolderKanban } from "lucide-react";
import { Modal } from "./Modal";
import { useProjectContext } from "./ProjectContext";
import { api } from "@/lib/api";
import { formatDate } from "@/lib/utils";

export function ProjectManager({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { projects, refreshProjects, selectProject } = useProjectContext();
  const [editing, setEditing] = useState<{ id?: number; name: string; description: string } | null>(null);
  const [saving, setSaving] = useState(false);

  const handleSave = async () => {
    if (!editing || !editing.name.trim()) return;
    setSaving(true);
    try {
      if (editing.id) {
        await api.projects.update(editing.id, { name: editing.name, description: editing.description });
      } else {
        const p = await api.projects.create({ name: editing.name, description: editing.description });
        selectProject(p.id);
      }
      await refreshProjects();
      setEditing(null);
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (id: number) => {
    if (!confirm("Delete this project and all its data?")) return;
    await api.projects.delete(id);
    await refreshProjects();
  };

  return (
    <Modal open={open} onClose={onClose} title="Manage Projects" wide>
      <div className="flex justify-between items-center mb-4">
        <p className="text-sm text-muted">Create and manage your testing projects.</p>
        <button
          onClick={() => setEditing({ name: "", description: "" })}
          className="flex items-center gap-1.5 px-3 py-1.5 bg-primary text-white text-sm rounded-md hover:bg-primary-hover transition-colors"
        >
          <Plus size={16} /> New Project
        </button>
      </div>

      {editing && (
        <div className="border border-border rounded-lg p-4 mb-4 bg-slate-50">
          <div className="grid gap-3">
            <div>
              <label className="text-sm font-medium text-slate-700 mb-1 block">Name</label>
              <input
                autoFocus
                value={editing.name}
                onChange={(e) => setEditing({ ...editing, name: e.target.value })}
                className="w-full border border-border rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
                placeholder="Project name"
              />
            </div>
            <div>
              <label className="text-sm font-medium text-slate-700 mb-1 block">Description</label>
              <textarea
                value={editing.description}
                onChange={(e) => setEditing({ ...editing, description: e.target.value })}
                className="w-full border border-border rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary min-h-[60px] resize-y"
                placeholder="Optional description"
              />
            </div>
            <div className="flex gap-2 justify-end">
              <button
                onClick={() => setEditing(null)}
                className="px-3 py-1.5 text-sm border border-border rounded-md hover:bg-slate-100 transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={handleSave}
                disabled={saving || !editing.name.trim()}
                className="px-3 py-1.5 text-sm bg-primary text-white rounded-md hover:bg-primary-hover transition-colors disabled:opacity-50"
              >
                {saving ? "Saving..." : editing.id ? "Update" : "Create"}
              </button>
            </div>
          </div>
        </div>
      )}

      <div className="space-y-2">
        {projects.length === 0 && (
          <div className="text-center py-8 text-muted text-sm">No projects yet. Create one to get started.</div>
        )}
        {projects.map((p) => (
          <div
            key={p.id}
            className="flex items-center justify-between border border-border rounded-lg px-4 py-3 hover:bg-slate-50 transition-colors"
          >
            <div className="flex items-center gap-3">
              <FolderKanban size={18} className="text-primary" />
              <div>
                <div className="font-medium text-sm text-slate-800">{p.name}</div>
                {p.description && (
                  <div className="text-xs text-muted mt-0.5">{p.description}</div>
                )}
              </div>
            </div>
            <div className="flex items-center gap-3">
              <span className="text-xs text-muted">{formatDate(p.created_at)}</span>
              <button
                onClick={() => setEditing({ id: p.id, name: p.name, description: p.description })}
                className="p-1 text-muted hover:text-slate-700 transition-colors"
              >
                <Pencil size={15} />
              </button>
              <button
                onClick={() => handleDelete(p.id)}
                className="p-1 text-muted hover:text-danger transition-colors"
              >
                <Trash2 size={15} />
              </button>
            </div>
          </div>
        ))}
      </div>
    </Modal>
  );
}
