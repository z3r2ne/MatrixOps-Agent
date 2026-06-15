const { app, BrowserWindow, ipcMain, nativeTheme } = require('electron');
const path = require('path');
const { spawn, spawnSync } = require('child_process');
const http = require('http');
const fs = require('fs');
const url = require('url');

let mainWindow;
const mainWindows = new Set();
let launcherWindow;
let backendProcess;
let backendStopPromise;
let isQuitting = false;
let backendPort = 8080;
let backendHost = 'localhost';
const isDev = !app.isPackaged;
const devRendererURL = process.env.ELECTRON_RENDERER_URL || 'http://localhost:3010';
const packagedBackendPortMin = 5000;
const packagedBackendPortMax = 5999;
const windowWorkspaceMap = new Map();
const DEFAULT_WINDOW_TITLE = 'MatrixOps';
/** 串行化工作区 open/close 的后端同步，避免关窗后立刻 quit 时请求未落库 */
let workspacePersistenceChain = Promise.resolve();

// 检查端口是否可用
function checkPortAvailable(port, host) {
  return new Promise((resolve) => {
    const server = http.createServer();
    
    server.once('error', (err) => {
      resolve(false);
    });
    
    server.once('listening', () => {
      server.close();
      resolve(true);
    });
    
    server.listen(port, host);
  });
}

// 查找可用端口
async function findAvailablePort(startPort = 8080) {
  let port = startPort;
  while (port < startPort + 100) {
    const available = await checkPortAvailable(port, backendHost);
    if (available) {
      return port;
    }
    port++;
  }
  throw new Error('无法找到可用端口');
}

async function findRandomAvailablePort(minPort, maxPort) {
  const ports = [];
  for (let port = minPort; port <= maxPort; port++) {
    ports.push(port);
  }

  for (let index = ports.length - 1; index > 0; index--) {
    const randomIndex = Math.floor(Math.random() * (index + 1));
    [ports[index], ports[randomIndex]] = [ports[randomIndex], ports[index]];
  }

  for (const port of ports) {
    const available = await checkPortAvailable(port, backendHost);
    if (available) {
      return port;
    }
  }

  throw new Error(`无法在 ${minPort}-${maxPort} 范围内找到可用端口`);
}

// 等待服务器就绪
function waitForServer(url, maxRetries = 60, retryDelay = 1000) {
  return new Promise((resolve, reject) => {
    let retries = 0;
    
    const check = () => {
      http.get(url, (res) => {
        if (res.statusCode === 200 || res.statusCode === 304) {
          console.log(`✅ 服务器已就绪: ${url}`);
          resolve();
        } else {
          retry();
        }
      }).on('error', () => {
        retry();
      });
    };
    
    const retry = () => {
      retries++;
      if (retries >= maxRetries) {
        reject(new Error(`服务器启动超时: ${url}`));
      } else {
        if (retries % 5 === 0) {
          console.log(`等待服务器... (${retries}/${maxRetries})`);
        }
        setTimeout(check, retryDelay);
      }
    };
    
    check();
  });
}

