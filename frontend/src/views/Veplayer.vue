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
          <span>{{ hasStreamUrl ? 'stream_url' : 'security_token' }}</span>
          <strong :title="hasStreamUrl ? streamUrl : securityToken">
            {{ hasStreamUrl ? trimToken(streamUrl) : (securityToken ? trimToken(securityToken) : "未提供") }}
          </strong>
        </article>
      </div>
    </section>

    <section class="player-workspace">
      <div class="player-stage">
        <video
          v-if="hasStreamUrl"
          ref="nativeVideoRef"
          class="native-video"
          :src="streamUrl"
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
      volc_play_auth_token?: string
    }>
  }>
}

const route = useRoute()
const router = useRouter()
const store = userStore()

const playerRoot = ref<HTMLDivElement | null>(null)
const playerSdk = ref<InstanceType<VePlayerCtor> | null>(null)
const nativeVideoRef = ref<HTMLVideoElement | null>(null)
const loading = ref(false)
const errorText = ref('')

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
const hasStreamUrl = computed(() => Boolean(streamUrl.value))
const routePlayAuthToken = computed(() => {
  const candidates = [
    route.query.play_auth_token,
    route.query.playAuthToken,
    route.query.volc_play_auth_token,
  ]
  const matched = candidates.find((item) => typeof item === 'string' && item.trim().length > 0)
  return String(matched ?? '').trim()
})
const hasRouteToken = computed(() => Boolean(routePlayAuthToken.value))
const lineAppId = computed(() => {
  const n = Number(route.query.line_app_id ?? '')
  return Number.isFinite(n) && n > 0 ? n : 233260
})
const hasPlaybackParams = computed(() => hasStreamUrl.value || hasRouteToken.value || Boolean(mediaId.value && securityToken.value))
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
  if (hasStreamUrl.value && nativeVideoRef.value) return '可播放'
  if (playerSdk.value) return '可播放'
  return '待启动'
})

const VEPLAYER_SDK_URLS = [
  {
    css: 'https://lf-unpkg.volccdn.com/obj/vcloudfe/sdk/@volcengine/veplayer/1.15.1/index.min.css',
    js: 'https://lf-unpkg.volccdn.com/obj/vcloudfe/sdk/@volcengine/veplayer/1.15.1/index.min.js',
  },
  {
    css: 'https://sf-unpkg.bytepluscdn.com/obj/byteplusfe-sg/sdk/@volcengine/veplayer/1.15.1/index.min.css',
    js: 'https://sf-unpkg.bytepluscdn.com/obj/byteplusfe-sg/sdk/@volcengine/veplayer/1.15.1/index.min.js',
  },
  {
    js: new URL('../assets/js/volcengine/1.3.5/index.min.js', import.meta.url).href,
  },
]

const WAILS_BRIDGE_WAIT_MS = 2200
const WAILS_BRIDGE_POLL_MS = 80

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
  const existing = (window as any).VePlayer
  if (hasRequiredApi(existing)) return existing as VePlayerCtor

  for (const url of VEPLAYER_SDK_URLS) {
    try {
      if (url.css) {
        await ensureCssLoaded(url.css)
      }
      await ensureScriptLoaded(url.js)
      const loaded = (window as any).VePlayer
      if (hasRequiredApi(loaded)) return loaded as VePlayerCtor
    } catch {
      continue
    }
  }

  throw new Error('VePlayer SDK 加载失败，请检查网络连接')
}

const pickPlayAuthToken = (volc: MediaVolcLike | null | undefined) => {
  const directToken = String(volc?.volc_play_auth_token ?? volc?.play_auth_token ?? '').trim()
  if (directToken) return directToken
  const formats = volc?.tracks?.flatMap((t) => t.formats ?? []) ?? []
  const token = formats.map((f) => f?.volc_play_auth_token).find((t) => typeof t === 'string' && t.length > 0)
  return token ?? ''
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
    if (playerRoot.value) playerRoot.value.innerHTML = ''
    if (nativeVideoRef.value) {
      nativeVideoRef.value.pause()
      nativeVideoRef.value.removeAttribute('src')
      nativeVideoRef.value.load()
    }
  }
}

