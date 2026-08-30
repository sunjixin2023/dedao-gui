<template>
  <div class="veplayer-page">
    <section class="player-hero">
      <div class="hero-main">
        <p class="hero-kicker">Video Workspace</p>
        <h1 class="hero-title" :title="title">{{ title }}</h1>
        <p class="hero-subtitle">
          沉浸观看模式已启用，支持自动续签播放 token 与快速重试。
        </p>

        <div class="hero-actions">
          <el-button type="primary" round @click="goBack">返回上一页</el-button>
          <el-button round :loading="loading" :icon="RefreshRight" @click="reload">重新加载</el-button>
          <el-button round :icon="DocumentCopy" @click="copyDebugInfo">复制调试信息</el-button>
          <el-select
            v-if="showQualitySelector"
            v-model="selectedQuality"
            class="quality-select"
            :disabled="loading || qualitySwitching || qualityOptions.length <= 1"
            placeholder="清晰度"
          >
            <el-option
              v-for="item in qualityOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </div>
      </div>

      <div class="hero-stats">
        <article class="stat-card">
          <span>播放状态</span>
          <strong>{{ statusText }}</strong>
        </article>
        <article class="stat-card">
          <span>line_app_id</span>
          <strong>{{ lineAppId }}</strong>
        </article>
        <article class="stat-card">
          <span>media_id</span>
          <strong :title="mediaId">{{ mediaId ? trimToken(mediaId) : "未提供" }}</strong>
        </article>
        <article class="stat-card">
          <span>{{ hasDirectStream ? 'stream_url' : 'security_token' }}</span>
          <strong :title="hasDirectStream ? effectiveStreamUrl : securityToken">
            {{ hasDirectStream ? trimToken(effectiveStreamUrl) : (securityToken ? trimToken(securityToken) : "未提供") }}
          </strong>
        </article>
      </div>
    </section>

    <section class="player-workspace">
      <div class="player-stage">
        <video
          v-if="shouldUseNativeVideo"
          ref="nativeVideoRef"
          class="native-video video-js vjs-default-skin vjs-big-play-centered"
          :src="effectiveStreamUrl"
          controls
          autoplay
          playsinline
          preload="auto"
          @error="onNativeVideoError"
          @loadeddata="onNativeVideoLoaded"
        />
        <div v-else ref="playerRoot" id="veplayer" class="veplayer-container"></div>

        <div v-if="loading" class="veplayer-loading">
          <el-skeleton :rows="3" animated style="width: 360px" />
        </div>

        <div v-if="missingParamsText" class="veplayer-empty">
          <el-result icon="warning" title="缺少播放参数" :sub-title="missingParamsText">
            <template #extra>
              <el-button type="primary" @click="goBack">返回重新选择</el-button>
            </template>
          </el-result>
        </div>

        <div v-else-if="errorText" class="veplayer-error">
          <el-result icon="error" title="播放失败" :sub-title="errorText">
            <template #extra>
              <el-button type="primary" @click="reload">重试</el-button>
            </template>
          </el-result>
        </div>
      </div>

      <aside class="player-sidebar">
        <h3>播放提示</h3>
        <ul>
          <li>当前运行环境：{{ runtimeModeText }}。</li>
          <li>优先确保当前账号有该视频的观看权限。</li>
          <li>若播放失败，可先点击“重新加载”。</li>
          <li>若仍失败，请复制调试信息用于排查。</li>
        </ul>

        <div class="sidebar-status">
          <span>最近错误</span>
          <p>{{ errorText || "暂无" }}</p>
        </div>
      </aside>
    </section>
  </div>
</template>

