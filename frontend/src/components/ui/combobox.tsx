"use client"

import * as React from "react"
import { createPortal } from "react-dom"
import { Check, ChevronDown } from "lucide-react"

import { cn } from "@/lib/utils"
import { Input } from "@/components/ui/input"

export type ComboboxOption = {
  value: string
  label: string
  description?: string
  searchText?: string
  disabled?: boolean
  isRemote?: boolean
  isCurrent?: boolean
}

type ComboboxProps<T extends ComboboxOption> = {
  id?: string
  items: T[]
  value: string
  onValueChange: (value: string) => void
  placeholder: string
  searchPlaceholder?: string
  emptyText?: string
  disabled?: boolean
  className?: string
  inputClassName?: string
  contentClassName?: string
  renderItem?: (item: T, selected: boolean) => React.ReactNode
}

function matchesMultiKeywordQuery(haystack: string, query: string): boolean {
  const tokens = query.trim().toLowerCase().split(/\s+/).filter(Boolean)
  if (tokens.length === 0) return true
  const normalizedHaystack = haystack.toLowerCase()
  return tokens.every((token) => normalizedHaystack.includes(token))
}

function nextEnabledIndex<T extends ComboboxOption>(
  items: T[],
  start: number,
  step: 1 | -1
) {
  if (items.length === 0) return -1
  let index = start
  for (let tries = 0; tries < items.length; tries += 1) {
    index = (index + step + items.length) % items.length
    if (!items[index]?.disabled) {
      return index
    }
  }
  return -1
}

function firstEnabledIndex<T extends ComboboxOption>(items: T[]) {
  return items.findIndex((item) => !item.disabled)
}

/** Radix Dialog 会给 body 加 block-interactivity-*，仅 allow-interactivity-* 区域可交互/滚动 */
function findDialogAllowInteractivityClass() {
  if (typeof document === "undefined") return undefined
  const blockClass = Array.from(document.body.classList).find((name) =>
    name.startsWith("block-interactivity-")
  )
  if (!blockClass) return undefined
  return blockClass.replace("block-interactivity-", "allow-interactivity-")
}

function scrollHighlightedIntoView(
  listEl: HTMLDivElement | null,
  itemEl: HTMLElement | null
) {
  if (!listEl || !itemEl) return
  const listTop = listEl.scrollTop
  const listBottom = listTop + listEl.clientHeight
  const itemTop = itemEl.offsetTop
  const itemBottom = itemTop + itemEl.offsetHeight
  if (itemTop < listTop) {
    listEl.scrollTop = itemTop
  } else if (itemBottom > listBottom) {
    listEl.scrollTop = itemBottom - listEl.clientHeight
  }
}

function getComboboxPortalRoot() {
  if (typeof document === "undefined") return null
  let root = document.getElementById("combobox-portal-root")
  if (!root) {
    root = document.createElement("div")
    root.id = "combobox-portal-root"
    root.setAttribute("data-combobox-portal", "")
    Object.assign(root.style, {
      position: "fixed",
      inset: "0",
      zIndex: "200",
      pointerEvents: "none",
      overflow: "visible",
    })
    document.body.appendChild(root)
  }
  return root
}

