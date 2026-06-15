import { Navigate } from "react-router-dom"

/** 根路径：进入工作台（Electron 主窗口；启动器为独立窗口路由 `/electron-launcher`）。 */
export function HomePage() {
  return <Navigate to="/workbench" replace />
}
