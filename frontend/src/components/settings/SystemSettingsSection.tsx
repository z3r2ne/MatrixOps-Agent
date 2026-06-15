import React, { useEffect, useMemo, useState } from "react"
import { Settings, CheckCircle2, AlertCircle, Database, Braces, Clock, Hash, Repeat, ChevronDown, Archive } from "lucide-react"
import { cn } from "@/lib/utils"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Combobox, type ComboboxOption } from "@/components/ui/combobox"
import { Label } from "@/components/ui/label"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Button } from "@/components/ui/button"
import { api, EditorInfo, LLMConfig, ShellInfo, ToolInfo } from "@/lib/api"
import { useNotification } from "@/components/NotificationProvider"
import { ProjectToolPermissionPanel } from "@/components/projects/ProjectToolPermissionPanel"
import {
  DEFAULT_TASK_LIST_GROUP_MODE,
  DEFAULT_TASK_LIST_GROUP_MODE_CONFIG_KEY,
  TASK_GROUP_MODE_OPTIONS,
  normalizeTaskListGroupMode,
} from "@/lib/taskGrouping"
import {
  DEFAULT_PROJECT_TOOL_PERMISSIONS_CONFIG_KEY,
  parseProjectToolPermissions,
  serializeProjectToolPermissions,
} from "@/lib/projectToolPermissions"

