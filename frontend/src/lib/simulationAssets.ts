/** 仿真视图精灵图路径与叠放区域配置 */

export const SIMULATION_ASSET_BASE = "/assets/simulation"

export const ROBOT_VARIANTS = ["blue", "purple", "green", "orange", "pink", "cyan"] as const
export type RobotVariant = (typeof ROBOT_VARIANTS)[number]

export interface ScreenRegion {
  left: number
  top: number
  width: number
  height: number
}

/** 显示器可点击 / 文字叠放区域（相对机器人精灵图 1024×1536） */
export const SCREEN_REGIONS = {
  robot: { left: 0.22, top: 0.10, width: 0.56, height: 0.17 },
  "monitor-focus": { left: 0.10, top: 0.12, width: 0.80, height: 0.72 },
} as const satisfies Record<string, ScreenRegion>

/**
 * 机器人脚底锚点（相对精灵图高度 0~1）。
 * 素材底部含等距阴影，锚点应对齐阴影底边而非桌椅主体下沿。
 */
export const ROBOT_FOOT_ANCHOR_Y = 0.993

/** 名称标签锚点：桌椅主体下沿附近（避开底部阴影透明区） */
export const ROBOT_LABEL_ANCHOR_Y = 0.72

/** office-backdrop.png 内可摆放工位的地板区域（相对背景图 0~1） */
export const BACKDROP_FLOOR = {
  left: 0.10,
  top: 0.35,
  width: 0.80,
  height: 0.55,
} as const

export const DECOR_ASSETS: Record<string, string> = {
  plant: "decor-plant.png",
  coffee: "decor-coffee.png",
}

export function simulationAsset(path: string): string {
  return `${SIMULATION_ASSET_BASE}/${path}`
}

export function robotVariantForWorker(workerName: string): RobotVariant {
  let hash = 0
  for (let i = 0; i < workerName.length; i += 1) {
    hash = (hash + workerName.charCodeAt(i) * (i + 1)) % ROBOT_VARIANTS.length
  }
  return ROBOT_VARIANTS[hash] ?? "blue"
}

export function robotIdleAsset(variant: RobotVariant): string {
  return simulationAsset(`robot-${variant}-idle.png`)
}

export function robotWorkingAssets(variant: RobotVariant): [string, string] | null {
  if (variant !== "blue") return null
  return [
    simulationAsset("robot-blue-working-1.png"),
    simulationAsset("robot-blue-working-2.png"),
  ]
}

export function isTaskRunning(status?: string): boolean {
  const normalized = (status || "").toLowerCase()
  return normalized === "running" || normalized === "active"
}

export function isTaskFailed(status?: string): boolean {
  const normalized = (status || "").toLowerCase()
  return normalized === "failed" || normalized === "error"
}

export function isTaskDone(status?: string): boolean {
  const normalized = (status || "").toLowerCase()
  return normalized === "done" || normalized === "completed" || normalized === "success"
}
