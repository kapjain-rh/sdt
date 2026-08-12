"use client";

import { createContext, useContext, useState, useEffect, useCallback, type ReactNode } from "react";
import { api } from "@/lib/api";
import type { Project } from "@/lib/types";

interface ProjectContextType {
  projects: Project[];
  selectedProject: Project | null;
  selectProject: (id: number) => void;
  refreshProjects: () => Promise<void>;
  loading: boolean;
}

const ProjectContext = createContext<ProjectContextType>({
  projects: [],
  selectedProject: null,
  selectProject: () => {},
  refreshProjects: async () => {},
  loading: true,
});

export function useProjectContext() {
  return useContext(ProjectContext);
}

export function ProjectProvider({ children }: { children: ReactNode }) {
  const [projects, setProjects] = useState<Project[]>([]);
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);

  const refreshProjects = useCallback(async () => {
    try {
      const data = await api.projects.list();
      setProjects(data);
      if (data.length > 0 && !selectedId) {
        const stored = typeof window !== "undefined" ? localStorage.getItem("tcms_project") : null;
        const id = stored ? Number(stored) : data[0].id;
        const valid = data.find((p) => p.id === id);
        setSelectedId(valid ? id : data[0].id);
      }
    } catch {
      // API not available yet
    } finally {
      setLoading(false);
    }
  }, [selectedId]);

  useEffect(() => {
    refreshProjects();
  }, [refreshProjects]);

  const selectProject = useCallback(
    (id: number) => {
      setSelectedId(id);
      localStorage.setItem("tcms_project", String(id));
    },
    []
  );

  const selectedProject = projects.find((p) => p.id === selectedId) || null;

  return (
    <ProjectContext.Provider value={{ projects, selectedProject, selectProject, refreshProjects, loading }}>
      {children}
    </ProjectContext.Provider>
  );
}
