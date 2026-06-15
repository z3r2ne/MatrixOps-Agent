import React, { useState, useEffect, useMemo } from "react"
import { ScrollArea } from "@/components/ui/scroll-area"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { FileCode, Loader2, Copy, Check } from "lucide-react"
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { cn } from "@/lib/utils"
import { api, type Memory } from "@/lib/api"
import { MarkdownRenderer } from "../MarkdownRenderer"
import { filePreviewURL, basename, type ProjectFilePromptItem } from "./chat-utils"

export const ProjectFilePromptsPanel: React.FC<{ prompts?: ProjectFilePromptItem[] }> = ({ prompts }) => {
  const items = useMemo(() => (prompts ?? []).filter((p) => p && String(p.path || "").trim()), [prompts])
  const [activePath, setActivePath] = useState<string>(() => items[0]?.path ?? "")

  useEffect(() => {
    setActivePath(items[0]?.path ?? "")
  }, [items])

  const activeItem = useMemo(() => items.find((it) => it.path === activePath) ?? items[0], [items, activePath])

  if (!items.length) return null

  return (
    <div className="mt-3 grid min-h-0 flex-1 grid-cols-1 gap-3 overflow-hidden md:grid-cols-[18rem_1fr]">
      <ScrollArea className="min-h-0 rounded-md border border-border/70 bg-muted/20">
        <div className="p-2">
          {items.map((it) => {
            const selected = it.path === activeItem?.path
            return (
              <button
                key={it.path}
                type="button"
                onClick={() => setActivePath(it.path)}
                className={cn(
                  "w-full rounded-md px-2.5 py-2 text-left transition",
                  "hover:bg-muted/60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/40",
                  selected ? "bg-muted text-foreground" : "text-muted-foreground",
                )}
              >
                <div className={cn("truncate text-sm", selected ? "font-medium" : "font-normal")}>
                  {basename(it.path) || it.path}
                </div>
                <div className="mt-0.5 truncate text-[11px] text-muted-foreground/90">{it.path}</div>
              </button>
            )
          })}
        </div>
      </ScrollArea>

      <div className="min-h-0 overflow-hidden rounded-md border border-border/70 bg-muted/20">
        <div className="border-b border-border/70 px-3 py-2">
          <div className="truncate text-sm font-medium text-foreground">{basename(activeItem?.path || "") || "文件预览"}</div>
          <div className="truncate text-[11px] text-muted-foreground">{activeItem?.path}</div>
        </div>
        <ScrollArea className="h-[60vh] min-w-0">
          <pre className="max-w-full whitespace-pre-wrap p-4 text-xs leading-relaxed break-words [overflow-wrap:anywhere]">
            {activeItem?.prompt?.trim() ? activeItem.prompt : "该文件没有提示词内容"}
          </pre>
        </ScrollArea>
      </div>
    </div>
  )
}

/** 仅从尾部若干条消息收集「执行中」工具，避免长会话全表扫描 */

