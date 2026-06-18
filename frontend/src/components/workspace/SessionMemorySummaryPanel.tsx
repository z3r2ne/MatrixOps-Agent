import React, { useCallback, useEffect, useState } from "react"
import { Brain, Loader2, Sparkles } from "lucide-react"

import { api, type SessionMemoryAnalysis } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { ScrollArea } from "@/components/ui/scroll-area"
import { cn } from "@/lib/utils"
import { toast } from "sonner"

interface SessionMemorySummaryPanelProps {
  sessionId?: string
  memoryAnalysis?: SessionMemoryAnalysis | null
  onMemoryAnalysisUpdated?: (analysis: SessionMemoryAnalysis) => void
  className?: string
}

export function SessionMemorySummaryPanel({
  sessionId,
  memoryAnalysis,
  onMemoryAnalysisUpdated,
  className,
}: SessionMemorySummaryPanelProps) {
  const [analysis, setAnalysis] = useState<SessionMemoryAnalysis | null>(memoryAnalysis ?? null)
  const [analyzing, setAnalyzing] = useState(false)

  useEffect(() => {
    setAnalysis(memoryAnalysis ?? null)
  }, [memoryAnalysis, sessionId])

  const handleAnalyze = useCallback(async () => {
    if (!sessionId) {
      toast.error("当前没有可分析的会话")
      return
    }
    setAnalyzing(true)
    try {
      const result = await api.analyzeSessionMemory(sessionId)
      setAnalysis(result)
      onMemoryAnalysisUpdated?.(result)
      toast.success("记忆分析已完成")
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "记忆分析失败")
    } finally {
      setAnalyzing(false)
    }
  }, [onMemoryAnalysisUpdated, sessionId])

  const updatedLabel = analysis?.updatedAt
    ? new Date(analysis.updatedAt).toLocaleString()
    : null

  return (
    <div className={cn("flex h-full min-h-0 flex-col bg-background", className)}>
      <div className="border-b px-3 py-2.5">
        <div className="flex items-center justify-between gap-2">
          <div className="min-w-0">
            <div className="flex items-center gap-1.5 text-sm font-medium">
              <Brain className="h-4 w-4 shrink-0 text-primary" />
              <span>记忆总结</span>
            </div>
            <p className="mt-1 text-[11px] leading-4 text-muted-foreground">
              基于当前会话记忆生成关键词与简短总结（不超过 150 字）
            </p>
          </div>
          <Button
            type="button"
            size="sm"
            className="h-7 shrink-0 gap-1 px-2 text-xs"
            onClick={() => void handleAnalyze()}
            disabled={!sessionId || analyzing}
          >
            {analyzing ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Sparkles className="h-3.5 w-3.5" />}
            记忆分析
          </Button>
        </div>
      </div>

      <ScrollArea className="flex-1">
        <div className="space-y-4 p-3">
          {!sessionId ? (
            <div className="rounded-md border border-dashed px-3 py-8 text-center text-xs text-muted-foreground">
              选择任务后可查看记忆总结
            </div>
          ) : !analysis ? (
            <div className="rounded-md border border-dashed px-3 py-8 text-center text-xs text-muted-foreground">
              尚未生成记忆总结，点击「记忆分析」开始
            </div>
          ) : (
            <>
              <section className="space-y-2">
                <div className="text-xs font-medium text-muted-foreground">关键词</div>
                <div className="flex flex-wrap gap-1.5">
                  {analysis.keywords.map((keyword) => (
                    <Badge key={keyword} variant="secondary" className="text-[11px] font-normal">
                      {keyword}
                    </Badge>
                  ))}
                </div>
              </section>

              <section className="space-y-2">
                <div className="text-xs font-medium text-muted-foreground">总结</div>
                <div className="rounded-md border bg-muted/20 px-3 py-2.5 text-sm leading-6 text-foreground/90">
                  {analysis.summary}
                </div>
              </section>

              {updatedLabel ? (
                <div className="text-[11px] text-muted-foreground">更新于 {updatedLabel}</div>
              ) : null}
            </>
          )}
        </div>
      </ScrollArea>
    </div>
  )
}
