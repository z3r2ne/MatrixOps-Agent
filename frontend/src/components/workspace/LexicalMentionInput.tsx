import React, { useCallback } from 'react'
import { LexicalComposer } from '@lexical/react/LexicalComposer'
import { useLexicalComposerContext } from '@lexical/react/LexicalComposerContext'
import { RichTextPlugin } from '@lexical/react/LexicalRichTextPlugin'
import { ContentEditable } from '@lexical/react/LexicalContentEditable'
import { HistoryPlugin } from '@lexical/react/LexicalHistoryPlugin'
import { MarkdownShortcutPlugin } from '@lexical/react/LexicalMarkdownShortcutPlugin'
import { TRANSFORMERS } from '@lexical/markdown'
import { LexicalErrorBoundary } from '@lexical/react/LexicalErrorBoundary'
import { HeadingNode, QuoteNode } from '@lexical/rich-text'
import { ListNode, ListItemNode } from '@lexical/list'
import { CodeNode } from '@lexical/code'
import { LinkNode } from '@lexical/link'
import { useEffect } from 'react'
import { cn } from '@/lib/utils'
import { MentionNode, $createMentionNode } from './lexical/nodes/MentionNode'
import { MentionPlugin } from './lexical/plugins/MentionPlugin'
import { MarkdownExportPlugin } from './lexical/plugins/MarkdownExportPlugin'
import { KeyboardPlugin } from './lexical/plugins/KeyboardPlugin'
import { MentionItem } from './MarkdownMentionInput'
import { $getSelection, $isRangeSelection, $getRoot, $createParagraphNode } from 'lexical'

export interface LexicalMentionInputProps {
  value: string
  onChange: (value: string) => void
  onSubmit?: () => void
  placeholder?: string
  disabled?: boolean
  className?: string
  maxHeight?: string
  fill?: boolean
  mentionItems?: MentionItem[]
  onMentionSearch?: (query: string) => Promise<MentionItem[]> | MentionItem[]
  onFileSearch?: (query: string) => Promise<MentionItem[]> | MentionItem[]
  onSpecialCommand?: (command: string) => void  // 处理特殊命令（如 @review）
  insertMentionRequest?: MentionItem | null
  onInsertMentionComplete?: () => void
}

const theme = {
  paragraph: 'mb-1',
  quote: 'border-l-4 border-muted-foreground/20 pl-4 italic',
  heading: {
    h1: 'text-2xl font-bold mb-2',
    h2: 'text-xl font-bold mb-2',
    h3: 'text-lg font-bold mb-1',
  },
  list: {
    nested: {
      listitem: 'list-none',
    },
    ol: 'list-decimal ml-4',
    ul: 'list-disc ml-4',
    listitem: 'ml-2',
  },
  code: 'bg-muted px-1 py-0.5 rounded font-mono text-sm',
  codeHighlight: {},
  link: 'text-primary underline hover:text-primary/80',
  text: {
    bold: 'font-bold',
    italic: 'italic',
    underline: 'underline',
    strikethrough: 'line-through',
    code: 'bg-muted px-1 py-0.5 rounded font-mono text-sm',
  },
}

export const LexicalMentionInput: React.FC<LexicalMentionInputProps> = ({
  value,
  onChange,
  onSubmit,
  placeholder = '输入 @ 来提及...',
  disabled = false,
  className,
  maxHeight = '120px',
  fill = false,
  mentionItems = [],
  onMentionSearch,
  onFileSearch,
  onSpecialCommand,
  insertMentionRequest,
  onInsertMentionComplete,
}) => {
  const initialConfig = {
    namespace: 'MentionEditor',
    theme,
    onError: (error: Error) => {
      console.error('Lexical error:', error)
    },
    editable: !disabled,
    nodes: [
      HeadingNode,
      QuoteNode,
      ListNode,
      ListItemNode,
      CodeNode,
      LinkNode,
      MentionNode,
    ],
    editorState: undefined, // 将通过 MarkdownExportPlugin 设置初始值
  }

  return (
    <div className={cn('relative flex-1', className)}>
      <LexicalComposer initialConfig={initialConfig}>
        <div className={cn('relative', fill && 'h-full')}>
          <RichTextPlugin
            contentEditable={
              <ContentEditable
                className={cn(
                  'px-3 py-2 text-sm',
                  'outline-none rounded-md border border-input',
                  'overflow-y-auto whitespace-pre-wrap break-words',
                  'bg-background',
                  fill ? 'h-full min-h-0' : 'min-h-[60px]',
                  disabled && 'opacity-50 cursor-not-allowed'
                )}
                style={{ maxHeight }}
              />
            }
            placeholder={
              <div className="absolute top-2 left-3 text-sm text-muted-foreground pointer-events-none">
                {placeholder}
              </div>
            }
            ErrorBoundary={LexicalErrorBoundary}
          />
          <HistoryPlugin />
          <MarkdownShortcutPlugin transformers={TRANSFORMERS} />
          <MentionPlugin
            mentionItems={mentionItems}
            onMentionSearch={onMentionSearch}
            onFileSearch={onFileSearch}
            onSpecialCommand={onSpecialCommand}
          />
          <EditorStatePlugin disabled={disabled} value={value} />
          <InsertMentionPlugin
            request={insertMentionRequest}
            onComplete={onInsertMentionComplete}
          />
          <MarkdownExportPlugin
            value={value}
            onChange={onChange}
          />
          <KeyboardPlugin onSubmit={onSubmit} />
        </div>
      </LexicalComposer>
    </div>
  )
}

const InsertMentionPlugin: React.FC<{ request?: MentionItem | null; onComplete?: () => void }> = ({
  request,
  onComplete,
}) => {
  const [editor] = useLexicalComposerContext()

  useEffect(() => {
    if (!request) return
    editor.update(() => {
      const selection = $getSelection()
      const mention = $createMentionNode(request.label, request.value, request.type)
      if ($isRangeSelection(selection)) {
        selection.insertNodes([mention])
      } else {
        const root = $getRoot()
        root.append(mention)
      }
    })
    onComplete?.()
  }, [editor, request, onComplete])

  return null
}

const EditorStatePlugin: React.FC<{ disabled: boolean; value: string }> = ({ disabled, value }) => {
  const [editor] = useLexicalComposerContext()

  useEffect(() => {
    editor.setEditable(!disabled)
  }, [editor, disabled])

  useEffect(() => {
    if (value !== '') return
    editor.update(() => {
      const root = $getRoot()
      root.clear()
      root.append($createParagraphNode())
    })
  }, [editor, value])

  return null
}
