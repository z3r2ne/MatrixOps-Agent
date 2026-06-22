import React, { useEffect, useState } from "react"
import { useNavigate } from "react-router-dom"
import { Plus, Folder, FolderOpen, ArrowRight, Trash2, Settings, Layers } from "lucide-react"
import { motion } from "framer-motion"
import { api, MemoryLibrary, Project, ToolInfo, Workspace } from "@/lib/api"
import { useWorkbench } from "@/components/workbench/WorkbenchProvider"
import { MemoryLibrarySelector } from "@/components/projects/MemoryLibrarySelector"
import { ProjectToolPermissionPanel } from "@/components/projects/ProjectToolPermissionPanel"
import { Button } from "@/components/ui/button"
import { Card, CardHeader, CardTitle, CardDescription, CardFooter, CardContent } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { ScrollArea } from "@/components/ui/scroll-area"
import { Badge } from "@/components/ui/badge"
import { Combobox } from "@/components/ui/combobox"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { useConfirmDialog } from "@/components/ConfirmDialogProvider"
import { toast } from "sonner"
import { useElectronSettingsShellClose } from "@/components/electron/electron-main-contexts"
import {
  DEFAULT_TASK_LIST_GROUP_MODE,
  TASK_GROUP_MODE_OPTIONS,
  getTaskListGroupModeLabel,
  normalizeTaskListGroupMode,
} from "@/lib/taskGrouping"
import {
  DEFAULT_PROJECT_TOOL_PERMISSIONS_CONFIG_KEY,
  cloneProjectToolPermissions,
  parseProjectToolPermissions,
  serializeProjectToolPermissions,
} from "@/lib/projectToolPermissions"

