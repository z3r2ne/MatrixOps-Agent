import type { Task } from "@/lib/api"

import type { AgentVisualState } from "../types"
import { truncateScreenPreview } from "./coordinates"

export function buildAgentSnapshots(
  tasks: Task[],
  rootTaskId: number | null,
  screenTextByTaskId: Map<number, string>,
  runningTaskIds: Set<number>,
  workingFrame: 0 | 1,
): AgentVisualState[] {
  return tasks.map((task, index) => {
    const raw = screenTextByTaskId.get(task.id) ?? "等待 Agent 输出…"

    return {
      taskId: task.id,
      task,
      // 工位坐标由 OfficeScene 按当前视口尺寸计算
      desk: {
        id: `desk-${index}`,
        gx: 0,
        gy: 0,
        scale: 1,
        depth: 10,
        x: 0,
        y: 0,
      },
      screenText: truncateScreenPreview(raw),
      isRoot: task.id === rootTaskId,
      isRunning: runningTaskIds.has(task.id),
      workingFrame,
    }
  })
}
