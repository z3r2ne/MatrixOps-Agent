import React, { ReactNode, useMemo, useState, useCallback } from "react"
import { unified } from "unified"
import remarkParse from "remark-parse"
import remarkGfm from "remark-gfm"
import { Check, Copy } from "lucide-react"
import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"

type MarkdownNode = {
  type: string
  value?: string
  depth?: number
  ordered?: boolean
  start?: number | null
  checked?: boolean | null
  url?: string
  alt?: string
  lang?: string | null
  align?: Array<"left" | "right" | "center" | null> | null
  children?: MarkdownNode[]
}

type MarkdownRoot = MarkdownNode & {
  children: MarkdownNode[]
}

interface MarkdownRendererProps {
  content: string
  className?: string
}

const markdownProcessor = unified().use(remarkParse).use(remarkGfm)

function MarkdownCodeBlock({ lang, code }: { lang?: string | null; code: string }) {
  const [copied, setCopied] = useState(false)
  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(code)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch (e) {
      console.error("copy code block:", e)
    }
  }, [code])

  return (
    <div className="my-1.5 overflow-hidden rounded-md border border-border/80 bg-background">
      <div className="flex items-center justify-end gap-1 border-b border-border/70 px-2 py-0.5">
        {lang ? (
          <span className="mr-auto truncate text-[11px] font-medium uppercase tracking-[0.14em] text-muted-foreground">
            {lang}
          </span>
        ) : (
          <span className="mr-auto" />
        )}
        <TooltipProvider>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="h-7 w-7 shrink-0 text-muted-foreground hover:text-foreground"
                onClick={handleCopy}
                aria-label="复制代码"
              >
                {copied ? <Check className="h-3.5 w-3.5 text-emerald-600" /> : <Copy className="h-3.5 w-3.5" />}
              </Button>
            </TooltipTrigger>
            <TooltipContent side="left">{copied ? "已复制" : "复制"}</TooltipContent>
          </Tooltip>
        </TooltipProvider>
      </div>
      <pre className="overflow-x-auto px-2.5 py-1.5 font-mono text-[13px] leading-snug text-foreground">
        <code>{code}</code>
      </pre>
    </div>
  )
}

function isSafeLink(url?: string) {
  if (!url) return false
  const trimmed = url.trim()
  if (!trimmed) return false
  if (trimmed.startsWith("#")) return true
  return /^(https?:|mailto:|tel:)/i.test(trimmed)
}

function isExternalLink(url?: string) {
  return !!url && /^https?:/i.test(url.trim())
}

function renderChildren(children?: MarkdownNode[], keyPrefix: string = "node"): ReactNode[] {
  if (!children?.length) return []

  return children.map((child, index) => renderNode(child, `${keyPrefix}-${index}`))
}

function renderTableCell(
  cell: MarkdownNode,
  key: string,
  options: { isHeader: boolean; align?: "left" | "right" | "center" | null },
) {
  const Tag = options.isHeader ? "th" : "td"
  const alignClass = options.align === "center"
    ? "text-center"
    : options.align === "right"
      ? "text-right"
      : "text-left"

  return (
    <Tag
      key={key}
      className={cn(
        "px-2 py-1 align-top whitespace-pre-wrap text-sm leading-snug",
        options.isHeader && "bg-background font-semibold text-foreground",
        alignClass,
      )}
    >
      {renderChildren(cell.children, `${key}-cell`)}
    </Tag>
  )
}

