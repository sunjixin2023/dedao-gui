import assert from 'node:assert/strict'
import test from 'node:test'

import {
  buildPrivateDrmAuthQuery,
  buildSafePlaybackDebugInfo,
  collectPlaybackRuntimeProbe,
  createV2PlayInfoResolver,
  getV2SignedPlayInfoQuery,
  installVolcCookieBridge,
  installVolcUrlBridge,
  isV2PlayAuthToken,
  pickVolcPlaybackAuth,
  extractVolcApiQuery,
  rewriteVolcRequestUrl,
  summarizeVolcLicenseResponse,
  canAttachDashPlugin,
  isDashPluginReady,
  privateDashPluginOptions,
  desktopPlayerChromeOptions,
  volcDefinitionMap,
  enterCssPictureInPicture,
  exitCssPictureInPicture,
  installDesktopPipAvailability,
  desktopPlaybackRates,
  shouldRecordPlaybackProbe,
  playbackPositionKey,
  readPlaybackPosition,
  writePlaybackPosition,
  isVePlayerReadyForPrivateDash,
  installVePlayerSdkLicenseBypass,
  installVePlayerLicenseExemption,
  isAllowedMediaHost,
  extractProxiedMediaUrl,
} from './volc.ts'

test('picks the play and private DRM credentials from the same format', () => {
  const got = pickVolcPlaybackAuth({
    tracks: [
      {
        formats: [
          {
            volc_id: 'vid-1',
            volc_play_auth_token: 'play-token',
            volc_key_token: 'key-token',
            format: 'volc-drm-dash',
          },
        ],
      },
    ],
  })

  assert.deepEqual(got, {
    playAuthToken: 'play-token',
    keyToken: 'key-token',
    vid: 'vid-1',
    format: 'volc-drm-dash',
  })
})

test('prefers a format that includes a private DRM key token over a play-only format', () => {
  const got = pickVolcPlaybackAuth({
    tracks: [
      {
        formats: [
          {
            volc_id: 'vid-plain',
            volc_play_auth_token: 'play-plain',
            format: 'dash',
          },
          {
            volc_id: 'vid-drm',
            volc_play_auth_token: 'play-drm',
            volc_key_token: 'key-drm',
            format: 'volc-drm-dash',
          },
        ],
      },
    ],
  })

  assert.deepEqual(got, {
    playAuthToken: 'play-drm',
    keyToken: 'key-drm',
    vid: 'vid-drm',
    format: 'volc-drm-dash',
  })
})

test('recognizes V2 wrappers without exposing their signed query', () => {
  const token = Buffer.from(JSON.stringify({
    TokenVersion: 'V2',
    GetPlayInfoToken: 'Action=GetPlayInfo&Version=2020-08-01&Vid=vid-1',
  })).toString('base64')

  assert.equal(isV2PlayAuthToken(token), true)
  assert.equal(isV2PlayAuthToken('not-base64'), false)
})

test('extracts the exact V2 query for backend resolution', () => {
  const signedQuery = 'Action=GetPlayInfo&Version=2020-08-01&Vid=vid-1&NeedThumbs=0&X-Signature=signed'
  const token = Buffer.from(JSON.stringify({
    TokenVersion: 'V2',
    GetPlayInfoToken: signedQuery,
  })).toString('base64')

  assert.equal(getV2SignedPlayInfoQuery(token), signedQuery)
})

test('resolves V2 play info through the local backend promise', async () => {
  const token = Buffer.from(JSON.stringify({
    TokenVersion: 'V2',
    GetPlayInfoToken: 'Action=GetPlayInfo&Version=2020-08-01&Vid=vid-1',
  })).toString('base64')
  const fallbackCalls: unknown[][] = []
  const fallback = (...args: unknown[]) => {
    fallbackCalls.push(args)
    return 'https://vod.example.invalid/fallback'
  }
  const resolvePlayInfo = createV2PlayInfoResolver(
    async () => ({ Status: 10, Vid: 'vid-1' }),
    fallback,
  )

  assert.deepEqual(await resolvePlayInfo(token, 'unused'), { Status: 10, Vid: 'vid-1' })
  assert.equal(resolvePlayInfo('legacy-token', 'vod.example.invalid'), 'https://vod.example.invalid/fallback')
  assert.deepEqual(fallbackCalls, [['legacy-token', 'vod.example.invalid']])
})

