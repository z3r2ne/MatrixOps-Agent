import React from "react"
import { Copy, Check, Sparkles, Wrench, Loader2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { ScrollArea } from "@/components/ui/scroll-area"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { AlertDialog, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { cn } from "@/lib/utils"
import { ProjectFilePromptsPanel } from "./ChatDialogs"
import type { SessionContextResponse, SessionPromptResponse, SkillCard } from "@/lib/api"

export interface ChatSessionDialogsProps {
  showSessionInfoDialog: boolean
  setShowSessionInfoDialog: (open: boolean) => void
  sessionContext: SessionContextResponse | null
  sessionContextLoading: boolean
  composerWorkerLabel?: string
  installedSkillsCatalog: SkillCard[]
  loadedSkillsFromChat: SkillCard[]
  handleCopyText: (text: string, successLabel: string) => void
  showContextDialog: boolean
  setShowContextDialog: (open: boolean) => void
  headerStats: Array<{
    label: string
    value: number
    formatter?: (v: number) => string
  }>
  contextLimitTokens: number
  formatByteCount: (value: number) => string
  formatTokenCount: (value: number) => string
  showProjectFilePromptsDialog: boolean
  setShowProjectFilePromptsDialog: (open: boolean) => void
  showPromptDialog: boolean
  setShowPromptDialog: (open: boolean) => void
  currentPrompt: SessionPromptResponse | null
  promptDialogTab: "prompt" | "response" | "history"
  setPromptDialogTab: (tab: "prompt" | "response" | "history") => void
  promptCopied: boolean
  handleCopyPrompt: () => void
  promptDialogHistoryText: string
  showImportConfirm: boolean
  setShowImportConfirm: (open: boolean) => void
  pendingImportFilename: string
  isImportingSession: boolean
  clearPendingImport: () => void
  handleConfirmImport: () => void
}

export const ChatSessionDialogs: React.FC<ChatSessionDialogsProps> = (props) => {
  const {
    showSessionInfoDialog,
    setShowSessionInfoDialog,
    sessionContext,
    sessionContextLoading,
    composerWorkerLabel,
    installedSkillsCatalog,
    loadedSkillsFromChat,
    handleCopyText,
    showContextDialog,
    setShowContextDialog,
    headerStats,
    contextLimitTokens,
    formatByteCount,
    formatTokenCount,
    showProjectFilePromptsDialog,
    setShowProjectFilePromptsDialog,
    showPromptDialog,
    setShowPromptDialog,
    currentPrompt,
    promptDialogTab,
    setPromptDialogTab,
    promptCopied,
    handleCopyPrompt,
    promptDialogHistoryText,
    showImportConfirm,
    setShowImportConfirm,
    pendingImportFilename,
    isImportingSession,
    clearPendingImport,
    handleConfirmImport,
  } = props

  return (
    <>
      <Dialog open={showSessionInfoDialog} onOpenChange={setShowSessionInfoDialog}>
        <DialogContent className="flex max-h-[80vh] w-[min(96vw,72rem)] max-w-5xl min-w-0 flex-col overflow-hidden">
          <DialogHeader>
            <DialogTitle>技能与工具</DialogTitle>
            <DialogDescription>
              当前 Worker：{sessionContext?.workerName || composerWorkerLabel || "未知"}。展示已安装技能、会话中已加载技能，以及当前会话 AI 实际可用的工具。
            </DialogDescription>
          </DialogHeader>
          <div className="grid min-h-0 flex-1 gap-4 overflow-hidden lg:grid-cols-3 md:grid-cols-2">
            <div className="min-h-0 rounded-lg border border-border/60 bg-muted/20 p-3">
              <div className="mb-3 flex items-center gap-2 text-sm font-medium">
                <Sparkles className="h-4 w-4 text-violet-500" />
                已安装技能
                <Badge variant="outline">{installedSkillsCatalog.length}</Badge>
              </div>
              <ScrollArea className="h-[44vh] pr-2">
                <div className="space-y-2">
                  {installedSkillsCatalog.length ? installedSkillsCatalog.map((skill) => (
                    <button
                      key={`${skill.sourceId}-${skill.relativePath}`}
                      type="button"
                      className="flex w-full items-start justify-between gap-3 rounded-md border border-border/60 bg-background px-3 py-2 text-left hover:bg-muted/50"
                      onClick={() => handleCopyText(skill.name, `已复制技能名：${skill.name}`)}
                    >
                      <div className="min-w-0">
                        <div className="truncate text-sm font-medium">{skill.name}</div>
                        {skill.description ? (
                          <div className="mt-1 line-clamp-2 text-xs text-muted-foreground">{skill.description}</div>
                        ) : null}
                      </div>
                      <Copy className="mt-0.5 h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                    </button>
                  )) : (
                    <div className="rounded-md border border-dashed border-border/70 px-3 py-6 text-center text-sm text-muted-foreground">
                      尚未安装任何技能
                    </div>
                  )}
                </div>
              </ScrollArea>
            </div>
            <div className="min-h-0 rounded-lg border border-border/60 bg-muted/20 p-3">
              <div className="mb-3 flex items-center gap-2 text-sm font-medium">
                <Sparkles className="h-4 w-4 text-fuchsia-500" />
                已加载技能
                <Badge variant="outline">{loadedSkillsFromChat.length}</Badge>
              </div>
              <ScrollArea className="h-[44vh] pr-2">
                <div className="space-y-2">
                  {loadedSkillsFromChat.length ? loadedSkillsFromChat.map((skill) => (
                    <button
                      key={skill.callID ? `${skill.name}-${skill.callID}` : skill.name}
                      type="button"
                      className="flex w-full items-start justify-between gap-3 rounded-md border border-border/60 bg-background px-3 py-2 text-left hover:bg-muted/50"
                      onClick={() => handleCopyText(skill.name, `已复制技能名：${skill.name}`)}
                    >
                      <div className="min-w-0">
                        <div className="truncate text-sm font-medium">{skill.name}</div>
                      </div>
                      <Copy className="mt-0.5 h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                    </button>
                  )) : (
                    <div className="rounded-md border border-dashed border-border/70 px-3 py-6 text-center text-sm text-muted-foreground">
                      聊天记录中尚未发现成功执行的 `load_skill` 调用
                    </div>
                  )}
                </div>
              </ScrollArea>
            </div>
            <div className="min-h-0 rounded-lg border border-border/60 bg-muted/20 p-3 md:col-span-2 lg:col-span-1">
              <div className="mb-3 flex items-center gap-2 text-sm font-medium">
                <Wrench className="h-4 w-4 text-sky-500" />
                可用工具
                <Badge variant="outline">{sessionContext?.tools.length || 0}</Badge>
              </div>
              <ScrollArea className="h-[44vh] pr-2">
                <div className="space-y-2">
                  {(sessionContext?.tools || []).map((tool) => (
                    <button
                      key={tool.name}
                      type="button"
                      className="flex w-full items-start justify-between gap-3 rounded-md border border-border/60 bg-background px-3 py-2 text-left hover:bg-muted/50"
                      onClick={() => handleCopyText(tool.name, `已复制工具名：${tool.name}`)}
                    >
                      <div className="min-w-0">
                        <div className="flex flex-wrap items-center gap-2">
                          <span className="truncate text-sm font-medium">{tool.verbosName || tool.name}</span>
                          {tool.isMcp ? (
                            <Badge variant="secondary" className="text-[10px]">MCP{tool.mcpServer ? ` · ${tool.mcpServer}` : ""}</Badge>
                          ) : null}
                        </div>
                        <div className="mt-0.5 truncate font-mono text-[11px] text-muted-foreground">{tool.name}</div>
                        {tool.description ? (
                          <div className="mt-1 line-clamp-2 text-xs text-muted-foreground">{tool.description}</div>
                        ) : null}
                      </div>
                      <Copy className="mt-0.5 h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                    </button>
                  ))}
                  {!sessionContext?.tools.length ? (
                    <div className="rounded-md border border-dashed border-border/70 px-3 py-6 text-center text-sm text-muted-foreground">
                      {sessionContextLoading ? "加载中…" : "暂无可用工具"}
                    </div>
                  ) : null}
                </div>
              </ScrollArea>
            </div>
          </div>
        </DialogContent>
      </Dialog>

      <Dialog open={showContextDialog} onOpenChange={setShowContextDialog}>
        <DialogContent className="flex max-h-[80vh] w-[min(96vw,56rem)] max-w-3xl min-w-0 flex-col overflow-hidden">
          <DialogHeader>
            <DialogTitle>当前上下文组成</DialogTitle>
            <DialogDescription>
              总大小约 {formatByteCount(sessionContext?.totalBytes || 0)}，按 UTF-8 字节估算。
              {contextLimitTokens > 0 ? ` 当前可用上下文窗口约 ${formatTokenCount(contextLimitTokens)} tokens。` : ""}
            </DialogDescription>
          </DialogHeader>
          {headerStats.length > 0 ? (
            <div className="mb-3 rounded-lg border border-border/60 bg-muted/25 px-3 py-2.5">
              <div className="mb-2 text-xs font-medium text-muted-foreground">用量概览</div>
              <div className="flex flex-wrap gap-2">
                {headerStats.map((stat) => (
                  <span
                    key={`ctx-stat-${stat.label}-${stat.value}`}
                    className="rounded-full border border-border/70 bg-background/80 px-2.5 py-0.5 text-[11px] font-medium text-foreground"
                  >
                    {stat.label}{" "}
                    {"formatter" in stat && stat.formatter ? stat.formatter(stat.value) : formatTokenCount(stat.value)}
                  </span>
                ))}
              </div>
            </div>
          ) : null}
          <ScrollArea className="min-h-0 flex-1 pr-2">
            <div className="space-y-3">
              {(sessionContext?.components || [])
                .filter((component) => component.bytes > 0)
                .sort((left, right) => right.bytes - left.bytes)
                .map((component) => (
                  <div key={component.key} className="rounded-lg border border-border/60 bg-background px-3 py-3">
                    <div className="mb-2 flex items-center justify-between gap-3">
                      <div className="text-sm font-medium">{component.label}</div>
                      <div className="text-xs text-muted-foreground">
                        {formatByteCount(component.bytes)} · {component.percent.toFixed(component.percent >= 10 ? 0 : 1)}%
                      </div>
                    </div>
                    <div className="h-2 overflow-hidden rounded-full bg-muted">
                      <div
                        className="h-full rounded-full bg-primary/70"
                        style={{ width: `${Math.max(component.percent, component.bytes > 0 ? 2 : 0)}%` }}
                      />
                    </div>
                  </div>
                ))}
            </div>
          </ScrollArea>
        </DialogContent>
      </Dialog>

      {/* 关联提示词弹窗 */}
      <Dialog open={showProjectFilePromptsDialog} onOpenChange={setShowProjectFilePromptsDialog}>
        <DialogContent className="flex max-h-[85vh] w-[min(96vw,64rem)] max-w-4xl min-w-0 flex-col overflow-hidden">
          <DialogHeader>
            <DialogTitle>当前会话关联的提示词文件</DialogTitle>
            <DialogDescription>
              这些文件（如 AGENTS.md、CLAUDE.md）被自动收集作为会话上下文的一部分。
            </DialogDescription>
          </DialogHeader>
          {(!sessionContext?.projectFilePrompts || sessionContext.projectFilePrompts.length === 0) ? (
            <div className="flex items-center justify-center rounded-md border border-dashed border-border/70 px-4 py-12 text-sm text-muted-foreground">
              当前会话没有关联的提示词文件
            </div>
          ) : (
            <ProjectFilePromptsPanel prompts={sessionContext.projectFilePrompts} />
          )}
        </DialogContent>
      </Dialog>

      <Dialog open={showPromptDialog} onOpenChange={setShowPromptDialog}>
        <DialogContent className="flex max-h-[80vh] w-[min(96vw,72rem)] max-w-4xl min-w-0 flex-col overflow-hidden">
          <DialogHeader>
            <div className="flex items-center justify-between gap-4">
              <div className="min-w-0">
                <DialogTitle>当前 Prompt</DialogTitle>
                <DialogDescription>
                  {currentPrompt?.sessionId || "当前任务最近一次实际生成的 Prompt"}
                </DialogDescription>
              </div>
              <Button
                variant="ghost"
                size="sm"
                onClick={handleCopyPrompt}
                disabled={
                  (promptDialogTab === "prompt" && !currentPrompt?.prompt) ||
                  (promptDialogTab === "response" && !currentPrompt?.rawResponse) ||
                  (promptDialogTab === "history" && !promptDialogHistoryText.trim())
                }
              >
                {promptCopied ? <Check className="mr-1 h-4 w-4 text-emerald-500" /> : <Copy className="mr-1 h-4 w-4" />}
                {promptDialogTab === "history" ? "复制聊天记录" : "复制"}
              </Button>
            </div>
          </DialogHeader>
          <Tabs value={promptDialogTab} onValueChange={(value) => setPromptDialogTab(value as "prompt" | "response" | "history")} className="w-full min-w-0">
            <TabsList className={cn("grid w-full", currentPrompt?.rawResponse ? "grid-cols-3" : "grid-cols-2")}>
              <TabsTrigger value="prompt">原始 Prompt</TabsTrigger>
              {currentPrompt?.rawResponse ? <TabsTrigger value="response">原始响应</TabsTrigger> : null}
              <TabsTrigger value="history">聊天记录</TabsTrigger>
            </TabsList>
            <TabsContent value="prompt" className="mt-4">
              <ScrollArea className="h-[55vh] min-w-0 rounded-md border border-border bg-muted/30">
                <pre className="max-w-full whitespace-pre-wrap p-4 text-xs leading-relaxed break-words [overflow-wrap:anywhere]">
                  {currentPrompt?.prompt || "暂无 Prompt"}
                </pre>
              </ScrollArea>
            </TabsContent>
            {currentPrompt?.rawResponse ? (
              <TabsContent value="response" className="mt-4">
                <ScrollArea className="h-[55vh] min-w-0 rounded-md border border-border bg-muted/30">
                  <pre className="max-w-full whitespace-pre-wrap p-4 text-xs leading-relaxed break-words [overflow-wrap:anywhere]">
                    {currentPrompt.rawResponse}
                  </pre>
                </ScrollArea>
              </TabsContent>
            ) : null}
            <TabsContent value="history" className="mt-4">
              <ScrollArea className="h-[55vh] min-w-0 rounded-md border border-border bg-muted/30">
                <pre className="max-w-full whitespace-pre-wrap p-4 text-xs leading-relaxed break-words [overflow-wrap:anywhere]">
                  {promptDialogHistoryText || "暂无聊天记录"}
                </pre>
              </ScrollArea>
            </TabsContent>
          </Tabs>
        </DialogContent>
      </Dialog>

      <AlertDialog open={showImportConfirm} onOpenChange={(open) => !open && clearPendingImport()}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>覆盖导入当前会话？</AlertDialogTitle>
            <AlertDialogDescription>
              <div className="space-y-3 text-sm text-muted-foreground">
                <p>导入会覆盖当前会话已有的聊天记录和记忆，这个操作不会保留当前旧内容。</p>
                <div className="rounded-md border border-border/70 bg-muted/30 px-3 py-2 text-xs">
                  <div>文件：{pendingImportFilename || "未选择"}</div>
                  <div>文件：{pendingImportFilename || "未选择"}</div>
                </div>
              </div>
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isImportingSession}>取消</AlertDialogCancel>
            <Button type="button" onClick={handleConfirmImport} disabled={isImportingSession}>
              {isImportingSession ? "导入中..." : "确认覆盖导入"}
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
