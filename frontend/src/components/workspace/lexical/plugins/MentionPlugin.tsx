import { useLexicalComposerContext } from '@lexical/react/LexicalComposerContext'
import { useCallback, useEffect, useState, useMemo, useRef } from 'react'
import {
  $getSelection,
  $isRangeSelection,
  TextNode,
  COMMAND_PRIORITY_LOW,
  COMMAND_PRIORITY_CRITICAL,
  KEY_ARROW_DOWN_COMMAND,
  KEY_ARROW_UP_COMMAND,
  KEY_ARROW_LEFT_COMMAND,
  KEY_ARROW_RIGHT_COMMAND,
  KEY_ESCAPE_COMMAND,
  KEY_ENTER_COMMAND,
  KEY_TAB_COMMAND,
  $createTextNode,
} from 'lexical'
import { $createMentionNode } from '../nodes/MentionNode'
import { MentionItem } from '../../MarkdownMentionInput'
import { cn } from '@/lib/utils'
import { File, Folder, Command, Hash, GitBranch, Bot, Sparkles } from 'lucide-react'

interface MentionCategory {
  name: string
  items: MentionItem[]
}

interface MentionPluginProps {
  mentionItems?: MentionItem[]
  onMentionSearch?: (query: string) => Promise<MentionItem[]> | MentionItem[]
  onFileSearch?: (query: string) => Promise<MentionItem[]> | MentionItem[]
  onSpecialCommand?: (command: string) => void
}

