"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import {
  Save,
  Play,
  RotateCcw,
  ChevronDown,
  ChevronRight,
  FileText,
  Circle,
  Trash2,
  Plus,
} from "lucide-react";

/* ── Section definition ── */

export interface SectionDef {
  key: string;
  label: string;
  mdHeader: string;
  color: string;
  dotColor: string;
  hint: string;
}

/* ── Section editor (one collapsible block) ── */

function StepRow({
  index,
  lineNum,
  value,
  section,
  onChangeStep,
  onRemove,
  onKeyDown,
  autoFocus,
}: {
  index: number;
  lineNum: number;
  value: string;
  section: SectionDef;
  onChangeStep: (i: number, v: string) => void;
  onRemove: (i: number) => void;
  onKeyDown: (i: number, e: React.KeyboardEvent<HTMLTextAreaElement>) => void;
  autoFocus: boolean;
}) {
  const ref = useRef<HTMLTextAreaElement>(null);

  const adjustHeight = useCallback(() => {
    const ta = ref.current;
    if (!ta) return;
    ta.style.height = "auto";
    ta.style.height = Math.max(ta.scrollHeight, 22) + "px";
  }, []);

  useEffect(() => { adjustHeight(); }, [value, adjustHeight]);
  useEffect(() => { if (autoFocus && ref.current) ref.current.focus(); }, [autoFocus]);

  return (
    <div className="flex group/step hover:bg-[#2a2d3a]/50 transition-colors">
      <div className="w-12 shrink-0 pt-[3px]">
        <div className="h-[22px] text-right pr-3 text-xs text-slate-600 font-mono tabular-nums leading-[22px]">
          {lineNum}
        </div>
      </div>
      <div className="w-4 shrink-0 ml-1 relative pt-[3px]">
        <div className={`absolute left-[7px] top-0 bottom-0 w-[2px] ${section.dotColor} opacity-20 rounded`} />
        <div className="h-[22px] flex items-center justify-center">
          {value.trim() ? (
            <Circle size={5} className={`${section.color} fill-current`} />
          ) : (
            <span className="w-[5px]" />
          )}
        </div>
      </div>
      <div className="flex-1 ml-1 pr-3 min-w-0">
        <textarea
          ref={ref}
          value={value}
          onChange={(e) => { onChangeStep(index, e.target.value); }}
          onKeyDown={(e) => onKeyDown(index, e)}
          spellCheck={false}
          rows={1}
          className="w-full bg-transparent text-slate-300 text-sm font-mono leading-[22px] resize-none outline-none placeholder:text-slate-700 caret-blue-400 pt-[3px] pb-[1px]"
          style={{ overflow: "hidden" }}
          placeholder={index === 0 ? "Type a step..." : ""}
        />
      </div>
      <div className="w-6 shrink-0 pt-[3px] flex items-start justify-center">
        <button
          onClick={() => onRemove(index)}
          className="h-[22px] flex items-center text-slate-700 hover:text-red-400 transition-colors opacity-0 group-hover/step:opacity-100"
          tabIndex={-1}
        >
          <Trash2 size={11} />
        </button>
      </div>
    </div>
  );
}