export function Combobox<T extends ComboboxOption>({
  id,
  items,
  value,
  onValueChange,
  placeholder,
  searchPlaceholder = "搜索...",
  emptyText = "未找到结果",
  disabled = false,
  className,
  inputClassName,
  contentClassName,
  renderItem,
}: ComboboxProps<T>) {
  const rootRef = React.useRef<HTMLDivElement | null>(null)
  const popupRef = React.useRef<HTMLDivElement | null>(null)
  const listRef = React.useRef<HTMLDivElement | null>(null)
  const inputRef = React.useRef<HTMLInputElement | null>(null)
  const itemRefs = React.useRef<Array<HTMLButtonElement | null>>([])

  const [open, setOpen] = React.useState(false)
  const [query, setQuery] = React.useState("")
  const [dirtyQuery, setDirtyQuery] = React.useState(false)
  const [highlightedIndex, setHighlightedIndex] = React.useState(-1)
  const [openDirection, setOpenDirection] = React.useState<"up" | "down">("down")
  const [maxPopupHeight, setMaxPopupHeight] = React.useState(220)
  const [popupStyle, setPopupStyle] = React.useState<React.CSSProperties>({})
  const [dialogAllowInteractivityClass, setDialogAllowInteractivityClass] = React.useState<string>()

  const selectedItem = React.useMemo(
    () => items.find((item) => item.value === value) ?? null,
    [items, value]
  )

  const filteredItems = React.useMemo(() => {
    if (!dirtyQuery) return items
    const trimmedQuery = query.trim()
    if (!trimmedQuery) return items
    return items.filter((item) => {
      const haystack = [
        item.label,
        item.description || "",
        item.searchText || "",
      ].join(" ")
      return matchesMultiKeywordQuery(haystack, query)
    })
  }, [dirtyQuery, items, query])

  React.useEffect(() => {
    itemRefs.current = itemRefs.current.slice(0, filteredItems.length)
    const selectedIndex = filteredItems.findIndex((item) => item.value === value && !item.disabled)
    if (selectedIndex >= 0) {
      setHighlightedIndex(selectedIndex)
      return
    }
    setHighlightedIndex(firstEnabledIndex(filteredItems))
  }, [filteredItems, value])

  React.useEffect(() => {
    if (!open) return
    const handlePointerDown = (event: MouseEvent) => {
      const target = event.target as Node
      if (rootRef.current?.contains(target) || popupRef.current?.contains(target)) {
        return
      }
      setOpen(false)
      setDirtyQuery(false)
      setQuery(selectedItem?.label || "")
    }
    document.addEventListener("mousedown", handlePointerDown)
    return () => document.removeEventListener("mousedown", handlePointerDown)
  }, [open, selectedItem])

  React.useEffect(() => {
    if (!open || highlightedIndex < 0) return
    scrollHighlightedIntoView(
      listRef.current,
      itemRefs.current[highlightedIndex]
    )
  }, [highlightedIndex, open])

  React.useEffect(() => {
    if (!open) return

    const updatePosition = () => {
      const rect = rootRef.current?.getBoundingClientRect()
      if (!rect) return
      const spaceBelow = window.innerHeight - rect.bottom - 8
      const spaceAbove = rect.top - 8
      const preferredDirection = spaceBelow >= 240 || spaceBelow >= spaceAbove ? "down" : "up"
      setOpenDirection(preferredDirection)
      const available = preferredDirection === "down" ? spaceBelow : spaceAbove
      const height = Math.max(120, Math.min(220, Math.floor(available)))
      setMaxPopupHeight(height)
      const baseStyle: React.CSSProperties = {
        position: "fixed",
        left: rect.left,
        width: rect.width,
        zIndex: 200,
        maxHeight: height,
      }
      if (preferredDirection === "down") {
        setPopupStyle({
          ...baseStyle,
          top: rect.bottom + 4,
        })
      } else {
        setPopupStyle({
          ...baseStyle,
          bottom: window.innerHeight - rect.top + 4,
        })
      }
    }

    const syncDialogInteractivity = () => {
      setDialogAllowInteractivityClass(findDialogAllowInteractivityClass())
    }

    updatePosition()
    syncDialogInteractivity()
    window.addEventListener("resize", updatePosition)
    window.addEventListener("scroll", updatePosition, true)
    const observer = new MutationObserver(syncDialogInteractivity)
    observer.observe(document.body, { attributes: true, attributeFilter: ["class"] })
    return () => {
      window.removeEventListener("resize", updatePosition)
      window.removeEventListener("scroll", updatePosition, true)
      observer.disconnect()
    }
  }, [open])

  React.useEffect(() => {
    if (!open) return
    let cleanup: (() => void) | undefined
    const frame = requestAnimationFrame(() => {
      const popup = popupRef.current
      if (!popup) return

      const handleWheelCapture = (event: WheelEvent) => {
        const target = event.target as Node | null
        if (!target || !popup.contains(target)) return
        event.preventDefault()
        event.stopPropagation()
        const list = listRef.current
        if (list) {
          list.scrollTop += event.deltaY
        }
      }

      popup.addEventListener("wheel", handleWheelCapture, { passive: false, capture: true })
      cleanup = () => popup.removeEventListener("wheel", handleWheelCapture, { capture: true })
    })
    return () => {
      cancelAnimationFrame(frame)
      cleanup?.()
    }
  }, [open])

  const openList = React.useCallback(() => {
    if (disabled) return
    setOpen(true)
    setDirtyQuery(false)
    setQuery(selectedItem?.label || "")
  }, [disabled, selectedItem])

  const closeList = React.useCallback(() => {
    setOpen(false)
    setDirtyQuery(false)
    setQuery(selectedItem?.label || "")
  }, [selectedItem])

  const focusInput = React.useCallback(() => {
    if (disabled) return
    inputRef.current?.focus()
  }, [disabled])

  const handleSelect = React.useCallback((item: T) => {
    if (item.disabled) return
    onValueChange(item.value)
    setQuery(item.label)
    setDirtyQuery(false)
    setOpen(false)
    requestAnimationFrame(() => {
      inputRef.current?.blur()
    })
  }, [onValueChange])

  const handleFocus = React.useCallback(() => {
    openList()
    requestAnimationFrame(() => {
      inputRef.current?.select()
    })
  }, [openList])

  const handleToggleMouseDown = React.useCallback((event: React.MouseEvent<HTMLButtonElement>) => {
    event.preventDefault()
    if (disabled) return
    if (open) {
      closeList()
      requestAnimationFrame(() => {
        inputRef.current?.blur()
      })
      return
    }
    focusInput()
  }, [closeList, disabled, focusInput, open])

  const handleChange = React.useCallback((event: React.ChangeEvent<HTMLInputElement>) => {
    setQuery(event.target.value)
    setDirtyQuery(true)
    if (!open) setOpen(true)
  }, [open])

  const handleKeyDown = React.useCallback((event: React.KeyboardEvent<HTMLInputElement>) => {
    if (disabled) return
    switch (event.key) {
      case "ArrowDown": {
        event.preventDefault()
        if (!open) {
          openList()
          return
        }
        setHighlightedIndex((current) =>
          current < 0 ? firstEnabledIndex(filteredItems) : nextEnabledIndex(filteredItems, current, 1)
        )
        break
      }
      case "ArrowUp": {
        event.preventDefault()
        if (!open) {
          openList()
          return
        }
        setHighlightedIndex((current) =>
          current < 0 ? firstEnabledIndex(filteredItems) : nextEnabledIndex(filteredItems, current, -1)
        )
        break
      }
      case "Enter": {
        if (!open) return
        if (highlightedIndex < 0) return
        event.preventDefault()
        const item = filteredItems[highlightedIndex]
        if (item) handleSelect(item)
        break
      }
      case "Escape": {
        if (!open) return
        event.preventDefault()
        closeList()
        break
      }
      default:
        break
    }
  }, [closeList, disabled, filteredItems, handleSelect, highlightedIndex, open, openList])

  const inputValue = open
    ? query
    : selectedItem?.label || ""

  const popupContent = open ? (
    <div
      ref={popupRef}
      className={cn(
        "pointer-events-auto overflow-hidden rounded-md border bg-popover text-popover-foreground shadow-md animate-in fade-in-0 zoom-in-95",
        openDirection === "up" ? "origin-bottom" : "origin-top",
        dialogAllowInteractivityClass,
        contentClassName
      )}
      style={{ ...popupStyle, height: maxPopupHeight, pointerEvents: "auto" }}
    >
      <div
        ref={listRef}
        className="h-full min-h-0 overflow-y-auto overscroll-contain p-1 [scrollbar-gutter:stable]"
      >
        {filteredItems.length === 0 ? (
          <div className="py-6 text-center text-sm text-muted-foreground">{emptyText}</div>
        ) : (
          filteredItems.map((item, index) => {
            const selected = value === item.value
            const highlighted = index === highlightedIndex
            return (
              <button
                key={item.value}
                ref={(node) => {
                  itemRefs.current[index] = node
                }}
                type="button"
                disabled={item.disabled}
                onMouseDown={(event) => event.preventDefault()}
                onClick={() => handleSelect(item)}
                className={cn(
                  "relative flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-left text-sm outline-none",
                  highlighted && "bg-accent text-accent-foreground",
                  item.disabled && "pointer-events-none opacity-50"
                )}
              >
                {renderItem ? (
                  <div className="flex min-w-0 flex-1 items-center gap-2">
                    {renderItem(item, selected)}
                  </div>
                ) : (
                  <>
                    <Check
                      className={cn(
                        "mr-2 h-4 w-4 shrink-0",
                        selected ? "opacity-100" : "opacity-0"
                      )}
                    />
                    <span className="truncate">{item.label}</span>
                  </>
                )}
              </button>
            )
          })
        )}
      </div>
    </div>
  ) : null

  return (
    <div
      ref={rootRef}
      className={cn("relative w-full", className)}
      onMouseDown={(event) => {
        if (disabled) return
        if (event.target instanceof HTMLElement && event.target.closest("button")) return
        if (event.target !== inputRef.current) {
          event.preventDefault()
          focusInput()
        }
      }}
    >
      <Input
        id={id}
        ref={inputRef}
        value={inputValue}
        onFocus={handleFocus}
        onChange={handleChange}
        onKeyDown={handleKeyDown}
        placeholder={open ? searchPlaceholder : placeholder}
        disabled={disabled}
        role="combobox"
        aria-expanded={open}
        aria-autocomplete="list"
        className={cn(
          "pr-9 transition-[border-color,box-shadow]",
          !open && "cursor-pointer caret-transparent select-none",
          open && "cursor-text",
          inputClassName
        )}
        autoComplete="off"
      />
      <button
        type="button"
        tabIndex={-1}
        aria-hidden="true"
        onMouseDown={handleToggleMouseDown}
        className={cn(
          "absolute inset-y-0 right-0 flex w-9 items-center justify-center text-muted-foreground/70 transition-colors",
          disabled ? "cursor-not-allowed opacity-50" : "hover:text-foreground"
        )}
      >
        <ChevronDown className={cn("h-4 w-4 transition-transform", open && "rotate-180")} />
      </button>
      {typeof document !== "undefined" && popupContent
        ? createPortal(popupContent, getComboboxPortalRoot() ?? document.body)
        : null}
    </div>
  )
}