export function MentionPlugin({
  mentionItems = [],
  onMentionSearch,
  onFileSearch,
  onSpecialCommand,
}: MentionPluginProps) {
  const [editor] = useLexicalComposerContext()
  const [queryString, setQueryString] = useState<string | null>(null)
  const [searchValue, setSearchValue] = useState('')
  const [categories, setCategories] = useState<MentionCategory[]>([])
  const [selectedCategoryIndex, setSelectedCategoryIndex] = useState(0)
  const [selectedItemIndex, setSelectedItemIndex] = useState(0)
  const [menuPosition, setMenuPosition] = useState<{ top: number; left: number } | null>(null)
  const [fileResults, setFileResults] = useState<MentionItem[]>([])
  const listRef = useRef<HTMLDivElement>(null)
  const searchInputRef = useRef<HTMLInputElement>(null)

  const currentItems = useMemo(() => {
    const category = categories[selectedCategoryIndex]
    if (!category) return []
    if (category.name === 'file') {
      return searchValue.trim() ? fileResults : category.items
    }
    return category.items
  }, [categories, selectedCategoryIndex, fileResults, searchValue])

  const categoryCount = useCallback((category: MentionCategory) => {
    if (category.name === 'file' && searchValue.trim()) {
      return fileResults.length
    }
    return category.items.length
  }, [fileResults.length, searchValue])

  useEffect(() => {
    if (!listRef.current) return
    const el = listRef.current.querySelector<HTMLElement>(`[data-mention-index="${selectedItemIndex}"]`)
    el?.scrollIntoView({ block: 'nearest' })
  }, [selectedItemIndex, currentItems.length])

  // 获取图标
  const getIcon = (type: MentionItem['type']) => {
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

  // 搜索 mentions
  const searchMentions = useCallback(async (query: string) => {
    let items: MentionItem[] = []

    if (onMentionSearch) {
      items = await onMentionSearch(query)
    } else {
      if (!query.trim()) {
        items = mentionItems
      } else {
        items = mentionItems.filter(
          item =>
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

    const cats: MentionCategory[] = Array.from(categoryMap.entries()).map(([name, items]) => ({
      name,
      items,
    }))

    setCategories(cats)
    setSelectedCategoryIndex(0)
    setSelectedItemIndex(0)
  }, [mentionItems, onMentionSearch])

  useEffect(() => {
    if (!onFileSearch) return
    if (!searchValue.trim()) {
      setFileResults([])
      return
    }
    let cancelled = false
    Promise.resolve(onFileSearch(searchValue.trim())).then((items) => {
      if (cancelled) return
      setFileResults(items)
    })
    return () => {
      cancelled = true
    }
  }, [searchValue, onFileSearch])

  useEffect(() => {
    if (queryString === null) {
      setSearchValue('')
      setFileResults([])
    }
  }, [queryString])

  useEffect(() => {
    if (queryString === null) return
    searchMentions(searchValue)
  }, [queryString, searchMentions, searchValue])

  // 插入 mention
  const insertMention = useCallback((item: MentionItem) => {
    // 检查是否是特殊命令
    if (item.isSpecialCommand && onSpecialCommand) {
      // 对于特殊命令，触发回调而不是直接插入
      onSpecialCommand(item.value)
      
      // 删除 @ 和查询文本
      editor.update(() => {
        const selection = $getSelection()
        if (!$isRangeSelection(selection)) return

        const anchor = selection.anchor
        const anchorNode = anchor.getNode()
        
        if (!(anchorNode instanceof TextNode)) return

        const text = anchorNode.getTextContent()
        const offset = anchor.offset

        // 找到 @ 的位置
        let atIndex = -1
        for (let i = offset - 1; i >= 0; i--) {
          if (text[i] === ' ' || text[i] === '\n') break
          if (text[i] === '@') {
            atIndex = i
            break
          }
        }

        if (atIndex !== -1) {
          // 删除从 @ 到当前位置的文本
          const textToDelete = offset - atIndex
          for (let i = 0; i < textToDelete; i++) {
            selection.deleteCharacter(true)
          }
        }
      })
      
      // 关闭菜单
      setQueryString(null)
      setMenuPosition(null)
      return
    }

    // 普通 mention 的插入逻辑
    editor.update(() => {
      const selection = $getSelection()
      if (!$isRangeSelection(selection)) return

      const anchor = selection.anchor
      const anchorNode = anchor.getNode()
      
      if (!(anchorNode instanceof TextNode)) return

      const text = anchorNode.getTextContent()
      const offset = anchor.offset

      // 找到 @ 的位置
      let atIndex = -1
      for (let i = offset - 1; i >= 0; i--) {
        if (text[i] === ' ' || text[i] === '\n') break
        if (text[i] === '@') {
          atIndex = i
          break
        }
      }

      if (atIndex === -1) return

      // 删除从 @ 到当前位置的文本
      const textToDelete = offset - atIndex
      for (let i = 0; i < textToDelete; i++) {
        selection.deleteCharacter(true) // true = backward
      }

      // 创建并插入 mention 节点
      const mention = $createMentionNode(
        item.label,
        item.value,
        item.type
      )
      
      selection.insertNodes([mention])
      
      // 关闭菜单
      setQueryString(null)
      setMenuPosition(null)
    })
  }, [editor, onSpecialCommand])

  // 监听文本变化检测 @
  useEffect(() => {
    const updateListener = editor.registerUpdateListener(({ editorState }) => {
      editorState.read(() => {
        const selection = $getSelection()
        if (!$isRangeSelection(selection) || !selection.isCollapsed()) {
          setQueryString(null)
          setMenuPosition(null)
          return
        }

        const anchor = selection.anchor
        const anchorNode = anchor.getNode()
        
        if (!(anchorNode instanceof TextNode)) {
          setQueryString(null)
          setMenuPosition(null)
          return
        }

        const text = anchorNode.getTextContent()
        const offset = anchor.offset

        // 查找最近的 @
        let atIndex = -1
        for (let i = offset - 1; i >= 0; i--) {
          if (text[i] === ' ' || text[i] === '\n') break
          if (text[i] === '@') {
            atIndex = i
            break
          }
        }

        if (atIndex !== -1) {
          const query = text.substring(atIndex + 1, offset)
          setQueryString(query)
          setSearchValue(query)
          searchMentions(query)

          // 计算菜单位置
          const domSelection = window.getSelection()
          if (domSelection && domSelection.rangeCount > 0) {
            const range = domSelection.getRangeAt(0)
            const rect = range.getBoundingClientRect()
            const menuHeight = 280
            const top = Math.max(8, rect.top - menuHeight - 12)
            setMenuPosition({
              top,
              left: rect.left,
            })
          }
        } else {
          setQueryString(null)
          setMenuPosition(null)
        }
      })
    })

    return () => {
      updateListener()
    }
  }, [editor, searchMentions])

  // 键盘导航
  useEffect(() => {
    if (queryString === null) return

    const removeArrowLeftCommand = editor.registerCommand(
      KEY_ARROW_LEFT_COMMAND,
      (event) => {
        if (document.activeElement === searchInputRef.current) return true
        event?.preventDefault()
        event?.stopPropagation()
        setSelectedCategoryIndex(prev => Math.max(0, prev - 1))
        setSelectedItemIndex(0)
        return true
      },
      COMMAND_PRIORITY_CRITICAL
    )

    const removeArrowRightCommand = editor.registerCommand(
      KEY_ARROW_RIGHT_COMMAND,
      (event) => {
        if (document.activeElement === searchInputRef.current) return true
        event?.preventDefault()
        event?.stopPropagation()
        setSelectedCategoryIndex(prev => Math.min(categories.length - 1, prev + 1))
        setSelectedItemIndex(0)
        return true
      },
      COMMAND_PRIORITY_CRITICAL
    )

    const removeArrowDownCommand = editor.registerCommand(
      KEY_ARROW_DOWN_COMMAND,
      (event) => {
        if (document.activeElement === searchInputRef.current) return true
        event?.preventDefault()
        event?.stopPropagation()
        setSelectedItemIndex(prev => Math.min(currentItems.length - 1, prev + 1))
        return true
      },
      COMMAND_PRIORITY_CRITICAL
    )

    const removeArrowUpCommand = editor.registerCommand(
      KEY_ARROW_UP_COMMAND,
      (event) => {
        if (document.activeElement === searchInputRef.current) return true
        event?.preventDefault()
        event?.stopPropagation()
        setSelectedItemIndex(prev => Math.max(0, prev - 1))
        return true
      },
      COMMAND_PRIORITY_CRITICAL
    )

    const removeEscapeCommand = editor.registerCommand(
      KEY_ESCAPE_COMMAND,
      () => {
        // 关闭菜单时，在光标位置插入一个空格来"终止" mention 触发
        editor.update(() => {
          const selection = $getSelection()
          if ($isRangeSelection(selection)) {
            selection.insertText(' ')
          }
        })
        
        setQueryString(null)
        setMenuPosition(null)
        editor.focus()
        return true
      },
      COMMAND_PRIORITY_CRITICAL
    )

    const removeEnterCommand = editor.registerCommand(
      KEY_ENTER_COMMAND,
      (event) => {
        if (currentItems[selectedItemIndex]) {
          event?.preventDefault()
          insertMention(currentItems[selectedItemIndex])
          return true
        }
        return false
      },
      COMMAND_PRIORITY_CRITICAL
    )

    const removeTabCommand = editor.registerCommand(
      KEY_TAB_COMMAND,
      (event) => {
        if (currentItems[selectedItemIndex]) {
          event?.preventDefault()
          insertMention(currentItems[selectedItemIndex])
          return true
        }
        return false
      },
      COMMAND_PRIORITY_CRITICAL
    )

    return () => {
      removeArrowLeftCommand()
      removeArrowRightCommand()
      removeArrowDownCommand()
      removeArrowUpCommand()
      removeEscapeCommand()
      removeEnterCommand()
      removeTabCommand()
    }
  }, [queryString, categories, currentItems, selectedCategoryIndex, selectedItemIndex, editor, insertMention])

  if (queryString === null || !menuPosition) return null

  return (
    <div
      className="fixed z-[100] w-[420px] max-w-[90vw] h-72 bg-popover border border-border rounded-md shadow-lg overflow-hidden"
      style={{
        top: `${menuPosition.top}px`,
        left: `${menuPosition.left}px`,
      }}
    >
      {categories.length === 0 ? (
        <div className="px-3 py-2 text-xs text-muted-foreground">
          {queryString ? '未找到匹配项' : '开始输入以搜索...'}
        </div>
      ) : (
        <>
          {/* 分类标签栏 */}
          <div className="flex border-b border-border overflow-x-auto">
            {categories.map((category, catIndex) => (
              <button
                key={category.name}
                type="button"
                className={cn(
                  'px-4 py-2 text-xs font-medium whitespace-nowrap transition-colors',
                  'border-b-2 -mb-[1px]',
                  catIndex === selectedCategoryIndex
                    ? 'border-primary text-primary bg-accent/50'
                    : 'border-transparent text-muted-foreground hover:text-foreground hover:bg-accent/30'
                )}
                onClick={() => {
                  setSelectedCategoryIndex(catIndex)
                  setSelectedItemIndex(0)
                }}
              >
                {category.name}
                <span className="ml-1.5 text-[10px] opacity-70">
                  ({categoryCount(category)})
                </span>
              </button>
            ))}
          </div>

          {/* 搜索框 */}
          <div className="border-b border-border px-3 py-2">
            <input
              ref={searchInputRef}
              value={searchValue}
              onChange={(e) => {
                setSearchValue(e.target.value)
                setSelectedItemIndex(0)
              }}
              onKeyDown={(event) => {
                if (event.key === 'ArrowLeft') {
                  event.preventDefault()
                  setSelectedCategoryIndex(prev => Math.max(0, prev - 1))
                  setSelectedItemIndex(0)
                } else if (event.key === 'ArrowRight') {
                  event.preventDefault()
                  setSelectedCategoryIndex(prev => Math.min(categories.length - 1, prev + 1))
                  setSelectedItemIndex(0)
                } else if (event.key === 'ArrowDown') {
                  event.preventDefault()
                  setSelectedItemIndex(prev => Math.min(currentItems.length - 1, prev + 1))
                } else if (event.key === 'ArrowUp') {
                  event.preventDefault()
                  setSelectedItemIndex(prev => Math.max(0, prev - 1))
                } else if (event.key === 'Enter') {
                  event.preventDefault()
                  if (currentItems[selectedItemIndex]) {
                    insertMention(currentItems[selectedItemIndex])
                  }
                } else if (event.key === 'Escape') {
                  event.preventDefault()
                  setQueryString(null)
                  setMenuPosition(null)
                  editor.focus()
                }
              }}
              placeholder="搜索..."
              className="w-full rounded border border-input bg-background px-2 py-1 text-xs"
            />
          </div>

          {/* 当前分类的项目列表 */}
          <div className="max-h-64 overflow-y-auto" ref={listRef}>
            {currentItems.map((item, itemIndex) => (
              <button
                key={item.id}
                type="button"
                className={cn(
                  'w-full flex items-start gap-2 px-3 py-2 text-left text-xs transition-colors',
                  'hover:bg-accent focus:bg-accent outline-none',
                  itemIndex === selectedItemIndex && 'bg-accent'
                )}
                data-mention-index={itemIndex}
                onClick={() => insertMention(item)}
                onMouseEnter={() => setSelectedItemIndex(itemIndex)}
              >
                <div className="flex h-5 w-5 shrink-0 items-center justify-center text-muted-foreground mt-0.5">
                  {item.icon || getIcon(item.type)}
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
            ))}
          </div>
        </>
      )}
    </div>
  )
}