export function IDESectionEditor({
  section,
  steps,
  onChange,
  lineStart,
  collapsed,
  onToggle,
  onRun,
}: {
  section: SectionDef;
  steps: string[];
  onChange: (steps: string[]) => void;
  lineStart: number;
  collapsed: boolean;
  onToggle: () => void;
  onRun?: () => void;
}) {
  const [focusIndex, setFocusIndex] = useState(-1);

  const displaySteps = steps.length > 0 ? steps : [""];

  const onChangeStep = (i: number, val: string) => {
    const lines = val.split("\n");
    if (lines.length > 1) {
      const next = [...displaySteps];
      next.splice(i, 1, ...lines);
      onChange(next);
      setFocusIndex(i + lines.length - 1);
    } else {
      const next = [...displaySteps];
      next[i] = val;
      onChange(next);
    }
  };

  const onRemoveStep = (i: number) => {
    if (displaySteps.length <= 1) {
      onChange([""]);
      return;
    }
    const next = displaySteps.filter((_, idx) => idx !== i);
    onChange(next);
    setFocusIndex(Math.min(i, next.length - 1));
  };

  const onAddStep = () => {
    onChange([...displaySteps, ""]);
    setFocusIndex(displaySteps.length);
  };

  const handleKeyDown = (i: number, e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      const ta = e.currentTarget;
      const pos = ta.selectionStart;
      const val = displaySteps[i];
      const before = val.substring(0, pos);
      const after = val.substring(pos);
      const next = [...displaySteps];
      next.splice(i, 1, before, after);
      onChange(next);
      setFocusIndex(i + 1);
    } else if (e.key === "Backspace" && displaySteps[i] === "" && displaySteps.length > 1) {
      e.preventDefault();
      onRemoveStep(i);
      setFocusIndex(Math.max(0, i - 1));
    } else if (e.key === "Tab") {
      e.preventDefault();
      const ta = e.currentTarget;
      const start = ta.selectionStart;
      const end = ta.selectionEnd;
      const val = displaySteps[i];
      const next = [...displaySteps];
      next[i] = val.substring(0, start) + "  " + val.substring(end);
      onChange(next);
      requestAnimationFrame(() => {
        ta.selectionStart = ta.selectionEnd = start + 2;
      });
    }
  };

  useEffect(() => {
    if (focusIndex >= 0) setFocusIndex(-1);
  }, [focusIndex]);

  return (
    <div className="group/sec">
      {/* Header */}
      <div className="flex items-center h-7 hover:bg-[#2a2d3a] transition-colors select-none">
        <span className="w-12 text-right pr-3 text-xs text-slate-600 font-mono shrink-0 tabular-nums">
          {lineStart}
        </span>
        <div className="w-4 shrink-0 ml-1">
          <button onClick={onToggle} className="text-slate-500 hover:text-slate-300 transition-colors">
            {collapsed ? <ChevronRight size={14} /> : <ChevronDown size={14} />}
          </button>
        </div>
        <span className={`font-bold font-mono text-sm ${section.color} ml-1`}>
          {section.mdHeader}
        </span>
        <span className="text-slate-600 text-xs ml-3 italic hidden group-hover/sec:inline">
          {section.hint}
        </span>
        <div className="ml-auto flex items-center gap-1 pr-3 opacity-0 group-hover/sec:opacity-100 transition-opacity">
          {onRun && steps.length > 0 && (
            <button
              onClick={onRun}
              className="flex items-center gap-1 px-2 py-0.5 text-[11px] bg-emerald-600/80 text-emerald-100 rounded hover:bg-emerald-500 transition-colors"
            >
              <Play size={10} /> Run
            </button>
          )}
          <span className="text-[10px] text-slate-600 tabular-nums">
            {steps.length} step{steps.length !== 1 ? "s" : ""}
          </span>
        </div>
      </div>

      {/* Content — one textarea per step so line numbers track wrapping */}
      {!collapsed && (
        <div>
          {displaySteps.map((step, i) => (
            <StepRow
              key={i}
              index={i}
              lineNum={lineStart + 1 + i}
              value={step}
              section={section}
              onChangeStep={onChangeStep}
              onRemove={onRemoveStep}
              onKeyDown={handleKeyDown}
              autoFocus={focusIndex === i}
            />
          ))}
          <div className="flex items-center h-6 opacity-0 group-hover/sec:opacity-100 transition-opacity">
            <div className="w-12 shrink-0" />
            <div className="w-4 shrink-0 ml-1" />
            <button
              onClick={onAddStep}
              className="flex items-center gap-1 ml-1 text-[11px] text-blue-400 hover:text-blue-300 font-mono transition-colors"
            >
              <Plus size={10} /> add step
            </button>
          </div>
        </div>
      )}

      {collapsed && steps.length > 0 && (
        <div className="flex items-center h-6 text-xs text-slate-600 italic pl-[72px]">
          {steps.length} step{steps.length !== 1 ? "s" : ""} (collapsed)
        </div>
      )}
    </div>
  );
}

/* ── Metadata input line ── */

