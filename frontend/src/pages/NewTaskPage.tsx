import { useCallback, useEffect, useMemo, useState } from "react"
import { useNavigate, useParams, useSearchParams } from "react-router-dom"
import { ArrowLeft, Bot, FolderOpen, Loader2, Sparkles, FolderPlus, GitBranch } from "lucide-react"
import { toast } from "sonner"
import { ChatInterfaceV2 } from "@/components/workspace/ChatInterfaceV2"
import { Button } from "@/components/ui/button"
import { Combobox, type ComboboxOption } from "@/components/ui/combobox"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { buildBranchOptions } from "@/lib/branches"
import { api, type BranchInfo, type MemoryLibrary, type Project, type TaskMemoryLibraryMode, type Worker } from "@/lib/api"
import { getApiErrorMessage } from "@/lib/httpClient"
import {
  TaskRagLibrarySelector,
  buildTaskRagLibraryPayload,
  validateTaskRagLibrarySelection,
} from "@/components/projects/TaskRagLibrarySelector"

const IMPORT_PROJECT_OPTION_VALUE = "__import_project__"

function buildWorkerOptions(workers: Worker[]): ComboboxOption[] {
  return workers.map((worker) => ({
    value: worker.name,
    label: worker.name,
    description: worker.description || worker.model || "",
    searchText: `${worker.name} ${worker.description || ""} ${worker.model || ""}`,
  }))
}

