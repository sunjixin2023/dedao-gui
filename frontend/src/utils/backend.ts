type BackendMethod = (...args: any[]) => Promise<any>

type WailsBackend = {
  App?: Record<string, BackendMethod>
}

type WailsWindow = Window & {
  go?: {
    backend?: WailsBackend
  }
}

export const hasBackendBridge = () => {
  const app = (window as WailsWindow).go?.backend?.App
  return Boolean(app)
}

export const invokeBackend = async <T>(method: string, ...args: any[]) => {
  const app = (window as WailsWindow).go?.backend?.App
  const fn = app?.[method]
  if (typeof fn !== 'function') {
    throw new Error('桌面后端未就绪，请稍后重试')
  }
  return (await fn(...args)) as T
}
