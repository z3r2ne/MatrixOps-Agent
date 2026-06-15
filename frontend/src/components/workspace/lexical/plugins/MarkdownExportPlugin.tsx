import { useLexicalComposerContext } from '@lexical/react/LexicalComposerContext'
import { $convertFromMarkdownString, $convertToMarkdownString, TRANSFORMERS } from '@lexical/markdown'
import { useEffect, useRef } from 'react'
import { $getRoot } from 'lexical'

interface MarkdownExportPluginProps {
  value: string
  onChange: (markdown: string) => void
}

export function MarkdownExportPlugin({ value, onChange }: MarkdownExportPluginProps) {
  const [editor] = useLexicalComposerContext()
  const isFirstRender = useRef(true)
  const prevValueRef = useRef(value)
  const isInternalUpdate = useRef(false)

  // 初始化：将 value 转换为编辑器状态
  useEffect(() => {
    if (isFirstRender.current && value && value !== prevValueRef.current) {
      isFirstRender.current = false
      isInternalUpdate.current = true
      editor.update(() => {
        try {
          $convertFromMarkdownString(value, TRANSFORMERS)
        } catch (error) {
          console.error('Failed to parse initial markdown:', error)
        }
      })
      prevValueRef.current = value
      setTimeout(() => {
        isInternalUpdate.current = false
      }, 100)
    }
  }, [value, editor])

  // 监听外部 value 变化（例如从 ReviewDialog 插入命令）
  useEffect(() => {
    if (!isFirstRender.current && value !== prevValueRef.current && !isInternalUpdate.current) {
      isInternalUpdate.current = true
      editor.update(() => {
        try {
          // 清空现有内容
          const root = $getRoot()
          root.clear()
          // 插入新内容
          $convertFromMarkdownString(value, TRANSFORMERS)
        } catch (error) {
          console.error('Failed to update markdown:', error)
        }
      })
      prevValueRef.current = value
      setTimeout(() => {
        isInternalUpdate.current = false
      }, 100)
    }
  }, [value, editor])

  // 监听变化：导出 Markdown
  useEffect(() => {
    return editor.registerUpdateListener(({ editorState, dirtyElements, dirtyLeaves }) => {
      // 跳过内部更新
      if (isInternalUpdate.current) {
        return
      }
      
      // 只在有实际变化时才导出
      if (dirtyElements.size === 0 && dirtyLeaves.size === 0) {
        return
      }
      
      editorState.read(() => {
        try {
          const markdown = $convertToMarkdownString(TRANSFORMERS)
          // 避免无意义的更新
          if (markdown !== prevValueRef.current) {
            prevValueRef.current = markdown
            onChange(markdown)
          }
        } catch (error) {
          console.error('Failed to export markdown:', error)
        }
      })
    })
  }, [editor, onChange])

  return null
}
