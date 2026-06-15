import { describe, expect, it } from "vitest"
import type { WithParts } from "@/lib/api"
import { loadedSkillsFromMessages } from "./loadedSkillsFromMessages"

function toolMessage(
  callID: string,
  tool: string,
  input: Record<string, unknown>,
  status: string,
  extra?: { error?: string; output?: string; rawOutput?: string },
): WithParts {
  return {
    info: {
      id: `msg-${callID}`,
      sessionID: "s1",
      role: "assistant",
      time: { created: 1 },
      completed: true,
    },
    parts: [
      {
        id: `part-${callID}`,
        messageID: `msg-${callID}`,
        sessionID: "s1",
        type: "tool",
        metadata: extra?.rawOutput ? { rawOutput: extra.rawOutput } : undefined,
        tool: {
          tool,
          callID,
          state: {
            status,
            input,
            error: extra?.error,
            output: extra?.output,
            time: { start: 1, end: 2, created: 1 },
          },
        },
      },
    ],
  }
}

describe("loadedSkillsFromMessages", () => {
  it("collects successful direct load_skill calls", () => {
    const messages = [
      toolMessage("c1", "load_skill", { name: "ui-ux-pro-max" }, "completed", {
        output: "[Skill: ui-ux-pro-max]\n# Skill",
      }),
    ]
    expect(loadedSkillsFromMessages(messages)).toEqual([{ name: "ui-ux-pro-max", callID: "c1" }])
  })

  it("collects load_skill nested in call_tool params", () => {
    const messages = [
      toolMessage(
        "c2",
        "call_tool",
        {
          tool_name: "load_skill",
          tool_input: { name: "shadcn" },
        },
        "completed",
        {
          output: "[Skill: shadcn]\nbody",
          rawOutput: `{"call_tool":"call_tool","params":{"tool_name":"load_skill","tool_input":{"name":"shadcn"}}}`,
        },
      ),
    ]
    expect(loadedSkillsFromMessages(messages)).toEqual([{ name: "shadcn", callID: "c2" }])
  })

  it("ignores failed or incomplete load_skill calls", () => {
    const messages = [
      toolMessage("c3", "load_skill", { name: "missing" }, "error", { error: "not found" }),
      toolMessage("c4", "load_skill", { name: "pending" }, "running"),
    ]
    expect(loadedSkillsFromMessages(messages)).toEqual([])
  })

  it("deduplicates the same skill loaded multiple times", () => {
    const messages = [
      toolMessage("c5", "load_skill", { name: "canvas" }, "completed"),
      toolMessage("c6", "load_skill", { name: "canvas" }, "completed"),
    ]
    expect(loadedSkillsFromMessages(messages)).toEqual([{ name: "canvas", callID: "c5" }])
  })
})