// 启动后端服务器
async function startBackendServer() {
  console.log('🚀 正在启动后端服务器...');
  
  // 生产模式使用 5000+ 随机可用端口
  backendPort = isDev
    ? await findAvailablePort(8080)
    : await findRandomAvailablePort(packagedBackendPortMin, packagedBackendPortMax);
  console.log(`使用端口: ${backendPort}`);
  
  // 获取可执行文件路径
  let backendPath;
  let backendCwd;
  
  if (isDev) {
    // 开发模式：使用编译后的二进制文件
    backendCwd = path.join(__dirname, '../..');
    if (process.platform === 'win32') {
      backendPath = path.join(__dirname, '../../build/matrixops.exe');
    } else {
      backendPath = path.join(__dirname, '../../build/matrixops');
    }
  } else {
    // 生产模式：使用打包后的二进制文件
    const resourcesPath = process.resourcesPath;
    backendCwd = resourcesPath;
    if (process.platform === 'win32') {
      backendPath = path.join(resourcesPath, 'matrixops.exe');
    } else {
      backendPath = path.join(resourcesPath, 'matrixops');
    }
  }
  
  console.log(`后端路径: ${backendPath}`);
  console.log(`后端工作目录: ${backendCwd}`);
  
  // 检查后端文件是否存在
  if (!fs.existsSync(backendPath)) {
    throw new Error(`后端可执行文件不存在: ${backendPath}\n请先运行: mkdir -p build && go build -o build/matrixops ./cmd/main.go`);
  }
  
  // 启动后端进程
  backendProcess = spawn(backendPath, ['server', '--host', backendHost, '--port', backendPort.toString()], {
    cwd: backendCwd,
    env: {
      ...process.env,
      HOST: backendHost,
      PORT: backendPort.toString(),
    },
    stdio: 'inherit',
    detached: process.platform !== 'win32',
  });
  
  const child = backendProcess;

  child.on('error', (err) => {
    console.error('❌ 后端服务器启动失败:', err);
  });
  
  child.on('exit', (code, signal) => {
    if (backendProcess === child) {
      backendProcess = null;
    }
    if (!isQuitting && code !== 0 && code !== null) {
      console.error(`❌ 后端服务器异常退出，代码: ${code}`);
    } else if (isQuitting) {
      console.log(`✅ 后端服务器已退出${signal ? ` (${signal})` : ''}`);
    }
  });
  
  // 等待后端服务器就绪
  try {
    await waitForServer(`http://${backendHost}:${backendPort}/health`);
  } catch (err) {
    console.error('❌', err.message);
    throw err;
  }
}

function signalBackend(signal) {
  if (!backendProcess || !backendProcess.pid) {
    return;
  }

  try {
    if (process.platform === 'win32') {
      backendProcess.kill(signal);
      return;
    }

    // 后端以 detached 方式启动，pid 同时是进程组 id；杀进程组可避免子进程残留。
    process.kill(-backendProcess.pid, signal);
  } catch (err) {
    try {
      backendProcess.kill(signal);
    } catch (fallbackErr) {
      console.warn(`发送 ${signal} 到后端失败:`, fallbackErr);
    }
  }
}

function forceKillBackendTree() {
  if (!backendProcess || !backendProcess.pid) {
    return;
  }

  if (process.platform === 'win32') {
    spawnSync('taskkill', ['/pid', String(backendProcess.pid), '/T', '/F'], {
      stdio: 'ignore',
      windowsHide: true,
    });
    return;
  }

  signalBackend('SIGKILL');
}

function stopBackendServer() {
  if (!backendProcess || !backendProcess.pid) {
    return Promise.resolve();
  }
  if (backendStopPromise) {
    return backendStopPromise;
  }

  const child = backendProcess;
  console.log('🔄 正在关闭后端服务器...');

  backendStopPromise = new Promise((resolve) => {
    let settled = false;
    let forceTimer;
    let finalTimer;

    const finish = () => {
      if (settled) {
        return;
      }
      settled = true;
      clearTimeout(forceTimer);
      clearTimeout(finalTimer);
      if (backendProcess === child) {
        backendProcess = null;
      }
      backendStopPromise = null;
      resolve();
    };

    child.once('exit', finish);

    forceTimer = setTimeout(() => {
      if (settled) {
        return;
      }
      console.log('⚠️  后端未及时退出，强制终止后端进程树');
      forceKillBackendTree();
    }, 5000);

    finalTimer = setTimeout(() => {
      if (!settled) {
        console.warn('⚠️  后端退出等待超时，继续退出 Electron');
        finish();
      }
    }, 8000);

    signalBackend('SIGTERM');
  });

  return backendStopPromise;
}

