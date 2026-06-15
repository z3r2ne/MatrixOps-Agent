import React, { useEffect, useState } from "react"
import { Loader2 } from "lucide-react"
import { api, type WorkspaceResponse } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { ScrollArea } from "@/components/ui/scroll-area"
import { toast } from "sonner"
import { cn } from "@/lib/utils"
import { ElectronWindowChromeSpacer } from "@/components/layout/ElectronWindowChrome"
import { setPendingElectronWorkbenchOpen } from "@/lib/electronPendingWorkbench"

async function handoffToMainWindow(workspace: { id: number; name: string }) {
  setPendingElectronWorkbenchOpen({ kind: "workspace", id: workspace.id, name: workspace.name })
  const apiBridge = window.electronAPI
  if (!apiBridge?.closeLauncherAndOpenMain) {
    toast.error("当前环境不支持窗口切换")
    return
  }
  const res = await apiBridge.closeLauncherAndOpenMain()
  if (!res?.ok) {
    toast.error(res?.error || "打开主窗口失败")
  }
}

/**
 * Electron 专用启动窗口（无侧栏）：选择或创建工作区后关闭本窗口并打开主窗口。
 */
export function ElectronLauncherPage() {
  const [pickOpen, setPickOpen] = useState(false)
  const [createOpen, setCreateOpen] = useState(false)
  const [workspaces, setWorkspaces] = useState<WorkspaceResponse[]>([])
  const [pickLoading, setPickLoading] = useState(false)
  const [newWorkspaceName, setNewWorkspaceName] = useState("")
  const [newWorkspaceType, setNewWorkspaceType] = useState("code")
  const [creating, setCreating] = useState(false)

  useEffect(() => {
    if (!pickOpen) return
    let cancelled = false
    const run = async () => {
      setPickLoading(true)
      try {
        const list = await api.getWorkspaces()
        if (!cancelled) setWorkspaces(list)
      } catch (e) {
        console.error(e)
        if (!cancelled) {
          setWorkspaces([])
          toast.error("加载工作区列表失败")
        }
      } finally {
        if (!cancelled) setPickLoading(false)
      }
    }
    void run()
    return () => {
      cancelled = true
    }
  }, [pickOpen])

  const pickWorkspace = async (ws: WorkspaceResponse) => {
    try {
      await handoffToMainWindow({ id: ws.id, name: ws.name })
      setPickOpen(false)
    } catch (e) {
      console.error(e)
      toast.error("打开主窗口失败")
    }
  }

  const handleCreateWorkspace = async () => {
    const name = newWorkspaceName.trim()
    if (!name) return
    setCreating(true)
    try {
      const created = await api.createWorkspace({ name, type: newWorkspaceType })
      setNewWorkspaceName("")
      setNewWorkspaceType("code")
      setCreateOpen(false)
      await handoffToMainWindow({ id: created.id, name: created.name })
      toast.success("工作区已创建")
    } catch (e) {
      console.error(e)
      toast.error("创建工作区失败")
    } finally {
      setCreating(false)
    }
  }

  return (
    <div className="flex h-screen w-full flex-col overflow-hidden bg-background text-foreground">
      <ElectronWindowChromeSpacer className="z-[60]" />
      <div className="flex min-h-0 flex-1 items-center justify-center px-8 py-10">
        <div className="flex w-full max-w-sm flex-col items-center justify-center text-center">
          <div className="mb-10 space-y-2">
            <h1 className="text-[2.1rem] font-semibold tracking-[-0.04em] text-black">MatrixOps</h1>
            <p className="text-sm text-neutral-500">工作区启动器</p>
          </div>

          <div className="w-full space-y-2">
            <button
              type="button"
              className="flex w-full items-center justify-center border border-neutral-300 bg-neutral-100 px-4 py-3 text-center text-[15px] font-medium text-black transition-colors hover:bg-neutral-200"
              onClick={() => setCreateOpen(true)}
            >
              新建工作区
            </button>

            <button
              type="button"
              className="flex w-full items-center justify-center px-4 py-2.5 text-center text-[15px] text-neutral-700 transition-colors hover:bg-neutral-100 hover:text-black"
              onClick={() => setPickOpen(true)}
            >
              打开工作区
            </button>
          </div>
        </div>
      </div>

      <Dialog open={pickOpen} onOpenChange={setPickOpen}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>打开工作区</DialogTitle>
            <DialogDescription>选择一个工作区，将在主窗口中打开。</DialogDescription>
          </DialogHeader>
          {pickLoading ? (
            <div className="flex justify-center py-12 text-muted-foreground">
              <Loader2 className="h-8 w-8 animate-spin" />
            </div>
          ) : (
            <ScrollArea className="max-h-[50vh] pr-3">
              <div className="space-y-2">
                {workspaces.length === 0 ? (
                  <p className="py-6 text-center text-sm text-muted-foreground">暂无工作区，请先创建。</p>
                ) : (
                  workspaces.map((ws) => (
                    <button
                      key={ws.id}
                      type="button"
                      className={cn(
                        "w-full rounded-md border px-3 py-3 text-left text-sm transition-colors",
                        "hover:bg-accent hover:text-accent-foreground",
                        !ws.pathExists && "border-dashed opacity-60"
                      )}
                      onClick={() => void pickWorkspace(ws)}
                    >
                      <div className="font-medium">{ws.name}</div>
                      <div className="mt-0.5 truncate text-xs text-muted-foreground">{ws.path}</div>
                      {!ws.pathExists ? <div className="mt-1 text-xs text-destructive">目录不存在</div> : null}
                    </button>
                  ))
                )}
              </div>
            </ScrollArea>
          )}
          <DialogFooter>
            <Button variant="outline" onClick={() => setPickOpen(false)}>
              取消
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>新建工作区</DialogTitle>
            <DialogDescription>创建成功后将进入主窗口并打开该工作区。</DialogDescription>
          </DialogHeader>
          <div className="grid gap-2 py-2">
            <Label htmlFor="electron-launcher-ws-name">名称</Label>
            <Input
              id="electron-launcher-ws-name"
              value={newWorkspaceName}
              onChange={(e) => setNewWorkspaceName(e.target.value)}
              placeholder="我的工作区"
              onKeyDown={(e) => {
                if (e.key === "Enter") void handleCreateWorkspace()
              }}
            />
          </div>
          <div className="grid gap-2 py-2">
            <Label htmlFor="electron-launcher-ws-type">类型</Label>
            <Select value={newWorkspaceType} onValueChange={(value) => setNewWorkspaceType(value)}>
              <SelectTrigger id="electron-launcher-ws-type">
                <SelectValue placeholder="选择类型" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="code">代码</SelectItem>
                <SelectItem value="test">测试</SelectItem>
                <SelectItem value="claw">抓取</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateOpen(false)} disabled={creating}>
              取消
            </Button>
            <Button onClick={() => void handleCreateWorkspace()} disabled={creating || !newWorkspaceName.trim()}>
              {creating ? <Loader2 className="h-4 w-4 animate-spin" /> : "创建并进入"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
