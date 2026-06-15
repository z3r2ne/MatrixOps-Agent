import React, { createContext, useCallback, useContext, useEffect, useMemo, useRef } from "react"
import { Toaster, toast } from "sonner"
import { httpClient } from "@/lib/httpClient"

type NotificationType = "success" | "error" | "info"

interface NotifyPayload {
  type: NotificationType
  title: string
  description?: string
}

interface NotificationContextValue {
  notify: (payload: NotifyPayload) => void
}

const NotificationContext = createContext<NotificationContextValue | null>(null)

const GENERIC_REQUEST_ERROR_TITLE = "请求失败"
const GENERIC_REQUEST_ERROR_DELAY_MS = 120

export const NotificationProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const originalToastErrorRef = useRef<typeof toast.error | null>(null)
  const pendingGenericErrorRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const clearPendingGenericError = useCallback(() => {
    if (pendingGenericErrorRef.current !== null) {
      clearTimeout(pendingGenericErrorRef.current)
      pendingGenericErrorRef.current = null
    }
  }, [])

  const notify = useCallback((payload: NotifyPayload) => {
    const options = payload.description ? { description: payload.description } : undefined

    switch (payload.type) {
      case "success":
        toast.success(payload.title, options)
        break
      case "error":
        toast.error(payload.title, options)
        break
      default:
        toast(payload.title, options)
        break
    }
  }, [])

  const value = useMemo(() => ({ notify }), [notify])

  useEffect(() => {
    const originalToastError = toast.error.bind(toast)
    originalToastErrorRef.current = originalToastError

    const patchedToastError = ((message: Parameters<typeof toast.error>[0], options?: Parameters<typeof toast.error>[1]) => {
      const title = typeof message === "string" ? message.trim() : String(message ?? "").trim()

      if (title === GENERIC_REQUEST_ERROR_TITLE) {
        clearPendingGenericError()
        pendingGenericErrorRef.current = setTimeout(() => {
          pendingGenericErrorRef.current = null
          originalToastError(message, options)
        }, GENERIC_REQUEST_ERROR_DELAY_MS)
        return title
      }

      clearPendingGenericError()
      return originalToastError(message, options)
    }) as typeof toast.error

    toast.error = patchedToastError

    return () => {
      clearPendingGenericError()
      if (originalToastErrorRef.current) {
        toast.error = originalToastErrorRef.current
      }
    }
  }, [clearPendingGenericError])

  useEffect(() => {
    httpClient.setNotifyCallback(notify)

    return () => {
      httpClient.setNotifyCallback(null)
    }
  }, [notify])

  return (
    <NotificationContext.Provider value={value}>
      {children}
      <Toaster closeButton duration={4000} position="bottom-right" richColors theme="light" />
    </NotificationContext.Provider>
  )
}

export const useNotification = () => {
  const context = useContext(NotificationContext)
  if (!context) {
    throw new Error("useNotification must be used within NotificationProvider")
  }
  return context
}
