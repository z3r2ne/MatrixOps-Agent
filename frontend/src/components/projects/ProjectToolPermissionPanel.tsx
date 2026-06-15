import React from "react"

import { ToolInfo } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Combobox, type ComboboxOption } from "@/components/ui/combobox"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"

interface ProjectToolPermissionPanelProps {
  toolInfos: ToolInfo[]
  permissions: Record<string, string>
  onPermissionsChange: React.Dispatch<React.SetStateAction<Record<string, string>>>
  yoloMode: boolean
  onYoloModeChange: (value: boolean) => void
  showYoloMode?: boolean
}

const permissionOptions = [
  { value: "allow", label: "允许" },
  { value: "ask", label: "询问" },
  { value: "deny", label: "禁止" },
] as const

const permissionComboboxOptions: ComboboxOption[] = permissionOptions.map((option) => ({
  value: option.value,
  label: option.label,
  searchText: `${option.value} ${option.label}`,
}))

export function ProjectToolPermissionPanel({
  toolInfos,
  permissions,
  onPermissionsChange,
  yoloMode,
  onYoloModeChange,
  showYoloMode = true,
}: ProjectToolPermissionPanelProps) {
  const applyPreset = (value: "allow" | "ask" | "deny") => {
    onPermissionsChange(
      toolInfos.reduce<Record<string, string>>((acc, toolInfo) => {
        acc[toolInfo.name] = value
        return acc
      }, {})
    )
  }

  return (
    <div className="rounded-xl border border-border/60 bg-muted/20 p-4 space-y-4">
      {showYoloMode ? (
        <div className="flex items-start justify-between gap-4">
          <div className="space-y-1">
            <Label className="text-sm">YOLO 模式</Label>
            <p className="text-xs text-muted-foreground">
              打开后，项目级工具权限检查会被跳过，worker 已启用的工具都可以直接执行。
            </p>
          </div>
          <Switch checked={yoloMode} onCheckedChange={onYoloModeChange} />
        </div>
      ) : null}

      <div className="flex flex-wrap items-center gap-2">
        <Button type="button" variant="outline" size="sm" onClick={() => applyPreset("ask")}>
          全部设为询问
        </Button>
        <Button type="button" variant="outline" size="sm" onClick={() => applyPreset("allow")}>
          全部设为允许
        </Button>
        <Button type="button" variant="outline" size="sm" onClick={() => applyPreset("deny")}>
          全部设为禁止
        </Button>
      </div>

      <div className="grid gap-2 xl:grid-cols-2">
        {toolInfos.map((toolInfo) => (
          <div
            key={toolInfo.name}
            className={`flex flex-col gap-3 rounded-lg border p-3 ${
              yoloMode ? "border-primary/30 bg-primary/5 opacity-80" : "border-border/60"
            }`}
          >
            <div className="min-w-0 flex-1">
              <p className="text-sm font-medium">
                {toolInfo.verbosName || toolInfo.name} ({toolInfo.name})
              </p>
              <p className="text-xs text-muted-foreground">{toolInfo.description}</p>
            </div>
            <Combobox
              id={`tool-permission-${toolInfo.name}`}
              items={permissionComboboxOptions}
              value={permissions[toolInfo.name] || "ask"}
              onValueChange={(value) =>
                onPermissionsChange((prev) => ({
                  ...prev,
                  [toolInfo.name]: value,
                }))
              }
              placeholder="选择权限"
              searchPlaceholder="搜索权限"
              emptyText="未找到权限"
              disabled={yoloMode}
            />
          </div>
        ))}
      </div>
    </div>
  )
}
