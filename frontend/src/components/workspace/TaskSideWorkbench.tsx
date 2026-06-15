"use client"

import { useEffect, useRef, useState } from "react"
import { BookMarked } from "lucide-react"
import { ScrollArea } from "@/components/ui/scroll-area"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import { LexicalMentionInput } from "@/components/workspace/LexicalMentionInput"
import type { Task } from "@/lib/api"

type WorkbenchTab = "memo"

interface TaskSideWorkbenchProps {
  task: Task | null
  memoValue: string
  onMemoChange: (value: string) => void
}

const EDGE_HIT_WIDTH = 28

export function TaskSideWorkbench({
  task,
  memoValue,
  onMemoChange,
}: TaskSideWorkbenchProps) {
  const [activeTab, setActiveTab] = useState<WorkbenchTab | null>("memo")
  const [pinnedOpen, setPinnedOpen] = useState(false)
  const [hoverOpen, setHoverOpen] = useState(false)
  const rootRef = useRef<HTMLDivElement | null>(null)
  const isOpen = pinnedOpen || hoverOpen

  useEffect(() => {
    if (!task) {
      setPinnedOpen(false)
      setHoverOpen(false)
      setActiveTab("memo")
      return
    }
    setActiveTab("memo")
  }, [task?.id])

  useEffect(() => {
    const handlePointerMove = (event: PointerEvent) => {
      if (pinnedOpen) return
      const viewportWidth = window.innerWidth || document.documentElement.clientWidth || 0
      const nearRightEdge = viewportWidth - event.clientX <= EDGE_HIT_WIDTH
      const insideWorkbench = !!rootRef.current?.contains(event.target as Node)
      setHoverOpen(nearRightEdge || insideWorkbench)
    }

    const handlePointerLeaveWindow = () => {
      if (!pinnedOpen) {
        setHoverOpen(false)
      }
    }

    window.addEventListener("pointermove", handlePointerMove)
    window.addEventListener("mouseout", handlePointerLeaveWindow)
    return () => {
      window.removeEventListener("pointermove", handlePointerMove)
      window.removeEventListener("mouseout", handlePointerLeaveWindow)
    }
  }, [pinnedOpen])

  return (
    <div
      ref={rootRef}
      className="pointer-events-none absolute inset-y-0 right-0 z-30 flex items-start justify-end pr-3 pt-4"
      onMouseEnter={() => setHoverOpen(true)}
      onMouseLeave={() => {
        if (!pinnedOpen) {
          setHoverOpen(false)
        }
      }}
    >
      <div
        className={cn(
          "pointer-events-auto flex h-[calc(100%-1rem)] items-start gap-3 transition-transform duration-200 ease-out",
          isOpen ? "translate-x-0" : "translate-x-[calc(100%-1.25rem)]",
        )}
      >
        <div className="flex h-full flex-col justify-start">
          <button
            type="button"
            className={cn(
              "mt-4 h-10 w-1.5 rounded-full border border-white/30 bg-white/80 shadow-[0_10px_30px_-20px_rgba(255,255,255,0.9)]",
              "transition-opacity hover:opacity-100",
              isOpen ? "opacity-70" : "opacity-95",
            )}
            onClick={() => {
              const nextOpen = !isOpen
              setPinnedOpen(nextOpen)
              setHoverOpen(nextOpen)
            }}
            aria-label={isOpen ? "折叠操作台" : "展开操作台"}
          />
        </div>

        <div className="flex h-full w-[min(24rem,34vw)] min-w-[20rem] flex-col gap-3">
          <section
            className={cn(
              "overflow-hidden border border-border/70 bg-background/92 shadow-[0_22px_70px_-34px_rgba(15,23,42,0.55)] backdrop-blur supports-[backdrop-filter]:bg-background/82",
              !task && "opacity-90",
            )}
          >
            <ScrollArea className="max-h-28">
              <div className="flex gap-2 px-3 py-3">
                <Button
                  type="button"
                  variant={activeTab === "memo" ? "default" : "outline"}
                  size="sm"
                  className="shrink-0"
                  onClick={() => setActiveTab("memo")}
                >
                  <BookMarked data-icon="inline-start" />
                  备忘录
                </Button>
              </div>
            </ScrollArea>
          </section>

          <section
            className={cn(
              "flex min-h-0 flex-1 flex-col overflow-hidden border border-border/70 bg-background/94 shadow-[0_22px_70px_-34px_rgba(15,23,42,0.55)] backdrop-blur supports-[backdrop-filter]:bg-background/84",
              !task && "opacity-80",
            )}
          >
            <div className="min-h-0 flex-1 px-4 py-4">
              {task && activeTab === "memo" ? (
                <div className="h-full min-h-0 border border-border/70 bg-background/80 p-2">
                  <LexicalMentionInput
                    value={memoValue}
                    onChange={onMemoChange}
                    placeholder="记录这个任务的上下文、待办、结论或后续动作..."
                    maxHeight="100%"
                    fill
                    className="h-full"
                  />
                </div>
              ) : (
                <div className="flex h-full items-center justify-center border border-dashed border-border/70 bg-muted/20 px-6 text-center text-sm text-muted-foreground">
                  当前没有可展示的工作台内容
                </div>
              )}
            </div>
          </section>
        </div>
      </div>
    </div>
  )
}
