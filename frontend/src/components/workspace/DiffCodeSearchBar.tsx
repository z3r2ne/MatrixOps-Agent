import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react"
import { ChevronDown, ChevronUp, Search, X } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { cn } from "@/lib/utils"
import { applyDiffSearchHighlights, clearDiffSearchHighlights } from "./diffCodeSearch"

type DiffCodeSearchBarProps = {
  containerRef: React.RefObject<HTMLElement | null>
  enabled: boolean
  contentKey: string
}

export function DiffCodeSearchBar({ containerRef, enabled, contentKey }: DiffCodeSearchBarProps) {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState("")
  const [activeIndex, setActiveIndex] = useState(0)
  const [matchCount, setMatchCount] = useState(0)
  const inputRef = useRef<HTMLInputElement>(null)

  const closeSearch = useCallback(() => {
    setOpen(false)
    setQuery("")
    setActiveIndex(0)
    setMatchCount(0)
    const root = containerRef.current
    if (root) clearDiffSearchHighlights(root)
  }, [containerRef])

  const openSearch = useCallback(() => {
    setOpen(true)
    requestAnimationFrame(() => {
      inputRef.current?.focus()
      inputRef.current?.select()
    })
  }, [])

  useEffect(() => {
    if (!enabled) {
      closeSearch()
    }
  }, [enabled, closeSearch])

  useEffect(() => {
    closeSearch()
  }, [contentKey, closeSearch])

  useEffect(() => {
    if (!enabled) return
    const onKeyDown = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "f") {
        event.preventDefault()
        openSearch()
        return
      }
      if (event.key === "Escape" && open) {
        event.preventDefault()
        closeSearch()
      }
    }
    window.addEventListener("keydown", onKeyDown)
    return () => window.removeEventListener("keydown", onKeyDown)
  }, [enabled, open, openSearch, closeSearch])

  useEffect(() => {
    setActiveIndex(0)
  }, [query])

  useLayoutEffect(() => {
    const root = containerRef.current
    if (!root || !enabled || !open) return

    const run = () => {
      const matches = applyDiffSearchHighlights(root, query, activeIndex)
      setMatchCount(matches.length)
      if (matches.length > 0 && activeIndex >= matches.length) {
        setActiveIndex(0)
      }
    }

    run()
    const t1 = window.setTimeout(run, 50)
    const t2 = window.setTimeout(run, 200)
    const shell = root.querySelector(".diff-view-shell")
    let observer: MutationObserver | undefined
    if (shell && query.trim()) {
      observer = new MutationObserver(() => run())
      observer.observe(shell, { childList: true, subtree: true, characterData: true })
    }
    return () => {
      window.clearTimeout(t1)
      window.clearTimeout(t2)
      observer?.disconnect()
    }
  }, [containerRef, enabled, open, query, activeIndex, contentKey])

  const goToMatch = (direction: 1 | -1) => {
    if (matchCount <= 0) return
    setActiveIndex((prev) => {
      const next = prev + direction
      if (next < 0) return matchCount - 1
      if (next >= matchCount) return 0
      return next
    })
  }

  if (!enabled || !open) return null

  return (
    <div
      className={cn(
        "absolute top-3 right-3 z-50 flex items-center gap-1 rounded-md border bg-background/95 px-2 py-1 shadow-md backdrop-blur-sm",
        "animate-in fade-in slide-in-from-top-1 duration-150",
      )}
      role="search"
      onMouseDown={(event) => event.stopPropagation()}
    >
      <Search className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
      <Input
        ref={inputRef}
        value={query}
        onChange={(event) => setQuery(event.target.value)}
        onKeyDown={(event) => {
          if (event.key === "Enter") {
            event.preventDefault()
            goToMatch(event.shiftKey ? -1 : 1)
          }
        }}
        placeholder="搜索代码…"
        className="h-7 w-[200px] border-0 bg-transparent px-1 shadow-none focus-visible:ring-0"
        aria-label="搜索 diff 代码"
      />
      <span className="text-[11px] text-muted-foreground tabular-nums min-w-[52px] text-center">
        {query.trim() ? (matchCount > 0 ? `${activeIndex + 1}/${matchCount}` : "无结果") : "—"}
      </span>
      <Button
        type="button"
        variant="ghost"
        size="icon"
        className="h-7 w-7"
        disabled={matchCount === 0}
        onClick={() => goToMatch(-1)}
        aria-label="上一个匹配"
      >
        <ChevronUp className="h-4 w-4" />
      </Button>
      <Button
        type="button"
        variant="ghost"
        size="icon"
        className="h-7 w-7"
        disabled={matchCount === 0}
        onClick={() => goToMatch(1)}
        aria-label="下一个匹配"
      >
        <ChevronDown className="h-4 w-4" />
      </Button>
      <Button type="button" variant="ghost" size="icon" className="h-7 w-7" onClick={closeSearch} aria-label="关闭搜索">
        <X className="h-4 w-4" />
      </Button>
    </div>
  )
}