test('keeps the private DRM union cookie readable on the Wails scheme', () => {
  const cookieJar = { cookie: 'theme=dark' }
  const restore = installVolcCookieBridge(cookieJar, 'wails:')

  cookieJar.cookie = 'volcvui=union-info'

  assert.match(cookieJar.cookie, /theme=dark/)
  assert.match(cookieJar.cookie, /volcvui=union-info/)
  restore()
  assert.equal(cookieJar.cookie, 'theme=dark')
})

test('does not replace native cookie behavior on HTTP origins', () => {
  const cookieJar = { cookie: 'theme=dark' }
  const restore = installVolcCookieBridge(cookieJar, 'https:')

  cookieJar.cookie = 'volcvui=native'

  assert.equal(cookieJar.cookie, 'volcvui=native')
  restore()
})

test('rewrites protocol-relative Volc license URLs to https', () => {
  assert.equal(
    rewriteVolcRequestUrl('//vod.volcengineapi.com/?Action=GetPrivateDrmPlayAuth&Vid=v1', 'wails:'),
    'https://vod.volcengineapi.com/?Action=GetPrivateDrmPlayAuth&Vid=v1',
  )
})

test('rewrites wails-scheme Volc license URLs to https', () => {
  assert.equal(
    rewriteVolcRequestUrl('wails://vod.volcengineapi.com/?Action=GetPrivateDrmPlayAuth', 'wails:'),
    'https://vod.volcengineapi.com/?Action=GetPrivateDrmPlayAuth',
  )
})

test('allows Dedao and Volc media hosts and rejects arbitrary hosts', () => {
  assert.equal(isAllowedMediaHost('bd-vod.umiwi.com'), true)
  assert.equal(isAllowedMediaHost('vod.umiwi.com'), true)
  assert.equal(isAllowedMediaHost('vod.volces.com'), true)
  assert.equal(isAllowedMediaHost('cdn.volccdn.com'), true)
  assert.equal(isAllowedMediaHost('evil.example'), false)
  assert.equal(isAllowedMediaHost('umiwi.com.evil.example'), false)
  assert.equal(isAllowedMediaHost('notumiwi.com'), false)
})

test('rewrites wails-scheme media URLs to https for the local proxy', () => {
  assert.equal(
    extractProxiedMediaUrl('wails://bd-vod.umiwi.com/path/seg.m4s?sig=1', 'wails:'),
    'https://bd-vod.umiwi.com/path/seg.m4s?sig=1',
  )
  assert.equal(
    extractProxiedMediaUrl('//bd-vod.umiwi.com/path/file.mpd', 'wails:'),
    'https://bd-vod.umiwi.com/path/file.mpd',
  )
  assert.equal(
    extractProxiedMediaUrl('http://bd-vod.umiwi.com/path/file.mpd', 'wails:'),
    'https://bd-vod.umiwi.com/path/file.mpd',
  )
  assert.equal(extractProxiedMediaUrl('https://evil.example/x', 'wails:'), '')
})

test('leaves unrelated URLs unchanged', () => {
  assert.equal(rewriteVolcRequestUrl('https://cdn.example/video.mpd', 'wails:'), 'https://cdn.example/video.mpd')
  assert.equal(rewriteVolcRequestUrl('//cdn.example/video.mpd', 'wails:'), '//cdn.example/video.mpd')
})

test('extracts the signed Volc query from protocol-relative license URLs', () => {
  assert.equal(
    extractVolcApiQuery('//vod.volcengineapi.com/?Action=GetPrivateDrmPlayAuth&Vid=v1'),
    'Action=GetPrivateDrmPlayAuth&Vid=v1',
  )
  assert.equal(
    extractVolcApiQuery('https://vod.volcengineapi.com?Action=GetPrivateDrmPlayAuth&Vid=v1'),
    'Action=GetPrivateDrmPlayAuth&Vid=v1',
  )
  assert.equal(extractVolcApiQuery('https://cdn.example/video.mpd'), '')
})

