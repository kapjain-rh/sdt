"use client";

import { Plus, Trash2, GripVertical } from "lucide-react";

export function StepListEditor({
  label,
  steps,
  onChange,
  placeholder = "Step description...",
}: {
  label: string;
  steps: string[];
  onChange: (steps: string[]) => void;
  placeholder?: string;
}) {
  const addStep = () => onChange([...steps, ""]);
  const removeStep = (i: number) => onChange(steps.filter((_, idx) => idx !== i));
  const updateStep = (i: number, val: string) => {
    const next = [...steps];
    next[i] = val;
    onChange(next);
  };

  return (
    <div>
      <div className="flex items-center justify-between mb-1.5">
        <label className="text-sm font-medium text-slate-700">{label}</label>
        <button
          type="button"
          onClick={addStep}
          className="flex items-center gap-1 text-xs text-primary hover:underline"
        >
          <Plus size={12} /> Add Step
        </button>
      </div>
      {steps.length === 0 ? (
        <div className="text-xs text-muted italic py-2">No steps. Click "Add Step" to begin.</div>
      ) : (
        <div className="space-y-1.5">
          {steps.map((step, i) => (
            <div key={i} className="flex gap-2 items-start">
              <span className="text-xs text-muted font-mono w-5 pt-2.5 text-right shrink-0">
                {i + 1}.
              </span>
              <input
                value={step}
                onChange={(e) => updateStep(i, e.target.value)}
                className="flex-1 border border-border rounded-md px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
                placeholder={placeholder}
              />
              <button
                type="button"
                onClick={() => removeStep(i)}
                className="p-1.5 text-muted hover:text-danger transition-colors shrink-0"
              >
                <Trash2 size={13} />
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
