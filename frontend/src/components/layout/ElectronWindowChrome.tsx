import type { ReactNode } from "react"
import { cn } from "@/lib/utils"
import { getElectronWindowChromeInsets, usesElectronCustomTitleBar } from "@/lib/electron"

/**
 * Electron 自定义标题栏区域：与 MainLayout 顶条对齐的可拖拽带，左右为 no-drag 以避让系统按钮。
 * `trailing` 置于右侧系统按钮区左侧（如设置），需使用 `[-webkit-app-region:no-drag]`。
 */
export function ElectronWindowChromeSpacer({
  className,
  leading,
  center,
  trailing,
}: {
  className?: string
  /** 置于红绿灯右侧、拖拽区左侧（如搜索、侧栏折叠），须 no-drag */
  leading?: ReactNode
  /** 居中展示的标题内容；保持在拖拽区内 */
  center?: ReactNode
  trailing?: ReactNode
}) {
  if (!usesElectronCustomTitleBar()) {
    return null
  }
  const { top, left, right } = getElectronWindowChromeInsets()
  return (
    <div
      className={cn(
        "relative flex shrink-0 select-none items-stretch border-b border-border/50 bg-muted/15",
        className
      )}
      style={{ height: top }}
    >
      {left > 0 ? <div className="shrink-0 [-webkit-app-region:no-drag]" style={{ width: left }} /> : null}
      {leading ? (
        <div className="flex shrink-0 items-center [-webkit-app-region:no-drag]">{leading}</div>
      ) : null}
      <div className="min-h-0 min-w-0 flex-1 [-webkit-app-region:drag]" />
      {center ? (
        <div className="pointer-events-none absolute inset-0 flex items-center justify-center px-4">
          <div className="max-w-[420px] truncate text-sm font-medium text-foreground/80">
            {center}
          </div>
        </div>
      ) : null}
      {trailing ? (
        <div className="flex shrink-0 items-center justify-center [-webkit-app-region:no-drag] pr-0.5">
          {trailing}
        </div>
      ) : null}
      {right > 0 ? <div className="shrink-0 [-webkit-app-region:no-drag]" style={{ width: right }} /> : null}
    </div>
  )
}

/**
 * 为全屏/模态内容增加与系统装饰区对齐的水平内边距（mac 左、Windows 右）。
 */
export function ElectronWindowChromeInset({
  children,
  className,
}: {
  children: ReactNode
  className?: string
}) {
  if (!usesElectronCustomTitleBar()) {
    return <>{children}</>
  }
  const { left, right } = getElectronWindowChromeInsets()
  if (left === 0 && right === 0) {
    return <>{children}</>
  }
  return (
    <div
      className={cn("flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden", className)}
      style={{ paddingLeft: left, paddingRight: right }}
    >
      {children}
    </div>
  )
}
