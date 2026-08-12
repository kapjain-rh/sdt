"use client";

import { useState, useEffect, useCallback } from "react";
import { api } from "./api";
import type { Project } from "./types";

export function useProjects() {
  const [projects, setProjects] = useState<Project[]>([]);
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const data = await api.projects.list();
      setProjects(data);
    } catch (e) {
      console.error("Failed to load projects", e);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  return { projects, loading, refresh };
}

export function useSelectedProject() {
  const [selectedId, setSelectedId] = useState<number | null>(null);

  useEffect(() => {
    const stored = localStorage.getItem("tcms_project_id");
    if (stored) setSelectedId(Number(stored));
  }, []);

  const select = useCallback((id: number) => {
    setSelectedId(id);
    localStorage.setItem("tcms_project_id", String(id));
  }, []);

  return { selectedId, select };
}
