export type DownloadNotifyState = 'queued' | 'downloading' | 'verifying' | 'completed' | 'failed' | 'cancelled'

export type DownloadNotifyPayload = {
  title: string
  message: string
  type: 'success' | 'error' | 'warning' | 'info'
}

const terminalStates = new Set<DownloadNotifyState>(['completed', 'failed', 'cancelled'])

export const downloadNotifyMessage = (
  state: DownloadNotifyState,
  content: string,
): DownloadNotifyPayload => {
  const message = String(content ?? '').trim() || '下载任务已更新'
  switch (state) {
    case 'completed':
      return { title: '下载完成', message, type: 'success' }
    case 'failed':
      return { title: '下载失败', message, type: 'error' }
    case 'cancelled':
      return { title: '下载已取消', message, type: 'warning' }
    default:
      return { title: '下载', message, type: 'info' }
  }
}

export const notifyDownloadEnd = (
  state: DownloadNotifyState,
  content: string,
  emit: (payload: DownloadNotifyPayload) => void,
) => {
  if (!terminalStates.has(state)) {
    return
  }
  emit(downloadNotifyMessage(state, content))
}

type SystemNotificationCtor = {
  permission?: string
  requestPermission?: () => Promise<string>
  new (title: string, options?: { body: string }): unknown
}

let permissionRequested = false

export const resetDownloadNotifySession = () => {
  permissionRequested = false
}

export const trySystemNotification = (
  payload: DownloadNotifyPayload,
  notificationCtor?: SystemNotificationCtor,
) => {
  const NotificationImpl = notificationCtor ?? (globalThis as { Notification?: SystemNotificationCtor }).Notification
  if (!NotificationImpl) {
    return
  }

  const show = () => {
    try {
      void new NotificationImpl(payload.title, { body: payload.message })
    } catch {
      // WebView may expose Notification without a usable constructor.
    }
  }

  if (NotificationImpl.permission === 'granted') {
    show()
    return
  }
  if (permissionRequested || typeof NotificationImpl.requestPermission !== 'function') {
    return
  }
  permissionRequested = true
  void NotificationImpl.requestPermission()
    .then((perm) => {
      if (perm === 'granted') {
        show()
      }
    })
    .catch(() => undefined)
}
