const { contextBridge, ipcRenderer } = require('electron');

// 暴露安全的 API 给渲染进程
contextBridge.exposeInMainWorld('electronAPI', {
  // 获取后端服务器 URL
  getBackendUrl: () => ipcRenderer.invoke('get-backend-url'),

  /** 启动器窗口：关闭本窗口并显示主窗口（主窗口加载工作台） */
  closeLauncherAndOpenMain: () => ipcRenderer.invoke('electron:close-launcher-open-main'),
  openLauncherWindow: () => ipcRenderer.invoke('electron:open-launcher-window'),
  openWorkspaceWindow: (payload) => ipcRenderer.invoke('electron:open-workspace-window', payload),
  setCurrentWorkspaceWindow: (payload) => ipcRenderer.invoke('electron:set-current-workspace-window', payload),
  openExternalWindow: (payload) => ipcRenderer.invoke('electron:open-external-window', payload),
  closeExternalWindow: (windowId) => ipcRenderer.invoke('electron:close-external-window', windowId),
  
  // 应用信息
  platform: process.platform,
  isElectron: true,
});
