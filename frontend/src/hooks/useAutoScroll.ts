import { useCallback, useEffect, useRef, useState } from "react"

const DEFAULT_THRESHOLD = 24

function isNearBottom(el: HTMLElement, threshold: number = DEFAULT_THRESHOLD): boolean {
  const { scrollTop, scrollHeight, clientHeight } = el
  return scrollHeight - scrollTop - clientHeight <= threshold
}

interface UseAutoScrollOptions {
  /** 触发自动滚动的依赖数组 */
  deps: React.DependencyList
  /** 是否在底部阈值内才自动滚动（默认 24px） */
  threshold?: number
  /** 是否启用自动滚动 */
  enabled?: boolean
  /** 滚动行为 */
  behavior?: ScrollBehavior
}

/**
 * 自动滚动 hook：只在用户位于容器底部时才自动跟随滚动。
 *
 * 使用方式：
 * ```tsx
 * const { ref, stickToBottom, scrollToBottom } = useAutoScroll({
 *   deps: [content],
 *   enabled: isStreaming,
 * })
 *
 * return <div ref={ref} onScroll={...}>...</div>
 * ```
 */
export function useAutoScroll(options: UseAutoScrollOptions) {
  const { deps, threshold = DEFAULT_THRESHOLD, enabled = true, behavior = "auto" } = options
  const ref = useRef<HTMLDivElement | null>(null)
  const [stickToBottom, setStickToBottom] = useState(true)

  const handleScroll = useCallback(() => {
    const el = ref.current
    if (!el) return
    setStickToBottom(isNearBottom(el, threshold))
  }, [threshold])

  const scrollToBottom = useCallback(() => {
    const el = ref.current
    if (!el) return
    el.scrollTo({ top: el.scrollHeight, behavior })
    setStickToBottom(true)
  }, [behavior])

  useEffect(() => {
    if (!enabled) return
    const el = ref.current
    if (!el) return
    if (!stickToBottom) return
    el.scrollTo({ top: el.scrollHeight, behavior })
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, enabled, stickToBottom, behavior])

  return {
    ref,
    stickToBottom,
    setStickToBottom,
    handleScroll,
    scrollToBottom,
  }
}
