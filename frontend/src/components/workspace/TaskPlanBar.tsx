import React, { useState } from "react"
import { Check, ChevronDown, ChevronUp, Circle, Loader2, X } from "lucide-react"
import { cn } from "@/lib/utils"
import type { TaskPlanDocument, TaskPlanItem, TaskPlanItemStatus } from "@/lib/api"
import { getTaskPlanSummary, hasRunningPlanStep } from "@/lib/taskPlan"

export interface TaskPlanBarProps {
  plan?: TaskPlanDocument | null
  taskTitle: string
  isRunning?: boolean
}

function PlanStepStatus({ status }: { status?: TaskPlanItemStatus }) {
  switch (status) {
    case "completed":
      return <Check className="h-3.5 w-3.5 shrink-0 text-emerald-600" aria-hidden />
    case "failed":
      return <X className="h-3.5 w-3.5 shrink-0 text-destructive" aria-hidden />
    case "running":
      return <Loader2 className="h-3.5 w-3.5 shrink-0 animate-spin text-primary" aria-hidden />
    default:
      return <Circle className="h-3.5 w-3.5 shrink-0 text-muted-foreground/70" aria-hidden />
  }
}

function PlanStepRow({ item, index }: { item: TaskPlanItem; index: number }) {
  const isRunning = item.status === "running"
  const isCompleted = item.status === "completed"
  const isFailed = item.status === "failed"

  return (
    <div
      className={cn(
        "flex items-start gap-2 rounded-md px-2 py-1.5",
        isRunning && "bg-primary/8",
        isFailed && "bg-destructive/8",
      )}
    >
      <span className="mt-0.5 w-4 shrink-0 text-center text-[10px] font-medium text-muted-foreground">
        {index + 1}
      </span>
      <PlanStepStatus status={item.status} />
      <div className="min-w-0 flex-1">
        <div
          className={cn(
            "text-xs font-medium leading-5",
            isCompleted && "text-muted-foreground line-through",
            isFailed && "text-destructive",
          )}
        >
          {item.title}
        </div>
        {item.detail ? (
          <div className="mt-0.5 whitespace-pre-wrap text-[11px] leading-5 text-muted-foreground">
            {item.detail}
          </div>
        ) : null}
      </div>
    </div>
  )
}

export const TaskPlanBar: React.FC<TaskPlanBarProps> = ({
  plan,
  taskTitle,
  isRunning = false,
}) => {
  const [expanded, setExpanded] = useState(false)
  const summary = getTaskPlanSummary(plan, taskTitle)
  const showRunningIndicator = isRunning && (hasRunningPlanStep(plan) || !plan)
  const hasExpandablePlan = Boolean(plan && (plan.plan.length > 0 || plan.goal || plan.request))

  return (
    <div className="relative">
      {expanded && hasExpandablePlan ? (
        <div className="absolute bottom-full left-0 right-0 z-20 mb-1 overflow-hidden rounded-md border border-border/70 bg-background shadow-md">
          <div className="max-h-64 overflow-y-auto p-2 space-y-2">
            {plan?.goal ? (
              <div className="rounded-md bg-muted/30 px-2.5 py-2">
                <div className="text-[10px] font-medium uppercase tracking-wide text-muted-foreground">
                  目标
                </div>
                <div className="mt-1 text-xs leading-5 text-foreground">{plan.goal}</div>
              </div>
            ) : null}
            {plan?.request ? (
              <div className="rounded-md bg-muted/20 px-2.5 py-2">
                <div className="text-[10px] font-medium uppercase tracking-wide text-muted-foreground">
                  原始请求
                </div>
                <div className="mt-1 text-xs leading-5 text-muted-foreground">{plan.request}</div>
              </div>
            ) : null}
            {plan?.plan.length ? (
              <div className="space-y-1">
                <div className="px-2 text-[10px] font-medium uppercase tracking-wide text-muted-foreground">
                  执行步骤
                </div>
                {plan.plan.map((item, index) => (
                  <PlanStepRow key={`${index}-${item.title}`} item={item} index={index} />
                ))}
              </div>
            ) : null}
          </div>
        </div>
      ) : null}

      <button
        type="button"
        className={cn(
          "flex w-full items-center gap-2 rounded-md border border-border/60 bg-muted/30 px-2.5 py-1.5 text-left transition-colors",
          hasExpandablePlan ? "hover:bg-muted/50" : "cursor-default",
        )}
        onClick={() => {
          if (hasExpandablePlan) {
            setExpanded((value) => !value)
          }
        }}
        aria-expanded={expanded}
        aria-label={expanded ? "收起任务计划" : "展开任务计划"}
      >
        {showRunningIndicator ? (
          <Loader2 className="h-3.5 w-3.5 shrink-0 animate-spin text-primary" aria-hidden />
        ) : (
          <Circle className="h-3.5 w-3.5 shrink-0 text-muted-foreground/70" aria-hidden />
        )}
        <span className="min-w-0 flex-1 truncate text-xs text-foreground">{summary}</span>
        {hasExpandablePlan ? (
          expanded ? (
            <ChevronDown className="h-3.5 w-3.5 shrink-0 text-muted-foreground" aria-hidden />
          ) : (
            <ChevronUp className="h-3.5 w-3.5 shrink-0 text-muted-foreground" aria-hidden />
          )
        ) : null}
      </button>
    </div>
  )
}