test('proxies Volc license XHR through the local backend', async () => {
  const xhr = {
    open(_method: string, _url: string) {},
    send(_body?: unknown) {},
    onload: undefined as ((ev: { target: unknown }) => void) | undefined,
    status: 0,
    readyState: 0,
    responseType: 'json',
    response: null as unknown,
    responseText: '',
  }
  const proto = {
    open(this: typeof xhr, method: string, url: string) {
      return Object.getPrototypeOf(xhr).open.call(this, method, url)
    },
    send(this: typeof xhr, body?: unknown) {
      return Object.getPrototypeOf(xhr).send.call(this, body)
    },
  }
  Object.setPrototypeOf(xhr, proto)
  const restore = installVolcUrlBridge(proto, 'wails:', async (query) => {
    assert.equal(query, 'Action=GetPrivateDrmPlayAuth&Vid=v1')
    return '{"Result":{"PlayAuthInfoList":[{"PlayAuthContent":"ok"}]}}'
  })

  await new Promise<void>((resolve, reject) => {
    xhr.onload = () => {
      try {
        assert.equal(xhr.status, 200)
        assert.equal((xhr.response as { Result?: { PlayAuthInfoList?: Array<{ PlayAuthContent?: string }> } }).Result?.PlayAuthInfoList?.[0]?.PlayAuthContent, 'ok')
        resolve()
      } catch (err) {
        reject(err)
      }
    }
    proto.open.call(xhr, 'GET', '//vod.volcengineapi.com/?Action=GetPrivateDrmPlayAuth&Vid=v1')
    proto.send.call(xhr)
  })
  restore()
})

test('proxies Volc license XHR when onload is assigned after send', async () => {
  const xhr = {
    open(_method: string, _url: string) {},
    send(_body?: unknown) {},
    onload: undefined as ((ev: { target: unknown }) => void) | undefined,
    status: 0,
    readyState: 0,
    responseType: 'json' as XMLHttpRequestResponseType,
    response: null as unknown,
    responseText: '',
    getResponseHeader(_name: string) {
      return null as string | null
    },
  }
  const proto = {
    open(this: typeof xhr, method: string, url: string) {
      return Object.getPrototypeOf(xhr).open.call(this, method, url)
    },
    send(this: typeof xhr, body?: unknown) {
      return Object.getPrototypeOf(xhr).send.call(this, body)
    },
  }
  Object.setPrototypeOf(xhr, proto)
  const restore = installVolcUrlBridge(proto, 'wails:', async (query) => {
    assert.equal(query, 'Action=GetPrivateDrmPlayAuth&Vid=v1')
    return '{"Result":{"PlayAuthInfoList":[{"PlayAuthContent":"ok"}]}}'
  })

  await new Promise<void>((resolve, reject) => {
    proto.open.call(xhr, 'GET', 'https://vod.volcengineapi.com?Action=GetPrivateDrmPlayAuth&Vid=v1')
    xhr.responseType = 'json'
    proto.send.call(xhr)
    xhr.onload = () => {
      try {
        assert.equal(xhr.status, 200)
        assert.equal(xhr.getResponseHeader('content-type'), 'application/json')
        assert.equal((xhr.response as { Result?: { PlayAuthInfoList?: Array<{ PlayAuthContent?: string }> } }).Result?.PlayAuthInfoList?.[0]?.PlayAuthContent, 'ok')
        resolve()
      } catch (err) {
        reject(err)
      }
    }
  })
  restore()
})

test('appends unsigned private DRM runtime values without rewriting the signed prefix', () => {
  const signed = 'Action=GetPrivateDrmPlayAuth&Version=2020-08-01&Vid=video-1&X-SignedQueries=Action%3BVersion%3BVid&X-Signature=signed'
  const got = buildPrivateDrmAuthQuery(signed, 'auth-a,auth-b', 'video-1', 'union value')
  assert.equal(got.startsWith(signed + '&'), true)
  const values = new URLSearchParams(got)
  assert.equal(values.get('DrmType'), 'webdevice')
  assert.equal(values.get('PlayAuthIds'), 'auth-a,auth-b')
  assert.equal(values.get('UnionInfo'), 'union value')
})

