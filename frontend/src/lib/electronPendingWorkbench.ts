/** 与主进程默认 session 一致时，启动器窗口与主窗口共享 localStorage，用于交接工作台 tab。 */
export const ELECTRON_PENDING_WORKBENCH_KEY = "matrixops.electron.pending-workbench.v1"

export type ElectronPendingWorkbenchOpen = {
  kind: "workspace"
  id: number
  name: string
}

export function setPendingElectronWorkbenchOpen(payload: ElectronPendingWorkbenchOpen): void {
  try {
    window.localStorage.setItem(ELECTRON_PENDING_WORKBENCH_KEY, JSON.stringify(payload))
  } catch (e) {
    console.error("写入 Electron 待打开工作区失败:", e)
  }
}

export function takePendingElectronWorkbenchOpen(): ElectronPendingWorkbenchOpen | null {
  if (typeof window === "undefined") {
    return null
  }
  let raw: string | null
  try {
    raw = window.localStorage.getItem(ELECTRON_PENDING_WORKBENCH_KEY)
  } catch {
    return null
  }
  if (!raw) {
    return null
  }
  try {
    window.localStorage.removeItem(ELECTRON_PENDING_WORKBENCH_KEY)
  } catch {
    /* ignore */
  }
  try {
    const parsed = JSON.parse(raw) as ElectronPendingWorkbenchOpen
    if (parsed?.kind === "workspace" && typeof parsed.id === "number" && typeof parsed.name === "string") {
      return parsed
    }
  } catch {
    /* ignore */
  }
  return null
}