export function WorkspacesPage() {
  const navigate = useNavigate()
  const { openWorkspaceTab } = useWorkbench()
  const { confirm } = useConfirmDialog()
  const closeSettingsShell = useElectronSettingsShellClose()
  const [workspaces, setWorkspaces] = useState<Workspace[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [newWorkspaceName, setNewWorkspaceName] = useState("")
  const [newWorkspaceType, setNewWorkspaceType] = useState("code")

  // 编辑工作区对话框状态
  const [editWorkspace, setEditWorkspace] = useState<Workspace | null>(null)
  const [editName, setEditName] = useState("")
  const [editType, setEditType] = useState("code")
  const [editGroupMode, setEditGroupMode] = useState(DEFAULT_TASK_LIST_GROUP_MODE)
  const [editProjects, setEditProjects] = useState<Project[]>([])
  const [newProjectName, setNewProjectName] = useState("")
  const [newProjectPath, setNewProjectPath] = useState("")
  const [defaultProjectToolPermissions, setDefaultProjectToolPermissions] = useState<Record<string, string>>({})
  const [newProjectToolPermissions, setNewProjectToolPermissions] = useState<Record<string, string>>({})
  const [newProjectYoloMode, setNewProjectYoloMode] = useState(false)
  const [newProjectMemoryLibraryIds, setNewProjectMemoryLibraryIds] = useState<number[]>([])
  const [isAddingProject, setIsAddingProject] = useState(false)
  
  // 选择现有项目对话框状态
  const [isSelectProjectOpen, setIsSelectProjectOpen] = useState(false)
  const [availableProjects, setAvailableProjects] = useState<Project[]>([])
  const [loadingAvailableProjects, setLoadingAvailableProjects] = useState(false)
  const [addProjectMode, setAddProjectMode] = useState<"create" | "select">("select")

  // 项目列表对话框状态
  const [projectsWorkspace, setProjectsWorkspace] = useState<Workspace | null>(null)
  const [projectsList, setProjectsList] = useState<Project[]>([])
  const [loadingProjects, setLoadingProjects] = useState(false)
  const [workspaceOpenTarget, setWorkspaceOpenTarget] = useState<Workspace | null>(null)

  // Git 初始化对话框状态
  const [gitInitProject, setGitInitProject] = useState<Project | null>(null)
  const [isInitializingGit, setIsInitializingGit] = useState(false)
  const [toolInfos, setToolInfos] = useState<ToolInfo[]>([])
  const [memoryLibraries, setMemoryLibraries] = useState<MemoryLibrary[]>([])

  const loadWorkspaces = async () => {
    try {
      setIsLoading(true)
      const data = await api.getWorkspaces()
      // 确保 data 是数组
      setWorkspaces(Array.isArray(data) ? data : [])
    } catch (error) {
      console.error("Failed to load workspaces", error)
      // 出错时确保 workspaces 保持为空数组
      setWorkspaces([])
    } finally {
      setIsLoading(false)
    }
  }

  useEffect(() => {
    loadWorkspaces()
  }, [])

  const handleOpenWorkspace = (workspace: Workspace) => {
    void window.electronAPI?.setCurrentWorkspaceWindow?.({
      id: workspace.id,
      name: workspace.name,
    })
    openWorkspaceTab({ id: workspace.id, name: workspace.name })
    if (closeSettingsShell) {
      closeSettingsShell()
    } else {
      navigate("/workbench")
    }
  }

  const requestOpenWorkspace = (workspace: Workspace) => {
    setWorkspaceOpenTarget(workspace)
  }

  const handleOpenWorkspaceInNewWindow = async () => {
    if (!workspaceOpenTarget) return
    try {
      const result = await window.electronAPI?.openWorkspaceWindow({
        id: workspaceOpenTarget.id,
        name: workspaceOpenTarget.name,
      })
      if (!result?.ok) {
        toast.error(result?.error || "新窗口打开失败")
        return
      }
      if (closeSettingsShell) {
        closeSettingsShell()
      }
      setWorkspaceOpenTarget(null)
    } catch (error) {
      console.error("Failed to open workspace in new window", error)
      toast.error("新窗口打开失败")
    }
  }

  useEffect(() => {
    const loadDependencies = async () => {
      try {
        const [toolsData, memoryLibraryData, defaultPermissionsConfig] = await Promise.all([
          api.getTools(),
          api.getMemoryLibraries({ isRag: false }),
          api.getConfig(DEFAULT_PROJECT_TOOL_PERMISSIONS_CONFIG_KEY, { skipErrorNotification: true }).catch(() => null),
        ])
        setToolInfos(toolsData)
        setMemoryLibraries(memoryLibraryData)
        const nextDefaults = parseProjectToolPermissions(defaultPermissionsConfig?.value)
        setDefaultProjectToolPermissions(nextDefaults)
        setNewProjectToolPermissions(nextDefaults)
      } catch (error) {
        console.error("Failed to load workspace dependencies", error)
        setToolInfos([])
        setMemoryLibraries([])
        setDefaultProjectToolPermissions({})
      }
    }
    void loadDependencies()
  }, [])

  const handleCreate = async () => {
    if (!newWorkspaceName.trim()) return

    try {
      await api.createWorkspace({ name: newWorkspaceName.trim(), type: newWorkspaceType })
      setNewWorkspaceName("")
      setNewWorkspaceType("code")
      setIsCreateOpen(false)
      loadWorkspaces()
    } catch (error) {
      console.error("Failed to create workspace", error)
    }
  }

  const handleDelete = async (e: React.MouseEvent, id: number) => {
    e.stopPropagation()
    const confirmed = await confirm({
      title: "删除工作区",
      description: "确定要删除这个工作区吗？",
      confirmLabel: "删除",
      tone: "destructive",
    })
    if (!confirmed) return

    try {
      await api.deleteWorkspace(id)
      loadWorkspaces()
    } catch (error) {
      console.error("Failed to delete workspace", error)
    }
  }

  // 打开编辑对话框
  const openEditDialog = async (e: React.MouseEvent, ws: Workspace) => {
    e.stopPropagation()
    setEditWorkspace(ws)
    setEditName(ws.name)
    setEditType(ws.type || "code")
    setEditGroupMode(normalizeTaskListGroupMode(ws.groupMode))
    try {
      const fullWs = await api.getWorkspace(ws.id)
      setEditGroupMode(normalizeTaskListGroupMode(fullWs.groupMode))
      setEditProjects(fullWs.projects || [])
    } catch (error) {
      console.error("Failed to load workspace projects", error)
      setEditProjects([])
    }
  }

  // 保存工作区编辑
  const handleSaveEdit = async () => {
    if (!editWorkspace || !editName.trim()) return
    try {
      await api.updateWorkspace(editWorkspace.id, {
        name: editName.trim(),
        type: editType,
        groupMode: editGroupMode,
      })
      toast.success("工作区已更新")
      setEditWorkspace(null)
      loadWorkspaces()
    } catch (error) {
      toast.error("更新失败")
    }
  }

  // 加载可用项目（未关联或可重新关联的项目）
  const loadAvailableProjects = async () => {
    if (!editWorkspace) return
    setLoadingAvailableProjects(true)
    try {
      const allProjects = await api.getAllProjects()
      // 过滤掉已经在当前工作区的项目
      const currentProjectIds = new Set(editProjects.map(p => p.id))
      const available = allProjects.filter(p => !currentProjectIds.has(p.id))
      setAvailableProjects(available)
    } catch (error) {
      console.error("Failed to load available projects", error)
      toast.error("加载可用项目失败")
    } finally {
      setLoadingAvailableProjects(false)
    }
  }

  // 打开选择项目对话框
  const openSelectProjectDialog = () => {
    loadAvailableProjects()
    setIsSelectProjectOpen(true)
  }

  // 添加现有项目到工作区
  const handleAddExistingProject = async (projectId: number) => {
    if (!editWorkspace) return
    try {
      // 添加项目到工作区
      await api.addProjectToWorkspace(editWorkspace.id, projectId)
      // 重新加载项目列表
      const fullWs = await api.getWorkspace(editWorkspace.id)
      setEditProjects(fullWs.projects || [])
      toast.success("项目已添加到工作区")
      setIsSelectProjectOpen(false)
    } catch (error: any) {
      toast.error(error.message || "添加项目失败")
    }
  }

  // 创建新项目并添加到工作区
  const handleAddProject = async () => {
    if (!editWorkspace || !newProjectName.trim() || !newProjectPath.trim()) return
    setIsAddingProject(true)
    try {
      const newProject = await api.createProject(editWorkspace.id, {
        name: newProjectName.trim(),
        path: newProjectPath.trim(),
        toolPermissions: serializeProjectToolPermissions(newProjectToolPermissions),
        memoryLibraryIds: newProjectMemoryLibraryIds,
        yoloMode: newProjectYoloMode,
      })
      setEditProjects([...editProjects, newProject])
      setNewProjectName("")
      setNewProjectPath("")
      setNewProjectToolPermissions(cloneProjectToolPermissions(defaultProjectToolPermissions))
      setNewProjectMemoryLibraryIds([])
      setNewProjectYoloMode(false)
      setAddProjectMode("select")
      toast.success("项目已创建并添加")
      
      // 检查项目是否为 Git 仓库
      try {
        const { isGitRepo } = await api.checkGitRepo(newProject.id)
        if (!isGitRepo) {
          setGitInitProject(newProject)
        }
      } catch (error) {
        console.error("检查 Git 仓库失败:", error)
      }
    } catch (error: any) {
      toast.error(error.message || "添加项目失败")
    } finally {
      setIsAddingProject(false)
    }
  }

  // 初始化 Git 仓库
  const handleInitGit = async () => {
    if (!gitInitProject) return
    setIsInitializingGit(true)
    try {
      await api.initGitRepo(gitInitProject.id, "Initial commit")
      toast.success(`${gitInitProject.name} 已初始化为 Git 仓库`)
      setGitInitProject(null)
    } catch (error: any) {
      toast.error(error.message || "初始化 Git 仓库失败")
    } finally {
      setIsInitializingGit(false)
    }
  }

  // 删除项目
  const handleDeleteProject = async (projectId: number) => {
    const confirmed = await confirm({
      title: "删除项目",
      description: "确定要删除这个项目吗？",
      confirmLabel: "删除",
      tone: "destructive",
    })
    if (!confirmed) return
    try {
      await api.deleteProject(projectId)
      setEditProjects(editProjects.filter(p => p.id !== projectId))
      toast.success("项目已删除")
    } catch (error) {
      toast.error("删除项目失败")
    }
  }

  // 打开项目列表对话框
  const openProjectsDialog = async (e: React.MouseEvent, ws: Workspace) => {
    e.stopPropagation()
    setProjectsWorkspace(ws)
    setLoadingProjects(true)
    try {
      const fullWs = await api.getWorkspace(ws.id)
      setProjectsList(fullWs.projects || [])
    } catch (error) {
      console.error("Failed to load projects", error)
      setProjectsList([])
    } finally {
      setLoadingProjects(false)
    }
  }

  return (
    <div className="flex-1 p-8 overflow-y-auto">
      <div className="max-w-6xl mx-auto space-y-8">
        <div className="flex items-center justify-end">
          <Dialog open={isCreateOpen} onOpenChange={setIsCreateOpen}>
            <DialogTrigger asChild>
              <Button>
                <Plus className="mr-2 h-4 w-4" /> 新建工作区
              </Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>新建工作区</DialogTitle>
                <DialogDescription>
                  创建一个新的工作区来组织您的项目和任务。
                </DialogDescription>
              </DialogHeader>
              <div className="grid gap-4 py-4">
                <div className="grid grid-cols-4 items-center gap-4">
                  <Label htmlFor="name" className="text-right">
                    名称
                  </Label>
                  <Input
                    id="name"
                    value={newWorkspaceName}
                    onChange={(e) => setNewWorkspaceName(e.target.value)}
                    className="col-span-3"
                  />
                </div>
                <div className="grid grid-cols-4 items-center gap-4">
                  <Label htmlFor="type" className="text-right">
                    类型
                  </Label>
                  <Select
                    value={newWorkspaceType}
                    onValueChange={(value) => setNewWorkspaceType(value)}
                  >
                    <SelectTrigger className="col-span-3">
                      <SelectValue placeholder="选择类型" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="code">代码</SelectItem>
                      <SelectItem value="test">测试</SelectItem>
                      <SelectItem value="claw">抓取</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </div>
              <DialogFooter>
                <Button variant="outline" onClick={() => setIsCreateOpen(false)}>取消</Button>
                <Button onClick={handleCreate}>创建</Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </div>

        {isLoading ? (
          <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
            {[1, 2, 3].map((i) => (
              <Skeleton key={i} className="h-32" />
            ))}
          </div>
        ) : workspaces.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-[400px] border border-dashed bg-muted/10">
            <div className="p-3 bg-muted mb-4">
              <Folder className="h-6 w-6 text-muted-foreground" />
            </div>
            <h3 className="text-base font-medium">没有工作区</h3>
            <p className="text-sm text-muted-foreground mb-4">创建一个工作区开始使用</p>
            <Button onClick={() => setIsCreateOpen(true)}>创建工作区</Button>
          </div>
        ) : (
          <div className="grid auto-rows-fr gap-3 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
            {workspaces.map((ws, index) => (
              <motion.div
                key={ws.id}
                className="h-full"
                initial={{ opacity: 0, y: 10 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.2, delay: index * 0.05 }}
              >
                <Card 
                  className="group relative flex h-full min-h-[120px] cursor-pointer flex-col transition-colors hover:bg-accent/50"
                  onClick={() => requestOpenWorkspace(ws)}
                >
                  <CardHeader className="flex-1 pb-1">
                    <div className="flex items-start gap-2 pr-8">
                      <div className="mt-0.5 bg-primary/10 p-2">
                        <Folder className="h-4 w-4 text-primary" />
                      </div>
                      <div className="min-w-0 flex-1">
                        <CardTitle className="min-h-[2.5rem] text-base leading-5">
                          <span className="line-clamp-2 break-all">{ws.name}</span>
                        </CardTitle>
                        <div className="mt-1 flex items-center gap-2">
                          <Badge variant="outline" className="text-[10px] h-5 px-1.5">
                            {ws.type === "code" ? "代码" : ws.type === "test" ? "测试" : ws.type === "claw" ? "抓取" : ws.type}
                          </Badge>
                          <span className="text-xs text-muted-foreground">
                            任务分组：{getTaskListGroupModeLabel(ws.groupMode)}
                          </span>
                        </div>
                      </div>
                    </div>
                  </CardHeader>
                  <CardFooter className="flex items-center justify-between pt-1 text-xs text-muted-foreground">
                    <div className="flex items-center gap-2">
                      <Button 
                        variant="ghost" 
                        size="sm"
                        className="h-7 px-2 text-xs"
                        onClick={(e) => openProjectsDialog(e, ws)}
                      >
                        <Layers className="h-3 w-3 mr-1" />
                        项目
                      </Button>
                      <Button 
                        variant="ghost" 
                        size="icon" 
                        className="h-7 w-7"
                        onClick={(e) => openEditDialog(e, ws)}
                      >
                        <Settings className="h-3.5 w-3.5" />
                      </Button>
                    </div>
                    <Button 
                      variant="ghost" 
                      size="icon" 
                      className="h-7 w-7 hover:text-destructive"
                      onClick={(e) => handleDelete(e, ws.id)}
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                  </CardFooter>
                  
                  <div className="absolute right-4 top-4">
                    <ArrowRight className="h-4 w-4 text-muted-foreground" />
                  </div>
                </Card>
              </motion.div>
            ))}
          </div>
        )}
      </div>

      {/* 编辑工作区对话框 */}
      <Dialog
        open={editWorkspace !== null}
        onOpenChange={(open) => {
          if (!open) {
            setEditWorkspace(null)
            setEditType("code")
            setNewProjectToolPermissions(cloneProjectToolPermissions(defaultProjectToolPermissions))
            setNewProjectMemoryLibraryIds([])
            setNewProjectYoloMode(false)
          }
        }}
      >
        <DialogContent className="flex max-h-[85vh] max-w-2xl flex-col overflow-hidden">
          <DialogHeader className="shrink-0">
            <DialogTitle>编辑工作区</DialogTitle>
            <DialogDescription>修改工作区设置和管理项目</DialogDescription>
          </DialogHeader>
          <div className="min-h-0 flex-1 space-y-4 overflow-y-auto py-4 pr-2">
            <div className="space-y-2">
              <Label>工作区名称</Label>
              <Input
                value={editName}
                onChange={(e) => setEditName(e.target.value)}
                placeholder="输入工作区名称"
              />
            </div>

            <div className="space-y-2">
              <Label>类型</Label>
              <Select value={editType} onValueChange={(value) => setEditType(value)}>
                <SelectTrigger className="w-full md:w-[280px]">
                  <SelectValue placeholder="选择类型" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="code">代码</SelectItem>
                  <SelectItem value="test">测试</SelectItem>
                  <SelectItem value="claw">抓取</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <Label htmlFor="workspace-group-mode">任务列表分组方式</Label>
              <Combobox
                id="workspace-group-mode"
                items={TASK_GROUP_MODE_OPTIONS}
                value={editGroupMode}
                onValueChange={(value) => setEditGroupMode(normalizeTaskListGroupMode(value))}
                placeholder="选择分组方式"
                searchPlaceholder="搜索分组方式"
                emptyText="未找到分组选项"
                className="w-full md:w-[280px]"
              />
              <p className="text-xs text-muted-foreground">
                进入该工作区时，会自动使用这里保存的任务列表分组方式。
              </p>
            </div>
            
            <div className="space-y-2">
              <Label className="flex items-center justify-between">
                <span>项目列表</span>
                <Badge variant="secondary">{editProjects.length} 个项目</Badge>
              </Label>
              <div className="max-h-[200px] overflow-y-auto border">
                {editProjects.length === 0 ? (
                  <div className="p-4 text-center text-sm text-muted-foreground">
                    暂无项目，请添加
                  </div>
                ) : (
                  <div className="divide-y">
                    {editProjects.map((project) => (
                      <div key={project.id} className="flex items-center justify-between p-3 hover:bg-muted/50">
                        <div className="space-y-0.5">
                          <div className="text-sm font-medium">{project.name}</div>
                          <div className="text-xs text-muted-foreground truncate max-w-[280px]">{project.path}</div>
                        </div>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-7 w-7 text-muted-foreground hover:text-destructive"
                          onClick={() => handleDeleteProject(project.id)}
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                        </Button>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </div>

            {/* 添加项目操作 */}
            <div className="space-y-2 border-t pt-4">
              <Label>添加项目</Label>
              <div className="flex gap-2">
                <Button
                  variant={addProjectMode === "select" ? "default" : "outline"}
                  size="sm"
                  onClick={() => {
                    setAddProjectMode("select")
                    setNewProjectName("")
                    setNewProjectPath("")
                    setNewProjectToolPermissions(cloneProjectToolPermissions(defaultProjectToolPermissions))
                    setNewProjectMemoryLibraryIds([])
                    setNewProjectYoloMode(false)
                  }}
                  className="flex-1"
                >
                  选择现有项目
                </Button>
                <Button
                  variant={addProjectMode === "create" ? "default" : "outline"}
                  size="sm"
                  onClick={() => {
                    setAddProjectMode("create")
                    setNewProjectToolPermissions(cloneProjectToolPermissions(defaultProjectToolPermissions))
                  }}
                  className="flex-1"
                >
                  创建新项目
                </Button>
              </div>
              
              {addProjectMode === "select" ? (
                <Button 
                  variant="outline" 
                  className="w-full"
                  onClick={openSelectProjectDialog}
                >
                  <Plus className="h-4 w-4 mr-2" />
                  从现有项目中选择
                </Button>
              ) : (
                <div className="space-y-2">
                  <Input
                    value={newProjectName}
                    onChange={(e) => setNewProjectName(e.target.value)}
                    placeholder="项目名称"
                  />
                  <Input
                    value={newProjectPath}
                    onChange={(e) => setNewProjectPath(e.target.value)}
                    placeholder="项目路径，如 /Users/xxx/projects/my-project"
                  />
                  <ProjectToolPermissionPanel
                    toolInfos={toolInfos}
                    permissions={newProjectToolPermissions}
                    onPermissionsChange={setNewProjectToolPermissions}
                    yoloMode={newProjectYoloMode}
                    onYoloModeChange={setNewProjectYoloMode}
                  />
                  <MemoryLibrarySelector
                    libraries={memoryLibraries}
                    selectedIds={newProjectMemoryLibraryIds}
                    onChange={setNewProjectMemoryLibraryIds}
                  />
                  <Button 
                    variant="outline" 
                    className="w-full"
                    onClick={handleAddProject}
                    disabled={isAddingProject || !newProjectName.trim() || !newProjectPath.trim()}
                  >
                    <Plus className="h-4 w-4 mr-2" />
                    创建并添加项目
                  </Button>
                </div>
              )}
            </div>
          </div>
          <DialogFooter className="shrink-0 border-t pt-4">
            <Button variant="outline" onClick={() => setEditWorkspace(null)}>取消</Button>
            <Button onClick={handleSaveEdit}>保存</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* 选择现有项目对话框 */}
      <Dialog open={isSelectProjectOpen} onOpenChange={setIsSelectProjectOpen}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>选择项目添加到工作区</DialogTitle>
            <DialogDescription>
              从现有项目中选择一个或多个添加到当前工作区
            </DialogDescription>
          </DialogHeader>
          <ScrollArea className="max-h-[400px] pr-4">
            {loadingAvailableProjects ? (
              <div className="space-y-2">
                {[1, 2, 3].map((i) => (
                  <Skeleton key={i} className="h-16 w-full" />
                ))}
              </div>
            ) : availableProjects.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-8 text-center">
                <FolderOpen className="h-12 w-12 text-muted-foreground mb-3" />
                <p className="text-sm text-muted-foreground">没有可用的项目</p>
                <p className="text-xs text-muted-foreground mt-1">
                  所有项目都已添加到工作区，或者您可以创建新项目
                </p>
              </div>
            ) : (
              <div className="space-y-2">
                {availableProjects.map((project) => (
                  <div
                    key={project.id}
                    className="flex items-center justify-between border p-3 transition-colors hover:bg-accent/50"
                  >
                    <div className="flex items-center gap-3 flex-1 min-w-0">
                      <div className="bg-primary/10 p-2">
                        <FolderOpen className="h-4 w-4 text-primary" />
                      </div>
                      <div className="flex-1 min-w-0">
                        <div className="font-medium text-sm">{project.name}</div>
                        <div className="text-xs text-muted-foreground truncate">
                          {project.path}
                        </div>
                        {project.workspaceId && project.workspaceId > 0 && (
                          <Badge variant="secondary" className="mt-1 text-xs">
                            已在其他工作区
                          </Badge>
                        )}
                      </div>
                    </div>
                    <Button
                      size="sm"
                      onClick={() => handleAddExistingProject(project.id)}
                    >
                      添加
                    </Button>
                  </div>
                ))}
              </div>
            )}
          </ScrollArea>
          <DialogFooter>
            <Button variant="outline" onClick={() => setIsSelectProjectOpen(false)}>
              关闭
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* 项目列表对话框 */}
      <Dialog open={projectsWorkspace !== null} onOpenChange={(open) => !open && setProjectsWorkspace(null)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <Layers className="h-4 w-4" />
              {projectsWorkspace?.name} - 项目列表
            </DialogTitle>
            <DialogDescription>查看该工作区包含的项目</DialogDescription>
          </DialogHeader>
          <ScrollArea className="max-h-[400px]">
            {loadingProjects ? (
              <div className="space-y-2 p-2">
                <Skeleton className="h-16" />
                <Skeleton className="h-16" />
              </div>
            ) : projectsList.length === 0 ? (
              <div className="p-8 text-center text-sm text-muted-foreground">
                暂无项目
              </div>
            ) : (
              <div className="space-y-2 p-2">
                {projectsList.map((project) => (
                  <Card
                    key={project.id}
                    className="transition-colors hover:bg-accent/30"
                  >
                    <CardHeader className="p-3">
                      <CardTitle className="flex items-center gap-2 text-sm">
                        <FolderOpen className="h-4 w-4 text-muted-foreground" />
                        {project.name}
                      </CardTitle>
                      <CardDescription className="text-xs truncate">{project.path}</CardDescription>
                    </CardHeader>
                  </Card>
                ))}
              </div>
            )}
          </ScrollArea>
          <DialogFooter>
            <Button 
              variant="outline" 
              className="w-full"
              onClick={() => {
                setProjectsWorkspace(null)
                if (projectsWorkspace) {
                  handleOpenWorkspace(projectsWorkspace)
                }
              }}
            >
              进入工作区（查看全部任务）
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={workspaceOpenTarget !== null} onOpenChange={(open) => !open && setWorkspaceOpenTarget(null)}>
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle>打开工作区</DialogTitle>
            <DialogDescription>
              {workspaceOpenTarget ? `选择如何打开“${workspaceOpenTarget.name}”` : "选择打开方式"}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter className="flex-col gap-2 sm:flex-col">
            <Button
              className="w-full"
              onClick={() => {
                if (workspaceOpenTarget) {
                  handleOpenWorkspace(workspaceOpenTarget)
                }
                setWorkspaceOpenTarget(null)
              }}
            >
              在当前窗口打开
            </Button>
            <Button variant="outline" className="w-full" onClick={() => void handleOpenWorkspaceInNewWindow()}>
              在新窗口打开
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Git 初始化确认对话框 */}
      <Dialog open={gitInitProject !== null} onOpenChange={(open) => !open && setGitInitProject(null)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>初始化 Git 仓库</DialogTitle>
            <DialogDescription>
              检测到项目 <span className="font-semibold text-foreground">{gitInitProject?.name}</span> 不是 Git 仓库
            </DialogDescription>
          </DialogHeader>
          <div className="py-4">
            <p className="text-sm text-muted-foreground">
              是否要初始化 Git 仓库并进行首次提交？
            </p>
            <p className="text-sm text-muted-foreground mt-2">
              初始化后将创建：
            </p>
            <ul className="text-sm text-muted-foreground mt-2 ml-4 list-disc space-y-1">
              <li>.git 目录</li>
              <li>.gitignore 文件（如果不存在）</li>
              <li>README.md 文件（如果不存在）</li>
              <li>首次提交（Initial commit）</li>
            </ul>
          </div>
          <DialogFooter>
            <Button 
              variant="outline" 
              onClick={() => setGitInitProject(null)}
              disabled={isInitializingGit}
            >
              跳过
            </Button>
            <Button 
              onClick={handleInitGit}
              disabled={isInitializingGit}
            >
              {isInitializingGit ? "初始化中..." : "初始化"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
