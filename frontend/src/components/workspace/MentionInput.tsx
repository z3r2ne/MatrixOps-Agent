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
  category?: string  // 新增：所属分类
}

export interface MentionCategory {
  name: string
  items: MentionItem[]
}

interface MentionInputProps {
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
  selectedCategoryIndex: number  // 当前选中的分类索引
  selectedItemIndex: number      // 当前分类中选中的项目索引
}

// Mention 匹配正则：匹配 @后跟文件路径形式的文本
// 例如：@file.txt, @./path/to/file.js, @component.tsx
const MENTION_REGEX = /@(\.?[^\s`,.]+(?:\.[^\s`,.]+)*)/g

/**
 * MentionInput 组件
 * 支持 @ 符号触发的提及功能
 * 
 * 插入格式：
 * - 文件/文件夹：[label](path)
 * - 命令：@command
 */
export const MentionInput: React.FC<MentionInputProps> = ({
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
  
  // 获取当前选中分类的项目
  const currentCategoryItems = useMemo(() => {
    const category = filteredCategories[mentionState.selectedCategoryIndex]
    return category ? category.items : []
  }, [filteredCategories, mentionState.selectedCategoryIndex])

  // 获取默认的 mention 图标
  const getDefaultIcon = (type: MentionItem['type']) => {
    switch (type) {
      case 'file':
        return <File className="h-3.5 w-3.5" />
      case 'folder':
        return <Folder className="h-3.5 w-3.5" />
      case 'command':
        return <Command className="h-3.5 w-3.5" />
      case 'branch':
        return <GitBranch className="h-3.5 w-3.5" />
      case 'worker':
        return <Bot className="h-3.5 w-3.5" />
      case 'skill':
        return <Sparkles className="h-3.5 w-3.5" />
      default:
        return <Hash className="h-3.5 w-3.5" />
    }
  }

  // 搜索 mention 项并按分类组织
  const searchMentions = useCallback(async (query: string) => {
    setIsSearching(true)
    try {
      let items: MentionItem[] = []
      
      if (onMentionSearch) {
        const result = await onMentionSearch(query)
        items = result
      } else {
        // 默认使用提供的 mentionItems 进行过滤
        // 如果 query 为空，显示所有项目
        if (!query.trim()) {
          items = mentionItems
        } else {
          items = mentionItems.filter(item =>
            item.label.toLowerCase().includes(query.toLowerCase()) ||
            item.value.toLowerCase().includes(query.toLowerCase())
          )
        }
      }
      
      // 按分类组织项目
      const categoryMap = new Map<string, MentionItem[]>()
      items.forEach(item => {
        const category = item.category || '其他'
        if (!categoryMap.has(category)) {
          categoryMap.set(category, [])
        }
        categoryMap.get(category)!.push(item)
      })
      
      // 转换为分类数组
      const categories: MentionCategory[] = Array.from(categoryMap.entries()).map(([name, items]) => ({
        name,
        items
      }))
      
      setFilteredCategories(categories)
    } finally {
      setIsSearching(false)
    }
  }, [mentionItems, onMentionSearch])

  // 检查是否应该触发 mention
  const checkMentionTrigger = useCallback((text: string, cursorPosition: number) => {
    // 查找最近的 @ 符号
    let triggerIndex = -1
    let query = ''
    
    for (let i = cursorPosition - 1; i >= 0; i--) {
      const char = text[i]
      
      // 如果遇到空格或换行，停止搜索
      if (char === ' ' || char === '\n') {
        break
      }
      
      // 找到 @ 符号
      if (char === '@') {
        triggerIndex = i
        query = text.substring(i + 1, cursorPosition)
        break
      }
    }
    
    return { triggerIndex, query }
  }, [])

  // 计算 mention 菜单位置（使用固定定位）
  const calculateMenuPosition = useCallback((textarea: HTMLTextAreaElement) => {
    const rect = textarea.getBoundingClientRect()
    // 菜单显示在输入框上方，预估菜单高度为 256px (max-h-64)
    const estimatedMenuHeight = 256
    return {
      bottom: window.innerHeight - rect.top + 4, // 距离底部的距离，4px 间距
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
      // 触发 mention
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
      // 关闭 mention
      setMentionState(prev => ({ ...prev, isOpen: false }))
    }
  }, [onChange, checkMentionTrigger, calculateMenuPosition, searchMentions])

  // 插入 mention
  const insertMention = useCallback((item: MentionItem) => {
    if (!textareaRef.current) return
    
    const { triggerIndex } = mentionState
    const beforeMention = value.substring(0, triggerIndex)
    const afterCursor = value.substring(textareaRef.current.selectionStart)
    
    // 根据类型决定插入格式
    let mentionText: string
    if (item.type === 'file' || item.type === 'folder') {
      // 文件和文件夹：插入 markdown 链接格式 [label](path)
      mentionText = `[${item.label}](${item.value})`
    } else {
      // 命令和其他：插入 @label 格式
      mentionText = `@${item.label}`
    }
    
    const newValue = beforeMention + mentionText + ' ' + afterCursor
    const newCursorPos = beforeMention.length + mentionText.length + 1
    
    onChange(newValue)
    
    // 关闭菜单
    setMentionState(prev => ({ ...prev, isOpen: false }))
    
    // 恢复焦点并设置光标位置
    setTimeout(() => {
      if (textareaRef.current) {
        textareaRef.current.focus()
        textareaRef.current.setSelectionRange(newCursorPos, newCursorPos)
      }
    }, 0)
  }, [mentionState, value, onChange])

  // 处理键盘事件
  const handleKeyDown = useCallback((e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (!mentionState.isOpen) {
      // 处理提交
      if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) {
        e.preventDefault()
        onSubmit?.()
      }
      return
    }
    
    // Mention 菜单打开时的键盘导航
    switch (e.key) {
      case 'ArrowLeft':
        e.preventDefault()
        // 切换到上一个分类
        setMentionState(prev => ({
          ...prev,
          selectedCategoryIndex: Math.max(prev.selectedCategoryIndex - 1, 0),
          selectedItemIndex: 0  // 重置项目选择
        }))
        break
      case 'ArrowRight':
        e.preventDefault()
        // 切换到下一个分类
        setMentionState(prev => ({
          ...prev,
          selectedCategoryIndex: Math.min(prev.selectedCategoryIndex + 1, filteredCategories.length - 1),
          selectedItemIndex: 0  // 重置项目选择
        }))
        break
      case 'ArrowDown':
        e.preventDefault()
        // 在当前分类中向下选择
        setMentionState(prev => ({
          ...prev,
          selectedItemIndex: Math.min(prev.selectedItemIndex + 1, currentCategoryItems.length - 1)
        }))
        break
      case 'ArrowUp':
        e.preventDefault()
        // 在当前分类中向上选择
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

  // 滚动选中项到可见区域
  useEffect(() => {
    if (mentionState.isOpen && mentionMenuRef.current) {
      const selectedElement = mentionMenuRef.current.querySelector(`[data-item-index="${mentionState.selectedItemIndex}"]`) as HTMLElement
      if (selectedElement) {
        selectedElement.scrollIntoView({ block: 'nearest', behavior: 'smooth' })
      }
    }
  }, [mentionState.selectedItemIndex, mentionState.isOpen])

  // 点击外部关闭菜单
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
      <textarea
        ref={textareaRef}
        value={value}
        onChange={handleChange}
        onKeyDown={handleKeyDown}
        placeholder={placeholder}
        disabled={disabled}
        className={cn(
          "w-full min-h-[60px] text-sm leading-5 resize-none bg-transparent",
          "overflow-y-auto whitespace-pre-wrap break-words outline-none rounded-md border border-input",
          "px-3 py-2 disabled:opacity-50",
          className
        )}
        style={{ maxHeight }}
      />
      
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
