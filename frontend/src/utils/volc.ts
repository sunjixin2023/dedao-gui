export type VolcPlaybackAuth = {
  playAuthToken: string
  keyToken: string
  vid: string
  format: string
}

export type MediaVolcLike = {
  volc_play_auth_token?: string
  play_auth_token?: string
  tracks?: Array<{
    formats?: Array<{
      volc_id?: string
      volc_play_auth_token?: string
      volc_key_token?: string
      format?: string
    }>
  }>
}

export const pickVolcPlaybackAuth = (volc: MediaVolcLike | null | undefined): VolcPlaybackAuth => {
  const formats = volc?.tracks?.flatMap((track) => track.formats ?? []) ?? []
  const format = formats.find((candidate) => String(candidate?.volc_play_auth_token ?? '').trim())
  if (format) {
    return {
      playAuthToken: String(format.volc_play_auth_token ?? '').trim(),
      keyToken: String(format.volc_key_token ?? '').trim(),
      vid: String(format.volc_id ?? '').trim(),
      format: String(format.format ?? '').trim(),
    }
  }

  return {
    playAuthToken: String(volc?.volc_play_auth_token ?? volc?.play_auth_token ?? '').trim(),
    keyToken: '',
    vid: '',
    format: '',
  }
}

export const isV2PlayAuthToken = (token: string): boolean => {
  return getV2SignedPlayInfoQuery(token) !== ''
}

export const getV2SignedPlayInfoQuery = (token: string): string => {
  try {
    const normalized = String(token ?? '').trim().replace(/-/g, '+').replace(/_/g, '/')
    const padded = normalized.padEnd(Math.ceil(normalized.length / 4) * 4, '=')
    const payload = JSON.parse(atob(padded)) as Record<string, unknown>
    if (payload.TokenVersion !== 'V2' || typeof payload.GetPlayInfoToken !== 'string') return ''
    const query = payload.GetPlayInfoToken.trim()
    const values = new URLSearchParams(query)
    if (values.get('Action') !== 'GetPlayInfo' || values.get('Version') !== '2020-08-01') return ''
    return query
  } catch {
    return ''
  }
}

export const createV2PlayInfoResolver = (
  resolveLocal: (...args: any[]) => Promise<unknown>,
  fallback: (...args: any[]) => unknown,
) => {
  return (...args: any[]): unknown => {
    const [token] = args
    if (getV2SignedPlayInfoQuery(String(token ?? ''))) return resolveLocal(...args)
    return fallback(...args)
  }
}

const VOLC_API_HOSTS = new Set(['vod.volcengineapi.com', 'vod.volces.com'])
const VOLC_QUERY_KEY = '__dedaoVolcQuery'

