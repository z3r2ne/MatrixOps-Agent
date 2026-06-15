import React, { useEffect, useState } from "react"

import AppShell from "@/components/layout/AppShell"
import { NotificationProvider } from "@/components/NotificationProvider"
import { Skeleton } from "@/components/ui/skeleton"
import { api } from "@/lib/api"
import { ViewState } from "@/types"
import CodeReviewView from "@/views/CodeReviewView"
import KanbanView from "@/views/KanbanView"
import SettingsView from "@/views/SettingsView"
import WelcomeView from "@/views/WelcomeView"
import WorkbenchView from "@/views/WorkbenchView"
import WorkspacesView from "@/views/WorkspacesView"
import { PromptsPage } from "@/pages/PromptsPage"

const App: React.FC = () => {
  const [currentView, setCurrentView] = useState<ViewState>("workspaces")
  const [isLoading, setIsLoading] = useState(true)
  const [hasWorkspaces, setHasWorkspaces] = useState<boolean | null>(null)
  const [currentProject, setCurrentProject] = useState<{ id: number; name: string } | null>(null)
  const [settingsTab, setSettingsTab] = useState<"providers" | "workers">("workers")

  const refreshWorkspaces = async () => {
    setIsLoading(true)
    try {
      const workspaces = await api.getWorkspaces()
      const valid = workspaces.filter((ws) => ws.pathExists !== false)
      setHasWorkspaces(valid.length > 0)
    } catch {
      setHasWorkspaces(false)
    } finally {
      setIsLoading(false)
    }
  }

  useEffect(() => {
    refreshWorkspaces()
  }, [])

  const handleNavigate = (view: ViewState) => {
    if (view === "settings") {
      setSettingsTab("workers")
    }
    if (view === "kanban") {
      setCurrentProject(null)
    }
    setCurrentView(view)
  }

  const renderView = () => {
    switch (currentView) {
      case "workspaces":
        return (
          <WorkspacesView
            onNavigate={handleNavigate}
            onOpenProject={(project) => {
              setCurrentProject(project)
              setCurrentView("kanban")
            }}
            onRefresh={refreshWorkspaces}
          />
        )
      case "kanban":
        return (
          <KanbanView
            project={currentProject}
            onRequireProvider={() => {
              setSettingsTab("providers")
              setCurrentView("settings")
            }}
          />
        )
      case "settings":
        return <SettingsView defaultTab={settingsTab} />
      case "codereview":
        return <CodeReviewView />
      case "workbench":
        return <WorkbenchView />
      case "prompts":
        return <PromptsPage />
      default:
        return null
    }
  }

  if (isLoading) {
    return (
      <NotificationProvider>
        <div className="min-h-screen bg-background p-8">
          <div className="mx-auto max-w-4xl space-y-6">
            <Skeleton className="h-14 w-full rounded-2xl" />
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              <Skeleton className="h-40 rounded-2xl" />
              <Skeleton className="h-40 rounded-2xl" />
              <Skeleton className="h-40 rounded-2xl" />
            </div>
            <Skeleton className="h-64 w-full rounded-2xl" />
          </div>
        </div>
      </NotificationProvider>
    )
  }

  if (hasWorkspaces === false) {
    return (
      <NotificationProvider>
        <WelcomeView onWorkspaceCreated={refreshWorkspaces} />
      </NotificationProvider>
    )
  }

  return (
    <NotificationProvider>
      <AppShell currentView={currentView} onNavigate={handleNavigate}>
        {renderView()}
      </AppShell>
    </NotificationProvider>
  )
}

export default App
