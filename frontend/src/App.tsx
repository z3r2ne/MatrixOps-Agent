import React from "react"
import { BrowserRouter, Routes, Route } from "react-router-dom"
import { ConfirmDialogProvider } from "@/components/ConfirmDialogProvider"
import { NotificationProvider } from "@/components/NotificationProvider"
import { WorkbenchProvider } from "@/components/workbench/WorkbenchProvider"
import { MainLayout } from "@/components/layout/MainLayout"
import { WorkbenchPage } from "@/pages/WorkbenchPage"
import { WorkspacesPage } from "@/pages/WorkspacesPage"
import { WorkspaceDetailPage } from "@/pages/WorkspaceDetailPage"
import { NewTaskPage } from "@/pages/NewTaskPage"
import { SettingsPage } from "@/pages/SettingsPage"
import { LogsPage } from "@/pages/LogsPage"
import { UsageAnalyticsPage } from "@/pages/UsageAnalyticsPage"
import { PromptsPage } from "@/pages/PromptsPage"
import { ProjectsPage } from "@/pages/ProjectsPage"
import { MemoryLibrariesPage } from "@/pages/MemoryLibrariesPage"
import { RagLibrariesPage } from "@/pages/RagLibrariesPage"
import { DebugPage } from "@/pages/DebugPage"
import { SkillsPage } from "@/pages/SkillsPage"
import { SkillSourcesPage } from "@/pages/SkillSourcesPage"
import { McpServersPage } from "@/pages/McpServersPage"
import { HomePage } from "@/pages/HomePage"
import { ElectronLauncherPage } from "@/pages/ElectronLauncherPage"

function App() {
  return (
    <NotificationProvider>
      <ConfirmDialogProvider>
        <WorkbenchProvider>
          <BrowserRouter>
            <Routes>
              <Route path="/electron-launcher" element={<ElectronLauncherPage />} />
              <Route element={<MainLayout />}>
                <Route path="/" element={<HomePage />} />
                <Route path="/workbench" element={<WorkbenchPage />} />
                <Route path="/workspaces" element={<WorkspacesPage />} />
                <Route path="/workspace/:id" element={<WorkspaceDetailPage />} />
                <Route path="/workspace/:id/new-task" element={<NewTaskPage />} />
                <Route path="/projects" element={<ProjectsPage />} />
                <Route path="/memory-libraries" element={<MemoryLibrariesPage />} />
                <Route path="/rag-libraries" element={<RagLibrariesPage />} />
                <Route path="/skills" element={<SkillsPage />} />
                <Route path="/skills/sources" element={<SkillSourcesPage />} />
                <Route path="/mcp" element={<McpServersPage />} />
                <Route path="/prompts" element={<PromptsPage />} />
                <Route path="/settings" element={<SettingsPage />} />
                <Route path="/logs" element={<LogsPage />} />
                <Route path="/usage" element={<UsageAnalyticsPage />} />
                <Route path="/debug" element={<DebugPage />} />
              </Route>
            </Routes>
          </BrowserRouter>
        </WorkbenchProvider>
      </ConfirmDialogProvider>
    </NotificationProvider>
  )
}

export default App