test('rejects a private DRM query whose action or vid does not match', () => {
  assert.throws(
    () => buildPrivateDrmAuthQuery('Action=DeleteSpace&Version=2020-08-01&Vid=video-1', 'auth', 'video-1', 'union'),
    /Action/,
  )
  assert.throws(
    () => buildPrivateDrmAuthQuery('Action=GetPrivateDrmPlayAuth&Version=2020-08-01&Vid=other', 'auth', 'video-1', 'union'),
    /Vid/,
  )
})

test('definition map covers common Volc rungs including 1080p and original', () => {
  const got = volcDefinitionMap()
  assert.equal(got['720p']?.definitionText.includes('720'), true)
  assert.equal(got['1080p']?.definition, '1080p')
  assert.equal(got.original?.definitionText.includes('原画'), true)
  assert.equal(got['360p']?.definition, '360p')
})

test('private DASH playback disables EME so the AES key is injected', () => {
  const got = privateDashPluginOptions()
  assert.equal(got.useEME, false)
  assert.equal(got.useUnionInfoDRM, true)
})

test('Wails desktop chrome uses CSS fullscreen and shows a PiP control', () => {
  const got = desktopPlayerChromeOptions('wails:')
  assert.equal(got.fullscreen.useCssFullscreen, true)
  assert.equal(got.cssFullscreen, true)
  assert.equal(got.pip.showIcon, true)
  assert.equal(got.pip.preferDocument, false)
})

test('HTTPS origins keep native fullscreen instead of forcing CSS fullscreen', () => {
  const got = desktopPlayerChromeOptions('https:')
  assert.equal(got.fullscreen.useCssFullscreen, false)
})

test('desktop PiP availability reports enabled on the Wails scheme', () => {
  const doc = { pictureInPictureEnabled: false } as Document & { pictureInPictureEnabled: boolean }
  const restore = installDesktopPipAvailability(doc, 'wails:')
  assert.equal(doc.pictureInPictureEnabled, true)
  restore()
})

test('CSS picture-in-picture toggles a host class without copying media bytes', () => {
  const host = { className: 'veplayer-container' } as unknown as HTMLElement
  const classList = {
    tokens: new Set<string>(),
    add(name: string) { this.tokens.add(name) },
    remove(name: string) { this.tokens.delete(name) },
    contains(name: string) { return this.tokens.has(name) },
  }
  Object.defineProperty(host, 'classList', { value: classList })
  enterCssPictureInPicture(host)
  assert.equal(classList.contains('dedao-css-pip'), true)
  exitCssPictureInPicture(host)
  assert.equal(classList.contains('dedao-css-pip'), false)
})

test('DASH plugin is ready only when DashPlugin is a constructor', () => {
  assert.equal(isDashPluginReady({ DashPlugin: function DashPlugin() {} }), true)
  assert.equal(isDashPluginReady({}), false)
  assert.equal(isDashPluginReady({ DashPlugin: 'plugin' }), false)
})

test('DASH plugin attach requires the VePlayer core Player global', () => {
  assert.equal(canAttachDashPlugin({ Player: function Player() {} }), true)
  assert.equal(canAttachDashPlugin({}), false)
})

test('SDK license bypass reports a valid premium edition without a domain license', async () => {
  const ve = {
    checkLicenseStatus: async () => 0,
    checkLicense: async () => 'none',
    checkLicenseModuleAuth: async () => false,
  }
  const restore = installVePlayerSdkLicenseBypass(ve)
  assert.equal(await ve.checkLicenseStatus(), 1)
  assert.equal(await ve.checkLicense(), 'premium_edition')
  assert.equal(await ve.checkLicenseModuleAuth(), true)
  restore()
  assert.equal(await ve.checkLicenseStatus(), 0)
})

test('license exemption forces _needExemptions without polluting _Module', () => {
  const restore = installVePlayerLicenseExemption()
  const host: { _needExemptions?: boolean; _Module?: string } = {}
  host._needExemptions = false
  assert.equal(host._needExemptions, true)
  assert.equal('_Module' in host, false)
  host._Module = 'dash-wasm'
  assert.equal(host._Module, 'dash-wasm')
  restore()
  const after: { _needExemptions?: boolean } = {}
  after._needExemptions = false
  assert.equal(after._needExemptions, false)
})