/** 与 frontend MainLayout 顶栏 h-9（36px）一致，便于 -webkit-app-region: drag 与系统按钮对齐 */
const CUSTOM_TITLEBAR_HEIGHT = 36;

/** Windows：与 nativeTheme 对齐的标题栏叠加层（系统切换深/浅色时同步） */
function winTitleBarOverlayOptions() {
  const dark = nativeTheme.shouldUseDarkColors;
  return {
    height: CUSTOM_TITLEBAR_HEIGHT,
    color: dark ? '#1c1c1e' : '#ffffff',
    symbolColor: dark ? '#e4e4e7' : '#1f2937',
  };
}

function winBrowserBackgroundColor() {
  return nativeTheme.shouldUseDarkColors ? '#1c1c1e' : '#ffffff';
}

function syncWindowsChromeTheme(win) {
  if (process.platform !== 'win32' || !win || win.isDestroyed()) {
    return;
  }
  try {
    win.setTitleBarOverlay(winTitleBarOverlayOptions());
    win.setBackgroundColor(winBrowserBackgroundColor());
  } catch (e) {
    console.warn('同步 Windows 标题栏主题失败:', e);
  }
}

function titlebarWindowOptions() {
  if (process.platform === 'darwin') {
    // 隐藏标题条，保留红绿灯；拖拽由渲染进程顶栏承担
    return { titleBarStyle: 'hiddenInset' };
  }
  if (process.platform === 'win32') {
    // 无边框 + 标题栏叠加层（最小化/最大化/关闭），拖拽由渲染进程顶栏承担
    return {
      frame: false,
      titleBarOverlay: winTitleBarOverlayOptions(),
    };
  }
  return {};
}

/** F12 切换开发者工具（与浏览器习惯一致；需主窗口获得焦点时生效） */
function attachToggleDevToolsShortcut(win) {
  win.webContents.on('before-input-event', (event, input) => {
    if (input.type !== 'keyDown' || input.key !== 'F12') {
      return;
    }
    event.preventDefault();
    win.webContents.toggleDevTools();
  });
}

function getRendererOrigin() {
  const base = isDev ? devRendererURL : `http://${backendHost}:${backendPort}`;
  return base.replace(/\/$/, '');
}

function buildWorkbenchUrl(pendingWorkspace) {
  const baseUrl = `${getRendererOrigin()}/workbench`;
  if (!pendingWorkspace || typeof pendingWorkspace.id !== 'number' || typeof pendingWorkspace.name !== 'string') {
    return baseUrl;
  }
  const params = new URLSearchParams({
    workspaceId: String(pendingWorkspace.id),
    workspaceName: pendingWorkspace.name,
  });
  return `${baseUrl}?${params.toString()}`;
}

function getBackendBaseURL() {
  return `http://${backendHost}:${backendPort}`;
}

function fetchJSON(pathname, options = {}) {
  const target = new URL(pathname, getBackendBaseURL());
  return new Promise((resolve, reject) => {
    const req = http.request(target, {
      method: options.method || 'GET',
      headers: {
        'Content-Type': 'application/json',
      },
    }, (res) => {
      let body = '';
      res.setEncoding('utf8');
      res.on('data', (chunk) => {
        body += chunk;
      });
      res.on('end', () => {
        if (res.statusCode && res.statusCode >= 400) {
          reject(new Error(`HTTP ${res.statusCode}: ${body}`));
          return;
        }
        if (!body.trim()) {
          resolve({});
          return;
        }
        try {
          resolve(JSON.parse(body));
        } catch (err) {
          reject(err);
        }
      });
    });
    req.on('error', reject);
    if (options.body) {
      req.write(JSON.stringify(options.body));
    }
    req.end();
  });
}

function resolveWindowTitle(workspaceName) {
  const trimmed = typeof workspaceName === 'string' ? workspaceName.trim() : '';
  return trimmed || DEFAULT_WINDOW_TITLE;
}

