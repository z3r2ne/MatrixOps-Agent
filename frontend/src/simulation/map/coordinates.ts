/**
 * 办公室布局坐标：工位锚点映射到背景图地板区域，再换算为屏幕像素。
 */

import { BACKDROP_FLOOR } from "@/lib/simulationAssets"
import type { DeskSlot } from "../types"

/** 机器人宽度 = min(视口宽, 视口高) × 此比例 × 工位 scale */
export const AGENT_WIDTH_RATIO = 0.17

export interface ScreenPoint {
  x: number
  y: number
}

export interface BackgroundLayout {
  x: number
  y: number
  width: number
  height: number
}

export interface FloorAnchor extends DeskSlot {
  x: number
  y: number
}

export function agentSpriteWidth(viewportW: number, viewportH: number, scale = 1): number {
  return Math.min(viewportW, viewportH) * AGENT_WIDTH_RATIO * scale
}

/** 背景图地板区域内的归一化坐标 → 屏幕像素（rx/ry 为地板内 0~1） */
export function backdropFloorPoint(bg: BackgroundLayout, rx: number, ry: number): ScreenPoint {
  const fx = BACKDROP_FLOOR.left + BACKDROP_FLOOR.width * rx
  const fy = BACKDROP_FLOOR.top + BACKDROP_FLOOR.height * ry
  return {
    x: bg.x + fx * bg.width,
    y: bg.y + fy * bg.height,
  }
}

export function backgroundLayout(sprite: BackgroundLayout): BackgroundLayout {
  return sprite
}

/** 按地板比例分配工位锚点（rx/ry 为地板区域内 0~1） */
export function assignDeskSlots(
  taskCount: number,
  bg: BackgroundLayout,
): FloorAnchor[] {
  if (taskCount <= 0) return []

  if (taskCount === 1) {
    const p = backdropFloorPoint(bg, 0.5, 0.62)
    return [{
      id: "desk-0",
      gx: 0,
      gy: 0,
      scale: 1.0,
      depth: 10,
      x: p.x,
      y: p.y,
    }]
  }

  const slots: FloorAnchor[] = []
  const root = backdropFloorPoint(bg, 0.5, 0.38)
  slots.push({
    id: "desk-0",
    gx: 0,
    gy: 0,
    scale: 1.05,
    depth: 10,
    x: root.x,
    y: root.y,
  })

  const childCount = taskCount - 1
  const cols = Math.min(4, Math.max(2, Math.ceil(Math.sqrt(childCount))))
  const rowRy = 0.62

  for (let i = 0; i < childCount; i += 1) {
    const col = i % cols
    const row = Math.floor(i / cols)
    const rx = cols === 1 ? 0.5 : 0.18 + (col / (cols - 1)) * 0.64
    const ry = rowRy + row * 0.11
    const p = backdropFloorPoint(bg, rx, ry)
    slots.push({
      id: `desk-${i + 1}`,
      gx: 0,
      gy: 0,
      scale: 0.92,
      depth: 20 + row * cols + col,
      x: p.x,
      y: p.y,
    })
  }

  return slots
}

export function decorAnchors(bg: BackgroundLayout) {
  // ry: 0=靠墙后侧，1=靠近镜头前侧。绿植放左前地板，咖啡机放右前。
  const plant = backdropFloorPoint(bg, 0.14, 0.84)
  const coffee = backdropFloorPoint(bg, 0.88, 0.78)

  return [
    {
      decorId: "plant",
      x: plant.x,
      y: plant.y,
      scale: 0.5,
      depth: 5,
    },
    {
      decorId: "coffee",
      x: coffee.x,
      y: coffee.y,
      scale: 0.55,
      depth: 25,
    },
  ]
}

/** 屏幕预览用截断文本，避免溢出 */
export function truncateScreenPreview(text: string, maxLen = 160): string {
  const normalized = text.replace(/\s+/g, " ").trim()
  if (normalized.length <= maxLen) return normalized
  return `${normalized.slice(0, maxLen)}…`
}
