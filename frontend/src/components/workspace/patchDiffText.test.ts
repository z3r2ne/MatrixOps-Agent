import { describe, expect, it } from "vitest"
import { parsePatchToUnifiedSections } from "./patchDiffText"

describe("parsePatchToUnifiedSections", () => {
  it("converts update patch into unified diff", () => {
    const patch = [
      "*** Begin Patch",
      "*** Update File: src/example.ts",
      "@@",
      " unchanged",
      "-old line",
      "+new line",
      "*** End Patch",
    ].join("\n")

    const sections = parsePatchToUnifiedSections(patch)
    expect(sections).toHaveLength(1)
    expect(sections?.[0].path).toBe("src/example.ts")
    expect(sections?.[0].unifiedDiff).toContain("--- a/src/example.ts")
    expect(sections?.[0].unifiedDiff).toContain("+++ b/src/example.ts")
    expect(sections?.[0].unifiedDiff).toContain("-old line")
    expect(sections?.[0].unifiedDiff).toContain("+new line")
  })

  it("converts multi-file patch into multiple sections", () => {
    const patch = [
      "*** Begin Patch",
      "*** Add File: a.txt",
      "+hello",
      "*** Update File: b.txt",
      "@@",
      "-x",
      "+y",
      "*** End Patch",
    ].join("\n")

    const sections = parsePatchToUnifiedSections(patch)
    expect(sections).toHaveLength(2)
    expect(sections?.[0].path).toBe("a.txt")
    expect(sections?.[1].path).toBe("b.txt")
  })
})
