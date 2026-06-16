import type { LucideIcon } from "lucide-react"
import { FolderTree, Terminal } from "lucide-react"

export const WORKBENCH_PANEL_DEFINITIONS = {
  terminal: {
    id: "terminal",
    label: "终端",
    icon: Terminal,
  },
  filesystem: {
    id: "filesystem",
    label: "文件系统",
    icon: FolderTree,
  },
} as const satisfies Record<string, WorkbenchPanelDefinition>

export type WorkbenchPanelId = keyof typeof WORKBENCH_PANEL_DEFINITIONS

export interface WorkbenchPanelDefinition {
  id: WorkbenchPanelId
  label: string
  icon: LucideIcon
}

export const WORKBENCH_PANEL_LIST: WorkbenchPanelDefinition[] = Object.values(WORKBENCH_PANEL_DEFINITIONS)