function renderNode(node: MarkdownNode, key: string): ReactNode {
  const children = renderChildren(node.children, key)

  switch (node.type) {
    case "root":
      return (
        <React.Fragment key={key}>
          {children}
        </React.Fragment>
      )
    case "paragraph":
      return (
        <p key={key} className="whitespace-pre-wrap leading-snug [&:not(:last-child)]:mb-1.5">
          {children}
        </p>
      )
    case "text":
      return <React.Fragment key={key}>{node.value || ""}</React.Fragment>
    case "strong":
      return <strong key={key} className="font-semibold text-foreground">{children}</strong>
    case "emphasis":
      return <em key={key} className="italic">{children}</em>
    case "delete":
      return <del key={key} className="line-through opacity-80">{children}</del>
    case "inlineCode":
      return (
        <code
          key={key}
          className="rounded bg-background px-1 py-0.5 font-mono text-[0.85em] text-foreground ring-1 ring-border/70"
        >
          {node.value || ""}
        </code>
      )
    case "code":
      return <MarkdownCodeBlock key={key} lang={node.lang} code={node.value || ""} />
    case "blockquote":
      return (
        <blockquote
          key={key}
          className="my-1.5 border-l-2 border-border pl-3 text-sm leading-snug text-muted-foreground [&_p]:whitespace-pre-wrap [&_p]:mb-1"
        >
          {children}
        </blockquote>
      )
    case "heading": {
      const classesByDepth: Record<number, string> = {
        1: "text-lg font-semibold tracking-tight",
        2: "text-base font-semibold tracking-tight",
        3: "text-sm font-semibold",
        4: "text-sm font-semibold uppercase tracking-[0.1em] text-foreground/80",
        5: "text-sm font-semibold text-foreground/80",
        6: "text-xs font-semibold uppercase tracking-[0.14em] text-muted-foreground",
      }

      const Tag = `h${Math.min(Math.max(node.depth || 1, 1), 6)}` as keyof JSX.IntrinsicElements

      return (
        <Tag key={key} className={cn("mt-2 mb-0.5 first:mt-0", classesByDepth[node.depth || 1] || classesByDepth[6])}>
          {children}
        </Tag>
      )
    }
    case "list":
      return node.ordered ? (
        <ol key={key} start={node.start || 1} className="my-1.5 ml-4 list-decimal space-y-0.5 text-sm leading-snug">
          {children}
        </ol>
      ) : (
        <ul key={key} className="my-1.5 ml-4 list-disc space-y-0.5 text-sm leading-snug">
          {children}
        </ul>
      )
    case "listItem":
      return (
        <li key={key} className="whitespace-pre-wrap marker:text-muted-foreground">
          {typeof node.checked === "boolean" ? (
            <label className="flex items-start gap-2">
              <input
                type="checkbox"
                checked={node.checked}
                readOnly
                className="mt-0.5 h-3.5 w-3.5 rounded border-border"
              />
              <span className="min-w-0 flex-1">{children}</span>
            </label>
          ) : (
            children
          )}
        </li>
      )
    case "link": {
      const safe = isSafeLink(node.url)

      if (!safe) {
        return (
          <span key={key} className="font-medium underline decoration-dotted underline-offset-2">
            {children}
          </span>
        )
      }

      return (
        <a
          key={key}
          href={node.url}
          target={isExternalLink(node.url) ? "_blank" : undefined}
          rel={isExternalLink(node.url) ? "noreferrer noopener" : undefined}
          className="font-medium text-blue-600 underline decoration-blue-300 underline-offset-2 transition-colors hover:text-blue-700"
        >
          {children}
        </a>
      )
    }
    case "image":
      if (!isSafeLink(node.url)) {
        return (
          <span key={key} className="text-muted-foreground">
            {node.alt || node.url || ""}
          </span>
        )
      }

      return (
        <img
          key={key}
          src={node.url}
          alt={node.alt || ""}
          loading="lazy"
          className="my-1.5 max-h-96 rounded-md border border-border/70 object-contain"
        />
      )
    case "table": {
      const rows = node.children || []
      const [headerRow, ...bodyRows] = rows
      const alignments = node.align || []

      return (
        <div key={key} className="my-1.5 overflow-x-auto rounded-md border border-border/80">
          <table className="min-w-full border-collapse text-left text-sm leading-snug">
            {headerRow && (
              <thead>
                <tr className="border-b border-border/70">
                  {(headerRow.children || []).map((cell, index) =>
                    renderTableCell(cell, `${key}-header-${index}`, {
                      isHeader: true,
                      align: alignments[index] || null,
                    }),
                  )}
                </tr>
              </thead>
            )}
            {bodyRows.length > 0 && (
              <tbody>
                {bodyRows.map((row, rowIndex) => (
                  <tr key={`${key}-row-${rowIndex}`} className="border-t border-border/70 first:border-t-0">
                    {(row.children || []).map((cell, cellIndex) =>
                      renderTableCell(cell, `${key}-row-${rowIndex}-cell-${cellIndex}`, {
                        isHeader: false,
                        align: alignments[cellIndex] || null,
                      }),
                    )}
                  </tr>
                ))}
              </tbody>
            )}
          </table>
        </div>
      )
    }
    case "thematicBreak":
      return <hr key={key} className="my-2 border-border/80" />
    case "break":
      return <br key={key} />
    default:
      return children.length ? <React.Fragment key={key}>{children}</React.Fragment> : null
  }
}

export function MarkdownRenderer({ content, className }: MarkdownRendererProps) {
  const rendered = useMemo(() => {
    try {
      const tree = markdownProcessor.parse(content) as MarkdownRoot
      return renderChildren(tree.children, "markdown")
    } catch (error) {
      console.error("Failed to render markdown:", error)
      return null
    }
  }, [content])

  if (!rendered) {
    return <div className={cn("whitespace-pre-wrap text-sm leading-snug", className)}>{content}</div>
  }

  return (
    <div className={cn("break-words text-sm leading-snug text-foreground", className)}>
      {rendered}
    </div>
  )
}