export function NewTaskPage() {
  const navigate = useNavigate()
  const { id: routeWorkspaceId, projectId: routeProjectId } = useParams<{ id?: string; projectId?: string }>()
  const [searchParams] = useSearchParams()

  const explicitWorkspaceId = routeWorkspaceId ? Number(routeWorkspaceId) : 0
  const projectId = routeProjectId ? Number(routeProjectId) : 0
  const [workspaceId, setWorkspaceId] = useState(explicitWorkspaceId)
  const [loading, setLoading] = useState(true)
  const [submitting, setSubmitting] = useState(false)
  const [projects, setProjects] = useState<Project[]>([])
  const [workers, setWorkers] = useState<Worker[]>([])
  const [selectedProjectId, setSelectedProjectId] = useState("0")
  const [selectedProjectIsGit, setSelectedProjectIsGit] = useState(false)
  const [branches, setBranches] = useState<BranchInfo[]>([])
  const [selectedBranch, setSelectedBranch] = useState("")
  const [useWorktree, setUseWorktree] = useState(false)
  const [newBranchName, setNewBranchName] = useState("")
  const [isCreateProjectDialogOpen, setIsCreateProjectDialogOpen] = useState(false)
  const [newProjectName, setNewProjectName] = useState("")
  const [newProjectPath, setNewProjectPath] = useState("")
  const [creatingProject, setCreatingProject] = useState(false)
  const [selectedWorkerName, setSelectedWorkerName] = useState("chat")
  const [memoryLibraries, setMemoryLibraries] = useState<MemoryLibrary[]>([])
  const [memoryLibraryMode, setMemoryLibraryMode] = useState<TaskMemoryLibraryMode>("none")
  const [memoryLibraryIds, setMemoryLibraryIds] = useState<number[]>([])

  const returnTo = searchParams.get("returnTo")
  const projectOptions = useMemo<ComboboxOption[]>(
    () => [
      {
        value: "0",
        label: "仅工作区",
        description: "不附加项目上下文",
        searchText: "workspace only",
      },
      ...projects.map((project) => ({
        value: String(project.id),
        label: project.name,
        description: project.path,
        searchText: `${project.name} ${project.path}`,
      })),
      {
        value: IMPORT_PROJECT_OPTION_VALUE,
        label: "导入项目",
        description: "创建并添加一个新项目",
        searchText: "import create project 导入项目",
      },
    ],
    [projects]
  )
  const workerOptions = useMemo(() => buildWorkerOptions(workers), [workers])
  const branchOptions = useMemo(() => buildBranchOptions(branches), [branches])

  const selectedProjectNumericId = Number(selectedProjectId) || 0

  const ensureWorktreeBranchName = useCallback((force = false) => {
    setNewBranchName((current) => {
      if (!force && current.trim()) return current
      return `matrixops/${Math.random().toString(36).slice(2, 10)}`
    })
  }, [])

  const loadBranches = useCallback(async (nextProjectId: number) => {
    if (!nextProjectId) {
      setBranches([])
      setSelectedBranch("")
      return
    }
    const branchList = await api.getBranches(nextProjectId)
    setBranches(branchList)
    const currentBranch = branchList.find((branch) => branch.isCurrent)
    if (currentBranch) {
      setSelectedBranch(currentBranch.name)
      return
    }
    setSelectedBranch(branchList[0]?.name || "")
  }, [])

  const refreshSelectedProjectGitState = useCallback(async (nextProjectId: number) => {
    if (!nextProjectId) {
      setSelectedProjectIsGit(false)
      setUseWorktree(false)
      setBranches([])
      setSelectedBranch("")
      return
    }
    const result = await api.checkGitRepo(nextProjectId)
    setSelectedProjectIsGit(Boolean(result.isGitRepo))
    if (!result.isGitRepo) {
      setUseWorktree(false)
      setBranches([])
      setSelectedBranch("")
      return
    }
    await loadBranches(nextProjectId)
  }, [loadBranches])

  useEffect(() => {
    const load = async () => {
      setLoading(true)
      try {
        let nextWorkspaceID = explicitWorkspaceId
        if (!nextWorkspaceID && projectId) {
          const workspaces = await api.getWorkspaces()
          nextWorkspaceID = workspaces.find((workspace) => workspace.projectIds.includes(projectId))?.id || 0
        }
        if (!nextWorkspaceID) {
          throw new Error("未找到任务所属工作区")
        }

        const [loadedWorkers, loadedProjects, loadedMemoryLibraries] = await Promise.all([
          api.getWorkers().catch(() => []),
          api.getProjects(nextWorkspaceID).catch(() => []),
          api.getRagLibraries().catch(() => []),
        ])
        setWorkspaceId(nextWorkspaceID)
        setProjects(loadedProjects)
        setWorkers(loadedWorkers)
        setMemoryLibraries(loadedMemoryLibraries)
        setMemoryLibraryMode("none")
        setMemoryLibraryIds([])
        setSelectedWorkerName(
          loadedWorkers.find((worker) => worker.name === "chat")?.name ||
            loadedWorkers[0]?.name ||
            "chat"
        )
        const nextProjectId = projectId > 0 ? String(projectId) : "0"
        setSelectedProjectId(nextProjectId)
        await refreshSelectedProjectGitState(Number(nextProjectId))
      } catch (error) {
        console.error("Failed to initialize new task page:", error)
        toast.error(error instanceof Error ? error.message : "加载新建任务页面失败")
      } finally {
        setLoading(false)
      }
    }

    void load()
  }, [explicitWorkspaceId, projectId, refreshSelectedProjectGitState])

  const handleBack = useCallback(() => {
    if (returnTo) {
      navigate(returnTo)
      return
    }
    if (workspaceId) {
      navigate(`/workspace/${workspaceId}`)
      return
    }
    navigate("/workspaces")
  }, [navigate, returnTo, workspaceId])

  const handleProjectChange = useCallback((value: string) => {
    if (value === IMPORT_PROJECT_OPTION_VALUE) {
      setIsCreateProjectDialogOpen(true)
      return
    }
    setSelectedProjectId(value)
    setUseWorktree(false)
    setNewBranchName("")
    void refreshSelectedProjectGitState(Number(value))
  }, [refreshSelectedProjectGitState])

  const handleCreateProject = useCallback(async () => {
    if (!workspaceId) {
      toast.error("未找到工作区")
      return
    }
    if (!newProjectName.trim() || !newProjectPath.trim()) {
      toast.error("请填写项目名称和路径")
      return
    }
    setCreatingProject(true)
    try {
      const project = await api.createProject(workspaceId, {
        name: newProjectName.trim(),
        path: newProjectPath.trim(),
      })
      setProjects((prev) => [...prev, project])
      setSelectedProjectId(String(project.id))
      await refreshSelectedProjectGitState(project.id)
      setIsCreateProjectDialogOpen(false)
      setNewProjectName("")
      setNewProjectPath("")
      toast.success("项目已创建并选中")
    } catch (error) {
      console.error("Failed to create project from new task page:", error)
      toast.error(error instanceof Error ? error.message : "导入项目失败")
    } finally {
      setCreatingProject(false)
    }
  }, [newProjectName, newProjectPath, refreshSelectedProjectGitState, workspaceId])

  const handleEnableWorktree = useCallback((checked: boolean) => {
    setUseWorktree(checked)
    if (checked) {
      ensureWorktreeBranchName(false)
    }
  }, [ensureWorktreeBranchName])

  const handleSubmit = useCallback(async (message: string) => {
    const trimmed = message.trim()
    if (!trimmed) {
      toast.error("请输入任务名称")
      return
    }
    if (!workspaceId) {
      toast.error("未找到工作区")
      return
    }
    if (selectedProjectNumericId > 0 && selectedProjectIsGit && !selectedBranch) {
      toast.error("请选择分支")
      return
    }
    if (useWorktree && !newBranchName.trim()) {
      toast.error("请输入新分支名")
      return
    }
    const memoryError = validateTaskRagLibrarySelection(memoryLibraryMode, memoryLibraryIds)
    if (memoryError) {
      toast.error(memoryError)
      return
    }

    setSubmitting(true)
    try {
      const newTask = await api.runTask(workspaceId, {
        name: trimmed,
        content: trimmed,
        projectId: Number(selectedProjectId) > 0 ? Number(selectedProjectId) : undefined,
        workerName: selectedWorkerName,
        branch: selectedProjectNumericId > 0 && selectedProjectIsGit && !useWorktree ? selectedBranch : undefined,
        newBranch: selectedProjectNumericId > 0 && selectedProjectIsGit && useWorktree ? newBranchName.trim() : undefined,
        baseBranch: selectedProjectNumericId > 0 && selectedProjectIsGit && useWorktree ? selectedBranch : undefined,
        ...buildTaskRagLibraryPayload(memoryLibraryMode, memoryLibraryIds),
      })
      toast.success("任务已创建并开始执行")
      navigate(`/workspace/${workspaceId}?taskId=${newTask.id}`)
    } catch (error) {
      console.error("Failed to create task:", error)
      toast.error(getApiErrorMessage(error, "创建任务失败"))
    } finally {
      setSubmitting(false)
    }
  }, [memoryLibraryIds, memoryLibraryMode, navigate, newBranchName, selectedBranch, selectedProjectId, selectedProjectIsGit, selectedProjectNumericId, selectedWorkerName, useWorktree, workspaceId])

  return (
    <div className="flex h-full min-h-0 flex-col bg-background">
      <div className="border-b px-6 py-4">
        <div className="mx-auto flex w-full max-w-6xl items-center justify-between gap-3">
          <Button type="button" variant="ghost" onClick={handleBack}>
            <ArrowLeft className="mr-2 h-4 w-4" />
            返回
          </Button>
        </div>
      </div>

      <div className="flex flex-1 items-center justify-center px-6 py-10">
        <div className="w-full max-w-5xl space-y-8">
          <div className="space-y-3 text-center">
            <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-2xl bg-primary/10 text-primary">
              <Sparkles className="h-6 w-6" />
            </div>
            <div className="space-y-1">
              <h1 className="text-2xl font-semibold tracking-tight">新建任务</h1>
              <p className="text-sm text-muted-foreground">任务现在只绑定工作区，不再绑定项目。</p>
            </div>
          </div>

          <div className="mx-auto w-full max-w-4xl space-y-6">
            <div className="rounded-3xl border bg-card/60 shadow-sm">
              <ChatInterfaceV2
                title=""
                messages={[]}
                readOnly={false}
                isRunning={submitting}
                isWorking={submitting}
                placeholder="请输入任务名称…"
                onSendMessage={(message) => {
                  void handleSubmit(message)
                }}
                headerExtra={null}
              />
            </div>

            <div className="grid min-h-[320px] gap-4 rounded-3xl border bg-card/50 p-4">
              <div className="grid gap-3 md:grid-cols-3">
                {workspaceId ? (
                  <div className="relative">
                    <FolderOpen className="pointer-events-none absolute left-3 top-1/2 z-10 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                    <Combobox
                      id="new-task-project"
                      items={projectOptions}
                      value={selectedProjectId}
                      onValueChange={handleProjectChange}
                      placeholder="项目上下文"
                      searchPlaceholder="搜索项目"
                      emptyText="未找到项目"
                      disabled={loading || submitting}
                      inputClassName="pl-9"
                      renderItem={(item) => {
                        if (item.value === IMPORT_PROJECT_OPTION_VALUE) {
                          return (
                            <div className="flex items-center gap-2 text-primary">
                              <FolderPlus className="h-4 w-4" />
                              <div className="min-w-0 flex-1">
                                <div className="truncate text-sm font-medium">{item.label}</div>
                                {item.description ? (
                                  <div className="truncate text-xs text-muted-foreground">{item.description}</div>
                                ) : null}
                              </div>
                            </div>
                          )
                        }
                        return (
                          <div className="min-w-0 flex-1">
                            <div className="truncate text-sm">{item.label}</div>
                            {item.description ? (
                              <div className="truncate text-xs text-muted-foreground">{item.description}</div>
                            ) : null}
                          </div>
                        )
                      }}
                    />
                  </div>
                ) : null}

                <div className="relative">
                  <Bot className="pointer-events-none absolute left-3 top-1/2 z-10 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                  <Combobox
                    id="new-task-worker"
                    items={workerOptions}
                    value={selectedWorkerName}
                    onValueChange={setSelectedWorkerName}
                    placeholder="Worker"
                    searchPlaceholder="搜索 Worker"
                    emptyText="未找到 Worker"
                    disabled={loading || submitting || workerOptions.length === 0}
                    inputClassName="pl-9"
                  />
                </div>

                <div className="relative">
                  <GitBranch className="pointer-events-none absolute left-3 top-1/2 z-10 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                  <Combobox
                    id="new-task-branch"
                    items={branchOptions}
                    value={selectedBranch}
                    onValueChange={setSelectedBranch}
                    placeholder={
                      selectedProjectNumericId === 0
                        ? "先选择项目"
                        : selectedProjectIsGit
                          ? (useWorktree ? "基准分支" : "分支")
                          : "当前项目不是 Git 项目"
                    }
                    searchPlaceholder="搜索分支"
                    emptyText={selectedProjectIsGit ? "未找到分支" : "当前项目不是 Git 项目"}
                    disabled={loading || submitting || !selectedProjectIsGit || branchOptions.length === 0}
                    inputClassName="pl-9"
                  />
                </div>

                <TaskRagLibrarySelector
                  libraries={memoryLibraries}
                  mode={memoryLibraryMode}
                  selectedIds={memoryLibraryIds}
                  onModeChange={setMemoryLibraryMode}
                  onSelectedIdsChange={setMemoryLibraryIds}
                  disabled={loading || submitting}
                />
              </div>

              <div className="space-y-4 md:col-span-2">
                <div className="min-h-[72px]">
                  <div className="grid gap-3 md:grid-cols-[180px_minmax(0,1fr)] md:items-center">
                    <div className="flex h-10 items-center justify-between rounded-md border border-border/60 bg-muted/30 px-3">
                      <div className="flex items-center gap-2 text-muted-foreground">
                        <GitBranch className="h-4 w-4" />
                        <span className="text-xs">worktree</span>
                      </div>
                      <div className="shrink-0">
                        <Switch
                          checked={useWorktree}
                          onCheckedChange={handleEnableWorktree}
                          disabled={loading || submitting || !selectedProjectIsGit}
                        />
                      </div>
                    </div>

                    <div className="relative">
                      <GitBranch className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                      <Input
                        value={newBranchName}
                        onChange={(e) => setNewBranchName(e.target.value)}
                        placeholder={selectedProjectIsGit ? "新分支，例如：feature/my-task" : "当前项目不是 Git 项目"}
                        disabled={loading || submitting || !selectedProjectIsGit || !useWorktree}
                        className={useWorktree && selectedProjectIsGit ? "pl-9 opacity-100" : "pl-9 opacity-40"}
                      />
                    </div>
                  </div>
                </div>

                <div className="flex items-center gap-2 rounded-2xl border bg-muted/20 px-4 py-3 text-sm text-muted-foreground">
                  {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
                  <span>{workspaceId ? `工作区 #${workspaceId}` : "正在解析工作区…"}</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <Dialog
        open={isCreateProjectDialogOpen}
        onOpenChange={(open) => {
          setIsCreateProjectDialogOpen(open)
          if (!open) {
            setNewProjectName("")
            setNewProjectPath("")
          }
        }}
      >
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>导入项目</DialogTitle>
            <DialogDescription>先创建一个项目并加入当前工作区，然后自动选中它作为任务上下文。</DialogDescription>
          </DialogHeader>
          <div className="grid gap-4 py-2">
            <div className="grid gap-2">
              <Label htmlFor="new-task-project-name">项目名称</Label>
              <Input
                id="new-task-project-name"
                value={newProjectName}
                onChange={(e) => setNewProjectName(e.target.value)}
                placeholder="my-project"
                disabled={creatingProject}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="new-task-project-path">项目路径</Label>
              <Input
                id="new-task-project-path"
                value={newProjectPath}
                onChange={(e) => setNewProjectPath(e.target.value)}
                placeholder="/path/to/project"
                disabled={creatingProject}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setIsCreateProjectDialogOpen(false)} disabled={creatingProject}>
              取消
            </Button>
            <Button onClick={() => void handleCreateProject()} disabled={creatingProject}>
              {creatingProject ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}
              创建并选择
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