export function IDEMetaLine({
  lineNum,
  label,
  value,
  onChange,
  placeholder,
  color = "text-orange-300",
}: {
  lineNum: number;
  label: string;
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  color?: string;
}) {
  return (
    <div className="flex items-center h-7 hover:bg-[#2a2d3a] transition-colors">
      <span className="w-12 text-right pr-3 text-xs text-slate-600 font-mono shrink-0 tabular-nums">
        {lineNum}
      </span>
      <div className="w-4 shrink-0 ml-1" />
      <span className="text-slate-500 font-mono text-sm mr-1">-</span>
      <span className="text-cyan-400 font-mono text-sm mr-1">{label}:</span>
      <input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className={`flex-1 bg-transparent ${color} text-sm font-mono outline-none caret-blue-400 placeholder:text-slate-700`}
        placeholder={placeholder}
        spellCheck={false}
      />
    </div>
  );
}

/* ── Metadata select line ── */

export function IDEMetaSelect({
  lineNum,
  label,
  value,
  onChange,
  options,
}: {
  lineNum: number;
  label: string;
  value: string;
  onChange: (v: string) => void;
  options: { value: string; label: string }[];
}) {
  return (
    <div className="flex items-center h-7 hover:bg-[#2a2d3a] transition-colors">
      <span className="w-12 text-right pr-3 text-xs text-slate-600 font-mono shrink-0 tabular-nums">
        {lineNum}
      </span>
      <div className="w-4 shrink-0 ml-1" />
      <span className="text-slate-500 font-mono text-sm mr-1">-</span>
      <span className="text-cyan-400 font-mono text-sm mr-1">{label}:</span>
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="bg-[#313244] text-orange-300 text-sm font-mono outline-none border border-[#414458] rounded px-1 py-0 caret-blue-400"
      >
        {options.map((o) => (
          <option key={o.value} value={o.value}>{o.label}</option>
        ))}
      </select>
    </div>
  );
}

/* ── Blank line ── */

export function IDEBlankLine({ lineNum }: { lineNum?: number }) {
  return (
    <div className="flex items-center h-5">
      <span className="w-12 text-right pr-3 text-xs text-slate-600 font-mono shrink-0 tabular-nums">
        {lineNum ?? ""}
      </span>
      <div className="w-4 shrink-0 ml-1" />
    </div>
  );
}

/* ── Separator line (---) ── */

export function IDESeparator({ lineNum }: { lineNum: number }) {
  return (
    <div className="flex items-center h-6">
      <span className="w-12 text-right pr-3 text-xs text-slate-600 font-mono shrink-0 tabular-nums">
        {lineNum}
      </span>
      <div className="w-4 shrink-0 ml-1" />
      <span className="text-slate-600 font-mono text-sm">---</span>
    </div>
  );
}

/* ── Title line (# heading) ── */

export function IDETitleLine({
  lineNum,
  value,
  onChange,
  placeholder,
}: {
  lineNum: number;
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
}) {
  return (
    <div className="flex items-center h-8 hover:bg-[#2a2d3a] transition-colors">
      <span className="w-12 text-right pr-3 text-xs text-slate-600 font-mono shrink-0 tabular-nums">
        {lineNum}
      </span>
      <div className="w-4 shrink-0 ml-1" />
      <span className="text-purple-400 font-bold font-mono text-sm mr-2">#</span>
      <input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="flex-1 bg-transparent text-slate-200 text-lg font-mono font-bold outline-none caret-blue-400 placeholder:text-slate-700"
        placeholder={placeholder || "Title"}
        spellCheck={false}
      />
    </div>
  );
}

/* ── Toolbar ── */

