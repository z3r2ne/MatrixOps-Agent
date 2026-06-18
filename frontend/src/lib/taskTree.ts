import type { Task } from "@/lib/api"

/** 收集指定任务及其所有子孙任务（深度优先，父任务在前）。 */
export function collectTaskTree(tasks: Task[], rootTaskId: number): Task[] {
  const taskMap = new Map(tasks.map((task) => [task.id, task]))
  const childrenByParent = new Map<number, Task[]>()

  for (const task of tasks) {
    if (!task.parentTaskId) continue
    const siblings = childrenByParent.get(task.parentTaskId) ?? []
    siblings.push(task)
    childrenByParent.set(task.parentTaskId, siblings)
  }

  for (const siblings of childrenByParent.values()) {
    siblings.sort((a, b) => a.id - b.id)
  }

  const root = taskMap.get(rootTaskId)
  if (!root) return []

  const result: Task[] = []
  const walk = (task: Task) => {
    result.push(task)
    for (const child of childrenByParent.get(task.id) ?? []) {
      walk(child)
    }
  }
  walk(root)
  return result
}