const onNativeVideoLoaded = () => {
  errorText.value = ''
}

const onNativeVideoError = () => {
  errorText.value = '直播流加载失败，请稍后重试'
}

const startNativePlayback = async () => {
  await nextTick()
  if (!nativeVideoRef.value || !streamUrl.value) {
    throw new Error('直播播放器容器未就绪')
  }

  destroyPlayer()
  nativeVideoRef.value.src = streamUrl.value
  nativeVideoRef.value.load()
  try {
    await nativeVideoRef.value.play()
  } catch {
  }
}

const fetchPlayAuthToken = async () => {
  if (hasRouteToken.value) {
    return routePlayAuthToken.value
  }

  if (!(mediaId.value && securityToken.value)) {
    throw new Error('缺少 media_id 或 security_token')
  }

  const backendReady = await waitForBackendBridge()
  if (!backendReady) {
    throw new Error('当前环境无法调用桌面端播放服务。请在桌面应用中播放，或在链接中提供 play_auth_token。')
  }

  const volc = await invokeBackend<MediaVolcLike>('GetVolcPlayAuthToken', mediaId.value, securityToken.value)
  const token = pickPlayAuthToken(volc)
  if (!token) {
    throw new Error('未获取到火山点播 playAuthToken')
  }
  return token
}

const createPlayer = async (playAuthToken: string) => {
  const VePlayer = await ensureVePlayer()

  await nextTick()
  if (!playerRoot.value) {
    throw new Error('播放器容器未就绪')
  }

  destroyPlayer()

  const playerOptions: Record<string, any> = {
    root: playerRoot.value,
    getVideoByToken: {
      playAuthToken,
      definitionMap: {
        original: { definition: 'ori', definitionTextKey: 'ORI' },
        '360p': { definition: 'ld', definitionTextKey: 'LD' },
        '480p': { definition: 'sd', definitionTextKey: 'SD' },
        '720p': { definition: 'hd', definitionTextKey: 'HD' },
      },
    },
    vodLogOpts: {
      vtype: "FLV",
      tag: "直播",
      codec_type: "h264",
      drm_type: 1,
      line_app_id: lineAppId.value,
      line_user_id: String(store.user?.uid_hazy ?? 'unknown'),
      playerCoreVersion: "2.16.2"
    },
    languages: {
      zh: {
        ORI: '原画',
        LD: '流畅',
        SD: '标清',
        HD: '高清',
      },
    },
    lang: 'zh',
    autoplay: true,
  }

  if (mediaId.value && securityToken.value) {
    playerOptions.onTokenExpired = async () => {
      const newToken = await fetchPlayAuthToken()
      return { playAuthToken: newToken }
    }
  }

  const instance = new (VePlayer as any)(playerOptions)

  instance?.on?.('error', (...args: any[]) => {
    const msg = args.map(getErrorMessage).join(' ')
    if (!msg) return
    errorText.value = msg
  })

  playerSdk.value = instance
}

const reload = async () => {
  if (!hasPlaybackParams.value) {
    errorText.value = missingParamsText.value
    return
  }
  loading.value = true
  errorText.value = ''
  try {
    if (hasStreamUrl.value) {
      await startNativePlayback()
      return
    }
    const token = await fetchPlayAuthToken()
    await createPlayer(token)
  } catch (err) {
    const msg = getErrorMessage(err)
    errorText.value = msg
    ElMessage({ message: msg, type: 'warning' })
  } finally {
    loading.value = false
  }
}

const goBack = () => {
  router.back()
}

const copyDebugInfo = async () => {
  const content = [
    `title=${title.value}`,
    `runtime=${runtimeModeText.value}`,
    `play_mode=${hasStreamUrl.value ? 'direct_stream' : 'token'}`,
    `stream_url=${streamUrl.value}`,
    `media_id=${mediaId.value}`,
    `security_token=${securityToken.value}`,
    `token_source=${hasRouteToken.value ? 'route_query' : hasStreamUrl.value ? 'stream_url' : 'wails_backend'}`,
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
watch(routeKey, () => reload(), { immediate: true })

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