export function IDEToolbar({
  fileName,
  dirty,
  saving,
  canSave,
  onSave,
  onReset,
  onDelete,
  stats,
}: {
  fileName: string;
  dirty: boolean;
  saving: boolean;
  canSave: boolean;
  onSave: () => void;
  onReset?: () => void;
  onDelete?: () => void;
  stats?: string;
}) {
  return (
    <div className="flex items-center justify-between px-4 py-2 bg-[#1e1e2e] border-b border-[#313244]">
      <div className="flex items-center gap-3">
        <div className="flex items-center gap-2 bg-[#313244] rounded px-2 py-1">
          <FileText size={14} className="text-slate-400" />
          <span className="text-sm text-slate-300 font-mono">{fileName}</span>
          {dirty && <span className="w-2 h-2 rounded-full bg-blue-400" title="Unsaved changes" />}
        </div>
        {stats && <span className="text-xs text-slate-500">{stats}</span>}
      </div>
      <div className="flex items-center gap-2">
        {onDelete && (
          <button
            onClick={onDelete}
            className="flex items-center gap-1.5 px-3 py-1.5 text-xs text-red-400 hover:text-red-300 border border-red-900/50 rounded hover:bg-red-900/20 transition-colors"
          >
            <Trash2 size={12} /> Delete
          </button>
        )}
        {dirty && onReset && (
          <button
            onClick={onReset}
            className="flex items-center gap-1.5 px-3 py-1.5 text-xs text-slate-400 hover:text-slate-200 border border-[#313244] rounded hover:bg-[#313244] transition-colors"
          >
            <RotateCcw size={12} /> Reset
          </button>
        )}
        <button
          onClick={onSave}
          disabled={saving || !dirty || !canSave}
          className="flex items-center gap-1.5 px-4 py-1.5 text-xs bg-blue-600 text-white rounded hover:bg-blue-500 transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
        >
          <Save size={12} /> {saving ? "Saving..." : "Save"}
        </button>
      </div>
    </div>
  );
}

/* ── Status bar ── */

export function IDEStatusBar({
  lineCount,
  fileName,
  dirty,
  lang = "Markdown",
}: {
  lineCount: number;
  fileName?: string;
  dirty: boolean;
  lang?: string;
}) {
  return (
    <div className="flex items-center justify-between px-4 py-1 bg-[#181825] border-t border-[#313244] text-[11px] text-slate-500">
      <div className="flex items-center gap-4">
        <span>{lang}</span>
        <span>UTF-8</span>
        <span>{lineCount} lines</span>
      </div>
      <div className="flex items-center gap-4">
        {fileName && <span>{fileName}</span>}
        <span>{dirty ? "Modified" : "Saved"}</span>
      </div>
    </div>
  );
}

/* ── Full-height shell ── */

export function IDEShell({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex flex-col h-[calc(100vh-64px)]">
      {children}
    </div>
  );
}

