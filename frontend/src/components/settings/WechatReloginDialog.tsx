import React, { useCallback, useEffect, useRef, useState } from "react"
import { useNotification } from "@/components/NotificationProvider"
import { api } from "@/lib/api"
import { useGlobalWebSocket } from "@/hooks/useGlobalWebSocket"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { QrCode } from "lucide-react"

type ExpiredAccountPayload = {
  accountId: number
  botId: string
  ilinkUserId: string
}

export function WechatReloginDialog() {
  const { notify } = useNotification()
  const [open, setOpen] = useState(false)
  const [expired, setExpired] = useState<ExpiredAccountPayload | null>(null)
  const [loading, setLoading] = useState(false)
  const pollTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const qrLoginWindowRef = useRef<{ electronId?: string; browserWindow?: Window | null } | null>(null)

  const closeQrLoginWindow = useCallback(() => {
    const w = qrLoginWindowRef.current
    if (!w) return
    if (window.electronAPI?.closeExternalWindow && w.electronId) {
      void window.electronAPI.closeExternalWindow(w.electronId)
    } else if (w.browserWindow && !w.browserWindow.closed) {
      w.browserWindow.close()
    }
    qrLoginWindowRef.current = null
  }, [])

  const resetDialog = useCallback(() => {
    if (pollTimerRef.current) {
      clearTimeout(pollTimerRef.current)
      pollTimerRef.current = null
    }
    closeQrLoginWindow()
    setLoading(false)
    setExpired(null)
    setOpen(false)
  }, [closeQrLoginWindow])

  useEffect(() => {
    return () => {
      if (pollTimerRef.current) {
        clearTimeout(pollTimerRef.current)
      }
      closeQrLoginWindow()
    }
  }, [closeQrLoginWindow])

  useGlobalWebSocket({
    onILinkSessionExpired: (payload) => {
      setExpired(payload)
      setOpen(true)
    },
  })

  const pollQRStatus = useCallback(
    async (qrcode: string) => {
      try {
        const account = await api.pollWechatQRStatus(qrcode)
        closeQrLoginWindow()
        notify({
          type: "success",
          title: "重新登录成功",
          description: `账号 ${account.ilinkUserId || account.botId} 已恢复启用`,
        })
        resetDialog()
      } catch (error: unknown) {
        const status = (error as { status?: number })?.status
        if (status === 202) {
          pollTimerRef.current = setTimeout(() => void pollQRStatus(qrcode), 3000)
        } else {
          notify({
            type: "error",
            title: "登录失败",
            description: error instanceof Error ? error.message : "未知错误",
          })
          setLoading(false)
        }
      }
    },
    [notify, resetDialog, closeQrLoginWindow],
  )

  const handleFetchQR = async () => {
    setLoading(true)
    try {
      const data = await api.fetchWechatQRCode()
      closeQrLoginWindow()
      if (window.electronAPI?.openExternalWindow) {
        const result = await window.electronAPI.openExternalWindow({
          url: data.qrcode_img_content,
          title: "扫码重新登录",
        })
        if (result.windowId) {
          qrLoginWindowRef.current = { electronId: result.windowId }
        }
      } else {
        const win = window.open(data.qrcode_img_content, "_blank")
        qrLoginWindowRef.current = { browserWindow: win }
      }
      void pollQRStatus(data.qrcode)
    } catch (error) {
      notify({
        type: "error",
        title: "获取二维码失败",
        description: error instanceof Error ? error.message : "未知错误",
      })
      setLoading(false)
    }
  }

  const accountLabel = expired?.ilinkUserId || expired?.botId || "微信账号"

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) resetDialog()
        else setOpen(true)
      }}
    >
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>微信会话已过期</DialogTitle>
          <DialogDescription>
            账号「{accountLabel}」的微信登录已失效，已自动关闭该账号。请扫码重新登录以恢复启用。
          </DialogDescription>
        </DialogHeader>
        <DialogFooter className="gap-2 sm:gap-0">
          <Button type="button" variant="outline" onClick={resetDialog} disabled={loading}>
            稍后
          </Button>
          <Button type="button" onClick={() => void handleFetchQR()} disabled={loading}>
            <QrCode className="mr-2 h-4 w-4" />
            {loading ? "等待扫码…" : "扫码重新登录"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