/** 同步系统级窗口标题（macOS 任务切换 / Mission Control 等依赖此项，而非应用内自定义顶栏） */
function syncNativeWindowTitle(win, workspaceName) {
  if (!win || win.isDestroyed()) {
    return;
  }
  try {
    win.setTitle(resolveWindowTitle(workspaceName));
  } catch (err) {
    console.warn('同步窗口标题失败:', err?.message || err);
  }
}

function markWindowWorkspace(win, pendingWorkspace) {
  if (!win || win.isDestroyed()) {
    return;
  }
  if (pendingWorkspace && typeof pendingWorkspace.id === 'number') {
    const name = typeof pendingWorkspace.name === 'string' ? pendingWorkspace.name : '';
    const previous = windowWorkspaceMap.get(win);
    if (backendPort && !isQuitting) {
      // 同一窗口内切换工作区时，先从「已打开」列表移除旧工作区，避免关窗后仍恢复旧窗口
      if (previous?.id && previous.id !== pendingWorkspace.id) {
        fetchJSON(`/api/ui/open/workspaces/${previous.id}`, { method: 'DELETE' }).catch((err) => {
          console.warn('切换工作区时移除旧已打开记录失败:', err?.message || err);
        });
      }
      fetchJSON(`/api/ui/open/workspaces/${pendingWorkspace.id}`, { method: 'POST' }).catch((err) => {
        console.warn('登记已打开工作区失败:', err?.message || err);
      });
    }
    windowWorkspaceMap.set(win, {
      id: pendingWorkspace.id,
      name,
    });
    syncNativeWindowTitle(win, name);
    return;
  }
  windowWorkspaceMap.delete(win);
  syncNativeWindowTitle(win);
}

function findWorkspaceWindow(workspaceId) {
  for (const [win, workspace] of windowWorkspaceMap.entries()) {
    if (!win || win.isDestroyed()) {
      windowWorkspaceMap.delete(win);
      continue;
    }
    if (workspace?.id === workspaceId) {
      return win;
    }
  }
  return null;
}

/**
 * @param {import('electron').BrowserWindow | null} win
 * @param {{ id: number, name?: string } | null | undefined} workspaceSnapshot closed 时窗口已销毁，须在回调里同步快照
 */
async function unmarkWindowWorkspace(win, workspaceSnapshot) {
  const workspace =
    workspaceSnapshot ?? (win ? windowWorkspaceMap.get(win) : undefined);

  if (workspace?.id && backendPort) {
    // 用户主动关窗：从已打开列表移除，便于下次只恢复仍保留的工作区
    if (!isQuitting) {
      try {
        await fetchJSON(`/api/ui/open/workspaces/${workspace.id}`, { method: 'DELETE' });
      } catch (err) {
        console.warn('移除已打开工作区失败:', err?.message || err);
      }
      // 每次手动关窗都更新「最后关闭」，避免同窗口切换后仍恢复更早的工作区
      try {
        await fetchJSON('/api/ui/open/last-closed-workspace', {
          method: 'POST',
          body: { id: workspace.id },
        });
      } catch (err) {
        console.warn('记录最后关闭工作区失败:', err?.message || err);
      }
    }
    // 应用退出（Cmd+Q 等）：保留已打开列表，下次启动恢复全部工作区窗口
  }
  if (win) {
    windowWorkspaceMap.delete(win);
    if (!win.isDestroyed()) {
      syncNativeWindowTitle(win);
    }
  }
}

function queueUnmarkWindowWorkspace(win, workspaceSnapshot) {
  workspacePersistenceChain = workspacePersistenceChain
    .then(() => unmarkWindowWorkspace(win, workspaceSnapshot))
    .catch((err) => {
      console.warn('同步工作区关闭状态失败:', err?.message || err);
    });
  return workspacePersistenceChain;
}

/**
 * 创建主窗口（完整应用，默认路由 /workbench）。
 * @param {{ autoShow?: boolean }} options autoShow 为 false 时由调用方在适当时机 show（例如启动器交接）。
 * @returns {Promise<import('electron').BrowserWindow>}
 */
