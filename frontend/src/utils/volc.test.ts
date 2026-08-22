import assert from 'node:assert/strict'
import test from 'node:test'

import {
  buildSafePlaybackDebugInfo,
  createV2PlayInfoResolver,
  getV2SignedPlayInfoQuery,
  installVolcCookieBridge,
  installVolcUrlBridge,
  isV2PlayAuthToken,
  pickVolcPlaybackAuth,
  rewriteVolcRequestUrl,
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

test('leaves unrelated URLs unchanged', () => {
  assert.equal(rewriteVolcRequestUrl('https://cdn.example/video.mpd', 'wails:'), 'https://cdn.example/video.mpd')
  assert.equal(rewriteVolcRequestUrl('//cdn.example/video.mpd', 'wails:'), '//cdn.example/video.mpd')
})

test('rewrites XHR open URLs on the Wails scheme', () => {
  const opened: string[] = []
  const proto = {
    open(method: string, url: string) {
      opened.push(`${method} ${url}`)
    },
  }
  const restore = installVolcUrlBridge(proto, 'wails:')
  proto.open('GET', '//vod.volcengineapi.com/?Action=GetPrivateDrmPlayAuth')
  restore()
  assert.deepEqual(opened, ['GET https://vod.volcengineapi.com/?Action=GetPrivateDrmPlayAuth'])
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
