import type { CSSProperties } from "react"

/** 是否在 Electron 壳内运行（与 preload 注入的 `electronAPI` 一致） */
export function isElectronApp(): boolean {
  if (typeof window === "undefined") return false
  return Boolean(window.electronAPI?.isElectron)
}

/** 是否在 Electron 且为 macOS（用于仅需 mac 区分的逻辑） */
export function isElectronMac(): boolean {
  if (typeof window === "undefined") return false
  const api = window.electronAPI
  return Boolean(api?.isElectron && api.platform === "darwin")
}

/**
 * 是否使用自定义顶栏（mac：hiddenInset；Windows：frameless + titleBarOverlay）。
 * 此时应显示可拖拽顶栏并配合 -webkit-app-region。
 */
export function usesElectronCustomTitleBar(): boolean {
  if (typeof window === "undefined") return false
  const api = window.electronAPI
  if (!api?.isElectron) return false
  return api.platform === "darwin" || api.platform === "win32"
}

/** 与 MainLayout `h-9`、electron `main.cjs` CUSTOM_TITLEBAR_HEIGHT 一致 */
export const ELECTRON_CUSTOM_TITLEBAR_HEIGHT_PX = 36

/**
 * macOS `hiddenInset`：红绿灯左侧占位（经验值），避免全屏层内容与系统按钮重叠。
 */
export const ELECTRON_MAC_TRAFFIC_LIGHTS_LEFT_INSET_PX = 78

/**
 * Windows `frameless` + `titleBarOverlay`：右侧系统标题按钮区近似宽度，避免与页面右上角控件重叠。
 */
export const ELECTRON_WIN_CAPTION_RIGHT_INSET_PX = 136

export type ElectronWindowChromeInsets = {
  /** 自定义顶栏条高度（与主界面一致） */
  top: number
  /** 左侧避让（mac 红绿灯） */
  left: number
  /** 右侧避让（Windows 标题栏按钮） */
  right: number
}

export type ElectronWindowChromeCSSVars = CSSProperties & {
  "--electron-window-chrome-top"?: string
  "--electron-window-chrome-left"?: string
  "--electron-window-chrome-right"?: string
}

/**
 * 全屏/固定层（Dialog等）在 Electron 下应使用的边距，避免与系统窗口装饰区重叠。
 * Web 主布局外的 portal 内容默认从 (0,0) 铺满，需自行套用。
 */
export function getElectronWindowChromeInsets(): ElectronWindowChromeInsets {
  if (typeof window === "undefined" || !usesElectronCustomTitleBar()) {
    return { top: 0, left: 0, right: 0 }
  }
  const p = window.electronAPI?.platform
  if (p === "darwin") {
    return {
      top: ELECTRON_CUSTOM_TITLEBAR_HEIGHT_PX,
      left: ELECTRON_MAC_TRAFFIC_LIGHTS_LEFT_INSET_PX,
      right: 0,
    }
  }
  if (p === "win32") {
    return {
      top: ELECTRON_CUSTOM_TITLEBAR_HEIGHT_PX,
      left: 0,
      right: ELECTRON_WIN_CAPTION_RIGHT_INSET_PX,
    }
  }
  return { top: ELECTRON_CUSTOM_TITLEBAR_HEIGHT_PX, left: 0, right: 0 }
}

export function getElectronWindowChromeCSSVars(): ElectronWindowChromeCSSVars {
  const { top, left, right } = getElectronWindowChromeInsets()
  return {
    "--electron-window-chrome-top": `${top}px`,
    "--electron-window-chrome-left": `${left}px`,
    "--electron-window-chrome-right": `${right}px`,
  }
}
