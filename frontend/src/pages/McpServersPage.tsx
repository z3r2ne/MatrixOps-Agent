import React, { useCallback, useEffect, useMemo, useState } from "react"
import { AlertCircle, Eye, Pencil, Plug, Plus, RefreshCw, Trash2 } from "lucide-react"

import { api, McpServer, McpServerCreate, McpServerUpdate, McpToolInfo } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { Badge } from "@/components/ui/badge"
import { Skeleton } from "@/components/ui/skeleton"
import { Textarea } from "@/components/ui/textarea"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { useConfirmDialog } from "@/components/ConfirmDialogProvider"
import { cn } from "@/lib/utils"
import { toast } from "sonner"

type DialogState =
  | { mode: "create"; server: null }
  | { mode: "edit"; server: McpServer }
  | null

type TransportKind = "stdio" | "sse" | "http"

function transportLabel(transport: string): string {
  switch (transport) {
    case "sse":
      return "SSE"
    case "http":
      return "HTTP"
    default:
      return "stdio"
  }
}

export function McpServersPage() {
  const { confirm } = useConfirmDialog()
  const [servers, setServers] = useState<McpServer[]>([])
  const [loading, setLoading] = useState(true)
  const [pendingId, setPendingId] = useState<number | null>(null)
  const [dialogState, setDialogState] = useState<DialogState>(null)
  const [toolsDialogServer, setToolsDialogServer] = useState<McpServer | null>(null)
  const [tools, setTools] = useState<McpToolInfo[]>([])
  const [toolsLoading, setToolsLoading] = useState(false)

  const [name, setName] = useState("")
  const [transport, setTransport] = useState<TransportKind>("stdio")
  const [command, setCommand] = useState("")
  const [argsJson, setArgsJson] = useState("[]")
  const [envJson, setEnvJson] = useState("{}")
  const [url, setUrl] = useState("")
  const [headersJson, setHeadersJson] = useState("{}")
  const [enabled, setEnabled] = useState(true)

  const loadServers = useCallback(async () => {
    try {
      setLoading(true)
      const data = await api.getMcpServers()
      setServers(data)
    } catch (error) {
      console.error("Failed to load MCP servers:", error)
      toast.error("加载 MCP 服务器失败")
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    loadServers()
  }, [loadServers])

  useEffect(() => {
    if (!dialogState) return
    if (dialogState.mode === "edit") {
      const server = dialogState.server
      setName(server.name)
      setTransport(server.transport === "sse" ? "sse" : server.transport === "http" ? "http" : "stdio")
      setCommand(server.command || "")
      setArgsJson(server.argsJson || "[]")
      setEnvJson(server.envJson || "{}")
      setUrl(server.url || "")
      setHeadersJson(server.headersJson || "{}")
      setEnabled(server.enabled)
      return
    }
    setName("")
    setTransport("stdio")
    setCommand("")
    setArgsJson("[]")
    setEnvJson("{}")
    setUrl("")
    setHeadersJson("{}")
    setEnabled(true)
  }, [dialogState])

  const enabledCount = useMemo(() => servers.filter((server) => server.enabled).length, [servers])
  const connectedCount = useMemo(() => servers.filter((server) => server.connected).length, [servers])

  const handleSubmit = useCallback(async () => {
    if (!name.trim()) {
      toast.error("请填写服务器名称")
      return
    }
    if (transport === "stdio" && !command.trim()) {
      toast.error("stdio 模式需要填写启动命令")
      return
    }
    if ((transport === "sse" || transport === "http") && !url.trim()) {
      toast.error(`${transport === "http" ? "HTTP" : "SSE"} 模式需要填写 URL`)
      return
    }
    try {
      JSON.parse(argsJson || "[]")
      JSON.parse(envJson || "{}")
      JSON.parse(headersJson || "{}")
    } catch {
      toast.error("Args / Env / Headers 必须是合法 JSON")
      return
    }

    const payload: McpServerCreate | McpServerUpdate = {
      name: name.trim(),
      transport,
      command: command.trim(),
      argsJson: argsJson.trim() || "[]",
      envJson: envJson.trim() || "{}",
      url: url.trim(),
      headersJson: headersJson.trim() || "{}",
      enabled,
    }

    try {
      if (dialogState?.mode === "edit") {
        await api.updateMcpServer(dialogState.server.id, payload)
        toast.success("MCP 服务器已更新")
      } else {
        await api.createMcpServer(payload as McpServerCreate)
        toast.success("MCP 服务器已创建")
      }
      setDialogState(null)
      await loadServers()
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "保存 MCP 服务器失败"
      toast.error(message)
      await loadServers()
    }
  }, [argsJson, command, dialogState, enabled, envJson, headersJson, loadServers, name, transport, url])

  const handleReconnect = useCallback(async (server: McpServer) => {
    try {
      setPendingId(server.id)
      await api.reconnectMcpServer(server.id)
      toast.success(`已重新连接 ${server.name}`)
      await loadServers()
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "连接失败"
      toast.error(message)
      await loadServers()
    } finally {
      setPendingId(null)
    }
  }, [loadServers])

  const handleDelete = useCallback(async (server: McpServer) => {
    const confirmed = await confirm({
      title: "删除 MCP 服务器",
      description: `确定删除 “${server.name}” 吗？`,
      confirmLabel: "删除",
      tone: "destructive",
    })
    if (!confirmed) return
    try {
      setPendingId(server.id)
      await api.deleteMcpServer(server.id)
      toast.success("MCP 服务器已删除")
      await loadServers()
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "删除失败"
      toast.error(message)
    } finally {
      setPendingId(null)
    }
  }, [confirm, loadServers])

  const handleToggleEnabled = useCallback(async (server: McpServer, nextEnabled: boolean) => {
    try {
      setPendingId(server.id)
      await api.updateMcpServer(server.id, { enabled: nextEnabled })
      toast.success(nextEnabled ? "已启用 MCP 服务器" : "已停用 MCP 服务器")
      await loadServers()
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "更新失败"
      toast.error(message)
      await loadServers()
    } finally {
      setPendingId(null)
    }
  }, [loadServers])

  const openToolsDialog = useCallback(async (server: McpServer) => {
    setToolsDialogServer(server)
    setToolsLoading(true)
    try {
      const data = await api.getMcpServerTools(server.id)
      setTools(data)
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "加载工具列表失败"
      toast.error(message)
      setTools([])
    } finally {
      setToolsLoading(false)
    }
  }, [])

  return (
    <div className="flex-1 overflow-y-auto p-8">
      <div className="mx-auto max-w-6xl space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant="outline">服务器 {servers.length}</Badge>
            <Badge variant="outline">已启用 {enabledCount}</Badge>
            <Badge variant="outline">已连接 {connectedCount}</Badge>
          </div>
          <div className="flex items-center gap-2">
            <Button variant="outline" onClick={loadServers} disabled={loading}>
              <RefreshCw className={cn("mr-2 h-4 w-4", loading && "animate-spin")} />
              刷新
            </Button>
            <Button onClick={() => setDialogState({ mode: "create", server: null })}>
              <Plus className="mr-2 h-4 w-4" />
              新建 MCP
            </Button>
          </div>
        </div>

        {loading ? (
          <div className="space-y-4">
            {Array.from({ length: 3 }).map((_, idx) => (
              <Skeleton key={idx} className="h-36 rounded-xl" />
            ))}
          </div>
        ) : servers.length === 0 ? (
          <div className="flex h-[280px] flex-col items-center justify-center rounded-xl border border-dashed border-border/70 bg-muted/10 text-center">
            <div className="mb-4 rounded-xl bg-muted p-3">
              <Plug className="h-6 w-6 text-muted-foreground" />
            </div>
            <h3 className="text-base font-medium">还没有 MCP 服务器</h3>
            <p className="mt-1 text-sm text-muted-foreground">添加 stdio、SSE 或 HTTP（Streamable HTTP）MCP 服务器，启动后会自动加载工具。</p>
          </div>
        ) : (
          <div className="space-y-3">
            {servers.map((server) => {
              const pending = pendingId === server.id
              return (
                <Card key={server.id} className="border-border/60 bg-background/80">
                  <CardHeader className="space-y-2 pb-2">
                    <div className="flex flex-wrap items-start justify-between gap-3">
                      <div className="min-w-0">
                        <CardTitle className="flex flex-wrap items-center gap-2 text-base">
                          <span className="truncate">{server.name}</span>
                          <Badge variant="secondary">{transportLabel(server.transport)}</Badge>
                          <Badge variant={server.enabled ? "default" : "outline"}>
                            {server.enabled ? "已启用" : "已停用"}
                          </Badge>
                          <Badge variant={server.connected ? "default" : "outline"}>
                            {server.connected ? "已连接" : "未连接"}
                          </Badge>
                        </CardTitle>
                        <CardDescription className="mt-1 break-all text-xs">
                          {server.transport === "stdio" ? `${server.command} ${server.argsJson}` : server.url}
                        </CardDescription>
                      </div>
                      <div className="flex flex-wrap items-center gap-2">
                        <Switch
                          checked={server.enabled}
                          disabled={pending}
                          onCheckedChange={(checked) => handleToggleEnabled(server, checked)}
                        />
                        <Button variant="outline" size="sm" className="h-8" disabled={pending || !server.enabled} onClick={() => handleReconnect(server)}>
                          <RefreshCw className={cn("mr-2 h-4 w-4", pending && "animate-spin")} />
                          重连
                        </Button>
                        <Button variant="outline" size="sm" className="h-8" onClick={() => openToolsDialog(server)}>
                          <Eye className="mr-2 h-4 w-4" />
                          工具
                        </Button>
                        <Button variant="outline" size="sm" className="h-8" onClick={() => setDialogState({ mode: "edit", server })}>
                          <Pencil className="mr-2 h-4 w-4" />
                          编辑
                        </Button>
                        <Button variant="destructive" size="sm" className="h-8" disabled={pending} onClick={() => handleDelete(server)}>
                          <Trash2 className="mr-2 h-4 w-4" />
                          删除
                        </Button>
                      </div>
                    </div>
                  </CardHeader>
                  <CardContent className="space-y-3 pt-0">
                    <div className="grid gap-2 md:grid-cols-3">
                      <div className="rounded-lg bg-muted/50 px-2.5 py-2">
                        <div className="text-[11px] font-medium text-foreground/80">工具数量</div>
                        <div className="mt-1 text-xs text-muted-foreground">{server.toolCount}</div>
                      </div>
                      <div className="rounded-lg bg-muted/50 px-2.5 py-2">
                        <div className="text-[11px] font-medium text-foreground/80">更新时间</div>
                        <div className="mt-1 text-xs text-muted-foreground">
                          {server.updatedAt ? new Date(server.updatedAt).toLocaleString("zh-CN") : "-"}
                        </div>
                      </div>
                      <div className="rounded-lg bg-muted/50 px-2.5 py-2">
                        <div className="text-[11px] font-medium text-foreground/80">连接状态</div>
                        <div className="mt-1 text-xs text-muted-foreground">{server.connected ? "正常" : "未连接"}</div>
                      </div>
                    </div>
                    {server.lastConnectError && (
                      <div className="flex items-start gap-2 rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-800">
                        <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
                        <pre className="whitespace-pre-wrap break-words font-mono">{server.lastConnectError}</pre>
                      </div>
                    )}
                  </CardContent>
                </Card>
              )
            })}
          </div>
        )}
      </div>

      <Dialog open={dialogState !== null} onOpenChange={(open) => !open && setDialogState(null)}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>{dialogState?.mode === "edit" ? "编辑 MCP 服务器" : "新建 MCP 服务器"}</DialogTitle>
            <DialogDescription>配置 stdio 子进程、SSE 或 HTTP（Streamable HTTP）远程 MCP 服务器。</DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="mcp-name">名称</Label>
              <Input id="mcp-name" value={name} onChange={(e) => setName(e.target.value)} placeholder="例如 filesystem" />
            </div>
            <Tabs value={transport} onValueChange={(value) => setTransport(value as TransportKind)}>
              <TabsList className="grid w-full grid-cols-3">
                <TabsTrigger value="stdio">stdio</TabsTrigger>
                <TabsTrigger value="sse">SSE</TabsTrigger>
                <TabsTrigger value="http">HTTP</TabsTrigger>
              </TabsList>
            </Tabs>
            {transport === "stdio" ? (
              <>
                <div className="space-y-2">
                  <Label htmlFor="mcp-command">启动命令</Label>
                  <Input id="mcp-command" value={command} onChange={(e) => setCommand(e.target.value)} placeholder="npx" />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="mcp-args">Args（JSON 数组）</Label>
                  <Textarea id="mcp-args" value={argsJson} onChange={(e) => setArgsJson(e.target.value)} rows={3} className="font-mono text-xs" />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="mcp-env">Env（JSON 对象）</Label>
                  <Textarea id="mcp-env" value={envJson} onChange={(e) => setEnvJson(e.target.value)} rows={3} className="font-mono text-xs" />
                </div>
              </>
            ) : (
              <>
                <div className="space-y-2">
                  <Label htmlFor="mcp-url">{transport === "http" ? "HTTP URL" : "SSE URL"}</Label>
                  <Input
                    id="mcp-url"
                    value={url}
                    onChange={(e) => setUrl(e.target.value)}
                    placeholder={transport === "http" ? "https://example.com/mcp" : "https://example.com/mcp/sse"}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="mcp-headers">Headers（JSON 对象，可选）</Label>
                  <Textarea
                    id="mcp-headers"
                    value={headersJson}
                    onChange={(e) => setHeadersJson(e.target.value)}
                    rows={3}
                    className="font-mono text-xs"
                    placeholder='{"Authorization": "Bearer ..."}'
                  />
                </div>
              </>
            )}
            <div className="flex items-center justify-between rounded-lg border border-border/60 px-3 py-2">
              <div>
                <div className="text-sm font-medium">启用</div>
                <div className="text-xs text-muted-foreground">停用后不会加载该服务器的工具</div>
              </div>
              <Switch checked={enabled} onCheckedChange={setEnabled} />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogState(null)}>取消</Button>
            <Button onClick={handleSubmit}>保存</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={toolsDialogServer !== null} onOpenChange={(open) => !open && setToolsDialogServer(null)}>
        <DialogContent className="flex max-h-[80vh] max-w-2xl flex-col overflow-hidden">
          <DialogHeader className="shrink-0">
            <DialogTitle>{toolsDialogServer?.name} 的工具</DialogTitle>
            <DialogDescription>来自 MCP 服务器 `tools/list` 的当前缓存。</DialogDescription>
          </DialogHeader>
          <div className="min-h-0 flex-1 overflow-y-auto overscroll-contain pr-2">
            {toolsLoading ? (
              <div className="space-y-2">
                {Array.from({ length: 4 }).map((_, idx) => (
                  <Skeleton key={idx} className="h-16 rounded-lg" />
                ))}
              </div>
            ) : tools.length === 0 ? (
              <div className="rounded-lg border border-dashed border-border/70 px-4 py-8 text-center text-sm text-muted-foreground">
                暂无工具。请确认服务器已连接并重试。
              </div>
            ) : (
              <div className="space-y-2">
                {tools.map((tool) => (
                  <div key={tool.fullName} className="rounded-lg border border-border/60 bg-muted/20 px-3 py-2">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="text-sm font-medium">{tool.name}</span>
                      <Badge variant="outline" className="font-mono text-[10px]">{tool.fullName}</Badge>
                    </div>
                    {tool.description ? (
                      <p className="mt-1 text-xs text-muted-foreground">{tool.description}</p>
                    ) : null}
                  </div>
                ))}
              </div>
            )}
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}
