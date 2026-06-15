import {
  DecoratorNode,
  NodeKey,
  SerializedLexicalNode,
  Spread,
  LexicalNode,
  EditorConfig,
  DOMExportOutput,
  DOMConversionMap,
  DOMConversionOutput,
} from 'lexical'
import { File, Command, GitBranch, Hash, Bot, Sparkles } from 'lucide-react'
import { cn } from '@/lib/utils'

export type SerializedMentionNode = Spread<
  {
    mentionLabel: string
    mentionValue: string
    mentionType: 'file' | 'folder' | 'command' | 'branch' | 'worker' | 'skill' | 'custom'
    type: 'mention'
    version: 1
  },
  SerializedLexicalNode
>

export class MentionNode extends DecoratorNode<JSX.Element> {
  __mentionLabel: string
  __mentionValue: string
  __mentionType: 'file' | 'folder' | 'command' | 'branch' | 'worker' | 'skill' | 'custom'

  static getType(): string {
    return 'mention'
  }

  static clone(node: MentionNode): MentionNode {
    return new MentionNode(
      node.__mentionLabel,
      node.__mentionValue,
      node.__mentionType,
      node.__key
    )
  }

  static importJSON(serializedNode: SerializedMentionNode): MentionNode {
    const node = $createMentionNode(
      serializedNode.mentionLabel,
      serializedNode.mentionValue,
      serializedNode.mentionType
    )
    return node
  }

  constructor(
    mentionLabel: string,
    mentionValue: string,
    mentionType: 'file' | 'folder' | 'command' | 'branch' | 'worker' | 'skill' | 'custom' = 'custom',
    key?: NodeKey
  ) {
    super(key)
    this.__mentionLabel = mentionLabel
    this.__mentionValue = mentionValue
    this.__mentionType = mentionType
  }

  createDOM(config: EditorConfig): HTMLElement {
    const dom = document.createElement('span')
    dom.className = 'mention-node inline'
    return dom
  }

  updateDOM(): false {
    return false
  }

  exportDOM(): DOMExportOutput {
    const element = document.createElement('span')
    element.setAttribute('data-lexical-mention', 'true')
    element.textContent = this.getTextContent()
    return { element }
  }

  exportJSON(): SerializedMentionNode {
    return {
      mentionLabel: this.__mentionLabel,
      mentionValue: this.__mentionValue,
      mentionType: this.__mentionType,
      type: 'mention',
      version: 1,
    }
  }

  isInline(): boolean {
    return true
  }

  getTextContent(): string {
    // 根据类型返回不同的文本格式
    if (this.__mentionType === 'file' || this.__mentionType === 'folder') {
      return `[${this.__mentionLabel}](${this.__mentionValue})`
    }
    if (this.__mentionValue.includes('://')) {
      return `[${this.__mentionLabel}](${this.__mentionValue})`
    }
    return `@${this.__mentionLabel}`
  }

  decorate(): JSX.Element {
    const isFile = this.__mentionType === 'file' || this.__mentionType === 'folder'
    const isCommand = this.__mentionType === 'command'
    const isBranch = this.__mentionType === 'branch'
    const isWorker = this.__mentionType === 'worker'
    const isSkill = this.__mentionType === 'skill'
    const isCustom = this.__mentionType === 'custom'
    
    return (
      <span
        className={cn(
          'inline-flex items-center gap-0.5 px-1 py-0 mx-0.5 rounded-sm text-sm border',
          isCommand && 'bg-blue-50 text-blue-700 border-blue-200',
          isFile && 'bg-emerald-50 text-emerald-700 border-emerald-200',
          isBranch && 'bg-amber-50 text-amber-700 border-amber-200',
          isWorker && 'bg-violet-50 text-violet-700 border-violet-200',
          isSkill && 'bg-fuchsia-50 text-fuchsia-700 border-fuchsia-200',
          isCustom && 'bg-muted text-muted-foreground border-border'
        )}
        contentEditable={false}
        suppressContentEditableWarning
      >
        {isFile ? (
          <File className="h-2.5 w-2.5 inline-block" />
        ) : isBranch ? (
          <GitBranch className="h-2.5 w-2.5 inline-block" />
        ) : isCommand ? (
          <Command className="h-2.5 w-2.5 inline-block" />
        ) : isWorker ? (
          <Bot className="h-2.5 w-2.5 inline-block" />
        ) : isSkill ? (
          <Sparkles className="h-2.5 w-2.5 inline-block" />
        ) : (
          <Hash className="h-2.5 w-2.5 inline-block" />
        )}
        <span>{this.__mentionLabel}</span>
      </span>
    )
  }
}

export function $createMentionNode(
  mentionLabel: string,
  mentionValue: string,
  mentionType: 'file' | 'folder' | 'command' | 'branch' | 'worker' | 'skill' | 'custom' = 'custom'
): MentionNode {
  return new MentionNode(mentionLabel, mentionValue, mentionType)
}

export function $isMentionNode(
  node: LexicalNode | null | undefined
): node is MentionNode {
  return node instanceof MentionNode
}
