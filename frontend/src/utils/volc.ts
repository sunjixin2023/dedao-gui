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