export function IDEBody({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex-1 overflow-auto bg-[#1e1e2e]">
      <div className="min-h-full py-2">
        {children}
        <div className="h-32" />
      </div>
    </div>
  );
}

/* ── Textarea meta line (multi-line YAML value) ── */

function MetaTextareaRow({
  index,
  lineNum,
  value,
  color,
  onChangeLine,
  onKeyDown,
  autoFocus,
  placeholder,
}: {
  index: number;
  lineNum: number;
  value: string;
  color: string;
  onChangeLine: (i: number, v: string) => void;
  onKeyDown: (i: number, e: React.KeyboardEvent<HTMLTextAreaElement>) => void;
  autoFocus: boolean;
  placeholder?: string;
}) {
  const ref = useRef<HTMLTextAreaElement>(null);

  const adjustHeight = useCallback(() => {
    const ta = ref.current;
    if (!ta) return;
    ta.style.height = "auto";
    ta.style.height = Math.max(ta.scrollHeight, 22) + "px";
  }, []);

  useEffect(() => { adjustHeight(); }, [value, adjustHeight]);
  useEffect(() => { if (autoFocus && ref.current) ref.current.focus(); }, [autoFocus]);

  return (
    <div className="flex hover:bg-[#2a2d3a]/50 transition-colors">
      <div className="w-12 shrink-0 pt-[3px]">
        <div className="h-[22px] text-right pr-3 text-xs text-slate-600 font-mono tabular-nums leading-[22px]">
          {lineNum}
        </div>
      </div>
      <div className="w-4 shrink-0 ml-1" />
      <div className="flex-1 ml-1 pr-3 min-w-0">
        <textarea
          ref={ref}
          value={value}
          onChange={(e) => onChangeLine(index, e.target.value)}
          onKeyDown={(e) => onKeyDown(index, e)}
          spellCheck={false}
          rows={1}
          className={`w-full bg-transparent ${color} text-sm font-mono leading-[22px] resize-none outline-none placeholder:text-slate-700 caret-blue-400 pt-[3px] pb-[1px]`}
          style={{ overflow: "hidden" }}
          placeholder={index === 0 ? placeholder : ""}
        />
      </div>
    </div>
  );
}

export function IDEMetaTextarea({
  lineStart,
  label,
  value,
  onChange,
  placeholder,
  color = "text-green-300",
  onRun,
}: {
  lineStart: number;
  label: string;
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  color?: string;
  onRun?: () => void;
}) {
  const [focusIndex, setFocusIndex] = useState(-1);
  const lines = value ? value.split("\n") : [""];

  const onChangeLine = (i: number, val: string) => {
    const parts = val.split("\n");
    if (parts.length > 1) {
      const next = [...lines];
      next.splice(i, 1, ...parts);
      onChange(next.join("\n"));
      setFocusIndex(i + parts.length - 1);
    } else {
      const next = [...lines];
      next[i] = val;
      onChange(next.join("\n"));
    }
  };

  const handleKeyDown = (i: number, e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      const ta = e.currentTarget;
      const pos = ta.selectionStart;
      const val = lines[i];
      const before = val.substring(0, pos);
      const after = val.substring(pos);
      const next = [...lines];
      next.splice(i, 1, before, after);
      onChange(next.join("\n"));
      setFocusIndex(i + 1);
    } else if (e.key === "Backspace" && lines[i] === "" && lines.length > 1) {
      e.preventDefault();
      const next = lines.filter((_, idx) => idx !== i);
      onChange(next.join("\n"));
      setFocusIndex(Math.max(0, i - 1));
    }
  };

  useEffect(() => {
    if (focusIndex >= 0) setFocusIndex(-1);
  }, [focusIndex]);

  return (
    <div className="group/ta">
      <div className="flex items-center h-7 hover:bg-[#2a2d3a] transition-colors">
        <span className="w-12 text-right pr-3 text-xs text-slate-600 font-mono shrink-0 tabular-nums">{lineStart}</span>
        <div className="w-4 shrink-0 ml-1" />
        <span className="text-cyan-400 font-mono text-sm font-semibold">{label}:</span>
        {onRun && value.trim() && (
          <button onClick={onRun} className="ml-auto mr-3 flex items-center gap-1 px-2 py-0.5 text-[11px] bg-emerald-600/80 text-emerald-100 rounded hover:bg-emerald-500 transition-colors opacity-0 group-hover/ta:opacity-100">
            <Play size={10} /> Run
          </button>
        )}
      </div>
      <div>
        {lines.map((line, i) => (
          <MetaTextareaRow
            key={i}
            index={i}
            lineNum={lineStart + 1 + i}
            value={line}
            color={color}
            onChangeLine={onChangeLine}
            onKeyDown={handleKeyDown}
            autoFocus={focusIndex === i}
            placeholder={placeholder}
          />
        ))}
      </div>
    </div>
  );
}

/* ── List editor (templates, inline items) ── */

