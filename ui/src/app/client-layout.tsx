"use client";

import { Sidebar } from "@/components/Sidebar";
import { ProjectProvider } from "@/components/ProjectContext";

export function ClientLayout({ children }: { children: React.ReactNode }) {
  return (
    <ProjectProvider>
      <div className="flex min-h-screen">
        <Sidebar />
        <main className="ml-60 flex-1 min-h-screen">{children}</main>
      </div>
    </ProjectProvider>
  );
}