export const MemoryDialog: React.FC<{ 
  memory: Memory | null
  prompt?: string
  rawResponse?: string
  loading?: boolean
  open: boolean
  onOpenChange: (open: boolean) => void 
}> = ({
  memory,
  prompt,
  rawResponse,
  loading = false,
  open,
  onOpenChange,
}) => {
  const [copied, setCopied] = useState(false)
  const [activeTab, setActiveTab] = useState<string>("prompts")

  useEffect(() => {
    if (!open) return
    if (prompt?.trim()) {
      setActiveTab("raw-prompt")
      return
    }
    if (rawResponse?.trim()) {
      setActiveTab("raw-response")
      return
    }
    setActiveTab("prompts")
  }, [open, prompt, rawResponse])
  
  const handleCopyAll = () => {
    if (activeTab === "raw-prompt" && prompt) {
      navigator.clipboard.writeText(prompt)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
      return
    }
    if (activeTab === "raw-response" && rawResponse) {
      navigator.clipboard.writeText(rawResponse)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
      return
    }
    
    if (!memory) return
    const text = [
      memory.globalPrompt && `# 全局提示词\n${memory.globalPrompt}`,
      memory.occupationPrompt && `# 职业提示词\n${memory.occupationPrompt}`,
      memory.projectPrompt && `# 项目提示词\n${memory.projectPrompt}`,
      memory.workerPrompt && `# Worker提示词\n${memory.workerPrompt}`,
      memory.modelPrompt && `# 模型提示词\n${memory.modelPrompt}`,
      memory.envPrompt && `# 环境信息\n${memory.envPrompt}`,
      memory.projectFilePrompt && memory.projectFilePrompt.length > 0 && 
        `# 项目文件提示词\n${memory.projectFilePrompt.map(f => `${f.path}:\n${f.prompt}`).join('\n\n')}`
    ].filter(Boolean).join('\n\n')
    
    navigator.clipboard.writeText(text)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  if (!loading && !memory && !prompt) return null

  const hasContent = (str?: string) => str && str.trim().length > 0
  const hasFilePrompts = memory?.projectFilePrompt && memory.projectFilePrompt.length > 0
  const hasPrompt = prompt && prompt.trim().length > 0
  const hasRawResponse = rawResponse && rawResponse.trim().length > 0
  const visibleTabCount =
    (memory ? 1 : 0) +
    (hasFilePrompts ? 1 : 0) +
    (hasPrompt ? 1 : 0) +
    (hasRawResponse ? 1 : 0)
  const tabsGridClass =
    visibleTabCount >= 4 ? "grid-cols-4" :
    visibleTabCount === 3 ? "grid-cols-3" :
    visibleTabCount === 2 ? "grid-cols-2" :
    "grid-cols-1"

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[80vh] w-[min(96vw,72rem)] max-w-4xl min-w-0 flex-col overflow-hidden">
        <DialogHeader>
          <div className="flex items-center justify-between">
            <DialogTitle>提示词信息</DialogTitle>
            <Button
              variant="ghost"
              size="sm"
              onClick={handleCopyAll}
              className="h-8"
            >
              {copied ? <Check className="h-4 w-4 mr-1 text-emerald-500" /> : <Copy className="h-4 w-4 mr-1" />}
              复制全部
            </Button>
          </div>
        </DialogHeader>
        
        <Tabs value={activeTab} onValueChange={setActiveTab} className="w-full min-w-0">
          <TabsList className={cn("grid w-full", tabsGridClass)}>
            {memory && <TabsTrigger value="prompts">提示词</TabsTrigger>}
            {memory && <TabsTrigger value="files" disabled={!hasFilePrompts}>文件提示词 {hasFilePrompts && `(${memory.projectFilePrompt?.length})`}</TabsTrigger>}
            {hasPrompt && <TabsTrigger value="raw-prompt">原始 Prompt</TabsTrigger>}
            {hasRawResponse && <TabsTrigger value="raw-response">原始响应</TabsTrigger>}
          </TabsList>

          <ScrollArea className="mt-4 h-[50vh] min-w-0">
            {loading && !prompt && (
              <div className="flex h-[50vh] items-center justify-center text-sm text-muted-foreground">
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                正在加载提示词...
              </div>
            )}
            {memory && <TabsContent value="prompts" className="space-y-4 mt-0">
              {hasContent(memory.globalPrompt) && (
                <div className="min-w-0 space-y-2">
                  <h4 className="text-sm font-semibold text-foreground flex items-center gap-2">
                    <div className="h-1 w-1 rounded-full bg-blue-500" />
                    全局提示词
                  </h4>
                  <div className="min-w-0 overflow-hidden rounded-md border border-border bg-muted/50 p-3 text-xs leading-relaxed">
                    <pre className="max-w-full whitespace-pre-wrap font-mono break-words [overflow-wrap:anywhere]">{memory.globalPrompt}</pre>
                  </div>
                </div>
              )}

              {hasContent(memory.occupationPrompt) && (
                <div className="min-w-0 space-y-2">
                  <h4 className="text-sm font-semibold text-foreground flex items-center gap-2">
                    <div className="h-1 w-1 rounded-full bg-purple-500" />
                    职业提示词
                  </h4>
                  <div className="min-w-0 overflow-hidden rounded-md border border-border bg-muted/50 p-3 text-xs leading-relaxed">
                    <pre className="max-w-full whitespace-pre-wrap font-mono break-words [overflow-wrap:anywhere]">{memory.occupationPrompt}</pre>
                  </div>
                </div>
              )}

              {hasContent(memory.projectPrompt) && (
                <div className="min-w-0 space-y-2">
                  <h4 className="text-sm font-semibold text-foreground flex items-center gap-2">
                    <div className="h-1 w-1 rounded-full bg-emerald-500" />
                    项目提示词
                  </h4>
                  <div className="min-w-0 overflow-hidden rounded-md border border-border bg-muted/50 p-3 text-xs leading-relaxed">
                    <pre className="max-w-full whitespace-pre-wrap font-mono break-words [overflow-wrap:anywhere]">{memory.projectPrompt}</pre>
                  </div>
                </div>
              )}
            {hasContent(memory.modelPrompt) && (
                <div className="min-w-0 space-y-2">
                  <h4 className="text-sm font-semibold text-foreground flex items-center gap-2">
                    <div className="h-1 w-1 rounded-full bg-amber-500" />
                    模型提示词
                  </h4>
                  <div className="min-w-0 overflow-hidden rounded-md border border-border bg-muted/50 p-3 text-xs leading-relaxed">
                    <pre className="max-w-full whitespace-pre-wrap font-mono break-words [overflow-wrap:anywhere]">{memory.modelPrompt}</pre>
                  </div>
                </div>
              )}
              {hasContent(memory.workerPrompt) && (
                <div className="min-w-0 space-y-2">
                  <h4 className="text-sm font-semibold text-foreground flex items-center gap-2">
                    <div className="h-1 w-1 rounded-full bg-amber-500" />
                    Worker提示词
                  </h4>
                  <div className="min-w-0 overflow-hidden rounded-md border border-border bg-muted/50 p-3 text-xs leading-relaxed">
                    <pre className="max-w-full whitespace-pre-wrap font-mono break-words [overflow-wrap:anywhere]">{memory.workerPrompt}</pre>
                  </div>
                </div>
              )}

              {hasContent(memory.envPrompt) && (
                <div className="min-w-0 space-y-2">
                  <h4 className="text-sm font-semibold text-foreground flex items-center gap-2">
                    <div className="h-1 w-1 rounded-full bg-cyan-500" />
                    环境信息
                  </h4>
                  <div className="min-w-0 overflow-hidden rounded-md border border-border bg-muted/50 p-3 text-xs leading-relaxed">
                    <pre className="max-w-full whitespace-pre-wrap font-mono break-words [overflow-wrap:anywhere]">{memory.envPrompt}</pre>
                  </div>
                </div>
              )}

              {!hasContent(memory.globalPrompt) && 
               !hasContent(memory.occupationPrompt) && 
               !hasContent(memory.projectPrompt) && 
               !hasContent(memory.workerPrompt) && 
               !hasContent(memory.envPrompt) && (
                <div className="text-center text-sm text-muted-foreground py-8">
                  暂无提示词信息
                </div>
              )}
            </TabsContent>}

            {memory && <TabsContent value="files" className="space-y-3 mt-0">
              {hasFilePrompts ? (
                memory.projectFilePrompt?.map((file, idx) => (
                  <div key={idx} className="min-w-0 space-y-2">
                    <h4 className="text-sm font-semibold text-foreground flex items-center gap-2">
                      <FileCode className="h-4 w-4 text-blue-500" />
                      {file.path}
                    </h4>
                    <div className="ml-6 min-w-0 overflow-hidden rounded-md border border-border bg-muted/50 p-3 text-xs leading-relaxed">
                      <pre className="max-w-full whitespace-pre-wrap font-mono break-words [overflow-wrap:anywhere]">{file.prompt}</pre>
                    </div>
                  </div>
                ))
              ) : (
                <div className="text-center text-sm text-muted-foreground py-8">
                  暂无文件提示词
                </div>
              )}
            </TabsContent>}

            {hasPrompt && <TabsContent value="raw-prompt" className="mt-0">
              <div className="min-w-0 overflow-hidden rounded-md border border-border bg-muted/50 p-3 text-xs leading-relaxed">
                <pre className="max-w-full whitespace-pre-wrap font-mono break-words [overflow-wrap:anywhere]">{prompt}</pre>
              </div>
            </TabsContent>}

            {hasRawResponse && <TabsContent value="raw-response" className="mt-0">
              <div className="min-w-0 overflow-hidden rounded-md border border-border bg-muted/50 p-3 text-xs leading-relaxed">
                <pre className="max-w-full whitespace-pre-wrap font-mono break-words [overflow-wrap:anywhere]">{rawResponse}</pre>
              </div>
            </TabsContent>}
          </ScrollArea>
        </Tabs>
      </DialogContent>
    </Dialog>
  )
}

// User file / image in history bubble