const hostOfOpaqueUrl = (value: string): string => value.split(/[/?#]/, 1)[0]

export const extractVolcApiQuery = (url: string): string => {
  const raw = String(url ?? '').trim()
  let rest = raw
  if (raw.startsWith('//')) rest = raw.slice(2)
  else if (raw.startsWith('wails://')) rest = raw.slice('wails://'.length)
  else if (raw.startsWith('https://')) rest = raw.slice('https://'.length)
  else if (raw.startsWith('http://')) rest = raw.slice('http://'.length)
  else return ''

  const host = hostOfOpaqueUrl(rest)
  if (!VOLC_API_HOSTS.has(host)) return ''
  const queryIndex = rest.indexOf('?')
  if (queryIndex < 0) return ''
  return rest.slice(queryIndex + 1)
}

export const rewriteVolcRequestUrl = (url: string, protocol: string): string => {
  const raw = String(url ?? '').trim()
  if (!raw || protocol !== 'wails:') return raw

  if (raw.startsWith('//')) {
    const host = hostOfOpaqueUrl(raw.slice(2))
    if (VOLC_API_HOSTS.has(host)) return `https:${raw}`
    return raw
  }

  if (raw.startsWith('wails://')) {
    const rest = raw.slice('wails://'.length)
    const host = hostOfOpaqueUrl(rest)
    if (VOLC_API_HOSTS.has(host)) return `https://${rest}`
  }

  return raw
}

type VolcXhrProto = {
  open: (this: any, method: string, url: string, ...rest: any[]) => any
  send: (this: any, ...args: any[]) => any
}

export const installVolcUrlBridge = (
  xhrProto: VolcXhrProto,
  protocol: string,
  proxyGet?: (query: string) => Promise<string>,
): (() => void) => {
  if (protocol !== 'wails:') return () => undefined

  const originalOpen = xhrProto.open
  const originalSend = xhrProto.send

  const patchedOpen: VolcXhrProto['open'] = function (this: any, method: string, url: string, ...rest: any[]) {
    const query = extractVolcApiQuery(String(url ?? ''))
    if (query && String(method).toUpperCase() === 'GET' && proxyGet) {
      this[VOLC_QUERY_KEY] = query
      return
    }
    this[VOLC_QUERY_KEY] = ''
    return originalOpen.call(this, method, rewriteVolcRequestUrl(String(url ?? ''), protocol), ...rest)
  }

  const patchedSend: VolcXhrProto['send'] = function (this: any, body?: unknown) {
    const query = String(this[VOLC_QUERY_KEY] ?? '')
    if (!query || !proxyGet) {
      return originalSend.call(this, body)
    }

    void proxyGet(query)
      .then((text) => {
        const parsed = (() => {
          try {
            return JSON.parse(text)
          } catch {
            return text
          }
        })()
        this.status = 200
        this.statusText = 'OK'
        this.responseText = text
        this.response = parsed
        this.readyState = 2
        const onready = this.onreadystatechange as (() => void) | undefined
        onready?.()
        this.readyState = 4
        onready?.()
        const onload = this.onload as ((ev: { target: unknown }) => void) | undefined
        onload?.({ target: this })
      })
      .catch(() => {
        this.status = 0
        this.readyState = 4
        const onerror = this.onerror as ((ev: unknown) => void) | undefined
        onerror?.(new Error('火山点播代理失败'))
      })
  }

  xhrProto.open = patchedOpen
  xhrProto.send = patchedSend

  return () => {
    if (xhrProto.open === patchedOpen) xhrProto.open = originalOpen
    if (xhrProto.send === patchedSend) xhrProto.send = originalSend
  }
}

type CookieJar = {
  cookie: string
}

export const installVolcCookieBridge = (
  cookieJar: CookieJar,
  protocol: string,
): (() => void) => {
  if (protocol !== 'wails:') return () => undefined

  const originalDescriptor = Object.getOwnPropertyDescriptor(cookieJar, 'cookie')
  const originalCookie = String(cookieJar.cookie ?? '')
  let volcCookie = ''

  Object.defineProperty(cookieJar, 'cookie', {
    configurable: true,
    get: () => [originalCookie, volcCookie].filter(Boolean).join('; '),
    set: (value: string) => {
      const cookie = String(value ?? '').split(';', 1)[0].trim()
      if (cookie.startsWith('volcvui=')) volcCookie = cookie
    },
  })

  return () => {
    delete (cookieJar as Partial<CookieJar>).cookie
    if (originalDescriptor) Object.defineProperty(cookieJar, 'cookie', originalDescriptor)
  }
}

type PlaybackDebugFields = {
  title: string
  runtime: string
  playMode: string
  streamUrl: string
  mediaId: string
  securityToken: string
  tokenSource: string
  lineAppId: number
  status: string
  error: string
}

export const buildSafePlaybackDebugInfo = (fields: PlaybackDebugFields): string => {
  return [
    `title=${fields.title}`,
    `runtime=${fields.runtime}`,
    `play_mode=${fields.playMode}`,
    `stream_url_present=${Boolean(fields.streamUrl)}`,
    `media_id=${fields.mediaId}`,
    `security_token_present=${Boolean(fields.securityToken)}`,
    `security_token_length=${fields.securityToken.length}`,
    `token_source=${fields.tokenSource}`,
    `line_app_id=${fields.lineAppId}`,
    `status=${fields.status}`,
    `error=${fields.error}`,
  ].join('\n')
}
