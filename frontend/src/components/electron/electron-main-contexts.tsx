import React, { createContext, useContext } from "react"

/** 设置弹层内的当前子页（不经过 react-router，避免嵌套 Router） */
export type ElectronSettingsPanel =
  | "settings"
  | "workspaces"
  | "projects"
  | "memory-libraries"
  | "rag-libraries"
  | "skills"
  | "mcp"
  | "search"
  | "embedding"
  | "skills-sources"
  | "logs"
  | "usage"
  | "prompts"
  | "ilink"
  | "debug"

export const ElectronSettingsPanelContext = createContext<{
  panel: ElectronSettingsPanel
  setPanel: (p: ElectronSettingsPanel) => void
} | null>(null)

export function useElectronSettingsPanel() {
  return useContext(ElectronSettingsPanelContext)
}

/** 主窗口标题栏：打开「设置」弹层 */
export type ElectronOpenSettingsOptions = {
  panel?: ElectronSettingsPanel
  usageTaskId?: number
  initialProjectId?: number
}

export const ElectronOpenSettingsContext = createContext<((options?: ElectronOpenSettingsOptions) => void) | null>(null)

export function useElectronOpenSettings(): ((options?: ElectronOpenSettingsOptions) => void) | null {
  return useContext(ElectronOpenSettingsContext)
}

export const ElectronUsageAnalyticsFilterContext = createContext<{
  taskId?: number
} | null>(null)

export function useElectronUsageAnalyticsFilter() {
  return useContext(ElectronUsageAnalyticsFilterContext)
}

/** 设置弹层内：关闭弹层（返回主任务界面） */
export const ElectronSettingsShellCloseContext = createContext<(() => void) | null>(null)

export function useElectronSettingsShellClose(): (() => void) | null {
  return useContext(ElectronSettingsShellCloseContext)
}

/** 设置弹层内：自动选中的项目 ID（用于提示词页面） */
export const ElectronSettingsInitialProjectIdContext = createContext<number | undefined>(undefined)

export function useElectronSettingsInitialProjectId(): number | undefined {
  return useContext(ElectronSettingsInitialProjectIdContext)
}
