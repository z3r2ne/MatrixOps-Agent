import { describe, expect, it } from "vitest"
import { normalizeTaskQueuePayload } from "./taskQueue"

describe("normalizeTaskQueuePayload", () => {
  it("wraps legacy array payloads with autoSend=true", () => {
    const result = normalizeTaskQueuePayload([
      { id: "a", content: "one", createdAt: 1 },
    ])
    expect(result.autoSend).toBe(true)
    expect(result.queue).toHaveLength(1)
    expect(result.queue[0]?.id).toBe("a")
  })

  it("reads queue and autoSend from object payload", () => {
    const result = normalizeTaskQueuePayload({
      queue: [{ id: "b", content: "two", createdAt: 2 }],
      autoSend: false,
    })
    expect(result.autoSend).toBe(false)
    expect(result.queue[0]?.id).toBe("b")
  })

  it("supports legacy field names messageQueue and messageQueueAutoSend", () => {
    const result = normalizeTaskQueuePayload({
      messageQueue: [{ id: "c", content: "three", createdAt: 3 }],
      messageQueueAutoSend: false,
    })
    expect(result.autoSend).toBe(false)
    expect(result.queue[0]?.id).toBe("c")
  })

  it("returns empty queue for invalid payload", () => {
    const result = normalizeTaskQueuePayload(null)
    expect(result.queue).toEqual([])
    expect(result.autoSend).toBe(true)
  })
})
