import { useEffect, useState, useCallback } from "react"
import {
  api,
  type WorkspaceResponse,
  type Task,
  type TestScenario,
  type TestResult,
} from "@/lib/api"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { ScrollArea } from "@/components/ui/scroll-area"
import { Skeleton } from "@/components/ui/skeleton"
import { Badge } from "@/components/ui/badge"
import {
  Plus,
  ArrowLeft,
  Play,
  Loader2,
  CheckCircle2,
  XCircle,
  FlaskConical,
  AlertCircle,
} from "lucide-react"
import { toast } from "sonner"
import { cn } from "@/lib/utils"

interface TestWorkspaceViewProps {
  workspace: WorkspaceResponse
}

const STATUS_LABEL_MAP: Record<string, string> = {
  passed: "通过",
  failed: "失败",
  partial: "部分通过",
  error: "错误",
  running: "运行中",
}

const STATUS_ICON_CLASS_MAP: Record<string, string> = {
  passed: "text-emerald-600",
  failed: "text-red-600",
  partial: "text-amber-600",
  error: "text-red-600",
  running: "text-blue-600",
}

function getStatusIcon(status: string) {
  const className = STATUS_ICON_CLASS_MAP[status] || "text-muted-foreground"
  if (status === "passed") {
    return <CheckCircle2 className={cn("h-6 w-6", className)} />
  }
  if (status === "partial") {
    return <AlertCircle className={cn("h-6 w-6", className)} />
  }
  return <XCircle className={cn("h-6 w-6", className)} />
}