<script lang="ts" setup>
import { computed, nextTick, onUnmounted, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { DocumentCopy, RefreshRight } from '@element-plus/icons-vue'
import videojs from 'video.js'
import 'video.js/dist/video-js.css'
import { useRoute, useRouter } from 'vue-router'
import { userStore } from '../stores/user'
import { hasBackendBridge, invokeBackend } from '../utils/backend'

type VePlayerCtor = new (options: Record<string, any>) => {
  dispose?: () => void
  on?: (event: string, cb: (...args: any[]) => void) => void
}

type MediaVolcLike = {
  volc_play_auth_token?: string
  play_auth_token?: string
  tracks?: Array<{
    formats?: Array<{
      volc_id?: string
      volc_play_auth_token?: string
      volc_key_token?: string
    }>
  }>
}

type VodPlayInfoLike = {
  MainPlayUrl?: string
  BackupPlayUrl?: string
  Format?: string
  FileType?: string
}

type VodPlayInfoRespLike = {
  Result?: {
    AdaptiveInfo?: {
      MainPlayUrl?: string
      BackupPlayUrl?: string
    }
    PlayInfoList?: VodPlayInfoLike[]
  }
}

type ResolvedPlaybackLike = {
  play_auth_token?: string
  stream_url?: string
  vid?: string
  key_token?: string
}

type MediaWebLike = {
  tracks?: Array<{
    formats?: Array<{
      url?: string
      drm_version?: number
      rates_kbps?: number
      resolution?: unknown
      tag?: string
      format?: string
      type?: string
    }>
  }>
}

type PlaybackTokenCandidate = {
  playAuthToken: string
  keyToken?: string
  vid?: string
  source: 'wails_backend' | 'route_query' | 'route_query_fallback'
}

const route = useRoute()
const router = useRouter()
const store = userStore()

const playerRoot = ref<HTMLDivElement | null>(null)
const playerSdk = ref<InstanceType<VePlayerCtor> | null>(null)
const nativeVideoRef = ref<HTMLVideoElement | null>(null)
let directVideoPlayer: any = null
const loading = ref(false)
const errorText = ref('')
const resolvedStreamUrl = ref('')
const directUrlError = ref('')
const directProbeState = ref('idle')
const forceTokenMode = ref(false)
const tokenSourceState = ref('unknown')
const sdkSourceState = ref('unknown')
const playerInitModeState = ref('idle')
const tokenCandidates = ref<PlaybackTokenCandidate[]>([])
const activeTokenIndex = ref(0)
const reloadVersion = ref(0)
type DirectQualityOption = {
  label: string
  value: string
  height: number
  bandwidth: number
  mode: 'auto' | 'vhs' | 'direct'
  streamUrl?: string
}
const vhsQualityOptions = ref<DirectQualityOption[]>([])
const manualQualityOptions = ref<DirectQualityOption[]>([])
const manualAutoStreamUrl = ref('')
const qualityOptions = computed(() => {
  const auto: DirectQualityOption = { label: '自动', value: 'auto', height: 0, bandwidth: 0, mode: 'auto' }
  if (vhsQualityOptions.value.length > 1) {
    return [auto, ...vhsQualityOptions.value]
  }
  if (manualQualityOptions.value.length > 1) {
    return [auto, ...manualQualityOptions.value]
  }
  return [auto]
})
const selectedQuality = ref('auto')
let directRepresentations: Array<{ value: string; rep: any }> = []
const qualitySwitching = ref(false)

const resetPlayerQualityOptions = () => {
  vhsQualityOptions.value = []
  directRepresentations = []
}

const resetAllQualityOptions = () => {
  resetPlayerQualityOptions()
  manualQualityOptions.value = []
  manualAutoStreamUrl.value = ''
  selectedQuality.value = 'auto'
}

const getRepresentationKey = (rep: any, index: number) => {
  const id = String(rep?.id ?? '').trim()
  if (id) return id
  const height = Number(rep?.height ?? 0)
  const bandwidth = Number(rep?.bandwidth ?? 0)
  return `${height || 0}-${bandwidth || 0}-${index}`
}

const buildQualityLabel = (height: number, bandwidth: number, index: number) => {
  if (height > 0) return `${height}p`
  if (bandwidth > 0) {
    const mbps = bandwidth / 1_000_000
    const fixed = mbps >= 10 ? 0 : 1
    return `${mbps.toFixed(fixed)} Mbps`
  }
  return `线路 ${index + 1}`
}

const getRepresentationsFromPlayer = (player: any) => {
  const tech = player?.tech?.(true) ?? player?.tech?.() ?? player?.tech_
  const fromVhs = tech?.vhs?.representations
  if (typeof fromVhs === 'function') {
    try {
      const reps = fromVhs.call(tech.vhs)
      if (Array.isArray(reps)) return reps
    } catch {
    }
  }
  const fromHls = tech?.hls?.representations
  if (typeof fromHls === 'function') {
    try {
      const reps = fromHls.call(tech.hls)
      if (Array.isArray(reps)) return reps
    } catch {
    }
  }
  return []
}

const hasRenderableVideoFrame = (player: any) => {
  const width = Number(player?.videoWidth?.() ?? 0)
  const height = Number(player?.videoHeight?.() ?? 0)
  return width > 0 && height > 0
}

const isDirectPlayerReady = (player: any) => {
  if (!player) return false
  try {
    if (player?.error?.()) return false
  } catch {
  }
  if (hasRenderableVideoFrame(player)) return true
  const state = Number(player?.readyState?.() ?? 0)
  return Number.isFinite(state) && state >= 3
}

const waitDirectPlayerSwitchReady = (player: any, timeoutMs = 9000) => {
  return new Promise<boolean>((resolve) => {
    let settled = false
    let cleanup: (() => void) | null = null
    const delayedChecks: number[] = []
    const finish = (ok: boolean) => {
      if (settled) return
      settled = true
      cleanup?.()
      resolve(ok)
    }

    const timer = window.setTimeout(() => finish(false), timeoutMs)
    const okEvents = ['loadeddata', 'canplay', 'playing']
    const handlers: Array<{ event: string; fn: () => void }> = []
    const probeReady = () => {
      if (isDirectPlayerReady(player)) {
        finish(true)
      }
    }
    const scheduleProbeReady = () => {
      if (settled) return
      probeReady()
      const offsets = [120, 320, 720]
      for (const offset of offsets) {
        const id = window.setTimeout(() => {
          probeReady()
        }, offset)
        delayedChecks.push(id)
      }
    }
    for (const event of okEvents) {
      const fn = () => scheduleProbeReady()
      handlers.push({ event, fn })
      try {
        player?.on?.(event, fn)
      } catch {
      }
    }
    const onError = () => finish(false)
    try {
      player?.on?.('error', onError)
    } catch {
    }

    cleanup = () => {
      window.clearTimeout(timer)
      for (const id of delayedChecks) {
        window.clearTimeout(id)
      }
      for (const item of handlers) {
        try {
          player?.off?.(item.event, item.fn)
        } catch {
        }
      }
      try {
        player?.off?.('error', onError)
      } catch {
      }
      cleanup = null
    }
  })
}

const switchDirectPlaybackQuality = async (targetUrl: string) => {
  const normalized = normalizeStreamUrl(targetUrl)
  if (!normalized) return

  const current = normalizeStreamUrl(effectiveStreamUrl.value)
  if (current && current === normalized) return
  if (qualitySwitching.value) return

  qualitySwitching.value = true
  const previousResolved = resolvedStreamUrl.value
  const previousUrl = normalizeStreamUrl(previousResolved || current)
  const previousTime = Number(directVideoPlayer?.currentTime?.() ?? 0)
  const restorePlaybackTime = (player: any, time: number) => {
    if (!(time > 1)) return
    try {
      const duration = Number(player?.duration?.() ?? 0)
      const resumeAt = duration > 2 ? Math.min(time, duration - 1) : time
      if (Number.isFinite(resumeAt) && resumeAt > 0) {
        player?.currentTime?.(resumeAt)
      }
    } catch {
    }
  }
  try {
    errorText.value = ''
    // Reuse existing video.js instance for m3u8 to avoid black screen caused by
    // frequent dispose/recreate in WebView2.
    if (isM3U8(normalized) && directVideoPlayer && typeof directVideoPlayer.src === 'function') {
      directVideoPlayer.src({ src: normalized, type: 'application/x-mpegURL' })
      try {
        await directVideoPlayer.play?.()
      } catch {
      }
      let ready = await waitDirectPlayerSwitchReady(directVideoPlayer)
      // If hot switch reports ready but no effective frame, do one hard rebuild.
      if (!ready) {
        try {
          await startM3U8Playback(normalized)
          ready = directVideoPlayer ? await waitDirectPlayerSwitchReady(directVideoPlayer) : false
        } catch {
          ready = false
        }
      }
      if (!ready) {
        if (previousUrl) {
          try {
            await startM3U8Playback(previousUrl)
            if (directVideoPlayer) {
              await waitDirectPlayerSwitchReady(directVideoPlayer, 5000).catch(() => false)
              restorePlaybackTime(directVideoPlayer, previousTime)
            }
          } catch {
          }
        }
        throw new Error('清晰度切换失败，已回退到上一条线路')
      }
      restorePlaybackTime(directVideoPlayer, previousTime)
      resolvedStreamUrl.value = normalized
      return
    }

    resolvedStreamUrl.value = normalized
    await startDirectPlayback(normalized)
  } catch (err) {
    resolvedStreamUrl.value = previousResolved
    const msg = getErrorMessage(err)
    errorText.value = msg
    ElMessage({ message: msg, type: 'warning' })
  } finally {
    qualitySwitching.value = false
  }
}

const applySelectedQuality = async (quality: string) => {
  if (vhsQualityOptions.value.length > 1) {
    if (!directRepresentations.length) return
    const enableAll = !quality || quality === 'auto'
    for (const item of directRepresentations) {
      try {
        item.rep?.enabled?.(enableAll || item.value === quality)
      } catch {
      }
    }
    return
  }

  if (manualQualityOptions.value.length > 1) {
    if (!quality || quality === 'auto') {
      const autoUrl = normalizeStreamUrl(manualAutoStreamUrl.value || effectiveStreamUrl.value)
      if (autoUrl) {
        await switchDirectPlaybackQuality(autoUrl)
      }
      return
    }

    const getManualOptionByValue = (value: string) => manualQualityOptions.value.find((item) => item.value === value)
    let directOption = getManualOptionByValue(quality)
    const selectedLabel = directOption?.label ?? ''

    // Refresh expiring direct URLs before switching quality to avoid blank playback
    // caused by stale auth keys inside media URLs.
    if (mediaId.value && securityToken.value) {
      await fetchDirectStreamFromMediaGateWeb(effectiveStreamUrl.value).catch(() => '')
      directOption = getManualOptionByValue(quality) ?? directOption
      if (!directOption && selectedLabel) {
        directOption = manualQualityOptions.value.find((item) => item.label === selectedLabel)
      }
    }

    if (directOption?.streamUrl) {
      await switchDirectPlaybackQuality(directOption.streamUrl)
      return
    }
  }
}

const updateQualityOptionsFromPlayer = (player: any) => {
  const list = getRepresentationsFromPlayer(player)
    .map((rep: any, index: number) => {
      const height = Number(rep?.height ?? 0)
      const bandwidth = Number(rep?.bandwidth ?? 0)
      return {
        key: getRepresentationKey(rep, index),
        rep,
        height,
        bandwidth,
      }
    })
    .filter((item) => typeof item.rep?.enabled === 'function')
    .sort((a, b) => {
      if (b.height !== a.height) return b.height - a.height
      return b.bandwidth - a.bandwidth
    })

  directRepresentations = list.map((item) => ({ value: `vhs:${item.key}`, rep: item.rep }))
  vhsQualityOptions.value = list.map((item, index) => ({
      label: buildQualityLabel(item.height, item.bandwidth, index),
      value: `vhs:${item.key}`,
      height: item.height,
      bandwidth: item.bandwidth,
      mode: 'vhs',
    }))

  const oldSelected = selectedQuality.value
  const hasSelected = qualityOptions.value.some((item) => item.value === oldSelected)
  selectedQuality.value = hasSelected ? oldSelected : 'auto'
  void applySelectedQuality(selectedQuality.value)
}

const beginReload = () => {
  reloadVersion.value += 1
  return reloadVersion.value
}

const isLatestReload = (version: number) => reloadVersion.value === version

const title = computed(() => String(route.query.title ?? '视频'))
const mediaId = computed(() => String(route.query.media_id ?? '').trim())
const securityToken = computed(() => String(route.query.security_token ?? '').trim())
const normalizeStreamUrl = (raw: unknown) => {
  const value = String(raw ?? '').trim()
  if (!value) return ''
  if (value.startsWith('http://') || value.startsWith('https://')) return value
  if (value.startsWith('//')) return `https:${value}`
  return ''
}
const streamUrl = computed(() => {
  const candidates = [route.query.stream_url, route.query.streamUrl, route.query.live_url]
  const matched = candidates.find((item) => typeof item === 'string' && item.trim().length > 0)
  return normalizeStreamUrl(matched ?? '')
})
const hasRouteStreamUrl = computed(() => Boolean(streamUrl.value))
const isLikelyDrmStreamUrl = (url: string) => {
  const raw = String(url || '').toLowerCase()
  if (!raw) return false
  return raw.includes('/drm/') || raw.includes('drm=')
}
const hasTokenPlaybackCapability = computed(() => {
  return hasRouteToken.value || Boolean(mediaId.value && securityToken.value)
})
const preferDirectOnly = computed(() => Boolean(mediaId.value && securityToken.value))
const canUseRouteStreamDirectly = computed(() => {
  if (!hasRouteStreamUrl.value) return false
  return !isLikelyDrmStreamUrl(streamUrl.value)
})
const shouldTryBackendBeforeRouteStream = computed(() => {
  if (!(mediaId.value && securityToken.value)) return false
  if (!canUseRouteStreamDirectly.value) return false
  return !isM3U8(streamUrl.value)
})
const effectiveStreamUrl = computed(() => {
  if (forceTokenMode.value) return ''
  if (resolvedStreamUrl.value) return resolvedStreamUrl.value
  return canUseRouteStreamDirectly.value ? streamUrl.value : ''
})
const hasDirectStream = computed(() => Boolean(effectiveStreamUrl.value))
const shouldUseNativeVideo = computed(() => {
  return hasDirectStream.value
})
const showQualitySelector = computed(() => {
  if (!shouldUseNativeVideo.value) return false
  return qualityOptions.value.length > 1
})
const routePlayAuthToken = computed(() => {
  const candidates = [
    route.query.play_auth_token,
    route.query.playAuthToken,
    route.query.volc_play_auth_token,
  ]
  const matched = candidates.find((item) => typeof item === 'string' && item.trim().length > 0)
  return String(matched ?? '').trim()
})
const routeKeyToken = computed(() => {
  const candidates = [
    route.query.key_token,
    route.query.keyToken,
    route.query.volc_key_token,
  ]
  const matched = candidates.find((item) => typeof item === 'string' && item.trim().length > 0)
  return String(matched ?? '').trim()
})
const routeVid = computed(() => {
  const candidates = [route.query.vid, route.query.volc_id]
  const matched = candidates.find((item) => typeof item === 'string' && item.trim().length > 0)
  return String(matched ?? '').trim()
})
const hasRouteToken = computed(() => Boolean(routePlayAuthToken.value))
const lineAppId = computed(() => {
  const n = Number(route.query.line_app_id ?? '')
  return Number.isFinite(n) && n > 0 ? n : 233260
})
const hasPlaybackParams = computed(() => hasRouteStreamUrl.value || hasRouteToken.value || Boolean(mediaId.value && securityToken.value))
const missingParamsText = computed(() => {
  if (hasPlaybackParams.value) return ''
  return '当前路由缺少播放参数：请提供 stream_url，或 media_id + security_token，或直接提供 play_auth_token。'
})
const runtimeModeText = computed(() => {
  return hasBackendBridge() ? '桌面应用' : '浏览器预览'
})
const statusText = computed(() => {
  if (loading.value) return '加载中'
  if (missingParamsText.value) return '参数缺失'
  if (errorText.value) return '播放异常'
  if (hasDirectStream.value && nativeVideoRef.value) return '可播放'
  if (playerSdk.value) return '可播放'
  return '待启动'
})

const VEPLAYER_SDK_URLS = [
  {
    name: 'local_1.15.1',
    css: new URL('../assets/css/volcengine/veplayer/1.15.1/index.min.css', import.meta.url).href,
    js: new URL('../assets/js/volcengine/1.15.1/index.min.js', import.meta.url).href,
  },
  {
    name: 'local_1.3.5',
    css: new URL('../assets/css/volcengine/veplayer/1.3.5/index.min.css', import.meta.url).href,
    js: new URL('../assets/js/volcengine/1.3.5/index.min.js', import.meta.url).href,
  },
  {
    name: 'cdn_lf_1.15.1',
    css: 'https://lf-unpkg.volccdn.com/obj/vcloudfe/sdk/@volcengine/veplayer/1.15.1/index.min.css',
    js: 'https://lf-unpkg.volccdn.com/obj/vcloudfe/sdk/@volcengine/veplayer/1.15.1/index.min.js',
  },
  {
    name: 'cdn_byteplus_1.15.1',
    css: 'https://sf-unpkg.bytepluscdn.com/obj/byteplusfe-sg/sdk/@volcengine/veplayer/1.15.1/index.min.css',
    js: 'https://sf-unpkg.bytepluscdn.com/obj/byteplusfe-sg/sdk/@volcengine/veplayer/1.15.1/index.min.js',
  },
]

const WAILS_BRIDGE_WAIT_MS = 2200
const WAILS_BRIDGE_POLL_MS = 80
const AUTO_RECOVER_COOLDOWN_MS = 15000
const AUTO_RECOVER_MAX_ATTEMPTS = 3
const PLAYBACK_HEALTHY_WINDOW_MS = 15000

const sleep = (ms: number) => new Promise<void>((resolve) => setTimeout(resolve, ms))

const waitForBackendBridge = async (timeoutMs = WAILS_BRIDGE_WAIT_MS) => {
  const deadline = Date.now() + timeoutMs
  while (Date.now() <= deadline) {
    if (hasBackendBridge()) return true
    await sleep(WAILS_BRIDGE_POLL_MS)
  }
  return false
}

const ensureCssLoaded = (href: string) => {
  const existing = document.querySelector(`link[data-veplayer-css="true"][href="${href}"]`)
  if (existing) return Promise.resolve()

  return new Promise<void>((resolve, reject) => {
    const link = document.createElement('link')
    link.rel = 'stylesheet'
    link.href = href
    link.dataset.veplayerCss = 'true'
    link.onload = () => resolve()
    link.onerror = () => reject(new Error(`VePlayer 样式加载失败：${href}`))
    document.head.appendChild(link)
  })
}

const ensureScriptLoaded = (src: string) => {
  const existing = document.querySelector(`script[data-veplayer-js="true"][src="${src}"]`)
  if (existing) return Promise.resolve()

  return new Promise<void>((resolve, reject) => {
    const script = document.createElement('script')
    script.src = src
    script.async = true
    script.dataset.veplayerJs = 'true'
    script.onload = () => resolve()
    script.onerror = () => reject(new Error(`VePlayer 脚本加载失败：${src}`))
    document.head.appendChild(script)
  })
}

const hasRequiredApi = (VePlayerLike: any) => {
  return typeof VePlayerLike === 'function'
}

const ensureVePlayer = async () => {
  for (const url of VEPLAYER_SDK_URLS) {
    try {
      if (url.css) {
        await ensureCssLoaded(url.css)
      }
      await ensureScriptLoaded(url.js)
      const loaded = (window as any).VePlayer
      if (hasRequiredApi(loaded)) {
        sdkSourceState.value = url.name
        return loaded as VePlayerCtor
      }
    } catch {
      continue
    }
  }

  const existing = (window as any).VePlayer
  if (hasRequiredApi(existing)) {
    sdkSourceState.value = 'global_existing'
    return existing as VePlayerCtor
  }

  throw new Error('VePlayer SDK 加载失败，请检查网络连接')
}

const appendTokenCandidate = (target: PlaybackTokenCandidate[], candidate: PlaybackTokenCandidate | null) => {
  if (!candidate) return
  const token = String(candidate.playAuthToken ?? '').trim()
  if (!token) return
  const normalized: PlaybackTokenCandidate = {
    playAuthToken: token,
    keyToken: String(candidate.keyToken ?? '').trim() || undefined,
    vid: String(candidate.vid ?? '').trim() || undefined,
    source: candidate.source,
  }

  const existingIndex = target.findIndex((item) => item.playAuthToken === token)
  if (existingIndex < 0) {
    target.push(normalized)
    return
  }

  const existing = target[existingIndex]
  if (!existing.keyToken && normalized.keyToken) {
    existing.keyToken = normalized.keyToken
  }
  if (!existing.vid && normalized.vid) {
    existing.vid = normalized.vid
  }
}

const extractBackendTokenCandidates = (volc: MediaVolcLike | null | undefined) => {
  const list: PlaybackTokenCandidate[] = []

  const formats = volc?.tracks?.flatMap((t) => t.formats ?? []) ?? []
  for (const format of formats) {
    const playAuthToken = String(format?.volc_play_auth_token ?? '').trim()
    if (!playAuthToken) continue
    appendTokenCandidate(list, {
      playAuthToken,
      keyToken: String(format?.volc_key_token ?? '').trim(),
      vid: String(format?.volc_id ?? '').trim(),
      source: 'wails_backend',
    })
  }

  const directToken = String(volc?.volc_play_auth_token ?? volc?.play_auth_token ?? '').trim()
  appendTokenCandidate(list, directToken ? { playAuthToken: directToken, source: 'wails_backend' } : null)
  return list
}

const setTokenCandidates = (candidates: PlaybackTokenCandidate[]) => {
  tokenCandidates.value = candidates
  activeTokenIndex.value = 0
  const first = candidates[0]
  tokenSourceState.value = first ? first.source : 'unknown'
  if (candidates.length > 0 && (directProbeState.value === 'idle' || directProbeState.value === 'no_direct_url')) {
    directProbeState.value = `${directProbeState.value}->tokens=${candidates.length}`
  }
}

const getCurrentTokenCandidate = () => {
  const idx = activeTokenIndex.value
  if (idx < 0 || idx >= tokenCandidates.value.length) return null
  return tokenCandidates.value[idx] ?? null
}

const shiftToNextTokenCandidate = () => {
  const next = activeTokenIndex.value + 1
  if (next >= tokenCandidates.value.length) return null
  activeTokenIndex.value = next
  const candidate = tokenCandidates.value[next] ?? null
  if (candidate) {
    tokenSourceState.value = `${candidate.source}#${next + 1}/${tokenCandidates.value.length}`
  }
  return candidate
}

const hasCurrentKeyToken = computed(() => {
  const current = getCurrentTokenCandidate()
  return Boolean(current?.keyToken)
})

const sortTokenCandidates = (candidates: PlaybackTokenCandidate[]) => {
  return [...candidates].sort((a, b) => {
    const score = (candidate: PlaybackTokenCandidate) => {
      let s = 0
      if (candidate.source === 'wails_backend') s += 4
      if (candidate.keyToken) s += 2
      if (candidate.vid) s += 1
      return s
    }
    return score(b) - score(a)
  })
}

const isM3U8 = (raw: string) => {
  return String(raw || '').toLowerCase().includes('.m3u8')
}

const isDrmErrorMessage = (raw: string) => {
  const msg = String(raw || '').toUpperCase()
  return msg.includes('DRM_ERROR') || msg.includes('DRM')
}

const isTokenPlayUrlErrorMessage = (raw: string) => {
  const msg = String(raw || '').toUpperCase()
  return msg.includes('PLAYAUTHTOKEN') || msg.includes('GET PLAY URL ERROR BY PLAYAUTHTOKEN') || msg.includes('"ERRORTYPE":"TOKEN"')
}

const isLicenseErrorMessage = (raw: string) => {
  const msg = String(raw || '').toUpperCase()
  return msg.includes('LICENSE') || msg.includes('"ERRORCODE":12001') || msg.includes('"VEERRORCODE":12001')
}

const pickPlayUrlFromVolcInfo = (info: VodPlayInfoRespLike | null | undefined) => {
  const adaptiveMain = normalizeStreamUrl(info?.Result?.AdaptiveInfo?.MainPlayUrl ?? '')
  const adaptiveBackup = normalizeStreamUrl(info?.Result?.AdaptiveInfo?.BackupPlayUrl ?? '')
  if (adaptiveMain) return adaptiveMain
  if (adaptiveBackup) return adaptiveBackup

  const list = Array.isArray(info?.Result?.PlayInfoList) ? info!.Result!.PlayInfoList! : []
  const normalized = list
    .flatMap((p) => [normalizeStreamUrl(p?.MainPlayUrl ?? ''), normalizeStreamUrl(p?.BackupPlayUrl ?? '')])
    .filter((url) => Boolean(url))

  const m3u8 = normalized.find((url) => isM3U8(url))
  if (m3u8) return m3u8
  return normalized[0] || ''
}

type WebStreamCandidate = {
  url: string
  height: number
  bandwidth: number
  tag: string
  isM3U8: boolean
}

const parseHeightFromText = (text: string) => {
  const raw = String(text || '').toLowerCase()
  if (!raw) return 0
  const p = raw.match(/(\d{3,4})\s*p/)
  if (p) return Number(p[1] || 0)
  const wh = raw.match(/(\d{3,5})\s*[x*]\s*(\d{3,5})/)
  if (wh) return Number(wh[2] || 0)
  return 0
}

const parseHeightFromResolution = (resolution: unknown) => {
  if (typeof resolution === 'number') {
    return Number.isFinite(resolution) && resolution > 0 ? resolution : 0
  }
  if (typeof resolution === 'string') {
    return parseHeightFromText(resolution)
  }
  if (!resolution || typeof resolution !== 'object') return 0
  const anyRes = resolution as Record<string, any>
  const direct = Number(anyRes.height ?? anyRes.h ?? 0)
  if (Number.isFinite(direct) && direct > 0) return direct
  const textFields = [anyRes.value, anyRes.text, anyRes.name]
  for (const field of textFields) {
    const parsed = parseHeightFromText(String(field || ''))
    if (parsed > 0) return parsed
  }
  return parseHeightFromText(JSON.stringify(anyRes))
}

const labelFromTag = (tag: string) => {
  const v = String(tag || '').toLowerCase()
  if (!v) return ''
  if (v.includes('ori') || v.includes('original')) return '原画'
  if (v.includes('fhd') || v.includes('fullhd')) return '超清'
  if (v.includes('hd')) return '高清'
  if (v.includes('sd')) return '标清'
  if (v.includes('ld')) return '流畅'
  return ''
}

const buildWebStreamCandidates = (info: MediaWebLike | null | undefined) => {
  const formats = info?.tracks?.flatMap((t) => t.formats ?? []) ?? []
  const map = new Map<string, WebStreamCandidate>()

  for (const format of formats) {
    const url = normalizeStreamUrl(format?.url ?? '')
    if (!url) continue

    const lower = url.toLowerCase()
    const isDrm = Number(format?.drm_version ?? 0) > 0 || lower.includes('/drm/') || lower.includes('drm=')
    if (isDrm) continue
    const formatText = String(format?.format ?? '').toLowerCase()
    const typeText = String(format?.type ?? '').toLowerCase()
    const isFlv = lower.includes('.flv') || formatText.includes('flv') || typeText.includes('flv')
    if (isFlv) continue

    const candidate: WebStreamCandidate = {
      url,
      height: parseHeightFromResolution(format?.resolution) || parseHeightFromText(String(format?.tag || '')),
      bandwidth: Math.max(0, Number(format?.rates_kbps ?? 0)) * 1000,
      tag: String(format?.tag ?? '').trim(),
      isM3U8: isM3U8(url),
    }

    if (!map.has(url)) {
      map.set(url, candidate)
      continue
    }

    const old = map.get(url)!
    if (candidate.height > old.height || candidate.bandwidth > old.bandwidth) {
      map.set(url, candidate)
    }
  }

  return [...map.values()].sort((a, b) => {
    if (b.height !== a.height) return b.height - a.height
    if (b.bandwidth !== a.bandwidth) return b.bandwidth - a.bandwidth
    if (Number(b.isM3U8) !== Number(a.isM3U8)) return Number(b.isM3U8) - Number(a.isM3U8)
    return 0
  })
}

const makeDirectQualityLabel = (candidate: WebStreamCandidate, index: number) => {
  if (candidate.height > 0) return `${candidate.height}p`
  const tagLabel = labelFromTag(candidate.tag)
  if (tagLabel) return tagLabel
  if (candidate.bandwidth > 0) {
    const mbps = candidate.bandwidth / 1_000_000
    return `${mbps.toFixed(mbps >= 10 ? 0 : 1)} Mbps`
  }
  return `线路 ${index + 1}`
}

const updateManualQualityOptionsFromWeb = (info: MediaWebLike | null | undefined, preferredUrl = '') => {
  const candidates = buildWebStreamCandidates(info)
  if (candidates.length <= 1) {
    manualQualityOptions.value = []
    manualAutoStreamUrl.value = normalizeStreamUrl(preferredUrl) || (candidates[0]?.url ?? '')
    return
  }

  const labelCounter = new Map<string, number>()
  const options: DirectQualityOption[] = candidates.map((candidate, index) => {
    const base = makeDirectQualityLabel(candidate, index)
    const count = (labelCounter.get(base) ?? 0) + 1
    labelCounter.set(base, count)
    const label = count > 1 ? `${base} ${count}` : base
    return {
      label,
      value: `direct:${index}`,
      height: candidate.height,
      bandwidth: candidate.bandwidth,
      mode: 'direct',
      streamUrl: candidate.url,
    }
  })

  manualQualityOptions.value = options

  const preferred = normalizeStreamUrl(preferredUrl)
  const autoCandidate = candidates.find((item) => item.url === preferred)
    ?? candidates.find((item) => item.isM3U8)
    ?? candidates[0]
  manualAutoStreamUrl.value = autoCandidate?.url ?? ''

  const hasSelected = qualityOptions.value.some((item) => item.value === selectedQuality.value)
  if (!hasSelected) {
    selectedQuality.value = 'auto'
  }
}

const pickWebMediaURL = (info: MediaWebLike | null | undefined) => {
  const candidates = buildWebStreamCandidates(info)
  if (!candidates.length) return ''
  const adaptive = candidates.find((f) => f.isM3U8)
  return adaptive?.url || candidates[0].url
}

const normalizeErrorMessage = (raw: string) => {
  const msg = String(raw ?? '').trim()
  if (!msg) return '播放失败，请稍后重试'
  if (msg.includes("reading 'backend'") || msg.includes('桌面后端未就绪')) {
    return '当前环境无法调用桌面端播放服务。请在桌面应用中播放，或在链接中提供 play_auth_token。'
  }
  return msg
}

const getErrorMessage = (err: unknown) => {
  if (err instanceof Error) return normalizeErrorMessage(err.message)
  if (typeof err === 'string') return normalizeErrorMessage(err)
  try {
    return normalizeErrorMessage(JSON.stringify(err))
  } catch {
    return normalizeErrorMessage(String(err))
  }
}

const trimToken = (input: string) => {
  const raw = String(input || '')
  if (raw.length <= 14) return raw
  return `${raw.slice(0, 7)}...${raw.slice(-5)}`
}

const destroyPlayer = () => {
  try {
    playerSdk.value?.dispose?.()
  } finally {
    playerSdk.value = null
    if (directVideoPlayer) {
      try {
        directVideoPlayer.dispose?.()
      } finally {
        directVideoPlayer = null
      }
    }
    if (playerRoot.value) playerRoot.value.innerHTML = ''
    if (nativeVideoRef.value) {
      nativeVideoRef.value.pause()
      nativeVideoRef.value.removeAttribute('src')
      nativeVideoRef.value.load()
    }
    resetPlayerQualityOptions()
  }
}

const onNativeVideoLoaded = () => {
  markPlaybackHealthy()
  errorText.value = ''
}

const nativeRecovering = ref(false)

const fallbackFromNativeToToken = async () => {
  if (preferDirectOnly.value) {
    directProbeState.value = 'direct_error_no_token_fallback'
    return false
  }
  if (nativeRecovering.value) return false
  if (!hasTokenPlaybackCapability.value) return false

  nativeRecovering.value = true
  try {
    directProbeState.value = 'native_error_fallback_token'
    forceTokenMode.value = true
    const candidate = await pickInitialTokenCandidate()
    await createPlayer(candidate)
    errorText.value = ''
    return true
  } catch {
    return false
  } finally {
    nativeRecovering.value = false
  }
}

const onNativeVideoError = () => {
  errorText.value = '直播流加载失败，请稍后重试'
  void fallbackFromNativeToToken()
}

const startNativePlayback = async (url: string) => {
  await nextTick()
  if (!nativeVideoRef.value || !url) {
    throw new Error('直播播放器容器未就绪')
  }

  destroyPlayer()
  resetPlayerQualityOptions()
  playerInitModeState.value = 'direct-mp4'
  nativeVideoRef.value.src = url
  nativeVideoRef.value.load()
  try {
    await nativeVideoRef.value.play()
  } catch {
  }
}

const startM3U8Playback = async (url: string) => {
  await nextTick()
  if (!nativeVideoRef.value || !url) {
    throw new Error('直播播放器容器未就绪')
  }

  destroyPlayer()
  resetPlayerQualityOptions()
  playerInitModeState.value = 'direct-m3u8'

  const player = videojs(nativeVideoRef.value, {
    controls: true,
    autoplay: true,
    preload: 'auto',
    fluid: false,
    html5: {
      vhs: {
        overrideNative: true,
      },
    },
  })
  directVideoPlayer = player
  player.src({ src: url, type: 'application/x-mpegURL' })
  player.on?.('loadedmetadata', () => {
    updateQualityOptionsFromPlayer(player)
    markPlaybackHealthy()
    errorText.value = ''
  })
  player.on?.('canplay', () => {
    if (vhsQualityOptions.value.length <= 1) {
      updateQualityOptionsFromPlayer(player)
    }
    markPlaybackHealthy()
    errorText.value = ''
  })
  player.on?.('playing', () => {
    markPlaybackHealthy()
    errorText.value = ''
  })
  player.on?.('error', () => {
    const detail = player.error?.()
    const msg = detail?.message ? normalizeErrorMessage(String(detail.message)) : '直播流加载失败，请稍后重试'
    errorText.value = msg
    void fallbackFromNativeToToken()
  })
  try {
    await player.play()
  } catch {
  }
}

const startDirectPlayback = async (url: string) => {
  if (!url) {
    throw new Error('缺少播放地址')
  }
  // Entering direct playback must disable token-forced mode so the native
  // video container is actually rendered.
  forceTokenMode.value = false
  if (isM3U8(url)) {
    await startM3U8Playback(url)
    return
  }
  await startNativePlayback(url)
}

const fetchVolcMeta = async () => {
  const backendReady = await waitForBackendBridge()
  if (!backendReady) {
    throw new Error('当前环境无法调用桌面端播放服务。请在桌面应用中播放，或在链接中提供 play_auth_token。')
  }
  return invokeBackend<MediaVolcLike>('GetVolcPlayAuthToken', mediaId.value, securityToken.value)
}

const resolvePlaybackFromBackend = async () => {
  if (!(mediaId.value && securityToken.value)) {
    return { token: '', url: '', vid: '', keyToken: '' }
  }

  const backendReady = await waitForBackendBridge()
  if (!backendReady) {
    throw new Error('当前环境无法调用桌面端播放服务。请在桌面应用中播放，或在链接中提供 play_auth_token。')
  }

  const resolved = await invokeBackend<ResolvedPlaybackLike>('ResolveVideoPlayback', mediaId.value, securityToken.value)
  const token = String(resolved?.play_auth_token ?? '').trim()
  const url = normalizeStreamUrl(resolved?.stream_url ?? '')
  const vid = String(resolved?.vid ?? '').trim()
  const keyToken = String(resolved?.key_token ?? '').trim()

  if (url) {
    directProbeState.value = `resolved:${vid || 'url'}`
  } else if (vid) {
    directProbeState.value = `formats:${vid}`
  } else {
    directProbeState.value = 'no_direct_url'
  }

  return { token, url, vid, keyToken }
}

const fetchDirectStreamFromMediaGateWeb = async (preferredUrl = '') => {
  if (!(mediaId.value && securityToken.value)) return ''
  const backendReady = await waitForBackendBridge()
  if (!backendReady) return ''
  const web = await invokeBackend<MediaWebLike>('GetMediaGateWebPlayInfo', mediaId.value, '', securityToken.value).catch(() => null)
  if (web) {
    const preferred = normalizeStreamUrl(preferredUrl)
    const initial = pickWebMediaURL(web)
    updateManualQualityOptionsFromWeb(web, preferred || initial)
  }
  const url = pickWebMediaURL(web)
  if (url) {
    directProbeState.value = 'media_gate_web_url'
  }
  return url
}

const buildPlayInfoQueries = (vid: string, playAuth: string, keyToken: string) => {
  const queries: string[] = []
  const appendQuery = (includeKey: boolean, extra: boolean) => {
    const params = new URLSearchParams()
    params.set('Vid', vid)
    params.set('PlayAuthToken', playAuth)
    params.set('Ssl', '1')
    if (extra) {
      params.set('NeedHttps', '1')
      params.set('NeedOriginal', '1')
    }
    if (includeKey && keyToken) {
      params.set('KeyToken', keyToken)
    }
    queries.push(params.toString())
  }
  appendQuery(false, true)
  appendQuery(true, true)
  appendQuery(false, false)
  appendQuery(true, false)
  return queries
}

const fetchDirectStreamUrlFromBackend = async (volcMeta?: MediaVolcLike | null) => {
  if (!(mediaId.value && securityToken.value)) return ''
  const volc = volcMeta ?? await fetchVolcMeta()
  const formats = volc?.tracks?.flatMap((t) => t.formats ?? []) ?? []
  directProbeState.value = `formats=${formats.length}`

  for (const format of formats) {
    const vid = String(format?.volc_id ?? '').trim()
    const playAuth = String(format?.volc_play_auth_token ?? '').trim()
    const keyToken = String(format?.volc_key_token ?? '').trim()
    if (!vid || !playAuth) continue

    const queries = buildPlayInfoQueries(vid, playAuth, keyToken)
    for (const query of queries) {
      // eslint-disable-next-line no-await-in-loop
      const info = await invokeBackend<VodPlayInfoRespLike>('GetVolcPlayInfo', query).catch(() => null)
      const url = pickPlayUrlFromVolcInfo(info)
      if (url) {
        directProbeState.value = `resolved:${vid}`
        return url
      }
    }
  }
  directProbeState.value = 'no_direct_url'
  return ''
}

const prepareTokenCandidates = async (volcMeta?: MediaVolcLike | null) => {
  const list: PlaybackTokenCandidate[] = []
  if (mediaId.value && securityToken.value) {
    let backendLoaded = false
    try {
      const volc = volcMeta ?? await fetchVolcMeta()
      const backendCandidates = extractBackendTokenCandidates(volc)
      for (const candidate of backendCandidates) {
        appendTokenCandidate(list, candidate)
      }
      backendLoaded = true
    } catch {
      if (hasRouteToken.value) {
        appendTokenCandidate(list, {
          playAuthToken: routePlayAuthToken.value,
          keyToken: routeKeyToken.value || undefined,
          vid: routeVid.value || undefined,
          source: 'route_query_fallback',
        })
        setTokenCandidates(sortTokenCandidates(list))
        return list
      }
      throw new Error('当前环境无法调用桌面端播放服务。请在桌面应用中播放，或在链接中提供 play_auth_token。')
    }

    // If backend token list exists, do not mix route fallback token to avoid
    // switching to stale/invalid query tokens that can trigger license failures.
    if (!backendLoaded || list.length === 0) {
      appendTokenCandidate(list, {
        playAuthToken: routePlayAuthToken.value,
        keyToken: routeKeyToken.value || undefined,
        vid: routeVid.value || undefined,
        source: 'route_query_fallback',
      })
    }
    setTokenCandidates(sortTokenCandidates(list))
    return list
  }

  if (hasRouteToken.value) {
    appendTokenCandidate(list, {
      playAuthToken: routePlayAuthToken.value,
      keyToken: routeKeyToken.value || undefined,
      vid: routeVid.value || undefined,
      source: 'route_query',
    })
    setTokenCandidates(sortTokenCandidates(list))
    return list
  }

  throw new Error('缺少 media_id 或 security_token')
}

const pickInitialTokenCandidate = async (volcMeta?: MediaVolcLike | null) => {
  const candidates = await prepareTokenCandidates(volcMeta)
  const first = candidates[0]
  if (!first?.playAuthToken) {
    throw new Error('未获取到火山点播 playAuthToken')
  }
  tokenSourceState.value = `${first.source}#1/${candidates.length}`
  return first
}

const refreshPrimaryTokenCandidate = async () => {
  const candidates = await prepareTokenCandidates()
  const first = candidates[0]
  if (!first?.playAuthToken) {
    throw new Error('token 刷新失败：未获取到 playAuthToken')
  }
  return first
}

const drmRecovering = ref(false)
const tokenSwitching = ref(false)
const tokenRefreshing = ref(false)
const autoRecoverAttempts = ref(0)
const lastAutoRecoverAt = ref(0)
const playbackStarted = ref(false)
const lastPlaybackHeartbeatAt = ref(0)

const markPlaybackHealthy = () => {
  playbackStarted.value = true
  lastPlaybackHeartbeatAt.value = Date.now()
}

const isPlaybackHealthyRecently = () => {
  if (!playbackStarted.value) return false
  return Date.now() - lastPlaybackHeartbeatAt.value < PLAYBACK_HEALTHY_WINDOW_MS
}

const resetPlaybackHealthState = () => {
  playbackStarted.value = false
  lastPlaybackHeartbeatAt.value = 0
}

const resetAutoRecoverState = () => {
  autoRecoverAttempts.value = 0
  lastAutoRecoverAt.value = 0
}

const shouldSuppressAutoRecover = () => {
  const now = Date.now()
  if (autoRecoverAttempts.value >= AUTO_RECOVER_MAX_ATTEMPTS) {
    directProbeState.value = `token_recover_limit_${AUTO_RECOVER_MAX_ATTEMPTS}`
    return true
  }
  if (now - lastAutoRecoverAt.value < AUTO_RECOVER_COOLDOWN_MS) {
    return true
  }
  return false
}

const markAutoRecoverAttempt = () => {
  autoRecoverAttempts.value += 1
  lastAutoRecoverAt.value = Date.now()
}

const trySwitchToNextToken = async () => {
  if (tokenSwitching.value) return false
  const next = shiftToNextTokenCandidate()
  if (!next?.playAuthToken) return false

  tokenSwitching.value = true
  try {
    directProbeState.value = `token_retry_${activeTokenIndex.value + 1}/${tokenCandidates.value.length}`
    await createPlayer(next)
    errorText.value = ''
    return true
  } catch {
    return false
  } finally {
    tokenSwitching.value = false
  }
}

const tryRefreshPrimaryToken = async () => {
  if (tokenRefreshing.value) return false
  if (!(mediaId.value && securityToken.value)) return false

  tokenRefreshing.value = true
  try {
    const refreshed = await refreshPrimaryTokenCandidate()
    directProbeState.value = 'token_refresh_primary'
    await createPlayer(refreshed)
    errorText.value = ''
    return true
  } catch {
    return false
  } finally {
    tokenRefreshing.value = false
  }
}

const tryRecoverFromDrmError = async () => {
  if (drmRecovering.value) return false
  if (!(mediaId.value && securityToken.value)) return false

  drmRecovering.value = true
  try {
    try {
      const resolved = await resolvePlaybackFromBackend()
      if (resolved.url) {
        resolvedStreamUrl.value = resolved.url
        await startDirectPlayback(resolved.url)
        errorText.value = ''
        return true
      }
    } catch {
    }

    const directUrl = await fetchDirectStreamUrlFromBackend()
    if (!directUrl) return false

    resolvedStreamUrl.value = directUrl
    await startDirectPlayback(directUrl)
    errorText.value = ''
    return true
  } catch {
    return false
  } finally {
    drmRecovering.value = false
  }
}

const tryRecoverFromTokenError = async () => {
  if (isPlaybackHealthyRecently()) {
    directProbeState.value = 'ignore_auth_error_while_playing'
    return true
  }
  if (preferDirectOnly.value) {
    directProbeState.value = 'direct_only_recover'
    const directRecovered = await tryRecoverFromDrmError()
    return directRecovered
  }
  if (shouldSuppressAutoRecover()) {
    return true
  }
  markAutoRecoverAttempt()
  const tokenRefreshed = await tryRefreshPrimaryToken()
  if (tokenRefreshed) return true
  const tokenRetried = await trySwitchToNextToken()
  if (tokenRetried) return true
  directProbeState.value = 'token_exhausted_try_direct'
  const directRecovered = await tryRecoverFromDrmError()
  return directRecovered
}

type PlayerInitMode = 'full' | 'safe' | 'minimal'

const createPlayerOptions = (candidate: PlaybackTokenCandidate, mode: PlayerInitMode) => {
  const getVideoByToken: Record<string, any> = { playAuthToken: candidate.playAuthToken }
  if (candidate.keyToken) {
    getVideoByToken.keyToken = candidate.keyToken
  }
  if (candidate.vid) {
    getVideoByToken.vid = candidate.vid
  }
  if (mode === 'full') {
    getVideoByToken.definitionMap = {
      original: { definition: 'ori', definitionTextKey: 'ORI' },
      '360p': { definition: 'ld', definitionTextKey: 'LD' },
      '480p': { definition: 'sd', definitionTextKey: 'SD' },
      '720p': { definition: 'hd', definitionTextKey: 'HD' },
    }
  }

  const options: Record<string, any> = {
    root: playerRoot.value,
    getVideoByToken,
  }

  if (mode === 'full') {
    options.languages = {
      zh: {
        ORI: '原画',
        LD: '流畅',
        SD: '标清',
        HD: '高清',
      },
    }
    options.lang = 'zh'
    options.autoplay = true
    options.vodLogOpts = {
      vtype: "FLV",
      tag: "直播",
      codec_type: "h264",
      line_app_id: lineAppId.value,
      line_user_id: String(store.user?.uid_hazy ?? 'unknown'),
      playerCoreVersion: "2.16.2"
    }
  }

  if (mode === 'safe') {
    options.autoplay = true
  }

  if ((mode === 'full' || mode === 'safe') && mediaId.value && securityToken.value) {
    options.onTokenExpired = async () => {
      const refreshed = await refreshPrimaryTokenCandidate()
      return refreshed.keyToken
        ? { playAuthToken: refreshed.playAuthToken, keyToken: refreshed.keyToken }
        : { playAuthToken: refreshed.playAuthToken }
    }
  }

  return options
}

const bindPlayerErrorHandler = (instance: any) => {
  instance?.on?.('error', (...args: any[]) => {
    const msg = args.map(getErrorMessage).join(' ')
    if (!msg) return
    if (isDrmErrorMessage(msg) || isTokenPlayUrlErrorMessage(msg) || isLicenseErrorMessage(msg)) {
      void (async () => {
        const recovered = await tryRecoverFromTokenError()
        if (!recovered) {
          errorText.value = msg
        }
      })()
      return
    }

    errorText.value = msg
  })
  instance?.on?.('canplay', () => {
    markPlaybackHealthy()
    if (!errorText.value) return
    if (isDrmErrorMessage(errorText.value) || isTokenPlayUrlErrorMessage(errorText.value) || isLicenseErrorMessage(errorText.value)) {
      errorText.value = ''
    }
  })
  instance?.on?.('playing', () => {
    markPlaybackHealthy()
    if (!errorText.value) return
    if (isDrmErrorMessage(errorText.value) || isTokenPlayUrlErrorMessage(errorText.value) || isLicenseErrorMessage(errorText.value)) {
      errorText.value = ''
    }
  })
}

const createPlayer = async (candidate: PlaybackTokenCandidate) => {
  const VePlayer = await ensureVePlayer()

  await nextTick()
  if (!playerRoot.value) {
    throw new Error('播放器容器未就绪')
  }

  destroyPlayer()
  let lastError: unknown = null
  const modes: PlayerInitMode[] = ['full', 'safe', 'minimal']

  for (const mode of modes) {
    try {
      const instance = new (VePlayer as any)(createPlayerOptions(candidate, mode))
      bindPlayerErrorHandler(instance)
      playerSdk.value = instance
      playerInitModeState.value = mode
      return
    } catch (err) {
      lastError = err
      destroyPlayer()
    }
  }

  playerInitModeState.value = 'failed'
  const reason = getErrorMessage(lastError)
  throw new Error(`播放器初始化失败：${reason}`)
}

const reload = async () => {
  const version = beginReload()
  if (!hasPlaybackParams.value) {
    errorText.value = missingParamsText.value
    return
  }
  loading.value = true
  errorText.value = ''
  directUrlError.value = ''
  resetAutoRecoverState()
  resetPlaybackHealthState()
  directProbeState.value = 'idle'
  tokenSourceState.value = 'unknown'
  playerInitModeState.value = 'idle'
  tokenCandidates.value = []
  activeTokenIndex.value = 0
  resetAllQualityOptions()
  try {
    forceTokenMode.value = false
    resolvedStreamUrl.value = ''
    if (!isLatestReload(version)) return
    if (canUseRouteStreamDirectly.value && !shouldTryBackendBeforeRouteStream.value) {
      directProbeState.value = 'from_route_stream_url'
      if (mediaId.value && securityToken.value) {
        void fetchDirectStreamFromMediaGateWeb(streamUrl.value).catch(() => '')
      }
      await startDirectPlayback(streamUrl.value)
      if (!isLatestReload(version)) {
        destroyPlayer()
        return
      }
      return
    }
    if (hasRouteStreamUrl.value && !canUseRouteStreamDirectly.value) {
      forceTokenMode.value = true
      directProbeState.value = 'route_stream_drm_token'
    }

    if (mediaId.value && securityToken.value) {
      try {
        directProbeState.value = 'resolving_playback'
        const resolved = await resolvePlaybackFromBackend()
        if (!isLatestReload(version)) return
        if (resolved.url) {
          resolvedStreamUrl.value = resolved.url
          void fetchDirectStreamFromMediaGateWeb(resolved.url).catch(() => '')
          if (isLikelyDrmStreamUrl(resolved.url)) {
            directProbeState.value = 'resolved_drm_url_direct_try'
          }
          await startDirectPlayback(resolved.url)
          if (!isLatestReload(version)) {
            destroyPlayer()
            return
          }
          return
        }
      } catch (err) {
        directProbeState.value = 'resolve_playback_failed'
        directUrlError.value = getErrorMessage(err)
      }

      const mediaGateUrl = await fetchDirectStreamFromMediaGateWeb().catch((err) => {
        directUrlError.value = getErrorMessage(err)
        return ''
      })
      if (!isLatestReload(version)) return
      if (mediaGateUrl) {
        resolvedStreamUrl.value = mediaGateUrl
        await startDirectPlayback(mediaGateUrl)
        if (!isLatestReload(version)) {
          destroyPlayer()
          return
        }
        return
      }

      const directUrl = await fetchDirectStreamUrlFromBackend().catch((err) => {
        directUrlError.value = getErrorMessage(err)
        return ''
      })
      if (!isLatestReload(version)) return
      if (directUrl) {
        resolvedStreamUrl.value = directUrl
        void fetchDirectStreamFromMediaGateWeb(directUrl).catch(() => '')
        await startDirectPlayback(directUrl)
        if (!isLatestReload(version)) {
          destroyPlayer()
          return
        }
        return
      }
    }

    if (canUseRouteStreamDirectly.value) {
      directProbeState.value = 'route_stream_fallback_after_resolve'
      await startDirectPlayback(streamUrl.value)
      if (!isLatestReload(version)) {
        destroyPlayer()
        return
      }
      return
    }

    if (preferDirectOnly.value) {
      if (!directUrlError.value) {
        directUrlError.value = '直链解析失败'
      }
      throw new Error('未获取到可用直链，已禁用 token 鉴权回退')
    }

    const candidate = await pickInitialTokenCandidate()
    if (!isLatestReload(version)) return
    await createPlayer(candidate)
    if (!isLatestReload(version)) {
      destroyPlayer()
      return
    }
  } catch (err) {
    if (!isLatestReload(version)) return
    const msg = getErrorMessage(err)
    if (isDrmErrorMessage(msg) || isTokenPlayUrlErrorMessage(msg) || isLicenseErrorMessage(msg)) {
      const recovered = await tryRecoverFromTokenError()
      if (recovered) {
        errorText.value = ''
      } else {
        errorText.value = msg
        ElMessage({ message: msg, type: 'warning' })
      }
    } else {
      errorText.value = msg
      ElMessage({ message: msg, type: 'warning' })
    }
  } finally {
    if (isLatestReload(version)) {
      loading.value = false
    }
  }
}

const goBack = () => {
  router.back()
}

const copyDebugInfo = async () => {
  const directSource = hasDirectStream.value
    ? (canUseRouteStreamDirectly.value ? 'route_stream_url' : 'resolved_stream_url')
    : tokenSourceState.value
  const content = [
    `title=${title.value}`,
    `runtime=${runtimeModeText.value}`,
    `play_mode=${hasDirectStream.value ? 'direct_stream' : 'token'}`,
    `stream_url=${effectiveStreamUrl.value}`,
    `resolved_direct_url=${resolvedStreamUrl.value}`,
    `direct_url_error=${directUrlError.value}`,
    `media_id=${mediaId.value}`,
    `security_token=${securityToken.value}`,
    `token_source=${directSource}`,
    `direct_probe=${directProbeState.value}`,
    `force_token_mode=${forceTokenMode.value}`,
    `sdk_source=${sdkSourceState.value}`,
    `player_init_mode=${playerInitModeState.value}`,
    `token_candidates=${tokenCandidates.value.length}`,
    `token_index=${tokenCandidates.value.length ? activeTokenIndex.value + 1 : 0}`,
    `token_has_keytoken=${hasCurrentKeyToken.value}`,
    `quality_options_total=${qualityOptions.value.length}`,
    `quality_options_vhs=${vhsQualityOptions.value.length}`,
    `quality_options_direct=${manualQualityOptions.value.length}`,
    `quality_selected=${selectedQuality.value}`,
    `quality_switching=${qualitySwitching.value}`,
    `reload_version=${reloadVersion.value}`,
    `line_app_id=${lineAppId.value}`,
    `status=${statusText.value}`,
    `error=${errorText.value}`,
  ].join('\n')

  try {
    await navigator.clipboard.writeText(content)
    ElMessage({ message: '调试信息已复制', type: 'success' })
  } catch {
    ElMessage({ message: '复制失败，请手动复制', type: 'warning' })
  }
}

const routeKey = computed(() => `${streamUrl.value}|${mediaId.value}|${securityToken.value}|${routePlayAuthToken.value}|${lineAppId.value}`)
watch(selectedQuality, (quality) => {
  void applySelectedQuality(quality)
})

watch(routeKey, () => {
  forceTokenMode.value = false
  resolvedStreamUrl.value = ''
  directUrlError.value = ''
  playerInitModeState.value = 'idle'
  resetAllQualityOptions()
  resetAutoRecoverState()
  resetPlaybackHealthState()
  tokenCandidates.value = []
  activeTokenIndex.value = 0
  reload()
}, { immediate: true })

onUnmounted(() => {
  destroyPlayer()
})
</script>

<style scoped>
.veplayer-page {
  min-height: calc(100vh - 60px);
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 14px;
  box-sizing: border-box;
  overflow-y: auto;
  scrollbar-width: none;
  -ms-overflow-style: none;
}

.veplayer-page::-webkit-scrollbar {
  display: none;
}

.player-hero {
  display: grid;
  grid-template-columns: 1fr 340px;
  gap: 14px;
  padding: 20px;
  border-radius: 16px;
  border: 1px solid color-mix(in srgb, var(--border-soft) 82%, transparent);
  background:
    radial-gradient(360px 180px at 14% 0%, color-mix(in srgb, var(--primary-color) 18%, transparent) 0%, transparent 72%),
    radial-gradient(260px 160px at 90% 0%, color-mix(in srgb, var(--accent-color) 15%, transparent) 0%, transparent 74%),
    color-mix(in srgb, var(--surface-glass) 74%, transparent);
  box-shadow: 0 12px 28px rgba(8, 18, 32, 0.08);
  backdrop-filter: blur(10px);
}

.hero-kicker {
  margin: 0;
  color: var(--accent-color);
  font-size: 12px;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  font-weight: 700;
}

.hero-title {
  margin: 8px 0 0;
  font-size: 28px;
  line-height: 1.2;
  color: var(--text-primary);
  font-family: var(--font-family-display);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.hero-subtitle {
  margin: 10px 0 0;
  color: var(--text-secondary);
  font-size: 14px;
}

.hero-actions {
  margin-top: 16px;
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: center;
}

.quality-select {
  width: 132px;
}

.hero-stats {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}

.stat-card {
  padding: 12px;
  border-radius: 12px;
  border: 1px solid color-mix(in srgb, var(--border-soft) 76%, transparent);
  background: color-mix(in srgb, var(--card-bg) 84%, transparent);
}

.stat-card span {
  display: block;
  font-size: 12px;
  color: var(--text-tertiary);
}

.stat-card strong {
  display: block;
  margin-top: 4px;
  font-size: 16px;
  color: var(--text-primary);
  line-height: 1.2;
}

.player-workspace {
  display: grid;
  grid-template-columns: 1fr 260px;
  gap: 12px;
  min-height: 580px;
}

.player-stage {
  position: relative;
  min-height: 420px;
  border-radius: 16px;
  overflow: hidden;
  border: 1px solid color-mix(in srgb, var(--border-soft) 82%, transparent);
  background: color-mix(in srgb, #000 88%, var(--card-bg) 12%);
}

.veplayer-container {
  width: 100%;
  height: 100%;
  min-height: 420px;
  background: #000;
}

.native-video {
  width: 100%;
  height: 100%;
  min-height: 420px;
  display: block;
  background: #000;
  object-fit: contain;
}

.veplayer-loading,
.veplayer-error,
.veplayer-empty {
  position: absolute;
  inset: 0;
  display: flex;
  justify-content: center;
  align-items: center;
  background: color-mix(in srgb, var(--bg-color) 86%, transparent);
  z-index: 2;
  padding: 10px;
}

.player-sidebar {
  border-radius: 16px;
  border: 1px solid color-mix(in srgb, var(--border-soft) 82%, transparent);
  background: color-mix(in srgb, var(--card-bg) 88%, transparent);
  padding: 16px;
  box-sizing: border-box;
}

.player-sidebar h3 {
  margin: 0 0 10px;
  font-size: 18px;
  color: var(--text-primary);
  font-family: var(--font-family-display);
}

.player-sidebar ul {
  margin: 0;
  padding-left: 18px;
  color: var(--text-secondary);
  font-size: 13px;
  line-height: 1.7;
}

.sidebar-status {
  margin-top: 16px;
  border-radius: 10px;
  padding: 10px 12px;
  background: color-mix(in srgb, var(--fill-color-light) 72%, transparent);
  border: 1px solid color-mix(in srgb, var(--border-soft) 70%, transparent);
}

.sidebar-status span {
  font-size: 12px;
  color: var(--text-tertiary);
}

.sidebar-status p {
  margin: 6px 0 0;
  color: var(--text-primary);
  font-size: 13px;
  word-break: break-word;
}

@media (max-width: 1180px) {
  .player-hero {
    grid-template-columns: 1fr;
  }

  .hero-stats {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }

  .player-workspace {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 860px) {
  .veplayer-page {
    padding: 10px;
  }

  .hero-title {
    font-size: 23px;
  }

  .hero-stats {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .player-sidebar {
    display: none;
  }
}

@media (max-width: 620px) {
  .hero-actions :deep(.el-button) {
    margin: 0;
  }
}
</style>
