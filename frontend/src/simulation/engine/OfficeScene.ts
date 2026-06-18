import {
  Application,
  Assets,
  Container,
  Sprite,
  Texture,
} from "pixi.js"

import type { AgentVisualState } from "../types"
import {
  agentSpriteWidth,
  assignDeskSlots,
  backgroundLayout,
  decorAnchors,
} from "../map/coordinates"
import { ROBOT_FOOT_ANCHOR_Y } from "@/lib/simulationAssets"
import { AgentEntity } from "./AgentEntity"
import { decorTexture, ensureSimulationTextures } from "./assets"
import { simulationAsset } from "@/lib/simulationAssets"

export class OfficeScene {
  private app: Application
  private background: Sprite
  private entities: Container
  private agents = new Map<number, AgentEntity>()
  private decorSprites: Sprite[] = []
  private destroyed = false
  private syncGeneration = 0
  private viewportW = 1
  private viewportH = 1
  private lastStates: AgentVisualState[] = []
  private onFocusTask: (taskId: number) => void

  private constructor(app: Application, onFocusTask: (taskId: number) => void) {
    this.app = app
    this.onFocusTask = onFocusTask
    this.background = new Sprite()
    this.entities = new Container()
    this.entities.sortableChildren = true
    this.entities.eventMode = "passive"
    this.app.stage.addChild(this.background, this.entities)
    this.app.stage.eventMode = "static"
  }

  static async create(
    app: Application,
    onFocusTask: (taskId: number) => void,
  ): Promise<OfficeScene> {
    await ensureSimulationTextures()
    const scene = new OfficeScene(app, onFocusTask)
    await scene.initBackground()
    return scene
  }

  private layoutCoverSprite(sprite: Sprite, viewportW: number, viewportH: number): void {
    const texture = sprite.texture
    if (!texture || texture.width <= 0 || texture.height <= 0) return

    const texAspect = texture.width / texture.height
    const viewAspect = viewportW / viewportH

    if (texAspect > viewAspect) {
      sprite.height = viewportH
      sprite.width = viewportH * texAspect
      sprite.x = (viewportW - sprite.width) / 2
      sprite.y = 0
    } else {
      sprite.width = viewportW
      sprite.height = viewportW / texAspect
      sprite.x = 0
      sprite.y = (viewportH - sprite.height) / 2
    }
  }

  private async initBackground(): Promise<void> {
    const texture = await Assets.load<Texture>(simulationAsset("office-backdrop.png"))
    if (this.destroyed) return
    this.background.texture = texture
  }

  private layoutDecor(): void {
    for (const sprite of this.decorSprites) {
      sprite.removeFromParent()
      sprite.destroy({ children: true, texture: false })
    }
    this.decorSprites = []

    if (this.destroyed) return

    const bg = backgroundLayout(this.background)
    for (const anchor of decorAnchors(bg)) {
      const url = decorTexture(anchor.decorId)
      if (!url) continue
      const sprite = Sprite.from(Assets.get(url))
      const width = agentSpriteWidth(this.viewportW, this.viewportH, anchor.scale)
      sprite.width = width
      sprite.height = width * 1.3
      sprite.anchor.set(0.5, ROBOT_FOOT_ANCHOR_Y)
      sprite.x = anchor.x
      sprite.y = anchor.y
      sprite.zIndex = anchor.depth
      this.decorSprites.push(sprite)
      this.entities.addChild(sprite)
    }
  }

  resize(viewportW: number, viewportH: number): void {
    if (this.destroyed) return

    this.viewportW = Math.max(viewportW, 1)
    this.viewportH = Math.max(viewportH, 1)

    this.layoutCoverSprite(this.background, this.viewportW, this.viewportH)
    this.layoutDecor()

    if (this.lastStates.length > 0) {
      void this.syncAgents(this.lastStates)
    }
  }

  async syncAgents(states: AgentVisualState[]): Promise<void> {
    if (this.destroyed) return

    this.lastStates = states
    const generation = ++this.syncGeneration
    const activeIds = new Set(states.map((s) => s.taskId))
    const slots = assignDeskSlots(states.length, backgroundLayout(this.background))

    const rebuilt = states.map((state, index) => ({
      ...state,
      desk: slots[index] ?? state.desk,
    }))

    for (const [taskId, entity] of this.agents) {
      if (!activeIds.has(taskId)) {
        this.entities.removeChild(entity)
        entity.dispose()
        this.agents.delete(taskId)
      }
    }

    for (const state of rebuilt) {
      if (this.destroyed || generation !== this.syncGeneration) return

      let entity = this.agents.get(state.taskId)
      if (!entity) {
        entity = new AgentEntity(state.taskId, this.onFocusTask)
        this.agents.set(state.taskId, entity)
        this.entities.addChild(entity)
      }

      entity.x = state.desk.x
      entity.y = state.desk.y
      entity.zIndex = state.desk.depth

      await entity.apply(state, this.viewportW, this.viewportH)
      if (this.destroyed || generation !== this.syncGeneration) return
    }

    if (this.destroyed || generation !== this.syncGeneration) return
    this.entities.sortChildren()
  }

  destroy(): void {
    if (this.destroyed) return
    this.destroyed = true
    this.syncGeneration += 1

    for (const entity of this.agents.values()) {
      this.entities.removeChild(entity)
      entity.dispose()
    }
    this.agents.clear()

    for (const sprite of this.decorSprites) {
      sprite.destroy({ children: true, texture: false })
    }
    this.decorSprites = []

    this.entities.removeChildren()
    this.background.removeFromParent()
    this.entities.removeFromParent()
  }
}
