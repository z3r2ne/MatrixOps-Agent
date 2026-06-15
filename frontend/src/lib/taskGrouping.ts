import type { TaskListGroupMode } from "@/lib/api"

export const DEFAULT_TASK_LIST_GROUP_MODE_CONFIG_KEY = "default_task_list_group_mode"
export const DEFAULT_TASK_LIST_GROUP_MODE: TaskListGroupMode = "project"

export const TASK_GROUP_MODE_OPTIONS: Array<{ value: TaskListGroupMode; label: string; searchText: string }> = [
  { value: "none", label: "不分组", searchText: "none 不分组" },
  { value: "date", label: "按日期", searchText: "date 按日期" },
  { value: "project", label: "按项目", searchText: "project 按项目" },
]

export function normalizeTaskListGroupMode(value?: string | null): TaskListGroupMode {
  if (value === "none" || value === "date" || value === "project") {
    return value
  }
  return DEFAULT_TASK_LIST_GROUP_MODE
}

export function getTaskListGroupModeLabel(value?: string | null): string {
  const mode = normalizeTaskListGroupMode(value)
  return TASK_GROUP_MODE_OPTIONS.find((item) => item.value === mode)?.label || "按项目"
}
