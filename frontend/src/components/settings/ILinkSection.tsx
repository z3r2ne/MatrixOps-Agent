import React, { useEffect, useState, useCallback, useRef, useMemo } from "react"
import { useNotification } from "@/components/NotificationProvider"
import { useWorkbench } from "@/components/workbench/WorkbenchProvider"
import { api, type WechatAccount, type Task } from "@/lib/api"
import { useGlobalWebSocket } from "@/hooks/useGlobalWebSocket"
import { Button } from "@/components/ui/button"
import { Switch } from "@/components/ui/switch"
import { Combobox, type ComboboxOption } from "@/components/ui/combobox"
import { cn } from "@/lib/utils"
import {
  MessageCircle,
  Trash2,
  QrCode,
  CheckCircle2,
  XCircle,
  AlertCircle,
} from "lucide-react"

const TASK_STATUS_LABEL: Record<Task["status"], string> = {
  queue: "排队中",
  running: "执行中",
  done: "已完成",
  cancelled: "已取消",
  failed: "失败",
}

function taskProjectName(task: Task): string {
  return task.projectName?.trim() || "未命名项目"
}

function taskDisplayName(task: Task): string {
  if (task.name?.trim()) return task.name.trim()
  if (task.sessionTitle?.trim()) return task.sessionTitle.trim()
  const content = task.content?.trim() || ""
  if (content.length > 40) return `${content.slice(0, 40)}…`
  return content || `任务 #${task.id}`
}

function taskBindingLabel(task: Task): string {
  return `${taskProjectName(task)} · ${taskDisplayName(task)}`
}

