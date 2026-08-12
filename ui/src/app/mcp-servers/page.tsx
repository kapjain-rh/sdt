"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import {
  Plus,
  Server,
  Pencil,
  Trash2,
  ChevronRight,
  ChevronDown,
  Plug,
  Unplug,
  RefreshCw,
  X,
  Loader2,
  Wrench,
  Play,
  Clock,
} from "lucide-react";
import { useProjectContext } from "@/components/ProjectContext";
import { Modal } from "@/components/Modal";
import { Badge } from "@/components/Badge";
import { EmptyState } from "@/components/EmptyState";
import { api } from "@/lib/api";
import { formatDate, formatDuration } from "@/lib/utils";
import type { MCPServer, MCPDiscoveredTool } from "@/lib/types";

interface MCPFormData {
  name: string;
  command: string;
  args: string[];
  env: Record<string, string>;
}

function ArgsEditor({
  args,
  onChange,
}: {
  args: string[];
  onChange: (args: string[]) => void;
}) {
  return (
    <div>
      <label className="text-sm font-medium text-slate-700 mb-2 block">Arguments</label>
      <div className="space-y-2 mb-3">
        {args.map((arg, i) => (
          <div key={i} className="flex gap-2 items-center">
            <span className="text-xs font-semibold text-muted w-5">{i + 1}</span>
            <input
              value={arg}
              onChange={(e) => {
                const updated = [...args];
                updated[i] = e.target.value;
                onChange(updated);
              }}
              className="flex-1 border border-border rounded px-2 py-1.5 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-primary"
              placeholder="argument"
            />
            <button
              onClick={() => onChange(args.filter((_, idx) => idx !== i))}
              className="p-1 text-muted hover:text-danger"
            >
              <X size={14} />
            </button>
          </div>
        ))}
      </div>
      <button
        onClick={() => onChange([...args, ""])}
        className="flex items-center gap-1 text-sm text-primary hover:text-primary/80"
      >
        <Plus size={14} /> Add Argument
      </button>
    </div>
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
              onChange={(e) => {
                const next: Record<string, string> = {};
                for (const [k, v] of Object.entries(env)) {
                  next[k === key ? e.target.value : k] = v;
                }
                onChange(next);
              }}
              className="flex-1 border border-border rounded px-2 py-1.5 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-primary"
              placeholder="KEY"
            />
            <span className="text-muted">=</span>
            <input
              value={val}
              onChange={(e) => onChange({ ...env, [key]: e.target.value })}
              className="flex-1 border border-border rounded px-2 py-1.5 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-primary"
              placeholder="value"
            />
            <button
              onClick={() => {
                const next = { ...env };
                delete next[key];
                onChange(next);
              }}
              className="p-1 text-muted hover:text-danger"
            >
              <X size={14} />
            </button>
          </div>
        ))}
      </div>
      <button
        onClick={() => onChange({ ...env, "": "" })}
        className="flex items-center gap-1 text-sm text-primary hover:text-primary/80"
      >
        <Plus size={14} /> Add Variable
      </button>
    </div>
  );
}

const streamColors: Record<string, string> = {
  stdout: "text-green-400",
  stderr: "text-red-400",
  system: "text-blue-400",
};

const streamIcons: Record<string, string> = {
  stdout: "→",
  stderr: "✗",
  system: "●",
};

