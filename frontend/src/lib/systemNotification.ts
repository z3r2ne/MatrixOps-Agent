export interface SystemNotificationOptions {
  body?: string;
  tag?: string;
  onClick?: () => void;
}

let permissionRequest: Promise<NotificationPermission> | null = null;

export type SystemNotificationSound = "success" | "error" | "neutral"

export function playSystemNotificationSound(kind: SystemNotificationSound) {
  if (typeof window === "undefined") return
  // Best-effort: browsers may block autoplay without a gesture.
  try {
    const AudioCtx = (window.AudioContext || (window as any).webkitAudioContext) as
      | (new () => AudioContext)
      | undefined
    if (!AudioCtx) return

    const ctx = new AudioCtx()
    const now = ctx.currentTime
    const master = ctx.createGain()
    master.gain.setValueAtTime(0.0001, now)
    master.gain.exponentialRampToValueAtTime(0.12, now + 0.01)
    master.gain.exponentialRampToValueAtTime(0.0001, now + 0.35)
    master.connect(ctx.destination)

    const beep = (freq: number, start: number, dur: number) => {
      const osc = ctx.createOscillator()
      osc.type = "sine"
      osc.frequency.setValueAtTime(freq, start)
      osc.connect(master)
      osc.start(start)
      osc.stop(start + dur)
    }

    if (kind === "success") {
      beep(880, now, 0.08)
      beep(1175, now + 0.09, 0.10)
    } else if (kind === "error") {
      beep(330, now, 0.12)
      beep(220, now + 0.14, 0.16)
    } else {
      beep(660, now, 0.10)
    }

    // Cleanup after the envelope finishes.
    window.setTimeout(() => {
      try {
        void ctx.close()
      } catch {}
    }, 600)
  } catch {
    // ignore
  }
}

async function ensurePermission(): Promise<NotificationPermission> {
  if (typeof window === "undefined" || !("Notification" in window)) {
    return "denied";
  }
  if (Notification.permission !== "default") {
    return Notification.permission;
  }
  if (!permissionRequest) {
    permissionRequest = Notification.requestPermission().finally(() => {
      permissionRequest = null;
    });
  }
  return permissionRequest;
}

export async function sendSystemNotification(title: string, options: SystemNotificationOptions = {}) {
  const permission = await ensurePermission();
  if (permission !== "granted") {
    return null;
  }

  const notification = new Notification(title, {
    body: options.body,
    tag: options.tag,
    renotify: false,
    silent: true,
  });

  if (options.onClick) {
    notification.onclick = () => {
      try {
        window.focus();
      } catch {}
      options.onClick?.();
      notification.close();
    };
  }

  return notification;
}
