import React, { useState, useRef, useEffect, useCallback, useMemo, KeyboardEvent } from "react"
import { cn } from "@/lib/utils"
import { File, Folder, Command, Hash, Sparkles, GitBranch, Bot } from "lucide-react"

export interface MentionItem {
  id: string
  type: 'file' | 'folder' | 'command' | 'branch' | 'worker' | 'skill' | 'custom'
  label: string
  value: string
  description?: string
  icon?: React.ReactNode
  category?: string
  isSpecialCommand?: boolean  // 标记特殊命令（如 review）
}

export interface MentionCategory {
  name: string
  items: MentionItem[]
}

interface MarkdownMentionInputProps {
  value: string
  onChange: (value: string) => void
  onSubmit?: () => void
  placeholder?: string
  disabled?: boolean
  className?: string
  mentionItems?: MentionItem[]
  onMentionSearch?: (query: string) => Promise<MentionItem[]> | MentionItem[]
  maxHeight?: string
}

interface MentionState {
  isOpen: boolean
  query: string
  position: { bottom: number; left: number }
  triggerIndex: number
  selectedCategoryIndex: number
  selectedItemIndex: number
}

// Mention 匹配正则
const MENTION_REGEX = /@(\.?[^\s`,.]+(?:\.[^\s`,.]+)*)/g

/**
 * 带 Markdown 渲染的 Mention 输入组件
 * 支持显示 [label](url) 格式为徽章样式
 */
