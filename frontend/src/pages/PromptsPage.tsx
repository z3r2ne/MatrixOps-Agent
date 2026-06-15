import React, { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react"
import { MessageSquare, Users, FolderOpen, Save } from "lucide-react"

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Textarea } from "@/components/ui/textarea"
import { Button } from "@/components/ui/button"
import { Label } from "@/components/ui/label"
import { Badge } from "@/components/ui/badge"
import { Combobox, type ComboboxOption } from "@/components/ui/combobox"
import { api, Occupation, Project } from "@/lib/api"
import { useNotification } from "@/components/NotificationProvider"
import { useElectronSettingsInitialProjectId } from "@/components/electron/electron-main-contexts"
import { cn } from "@/lib/utils"

type TabType = "global" | "occupation" | "project"

export function PromptsPage() {
  const initialProjectId = useElectronSettingsInitialProjectId()
  const [activeTab, setActiveTab] = useState<TabType>("global")
  const [globalPrompt, setGlobalPrompt] = useState("")
  const [occupations, setOccupations] = useState<Occupation[]>([])
  const [selectedOccupation, setSelectedOccupation] = useState<Occupation | null>(null)
  const [projects, setProjects] = useState<Project[]>([])
  const [selectedProject, setSelectedProject] = useState<Project | null>(null)
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const { notify } = useNotification()
  const scrollRef = useRef<HTMLDivElement>(null)

  const occupationOptions = useMemo<ComboboxOption[]>(
    () =>
      occupations.map((occupation) => ({
        value: String(occupation.id),
        label: occupation.name,
        description: occupation.code,
        searchText: `${occupation.name} ${occupation.code} ${occupation.description || ""}`,
      })),
    [occupations]
  )

  const projectOptions = useMemo<ComboboxOption[]>(
    () =>
      projects.map((project) => ({
        value: String(project.id),
        label: project.name,
        description: project.path,
        searchText: `${project.name} ${project.path}`,
      })),
    [projects]
  )

  useEffect(() => {
    loadData()
  }, [])

  useLayoutEffect(() => {
    scrollRef.current?.scrollTo({ top: 0, behavior: "smooth" })
  }, [activeTab])

  const loadData = async () => {
    setLoading(true)
    try {
      const [globalData, occupationsData, projectsData] = await Promise.all([
        api.getGlobalPrompt(),
        api.getOccupations(),
        api.getAllProjects(),
      ])
      setGlobalPrompt(globalData.prompt || "")
      setOccupations(occupationsData)
      setProjects(projectsData)

      // 默认选择第一个职业
      if (occupationsData.length > 0 && !selectedOccupation) {
        setSelectedOccupation(occupationsData[0])
      }

      // 如果有初始项目ID，自动切换到项目提示词并选中
      if (initialProjectId) {
        const project = projectsData.find((p) => p.id === initialProjectId)
        if (project) {
          setActiveTab("project")
          try {
            const promptData = await api.getProjectPrompt(project.id)
            setSelectedProject({ ...project, prompt: promptData.prompt })
          } catch {
            setSelectedProject(project)
          }
        }
      }
    } catch (error) {
      notify({ type: "error", title: "加载失败", description: error instanceof Error ? error.message : "未知错误" })
    } finally {
      setLoading(false)
    }
  }

  const handleSaveGlobalPrompt = async () => {
    setSaving(true)
    try {
      await api.updateGlobalPrompt(globalPrompt)
      notify({ type: "success", title: "保存成功", description: "全局提示词已更新" })
    } catch (error) {
      notify({ type: "error", title: "保存失败", description: error instanceof Error ? error.message : "未知错误" })
    } finally {
      setSaving(false)
    }
  }

  const handleSaveOccupationPrompt = async () => {
    if (!selectedOccupation) return
    setSaving(true)
    try {
      await api.updateOccupation(selectedOccupation.id, {
        prompt: selectedOccupation.prompt,
      })
      notify({ type: "success", title: "保存成功", description: `${selectedOccupation.name}提示词已更新` })
      await loadData()
    } catch (error) {
      notify({ type: "error", title: "保存失败", description: error instanceof Error ? error.message : "未知错误" })
    } finally {
      setSaving(false)
    }
  }

  const handleSaveProjectPrompt = async () => {
    if (!selectedProject) return
    setSaving(true)
    try {
      await api.updateProjectPrompt(selectedProject.id, selectedProject.prompt || "")
      notify({ type: "success", title: "保存成功", description: `项目 ${selectedProject.name} 提示词已更新` })
      await loadData()
    } catch (error) {
      notify({ type: "error", title: "保存失败", description: error instanceof Error ? error.message : "未知错误" })
    } finally {
      setSaving(false)
    }
  }

  const handleOccupationChange = (occupationId: string) => {
    const occupation = occupations.find(o => String(o.id) === occupationId)
    setSelectedOccupation(occupation || null)
  }

  const handleProjectChange = async (projectId: string) => {
    const project = projects.find(p => String(p.id) === projectId)
    if (project) {
      try {
        const promptData = await api.getProjectPrompt(project.id)
        setSelectedProject({ ...project, prompt: promptData.prompt })
      } catch (error) {
        setSelectedProject(project)
      }
    } else {
      setSelectedProject(null)
    }
  }

  const menuItems = [
    {
      id: "global" as const,
      label: "全局提示词",
      icon: MessageSquare,
      description: "应用于所有任务的基础提示词",
    },
    {
      id: "occupation" as const,
      label: "职业提示词",
      icon: Users,
      description: "为每个职业定制专属提示词",
    },
    {
      id: "project" as const,
      label: "项目提示词",
      icon: FolderOpen,
      description: "为每个项目设置特定提示词",
    },
  ]

  return (
    <div ref={scrollRef} className="flex min-h-0 flex-1 overflow-y-auto overscroll-y-contain [scrollbar-gutter:stable]">
      <div className="mx-auto flex w-full max-w-6xl min-w-0 flex-col gap-6 p-4 sm:p-6 lg:p-8">
        {/* Tab 区 */}
        <section className="border border-border/60 bg-card/50 p-4 shadow-sm sm:p-5">
          <nav className="grid gap-2 sm:grid-cols-2 xl:grid-cols-4">
            {menuItems.map((item) => {
              const Icon = item.icon
              const isActive = activeTab === item.id

              return (
                <button
                  key={item.id}
                  onClick={() => setActiveTab(item.id)}
                  className={cn(
                    "flex min-w-0 items-start gap-3 border px-4 py-3 text-left transition-colors",
                    "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2",
                    isActive ? "border-primary/30 bg-primary/5 text-foreground shadow-sm" : "border-border/60 bg-background hover:bg-accent/50"
                  )}
                >
                  <Icon className={cn("mt-0.5 h-4 w-4 shrink-0", isActive ? "text-primary" : "text-muted-foreground")} />
                  <div className="min-w-0">
                    <div className="text-sm font-medium">{item.label}</div>
                    <div className="mt-1 text-xs text-muted-foreground">{item.description}</div>
                  </div>
                </button>
              )
            })}
          </nav>
        </section>

        {/* 内容面板 */}
        <div className="min-w-0 overflow-x-clip">
          {loading ? (
            <Card>
              <CardContent className="pt-6">
                <div className="text-center text-muted-foreground">加载中...</div>
              </CardContent>
            </Card>
          ) : (
            <>
              {/* 全局提示词 */}
              {activeTab === "global" && (
                <div key="global">
                  <Card>
                    <CardHeader>
                      <div className="flex items-center gap-2">
                        <MessageSquare className="h-5 w-5 text-primary" />
                        <CardTitle>全局提示词</CardTitle>
                      </div>
                      <CardDescription>
                        全局提示词会作为所有任务的基础上下文，优先级最低
                      </CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-4">
                      <div className="space-y-2">
                        <Label htmlFor="global-prompt">提示词内容</Label>
                        <Textarea
                          id="global-prompt"
                          value={globalPrompt}
                          onChange={(e) => setGlobalPrompt(e.target.value)}
                          placeholder="输入全局提示词..."
                          rows={15}
                          className="font-mono text-sm"
                        />
                        <p className="text-xs text-muted-foreground">
                          支持 Markdown 格式。此提示词将添加到所有 Worker 的系统消息中。
                        </p>
                      </div>
                      <Button onClick={handleSaveGlobalPrompt} disabled={saving}>
                        <Save className="h-4 w-4 mr-2" />
                        {saving ? "保存中..." : "保存"}
                      </Button>
                    </CardContent>
                  </Card>
                </div>
              )}

              {/* 职业提示词 */}
              {activeTab === "occupation" && (
                <div key="occupation">
                  <Card>
                    <CardHeader>
                      <div className="flex items-center gap-2">
                        <Users className="h-5 w-5 text-primary" />
                        <CardTitle>职业提示词</CardTitle>
                      </div>
                      <CardDescription>
                        为每个职业设置专属提示词，优先级高于全局提示词
                      </CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-4">
                      <div className="space-y-2">
                        <Label htmlFor="occupation-select">选择职业</Label>
                        <Combobox
                          id="occupation-select"
                          items={occupationOptions}
                          value={selectedOccupation ? String(selectedOccupation.id) : ""}
                          onValueChange={handleOccupationChange}
                          placeholder="选择职业"
                          searchPlaceholder="搜索职业"
                          emptyText="未找到职业"
                          renderItem={(item) => (
                            <div className="flex items-center gap-2">
                              <Badge variant="outline">{item.label}</Badge>
                              {item.description ? (
                                <span className="text-xs text-muted-foreground">({item.description})</span>
                              ) : null}
                            </div>
                          )}
                        />
                      </div>

                      {selectedOccupation && (
                        <>
                          <div className="space-y-2">
                            <div className="flex items-center justify-between">
                              <Label htmlFor="occupation-prompt">
                                {selectedOccupation.name} 提示词
                              </Label>
                              <Badge variant="secondary">{selectedOccupation.code}</Badge>
                            </div>
                            <Textarea
                              id="occupation-prompt"
                              value={selectedOccupation.prompt || ""}
                              onChange={(e) =>
                                setSelectedOccupation({
                                  ...selectedOccupation,
                                  prompt: e.target.value,
                                })
                              }
                              placeholder={`输入 ${selectedOccupation.name} 的专属提示词...`}
                              rows={15}
                              className="font-mono text-sm"
                            />
                            <p className="text-xs text-muted-foreground">
                              {selectedOccupation.description}
                            </p>
                          </div>
                          <Button onClick={handleSaveOccupationPrompt} disabled={saving}>
                            <Save className="h-4 w-4 mr-2" />
                            {saving ? "保存中..." : "保存"}
                          </Button>
                        </>
                      )}
                    </CardContent>
                  </Card>
                </div>
              )}

              {/* 项目提示词 */}
              {activeTab === "project" && (
                <div key="project">
                  <Card>
                    <CardHeader>
                      <div className="flex items-center gap-2">
                        <FolderOpen className="h-5 w-5 text-primary" />
                        <CardTitle>项目提示词</CardTitle>
                      </div>
                      <CardDescription>
                        为每个项目设置特定提示词，优先级高于职业提示词
                      </CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-4">
                      <div className="space-y-2">
                        <Label htmlFor="project-select">选择项目</Label>
                        <Combobox
                          id="project-select"
                          items={projectOptions}
                          value={selectedProject ? String(selectedProject.id) : ""}
                          onValueChange={handleProjectChange}
                          placeholder="选择项目"
                          searchPlaceholder="搜索项目"
                          emptyText="未找到项目"
                          renderItem={(item) => (
                            <div className="flex min-w-0 flex-col">
                              <span className="truncate">{item.label}</span>
                              {item.description ? (
                                <span className="truncate text-xs text-muted-foreground">{item.description}</span>
                              ) : null}
                            </div>
                          )}
                        />
                      </div>

                      {selectedProject && (
                        <>
                          <div className="space-y-2">
                            <div className="flex items-center justify-between">
                              <Label htmlFor="project-prompt">
                                {selectedProject.name} 提示词
                              </Label>
                              <Badge variant="secondary">{selectedProject.path}</Badge>
                            </div>
                            <Textarea
                              id="project-prompt"
                              value={selectedProject.prompt || ""}
                              onChange={(e) =>
                                setSelectedProject({
                                  ...selectedProject,
                                  prompt: e.target.value,
                                })
                              }
                              placeholder={`输入项目 ${selectedProject.name} 的专属提示词...`}
                              rows={15}
                              className="font-mono text-sm"
                            />
                            <p className="text-xs text-muted-foreground">
                              此提示词将添加到所有在此项目中执行的任务的系统消息中。
                            </p>
                          </div>
                          <Button onClick={handleSaveProjectPrompt} disabled={saving}>
                            <Save className="h-4 w-4 mr-2" />
                            {saving ? "保存中..." : "保存"}
                          </Button>
                        </>
                      )}

                      {projects.length === 0 && (
                        <div className="rounded-xl border border-dashed border-border/70 p-6 text-center text-sm text-muted-foreground">
                          暂无项目，请先创建项目
                        </div>
                      )}
                    </CardContent>
                  </Card>
                </div>
              )}
            </>
          )}
        </div>
      </div>
    </div>
  )
}
