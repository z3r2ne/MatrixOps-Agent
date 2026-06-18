import { Assets } from "pixi.js"

import {
  DECOR_ASSETS,
  robotIdleAsset,
  robotVariantForWorker,
  robotWorkingAssets,
  simulationAsset,
} from "@/lib/simulationAssets"

const loaded = new Set<string>()

export async function ensureSimulationTextures(): Promise<void> {
  const urls = new Set<string>([
    simulationAsset("office-backdrop.png"),
    ...Object.values(DECOR_ASSETS).map(simulationAsset),
    ...["blue", "purple", "green", "orange", "pink", "cyan"].map((v) =>
      robotIdleAsset(v as "blue"),
    ),
    simulationAsset("robot-blue-working-1.png"),
    simulationAsset("robot-blue-working-2.png"),
    simulationAsset("status-running-glow.png"),
    simulationAsset("status-done-check.png"),
  ])

  const pending: Promise<unknown>[] = []
  for (const url of urls) {
    if (loaded.has(url)) continue
    loaded.add(url)
    pending.push(Assets.load(url))
  }

  await Promise.all(pending)
}

export function textureForWorker(
  workerName: string,
  isRunning: boolean,
  workingFrame: 0 | 1,
): string {
  const variant = robotVariantForWorker(workerName)
  if (isRunning) {
    const frames = robotWorkingAssets(variant)
    if (frames) {
      return frames[workingFrame]
    }
  }
  return robotIdleAsset(variant)
}

export function decorTexture(decorId: string): string {
  const file = DECOR_ASSETS[decorId]
  return file ? simulationAsset(file) : ""
}
