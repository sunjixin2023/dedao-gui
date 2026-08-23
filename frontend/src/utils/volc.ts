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

const hasPlayAuthToken = (format: { volc_play_auth_token?: string } | null | undefined) => {
  return Boolean(String(format?.volc_play_auth_token ?? '').trim())
}

const hasKeyToken = (format: { volc_key_token?: string } | null | undefined) => {
  return Boolean(String(format?.volc_key_token ?? '').trim())
}

export const pickVolcPlaybackAuth = (volc: MediaVolcLike | null | undefined): VolcPlaybackAuth => {
  const formats = volc?.tracks?.flatMap((track) => track.formats ?? []) ?? []
  const format =
    formats.find((candidate) => hasPlayAuthToken(candidate) && hasKeyToken(candidate)) ??
    formats.find((candidate) => hasPlayAuthToken(candidate))
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
const MEDIA_URL_KEY = '__dedaoMediaUrl'
const MEDIA_HEADERS_KEY = '__dedaoMediaHeaders'
const ALLOWED_MEDIA_HOST_SUFFIXES = [
  'umiwi.com',
  'volces.com',
  'volccdn.com',
  'byteimg.com',
  'bytedance.com',
  'bytecdn.com',
]

const hostOfOpaqueUrl = (value: string): string => value.split(/[/?#]/, 1)[0]

export const hostOfRequestUrl = (url: string): string => {
  const raw = String(url ?? '').trim()
  let rest = raw
  if (raw.startsWith('//')) rest = raw.slice(2)
  else if (raw.startsWith('wails://')) rest = raw.slice('wails://'.length)
  else if (raw.startsWith('https://')) rest = raw.slice('https://'.length)
  else if (raw.startsWith('http://')) rest = raw.slice('http://'.length)
  else return hostOfOpaqueUrl(raw)
  return hostOfOpaqueUrl(rest)
}

export const isAllowedMediaHost = (host: string): boolean => {
  const hostname = String(host ?? '').trim().toLowerCase().split(':')[0]
  if (!hostname) return false
  return ALLOWED_MEDIA_HOST_SUFFIXES.some(
    (suffix) => hostname === suffix || hostname.endsWith(`.${suffix}`),
  )
}

export const extractProxiedMediaUrl = (url: string, protocol: string): string => {
  if (protocol !== 'wails:') return ''
  const raw = String(url ?? '').trim()
  let rest = raw
  if (raw.startsWith('//')) rest = raw.slice(2)
  else if (raw.startsWith('wails://')) rest = raw.slice('wails://'.length)
  else if (raw.startsWith('https://')) rest = raw.slice('https://'.length)
  else if (raw.startsWith('http://')) rest = raw.slice('http://'.length)
  else return ''

  const host = hostOfOpaqueUrl(rest)
  if (!isAllowedMediaHost(host)) return ''
  return `https://${rest}`
}

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

export const buildPrivateDrmAuthQuery = (
  keyToken: string,
  playAuthIDs: string,
  vid: string,
  unionInfo: string,
): string => {
  const query = String(keyToken ?? '').trim()
  const ids = String(playAuthIDs ?? '').trim()
  const videoId = String(vid ?? '').trim()
  const union = String(unionInfo ?? '').trim()
  if (!query || !ids || !videoId || !union) {
    throw new Error('私有加密播放参数不完整')
  }
  if (query.includes('\r') || query.includes('\n')) {
    throw new Error('火山点播请求参数包含非法换行')
  }
  const values = new URLSearchParams(query)
  if (values.get('Action') !== 'GetPrivateDrmPlayAuth') {
    throw new Error('火山点播 Action 必须为 GetPrivateDrmPlayAuth')
  }
  if (values.get('Version') !== '2020-08-01') {
    throw new Error('火山点播 Version 必须为 2020-08-01')
  }
  if (values.get('Vid') !== videoId) {
    throw new Error('私有加密凭证 Vid 与播放器 Vid 不一致')
  }
  const runtime = new URLSearchParams({
    DrmType: 'webdevice',
    PlayAuthIds: ids,
    UnionInfo: union,
  })
  return `${query}&${runtime.toString()}`
}

export type PlaybackRuntimeProbe = {
  protocol: string
  secure_context: boolean
  crypto_subtle: boolean
  media_source: boolean
}

export const privateDashPluginOptions = (): { useEME: boolean; useUnionInfoDRM: boolean } => {
  // VePlayer DASH defaults useEME:true (Widevine). Dedao streams are private_encrypt.
  return {
    useEME: false,
    useUnionInfoDRM: true,
  }
}

export const desktopPlaybackRates = (): number[] => [0.5, 0.75, 1, 1.25, 1.5, 2]

export const shouldRecordPlaybackProbe = (kind: string, ok: boolean | undefined, mediaSuccessCount: number): boolean => {
  if (kind !== 'media_proxy') return true
  if (ok === false) return true
  return mediaSuccessCount < 2
}

const PLAYBACK_POSITION_PREFIX = 'dedao-playback-position:'

export const playbackPositionKey = (mediaId: string): string => {
  const id = String(mediaId ?? '').trim()
  return id ? `${PLAYBACK_POSITION_PREFIX}${id}` : ''
}

export const readPlaybackPosition = (store: Storage, mediaId: string): number => {
  const key = playbackPositionKey(mediaId)
  if (!key) return 0
  try {
    const raw = store.getItem(key)
    if (!raw) return 0
    const parsed = JSON.parse(raw) as { seconds?: unknown }
    const seconds = Math.floor(Number(parsed?.seconds))
    if (!Number.isFinite(seconds) || seconds < 5) return 0
    return seconds
  } catch {
    return 0
  }
}

export const writePlaybackPosition = (store: Storage, mediaId: string, seconds: number, duration = 0): void => {
  const key = playbackPositionKey(mediaId)
  if (!key) return
  const value = Math.floor(Number(seconds))
  if (!Number.isFinite(value) || value < 5 || (duration > 0 && value > duration - 5)) {
    store.removeItem(key)
    return
  }
  store.setItem(key, JSON.stringify({ seconds: value }))
}

export const volcDefinitionMap = (): Record<string, { definition: string; definitionText: string }> => {
  return {
    '240p': { definition: '240p', definitionText: '240P' },
    '360p': { definition: '360p', definitionText: '流畅 360P' },
    '480p': { definition: '480p', definitionText: '标清 480P' },
    '540p': { definition: '540p', definitionText: '540P' },
    '720p': { definition: '720p', definitionText: '高清 720P' },
    '1080p': { definition: '1080p', definitionText: '全高清 1080P' },
    '2k': { definition: '2k', definitionText: '2K' },
    '4k': { definition: '4k', definitionText: '4K' },
    original: { definition: 'original', definitionText: '原画' },
  }
}

export const CSS_PIP_CLASS = 'dedao-css-pip'

export const desktopPlayerChromeOptions = (protocol: string) => {
  const wails = protocol === 'wails:'
  return {
    fullscreen: { useCssFullscreen: wails },
    cssFullscreen: true,
    pip: { showIcon: true, preferDocument: false },
  }
}

export const enterCssPictureInPicture = (host: HTMLElement) => {
  host.classList.add(CSS_PIP_CLASS)
}

export const exitCssPictureInPicture = (host: HTMLElement) => {
  host.classList.remove(CSS_PIP_CLASS)
}

export const installDesktopPipAvailability = (doc: Document, protocol: string): (() => void) => {
  if (protocol !== 'wails:') return () => undefined
  const previous = Object.getOwnPropertyDescriptor(doc, 'pictureInPictureEnabled')
  Object.defineProperty(doc, 'pictureInPictureEnabled', {
    configurable: true,
    enumerable: true,
    get: () => true,
  })
  return () => {
    if (previous) Object.defineProperty(doc, 'pictureInPictureEnabled', previous)
    else delete (doc as { pictureInPictureEnabled?: boolean }).pictureInPictureEnabled
  }
}

export const installCssPictureInPictureFallback = (
  video: HTMLVideoElement,
  host: HTMLElement,
): (() => void) => {
  const originalRequest = video.requestPictureInPicture?.bind(video)
  const originalWebkit = (video as HTMLVideoElement & {
    webkitSetPresentationMode?: (mode: string) => void
  }).webkitSetPresentationMode?.bind(video)

  const enter = () => {
    enterCssPictureInPicture(host)
    video.dispatchEvent(new Event('enterpictureinpicture'))
  }
  const leave = () => {
    exitCssPictureInPicture(host)
    video.dispatchEvent(new Event('leavepictureinpicture'))
  }

  video.requestPictureInPicture = async () => {
    if (originalRequest) {
      try {
        return await originalRequest()
      } catch {
        // WKWebView often advertises PiP then rejects the native request.
      }
    }
    enter()
    return { width: host.clientWidth, height: host.clientHeight } as PictureInPictureWindow
  }

  ;(video as HTMLVideoElement & { webkitSetPresentationMode?: (mode: string) => void }).webkitSetPresentationMode = (
    mode: string,
  ) => {
    if (mode === 'picture-in-picture') {
      if (originalWebkit) {
        try {
          originalWebkit(mode)
          return
        } catch {
          enter()
          return
        }
      }
      enter()
      return
    }
    if (originalWebkit) {
      try {
        originalWebkit(mode)
        return
      } catch {
        leave()
        return
      }
    }
    leave()
  }

  const originalExit = document.exitPictureInPicture?.bind(document)
  document.exitPictureInPicture = async () => {
    if (originalExit) {
      try {
        await originalExit()
        return
      } catch {
        // fall through to CSS leave
      }
    }
    leave()
  }

  return () => {
    if (originalRequest) video.requestPictureInPicture = originalRequest
    if (originalWebkit) {
      ;(video as HTMLVideoElement & { webkitSetPresentationMode?: (mode: string) => void }).webkitSetPresentationMode =
        originalWebkit
    }
    if (originalExit) document.exitPictureInPicture = originalExit
    exitCssPictureInPicture(host)
  }
}

export const isDashPluginReady = (globals: { DashPlugin?: unknown }): boolean => {
  return typeof globals.DashPlugin === 'function'
}

export const canAttachDashPlugin = (globals: { Player?: unknown }): boolean => {
  return typeof globals.Player === 'function'
}

export const VEPLAYER_LICENSE_STATUS_OK = 1

type VePlayerLicenseApi = {
  checkLicenseStatus?: (...args: any[]) => any
  checkLicense?: (...args: any[]) => any
  checkLicenseModuleAuth?: (...args: any[]) => any
}

export const installVePlayerLicenseExemption = (): (() => void) => {
  const proto = Object.prototype as Record<string, unknown>
  const previous = Object.getOwnPropertyDescriptor(proto, '_needExemptions')
  Object.defineProperty(proto, '_needExemptions', {
    configurable: true,
    enumerable: false,
    get() {
      return true
    },
    set(this: object) {
      Object.defineProperty(this, '_needExemptions', {
        configurable: true,
        enumerable: true,
        writable: true,
        value: true,
      })
    },
  })
  return () => {
    if (previous) Object.defineProperty(proto, '_needExemptions', previous)
    else delete proto._needExemptions
  }
}

export const installVePlayerSdkLicenseBypass = (vePlayer: VePlayerLicenseApi | null | undefined): (() => void) => {
  if (!vePlayer) return () => undefined
  const originalStatus = vePlayer.checkLicenseStatus
  const originalLicense = vePlayer.checkLicense
  const originalAuth = vePlayer.checkLicenseModuleAuth
  vePlayer.checkLicenseStatus = async () => VEPLAYER_LICENSE_STATUS_OK
  vePlayer.checkLicense = async () => 'premium_edition'
  vePlayer.checkLicenseModuleAuth = async () => true
  return () => {
    vePlayer.checkLicenseStatus = originalStatus
    vePlayer.checkLicense = originalLicense
    vePlayer.checkLicenseModuleAuth = originalAuth
  }
}

export const isVePlayerReadyForPrivateDash = (globals: {
  VePlayer?: unknown
  Player?: unknown
  DashPlugin?: unknown
}): boolean => {
  if (typeof globals.VePlayer !== 'function') return false
  return isDashPluginReady(globals) || canAttachDashPlugin(globals)
}

export const collectPlaybackRuntimeProbe = (env: {
  protocol: string
  isSecureContext: boolean
  hasSubtleCrypto: boolean
  hasMediaSource: boolean
}): PlaybackRuntimeProbe => {
  return {
    protocol: String(env.protocol ?? ''),
    secure_context: Boolean(env.isSecureContext),
    crypto_subtle: Boolean(env.hasSubtleCrypto),
    media_source: Boolean(env.hasMediaSource),
  }
}

export const readPlaybackRuntimeEnv = (win: Window): PlaybackRuntimeProbe => {
  return collectPlaybackRuntimeProbe({
    protocol: win.location.protocol,
    isSecureContext: Boolean(win.isSecureContext),
    hasSubtleCrypto: Boolean(win.crypto && 'subtle' in win.crypto && win.crypto.subtle),
    hasMediaSource: typeof (win as Window & { MediaSource?: unknown }).MediaSource === 'function',
  })
}

export type VolcLicenseResponseSummary = {
  json: boolean
  has_result: boolean
  has_play_auth_list: boolean
  play_auth_count: number
  has_error: boolean
  body_bytes: number
}

export const summarizeVolcLicenseResponse = (body: string): VolcLicenseResponseSummary => {
  const text = String(body ?? '')
  const empty: VolcLicenseResponseSummary = {
    json: false,
    has_result: false,
    has_play_auth_list: false,
    play_auth_count: 0,
    has_error: true,
    body_bytes: text.length,
  }
  try {
    const parsed = JSON.parse(text) as {
      ResponseMetadata?: { Error?: unknown }
      Result?: { PlayAuthInfoList?: unknown[] }
    }
    const list = parsed?.Result?.PlayAuthInfoList
    const error = parsed?.ResponseMetadata?.Error
    return {
      json: true,
      has_result: Boolean(parsed?.Result),
      has_play_auth_list: Array.isArray(list) && list.length > 0,
      play_auth_count: Array.isArray(list) ? list.length : 0,
      has_error: error !== undefined && error !== null,
      body_bytes: text.length,
    }
  } catch {
    return empty
  }
}

export const rewriteVolcRequestUrl = (url: string, protocol: string): string => {
  const raw = String(url ?? '').trim()
  if (!raw || protocol !== 'wails:') return raw

  if (raw.startsWith('//')) {
    const host = hostOfOpaqueUrl(raw.slice(2))
    if (VOLC_API_HOSTS.has(host) || isAllowedMediaHost(host)) return `https:${raw}`
    return raw
  }

  if (raw.startsWith('wails://')) {
    const rest = raw.slice('wails://'.length)
    const host = hostOfOpaqueUrl(rest)
    if (VOLC_API_HOSTS.has(host) || isAllowedMediaHost(host)) return `https://${rest}`
  }

  if (raw.startsWith('http://')) {
    const rest = raw.slice('http://'.length)
    const host = hostOfOpaqueUrl(rest)
    if (VOLC_API_HOSTS.has(host) || isAllowedMediaHost(host)) return `https://${rest}`
  }

  return raw
}

type VolcXhrProto = {
  open: (this: any, method: string, url: string, ...rest: any[]) => any
  send: (this: any, ...args: any[]) => any
  setRequestHeader?: (this: any, name: string, value: string) => any
}

export type MediaProxyResult = {
  status: number
  contentType: string
  contentRange: string
  contentLength: number
  bodyB64: string
}

export type VolcXhrOpenTrace = {
  host: string
  proxied: 'volc' | 'media' | ''
}

const defineXhrValue = (xhr: object, name: string, value: unknown) => {
  try {
    Object.defineProperty(xhr, name, {
      configurable: true,
      enumerable: true,
      writable: true,
      value,
    })
  } catch {
    try {
      ;(xhr as Record<string, unknown>)[name] = value
    } catch {
      // Native XHR fields can be getter-only; ignore if they cannot be shadowed.
    }
  }
}

const applyXhrResponse = (
  xhr: any,
  fields: {
    status: number
    statusText: string
    response: unknown
    responseText: string
    headers: Record<string, string>
  },
) => {
  const headers = Object.fromEntries(
    Object.entries(fields.headers)
      .filter(([, value]) => String(value ?? '').trim() !== '')
      .map(([key, value]) => [key.toLowerCase(), value]),
  )
  defineXhrValue(xhr, 'status', fields.status)
  defineXhrValue(xhr, 'statusText', fields.statusText)
  defineXhrValue(xhr, 'response', fields.response)
  defineXhrValue(xhr, 'responseText', fields.responseText)
  xhr.getResponseHeader = (name: string) => headers[String(name).toLowerCase()] || null
  xhr.getAllResponseHeaders = () =>
    Object.entries(headers)
      .map(([key, value]) => `${key}: ${value}`)
      .join('\r\n')
  defineXhrValue(xhr, 'readyState', 2)
  xhr.onreadystatechange?.()
  defineXhrValue(xhr, 'readyState', 4)
  xhr.onreadystatechange?.()
  xhr.onload?.({ type: 'load', target: xhr })
}

export const normalizeMediaProxyResult = (raw: Partial<MediaProxyResult> & Record<string, unknown>): MediaProxyResult => {
  const bodyB64 = String(raw.bodyB64 ?? raw.BodyB64 ?? '')
  return {
    status: Number(raw.status ?? raw.Status ?? 0),
    contentType: String(raw.contentType ?? raw.ContentType ?? ''),
    contentRange: String(raw.contentRange ?? raw.ContentRange ?? ''),
    contentLength: Number(raw.contentLength ?? raw.ContentLength ?? Math.floor((bodyB64.length * 3) / 4)),
    bodyB64,
  }
}

export const installVolcUrlBridge = (
  xhrProto: VolcXhrProto,
  protocol: string,
  proxyGet?: (query: string) => Promise<string>,
  proxyMedia?: (url: string, rangeHeader: string) => Promise<MediaProxyResult>,
  onOpen?: (trace: VolcXhrOpenTrace) => void,
): (() => void) => {
  if (protocol !== 'wails:') return () => undefined

  const originalOpen = xhrProto.open
  const originalSend = xhrProto.send
  const originalSetHeader = xhrProto.setRequestHeader

  const patchedOpen: VolcXhrProto['open'] = function (this: any, method: string, url: string, ...rest: any[]) {
    const rawUrl = String(url ?? '')
    const host = hostOfRequestUrl(rawUrl)
    const query = extractVolcApiQuery(rawUrl)
    const action = query ? new URLSearchParams(query).get('Action') : ''
    const shouldProxy = Boolean(
      query &&
      String(method).toUpperCase() === 'GET' &&
      proxyGet &&
      (action === 'GetPrivateDrmPlayAuth' || action === 'GetPlayInfo'),
    )
    if (shouldProxy) {
      onOpen?.({ host, proxied: 'volc' })
      this[VOLC_QUERY_KEY] = query
      this[MEDIA_URL_KEY] = ''
      return
    }
    const mediaUrl = extractProxiedMediaUrl(rawUrl, protocol)
    if (mediaUrl && String(method).toUpperCase() === 'GET' && proxyMedia) {
      onOpen?.({ host, proxied: 'media' })
      this[VOLC_QUERY_KEY] = ''
      this[MEDIA_URL_KEY] = mediaUrl
      this[MEDIA_HEADERS_KEY] = {}
      return
    }
    onOpen?.({ host, proxied: '' })
    this[VOLC_QUERY_KEY] = ''
    this[MEDIA_URL_KEY] = ''
    return originalOpen.call(this, method, rewriteVolcRequestUrl(rawUrl, protocol), ...rest)
  }

  const patchedSetHeader = function (this: any, name: string, value: string) {
    if (this[MEDIA_URL_KEY] || this[VOLC_QUERY_KEY]) {
      const headers = (this[MEDIA_HEADERS_KEY] ||= {})
      headers[String(name)] = String(value)
      return
    }
    return originalSetHeader?.call(this, name, value)
  }

  const patchedSend: VolcXhrProto['send'] = function (this: any, body?: unknown) {
    const mediaUrl = String(this[MEDIA_URL_KEY] ?? '')
    if (mediaUrl && proxyMedia) {
      const headers = (this[MEDIA_HEADERS_KEY] || {}) as Record<string, string>
      const rangeHeader = headers.Range || headers.range || ''
      void proxyMedia(mediaUrl, rangeHeader)
        .then((raw) => {
          const result = normalizeMediaProxyResult(raw)
          const binary = Uint8Array.from(atob(result.bodyB64), (ch) => ch.charCodeAt(0))
          const status = result.status || 200
          applyXhrResponse(this, {
            status,
            statusText: status === 206 ? 'Partial Content' : 'OK',
            response: this.responseType === 'arraybuffer' ? binary.buffer : binary.buffer,
            responseText: '',
            headers: {
              'content-type': result.contentType || 'application/octet-stream',
              'content-range': result.contentRange,
              'content-length': String(result.contentLength || binary.byteLength),
            },
          })
        })
        .catch(() => {
          defineXhrValue(this, 'status', 0)
          defineXhrValue(this, 'readyState', 4)
          const onerror = this.onerror as ((ev: unknown) => void) | undefined
          onerror?.(new Error('媒体代理失败'))
        })
      return
    }

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
        applyXhrResponse(this, {
          status: 200,
          statusText: 'OK',
          response: this.responseType === 'json' || this.responseType === '' ? parsed : text,
          responseText: text,
          headers: {
            'content-type': 'application/json',
            'content-length': String(text.length),
          },
        })
      })
      .catch(() => {
        defineXhrValue(this, 'status', 0)
        defineXhrValue(this, 'readyState', 4)
        const onerror = this.onerror as ((ev: unknown) => void) | undefined
        onerror?.(new Error('火山点播代理失败'))
      })
  }

  xhrProto.open = patchedOpen
  xhrProto.send = patchedSend
  xhrProto.setRequestHeader = patchedSetHeader

  return () => {
    if (xhrProto.open === patchedOpen) xhrProto.open = originalOpen
    if (xhrProto.send === patchedSend) xhrProto.send = originalSend
    if (xhrProto.setRequestHeader === patchedSetHeader) xhrProto.setRequestHeader = originalSetHeader
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