function createMainWindow(options = {}) {
  const { autoShow = true, reuseExisting = true, pendingWorkspace = null } = options;

  if (reuseExisting && mainWindow && !mainWindow.isDestroyed()) {
    return Promise.resolve(mainWindow);
  }

  const initialTitle = resolveWindowTitle(pendingWorkspace?.name);

  const win = new BrowserWindow({
    width: 1400,
    height: 900,
    minWidth: 1000,
    minHeight: 600,
    webPreferences: {
      nodeIntegration: false,
      contextIsolation: true,
      preload: path.join(__dirname, 'preload.cjs'),
    },
    title: initialTitle,
    backgroundColor:
      process.platform === 'win32' ? winBrowserBackgroundColor() : '#ffffff',
    show: false,
    ...titlebarWindowOptions(),
  });

  if (!mainWindow || mainWindow.isDestroyed()) {
    mainWindow = win;
  }
  mainWindows.add(win);
  markWindowWorkspace(win, pendingWorkspace);

  const reapplyWorkspaceWindowTitle = () => {
    const workspace = windowWorkspaceMap.get(win);
    if (workspace) {
      syncNativeWindowTitle(win, workspace.name);
    }
  };
  win.webContents.on('page-title-updated', reapplyWorkspaceWindowTitle);
  win.webContents.on('did-finish-load', reapplyWorkspaceWindowTitle);

  const startUrl = buildWorkbenchUrl(pendingWorkspace);
  console.log(`📱 加载主窗口: ${startUrl}`);
  win.loadURL(startUrl);
  attachToggleDevToolsShortcut(win);

  win.on('closed', () => {
    // closed 触发时 win 已销毁，必须先同步读出工作区再异步写库
    const workspaceSnapshot = windowWorkspaceMap.get(win);
    mainWindows.delete(win);
    queueUnmarkWindowWorkspace(win, workspaceSnapshot);
    if (mainWindow === win) {
      mainWindow = Array.from(mainWindows).find((item) => !item.isDestroyed()) || null;
    }
  });

  win.webContents.setWindowOpenHandler(({ url }) => {
    require('electron').shell.openExternal(url);
    return { action: 'deny' };
  });

  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      reject(new Error('主窗口 ready-to-show 超时'));
    }, 60000);

    win.once('ready-to-show', () => {
      clearTimeout(timer);
      if (autoShow) {
        win.show();
        if (isDev) {
          win.webContents.openDevTools();
        }
      }
      resolve(win);
    });
  });
}

/** 启动时：仅选择/创建工作区，无侧栏 */
function createLauncherWindow() {
  if (launcherWindow && !launcherWindow.isDestroyed()) {
    launcherWindow.focus();
    return launcherWindow;
  }

  launcherWindow = new BrowserWindow({
    width: 520,
    height: 580,
    minWidth: 400,
    minHeight: 440,
    webPreferences: {
      nodeIntegration: false,
      contextIsolation: true,
      preload: path.join(__dirname, 'preload.cjs'),
    },
    title: DEFAULT_WINDOW_TITLE,
    backgroundColor:
      process.platform === 'win32' ? winBrowserBackgroundColor() : '#ffffff',
    show: false,
    ...titlebarWindowOptions(),
  });

  const startUrl = `${getRendererOrigin()}/electron-launcher`;
  console.log(`🪟 加载启动窗口: ${startUrl}`);
  launcherWindow.loadURL(startUrl);
  attachToggleDevToolsShortcut(launcherWindow);

  launcherWindow.once('ready-to-show', () => {
    launcherWindow.show();

    if (isDev) {
      launcherWindow.webContents.openDevTools();
    }
  });

  launcherWindow.on('closed', () => {
    launcherWindow = null;
    const mainAlive = mainWindow && !mainWindow.isDestroyed();
    if (!mainAlive && !isQuitting) {
      app.quit();
    }
  });

  launcherWindow.webContents.setWindowOpenHandler(({ url }) => {
    require('electron').shell.openExternal(url);
    return { action: 'deny' };
  });

  return launcherWindow;
}