export function SystemSettingsSection() {
  const [editors, setEditors] = useState<EditorInfo[]>([])
  const [defaultEditor, setDefaultEditor] = useState<string>("")
  const [customCommand, setCustomCommand] = useState<string>("")
  const [shellOptions, setShellOptions] = useState<ShellInfo[]>([])
  const [currentShell, setCurrentShell] = useState<ShellInfo | null>(null)
  const [defaultShell, setDefaultShell] = useState<string>("")
  const [customShellCommand, setCustomShellCommand] = useState<string>("")
  const [defaultTaskListGroupMode, setDefaultTaskListGroupMode] = useState(DEFAULT_TASK_LIST_GROUP_MODE)
  const [toolInfos, setToolInfos] = useState<ToolInfo[]>([])
  const [defaultProjectToolPermissions, setDefaultProjectToolPermissions] = useState<Record<string, string>>({})
  const [llmConfigs, setLlmConfigs] = useState<LLMConfig[]>([])
  const [defaultLLMConfigId, setDefaultLLMConfigId] = useState<number | null>(null)
  const [llmCustomHeadersText, setLlmCustomHeadersText] = useState("")
  const [llmHttpTimeoutSeconds, setLlmHttpTimeoutSeconds] = useState("")
  const [llmHttpConnectTimeoutSeconds, setLlmHttpConnectTimeoutSeconds] = useState("")
  const [llmMaxOutputTokens, setLlmMaxOutputTokens] = useState("")
  const [agentMaxSteps, setAgentMaxSteps] = useState("")
  const [memoryCompactionThresholdPercent, setMemoryCompactionThresholdPercent] = useState("")
  const [memoryCompactionTargetPercent, setMemoryCompactionTargetPercent] = useState("")
  const [memoryCompactionL2ScopePercent, setMemoryCompactionL2ScopePercent] = useState("")
  const [projectPermissionsExpanded, setProjectPermissionsExpanded] = useState(false)
  const [loading, setLoading] = useState(true)
  const { notify } = useNotification()

  const AGENT_MAX_STEPS_KEY = "agent_max_steps"
  const DEFAULT_AGENT_MAX_STEPS = 1000
  const MEMORY_COMPACTION_TRIGGER_THRESHOLD_KEY = "memory_compaction_trigger_threshold_percent"
  const MEMORY_COMPACTION_TARGET_PERCENT_KEY = "memory_compaction_target_percent"
  const MEMORY_COMPACTION_L2_SCOPE_PERCENT_KEY = "memory_compaction_l2_scope_percent"
  const DEFAULT_MEMORY_COMPACTION_TRIGGER_THRESHOLD = 80
  const DEFAULT_MEMORY_COMPACTION_TARGET_PERCENT = 60
  const DEFAULT_MEMORY_COMPACTION_L2_SCOPE_PERCENT = 80
  const LLM_CUSTOM_HEADERS_KEY = "llm_custom_headers"
  const LLM_HTTP_TIMEOUT_SECONDS_KEY = "llm_http_timeout_seconds"
  const LLM_HTTP_CONNECT_TIMEOUT_SECONDS_KEY = "llm_http_connect_timeout_seconds"
  const LLM_MAX_OUTPUT_TOKENS_KEY = "llm_max_output_tokens"

  const editorOptions = useMemo<ComboboxOption[]>(() => {
    const options = editors.map((editor) => ({
      value: editor.id,
      label: editor.name,
      description: editor.isAvailable ? "已检测到" : "未检测到",
      searchText: `${editor.id} ${editor.name} ${editor.isAvailable ? "available 已检测到" : "missing 未检测到"}`,
    }))
    if (!options.some((item) => item.value === "custom")) {
      options.push({
        value: "custom",
        label: "自定义命令",
        description: "手动输入编辑器命令",
        searchText: "custom 自定义 编辑器 命令",
      })
    }
    return options
  }, [editors])

  const shellComboboxOptions = useMemo<ComboboxOption[]>(() => {
    const options = shellOptions.map((shell) => ({
      value: shell.id,
      label: shell.name,
      description: shell.isAvailable ? (shell.path || "已检测到") : "未检测到",
      searchText: `${shell.id} ${shell.name} ${shell.path || ""} ${shell.isAvailable ? "available 已检测到" : "missing 未检测到"}`,
    }))
    if (!options.some((item) => item.value === "custom")) {
      options.push({
        value: "custom",
        label: "自定义",
        description: "手动输入 shell 命令",
        searchText: "custom 自定义 shell",
      })
    }
    return options
  }, [shellOptions])

  const llmConfigOptions = useMemo<ComboboxOption[]>(
    () =>
      llmConfigs.map((config) => ({
        value: config.id.toString(),
        label: config.name,
        description: config.type,
        searchText: `${config.name} ${config.type} ${config.model || ""}`,
      })),
    [llmConfigs]
  )

  const loadSettings = async () => {
    setLoading(true)
    try {
      const [
        editorsData,
        configData,
        customCmdData,
        shellInfoData,
        defaultShellData,
        customShellData,
        toolsData,
        llmConfigsData,
        defaultLLMData,
        defaultTaskGroupModeConfig,
        defaultProjectToolPermissionsConfig,
        llmHeadersConfig,
        llmTimeoutConfig,
        llmConnectTimeoutConfig,
        llmMaxOutConfig,
        agentMaxStepsConfig,
        memoryCompactionThresholdConfig,
        memoryCompactionTargetConfig,
        memoryCompactionL2ScopeConfig,
      ] = await Promise.all([
        api.getEditors(),
        api.getConfig("default_editor", { skipErrorNotification: true }).catch(() => null),
        api.getConfig("custom_editor_command", { skipErrorNotification: true }).catch(() => null),
        api.getCurrentShell().catch(() => null),
        api.getConfig("default_shell", { skipErrorNotification: true }).catch(() => null),
        api.getConfig("custom_shell_command", { skipErrorNotification: true }).catch(() => null),
        api.getTools().catch(() => []),
        api.getLLMConfigs(),
        api.getDefaultLLMConfig().catch(() => null),
        api.getConfig(DEFAULT_TASK_LIST_GROUP_MODE_CONFIG_KEY, { skipErrorNotification: true }).catch(() => null),
        api.getConfig(DEFAULT_PROJECT_TOOL_PERMISSIONS_CONFIG_KEY, { skipErrorNotification: true }).catch(() => null),
        api.getConfig(LLM_CUSTOM_HEADERS_KEY, { skipErrorNotification: true }).catch(() => null),
        api.getConfig(LLM_HTTP_TIMEOUT_SECONDS_KEY, { skipErrorNotification: true }).catch(() => null),
        api.getConfig(LLM_HTTP_CONNECT_TIMEOUT_SECONDS_KEY, { skipErrorNotification: true }).catch(() => null),
        api.getConfig(LLM_MAX_OUTPUT_TOKENS_KEY, { skipErrorNotification: true }).catch(() => null),
        api.getConfig(AGENT_MAX_STEPS_KEY, { skipErrorNotification: true }).catch(() => null),
        api.getConfig(MEMORY_COMPACTION_TRIGGER_THRESHOLD_KEY, { skipErrorNotification: true }).catch(() => null),
        api.getConfig(MEMORY_COMPACTION_TARGET_PERCENT_KEY, { skipErrorNotification: true }).catch(() => null),
        api.getConfig(MEMORY_COMPACTION_L2_SCOPE_PERCENT_KEY, { skipErrorNotification: true }).catch(() => null),
      ])
      setEditors(editorsData)
      if (configData) {
        setDefaultEditor(configData.value)
      } else if (editorsData.length > 0) {
        // ... (same as before)
      }
      if (customCmdData) {
        setCustomCommand(customCmdData.value)
      }
      setShellOptions(shellInfoData?.options || [])
      setCurrentShell(shellInfoData?.current || null)
      setDefaultShell(defaultShellData?.value || shellInfoData?.current?.id || "")
      setCustomShellCommand(customShellData?.value || "")
      setDefaultTaskListGroupMode(normalizeTaskListGroupMode(defaultTaskGroupModeConfig?.value))
      setToolInfos(Array.isArray(toolsData) ? toolsData : [])
      setDefaultProjectToolPermissions(parseProjectToolPermissions(defaultProjectToolPermissionsConfig?.value))
      setLlmConfigs(llmConfigsData)
      if (defaultLLMData) {
        setDefaultLLMConfigId(defaultLLMData.id)
      }
      setLlmCustomHeadersText(llmHeadersConfig?.value ?? "")
      setLlmHttpTimeoutSeconds(llmTimeoutConfig?.value?.trim() ?? "")
      setLlmHttpConnectTimeoutSeconds(llmConnectTimeoutConfig?.value?.trim() ?? "")
      setLlmMaxOutputTokens(llmMaxOutConfig?.value?.trim() ?? "")
      setAgentMaxSteps(agentMaxStepsConfig?.value?.trim() ?? "")
      setMemoryCompactionThresholdPercent(memoryCompactionThresholdConfig?.value?.trim() ?? "")
      setMemoryCompactionTargetPercent(memoryCompactionTargetConfig?.value?.trim() ?? "")
      setMemoryCompactionL2ScopePercent(memoryCompactionL2ScopeConfig?.value?.trim() ?? "")
    } catch (error) {
      console.error("加载系统设置失败", error)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadSettings()
  }, [])

  const handleEditorChange = async (value: string) => {
    try {
      await api.updateConfig("default_editor", value)
      setDefaultEditor(value)
      notify({ type: "success", title: "设置已更新", description: "默认编辑器已保存" })
    } catch (error) {
      notify({ type: "error", title: "更新失败", description: error instanceof Error ? error.message : "未知错误" })
    }
  }

  const handleDefaultLLMChange = async (value: string) => {
    try {
      const configId = parseInt(value)
      await api.setDefaultLLMConfig(configId)
      setDefaultLLMConfigId(configId)
      notify({ type: "success", title: "设置已更新", description: "默认 Provider 已保存" })
    } catch (error) {
      notify({ type: "error", title: "更新失败", description: error instanceof Error ? error.message : "未知错误" })
    }
  }

  const handleShellChange = async (value: string) => {
    try {
      await api.updateConfig("default_shell", value)
      setDefaultShell(value)
      notify({ type: "success", title: "设置已更新", description: "默认 shell 已保存" })
    } catch (error) {
      notify({ type: "error", title: "更新失败", description: error instanceof Error ? error.message : "未知错误" })
    }
  }

  const handleDefaultTaskListGroupModeChange = async (value: string) => {
    const nextMode = normalizeTaskListGroupMode(value)
    try {
      await api.updateConfig(DEFAULT_TASK_LIST_GROUP_MODE_CONFIG_KEY, nextMode)
      setDefaultTaskListGroupMode(nextMode)
      notify({ type: "success", title: "设置已更新", description: "任务列表默认分组方式已保存" })
    } catch (error) {
      notify({ type: "error", title: "更新失败", description: error instanceof Error ? error.message : "未知错误" })
    }
  }

  const handleSaveDefaultProjectToolPermissions = async () => {
    try {
      await api.updateConfig(
        DEFAULT_PROJECT_TOOL_PERMISSIONS_CONFIG_KEY,
        serializeProjectToolPermissions(defaultProjectToolPermissions)
      )
      notify({ type: "success", title: "设置已更新", description: "新建项目默认权限已保存" })
    } catch (error) {
      notify({ type: "error", title: "更新失败", description: error instanceof Error ? error.message : "未知错误" })
    }
  }

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <Settings className="h-5 w-5 text-primary" />
            <CardTitle>常规设置</CardTitle>
          </div>  
          <CardDescription>配置系统的全局行为和工具偏好</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="default-editor">默认编辑器</Label>
            <Combobox
              id="default-editor"
              items={editorOptions}
              value={defaultEditor}
              onValueChange={handleEditorChange}
              placeholder="选择编辑器"
              searchPlaceholder="搜索编辑器"
              emptyText="未找到编辑器"
              disabled={loading}
              className="w-full md:w-[300px]"
              renderItem={(item) => {
                const editor = editors.find((candidate) => candidate.id === item.value)
                return (
                  <div className="flex items-center justify-between gap-4">
                    <span>{item.label}</span>
                    {editor?.isAvailable ? (
                      <CheckCircle2 className="h-3 w-3 text-emerald-500" />
                    ) : editor ? (
                      <AlertCircle className="h-3 w-3 text-muted-foreground/50" />
                    ) : null}
                  </div>
                )
              }}
            />
            <p className="text-xs text-muted-foreground">
              选择用于打开项目目录的编辑器。带有绿色图标的编辑器已在系统中检测到。
            </p>
          </div>

          {defaultEditor === "custom" && (
            <div className="space-y-2 pt-2 border-t">
              <Label htmlFor="custom-command">自定义启动命令</Label>
              <div className="flex flex-col gap-2 sm:flex-row">
                <Input
                  id="custom-command"
                  value={customCommand}
                  onChange={(e) => setCustomCommand(e.target.value)}
                  placeholder="例如: /usr/local/bin/my-editor"
                  className="w-full md:w-[400px]"
                />
                <Button
                  className="w-full sm:w-auto"
                  onClick={async () => {
                    try {
                      await api.updateConfig("custom_editor_command", customCommand)
                      notify({ type: "success", title: "保存成功", description: "自定义命令已更新" })
                    } catch (error) {
                      notify({ type: "error", title: "保存失败", description: error instanceof Error ? error.message : "未知错误" })
                    }
                  }}
                >
                  保存
                </Button>
              </div>
              <p className="text-xs text-muted-foreground">
                请输入编辑器的可执行文件路径或系统命令。
              </p>
            </div>
          )}

          <div className="space-y-2 pt-2 border-t">
            <Label htmlFor="default-shell">默认 Shell</Label>
            <Combobox
              id="default-shell"
              items={shellComboboxOptions}
              value={defaultShell}
              onValueChange={handleShellChange}
              placeholder="选择 shell"
              searchPlaceholder="搜索 shell"
              emptyText="未找到 shell"
              disabled={loading}
              className="w-full md:w-[300px]"
              renderItem={(item) => {
                const shell = shellOptions.find((candidate) => candidate.id === item.value)
                return (
                  <div className="flex items-center justify-between gap-4">
                    <span>{item.label}</span>
                    {shell?.isAvailable ? (
                      <CheckCircle2 className="h-3 w-3 text-emerald-500" />
                    ) : shell ? (
                      <AlertCircle className="h-3 w-3 text-muted-foreground/50" />
                    ) : null}
                  </div>
                )
              }}
            />
            <p className="text-xs text-muted-foreground">
              当前系统 Shell：{currentShell ? `${currentShell.name}${currentShell.path ? ` (${currentShell.path})` : ""}` : "未检测到"}。
              构建命令时会优先使用这里配置的 shell；若未配置，初始化时会自动选择当前系统 shell。
            </p>
          </div>

          <div className="space-y-2 pt-2 border-t">
            <Label htmlFor="default-task-list-group-mode">任务列表默认分组</Label>
            <Combobox
              id="default-task-list-group-mode"
              items={TASK_GROUP_MODE_OPTIONS}
              value={defaultTaskListGroupMode}
              onValueChange={handleDefaultTaskListGroupModeChange}
              placeholder="选择默认分组方式"
              searchPlaceholder="搜索分组方式"
              emptyText="未找到分组选项"
              disabled={loading}
              className="w-full md:w-[300px]"
            />
            <p className="text-xs text-muted-foreground">
              新建工作区会继承这个初始分组方式；在工作区内切换后，会保存到该工作区自身。
            </p>
          </div>

          {defaultShell === "custom" && (
            <div className="space-y-2 pt-2 border-t">
              <Label htmlFor="custom-shell-command">自定义 Shell 命令</Label>
              <div className="flex flex-col gap-2 sm:flex-row">
                <Input
                  id="custom-shell-command"
                  value={customShellCommand}
                  onChange={(e) => setCustomShellCommand(e.target.value)}
                  placeholder="例如: /bin/zsh 或 /opt/homebrew/bin/fish"
                  className="w-full md:w-[400px]"
                />
                <Button
                  className="w-full sm:w-auto"
                  onClick={async () => {
                    try {
                      await api.updateConfig("custom_shell_command", customShellCommand)
                      notify({ type: "success", title: "保存成功", description: "自定义 shell 已更新" })
                    } catch (error) {
                      notify({ type: "error", title: "保存失败", description: error instanceof Error ? error.message : "未知错误" })
                    }
                  }}
                >
                  保存
                </Button>
              </div>
              <p className="text-xs text-muted-foreground">
                输入 shell 可执行文件路径或系统命令，例如 `/bin/zsh`、`bash`、`pwsh`。
              </p>
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <Database className="h-5 w-5 text-primary" />
            <CardTitle>默认 Provider</CardTitle>
          </div>
          <CardDescription>选择系统默认使用的 LLM 配置</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="default-llm">默认 LLM 配置</Label>
            <Combobox 
              id="default-llm"
              items={llmConfigOptions}
              value={defaultLLMConfigId?.toString() || ""} 
              onValueChange={handleDefaultLLMChange} 
              placeholder="选择默认 Provider"
              searchPlaceholder="搜索默认 Provider"
              emptyText="未找到 Provider 配置"
              disabled={loading || llmConfigs.length === 0}
              className="w-full md:w-[300px]"
            />
            <p className="text-xs text-muted-foreground">
              默认 Provider 将用于新创建的 Worker 和其他需要 LLM 配置的功能。
            </p>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader
          className="cursor-pointer select-none"
          onClick={() => setProjectPermissionsExpanded((value) => !value)}
        >
          <div className="flex items-start justify-between gap-3">
            <div className="space-y-1.5">
              <div className="flex items-center gap-2">
                <Settings className="h-5 w-5 text-primary" />
                <CardTitle>新建项目默认权限</CardTitle>
              </div>
              <CardDescription>作为新建项目的权限模板，不影响已有项目。点击展开编辑。</CardDescription>
            </div>
            <ChevronDown
              className={cn(
                "mt-1 h-5 w-5 shrink-0 text-muted-foreground transition-transform",
                projectPermissionsExpanded && "rotate-180",
              )}
            />
          </div>
        </CardHeader>
        {projectPermissionsExpanded && (
          <CardContent className="space-y-4">
            <ProjectToolPermissionPanel
              toolInfos={toolInfos}
              permissions={defaultProjectToolPermissions}
              onPermissionsChange={setDefaultProjectToolPermissions}
              yoloMode={false}
              onYoloModeChange={() => {}}
              showYoloMode={false}
            />
            <div className="flex justify-end">
              <Button type="button" disabled={loading} onClick={() => void handleSaveDefaultProjectToolPermissions()}>
                保存默认权限
              </Button>
            </div>
          </CardContent>
        )}
      </Card>

      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <Clock className="h-5 w-5 text-primary" />
            <CardTitle>大模型 API 超时</CardTitle>
          </div>
          <CardDescription>控制会话内调用大模型 HTTP 客户端的总超时（单位：秒）</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="llm-http-timeout">超时时长（秒）</Label>
            <div className="flex flex-wrap items-end gap-2">
              <Input
                id="llm-http-timeout"
                type="number"
                min={0}
                step={1}
                inputMode="numeric"
                placeholder="留空表示不限制"
                value={llmHttpTimeoutSeconds}
                onChange={(e) => setLlmHttpTimeoutSeconds(e.target.value)}
                disabled={loading}
                className="w-full max-w-[200px]"
              />
              <Button
                type="button"
                disabled={loading}
                onClick={async () => {
                  const raw = llmHttpTimeoutSeconds.trim()
                  if (raw !== "") {
                    const n = Number(raw)
                    if (!Number.isInteger(n) || n < 1) {
                      notify({ type: "error", title: "无效值", description: "请输入正整数（秒），或留空以取消超时限制" })
                      return
                    }
                  }
                  try {
                    await api.updateConfig(LLM_HTTP_TIMEOUT_SECONDS_KEY, raw)
                    notify({
                      type: "success",
                      title: "已保存",
                      description: raw === "" ? "已取消超时限制" : `已设置为 ${raw} 秒`,
                    })
                  } catch (error) {
                    notify({
                      type: "error",
                      title: "保存失败",
                      description: error instanceof Error ? error.message : "未知错误",
                    })
                  }
                }}
              >
                保存
              </Button>
            </div>
            <p className="text-xs text-muted-foreground">
              对应全局配置键 <span className="font-mono">{LLM_HTTP_TIMEOUT_SECONDS_KEY}</span>。留空或清除为不限制（适合长流式输出）；设置过小可能导致长回答被中断。
            </p>
          </div>
          <div className="space-y-2 pt-4 border-t">
            <Label htmlFor="llm-http-connect-timeout">连接超时时长（秒）</Label>
            <div className="flex flex-wrap items-end gap-2">
              <Input
                id="llm-http-connect-timeout"
                type="number"
                min={0}
                step={1}
                inputMode="numeric"
                placeholder="留空表示不限制"
                value={llmHttpConnectTimeoutSeconds}
                onChange={(e) => setLlmHttpConnectTimeoutSeconds(e.target.value)}
                disabled={loading}
                className="w-full max-w-[200px]"
              />
              <Button
                type="button"
                disabled={loading}
                onClick={async () => {
                  const raw = llmHttpConnectTimeoutSeconds.trim()
                  if (raw !== "") {
                    const n = Number(raw)
                    if (!Number.isInteger(n) || n < 1) {
                      notify({ type: "error", title: "无效值", description: "请输入正整数（秒），或留空以取消超时限制" })
                      return
                    }
                  }
                  try {
                    await api.updateConfig(LLM_HTTP_CONNECT_TIMEOUT_SECONDS_KEY, raw)
                    notify({
                      type: "success",
                      title: "已保存",
                      description: raw === "" ? "已取消连接超时限制" : `已设置为 ${raw} 秒`,
                    })
                  } catch (error) {
                    notify({
                      type: "error",
                      title: "保存失败",
                      description: error instanceof Error ? error.message : "未知错误",
                    })
                  }
                }}
              >
                保存
              </Button>
            </div>
            <p className="text-xs text-muted-foreground">
              对应全局配置键 <span className="font-mono">{LLM_HTTP_CONNECT_TIMEOUT_SECONDS_KEY}</span>。控制从建立连接到收到第一个响应字符的超时；留空或清除为不限制。与上方「总超时」独立。
            </p>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <Hash className="h-5 w-5 text-primary" />
            <CardTitle>大模型单次输出上限</CardTitle>
          </div>
          <CardDescription>
            控制未在模型设置里单独配置「输出上限」时，单次补全允许的最大输出 token 数（服务端默认 100000）
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="llm-max-output-tokens">Max output tokens</Label>
            <div className="flex flex-wrap items-end gap-2">
              <Input
                id="llm-max-output-tokens"
                type="number"
                min={1}
                step={1}
                inputMode="numeric"
                placeholder={`默认 ${100000}`}
                value={llmMaxOutputTokens}
                onChange={(e) => setLlmMaxOutputTokens(e.target.value)}
                disabled={loading}
                className="w-full max-w-[220px]"
              />
              <Button
                type="button"
                disabled={loading}
                onClick={async () => {
                  const raw = llmMaxOutputTokens.trim()
                  if (raw !== "") {
                    const n = Number(raw)
                    if (!Number.isInteger(n) || n < 1) {
                      notify({ type: "error", title: "无效值", description: "请输入正整数，或留空以使用服务端默认（100000）" })
                      return
                    }
                  }
                  try {
                    await api.updateConfig(LLM_MAX_OUTPUT_TOKENS_KEY, raw)
                    notify({
                      type: "success",
                      title: "已保存",
                      description: raw === "" ? "已恢复为默认 100000" : `已设置为 ${raw}`,
                    })
                  } catch (error) {
                    notify({
                      type: "error",
                      title: "保存失败",
                      description: error instanceof Error ? error.message : "未知错误",
                    })
                  }
                }}
              >
                保存
              </Button>
            </div>
            <p className="text-xs text-muted-foreground">
              对应全局配置键 <span className="font-mono">{LLM_MAX_OUTPUT_TOKENS_KEY}</span>。若 Worker
              的模型设置中配置了「输出上限」，仍以模型设置为准。
            </p>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <Archive className="h-5 w-5 text-primary" />
            <CardTitle>记忆压缩</CardTitle>
          </div>
          <CardDescription>控制 Agent 自动记忆压缩与「自动记忆整理」页的触发阈值、目标占用率与 L2 摘要范围。</CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="space-y-2">
            <Label htmlFor="memory-compaction-threshold">压缩阈值（%）</Label>
            <p className="text-xs text-muted-foreground">
              上下文 token 占用达到模型可用窗口的该比例时触发自动压缩（默认 {DEFAULT_MEMORY_COMPACTION_TRIGGER_THRESHOLD}）
            </p>
            <Input
              id="memory-compaction-threshold"
              type="number"
              min={1}
              max={100}
              step={1}
              inputMode="numeric"
              placeholder={`默认 ${DEFAULT_MEMORY_COMPACTION_TRIGGER_THRESHOLD}`}
              value={memoryCompactionThresholdPercent}
              onChange={(e) => setMemoryCompactionThresholdPercent(e.target.value)}
              disabled={loading}
              className="w-full max-w-[220px]"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="memory-compaction-target">目标占用率（%）</Label>
            <p className="text-xs text-muted-foreground">
              压缩后上下文 token 占用应降至模型可用窗口的该比例以下（默认 {DEFAULT_MEMORY_COMPACTION_TARGET_PERCENT}）
            </p>
            <Input
              id="memory-compaction-target"
              type="number"
              min={1}
              max={100}
              step={1}
              inputMode="numeric"
              placeholder={`默认 ${DEFAULT_MEMORY_COMPACTION_TARGET_PERCENT}`}
              value={memoryCompactionTargetPercent}
              onChange={(e) => setMemoryCompactionTargetPercent(e.target.value)}
              disabled={loading}
              className="w-full max-w-[220px]"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="memory-compaction-l2-scope">L2 摘要范围（%）</Label>
            <p className="text-xs text-muted-foreground">
              L2 阶段在可压缩记忆中按 token 从最旧选取的占比（默认 {DEFAULT_MEMORY_COMPACTION_L2_SCOPE_PERCENT}）
            </p>
            <Input
              id="memory-compaction-l2-scope"
              type="number"
              min={1}
              max={100}
              step={1}
              inputMode="numeric"
              placeholder={`默认 ${DEFAULT_MEMORY_COMPACTION_L2_SCOPE_PERCENT}`}
              value={memoryCompactionL2ScopePercent}
              onChange={(e) => setMemoryCompactionL2ScopePercent(e.target.value)}
              disabled={loading}
              className="w-full max-w-[220px]"
            />
          </div>
          <div className="flex flex-wrap items-end gap-2">
            <Button
              type="button"
              disabled={loading}
              onClick={async () => {
                const parsePercent = (raw: string, label: string, fallback: number) => {
                  const trimmed = raw.trim()
                  if (trimmed === "") return { ok: true as const, value: "" }
                  const n = Number(trimmed)
                  if (!Number.isInteger(n) || n < 1 || n > 100) {
                    return { ok: false as const, message: `${label} 请输入 1–100 的整数，或留空使用默认 ${fallback}` }
                  }
                  return { ok: true as const, value: trimmed }
                }

                const trigger = parsePercent(
                  memoryCompactionThresholdPercent,
                  "压缩阈值",
                  DEFAULT_MEMORY_COMPACTION_TRIGGER_THRESHOLD,
                )
                if (!trigger.ok) {
                  notify({ type: "error", title: "无效值", description: trigger.message })
                  return
                }
                const target = parsePercent(
                  memoryCompactionTargetPercent,
                  "目标占用率",
                  DEFAULT_MEMORY_COMPACTION_TARGET_PERCENT,
                )
                if (!target.ok) {
                  notify({ type: "error", title: "无效值", description: target.message })
                  return
                }
                const l2Scope = parsePercent(
                  memoryCompactionL2ScopePercent,
                  "L2 摘要范围",
                  DEFAULT_MEMORY_COMPACTION_L2_SCOPE_PERCENT,
                )
                if (!l2Scope.ok) {
                  notify({ type: "error", title: "无效值", description: l2Scope.message })
                  return
                }

                try {
                  await Promise.all([
                    api.updateConfig(MEMORY_COMPACTION_TRIGGER_THRESHOLD_KEY, trigger.value),
                    api.updateConfig(MEMORY_COMPACTION_TARGET_PERCENT_KEY, target.value),
                    api.updateConfig(MEMORY_COMPACTION_L2_SCOPE_PERCENT_KEY, l2Scope.value),
                  ])
                  notify({ type: "success", title: "已保存", description: "记忆压缩配置已更新" })
                } catch (error) {
                  notify({
                    type: "error",
                    title: "保存失败",
                    description: error instanceof Error ? error.message : "未知错误",
                  })
                }
              }}
            >
              保存
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <Repeat className="h-5 w-5 text-primary" />
            <CardTitle>Agent 最大迭代步数</CardTitle>
          </div>
          <CardDescription>
            单轮任务中 Agent 与模型交互的最大步数（工具调用、继续推理等计为一步）；服务端默认 {DEFAULT_AGENT_MAX_STEPS}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="agent-max-steps">Max steps</Label>
            <div className="flex flex-wrap items-end gap-2">
              <Input
                id="agent-max-steps"
                type="number"
                min={1}
                step={1}
                inputMode="numeric"
                placeholder={`默认 ${DEFAULT_AGENT_MAX_STEPS}`}
                value={agentMaxSteps}
                onChange={(e) => setAgentMaxSteps(e.target.value)}
                disabled={loading}
                className="w-full max-w-[220px]"
              />
              <Button
                type="button"
                disabled={loading}
                onClick={async () => {
                  const raw = agentMaxSteps.trim()
                  if (raw !== "") {
                    const n = Number(raw)
                    if (!Number.isInteger(n) || n < 1) {
                      notify({
                        type: "error",
                        title: "无效值",
                        description: `请输入正整数，或留空以使用服务端默认（${DEFAULT_AGENT_MAX_STEPS}）`,
                      })
                      return
                    }
                  }
                  try {
                    await api.updateConfig(AGENT_MAX_STEPS_KEY, raw)
                    notify({
                      type: "success",
                      title: "已保存",
                      description: raw === "" ? `已恢复为默认 ${DEFAULT_AGENT_MAX_STEPS}` : `已设置为 ${raw}`,
                    })
                  } catch (error) {
                    notify({
                      type: "error",
                      title: "保存失败",
                      description: error instanceof Error ? error.message : "未知错误",
                    })
                  }
                }}
              >
                保存
              </Button>
            </div>
            <p className="text-xs text-muted-foreground">
              对应全局配置键 <span className="font-mono">{AGENT_MAX_STEPS_KEY}</span>。若 Worker 配置了更大的
              <span className="font-mono"> steps</span> 字段，则优先使用 Worker 上的值。
            </p>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <Braces className="h-5 w-5 text-primary" />
            <CardTitle>LLM 自定义请求头</CardTitle>
          </div>
          <CardDescription>附加到所有调用大模型 API 的 HTTP 请求（JSON 对象，键为 Header 名，值为字符串）</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="llm-custom-headers">自定义 Header（JSON）</Label>
            <Textarea
              id="llm-custom-headers"
              value={llmCustomHeadersText}
              onChange={(e) => setLlmCustomHeadersText(e.target.value)}
              placeholder={'例如：{"X-Custom-Gateway-Auth":"your-token"}'}
              disabled={loading}
              className="font-mono text-sm min-h-[120px]"
              spellCheck={false}
            />
            <p className="text-xs text-muted-foreground">
              保存后立即对服务端生效；留空或填写 {"{}"} 可清除附加请求头。请勿在此存放可被日志记录的极高敏感信息。
            </p>
          </div>
          <div className="flex justify-end">
            <Button
              type="button"
              disabled={loading}
              onClick={async () => {
                const trimmed = llmCustomHeadersText.trim()
                if (trimmed !== "") {
                  try {
                    JSON.parse(trimmed)
                  } catch {
                    notify({ type: "error", title: "JSON 无效", description: "请检查 JSON 格式" })
                    return
                  }
                }
                try {
                  await api.updateConfig(LLM_CUSTOM_HEADERS_KEY, trimmed)
                  notify({ type: "success", title: "已保存", description: "LLM 自定义请求头已更新" })
                } catch (error) {
                  notify({
                    type: "error",
                    title: "保存失败",
                    description: error instanceof Error ? error.message : "未知错误",
                  })
                }
              }}
            >
              保存请求头
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
