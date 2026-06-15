import React from "react"
import { Download, FileText, Paperclip, Play } from "lucide-react"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import type { Part } from "@/lib/api"

type AttachmentVariant = "user" | "assistant"

function attachmentURL(part: Part): string {
  return (part.url || "").trim()
}

function attachmentName(part: Part): string {
  const name = (part.filename || "").trim()
  if (name) return name
  const url = attachmentURL(part)
  if (!url) return "附件"
  try {
    if (url.startsWith("data:")) return "附件"
    const base = url.split("/").pop()?.split("?")[0]
    return base || "附件"
  } catch {
    return "附件"
  }
}

function attachmentMime(part: Part): string {
  const mime = (part.mime || "").trim()
  if (mime) return mime
  const url = attachmentURL(part)
  if (url.startsWith("data:")) {
    const semi = url.indexOf(";")
    if (semi > 5) return url.slice(5, semi)
  }
  return ""
}

function hasImageExtension(value: string): boolean {
  return /\.(png|jpe?g|gif|webp|bmp|svg)$/i.test(value)
}

function hasVideoExtension(value: string): boolean {
  return /\.(mp4|webm|mov|mkv|avi)$/i.test(value)
}

function hasAudioExtension(value: string): boolean {
  return /\.(mp3|wav|ogg|m4a|aac|flac)$/i.test(value)
}

function isImageAttachment(part: Part): boolean {
  const mime = attachmentMime(part)
  if (mime.startsWith("image/")) return true
  const label = `${part.filename || ""} ${attachmentURL(part)}`
  return hasImageExtension(label)
}

function isVideoAttachment(part: Part): boolean {
  const mime = attachmentMime(part)
  if (mime.startsWith("video/")) return true
  return hasVideoExtension(part.filename || attachmentURL(part))
}

function isAudioAttachment(part: Part): boolean {
  const mime = attachmentMime(part)
  if (mime.startsWith("audio/")) return true
  return hasAudioExtension(part.filename || attachmentURL(part))
}

function downloadAttachment(part: Part) {
  const url = attachmentURL(part)
  if (!url) return
  const name = attachmentName(part)
  const anchor = document.createElement("a")
  anchor.href = url
  anchor.download = name
  anchor.rel = "noreferrer"
  anchor.target = "_blank"
  document.body.appendChild(anchor)
  anchor.click()
  document.body.removeChild(anchor)
}

export function isRenderableFilePart(part: Part | null | undefined): boolean {
  return Boolean(part && part.type === "file" && (attachmentURL(part) || part.path))
}

interface MessageAttachmentBubbleProps {
  part: Part
  variant?: AttachmentVariant
  className?: string
}

export function MessageAttachmentBubble({
  part,
  variant = "user",
  className,
}: MessageAttachmentBubbleProps) {
  const url = attachmentURL(part)
  if (!url) return null

  const name = attachmentName(part)
  const mime = attachmentMime(part)
  const isUser = variant === "user"

  const shellClass = cn(
    "max-w-full overflow-hidden rounded-md border px-2 py-1.5 text-sm shadow-sm",
    isUser
      ? "border-primary-foreground/25 bg-primary/85 text-primary-foreground"
      : "border-border/70 bg-muted/50 text-foreground",
    className,
  )

  const actionButtonClass = cn(
    "h-7 gap-1 px-2 text-xs",
    isUser
      ? "text-primary-foreground hover:bg-primary-foreground/10 hover:text-primary-foreground"
      : "text-muted-foreground hover:bg-muted hover:text-foreground",
  )

  const renderDownload = () => (
    <Button
      type="button"
      variant="ghost"
      size="sm"
      className={actionButtonClass}
      onClick={() => downloadAttachment(part)}
    >
      <Download className="h-3.5 w-3.5" />
      下载
    </Button>
  )

  if (isImageAttachment(part)) {
    return (
      <div className={cn(shellClass, "space-y-2")}>
        <img src={url} alt={name} className="max-h-64 max-w-full rounded object-contain" />
        <div className={cn("flex items-center justify-between gap-2 text-xs", isUser ? "text-primary-foreground/85" : "text-muted-foreground")}>
          <span className="truncate" title={name}>{name}</span>
          {renderDownload()}
        </div>
      </div>
    )
  }

  if (isVideoAttachment(part)) {
    return (
      <div className={cn(shellClass, "space-y-2")}>
        <video src={url} controls className="max-h-64 max-w-full rounded bg-black/80" preload="metadata">
          您的浏览器不支持视频播放
        </video>
        <div className={cn("flex items-center justify-between gap-2 text-xs", isUser ? "text-primary-foreground/85" : "text-muted-foreground")}>
          <span className="inline-flex items-center gap-1 truncate" title={name}>
            <Play className="h-3 w-3 shrink-0" />
            {name}
          </span>
          {renderDownload()}
        </div>
      </div>
    )
  }

  if (isAudioAttachment(part)) {
    return (
      <div className={cn(shellClass, "space-y-2")}>
        <audio src={url} controls className="w-full max-w-full" preload="metadata">
          您的浏览器不支持音频播放
        </audio>
        <div className={cn("flex items-center justify-between gap-2 text-xs", isUser ? "text-primary-foreground/85" : "text-muted-foreground")}>
          <span className="truncate" title={name}>{name}</span>
          {renderDownload()}
        </div>
      </div>
    )
  }

  const linkClass = cn(
    "break-all font-medium underline underline-offset-2 [overflow-wrap:anywhere]",
    isUser ? "decoration-primary-foreground/50" : "decoration-muted-foreground/50",
  )

  return (
    <div className={shellClass}>
      <div className="flex items-start justify-between gap-2">
        <a href={url} target="_blank" rel="noreferrer" className={linkClass}>
          {mime === "application/pdf" ? (
            <FileText className="mr-1 inline h-3.5 w-3.5 align-text-bottom opacity-90" />
          ) : (
            <Paperclip className="mr-1 inline h-3.5 w-3.5 align-text-bottom opacity-90" />
          )}
          {name}
        </a>
        {renderDownload()}
      </div>
      {mime && (
        <div className={cn("mt-1 text-[11px]", isUser ? "text-primary-foreground/70" : "text-muted-foreground")}>
          {mime}
        </div>
      )}
    </div>
  )
}
