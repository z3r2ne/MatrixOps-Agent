import { Application } from "pixi.js"

import { OfficeScene } from "./OfficeScene"

export interface SimulationRuntime {
  readonly isDestroyed: boolean
  readonly scene: OfficeScene
  destroy: () => void
}

function clearContainer(container: HTMLElement): void {
  while (container.firstChild) {
    container.removeChild(container.firstChild)
  }
}

export async function createSimulationRuntime(
  container: HTMLElement,
  onFocusTask: (taskId: number) => void,
): Promise<SimulationRuntime> {
  clearContainer(container)

  const app = new Application()

  await app.init({
    background: "#b8b0a4",
    backgroundAlpha: 1,
    antialias: true,
    resolution: Math.min(window.devicePixelRatio || 1, 2),
    autoDensity: true,
    resizeTo: container,
  })

  const canvas = app.canvas as HTMLCanvasElement
  canvas.style.display = "block"
  canvas.style.width = "100%"
  canvas.style.height = "100%"
  canvas.style.touchAction = "manipulation"
  container.appendChild(canvas)

  const scene = await OfficeScene.create(app, onFocusTask)
  scene.resize(container.clientWidth, container.clientHeight)

  let destroyed = false

  return {
    get isDestroyed() {
      return destroyed
    },
    scene,
    destroy: () => {
      if (destroyed) return
      destroyed = true

      app.ticker.stop()
      app.resizeTo = null

      try {
        scene.destroy()
      } catch (error) {
        console.warn("[simulation] scene destroy failed:", error)
      }

      try {
        app.stage.removeChildren()
        app.destroy(true, { children: true, texture: false })
      } catch (error) {
        console.warn("[simulation] pixi destroy failed:", error)
      }

      clearContainer(container)
    },
  }
}
