import type { TaskPlanDocument, TaskPlanItem, TaskPlanItemStatus } from "@/lib/api"

const PLAN_STATUSES: TaskPlanItemStatus[] = ["pending", "running", "completed", "failed"]

function normalizePlanItemStatus(value: unknown): TaskPlanItemStatus | undefined {
  if (typeof value !== "string") return undefined
  const normalized = value.trim().toLowerCase()
  return PLAN_STATUSES.includes(normalized as TaskPlanItemStatus)
    ? (normalized as TaskPlanItemStatus)
    : undefined
}

function normalizePlanItem(value: unknown): TaskPlanItem | null {
  if (!value || typeof value !== "object") return null
  const raw = value as Record<string, unknown>
  const title = typeof raw.title === "string" ? raw.title.trim() : ""
  const detail = typeof raw.detail === "string" ? raw.detail.trim() : ""
  if (!title) return null
  return {
    title,
    detail,
    status: normalizePlanItemStatus(raw.status),
  }
}

export function normalizeTaskPlanPayload(value: unknown): TaskPlanDocument | null {
  if (!value || typeof value !== "object") return null
  const raw = value as Record<string, unknown>
  const request = typeof raw.request === "string" ? raw.request.trim() : ""
  const goal = typeof raw.goal === "string" ? raw.goal.trim() : ""
  const items = Array.isArray(raw.plan)
    ? raw.plan.map(normalizePlanItem).filter((item): item is TaskPlanItem => item != null)
    : []
  if (!request && !goal && items.length === 0) return null
  return { request, goal, plan: items }
}

export function getTaskPlanSummary(plan: TaskPlanDocument | null | undefined, taskTitle: string): string {
  const fallback = taskTitle.trim() || "任务计划"
  if (!plan) return fallback

  const running = plan.plan.find((item) => item.status === "running")
  if (running?.title) return running.title

  const pending = plan.plan.find((item) => !item.status || item.status === "pending")
  if (pending?.title) return pending.title

  if (plan.goal.trim()) return plan.goal.trim()
  if (plan.plan[0]?.title) return plan.plan[0].title
  return fallback
}

export function hasRunningPlanStep(plan: TaskPlanDocument | null | undefined): boolean {
  return Boolean(plan?.plan.some((item) => item.status === "running"))
}
