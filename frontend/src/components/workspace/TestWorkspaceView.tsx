import React, { useCallback, useEffect, useMemo, useRef, useState } from "react"
import {
  AlertCircle,
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  FlaskConical,
  Loader2,
  MinusCircle,
  Play,
  RefreshCw,
  Square,
  XCircle,
} from "lucide-react"
import { toast } from "sonner"

import {
  api,
  type SemregRunReport,
  type SemregScenarioInfo,
  type SemregScenarioResult,
  type WorkspaceResponse,
} from "@/lib/api"
import { useTaskMessages } from "@/hooks/useGlobalWebSocket"
import { ChatV2MessageBlock } from "@/components/workspace/chat/ChatV2MessageBlock"
import { Button } from "@/components/ui/button"
import { Card, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { ScrollArea } from "@/components/ui/scroll-area"
import { cn } from "@/lib/utils"

interface TestWorkspaceViewProps {
  workspace: WorkspaceResponse
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`
  const sec = Math.floor(ms / 1000)
  if (sec < 60) return `${sec}s`
  const min = Math.floor(sec / 60)
  return `${min}m ${sec % 60}s`
}

function statusIcon(status: string) {
  switch (status) {
    case "passed":
    case "completed":
      return <CheckCircle2 className="h-4 w-4 text-emerald-500" />
    case "failed":
      return <XCircle className="h-4 w-4 text-destructive" />
    case "skipped":
      return <MinusCircle className="h-4 w-4 text-muted-foreground" />
    case "running":
      return <Loader2 className="h-4 w-4 animate-spin text-primary" />
    default:
      return <AlertCircle className="h-4 w-4 text-amber-500" />
  }
}

function statusBadgeClass(status: string): string {
  switch (status) {
    case "passed":
      return "bg-emerald-500/10 text-emerald-700 dark:text-emerald-400"
    case "failed":
      return "bg-destructive/10 text-destructive"
    case "skipped":
      return "bg-muted text-muted-foreground"
    case "running":
      return "bg-primary/10 text-primary"
    default:
      return "bg-amber-500/10 text-amber-700 dark:text-amber-400"
  }
}

const ScenarioRow: React.FC<{
  scenario: SemregScenarioInfo
  result?: SemregScenarioResult
  expanded: boolean
  isRunning: boolean
  onToggle: () => void
  onRun: () => void
}> = ({ scenario, result, expanded, isRunning, onToggle, onRun }) => {
  const status = result?.status ?? "pending"
  const metrics = result?.metrics ?? []

  return (
    <div className="min-w-0 max-w-full overflow-hidden rounded-lg border bg-card">
      <div className="flex items-start gap-2 px-3 py-2.5">
        <button
          type="button"
          className="flex min-w-0 flex-1 items-start gap-3 text-left hover:opacity-90"
          onClick={onToggle}
        >
          <div className="mt-0.5 shrink-0">{statusIcon(status)}</div>
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-2">
              <span className="text-sm font-medium">{scenario.name}</span>
              <span className="rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground">
                {scenario.kind}
              </span>
              <span className={cn("rounded px-1.5 py-0.5 text-[10px] font-medium", statusBadgeClass(status))}>
                {status}
              </span>
              {result?.durationMs ? (
                <span className="text-[11px] text-muted-foreground">{formatDuration(result.durationMs)}</span>
              ) : null}
            </div>
            <p className="mt-1 line-clamp-2 text-xs text-muted-foreground">{scenario.description}</p>
          </div>
          <div className="shrink-0 pt-0.5 text-muted-foreground">
            {expanded ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
          </div>
        </button>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="h-8 w-8 shrink-0"
          disabled={isRunning}
          onClick={onRun}
          title="运行此用例"
        >
          <Play className="h-4 w-4" />
        </Button>
      </div>

      {expanded ? (
        <div className="min-w-0 max-w-full space-y-3 overflow-hidden border-t px-3 py-3 text-xs">
          <div className="break-all text-muted-foreground">
            <span className="font-medium text-foreground">ID：</span>
            {scenario.id}
            {scenario.requiresLlm ? " · 需要真实 LLM" : " · Mock / 无 LLM"}
            {scenario.hasBaseline ? " · 有 baseline" : ""}
          </div>

          {result?.errors && result.errors.length > 0 ? (
            <div className="max-w-full overflow-x-auto rounded-md border border-destructive/30 bg-destructive/5 p-2 text-destructive">
              {result.errors.map((err, index) => (
                <div key={`${index}-${err.slice(0, 32)}`} className="break-all whitespace-pre-wrap">
                  {err}
                </div>
              ))}
            </div>
          ) : null}

          {metrics.length > 0 ? (
            <div className="min-w-0 max-w-full">
              <div className="mb-1.5 font-medium text-foreground">Benchmark 对比</div>
              <div className="max-w-full overflow-x-auto rounded-md border">
                <table className="w-full min-w-[480px] text-left">
                  <thead className="bg-muted/40 text-muted-foreground">
                    <tr>
                      <th className="px-2 py-1.5 font-medium">指标</th>
                      <th className="px-2 py-1.5 font-medium">实际</th>
                      <th className="px-2 py-1.5 font-medium">基线</th>
                      <th className="px-2 py-1.5 font-medium">结果</th>
                    </tr>
                  </thead>
                  <tbody>
                    {metrics.map((metric) => (
                      <tr key={metric.name} className="border-t">
                        <td className="px-2 py-1.5 font-mono">{metric.name}</td>
                        <td className="px-2 py-1.5">{metric.actual}</td>
                        <td className="px-2 py-1.5">{metric.baseline}</td>
                        <td className="px-2 py-1.5">
                          <span className={metric.passed ? "text-emerald-600" : "text-destructive"}>
                            {metric.passed ? "通过" : "失败"}
                          </span>
                          {metric.detail ? (
                            <div className="mt-0.5 text-[10px] text-muted-foreground">{metric.detail}</div>
                          ) : null}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          ) : null}

          {result?.toolCalls && result.toolCalls.length > 0 ? (
            <div>
              <div className="mb-1.5 font-medium text-foreground">
                工具调用 ({result.toolCalls.length})
              </div>
              <div className="max-h-40 space-y-1 overflow-y-auto rounded-md border bg-muted/20 p-2 font-mono text-[11px]">
                {result.toolCalls.map((call, index) => (
                  <div key={`${call.tool}-${index}`}>
                    {call.tool} [{call.status ?? "unknown"}]
                    {call.outputChars != null ? ` · ${call.outputChars} chars` : ""}
                  </div>
                ))}
              </div>
            </div>
          ) : null}

          {result?.details && Object.keys(result.details).length > 0 ? (
            <details className="min-w-0 max-w-full overflow-hidden rounded-md border bg-muted/10 p-2">
              <summary className="cursor-pointer font-medium text-foreground">原始详情</summary>
              <pre className="mt-2 max-h-48 max-w-full overflow-auto whitespace-pre-wrap break-all text-[10px]">
                {JSON.stringify(result.details, null, 2)}
              </pre>
            </details>
          ) : null}
        </div>
      ) : null}
    </div>
  )
}

function SemregLiveOutputPanel({
  taskId,
  scenarioName,
  tier,
}: {
  taskId: number | null
  scenarioName?: string
  tier?: string
}) {
  const { messagesV2, isWorking } = useTaskMessages(taskId)
  const bottomRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" })
  }, [messagesV2.length, isWorking])

  if (!taskId) {
    return (
      <div className="flex h-full flex-col items-center justify-center px-6 text-center text-muted-foreground">
        <FlaskConical className="mb-3 h-10 w-10 opacity-20" />
        <p className="text-sm">运行 L1/L2 用例时，此处将显示实时 AI 输出</p>
        <p className="mt-1 text-xs">L0 在隔离环境运行，无 WebSocket 流式输出</p>
      </div>
    )
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="border-b px-4 py-3">
        <div className="flex items-center gap-2">
          <h3 className="text-sm font-semibold">实时 AI 输出</h3>
          {isWorking ? (
            <span className="inline-flex items-center gap-1 text-xs text-primary">
              <Loader2 className="h-3 w-3 animate-spin" />
              运行中
            </span>
          ) : null}
        </div>
        {scenarioName ? (
          <p className="mt-1 truncate text-xs text-muted-foreground">
            {tier ? `${tier} · ` : ""}
            {scenarioName} · 任务 #{taskId}
          </p>
        ) : (
          <p className="mt-1 text-xs text-muted-foreground">任务 #{taskId}</p>
        )}
      </div>
      <ScrollArea className="flex-1">
        <div className="px-3 py-4">
          {messagesV2.length === 0 ? (
            <div className="py-12 text-center text-sm text-muted-foreground">
              {isWorking ? "等待 AI 响应…" : "暂无消息"}
            </div>
          ) : (
            messagesV2.map((msg, index) => (
              <ChatV2MessageBlock
                key={msg.info.id}
                msg={msg}
                isLastRetryableUserMessage={false}
                isHighlighted={false}
                readOnly
                canViewPrompt={false}
                onViewMemory={() => {}}
                isLastMessage={index === messagesV2.length - 1}
              />
            ))
          )}
          <div ref={bottomRef} />
        </div>
      </ScrollArea>
    </div>
  )
}

export function TestWorkspaceView({ workspace }: TestWorkspaceViewProps) {
  const [scenarios, setScenarios] = useState<SemregScenarioInfo[]>([])
  const [projectId, setProjectId] = useState("")
  const [fixturePath, setFixturePath] = useState("")
  const [bootstrapLoading, setBootstrapLoading] = useState(true)
  const [bootstrapError, setBootstrapError] = useState("")
  const [l1l2Ready, setL1l2Ready] = useState(false)
  const [loading, setLoading] = useState(true)
  const [runReport, setRunReport] = useState<SemregRunReport | null>(null)
  const [expandedIds, setExpandedIds] = useState<Record<string, boolean>>({})
  const [polling, setPolling] = useState(false)

  const loadBase = useCallback(async () => {
    setLoading(true)
    try {
      const scenarioResp = await api.getSemregScenarios()
      setScenarios(scenarioResp.scenarios ?? [])
    } catch (error) {
      console.error("Failed to load semreg scenarios:", error)
      toast.error("加载测试场景失败")
    } finally {
      setLoading(false)
    }
  }, [])

  const bootstrapWorkspace = useCallback(async () => {
    if (!workspace.id) return
    setBootstrapLoading(true)
    setBootstrapError("")
    try {
      const result = await api.bootstrapSemregWorkspace(workspace.id)
      setProjectId(result.projectId)
      setFixturePath(result.workDir)
      const status = await api.getSemregStatus({
        workspaceId: workspace.id,
        projectId: result.projectId,
        workDir: result.workDir,
      })
      setL1l2Ready(status.l1l2Ready)
    } catch (error) {
      console.error("Failed to bootstrap test workspace:", error)
      const message = error instanceof Error ? error.message : "准备内置测试项目失败"
      setBootstrapError(message)
      setL1l2Ready(false)
      toast.error(message)
    } finally {
      setBootstrapLoading(false)
    }
  }, [workspace.id])

  useEffect(() => {
    void loadBase()
  }, [loadBase])

  useEffect(() => {
    void bootstrapWorkspace()
  }, [bootstrapWorkspace])

  const resultByScenarioId = useMemo(() => {
    const map = new Map<string, SemregScenarioResult>()
    for (const result of runReport?.results ?? []) {
      map.set(result.scenarioId, result)
    }
    return map
  }, [runReport])

  const groupedScenarios = useMemo(() => {
    const groups: Record<string, SemregScenarioInfo[]> = { L0: [], L1: [], L2: [] }
    for (const scenario of scenarios) {
      const tier = scenario.tier in groups ? scenario.tier : "L0"
      groups[tier].push(scenario)
    }
    return groups
  }, [scenarios])

  const pollRun = useCallback(async (runId: string) => {
    setPolling(true)
    try {
      while (true) {
        const report = await api.getSemregRun(runId)
        setRunReport(report)
        if (report.status !== "running") {
          break
        }
        await new Promise((resolve) => setTimeout(resolve, 1000))
      }
    } catch (error) {
      console.error("Failed to poll semreg run:", error)
      toast.error("获取测试结果失败")
    } finally {
      setPolling(false)
    }
  }, [])

  const startRun = async (tiers: string[], scenarioIds?: string[]) => {
    try {
      const report = await api.startSemregRun({
        tiers,
        scenarioIds,
        workspaceId: workspace.id,
        projectId: projectId || undefined,
      })
      setRunReport(report)
      setExpandedIds({})
      void pollRun(report.id)
    } catch (error) {
      console.error("Failed to start semreg run:", error)
      toast.error(error instanceof Error ? error.message : "启动测试失败")
    }
  }

  const handleCancel = async () => {
    if (!runReport?.id) return
    try {
      await api.cancelSemregRun(runReport.id)
      toast.info("已请求取消测试")
    } catch (error) {
      console.error("Failed to cancel semreg run:", error)
      toast.error("取消失败")
    }
  }

  const runningResult = useMemo(
    () => runReport?.results.find((result) => result.status === "running"),
    [runReport],
  )

  const activeTaskId = useMemo(() => {
    if (!runningResult) return null
    if (runningResult.activeTaskId) return runningResult.activeTaskId
    const details = runningResult.details
    if (details && typeof details.taskId === "number") return details.taskId
    return null
  }, [runningResult])

  const summary = runReport?.summary
  const isRunning = runReport?.status === "running" || polling

  return (
    <div className="flex h-full min-h-0 min-w-0 overflow-hidden flex-col bg-background lg:flex-row">
      <div className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden border-r">
        <div className="flex items-center gap-2 bg-emerald-600 px-4 py-3 text-white">
          <FlaskConical className="h-5 w-5 shrink-0" />
          <div className="min-w-0">
            <div className="truncate text-sm font-semibold">{workspace.name}</div>
            <div className="text-[11px] text-emerald-100">语义测试 · L0 / L1 / L2</div>
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-2 border-b px-4 py-3">
          <Button variant="outline" size="sm" onClick={() => void loadBase()} disabled={loading || isRunning}>
            {loading ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <RefreshCw className="mr-2 h-4 w-4" />}
            刷新
          </Button>
          <Button size="sm" onClick={() => void startRun(["L0"])} disabled={isRunning}>
            <Play className="mr-2 h-4 w-4" />
            L0
          </Button>
          <Button
            size="sm"
            variant="secondary"
            onClick={() => void startRun(["L1", "L2"])}
            disabled={isRunning || bootstrapLoading || !l1l2Ready}
          >
            <Play className="mr-2 h-4 w-4" />
            L1/L2
          </Button>
          <Button size="sm" onClick={() => void startRun(["L0", "L1", "L2"])} disabled={isRunning}>
            <Play className="mr-2 h-4 w-4" />
            全部
          </Button>
          {isRunning ? (
            <Button size="sm" variant="outline" onClick={() => void handleCancel()}>
              <Square className="mr-2 h-4 w-4" />
              取消
            </Button>
          ) : null}
        </div>

        <div className="grid gap-3 border-b px-4 py-3 md:grid-cols-4">
          <Card className="shadow-none">
            <CardHeader className="pb-1 pt-3">
              <CardDescription>总计</CardDescription>
              <CardTitle className="text-xl">{summary?.total ?? scenarios.length}</CardTitle>
            </CardHeader>
          </Card>
          <Card className="shadow-none">
            <CardHeader className="pb-1 pt-3">
              <CardDescription>通过</CardDescription>
              <CardTitle className="text-xl text-emerald-600">{summary?.passed ?? 0}</CardTitle>
            </CardHeader>
          </Card>
          <Card className="shadow-none">
            <CardHeader className="pb-1 pt-3">
              <CardDescription>失败</CardDescription>
              <CardTitle className="text-xl text-destructive">{summary?.failed ?? 0}</CardTitle>
            </CardHeader>
          </Card>
          <Card className="shadow-none">
            <CardHeader className="pb-1 pt-3">
              <CardDescription>耗时</CardDescription>
              <CardTitle className="text-xl">{summary ? formatDuration(summary.durationMs) : "—"}</CardTitle>
            </CardHeader>
          </Card>
        </div>

        <div className="border-b px-4 py-3">
          <div className="mb-1 text-xs font-medium text-muted-foreground">内置测试项目</div>
          {bootstrapLoading ? (
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
              正在准备内置测试项目…
            </div>
          ) : bootstrapError ? (
            <div className="space-y-2">
              <p className="text-xs text-destructive">{bootstrapError}</p>
              <Button type="button" variant="outline" size="sm" onClick={() => void bootstrapWorkspace()}>
                重试
              </Button>
            </div>
          ) : (
            <div className="space-y-1 text-xs text-muted-foreground">
              <p>
                <span className="font-medium text-foreground">内置测试项目</span>
                {projectId ? ` · ID ${projectId}` : ""}
              </p>
              <p className="truncate">fixture 路径：{fixturePath || "—"}</p>
              <p className="text-emerald-600">
                {l1l2Ready
                  ? "L1/L2 已就绪；每次跑测会将 embed 代码释放到临时目录执行"
                  : "L0 可直接运行"}
              </p>
            </div>
          )}
        </div>

        <ScrollArea className="min-h-0 min-w-0 flex-1">
          <div className="min-w-0 space-y-6 p-4">
            {(["L0", "L1", "L2"] as const).map((tier) => {
              const items = groupedScenarios[tier]
              if (!items.length) return null
              return (
                <div key={tier} className="space-y-2">
                  <div className="flex items-center gap-2">
                    <h2 className="text-sm font-semibold">{tier}</h2>
                    <span className="text-xs text-muted-foreground">{items.length} 个用例</span>
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      className="ml-auto h-7 text-xs"
                      disabled={isRunning || bootstrapLoading || (tier !== "L0" && !l1l2Ready)}
                      onClick={() => void startRun([tier])}
                    >
                      运行 {tier}
                    </Button>
                  </div>
                  <div className="space-y-2">
                    {items.map((scenario) => (
                      <ScenarioRow
                        key={scenario.id}
                        scenario={scenario}
                        result={resultByScenarioId.get(scenario.id)}
                        expanded={!!expandedIds[scenario.id]}
                        isRunning={isRunning}
                        onToggle={() =>
                          setExpandedIds((prev) => ({ ...prev, [scenario.id]: !prev[scenario.id] }))
                        }
                        onRun={() => void startRun([scenario.tier], [scenario.id])}
                      />
                    ))}
                  </div>
                </div>
              )
            })}
          </div>
        </ScrollArea>
      </div>

      <div className="flex h-[min(42vh,480px)] min-h-[280px] min-w-0 flex-col overflow-hidden border-t lg:h-full lg:min-h-0 lg:w-[min(44%,520px)] lg:shrink-0 lg:border-l lg:border-t-0">
        <SemregLiveOutputPanel
          taskId={activeTaskId}
          scenarioName={runningResult?.name}
          tier={runningResult?.tier}
        />
      </div>
    </div>
  )
}