// 应用启动
app.whenReady().then(async () => {
  try {
    if (!isDev) {
      // 生产模式由 Electron 自行拉起后端
      await startBackendServer();
    } else {
      console.log('🧪 Electron dev mode: skip embedded backend startup');
      console.log(`请确保前端 dev server 已启动: ${devRendererURL}`);
      console.log(`请确保后端服务已单独启动并与前端代理配置一致（默认 http://localhost:8080）`);
    }

    const openState = await fetchJSON('/api/ui/open').catch(() => ({ items: [] }));
    const openWorkspaces = Array.isArray(openState?.items)
      ? openState.items
          .filter((item) => item?.kind === 'workspace' && item.workspace)
          .map((item) => item.workspace)
      : [];

    if (openWorkspaces.length > 0) {
      for (let index = 0; index < openWorkspaces.length; index += 1) {
        const ws = openWorkspaces[index];
        await createMainWindow({
          autoShow: true,
          reuseExisting: false,
          pendingWorkspace: { id: ws.id, name: ws.name },
        });
      }
    } else if (openState?.lastClosedWorkspace?.id && openState?.lastClosedWorkspace?.name) {
      await createMainWindow({
        autoShow: true,
        reuseExisting: false,
        pendingWorkspace: {
          id: openState.lastClosedWorkspace.id,
          name: openState.lastClosedWorkspace.name,
        },
      });
    } else {
      createLauncherWindow();
    }

    if (process.platform === 'win32') {
      nativeTheme.on('updated', () => {
        for (const win of mainWindows) {
          if (win && !win.isDestroyed()) {
            syncWindowsChromeTheme(win);
          }
        }
        if (launcherWindow && !launcherWindow.isDestroyed()) {
          syncWindowsChromeTheme(launcherWindow);
        }
      });
    }
  } catch (err) {
    console.error('应用启动失败:', err);
    app.quit();
  }

  app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0) {
      createLauncherWindow();
    }
  });
});

// 所有窗口关闭时退出应用（先等工作区状态写入后端，避免下次误恢复全部窗口）
app.on('window-all-closed', () => {
  workspacePersistenceChain
    .then(() => {
      app.quit();
    })
    .catch((err) => {
      console.warn('退出前同步工作区状态失败，仍退出应用:', err?.message || err);
      app.quit();
    });
});

// 应用退出前的清理
app.on('before-quit', async (event) => {
  if (isQuitting) {
    return;
  }

  event.preventDefault();
  isQuitting = true;
  try {
    await workspacePersistenceChain;
  } catch (err) {
    console.warn('退出前等待工作区状态同步失败:', err?.message || err);
  }
  await stopBackendServer();
  app.quit();
});

process.on('exit', () => {
  // 兜底：进程被系统直接结束时，尽可能避免后端残留。
  if (backendProcess && backendProcess.pid) {
    forceKillBackendTree();
  }
});

// IPC 通信处理
ipcMain.handle('get-backend-url', () => {
  return `http://${backendHost}:${backendPort}`;
});

/** 启动器：先确保主窗口已加载工作台，再关闭启动器并显示主窗口 */
ipcMain.handle('electron:close-launcher-open-main', async () => {
  try {
    if (!mainWindow || mainWindow.isDestroyed()) {
      await createMainWindow({ autoShow: false });
    }

    if (launcherWindow && !launcherWindow.isDestroyed()) {
      launcherWindow.close();
    }

    if (mainWindow && !mainWindow.isDestroyed()) {
      mainWindow.show();
      mainWindow.focus();
      if (isDev) {
        mainWindow.webContents.openDevTools();
      }
    }

    return { ok: true };
  } catch (err) {
    console.error('electron:close-launcher-open-main 失败:', err);
    return { ok: false, error: err?.message || String(err) };
  }
});