export function IDEListEditor({
  lineStart,
  label,
  items,
  onChange,
  placeholder,
  color = "text-pink-300",
}: {
  lineStart: number;
  label: string;
  items: string[];
  onChange: (items: string[]) => void;
  placeholder?: string;
  color?: string;
}) {
  const updateItem = (i: number, v: string) => { const n = [...items]; n[i] = v; onChange(n); };
  const removeItem = (i: number) => onChange(items.filter((_, idx) => idx !== i));
  const addItem = () => onChange([...items, ""]);

  return (
    <div className="group/list">
      <div className="flex items-center h-7 hover:bg-[#2a2d3a] transition-colors">
        <span className="w-12 text-right pr-3 text-xs text-slate-600 font-mono shrink-0 tabular-nums">{lineStart}</span>
        <div className="w-4 shrink-0 ml-1" />
        <span className="text-cyan-400 font-mono text-sm font-semibold">{label}:</span>
        <span className="text-[10px] text-slate-600 ml-2 tabular-nums">{items.length} item{items.length !== 1 ? "s" : ""}</span>
        <button onClick={addItem} className="ml-auto mr-3 text-[11px] text-blue-400 hover:text-blue-300 opacity-0 group-hover/list:opacity-100 transition-opacity font-mono">+ add</button>
      </div>
      {items.map((item, i) => (
        <div key={i} className="flex items-center h-[26px] hover:bg-[#2a2d3a] transition-colors group/item">
          <span className="w-12 text-right pr-3 text-xs text-slate-600 font-mono shrink-0 tabular-nums">{lineStart + 1 + i}</span>
          <div className="w-4 shrink-0 ml-1" />
          <span className="text-slate-500 font-mono text-sm mr-1">-</span>
          <input
            value={item}
            onChange={(e) => updateItem(i, e.target.value)}
            className={`flex-1 bg-transparent ${color} text-sm font-mono outline-none caret-blue-400 placeholder:text-slate-700`}
            placeholder={placeholder}
            spellCheck={false}
          />
          <button onClick={() => removeItem(i)} className="mr-3 text-slate-600 hover:text-red-400 transition-colors opacity-0 group-hover/item:opacity-100">
            <Trash2 size={12} />
          </button>
        </div>
      ))}
    </div>
  );
}

/* ── Key-Value editor (parameters) ── */

export function IDEKeyValueEditor({
  lineStart,
  label,
  entries,
  onChange,
}: {
  lineStart: number;
  label: string;
  entries: Record<string, string>;
  onChange: (entries: Record<string, string>) => void;
}) {
  const keys = Object.keys(entries);

  const updateKey = (oldKey: string, newKey: string) => {
    const next: Record<string, string> = {};
    for (const [k, v] of Object.entries(entries)) {
      next[k === oldKey ? newKey : k] = v;
    }
    onChange(next);
  };

  const updateVal = (key: string, val: string) => { onChange({ ...entries, [key]: val }); };
  const removeEntry = (key: string) => { const n = { ...entries }; delete n[key]; onChange(n); };
  const addEntry = () => { onChange({ ...entries, [`param${keys.length + 1}`]: "" }); };

  return (
    <div className="group/kv">
      <div className="flex items-center h-7 hover:bg-[#2a2d3a] transition-colors">
        <span className="w-12 text-right pr-3 text-xs text-slate-600 font-mono shrink-0 tabular-nums">{lineStart}</span>
        <div className="w-4 shrink-0 ml-1" />
        <span className="text-cyan-400 font-mono text-sm font-semibold">{label}:</span>
        <span className="text-[10px] text-slate-600 ml-2 tabular-nums">{keys.length} key{keys.length !== 1 ? "s" : ""}</span>
        <button onClick={addEntry} className="ml-auto mr-3 text-[11px] text-blue-400 hover:text-blue-300 opacity-0 group-hover/kv:opacity-100 transition-opacity font-mono">+ add</button>
      </div>
      {keys.map((key, i) => (
        <div key={i} className="flex items-center h-[26px] hover:bg-[#2a2d3a] transition-colors group/kvitem">
          <span className="w-12 text-right pr-3 text-xs text-slate-600 font-mono shrink-0 tabular-nums">{lineStart + 1 + i}</span>
          <div className="w-4 shrink-0 ml-1" />
          <span className="text-slate-500 font-mono text-sm mr-0.5 ml-2" />
          <input
            value={key}
            onChange={(e) => updateKey(key, e.target.value)}
            className="w-36 bg-transparent text-yellow-300 text-sm font-mono outline-none caret-blue-400 placeholder:text-slate-700"
            placeholder="key"
            spellCheck={false}
          />
          <span className="text-slate-500 font-mono text-sm mx-1">:</span>
          <input
            value={entries[key]}
            onChange={(e) => updateVal(key, e.target.value)}
            className="flex-1 bg-transparent text-green-300 text-sm font-mono outline-none caret-blue-400 placeholder:text-slate-700"
            placeholder="value"
            spellCheck={false}
          />
          <button onClick={() => removeEntry(key)} className="mr-3 text-slate-600 hover:text-red-400 transition-colors opacity-0 group-hover/kvitem:opacity-100">
            <Trash2 size={12} />
          </button>
        </div>
      ))}
    </div>
  );
}
