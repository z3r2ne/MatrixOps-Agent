import { useLexicalComposerContext } from '@lexical/react/LexicalComposerContext'
import { useEffect } from 'react'
import { KEY_ENTER_COMMAND, COMMAND_PRIORITY_LOW } from 'lexical'

interface KeyboardPluginProps {
  onSubmit?: () => void
}

export function KeyboardPlugin({ onSubmit }: KeyboardPluginProps) {
  const [editor] = useLexicalComposerContext()

  useEffect(() => {
    return editor.registerCommand(
      KEY_ENTER_COMMAND,
      (event: KeyboardEvent | null) => {
        // Enter 不带 Shift：提交
        if (event && !event.shiftKey && onSubmit) {
          event.preventDefault()
          onSubmit()
          return true
        }
        // Shift + Enter：换行（默认行为）
        return false
      },
      COMMAND_PRIORITY_LOW // 降低优先级，让 MentionPlugin 先处理
    )
  }, [editor, onSubmit])

  return null
}
