import {
  Assets,
  Container,
  Graphics,
  Rectangle,
  Sprite,
  Text,
  TextStyle,
} from "pixi.js"

import { ROBOT_FOOT_ANCHOR_Y, ROBOT_LABEL_ANCHOR_Y, SCREEN_REGIONS } from "@/lib/simulationAssets"
import { taskStatusLabel } from "@/lib/simulationOutput"
import type { AgentVisualState } from "../types"
import { agentSpriteWidth } from "../map/coordinates"
import { textureForWorker } from "./assets"

const LABEL_STYLE = new TextStyle({
  fontFamily: "ui-sans-serif, system-ui, sans-serif",
  fontSize: 12,
  fill: 0xffffff,
  align: "center",
  dropShadow: { color: 0x000000, alpha: 0.8, blur: 2, distance: 1 },
})

const SUBLABEL_STYLE = new TextStyle({
  fontFamily: "ui-sans-serif, system-ui, sans-serif",
  fontSize: 10,
  fill: 0xe2e8f0,
  align: "center",
  dropShadow: { color: 0x000000, alpha: 0.8, blur: 2, distance: 1 },
})

const SCREEN_STYLE = new TextStyle({
  fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace",
  fontSize: 10,
  fill: 0xa7f3d0,
  wordWrap: true,
  breakWords: true,
  lineHeight: 12,
})

export class AgentEntity extends Container {
  readonly taskId: number

  private robot: Sprite
  private screenLayer: Container
  private screenBg: Graphics
  private screenMask: Graphics
  private screenText: Text
  private labelLayer: Container
  private titleText: Text
  private subtitleText: Text
  private glow?: Sprite
  private check?: Sprite
  private lastTextureUrl = ""
  private disposed = false

  private onFocus: (taskId: number) => void
  private handlePointerTap = () => {
    if (!this.disposed) this.onFocus(this.taskId)
  }

  constructor(taskId: number, onFocus: (taskId: number) => void) {
    super()
    this.taskId = taskId
    this.onFocus = onFocus

    this.robot = new Sprite()
    this.screenLayer = new Container()
    this.screenBg = new Graphics()
    this.screenMask = new Graphics()
    this.screenText = new Text({ text: "", style: SCREEN_STYLE })
    this.labelLayer = new Container()
    this.titleText = new Text({ text: "", style: LABEL_STYLE })
    this.subtitleText = new Text({ text: "", style: SUBLABEL_STYLE })

    this.screenLayer.addChild(this.screenBg, this.screenText)
    this.screenLayer.mask = this.screenMask
    this.labelLayer.addChild(this.titleText, this.subtitleText)

    this.addChild(this.robot, this.screenLayer, this.labelLayer)

    this.eventMode = "static"
    this.cursor = "pointer"
    this.on("pointertap", this.handlePointerTap)

    this.screenBg.eventMode = "none"
    this.labelLayer.eventMode = "none"
    this.screenText.eventMode = "none"
    this.titleText.eventMode = "none"
    this.subtitleText.eventMode = "none"
  }

  async apply(state: AgentVisualState, viewportW: number, viewportH: number): Promise<void> {
    if (this.disposed) return

    const workerName = state.task.workerName?.trim() || "agent"
    const title = state.task.name?.trim() || state.task.content?.trim() || `任务 #${state.task.id}`
    const subtitle = `${workerName} · ${taskStatusLabel(state.task.status)}`
    const width = agentSpriteWidth(viewportW, viewportH, state.desk.scale)
    const height = width * 1.48
    const region = SCREEN_REGIONS.robot

    const textureUrl = textureForWorker(workerName, state.isRunning, state.workingFrame)
    if (textureUrl !== this.lastTextureUrl) {
      const texture = await Assets.load(textureUrl)
      if (this.disposed) return
      this.robot.texture = texture
      this.lastTextureUrl = textureUrl
    }
    this.robot.width = width
    this.robot.height = height
    this.robot.eventMode = "none"

    this.pivot.set(width / 2, height * ROBOT_FOOT_ANCHOR_Y)
    this.hitArea = new Rectangle(0, 0, width, height * ROBOT_LABEL_ANCHOR_Y + 28)

    const screenX = width * region.left
    const screenY = height * region.top
    const screenW = width * region.width
    const screenH = height * region.height

    this.screenMask.clear()
    this.screenMask.roundRect(screenX, screenY, screenW, screenH, 2).fill(0xffffff)

    this.screenBg.clear()
    this.screenBg
      .roundRect(screenX, screenY, screenW, screenH, 2)
      .fill({ color: 0x0a0f18, alpha: 0.96 })
      .stroke({ color: 0x334155, width: 1 })

    if (this.screenText.text !== state.screenText) {
      this.screenText.text = state.screenText
    }
    this.screenText.x = screenX + 3
    this.screenText.y = screenY + 2
    const wrapWidth = Math.max(8, Math.floor(screenW - 6))
    if (this.screenText.style.wordWrapWidth !== wrapWidth) {
      this.screenText.style.wordWrapWidth = wrapWidth
    }

    if (this.titleText.text !== title) {
      this.titleText.text = title
    }
    this.titleText.x = width / 2
    this.titleText.y = height * ROBOT_LABEL_ANCHOR_Y + 4
    this.titleText.anchor.set(0.5, 0)

    if (this.subtitleText.text !== subtitle) {
      this.subtitleText.text = subtitle
    }
    this.subtitleText.x = width / 2
    this.subtitleText.y = height * ROBOT_LABEL_ANCHOR_Y + 20
    this.subtitleText.anchor.set(0.5, 0)

    if (state.isRunning) {
      if (!this.glow) {
        const glowTexture = await Assets.load("/assets/simulation/status-running-glow.png")
        if (this.disposed) return
        this.glow = Sprite.from(glowTexture)
        this.glow.alpha = 0.7
        this.glow.eventMode = "none"
        this.addChildAt(this.glow, 2)
      }
      this.glow.visible = true
      this.glow.width = screenW * 1.3
      this.glow.height = screenH * 1.6
      this.glow.x = screenX + screenW / 2 - this.glow.width / 2
      this.glow.y = screenY + screenH / 2 - this.glow.height / 2
    } else if (this.glow) {
      this.glow.visible = false
    }

    const isDone = ["done", "completed", "success"].includes((state.task.status || "").toLowerCase())
    if (isDone) {
      if (!this.check) {
        const checkTexture = await Assets.load("/assets/simulation/status-done-check.png")
        if (this.disposed) return
        this.check = Sprite.from(checkTexture)
        this.check.eventMode = "none"
        this.addChild(this.check)
      }
      const checkSize = width * 0.12
      this.check.visible = true
      this.check.width = checkSize
      this.check.height = checkSize
      this.check.x = screenX + screenW - checkSize * 0.85
      this.check.y = screenY - checkSize * 0.15
    } else if (this.check) {
      this.check.visible = false
    }
  }

  dispose(): void {
    if (this.disposed) return
    this.disposed = true
    this.renderable = false
    this.screenLayer.mask = null
    this.off("pointertap", this.handlePointerTap)
    this.removeFromParent()
    this.destroy({ children: true, texture: false })
  }
}
