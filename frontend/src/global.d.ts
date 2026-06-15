export {}

declare global {
  interface Window {
    electronAPI?: {
      getBackendUrl: () => Promise<string>
      closeLauncherAndOpenMain: () => Promise<{ ok: boolean; error?: string }>
      openLauncherWindow: () => Promise<{ ok: boolean; error?: string }>
      openWorkspaceWindow: (payload: { id: number; name: string }) => Promise<{ ok: boolean; error?: string }>
      setCurrentWorkspaceWindow: (payload: { id: number; name: string }) => Promise<{ ok: boolean; error?: string }>
      openExternalWindow: (
        payload: string | { url: string; title?: string }
      ) => Promise<{ ok: boolean; windowId?: string; error?: string }>
      closeExternalWindow: (windowId: string) => Promise<{ ok: boolean; error?: string }>
      platform: NodeJS.Platform
      isElectron: boolean
    }
  }
}
