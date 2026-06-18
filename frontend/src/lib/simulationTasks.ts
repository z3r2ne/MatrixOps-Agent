import type { Task, WithParts } from "@/lib/api"

function toTaskId(value: unknown): number | null {
  if (typeof value === "number" && Number.isFinite(value) && value > 0) {
    return value
  }
  if (typeof value === "string" && value.trim()) {
    const parsed = Number(value)
    if (Number.isFinite(parsed) && parsed > 0) return parsed
  }
  return null
}

/** 从对话消息里提取 run_worker_task 创建的子任务 ID */
export function extractSubtaskIdsFromMessages(messages: WithParts[]): number[] {
  const ids = new Set<number>()

  for (const message of messages) {
    for (const part of message.parts ?? []) {
      if (part.type !== "tool" && part.type !== "tool-delta") continue
      const tool = part.tool
      if (!tool || tool.tool !== "run_worker_task") continue

      const meta = (tool.state?.metadata ?? {}) as Record<string, unknown>
      const subtaskId = toTaskId(meta.subtaskTaskId)
      if (subtaskId) ids.add(subtaskId)
    }
  }

  return [...ids]
}

/** 根据工具 metadata 补全缺失的 parentTaskId */
export function patchSubtaskParentLinks(
  tasks: Task[],
  taskStates: Map<number, { messagesV2?: WithParts[] }>,
): Task[] {
  const parentBySubtask = new Map<number, number>()

  for (const task of tasks) {
    const messages = taskStates.get(task.id)?.messagesV2 ?? []
    for (const message of messages) {
      for (const part of message.parts ?? []) {
        if (part.type !== "tool" && part.type !== "tool-delta") continue
        const tool = part.tool
        if (!tool || tool.tool !== "run_worker_task") continue

        const meta = (tool.state?.metadata ?? {}) as Record<string, unknown>
        const subtaskId = toTaskId(meta.subtaskTaskId)
        const parentId = toTaskId(meta.subtaskParentTaskId) ?? task.id
        if (subtaskId) parentBySubtask.set(subtaskId, parentId)
      }
    }
  }

  if (parentBySubtask.size === 0) return tasks

  return tasks.map((task) => {
    const parentId = parentBySubtask.get(task.id)
    if (!parentId || task.parentTaskId) return task
    return { ...task, parentTaskId: parentId }
  })
}

export function dedupeTasksById(tasks: Task[]): Task[] {
  const byId = new Map<number, Task>()
  for (const task of tasks) {
    byId.set(task.id, task)
  }
  return [...byId.values()]
}

/** 选中子任务时，向上找到仿真树的根任务 */
export function resolveSimulationRootId(tasks: Task[], selectedTaskId: number): number {
  const selected = tasks.find((task) => task.id === selectedTaskId)
  if (!selected) return selectedTaskId

  let current = selected
  while (current.parentTaskId) {
    const parent = tasks.find((task) => task.id === current.parentTaskId)
    if (!parent) break
    current = parent
  }
  return current.id
}