test('license proxy fills native-like read-only XHR status and response', async () => {
  const xhr: {
    open: (method: string, url: string) => void
    send: () => void
    onload?: (ev: { target: unknown }) => void
    responseType: XMLHttpRequestResponseType
    getResponseHeader?: (name: string) => string | null
  } = {
    open() {},
    send() {},
    responseType: 'json',
  }
  Object.defineProperty(xhr, 'readyState', { configurable: true, enumerable: true, get: () => 0 })
  Object.defineProperty(xhr, 'status', { configurable: true, enumerable: true, get: () => 0 })
  Object.defineProperty(xhr, 'response', { configurable: true, enumerable: true, get: () => null })
  const proto = {
    open(this: typeof xhr, method: string, url: string) {
      return Object.getPrototypeOf(xhr).open.call(this, method, url)
    },
    send(this: typeof xhr) {
      return Object.getPrototypeOf(xhr).send.call(this)
    },
  }
  Object.setPrototypeOf(xhr, proto)
  const restore = installVolcUrlBridge(proto, 'wails:', async () => {
    return '{"Result":{"PlayAuthInfoList":[{"PlayAuthContent":"ok"}]}}'
  })

  await new Promise<void>((resolve, reject) => {
    assert.doesNotThrow(() => {
      proto.open.call(xhr, 'GET', 'https://vod.volcengineapi.com?Action=GetPrivateDrmPlayAuth&Vid=v1')
    })
    proto.send.call(xhr)
    xhr.onload = () => {
      try {
        assert.equal((xhr as { status: number }).status, 200)
        assert.equal((xhr as { readyState: number }).readyState, 4)
        assert.equal(
          (xhr as { response: { Result?: { PlayAuthInfoList?: Array<{ PlayAuthContent?: string }> } } }).response?.Result?.PlayAuthInfoList?.[0]?.PlayAuthContent,
          'ok',
        )
        resolve()
      } catch (err) {
        reject(err)
      }
    }
  })
  restore()
})

test('media proxy XHR exposes content-length and content-range via getAllResponseHeaders', async () => {
  const xhr: {
    open: () => void
    send: () => void
    onload?: (ev: { target: unknown }) => void
    responseType: XMLHttpRequestResponseType
    getResponseHeader?: (name: string) => string | null
    getAllResponseHeaders?: () => string
  } = {
    open() {},
    send() {},
    responseType: 'arraybuffer',
  }
  const proto = {
    open() {},
    send() {},
  }
  Object.setPrototypeOf(xhr, proto)
  const restore = installVolcUrlBridge(
    proto,
    'wails:',
    undefined,
    async () => ({
      status: 206,
      contentType: 'video/mp4',
      contentRange: 'bytes 0-3/8',
      contentLength: 4,
      bodyB64: btoa('abcd'),
    }),
  )

  await new Promise<void>((resolve, reject) => {
    proto.open.call(xhr, 'GET', 'https://bd-vod.umiwi.com/seg.m4s')
    proto.send.call(xhr)
    xhr.onload = () => {
      try {
        const headers = String(xhr.getAllResponseHeaders?.() ?? '')
        assert.match(headers, /content-range: bytes 0-3\/8/i)
        assert.match(headers, /content-length: 4/i)
        assert.equal(xhr.getResponseHeader?.('content-length'), '4')
        resolve()
      } catch (err) {
        reject(err)
      }
    }
  })
  restore()
})

test('XHR bridge reports opened hosts without copying the signed URL', () => {
  const opened: string[] = []
  const proto = {
    open(_method: string, url: string) {
      opened.push(url)
    },
    send(_body?: unknown) {},
  }
  const traces: Array<{ host: string; proxied: string }> = []
  const restore = installVolcUrlBridge(
    proto,
    'wails:',
    undefined,
    async () => ({ status: 206, contentType: 'video/mp4', contentRange: 'bytes 0-3/8', bodyB64: btoa('abcd') }),
    (trace) => traces.push({ host: trace.host, proxied: trace.proxied }),
  )

  proto.open('GET', 'https://bd-vod.umiwi.com/seg.m4s?sig=secret-token')
  proto.open('GET', 'https://tracker.example/pixel?sig=secret-token')
  restore()

  assert.deepEqual(
    traces.map((item) => item.host),
    ['bd-vod.umiwi.com', 'tracker.example'],
  )
  assert.equal(traces[0]?.proxied, 'media')
  assert.equal(traces[1]?.proxied, '')
  assert.deepEqual(opened, ['https://tracker.example/pixel?sig=secret-token'])
  assert.equal(JSON.stringify(traces).includes('secret-token'), false)
})

