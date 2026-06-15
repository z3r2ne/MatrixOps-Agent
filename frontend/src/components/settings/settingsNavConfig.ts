import type { LucideIcon } from "lucide-react"
import { Bot, Key, MessageCircle, Wrench } from "lucide-react"

export type SettingsTabId = "system" | "workers" | "providers" | "models"

export type SettingsNavItem = {
  id: SettingsTabId
  label: string
  description: string
  icon: LucideIcon
}

export const SETTINGS_NAV_ITEMS: SettingsNavItem[] = [
  {
    id: "system",
    label: "系统设置",
    icon: Wrench,
    description: "管理全局偏好设置",
  },
  {
    id: "workers",
    label: "Worker 配置",
    icon: Wrench,
    description: "配置任务执行器",
  },
  {
    id: "providers",
    label: "Provider 配置",
    icon: Key,
    description: "配置 AI 模型 API",
  },
  {
    id: "models",
    label: "大模型配置",
    icon: Bot,
    description: "配置模型参数和限制",
  },
]