export function TestWorkspaceView({ workspace }: TestWorkspaceViewProps) {
  const [tasks, setTasks] = useState<Task[]>([])
  const [scenarios, setScenarios] = useState<TestScenario[]>([])
  const [isLoadingTasks, setIsLoadingTasks] = useState(false)
  const [isLoadingScenarios, setIsLoadingScenarios] = useState(false)
  const [showTestPanel, setShowTestPanel] = useState(false)
  const [activeTab, setActiveTab] = useState("scenarios")
  const [selectedScenarioId, setSelectedScenarioId] = useState<string | null>(null)
  const [isRunningTest, setIsRunningTest] = useState(false)
  const [testResult, setTestResult] = useState<TestResult | null>(null)

  const loadTasks = useCallback(async () => {
    if (!workspace.id) return
    setIsLoadingTasks(true)
    try {
      const data = await api.getWorkspaceTasks(workspace.id)
      setTasks(Array.isArray(data) ? data : [])
    } catch (error) {
      console.error("Failed to load tasks:", error)
      toast.error("加载任务列表失败")
    } finally {
      setIsLoadingTasks(false)
    }
  }, [workspace.id])

  const loadScenarios = useCallback(async () => {
    setIsLoadingScenarios(true)
    try {
      const data = await api.getTestScenarios()
      setScenarios(Array.isArray(data) ? data : [])
    } catch (error) {
      console.error("Failed to load test scenarios:", error)
      toast.error("加载测试场景失败")
    } finally {
      setIsLoadingScenarios(false)
    }
  }, [])

  useEffect(() => {
    loadTasks()
    loadScenarios()
  }, [loadTasks, loadScenarios])

  const handleRunTest = useCallback(
    async (scenarioId: string) => {
      if (!workspace.id) return
      setIsRunningTest(true)
      setTestResult(null)
      try {
        const result = await api.runTestScenario(workspace.id, scenarioId)
        setTestResult(result)
        toast.success(
          result.status === "passed" ? "测试通过" : "测试未通过"
        )
      } catch (error) {
        console.error("Failed to run test scenario:", error)
        toast.error(error instanceof Error ? error.message : "运行测试失败")
      } finally {
        setIsRunningTest(false)
      }
    },
    [workspace.id]
  )

  const selectedScenario = scenarios.find((s) => s.id === selectedScenarioId)

  return (
    <div className="flex h-full bg-background">
      {/* LEFT SIDEBAR */}
      <div className="flex w-80 flex-col border-r">
        {/* Green Header */}
        <div className="flex items-center gap-2 bg-emerald-600 px-4 py-3 text-white">
          <FlaskConical className="h-5 w-5 shrink-0" />
          <span className="truncate text-sm font-semibold">
            {workspace.name}
          </span>
        </div>

        {/* Toolbar */}
        <div className="flex items-center justify-between border-b px-3 py-2">
          <span className="text-xs font-medium text-muted-foreground">
            任务列表
          </span>
          <Button
            variant="ghost"
            size="icon"
            className={cn(
              "h-7 w-7",
              showTestPanel && "bg-accent text-accent-foreground"
            )}
            onClick={() => setShowTestPanel((v) => !v)}
            title={showTestPanel ? "关闭测试面板" : "打开测试面板"}
          >
            <Plus className="h-4 w-4" />
          </Button>
        </div>

        {/* Task List */}
        <ScrollArea className="flex-1">
          <div className="p-2">
            {isLoadingTasks ? (
              <div className="space-y-2">
                <Skeleton className="h-8 w-full" />
                <Skeleton className="h-8 w-full" />
                <Skeleton className="h-8 w-full" />
              </div>
            ) : tasks.length === 0 ? (
              <div className="py-8 text-center text-xs text-muted-foreground">
                暂无任务
              </div>
            ) : (
              <div className="space-y-1">
                {tasks.map((task) => (
                  <div
                    key={task.id}
                    className="rounded-md px-2 py-1.5 text-sm text-foreground hover:bg-accent hover:text-accent-foreground"
                  >
                    {task.name?.trim() ||
                      task.content?.trim() ||
                      `任务 #${task.id}`}
                  </div>
                ))}
              </div>
            )}
          </div>
        </ScrollArea>
      </div>

      {/* RIGHT PANEL */}
      <div className="flex min-w-0 flex-1 flex-col">
        {showTestPanel ? (
          <div className="flex h-full flex-col p-4">
            <Tabs
              value={activeTab}
              onValueChange={setActiveTab}
              className="flex h-full flex-col"
            >
              <TabsList className="w-fit">
                <TabsTrigger value="scenarios">场景测试</TabsTrigger>
                <TabsTrigger value="prompts">提示词测试</TabsTrigger>
              </TabsList>

              <TabsContent
                value="scenarios"
                className="mt-4 flex-1 overflow-auto"
              >
                {isRunningTest ? (
                  <div className="flex h-full flex-col items-center justify-center gap-3 text-muted-foreground">
                    <Loader2 className="h-8 w-8 animate-spin text-primary" />
                    <p className="text-sm">测试中，请稍候...</p>
                  </div>
                ) : testResult && selectedScenarioId ? (
                  <div className="mx-auto max-w-2xl space-y-4">
                    <div className="flex items-center gap-2">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => {
                          setSelectedScenarioId(null)
                          setTestResult(null)
                        }}
                      >
                        <ArrowLeft className="mr-1 h-4 w-4" />
                        返回
                      </Button>
                    </div>
                    <div className="flex items-center gap-3">
                      {getStatusIcon(testResult.status)}
                      <div>
                        <h3 className="text-lg font-semibold">
                          {selectedScenario?.name}
                        </h3>
                        <Badge
                          variant={
                            testResult.status === "passed"
                              ? "default"
                              : "destructive"
                          }
                          className={
                            testResult.status === "passed"
                              ? "bg-emerald-600 hover:bg-emerald-600"
                              : undefined
                          }
                        >
                          {STATUS_LABEL_MAP[testResult.status] ||
                            testResult.status}
                        </Badge>
                      </div>
                    </div>
                    <Card>
                      <CardHeader>
                        <CardTitle className="text-sm">验证结果</CardTitle>
                      </CardHeader>
                      <CardContent>
                        <pre className="whitespace-pre-wrap break-all text-xs text-muted-foreground">
                          {testResult.verificationOutput}
                        </pre>
                      </CardContent>
                    </Card>
                    {testResult.error ? (
                      <Card className="border-red-200">
                        <CardHeader>
                          <CardTitle className="text-sm text-red-600">
                            错误信息
                          </CardTitle>
                        </CardHeader>
                        <CardContent>
                          <pre className="whitespace-pre-wrap break-all text-xs text-red-600">
                            {testResult.error}
                          </pre>
                        </CardContent>
                      </Card>
                    ) : null}
                  </div>
                ) : selectedScenarioId && selectedScenario ? (
                  <div className="mx-auto max-w-2xl space-y-4">
                    <div className="flex items-center gap-2">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => setSelectedScenarioId(null)}
                      >
                        <ArrowLeft className="mr-1 h-4 w-4" />
                        返回
                      </Button>
                    </div>
                    <Card>
                      <CardHeader>
                        <CardTitle>{selectedScenario.name}</CardTitle>
                        <CardDescription>
                          {selectedScenario.description}
                        </CardDescription>
                      </CardHeader>
                      <CardContent>
                        <Button
                          onClick={() => handleRunTest(selectedScenario.id)}
                          disabled={isRunningTest}
                        >
                          <Play className="mr-2 h-4 w-4" />
                          开始测试
                        </Button>
                      </CardContent>
                    </Card>
                  </div>
                ) : (
                  <div className="space-y-4">
                    <h3 className="text-sm font-medium text-muted-foreground">
                      选择测试场景
                    </h3>
                    {isLoadingScenarios ? (
                      <div className="grid gap-4 md:grid-cols-3">
                        <Skeleton className="h-32 w-full" />
                        <Skeleton className="h-32 w-full" />
                        <Skeleton className="h-32 w-full" />
                      </div>
                    ) : (
                      <div className="grid gap-4 md:grid-cols-3">
                        {scenarios.map((scenario) => (
                          <Card
                            key={scenario.id}
                            className="cursor-pointer transition-shadow hover:shadow-md"
                            onClick={() => {
                              setSelectedScenarioId(scenario.id)
                              setTestResult(null)
                            }}
                          >
                            <CardHeader className="pb-2">
                              <CardTitle className="text-base">
                                {scenario.name}
                              </CardTitle>
                            </CardHeader>
                            <CardContent>
                              <p className="text-xs text-muted-foreground">
                                {scenario.description}
                              </p>
                            </CardContent>
                          </Card>
                        ))}
                      </div>
                    )}
                  </div>
                )}
              </TabsContent>

              <TabsContent
                value="prompts"
                className="mt-4 flex-1 overflow-auto"
              >
                <div className="flex h-full flex-col items-center justify-center text-muted-foreground">
                  <p className="text-sm">提示词测试功能即将上线</p>
                </div>
              </TabsContent>
            </Tabs>
          </div>
        ) : (
          <div className="flex h-full flex-col items-center justify-center text-muted-foreground">
            <FlaskConical className="mb-3 h-10 w-10 opacity-20" />
            <p className="text-sm">点击左侧 + 按钮打开测试面板</p>
          </div>
        )}
      </div>
    </div>
  )
}
