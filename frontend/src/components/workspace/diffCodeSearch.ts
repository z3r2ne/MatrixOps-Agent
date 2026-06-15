const DIFF_SEARCH_MATCH_CLASS = "diff-search-match"
const DIFF_SEARCH_ACTIVE_CLASS = "diff-search-active"

/** git-diff-view 实际 DOM：tr.diff-line + td.diff-line-*-content */
const DIFF_ROW_SELECTORS = [
  ".diff-view-shell tr.diff-line[data-state='diff']",
  ".diff-view-shell tr.diff-line[data-state='plain']",
  ".diff-view-shell tr.diff-line",
  "[data-diff-fallback-line]",
].join(", ")

export function clearDiffSearchHighlights(root: HTMLElement) {
  root.querySelectorAll(`.${DIFF_SEARCH_MATCH_CLASS}, .${DIFF_SEARCH_ACTIVE_CLASS}`).forEach((node) => {
    node.classList.remove(DIFF_SEARCH_MATCH_CLASS, DIFF_SEARCH_ACTIVE_CLASS)
  })
}

export function collectDiffSearchRows(root: HTMLElement): HTMLElement[] {
  const shell = root.querySelector(".diff-view-shell")
  if (shell) {
    const rows = Array.from(shell.querySelectorAll<HTMLElement>(DIFF_ROW_SELECTORS))
    const seen = new Set<HTMLElement>()
    const out: HTMLElement[] = []
    for (const row of rows) {
      if (seen.has(row)) continue
      seen.add(row)
      const text = (row.textContent ?? "").trim()
      if (text.length === 0) continue
      out.push(row)
    }
    return out
  }
  return Array.from(root.querySelectorAll<HTMLElement>("[data-diff-fallback-line]"))
}

export function applyDiffSearchHighlights(root: HTMLElement, query: string, activeIndex: number): HTMLElement[] {
  clearDiffSearchHighlights(root)
  const normalized = query.trim().toLowerCase()
  if (!normalized) return []

  const matches = collectDiffSearchRows(root).filter((row) =>
    (row.textContent ?? "").toLowerCase().includes(normalized),
  )

  matches.forEach((row, index) => {
    row.classList.add(DIFF_SEARCH_MATCH_CLASS)
    if (index === activeIndex) {
      row.classList.add(DIFF_SEARCH_ACTIVE_CLASS)
      row.scrollIntoView({ block: "center", behavior: "smooth" })
    }
  })

  return matches
}
