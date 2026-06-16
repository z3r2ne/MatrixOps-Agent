import React, { useEffect, useState } from "react"
import { Plus, FolderOpen, Trash2, Edit, Search } from "lucide-react"
import { motion } from "framer-motion"
import { api, MemoryLibrary, Project, ToolInfo } from "@/lib/api"
import { MemoryLibrarySelector } from "@/components/projects/MemoryLibrarySelector"
import { ProjectToolPermissionPanel } from "@/components/projects/ProjectToolPermissionPanel"
import { Button } from "@/components/ui/button"
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
import { Textarea } from "@/components/ui/textarea"
import { useConfirmDialog } from "@/components/ConfirmDialogProvider"
import { toast } from "sonner"
import {
  DEFAULT_PROJECT_TOOL_PERMISSIONS_CONFIG_KEY,
  cloneProjectToolPermissions,
  parseProjectToolPermissions,
  serializeProjectToolPermissions,
} from "@/lib/projectToolPermissions"

export function ProjectsPage() {
  const { confirm } = useConfirmDialog()
  const [projects, setProjects] = useState<Project[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [searchQuery, setSearchQuery] = useState("")

  // 创建项目对话框状态
  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [newProjectName, setNewProjectName] = useState("")
  const [newProjectPath, setNewProjectPath] = useState("")
  const [defaultProjectToolPermissions, setDefaultProjectToolPermissions] = useState<Record<string, string>>({})
  const [newProjectToolPermissions, setNewProjectToolPermissions] = useState<Record<string, string>>({})
  const [newProjectYoloMode, setNewProjectYoloMode] = useState(false)
  const [newProjectMemoryLibraryIds, setNewProjectMemoryLibraryIds] = useState<number[]>([])

  // 编辑项目对话框状态
  const [editProject, setEditProject] = useState<Project | null>(null)
  const [editName, setEditName] = useState("")
  const [editPath, setEditPath] = useState("")
  const [editPrompt, setEditPrompt] = useState("")
  const [toolInfos, setToolInfos] = useState<ToolInfo[]>([])
  const [memoryLibraries, setMemoryLibraries] = useState<MemoryLibrary[]>([])
  const [projectToolPermissions, setProjectToolPermissions] = useState<Record<string, string>>({})
  const [projectYoloMode, setProjectYoloMode] = useState(false)
  const [editProjectMemoryLibraryIds, setEditProjectMemoryLibraryIds] = useState<number[]>([])

  const loadProjects = async () => {
    try {
      setIsLoading(true)
      const data = await api.getAllProjects()
      setProjects(Array.isArray(data) ? data : [])
    } catch (error) {
      console.error("Failed to load projects", error)
      setProjects([])
      toast.error("加载项目列表失败")
    } finally {
      setIsLoading(false)
    }
  }

  useEffect(() => {
    loadProjects()
  }, [])

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
        console.error("Failed to load project dependencies", error)
        setToolInfos([])
        setMemoryLibraries([])
        setDefaultProjectToolPermissions({})
      }
    }
    void loadDependencies()
  }, [])

  const isProjectNameTaken = (name: string, excludeId?: number) => {
    const normalized = name.trim().toLowerCase()
    if (!normalized) return false
    return projects.some((project) => {
      if (excludeId != null && project.id === excludeId) return false
      return project.name.trim().toLowerCase() === normalized
    })
  }

  const handleCreate = async () => {
    if (!newProjectName.trim() || !newProjectPath.trim()) {
      toast.error("请填写项目名称和路径")
      return
    }
    if (isProjectNameTaken(newProjectName)) {
      toast.error("项目名称已存在")
      return
    }

    try {
      // 创建独立项目（不关联工作区）
      await api.createStandaloneProject({
        name: newProjectName.trim(),
        path: newProjectPath.trim(),
        toolPermissions: serializeProjectToolPermissions(newProjectToolPermissions),
        memoryLibraryIds: newProjectMemoryLibraryIds,
        yoloMode: newProjectYoloMode,
      })
      setNewProjectName("")
      setNewProjectPath("")
      setNewProjectToolPermissions(cloneProjectToolPermissions(defaultProjectToolPermissions))
      setNewProjectMemoryLibraryIds([])
      setNewProjectYoloMode(false)
      setIsCreateOpen(false)
      toast.success("项目已创建")
      loadProjects()
    } catch (error: any) {
      toast.error(error.message || "创建项目失败")
    }
  }

  const handleDelete = async (e: React.MouseEvent, id: number) => {
    e.stopPropagation()
    const confirmed = await confirm({
      title: "删除项目",
      description: "确定要删除这个项目吗？",
      confirmLabel: "删除",
      tone: "destructive",
    })
    if (!confirmed) return

    try {
      await api.deleteProject(id)
      toast.success("项目已删除")
      loadProjects()
    } catch (error) {
      toast.error("删除项目失败")
    }
  }

  const openEditDialog = async (e: React.MouseEvent, project: Project) => {
    e.stopPropagation()
    setEditProject(project)
    setEditName(project.name)
    setEditPath(project.path)
    setProjectToolPermissions(parseProjectToolPermissions(project.toolPermissions))
    setProjectYoloMode(Boolean(project.yoloMode))
    setEditProjectMemoryLibraryIds(project.memoryLibraryIds || [])
    try {
      const [projectData, promptData] = await Promise.all([
        api.getProject(project.id),
        api.getProjectPrompt(project.id),
      ])
      setEditProjectMemoryLibraryIds(projectData.memoryLibraryIds || [])
      setEditPrompt(promptData.prompt || "")
    } catch (error) {
      setEditPrompt("")
    }
  }

  const handleSaveEdit = async () => {
    if (!editProject || !editName.trim() || !editPath.trim()) return
    if (isProjectNameTaken(editName, editProject.id)) {
      toast.error("项目名称已存在")
      return
    }

    try {
      await api.updateProject(editProject.id, {
        name: editName.trim(),
        path: editPath.trim(),
        toolPermissions: serializeProjectToolPermissions(projectToolPermissions),
        memoryLibraryIds: editProjectMemoryLibraryIds,
        yoloMode: projectYoloMode,
      })
      if (editPrompt !== undefined) {
        await api.updateProjectPrompt(editProject.id, editPrompt)
      }
      toast.success("项目已更新")
      setEditProject(null)
      loadProjects()
    } catch (error) {
      toast.error("更新项目失败")
    }
  }

  const filteredProjects = projects.filter((project) =>
    project.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
    project.path.toLowerCase().includes(searchQuery.toLowerCase())
  )

  return (
    <div className="flex-1 p-8 overflow-y-auto">
      <div className="max-w-6xl mx-auto space-y-8">
        <div className="flex items-center justify-end">
          <Dialog
            open={isCreateOpen}
            onOpenChange={(open) => {
              setIsCreateOpen(open)
              if (!open) {
                setNewProjectToolPermissions(cloneProjectToolPermissions(defaultProjectToolPermissions))
                setNewProjectMemoryLibraryIds([])
                setNewProjectYoloMode(false)
              } else {
                setNewProjectToolPermissions(cloneProjectToolPermissions(defaultProjectToolPermissions))
              }
            }}
          >
            <DialogTrigger asChild>
              <Button>
                <Plus className="mr-2 h-4 w-4" /> 新建项目
              </Button>
            </DialogTrigger>
            <DialogContent className="flex max-h-[90vh] w-[min(96vw,72rem)] max-w-5xl flex-col overflow-hidden p-0">
              <DialogHeader className="border-b px-6 py-5">
                <DialogTitle>新建项目</DialogTitle>
                <DialogDescription>
                  创建一个新项目，稍后可以添加到工作区中。
                </DialogDescription>
              </DialogHeader>
              <div className="min-h-0 flex-1 overflow-y-auto px-6 py-5">
                <div className="grid gap-6 lg:grid-cols-[minmax(0,340px)_minmax(0,1fr)]">
                  <div className="space-y-4">
                    <div className="grid gap-2">
                      <Label htmlFor="project-name">项目名称</Label>
                      <Input
                        id="project-name"
                        placeholder="my-awesome-project"
                        value={newProjectName}
                        onChange={(e) => setNewProjectName(e.target.value)}
                      />
                    </div>
                    <div className="grid gap-2">
                      <Label htmlFor="project-path">项目路径</Label>
                      <Input
                        id="project-path"
                        placeholder="/path/to/project"
                        value={newProjectPath}
                        onChange={(e) => setNewProjectPath(e.target.value)}
                      />
                      <p className="text-xs text-muted-foreground">
                        项目在文件系统中的绝对路径
                      </p>
                    </div>
                    <MemoryLibrarySelector
                      libraries={memoryLibraries}
                      selectedIds={newProjectMemoryLibraryIds}
                      onChange={setNewProjectMemoryLibraryIds}
                    />
                  </div>
                  <ProjectToolPermissionPanel
                    toolInfos={toolInfos}
                    permissions={newProjectToolPermissions}
                    onPermissionsChange={setNewProjectToolPermissions}
                    yoloMode={newProjectYoloMode}
                    onYoloModeChange={setNewProjectYoloMode}
                  />
                </div>
              </div>
              <DialogFooter className="border-t px-6 py-4">
                <Button variant="outline" onClick={() => setIsCreateOpen(false)}>取消</Button>
                <Button onClick={handleCreate}>创建</Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </div>

        {/* Search */}
        <div className="relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder="搜索项目名称或路径..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="pl-10"
          />
        </div>

        {/* Projects Grid */}
        {isLoading ? (
          <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
            {[1, 2, 3, 4, 5, 6].map((i) => (
              <Skeleton key={i} className="h-14 rounded-lg" />
            ))}
          </div>
        ) : filteredProjects.length === 0 ? (
          <div className="flex h-[400px] flex-col items-center justify-center border border-dashed bg-muted/10">
            <div className="mb-4 bg-muted p-3">
              <FolderOpen className="h-6 w-6 text-muted-foreground" />
            </div>
            <h3 className="text-base font-medium">
              {searchQuery ? "未找到匹配的项目" : "没有项目"}
            </h3>
            <p className="text-sm text-muted-foreground mb-4">
              {searchQuery ? "尝试使用不同的关键词搜索" : "创建第一个项目开始使用"}
            </p>
            {!searchQuery && (
              <Button onClick={() => setIsCreateOpen(true)}>
                <Plus className="mr-2 h-4 w-4" />
                创建项目
              </Button>
            )}
          </div>
        ) : (
          <motion.div
            className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            transition={{ duration: 0.2 }}
          >
            {filteredProjects.map((project, index) => (
              <motion.div
                key={project.id}
                initial={{ opacity: 0, y: 8 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.2, delay: index * 0.02 }}
              >
                <div className="group flex items-center gap-3 rounded-lg border border-border/60 bg-card px-3 py-2.5 transition-colors hover:border-border hover:bg-muted/20">
                  <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-primary/10">
                    <FolderOpen className="h-4 w-4 text-primary" />
                  </div>
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-medium text-foreground" title={project.name}>
                      {project.name}
                    </p>
                    <p className="truncate text-xs text-muted-foreground" title={project.path}>
                      {project.path}
                    </p>
                  </div>
                  <div className="flex shrink-0 items-center gap-0.5 opacity-0 transition-opacity group-hover:opacity-100 group-focus-within:opacity-100">
                    <Button
                      size="icon"
                      variant="ghost"
                      className="h-7 w-7"
                      onClick={(e) => openEditDialog(e, project)}
                    >
                      <Edit className="h-3.5 w-3.5" />
                    </Button>
                    <Button
                      size="icon"
                      variant="ghost"
                      className="h-7 w-7 text-destructive hover:text-destructive"
                      onClick={(e) => handleDelete(e, project.id)}
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                  </div>
                </div>
              </motion.div>
            ))}
          </motion.div>
        )}

        {/* 统计信息 */}
        {!isLoading && projects.length > 0 && (
          <div className="text-sm text-muted-foreground">
            共 {projects.length} 个项目
            {searchQuery && filteredProjects.length !== projects.length && 
              `, 显示 ${filteredProjects.length} 个`
            }
          </div>
        )}
      </div>

      {/* 编辑项目对话框 */}
      <Dialog
        open={!!editProject}
        onOpenChange={(open) => {
          if (!open) {
            setEditProject(null)
            setEditProjectMemoryLibraryIds([])
          }
        }}
      >
        <DialogContent className="flex max-h-[90vh] w-[min(96vw,78rem)] max-w-6xl flex-col overflow-hidden p-0">
          <DialogHeader className="border-b px-6 py-5">
            <DialogTitle>编辑项目</DialogTitle>
            <DialogDescription>
              修改项目名称、项目提示词，以及项目级工具权限。
            </DialogDescription>
          </DialogHeader>
          <div className="min-h-0 flex-1 overflow-y-auto px-6 py-5">
            <div className="grid gap-6 xl:grid-cols-[minmax(0,360px)_minmax(0,1fr)]">
              <div className="space-y-4">
                <div className="grid gap-2">
                  <Label htmlFor="edit-name">项目名称</Label>
                  <Input
                    id="edit-name"
                    value={editName}
                    onChange={(e) => setEditName(e.target.value)}
                  />
                </div>
                <div className="grid gap-2">
                  <Label htmlFor="edit-path">项目路径</Label>
                  <Input
                    id="edit-path"
                    value={editPath}
                    onChange={(e) => setEditPath(e.target.value)}
                    placeholder="/path/to/project"
                  />
                  <p className="text-xs text-muted-foreground">
                    需要填写有效的绝对路径。
                  </p>
                </div>
                <div className="grid gap-2">
                  <Label htmlFor="edit-prompt">项目提示词</Label>
                  <Textarea
                    id="edit-prompt"
                    placeholder="为这个项目设置专属的 AI 提示词..."
                    value={editPrompt}
                    onChange={(e) => setEditPrompt(e.target.value)}
                    rows={10}
                    className="font-mono text-sm"
                  />
                  <p className="text-xs text-muted-foreground">
                    此提示词将在该项目的所有任务中自动包含，优先级高于全局和职业提示词。
                  </p>
                </div>
                <MemoryLibrarySelector
                  libraries={memoryLibraries}
                  selectedIds={editProjectMemoryLibraryIds}
                  onChange={setEditProjectMemoryLibraryIds}
                />
              </div>
              <ProjectToolPermissionPanel
                toolInfos={toolInfos}
                permissions={projectToolPermissions}
                onPermissionsChange={setProjectToolPermissions}
                yoloMode={projectYoloMode}
                onYoloModeChange={setProjectYoloMode}
              />
            </div>
          </div>
          <DialogFooter className="border-t px-6 py-4">
            <Button variant="outline" onClick={() => setEditProject(null)}>取消</Button>
            <Button onClick={handleSaveEdit}>保存</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
