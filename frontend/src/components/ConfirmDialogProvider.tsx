import React, { createContext, useCallback, useContext, useMemo, useRef, useState } from "react"

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { cn } from "@/lib/utils"

type ConfirmDialogTone = "default" | "destructive"

export interface ConfirmDialogOptions {
  title: string
  description?: string
  confirmLabel?: string
  cancelLabel?: string
  tone?: ConfirmDialogTone
}

type ConfirmDialogContextValue = {
  confirm: (options: ConfirmDialogOptions) => Promise<boolean>
}

const ConfirmDialogContext = createContext<ConfirmDialogContextValue | null>(null)

type PendingConfirm = Required<Omit<ConfirmDialogOptions, "description">> & {
  description: string
}

const DEFAULT_CONFIRM_OPTIONS: PendingConfirm = {
  title: "",
  description: "",
  confirmLabel: "确认",
  cancelLabel: "取消",
  tone: "default",
}

export function ConfirmDialogProvider({ children }: { children: React.ReactNode }) {
  const [pending, setPending] = useState<PendingConfirm | null>(null)
  const resolverRef = useRef<((value: boolean) => void) | null>(null)

  const settle = useCallback((value: boolean) => {
    const resolver = resolverRef.current
    resolverRef.current = null
    setPending(null)
    resolver?.(value)
  }, [])

  const confirm = useCallback((options: ConfirmDialogOptions) => {
    return new Promise<boolean>((resolve) => {
      if (resolverRef.current) {
        resolverRef.current(false)
      }
      resolverRef.current = resolve
      setPending({
        ...DEFAULT_CONFIRM_OPTIONS,
        ...options,
        description: options.description ?? "",
      })
    })
  }, [])

  const value = useMemo<ConfirmDialogContextValue>(() => ({ confirm }), [confirm])

  return (
    <ConfirmDialogContext.Provider value={value}>
      {children}
      <AlertDialog
        open={!!pending}
        onOpenChange={(open) => {
          if (!open && pending) {
            settle(false)
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{pending?.title}</AlertDialogTitle>
            {pending?.description ? (
              <AlertDialogDescription className="whitespace-pre-wrap">
                {pending.description}
              </AlertDialogDescription>
            ) : null}
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => settle(false)}>
              {pending?.cancelLabel ?? DEFAULT_CONFIRM_OPTIONS.cancelLabel}
            </AlertDialogCancel>
            <AlertDialogAction
              onClick={() => settle(true)}
              className={cn(
                pending?.tone === "destructive" &&
                  "bg-destructive text-destructive-foreground hover:bg-destructive/90",
              )}
            >
              {pending?.confirmLabel ?? DEFAULT_CONFIRM_OPTIONS.confirmLabel}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </ConfirmDialogContext.Provider>
  )
}

export function useConfirmDialog() {
  const context = useContext(ConfirmDialogContext)
  if (!context) {
    throw new Error("useConfirmDialog must be used within ConfirmDialogProvider")
  }
  return context
}