export function ILinkSection() {
  const { notify } = useNotification()
  const { tabs, activeTabId } = useWorkbench()
  const [accounts, setAccounts] = useState<WechatAccount[]>([])
  const [tasks, setTasks] = useState<Task[]>([])
  const [loading, setLoading] = useState(false)
  const pollTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const qrLoginWindowRef = useRef<{ electronId?: string; browserWindow?: Window | null } | null>(null)

  const workspaceId = useMemo(() => {
    const tab = tabs.find((t) => t.id === activeTabId)
    return tab?.workspaceId
  }, [tabs, activeTabId])
  const workspaceName = useMemo(() => {
    const tab = tabs.find((t) => t.id === activeTabId)
    return tab?.title
  }, [tabs, activeTabId])

  const loadAccounts = useCallback(async () => {
    try {
      const data = await api.getWechatAccounts()
      setAccounts(data)
    } catch (error) {
      notify({
        type: "error",
        title: "加载账号失败",
        description: error instanceof Error ? error.message : "未知错误",
      })
    }
  }, [notify])

  const loadTasks = useCallback(async (wsId: number) => {
    try {
      const data = await api.getTasksForBinding(wsId)
      setTasks(data)
    } catch {
      setTasks([])
    }
  }, [])

  useEffect(() => {
    loadAccounts()
  }, [loadAccounts])

  useGlobalWebSocket({
    onILinkSessionExpired: () => {
      void loadAccounts()
    },
  })

  useEffect(() => {
    if (workspaceId) {
      void loadTasks(workspaceId)
    } else {
      setTasks([])
    }
  }, [workspaceId, loadTasks])

  useEffect(() => {
    return () => {
      if (pollTimerRef.current) {
        clearTimeout(pollTimerRef.current)
      }
    }
  }, [])

  const handleToggleEnabled = async (account: WechatAccount) => {
    try {
      await api.updateWechatAccount(account.id, { enabled: !account.enabled })
      notify({ type: "success", title: account.enabled ? "账号已禁用" : "账号已启用" })
      await loadAccounts()
    } catch (error) {
      notify({
        type: "error",
        title: "操作失败",
        description: error instanceof Error ? error.message : "未知错误",
      })
    }
  }

  const handleBindTask = async (accountId: number, taskId: string) => {
    try {
      const tid = taskId === "__none__" ? null : Number(taskId)
      await api.updateWechatAccount(accountId, { boundTaskId: tid })
      notify({ type: "success", title: tid ? "会话已绑定" : "会话已解绑" })
      await loadAccounts()
    } catch (error) {
      notify({
        type: "error",
        title: "绑定失败",
        description: error instanceof Error ? error.message : "未知错误",
      })
    }
  }

  const handleDelete = async (account: WechatAccount) => {
    if (!window.confirm(`确定要删除账号 ${account.ilinkUserId || account.botId} 吗？`)) return
    try {
      await api.deleteWechatAccount(account.id)
      notify({ type: "success", title: "账号已删除" })
      await loadAccounts()
    } catch (error) {
      notify({
        type: "error",
        title: "删除失败",
        description: error instanceof Error ? error.message : "未知错误",
      })
    }
  }

  const closeQrLoginWindow = useCallback(() => {
    const w = qrLoginWindowRef.current
    if (!w) return
    if (window.electronAPI?.closeExternalWindow && w.electronId) {
      void window.electronAPI.closeExternalWindow(w.electronId)
    } else if (w.browserWindow && !w.browserWindow.closed) {
      w.browserWindow.close()
    }
    qrLoginWindowRef.current = null
  }, [])

  const handleFetchQR = async () => {
    setLoading(true)
    try {
      const data = await api.fetchWechatQRCode()
      closeQrLoginWindow()
      if (window.electronAPI?.openExternalWindow) {
        const result = await window.electronAPI.openExternalWindow({
          url: data.qrcode_img_content,
          title: "扫码登陆",
        })
        if (result.windowId) {
          qrLoginWindowRef.current = { electronId: result.windowId }
        }
      } else {
        const win = window.open(data.qrcode_img_content, "_blank")
        qrLoginWindowRef.current = { browserWindow: win }
      }
      pollQRStatus(data.qrcode)
    } catch (error) {
      notify({
        type: "error",
        title: "获取二维码失败",
        description: error instanceof Error ? error.message : "未知错误",
      })
    } finally {
      setLoading(false)
    }
  }

  const pollQRStatus = useCallback(
    async (qrcode: string) => {
      try {
        const account = await api.pollWechatQRStatus(qrcode)
        closeQrLoginWindow()
        notify({ type: "success", title: "登录成功", description: `账号 ${account.ilinkUserId || account.botId} 已添加` })
        await loadAccounts()
      } catch (error: any) {
        if (error?.status === 202) {
          // Still waiting, poll again
          pollTimerRef.current = setTimeout(() => pollQRStatus(qrcode), 3000)
        } else {
          notify({
            type: "error",
            title: "登录失败",
            description: error instanceof Error ? error.message : "未知错误",
          })
        }
      }
    },
    [notify, loadAccounts, closeQrLoginWindow]
  )

  const getStatusIcon = (status: string) => {
    switch (status) {
      case "online":
        return <CheckCircle2 className="h-4 w-4 text-emerald-500" />
      case "error":
        return <AlertCircle className="h-4 w-4 text-red-500" />
      default:
        return <XCircle className="h-4 w-4 text-muted-foreground" />
    }
  }

  const bindingOptionsByAccount = useMemo(() => {
    const taskById = new Map(tasks.map((t) => [t.id, t]))
    return accounts.reduce<Record<number, ComboboxOption[]>>((acc, account) => {
      const options: ComboboxOption[] = [
        {
          value: "__none__",
          label: "不绑定会话",
          description: "收到微信消息时不自动触发任务",
          searchText: "不绑定",
        },
      ]
      for (const task of tasks) {
        options.push({
          value: String(task.id),
          label: taskBindingLabel(task),
          description: TASK_STATUS_LABEL[task.status] || task.status,
          searchText: `${task.id} ${task.projectName || ""} ${task.name || ""} ${task.sessionTitle || ""} ${task.content || ""}`,
        })
      }
      const boundId = account.boundTaskId
      if (boundId && !taskById.has(boundId)) {
        options.push({
          value: String(boundId),
          label: `会话 #${boundId}`,
          description: "不在当前工作区，请重新选择",
          disabled: true,
        })
      }
      acc[account.id] = options
      return acc
    }, {})
  }, [accounts, tasks])

  const getStatusLabel = (status: string) => {
    switch (status) {
      case "online":
        return "在线"
      case "error":
        return "错误"
      default:
        return "离线"
    }
  }

  return (
    <div className="space-y-6">
      <section className="border border-border/60 bg-card/50 p-4 shadow-sm sm:p-5">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-lg font-semibold">iLink 微信账号</h2>
            <p className="text-sm text-muted-foreground">
              管理已绑定的微信 Bot 账号；启用后将在收到消息时自动触发当前工作区绑定的会话
              {workspaceName ? `（${workspaceName}）` : ""}
            </p>
          </div>
          <Button onClick={handleFetchQR} disabled={loading}>
            <QrCode className="mr-2 h-4 w-4" />
            {loading ? "获取中..." : "扫码登录"}
          </Button>
        </div>
      </section>

      <section className="border border-border/60 bg-card/50 p-4 shadow-sm sm:p-5">
        {accounts.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-12 text-muted-foreground">
            <MessageCircle className="mb-3 h-10 w-10 opacity-40" />
            <p className="text-sm">暂无微信账号</p>
            <p className="mt-1 text-xs">点击上方「扫码登录」添加第一个账号</p>
          </div>
        ) : (
          <div className="space-y-3">
            {accounts.map((account) => (
              <div
                key={account.id}
                className="flex flex-col gap-3 border border-border/40 bg-background p-4 sm:flex-row sm:items-center sm:justify-between"
              >
                <div className="flex items-center gap-3 min-w-0">
                  {getStatusIcon(account.status)}
                  <div className="min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="font-medium truncate">
                        {account.ilinkUserId || account.botId}
                      </span>
                      <span
                        className={cn(
                          "inline-flex items-center rounded px-1.5 py-0.5 text-xs font-medium",
                          account.status === "online"
                            ? "bg-emerald-500/10 text-emerald-600"
                            : account.status === "error"
                            ? "bg-red-500/10 text-red-600"
                            : "bg-muted text-muted-foreground"
                        )}
                      >
                        {getStatusLabel(account.status)}
                      </span>
                    </div>
                    <div className="mt-0.5 text-xs text-muted-foreground truncate">
                      {account.baseURL}
                    </div>
                  </div>
                </div>

                <div className="flex flex-wrap items-center gap-3 sm:justify-end">
                  <div className="flex min-w-[220px] flex-col gap-1 sm:min-w-[280px] sm:max-w-[360px]">
                    <span className="text-xs text-muted-foreground">绑定会话</span>
                    <Combobox
                      items={bindingOptionsByAccount[account.id] ?? []}
                      value={account.boundTaskId?.toString() || "__none__"}
                      onValueChange={(value) => handleBindTask(account.id, value)}
                      placeholder={workspaceId ? "选择工作区会话" : "请先打开工作区"}
                      searchPlaceholder="搜索会话标题、内容…"
                      emptyText={workspaceId ? "当前工作区暂无会话" : "请在工作台打开一个工作区"}
                      disabled={!workspaceId}
                      className="w-full"
                      inputClassName="h-9 text-sm"
                      contentClassName="max-h-[min(320px,50vh)]"
                      renderItem={(item, selected) => (
                        <div className="flex min-w-0 flex-col gap-0.5 py-0.5">
                          <div className="flex items-center gap-2">
                            <span className={cn("truncate text-sm", selected && "font-medium")}>
                              {item.label}
                            </span>
                            {selected ? (
                              <CheckCircle2 className="h-3.5 w-3.5 shrink-0 text-primary" />
                            ) : null}
                          </div>
                          {item.description ? (
                            <span className="truncate text-xs text-muted-foreground">
                              {item.description}
                            </span>
                          ) : null}
                        </div>
                      )}
                    />
                  </div>

                  <div className="flex items-center gap-2">
                    <Switch
                      checked={account.enabled}
                      onCheckedChange={() => handleToggleEnabled(account)}
                    />
                    <span className="text-xs text-muted-foreground whitespace-nowrap">
                      {account.enabled ? "已启用" : "已禁用"}
                    </span>
                  </div>

                  <Button
                    variant="ghost"
                    size="sm"
                    className="text-red-500 hover:text-red-600 hover:bg-red-500/10"
                    onClick={() => handleDelete(account)}
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </Button>
                </div>
              </div>
            ))}
          </div>
        )}
      </section>
    </div>
  )
}