test('private DASH requires VePlayer plus Player or an already loaded DashPlugin', () => {
  assert.equal(isVePlayerReadyForPrivateDash({ VePlayer: function VePlayer() {} }), false)
  assert.equal(
    isVePlayerReadyForPrivateDash({ VePlayer: function VePlayer() {}, Player: function Player() {} }),
    true,
  )
  assert.equal(
    isVePlayerReadyForPrivateDash({ VePlayer: function VePlayer() {}, DashPlugin: function DashPlugin() {} }),
    true,
  )
})

test('playback runtime probe records capability flags without player secrets', () => {
  const probe = collectPlaybackRuntimeProbe({
    protocol: 'wails:',
    isSecureContext: false,
    hasSubtleCrypto: false,
    hasMediaSource: true,
  })
  assert.deepEqual(probe, {
    protocol: 'wails:',
    secure_context: false,
    crypto_subtle: false,
    media_source: true,
  })
})

test('license response summary reports auth presence without copying PlayAuthContent', () => {
  const summary = summarizeVolcLicenseResponse('{"Result":{"PlayAuthInfoList":[{"PlayAuthContent":"secret-key"}]}}')
  assert.equal(summary.json, true)
  assert.equal(summary.has_result, true)
  assert.equal(summary.has_play_auth_list, true)
  assert.equal(summary.play_auth_count, 1)
  assert.equal(summary.has_error, false)
  assert.doesNotMatch(JSON.stringify(summary), /secret-key/)
})

test('desktop playback rates include half-speed through 2x', () => {
  assert.deepEqual(desktopPlaybackRates(), [0.5, 0.75, 1, 1.25, 1.5, 2])
})

test('routine successful media proxy probes are sampled after the first two', () => {
  assert.equal(shouldRecordPlaybackProbe('play_info', true, 99), true)
  assert.equal(shouldRecordPlaybackProbe('media_proxy', false, 99), true)
  assert.equal(shouldRecordPlaybackProbe('media_proxy', true, 0), true)
  assert.equal(shouldRecordPlaybackProbe('media_proxy', true, 1), true)
  assert.equal(shouldRecordPlaybackProbe('media_proxy', true, 2), false)
})

test('playback position is stored by media id without copying tokens', () => {
  const store = new Map<string, string>()
  const memory = {
    getItem: (key: string) => store.get(key) ?? null,
    setItem: (key: string, value: string) => { store.set(key, value) },
    removeItem: (key: string) => { store.delete(key) },
  } as Storage

  assert.equal(playbackPositionKey('media-1'), 'dedao-playback-position:media-1')
  assert.equal(readPlaybackPosition(memory, 'media-1'), 0)
  writePlaybackPosition(memory, 'media-1', 87, 600)
  assert.equal(readPlaybackPosition(memory, 'media-1'), 87)
  writePlaybackPosition(memory, 'media-1', 598, 600)
  assert.equal(readPlaybackPosition(memory, 'media-1'), 0)
  assert.equal(JSON.stringify([...store.values()]).includes('token'), false)
})

test('debug info records credential presence without copying secrets', () => {
  const debugInfo = buildSafePlaybackDebugInfo({
    title: 'Video',
    runtime: 'desktop',
    playMode: 'token',
    streamUrl: 'https://cdn.example/video?signature=secret-url',
    mediaId: 'media-1',
    securityToken: 'secret-token',
    tokenSource: 'wails_backend',
    lineAppId: 233260,
    status: 'error',
    error: 'network',
  })

  assert.doesNotMatch(debugInfo, /secret-url|secret-token/)
  assert.match(debugInfo, /stream_url_present=true/)
  assert.match(debugInfo, /security_token_present=true/)
  assert.match(debugInfo, /security_token_length=12/)
})