ipcMain.handle('electron:open-launcher-window', async () => {
  try {
    const win = createLauncherWindow();
    if (win && !win.isDestroyed()) {
      win.show();
      win.focus();
      if (isDev) {
        win.webContents.openDevTools();
      }
    }
    return { ok: true };
  } catch (err) {
    console.error('electron:open-launcher-window 失败:', err);
    return { ok: false, error: err?.message || String(err) };
  }
});

ipcMain.handle('electron:open-workspace-window', async (_event, payload) => {
  try {
    const id = Number(payload?.id);
    const name = typeof payload?.name === 'string' ? payload.name : '';
    if (!id || !name) {
      return { ok: false, error: 'invalid workspace payload' };
    }

    const existingWindow = findWorkspaceWindow(id);
    if (existingWindow) {
      if (existingWindow.isMinimized()) {
        existingWindow.restore();
      }
      existingWindow.show();
      existingWindow.focus();
      return { ok: true, reused: true };
    }

    const win = await createMainWindow({
      autoShow: true,
      reuseExisting: false,
      pendingWorkspace: { id, name },
    });

    if (win && !win.isDestroyed()) {
      win.show();
      win.focus();
      if (isDev) {
        win.webContents.openDevTools();
      }
    }

    return { ok: true };
  } catch (err) {
    console.error('electron:open-workspace-window 失败:', err);
    return { ok: false, error: err?.message || String(err) };
  }
});

ipcMain.handle('electron:set-current-workspace-window', (event, payload) => {
  try {
    const id = Number(payload?.id);
    const name = typeof payload?.name === 'string' ? payload.name : '';
    const win = BrowserWindow.fromWebContents(event.sender);
    if (!win || !id || !name) {
      return { ok: false, error: 'invalid workspace payload' };
    }
    markWindowWorkspace(win, { id, name });
    if (!mainWindow || mainWindow.isDestroyed()) {
      mainWindow = win;
    }
    return { ok: true };
  } catch (err) {
    console.error('electron:set-current-workspace-window 失败:', err);
    return { ok: false, error: err?.message || String(err) };
  }
});


const externalWindows = new Map();

ipcMain.handle("electron:open-external-window", async (_event, payload) => {
  try {
    const url = typeof payload === "string" ? payload : payload?.url;
    const title =
      typeof payload === "object" && payload?.title
        ? String(payload.title)
        : "External";
    if (!url || typeof url !== "string") {
      return { ok: false, error: "invalid url" };
    }
    const windowId = `ext-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`;
    const win = new BrowserWindow({
      width: 480,
      height: 640,
      title,
      webPreferences: {
        contextIsolation: true,
        nodeIntegration: false,
      },
    });
    externalWindows.set(windowId, win);
    win.on("closed", () => externalWindows.delete(windowId));
    win.loadURL(url);
    return { ok: true, windowId };
  } catch (err) {
    console.error("electron:open-external-window 失败:", err);
    return { ok: false, error: err?.message || String(err) };
  }
});

ipcMain.handle("electron:close-external-window", async (_event, windowId) => {
  try {
    if (!windowId || typeof windowId !== "string") {
      return { ok: false, error: "invalid windowId" };
    }
    const win = externalWindows.get(windowId);
    if (win && !win.isDestroyed()) {
      win.close();
    }
    externalWindows.delete(windowId);
    return { ok: true };
  } catch (err) {
    console.error("electron:close-external-window 失败:", err);
    return { ok: false, error: err?.message || String(err) };
  }
});
// 处理未捕获的异常
process.on('uncaughtException', (error) => {
  console.error('未捕获的异常:', error);
});

process.on('unhandledRejection', (reason, promise) => {
  console.error('未处理的 Promise 拒绝:', reason);
});