export const MarkdownMentionInput: React.FC<MarkdownMentionInputProps> = ({
  value,
  onChange,
  onSubmit,
  placeholder = "输入 @ 来提及...",
  disabled = false,
  className,
  mentionItems = [],
  onMentionSearch,
  maxHeight = "120px"
}) => {
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const displayRef = useRef<HTMLDivElement>(null)
  const mentionMenuRef = useRef<HTMLDivElement>(null)
  
  const [mentionState, setMentionState] = useState<MentionState>({
    isOpen: false,
    query: '',
    position: { bottom: 0, left: 0 },
    triggerIndex: -1,
    selectedCategoryIndex: 0,
    selectedItemIndex: 0
  })
  const [filteredCategories, setFilteredCategories] = useState<MentionCategory[]>([])
  const [isSearching, setIsSearching] = useState(false)
  
  const currentCategoryItems = useMemo(() => {
    const category = filteredCategories[mentionState.selectedCategoryIndex]
    return category ? category.items : []
  }, [filteredCategories, mentionState.selectedCategoryIndex])

  // 获取默认图标
  const getDefaultIcon = (type: MentionItem['type']) => {
    switch (type) {
      case 'file':
        return <File className="h-3 w-3" />
      case 'folder':
        return <Folder className="h-3 w-3" />
      case 'command':
        return <Command className="h-3 w-3" />
      case 'branch':
        return <GitBranch className="h-3 w-3" />
      case 'worker':
        return <Bot className="h-3 w-3" />
      case 'skill':
        return <Sparkles className="h-3 w-3" />
      default:
        return <Hash className="h-3 w-3" />
    }
  }

  // 渲染带 Markdown 样式的文本
  const renderMarkdownText = useCallback((text: string) => {
    if (!text) return null
    
    // 匹配 Markdown 链接 [label](url) 和 @ mention
    const combinedRegex = /(\[([^\]]+)\]\(([^)]+)\))|(@[^\s`,.]+(?:\.[^\s`,.]+)*)/g
    const parts: React.ReactNode[] = []
    let lastIndex = 0
    let match: RegExpExecArray | null
    let key = 0
    
    while ((match = combinedRegex.exec(text)) !== null) {
      // 添加匹配前的普通文本
      if (match.index > lastIndex) {
        parts.push(
          <span key={`text-${key++}`}>
            {text.substring(lastIndex, match.index)}
          </span>
        )
      }
      
      if (match[1]) {
        // Markdown 链接 [label](url) - 保持原始文本宽度
        const label = match[2]
        const url = match[3]
        const fullText = match[0] // 原始文本 [label](url)
        parts.push(
          <span
            key={`link-${key++}`}
            className="inline-flex items-center gap-0.5 px-1 py-0 bg-primary/10 text-primary rounded-sm border border-primary/20"
            style={{ 
              fontSize: 'inherit',
              lineHeight: 'inherit'
            }}
          >
            <File className="h-2.5 w-2.5 inline-block" />
            <span>{label}</span>
          </span>
        )
      } else if (match[4]) {
        // @ mention
        const mention = match[4]
        parts.push(
          <span
            key={`mention-${key++}`}
            className="inline-flex items-center gap-0.5 px-1 py-0 bg-blue-50 text-blue-700 rounded-sm"
            style={{ 
              fontSize: 'inherit',
              lineHeight: 'inherit'
            }}
          >
            <Command className="h-2.5 w-2.5 inline-block" />
            <span>{mention}</span>
          </span>
        )
      }
      
      lastIndex = match.index + match[0].length
    }
    
    // 添加剩余文本
    if (lastIndex < text.length) {
      parts.push(
        <span key={`text-${key++}`}>
          {text.substring(lastIndex)}
        </span>
      )
    }
    
    return parts.length > 0 ? parts : text
  }, [])

  // 搜索 mention 项并按分类组织
  const searchMentions = useCallback(async (query: string) => {
    setIsSearching(true)
    try {
      let items: MentionItem[] = []
      
      if (onMentionSearch) {
        const result = await onMentionSearch(query)
        items = result
      } else {
        if (!query.trim()) {
          items = mentionItems
        } else {
          items = mentionItems.filter(item =>
            item.label.toLowerCase().includes(query.toLowerCase()) ||
            item.value.toLowerCase().includes(query.toLowerCase())
          )
        }
      }
      
      // 按分类组织
      const categoryMap = new Map<string, MentionItem[]>()
      items.forEach(item => {
        const category = item.category || '其他'
        if (!categoryMap.has(category)) {
          categoryMap.set(category, [])
        }
        categoryMap.get(category)!.push(item)
      })
      
      const categories: MentionCategory[] = Array.from(categoryMap.entries()).map(([name, items]) => ({
        name,
        items
      }))
      
      setFilteredCategories(categories)
    } finally {
      setIsSearching(false)
    }
  }, [mentionItems, onMentionSearch])

  // 检查是否触发 mention
  const checkMentionTrigger = useCallback((text: string, cursorPosition: number) => {
    let triggerIndex = -1
    let query = ''
    
    for (let i = cursorPosition - 1; i >= 0; i--) {
      const char = text[i]
      if (char === ' ' || char === '\n') break
      if (char === '@') {
        triggerIndex = i
        query = text.substring(i + 1, cursorPosition)
        break
      }
    }
    
    return { triggerIndex, query }
  }, [])

  // 计算菜单位置
  const calculateMenuPosition = useCallback((textarea: HTMLTextAreaElement) => {
    const rect = textarea.getBoundingClientRect()
    return {
      bottom: window.innerHeight - rect.top + 4,
      left: rect.left + window.scrollX
    }
  }, [])

  // 处理文本变化
  const handleChange = useCallback((e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const newValue = e.target.value
    onChange(newValue)
    
    const cursorPosition = e.target.selectionStart
    const { triggerIndex, query } = checkMentionTrigger(newValue, cursorPosition)
    
    if (triggerIndex !== -1) {
      const position = calculateMenuPosition(e.target)
      setMentionState(prev => ({
        ...prev,
        isOpen: true,
        query,
        position,
        triggerIndex,
        selectedCategoryIndex: 0,
        selectedItemIndex: 0
      }))
      searchMentions(query)
    } else {
      setMentionState(prev => ({ ...prev, isOpen: false }))
    }
  }, [onChange, checkMentionTrigger, calculateMenuPosition, searchMentions])

  // 插入 mention
  const insertMention = useCallback((item: MentionItem) => {
    if (!textareaRef.current) return
    
    const { triggerIndex } = mentionState
    const beforeMention = value.substring(0, triggerIndex)
    const afterCursor = value.substring(textareaRef.current.selectionStart)
    
    let mentionText: string
    if (item.type === 'file' || item.type === 'folder') {
      mentionText = `[${item.label}](${item.value})`
    } else {
      mentionText = `@${item.label}`
    }
    
    const newValue = beforeMention + mentionText + ' ' + afterCursor
    const newCursorPos = beforeMention.length + mentionText.length + 1
    
    onChange(newValue)
    setMentionState(prev => ({ ...prev, isOpen: false }))
    
    setTimeout(() => {
      if (textareaRef.current) {
        textareaRef.current.focus()
        textareaRef.current.setSelectionRange(newCursorPos, newCursorPos)
      }
    }, 0)
  }, [mentionState, value, onChange])

  // 键盘导航
  const handleKeyDown = useCallback((e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (!mentionState.isOpen) {
      if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) {
        e.preventDefault()
        onSubmit?.()
      }
      return
    }
    
    switch (e.key) {
      case 'ArrowLeft':
        e.preventDefault()
        setMentionState(prev => ({
          ...prev,
          selectedCategoryIndex: Math.max(prev.selectedCategoryIndex - 1, 0),
          selectedItemIndex: 0
        }))
        break
      case 'ArrowRight':
        e.preventDefault()
        setMentionState(prev => ({
          ...prev,
          selectedCategoryIndex: Math.min(prev.selectedCategoryIndex + 1, filteredCategories.length - 1),
          selectedItemIndex: 0
        }))
        break
      case 'ArrowDown':
        e.preventDefault()
        setMentionState(prev => ({
          ...prev,
          selectedItemIndex: Math.min(prev.selectedItemIndex + 1, currentCategoryItems.length - 1)
        }))
        break
      case 'ArrowUp':
        e.preventDefault()
        setMentionState(prev => ({
          ...prev,
          selectedItemIndex: Math.max(prev.selectedItemIndex - 1, 0)
        }))
        break
      case 'Enter':
      case 'Tab':
        e.preventDefault()
        if (currentCategoryItems[mentionState.selectedItemIndex]) {
          insertMention(currentCategoryItems[mentionState.selectedItemIndex])
        }
        break
      case 'Escape':
        e.preventDefault()
        setMentionState(prev => ({ ...prev, isOpen: false }))
        break
    }
  }, [mentionState, filteredCategories, currentCategoryItems, insertMention, onSubmit])

  // 滚动选中项
  useEffect(() => {
    if (mentionState.isOpen && mentionMenuRef.current) {
      const selectedElement = mentionMenuRef.current.querySelector(`[data-item-index="${mentionState.selectedItemIndex}"]`) as HTMLElement
      if (selectedElement) {
        selectedElement.scrollIntoView({ block: 'nearest', behavior: 'smooth' })
      }
    }
  }, [mentionState.selectedItemIndex, mentionState.isOpen])

  // 点击外部关闭
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (
        mentionState.isOpen &&
        mentionMenuRef.current &&
        !mentionMenuRef.current.contains(e.target as Node) &&
        textareaRef.current &&
        !textareaRef.current.contains(e.target as Node)
      ) {
        setMentionState(prev => ({ ...prev, isOpen: false }))
      }
    }
    
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [mentionState.isOpen])

  return (
    <div className="relative flex-1">
      {/* 正常的 textarea（可见输入） */}
      <textarea
        ref={textareaRef}
        value={value}
        onChange={handleChange}
        onKeyDown={handleKeyDown}
        placeholder={placeholder}
        disabled={disabled}
        className={cn(
          "w-full min-h-[60px] text-sm leading-5 resize-none bg-background",
          "overflow-y-auto whitespace-pre-wrap break-words outline-none rounded-md border border-input",
          "px-3 py-2 disabled:opacity-50",
          className
        )}
        style={{ 
          maxHeight
        }}
      />
      
      {/* Markdown 预览（在输入框下方） */}
      {value && (
        <div
          ref={displayRef}
          className={cn(
            "mt-2 px-3 py-2 bg-muted/30 rounded-md border border-border/50",
            "text-sm leading-5 whitespace-pre-wrap break-words"
          )}
        >
          <div className="text-xs text-muted-foreground mb-1 font-medium">预览:</div>
          {renderMarkdownText(value)}
        </div>
      )}
      
      {/* Mention 菜单 */}
      {mentionState.isOpen && (
        <div
          ref={mentionMenuRef}
          className="fixed z-[100] w-72 bg-popover border border-border rounded-md shadow-lg overflow-hidden"
          style={{
            bottom: `${mentionState.position.bottom}px`,
            left: `${mentionState.position.left}px`,
          }}
        >
          {isSearching ? (
            <div className="px-3 py-2 text-xs text-muted-foreground">
              搜索中...
            </div>
          ) : filteredCategories.length === 0 ? (
            <div className="px-3 py-2 text-xs text-muted-foreground">
              {mentionState.query ? '未找到匹配项' : '开始输入以搜索...'}
            </div>
          ) : (
            <>
              {/* 分类标签栏 */}
              <div className="flex border-b border-border overflow-x-auto">
                {filteredCategories.map((category, catIndex) => (
                  <button
                    key={category.name}
                    type="button"
                    className={cn(
                      "px-4 py-2 text-xs font-medium whitespace-nowrap transition-colors",
                      "border-b-2 -mb-[1px]",
                      catIndex === mentionState.selectedCategoryIndex
                        ? "border-primary text-primary bg-accent/50"
                        : "border-transparent text-muted-foreground hover:text-foreground hover:bg-accent/30"
                    )}
                    onClick={() => {
                      setMentionState(prev => ({
                        ...prev,
                        selectedCategoryIndex: catIndex,
                        selectedItemIndex: 0
                      }))
                    }}
                  >
                    {category.name}
                    <span className="ml-1.5 text-[10px] opacity-70">
                      ({category.items.length})
                    </span>
                  </button>
                ))}
              </div>

              {/* 当前分类的项目列表 */}
              <div className="max-h-64 overflow-y-auto">
                {currentCategoryItems.length === 0 ? (
                  <div className="px-3 py-2 text-xs text-muted-foreground">
                    此分类暂无项目
                  </div>
                ) : (
                  currentCategoryItems.map((item, itemIndex) => (
                    <button
                      key={item.id}
                      type="button"
                      data-item-index={itemIndex}
                      className={cn(
                        "w-full flex items-start gap-2 px-3 py-2 text-left text-xs transition-colors",
                        "hover:bg-accent focus:bg-accent outline-none",
                        itemIndex === mentionState.selectedItemIndex && "bg-accent"
                      )}
                      onClick={() => insertMention(item)}
                      onMouseEnter={() => setMentionState(prev => ({ ...prev, selectedItemIndex: itemIndex }))}
                    >
                      <div className="flex h-5 w-5 shrink-0 items-center justify-center text-muted-foreground mt-0.5">
                        {item.icon || getDefaultIcon(item.type)}
                      </div>
                      <div className="flex-1 min-w-0">
                        <div className="font-medium text-foreground truncate">
                          {item.label}
                        </div>
                        {item.description && (
                          <div className="text-muted-foreground text-[11px] truncate mt-0.5">
                            {item.description}
                          </div>
                        )}
                      </div>
                    </button>
                  ))
                )}
              </div>
            </>
          )}
        </div>
      )}
    </div>
  )
}