function ToolCallModal({
  open,
  onClose,
  serverId,
  tool,
}: {
  open: boolean;
  onClose: () => void;
  serverId: number;
  tool: MCPDiscoveredTool | null;
}) {
  const [phase, setPhase] = useState<"idle" | "running" | "done">("idle");
  const [argsText, setArgsText] = useState("{}");
  const [logs, setLogs] = useState<{ id: number; stream: string; message: string; timestamp: string }[]>([]);
  const [status, setStatus] = useState<string | null>(null);
  const [exitCode, setExitCode] = useState(0);
  const [durationMs, setDurationMs] = useState(0);
  const [elapsed, setElapsed] = useState(0);
  const logEndRef = useRef<HTMLDivElement>(null);
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const startRef = useRef(0);
  const eventSourceRef = useRef<EventSource | null>(null);

  useEffect(() => {
    if (!open) return;
    setPhase("idle");
    setArgsText("{}");
    setLogs([]);
    setStatus(null);
    setExitCode(0);
    setDurationMs(0);
    setElapsed(0);
    return () => {
      if (eventSourceRef.current) eventSourceRef.current.close();
      if (timerRef.current) clearInterval(timerRef.current);
    };
  }, [open, tool]);

  const scrollToBottom = useCallback(() => {
    logEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, []);

  useEffect(() => {
    scrollToBottom();
  }, [logs, scrollToBottom]);

  const handleCall = async () => {
    if (!tool) return;

    setPhase("running");
    setLogs([]);
    setStatus(null);

    startRef.current = Date.now();
    timerRef.current = setInterval(() => setElapsed(Date.now() - startRef.current), 200);

    try {
      const args = JSON.parse(argsText);
      const { run_id } = await api.mcpServers.callTool(serverId, tool.name, args);

      const es = new EventSource(`/api/tool-runs/${run_id}/stream`);
      eventSourceRef.current = es;

      es.onmessage = (event) => {
        const data = JSON.parse(event.data);

        if (data.type === "done") {
          setStatus(data.status);
          setExitCode(data.exit_code);
          setDurationMs(data.duration);
          setPhase("done");
          if (timerRef.current) clearInterval(timerRef.current);
          es.close();
          return;
        }

        setLogs((prev) => [...prev, data]);
      };

      es.onerror = () => {
        es.close();
        setPhase("done");
        setStatus("error");
        if (timerRef.current) clearInterval(timerRef.current);
      };
    } catch {
      setPhase("done");
      setStatus("error");
      if (timerRef.current) clearInterval(timerRef.current);
    }
  };

  if (!tool) return null;

  return (
    <Modal open={open} onClose={onClose} title="" wide>
      <div>
        {/* Header */}
        <div className="flex items-start justify-between mb-4">
          <div className="flex-1">
            <div className="flex items-center gap-2 mb-1">
              <Server size={18} className="text-primary" />
              <span className="text-xs font-semibold text-muted uppercase tracking-wider">
                MCP Tool Call
              </span>
              {phase === "running" && (
                <span className="flex items-center gap-1 text-xs text-blue-600 bg-blue-50 px-2 py-0.5 rounded-full">
                  <Loader2 size={12} className="animate-spin" /> Running
                </span>
              )}
              {phase === "done" && status && (
                <span
                  className={`flex items-center gap-1 text-xs px-2 py-0.5 rounded-full font-semibold uppercase ${
                    status === "passed"
                      ? "bg-emerald-100 text-emerald-800"
                      : "bg-red-100 text-red-800"
                  }`}
                >
                  {status}
                </span>
              )}
            </div>
            <h2 className="text-lg font-semibold text-slate-800">{tool.name}</h2>
            {tool.description && (
              <p className="text-xs text-muted mt-1">{tool.description}</p>
            )}
          </div>
          <div className="flex items-center gap-2 text-muted">
            <Clock size={16} />
            <span className="font-mono text-lg tabular-nums">
              {phase === "done" ? formatDuration(durationMs) : formatDuration(elapsed)}
            </span>
          </div>
        </div>

        {/* Idle: args form + run button */}
        {phase === "idle" && (
          <div className="bg-slate-50 border border-border rounded-lg p-4 space-y-4">
            {tool.inputSchema && (
              <details>
                <summary className="text-xs text-muted cursor-pointer hover:text-slate-600 font-semibold uppercase tracking-wider">
                  Input Schema
                </summary>
                <pre className="mt-2 text-xs bg-white border border-border rounded p-2 overflow-auto max-h-32 font-mono">
                  {JSON.stringify(tool.inputSchema, null, 2)}
                </pre>
              </details>
            )}

            <div>
              <label className="text-sm font-medium text-slate-700 mb-1 block">
                Arguments (JSON)
              </label>
              <textarea
                value={argsText}
                onChange={(e) => setArgsText(e.target.value)}
                rows={4}
                className="w-full border border-border rounded-md px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-primary resize-none"
                placeholder='{"key": "value"}'
              />
            </div>

            <div className="flex justify-end">
              <button
                onClick={handleCall}
                className="flex items-center gap-2 px-6 py-2.5 rounded-lg font-semibold text-sm text-white bg-emerald-500 hover:bg-emerald-600 transition-colors shadow-sm"
              >
                <Play size={18} /> Run
              </button>
            </div>
          </div>
        )}

        {/* Running / Done: terminal */}
        {(phase === "running" || phase === "done") && (
          <div className="bg-slate-900 rounded-lg overflow-hidden">
            <div className="flex items-center gap-2 px-4 py-2 bg-slate-800 border-b border-slate-700">
              <div className="flex gap-1.5">
                <div className="w-3 h-3 rounded-full bg-red-500/80" />
                <div className="w-3 h-3 rounded-full bg-yellow-500/80" />
                <div className="w-3 h-3 rounded-full bg-green-500/80" />
              </div>
              <span className="text-xs text-slate-400 font-mono ml-2">
                {tool.name} — MCP tool
              </span>
              {phase === "running" && (
                <Loader2 size={12} className="animate-spin text-blue-400 ml-auto" />
              )}
            </div>

            <div className="px-4 py-3 max-h-[400px] overflow-y-auto font-mono text-sm">
              {logs.map((log) => {
                const color = streamColors[log.stream] || "text-slate-400";
                const icon = streamIcons[log.stream] || "·";
                const isSep = log.message.startsWith("──");

                if (isSep) {
                  return (
                    <div key={log.id} className="text-slate-600 py-0.5 text-xs">
                      {log.message}
                    </div>
                  );
                }

                return (
                  <div key={log.id} className="flex gap-2 py-0.5">
                    <span className="text-slate-600 text-xs w-16 shrink-0 text-right tabular-nums">
                      {new Date(log.timestamp).toLocaleTimeString("en-US", {
                        hour12: false,
                        hour: "2-digit",
                        minute: "2-digit",
                        second: "2-digit",
                      })}
                    </span>
                    <span className={`w-4 text-center shrink-0 ${color}`}>{icon}</span>
                    <span
                      className={`break-words ${
                        log.stream === "system" ? "text-blue-400 font-semibold" : color
                      }`}
                    >
                      {log.message}
                    </span>
                  </div>
                );
              })}

              {phase === "running" && (
                <div className="flex items-center gap-2 py-1 text-blue-400">
                  <Loader2 size={14} className="animate-spin" />
                  <span className="text-sm animate-pulse">Calling tool...</span>
                </div>
              )}
              <div ref={logEndRef} />
            </div>

            {phase === "done" && (
              <div
                className={`px-4 py-3 border-t flex items-center justify-between ${
                  status === "passed"
                    ? "bg-emerald-900/30 border-emerald-800"
                    : "bg-red-900/30 border-red-800"
                }`}
              >
                <div className="flex items-center gap-3">
                  <div
                    className={`font-bold uppercase ${
                      status === "passed" ? "text-emerald-400" : "text-red-400"
                    }`}
                  >
                    {status}
                  </div>
                  <div className="text-xs text-slate-400">
                    Exit code: {exitCode} · Duration: {formatDuration(durationMs)}
                  </div>
                </div>
                <div className="flex gap-2">
                  <button
                    onClick={() => setPhase("idle")}
                    className="px-4 py-1.5 text-sm bg-slate-700 text-slate-200 rounded-md hover:bg-slate-600 transition-colors"
                  >
                    Re-run
                  </button>
                  <button
                    onClick={onClose}
                    className="px-4 py-1.5 text-sm bg-slate-700 text-slate-200 rounded-md hover:bg-slate-600 transition-colors"
                  >
                    Close
                  </button>
                </div>
              </div>
            )}
          </div>
        )}
      </div>
    </Modal>
  );
}

export default function MCPServersPage() {
  const { selectedProject } = useProjectContext();
  const [servers, setServers] = useState<MCPServer[]>([]);
  const [loading, setLoading] = useState(true);
  const [modalOpen, setModalOpen] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [saving, setSaving] = useState(false);
  const [expandedId, setExpandedId] = useState<number | null>(null);
  const [connectingId, setConnectingId] = useState<number | null>(null);
  const [serverTools, setServerTools] = useState<Record<number, MCPDiscoveredTool[]>>({});
  const [connectErrors, setConnectErrors] = useState<Record<number, string>>({});
  const [callTool, setCallTool] = useState<{
    serverId: number;
    tool: MCPDiscoveredTool;
  } | null>(null);
  const [formData, setFormData] = useState<MCPFormData>({
    name: "",
    command: "",
    args: [],
    env: {},
  });

  const loadServers = useCallback(async () => {
    if (!selectedProject) return;
    setLoading(true);
    try {
      const data = await api.mcpServers.list(selectedProject.id);
      setServers(data);
      // Load tools for connected servers
      for (const s of data) {
        if (s.status === "connected") {
          try {
            const tools = await api.mcpServers.tools(s.id);
            setServerTools((prev) => ({ ...prev, [s.id]: tools }));
          } catch {
            // ignore
          }
        }
      }
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  }, [selectedProject]);

  useEffect(() => {
    loadServers();
  }, [loadServers]);

  const openNew = () => {
    setEditingId(null);
    setFormData({ name: "", command: "", args: [], env: {} });
    setModalOpen(true);
  };

  const openEdit = (s: MCPServer) => {
    setEditingId(s.id);
    setFormData({
      name: s.name,
      command: s.command,
      args: s.args || [],
      env: s.env || {},
    });
    setModalOpen(true);
  };

  const handleSave = async () => {
    if (!selectedProject || !formData.name.trim()) return;
    setSaving(true);
    try {
      if (editingId) {
        await api.mcpServers.update(editingId, formData);
      } else {
        await api.mcpServers.create(selectedProject.id, formData);
      }
      setModalOpen(false);
      await loadServers();
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (id: number) => {
    if (!confirm("Delete this MCP server?")) return;
    await api.mcpServers.delete(id);
    setServerTools((prev) => {
      const next = { ...prev };
      delete next[id];
      return next;
    });
    await loadServers();
  };

  const handleConnect = async (id: number) => {
    setConnectingId(id);
    setConnectErrors((prev) => {
      const next = { ...prev };
      delete next[id];
      return next;
    });
    try {
      const result = await api.mcpServers.connect(id);
      if (result.status === "error") {
        setConnectErrors((prev) => ({ ...prev, [id]: result.error || "Connection failed" }));
      } else {
        setServerTools((prev) => ({ ...prev, [id]: result.tools }));
        setExpandedId(id);
      }
      await loadServers();
    } catch (e) {
      setConnectErrors((prev) => ({ ...prev, [id]: String(e) }));
    } finally {
      setConnectingId(null);
    }
  };

  const handleDisconnect = async (id: number) => {
    await api.mcpServers.disconnect(id);
    setServerTools((prev) => {
      const next = { ...prev };
      delete next[id];
      return next;
    });
    await loadServers();
  };

  const handleRefresh = async (id: number) => {
    try {
      const tools = await api.mcpServers.refresh(id);
      setServerTools((prev) => ({ ...prev, [id]: tools }));
    } catch {
      // ignore
    }
  };

  if (!selectedProject) {
    return (
      <div className="p-8">
        <EmptyState
          icon={<Server size={48} />}
          title="No Project Selected"
          description="Select a project from the sidebar to manage MCP servers."
        />
      </div>
    );
  }

  const connectedCount = servers.filter((s) => s.status === "connected").length;

  return (
    <div className="p-8">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">MCP Servers</h1>
          <p className="text-sm text-muted mt-1">
            Connect to MCP tool servers for LLM-powered test execution
          </p>
        </div>
        <button
          onClick={openNew}
          className="flex items-center gap-2 px-4 py-2 bg-primary text-white rounded-lg text-sm font-semibold hover:bg-primary/90 transition-colors shadow-sm"
        >
          <Plus size={16} /> Add Server
        </button>
      </div>

      {/* Summary */}
      <div className="grid grid-cols-2 gap-4 mb-6">
        <div className="bg-white border border-border rounded-lg p-4 flex items-center gap-3">
          <div className="w-10 h-10 rounded-lg bg-slate-100 flex items-center justify-center">
            <Server size={20} className="text-slate-600" />
          </div>
          <div>
            <div className="text-2xl font-bold text-slate-900">{servers.length}</div>
            <div className="text-xs text-muted uppercase tracking-wider">Configured</div>
          </div>
        </div>
        <div className="bg-white border border-border rounded-lg p-4 flex items-center gap-3">
          <div className="w-10 h-10 rounded-lg bg-emerald-100 flex items-center justify-center">
            <Plug size={20} className="text-emerald-600" />
          </div>
          <div>
            <div className="text-2xl font-bold text-slate-900">{connectedCount}</div>
            <div className="text-xs text-muted uppercase tracking-wider">Connected</div>
          </div>
        </div>
      </div>

      {/* Server List */}
      {loading ? (
        <div className="text-center py-12 text-muted">Loading...</div>
      ) : servers.length === 0 ? (
        <EmptyState
          icon={<Server size={48} />}
          title="No MCP Servers"
          description="Add an MCP server to connect external tools for LLM-powered test execution."
          action={
            <button
              onClick={openNew}
              className="flex items-center gap-2 px-4 py-2 bg-primary text-white rounded-lg text-sm font-semibold hover:bg-primary/90"
            >
              <Plus size={16} /> Add Server
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
                  Status
                </th>
                <th className="text-left px-4 py-3 text-xs font-semibold text-muted uppercase tracking-wider">
                  Tools
                </th>
                <th className="text-left px-4 py-3 text-xs font-semibold text-muted uppercase tracking-wider">
                  Updated
                </th>
                <th className="text-left px-4 py-3 text-xs font-semibold text-muted uppercase tracking-wider w-44" />
              </tr>
            </thead>
            <tbody>
              {servers.map((s) => {
                const expanded = expandedId === s.id;
                const tools = serverTools[s.id] || [];
                const isConnecting = connectingId === s.id;
                const connectError = connectErrors[s.id];

                return (
                  <ServerRow
                    key={s.id}
                    server={s}
                    expanded={expanded}
                    tools={tools}
                    isConnecting={isConnecting}
                    connectError={connectError}
                    onToggle={() => setExpandedId(expanded ? null : s.id)}
                    onEdit={() => openEdit(s)}
                    onDelete={() => handleDelete(s.id)}
                    onConnect={() => handleConnect(s.id)}
                    onDisconnect={() => handleDisconnect(s.id)}
                    onRefresh={() => handleRefresh(s.id)}
                    onCallTool={(tool) => setCallTool({ serverId: s.id, tool })}
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
        title={editingId ? "Edit MCP Server" : "New MCP Server"}
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
              disabled={saving || !formData.name.trim() || !formData.command.trim()}
              className="px-4 py-2 text-sm bg-primary text-white rounded-md hover:bg-primary/90 transition-colors disabled:opacity-50"
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
              placeholder="e.g. github, openshift, filesystem"
              className="w-full border border-border rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
            />
          </div>
          <div>
            <label className="text-sm font-medium text-slate-700 mb-1 block">Command *</label>
            <input
              value={formData.command}
              onChange={(e) => setFormData({ ...formData, command: e.target.value })}
              placeholder="e.g. npx, python, ./my-server"
              className="w-full border border-border rounded-md px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-primary"
            />
          </div>
          <ArgsEditor
            args={formData.args}
            onChange={(args) => setFormData({ ...formData, args })}
          />
          <EnvEditor
            env={formData.env}
            onChange={(env) => setFormData({ ...formData, env })}
          />
        </div>
      </Modal>

      {/* Tool Call Modal */}
      <ToolCallModal
        open={!!callTool}
        onClose={() => setCallTool(null)}
        serverId={callTool?.serverId || 0}
        tool={callTool?.tool || null}
      />
    </div>
  );
}

function ServerRow({
  server,
  expanded,
  tools,
  isConnecting,
  connectError,
  onToggle,
  onEdit,
  onDelete,
  onConnect,
  onDisconnect,
  onRefresh,
  onCallTool,
}: {
  server: MCPServer;
  expanded: boolean;
  tools: MCPDiscoveredTool[];
  isConnecting: boolean;
  connectError?: string;
  onToggle: () => void;
  onEdit: () => void;
  onDelete: () => void;
  onConnect: () => void;
  onDisconnect: () => void;
  onRefresh: () => void;
  onCallTool: (tool: MCPDiscoveredTool) => void;
}) {
  const isConnected = server.status === "connected";
  const cmdPreview = [server.command, ...(server.args || [])].join(" ");

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
            <Server size={14} className={isConnected ? "text-emerald-500" : "text-slate-400"} />
            <span className="text-sm font-medium text-slate-800">{server.name}</span>
          </div>
        </td>
        <td className="px-4 py-3">
          <code className="text-xs bg-slate-100 px-2 py-0.5 rounded font-mono text-slate-700 max-w-[300px] truncate block">
            {cmdPreview}
          </code>
        </td>
        <td className="px-4 py-3">
          {isConnecting ? (
            <span className="flex items-center gap-1 text-xs text-blue-600">
              <Loader2 size={12} className="animate-spin" /> Connecting...
            </span>
          ) : (
            <Badge value={server.status} />
          )}
        </td>
        <td className="px-4 py-3 text-sm text-muted">
          {isConnected ? (
            <span className="flex items-center gap-1">
              <Wrench size={12} />
              {tools.length}
            </span>
          ) : (
            "—"
          )}
        </td>
        <td className="px-4 py-3 text-sm text-muted">{formatDate(server.updated_at)}</td>
        <td className="px-4 py-3">
          <div className="flex gap-1" onClick={(e) => e.stopPropagation()}>
            {isConnected ? (
              <>
                <button
                  onClick={onRefresh}
                  className="p-1.5 text-muted hover:text-blue-600 transition-colors"
                  title="Refresh Tools"
                >
                  <RefreshCw size={14} />
                </button>
                <button
                  onClick={onDisconnect}
                  className="flex items-center gap-1 px-2 py-1 text-xs text-white bg-red-500 hover:bg-red-600 rounded transition-colors"
                  title="Disconnect"
                >
                  <Unplug size={12} />
                  <span className="hidden xl:inline">Disconnect</span>
                </button>
              </>
            ) : (
              <button
                onClick={onConnect}
                disabled={isConnecting}
                className="flex items-center gap-1 px-2 py-1 text-xs text-white bg-emerald-500 hover:bg-emerald-600 rounded transition-colors disabled:opacity-50"
                title="Connect"
              >
                {isConnecting ? (
                  <Loader2 size={12} className="animate-spin" />
                ) : (
                  <Plug size={12} />
                )}
                <span className="hidden xl:inline">Connect</span>
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
          <td colSpan={7} className="px-8 py-4">
            {connectError && (
              <div className="mb-4 bg-red-50 border border-red-200 rounded-lg p-3 text-sm text-red-700">
                <strong>Connection Error:</strong> {connectError}
              </div>
            )}

            <div className="grid grid-cols-2 gap-6">
              <div>
                <div className="mb-3">
                  <div className="text-[11px] text-muted uppercase font-semibold mb-1">
                    Command
                  </div>
                  <code className="text-sm font-mono bg-slate-100 px-2 py-1 rounded block">
                    {cmdPreview}
                  </code>
                </div>
                {server.args && server.args.length > 0 && (
                  <div className="mb-3">
                    <div className="text-[11px] text-muted uppercase font-semibold mb-1">
                      Arguments ({server.args.length})
                    </div>
                    <div className="space-y-1">
                      {server.args.map((arg, i) => (
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
                {server.env && Object.keys(server.env).length > 0 && (
                  <div className="mb-3">
                    <div className="text-[11px] text-muted uppercase font-semibold mb-1">
                      Environment ({Object.keys(server.env).length})
                    </div>
                    <div className="space-y-1">
                      {Object.entries(server.env).map(([key, val]) => (
                        <div
                          key={key}
                          className="flex items-center gap-1 text-sm font-mono bg-white border border-border rounded px-2 py-1"
                        >
                          <span className="text-indigo-700">{key}</span>
                          <span className="text-muted">=</span>
                          <span className="text-slate-600 truncate">
                            {val.length > 20 ? val.slice(0, 20) + "..." : val}
                          </span>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
              </div>

              <div>
                <div className="text-[11px] text-muted uppercase font-semibold mb-2">
                  Discovered Tools ({tools.length})
                </div>
                {!isConnected && tools.length === 0 && (
                  <p className="text-sm text-muted italic">
                    Connect the server to discover tools.
                  </p>
                )}
                {tools.length > 0 && (
                  <div className="space-y-2 max-h-[400px] overflow-y-auto">
                    {tools.map((tool) => (
                      <div
                        key={tool.name}
                        className="bg-white border border-border rounded-lg p-3"
                      >
                        <div className="flex items-center justify-between mb-1">
                          <div className="flex items-center gap-2">
                            <Wrench size={12} className="text-primary" />
                            <span className="text-sm font-semibold font-mono text-slate-800">
                              {tool.name}
                            </span>
                          </div>
                          {isConnected && (
                            <button
                              onClick={() => onCallTool(tool)}
                              className="flex items-center gap-1 px-2 py-0.5 text-xs text-primary hover:bg-primary/10 rounded transition-colors"
                            >
                              <Play size={10} /> Call
                            </button>
                          )}
                        </div>
                        <p className="text-xs text-muted leading-relaxed">
                          {tool.description || "No description"}
                        </p>
                        {tool.inputSchema && (
                          <details className="mt-1">
                            <summary className="text-[10px] text-muted cursor-pointer hover:text-slate-600">
                              Schema
                            </summary>
                            <pre className="mt-1 text-[10px] bg-slate-50 border border-border rounded p-1.5 overflow-auto max-h-24 font-mono">
                              {JSON.stringify(tool.inputSchema, null, 2)}
                            </pre>
                          </details>
                        )}
                      </div>
                    ))}
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
