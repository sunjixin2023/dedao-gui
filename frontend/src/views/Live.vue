<template>
  <div class="live-page">
    <section class="live-hero">
      <div class="hero-main">
        <p class="hero-kicker">Live Workspace</p>
        <h1 class="hero-title">直播学习台</h1>
        <p class="hero-subtitle">
          对齐 dedao.cn 的直播内容结构，支持预告查看、详情展开与快速跳转直播间。
        </p>

        <div class="hero-actions">
          <el-button type="primary" round :loading="refreshing" @click="refreshAll">刷新直播</el-button>
          <div class="view-toggle">
            <button class="toggle-btn" :class="viewMode === 'card' ? 'active' : ''" @click="setViewMode('card')">卡片</button>
            <button class="toggle-btn" :class="viewMode === 'list' ? 'active' : ''" @click="setViewMode('list')">列表</button>
          </div>
        </div>
      </div>

      <div class="hero-stats">
        <article class="stat-card">
          <span>直播总量</span>
          <strong>{{ totalCount }}</strong>
        </article>
        <article class="stat-card">
          <span>直播中</span>
          <strong>{{ liveNowCount }}</strong>
        </article>
        <article class="stat-card">
          <span>即将开始</span>
          <strong>{{ upcomingCount }}</strong>
        </article>
        <article class="stat-card">
          <span>可回放</span>
          <strong>{{ replayCount }}</strong>
        </article>
      </div>
    </section>

    <section class="live-tabs" v-if="tabs.length > 0">
      <button
        v-for="tab in tabs"
        :key="tab.live_type"
        class="live-tab"
        :class="activeType === tab.live_type ? 'active' : ''"
        @click="changeTab(tab.live_type)"
      >
        <span>{{ tab.tab_name }}</span>
        <em>{{ tab.tab_count }}</em>
      </button>
    </section>

    <section v-loading="loading" class="live-grid-wrapper">
      <div v-if="liveItems.length > 0" class="live-grid" :class="viewMode === 'list' ? 'list-mode' : ''">
        <article
          v-for="item in liveItems"
          :key="item.alias_id || item.id"
          class="live-card"
          @click="openDetail(item)"
        >
          <div class="live-cover">
            <el-image :src="item.logo || item.home_img" fit="cover" class="cover-image">
              <template #error>
                <div class="cover-fallback">直播</div>
              </template>
            </el-image>

            <div class="live-badges">
              <span class="badge" :class="statusTone(item)">{{ statusText(item) }}</span>
              <span class="badge ghost">{{ item.live_duration_text || "直播" }}</span>
            </div>
          </div>

          <div class="live-content">
            <h3 class="live-title" :title="item.title">{{ item.title }}</h3>
            <p class="live-intro" :title="item.intro">{{ item.intro || "暂无简介" }}</p>
            <div class="live-meta">
              <span>{{ formatStart(item) }}</span>
              <span>{{ liveAudience(item) }}</span>
            </div>

            <div class="live-actions">
              <el-button size="small" type="primary" @click.stop="openDetail(item)">查看详情</el-button>
              <el-button
                size="small"
                :loading="openingAliasId === String(item.alias_id || '')"
                @click.stop="openLiveInApp(item)"
              >
                进入直播
              </el-button>
            </div>
          </div>
        </article>
      </div>

      <div v-else class="empty-state">
        <el-icon class="empty-icon"><VideoCamera /></el-icon>
        <h3>暂时没有可展示的直播</h3>
        <p>可尝试切换标签或刷新列表。</p>
        <el-button type="primary" round @click="refreshAll">立即刷新</el-button>
      </div>

      <div class="load-more" v-if="hasMore && liveItems.length > 0">
        <el-button round :loading="loadingMore" @click="loadMore">加载更多</el-button>
      </div>
    </section>

    <el-drawer v-model="detailVisible" size="520px" :append-to-body="true" title="直播详情">
      <div class="detail-pane" v-loading="detailLoading">
        <template v-if="activeLive">
          <h3 class="detail-title">{{ detailBase?.title || activeLive.title }}</h3>
          <p class="detail-meta">{{ formatStart(activeLive) }} · {{ statusText(activeLive) }}</p>
          <p class="detail-intro">{{ detailBase?.intro || activeLive.intro || "暂无详情介绍" }}</p>

          <div class="detail-stats">
            <div class="detail-stat">
              <span>预约/观看</span>
              <strong>{{ detailBase?.invite_count || activeLive.reservation_num || 0 }}</strong>
            </div>
            <div class="detail-stat">
              <span>在线人数</span>
              <strong>{{ detailBase?.live_viewers || activeLive.live_viewers || activeLive.online_num || 0 }}</strong>
            </div>
            <div class="detail-stat">
              <span>浏览量</span>
              <strong>{{ detailBase?.live_pv || activeLive.live_pv || activeLive.pv_num || 0 }}</strong>
            </div>
          </div>

          <el-alert
            v-if="detailCheck?.need_buy || detailCheck?.error_msg"
            :title="detailCheck?.error_msg || detailCheck?.live_product_buy_tips || '该直播可能需要权限'"
            type="warning"
            :closable="false"
          />

          <div class="detail-actions">
            <el-button
              type="primary"
              round
              :loading="openingAliasId === String(activeLive?.alias_id || '')"
              @click="openLiveInApp(activeLive)"
            >
              进入直播
            </el-button>
            <el-button round @click="openLiveArticle" :disabled="!hasLiveArticle">查看关联文稿</el-button>
          </div>
        </template>
      </div>
    </el-drawer>
  </div>
</template>

<script lang="ts" setup>
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { VideoCamera } from '@element-plus/icons-vue'
import { useAppRouter } from '../composables/useRouter'
import { invokeBackend } from '../utils/backend'
import { timestampToTime } from '../utils/utils'
import { Local } from '../utils/storage'

type LiveTab = {
  tab_name: string
  tab_count: number
  live_type: number
}

type LiveItem = {
  id: number
  alias_id: string
  title: string
  intro: string
  logo: string
  home_img: string
  dd_url: string
  status: number
  starttime: number
  endtime: number
  starttime_desc: string
  reservation_num: number
  online_num: number
  live_pv: number
  live_viewers: number
  pv_num: number
  playback_status: number
  live_duration_text: string
}

type LiveTabListResp = {
  list: LiveTab[]
}

type LiveListResp = {
  list: LiveItem[]
  is_more: number
}

type LiveBaseResp = {
  title?: string
  intro?: string
  room_detail_ddurl?: string
  share_url?: string
  invite_count?: number
  live_viewers?: number
  live_pv?: number
  live_article_info?: {
    article_id?: number
    article_detail_ddurl?: string
  }
}

type LiveCheckResp = {
  token?: string
  need_buy?: boolean
  error_msg?: string
  live_product_buy_tips?: string
}

type LiveRoomPlaybackInfoResp = {
  web_pc_media_token?: string
  token?: string
}

type LiveRoomDetailResp = {
  alias_id?: string
  title?: string
  intro?: string
  dd_url?: string
  ld?: string
  hd?: string
  ld_m3u8?: string
  hd_m3u8?: string
  minibar_stream_url?: string
  playback_info?: LiveRoomPlaybackInfoResp
  web_pc_media_token?: string
  L1flv?: string
  L2flv?: string
  L3flv?: string
}

const { pushLogin, pushVideo, pushArticleDetail } = useAppRouter()

const tabs = ref<LiveTab[]>([])
const liveItems = ref<LiveItem[]>([])
const activeType = ref(0)
const page = ref(1)
const pageSize = 12
const hasMore = ref(false)
const viewMode = ref<'card' | 'list'>(Local.get('live_view_mode') === 'list' ? 'list' : 'card')

const loading = ref(false)
const refreshing = ref(false)
const loadingMore = ref(false)

const detailVisible = ref(false)
const detailLoading = ref(false)
const activeLive = ref<LiveItem | null>(null)
const detailBase = ref<LiveBaseResp | null>(null)
const detailCheck = ref<LiveCheckResp | null>(null)
const openingAliasId = ref('')

const nowSec = () => Math.floor(Date.now() / 1000)

const normalizeUrl = (raw: string) => {
  const url = String(raw || '').trim()
  if (!url) return ''
  if (url.startsWith('http://') || url.startsWith('https://')) return url
  if (url.startsWith('//')) return `https:${url}`
  if (url.startsWith('/')) return `https://www.dedao.cn${url}`
  return ''
}

const statusTone = (item: LiveItem) => {
  const now = nowSec()
  if (item.status === 2 || (item.starttime > 0 && item.endtime > 0 && now >= item.starttime && now < item.endtime)) {
    return 'live'
  }
  if (item.playback_status === 1) return 'replay'
  if (item.starttime > now) return 'upcoming'
  return 'ended'
}

const statusText = (item: LiveItem) => {
  const tone = statusTone(item)
  if (tone === 'live') return '直播中'
  if (tone === 'replay') return '可回放'
  if (tone === 'upcoming') return '直播预告'
  return '已结束'
}

const formatStart = (item: LiveItem) => {
  const ts = Number(item.starttime || 0)
  if (ts > 0) return timestampToTime(ts)
  return String(item.starttime_desc || '时间待定')
}

const liveAudience = (item: LiveItem) => {
  const viewers = Number(item.live_viewers || item.online_num || 0)
  const reserve = Number(item.reservation_num || 0)
  if (viewers > 0) return `${viewers} 人在线`
  return `${reserve} 人预约`
}

const totalCount = computed(() => {
  const fromTabs = tabs.value.reduce((sum, tab) => sum + Math.max(0, Number(tab.tab_count || 0)), 0)
  return fromTabs > 0 ? fromTabs : liveItems.value.length
})

const liveNowCount = computed(() => liveItems.value.filter((item) => statusTone(item) === 'live').length)
const upcomingCount = computed(() => liveItems.value.filter((item) => statusTone(item) === 'upcoming').length)
const replayCount = computed(() => liveItems.value.filter((item) => statusTone(item) === 'replay').length)
const hasLiveArticle = computed(() => {
  const articleId = String(detailBase.value?.live_article_info?.article_id || '').trim()
  const articleUrl = normalizeUrl(detailBase.value?.live_article_info?.article_detail_ddurl || '')
  return Boolean(articleId || articleUrl)
})

const setViewMode = (mode: 'card' | 'list') => {
  viewMode.value = mode
  Local.set('live_view_mode', mode)
}

const handleApiError = (error: unknown) => {
  const message = String((error as any)?.message || error || '')
  if (message.includes('401')) {
    pushLogin()
    return
  }
  ElMessage({
    message: message || '加载失败，请稍后重试',
    type: 'warning',
  })
}

const loadTabs = async () => {
  const data = await invokeBackend<LiveTabListResp>('LiveTabList')
  tabs.value = Array.isArray(data?.list) ? data.list : []
  if (tabs.value.length > 0 && !tabs.value.some((tab) => tab.live_type === activeType.value)) {
    activeType.value = Number(tabs.value[0].live_type || 0)
  }
}

const loadLiveList = async (append = false) => {
  if (append) {
    loadingMore.value = true
  } else {
    loading.value = true
  }
  try {
    const data = await invokeBackend<LiveListResp>('LiveList', activeType.value, page.value, pageSize)
    const fetched = Array.isArray(data?.list) ? data.list : []
    hasMore.value = Number(data?.is_more || 0) === 1
    if (append) {
      liveItems.value = liveItems.value.concat(fetched)
    } else {
      liveItems.value = fetched
    }
  } catch (error) {
    handleApiError(error)
  } finally {
    loading.value = false
    loadingMore.value = false
  }
}

const refreshAll = async () => {
  refreshing.value = true
  try {
    page.value = 1
    await loadTabs()
    await loadLiveList(false)
  } catch (error) {
    handleApiError(error)
  } finally {
    refreshing.value = false
  }
}

const loadMore = async () => {
  if (!hasMore.value || loadingMore.value) return
  page.value += 1
  await loadLiveList(true)
}

const changeTab = async (liveType: number) => {
  if (activeType.value === liveType) return
  activeType.value = liveType
  page.value = 1
  await loadLiveList(false)
}

const parseArticleIdFromUrl = (raw: string) => {
  const normalized = normalizeUrl(raw)
  if (!normalized) return ''
  try {
    const url = new URL(normalized)
    return String(url.searchParams.get('id') || '').trim()
  } catch {
    return ''
  }
}

const pickStreamUrl = (room: LiveRoomDetailResp | null) => {
  const candidates = [
    room?.hd_m3u8,
    room?.ld_m3u8,
    room?.minibar_stream_url,
    room?.hd,
    room?.ld,
    room?.L1flv,
    room?.L2flv,
    room?.L3flv,
  ]
  for (const url of candidates) {
    const normalized = normalizeUrl(String(url || ''))
    if (normalized) return normalized
  }
  return ''
}

const pickPlayAuthToken = (room: LiveRoomDetailResp | null) => {
  const token = String(room?.playback_info?.web_pc_media_token || room?.playback_info?.token || room?.web_pc_media_token || '').trim()
  return token
}

const openLiveInApp = async (item: LiveItem | null) => {
  if (!item) return
  const aliasId = String(item.alias_id || '').trim()
  if (!aliasId) {
    ElMessage({ message: '未获取到直播 alias_id', type: 'warning' })
    return
  }

  openingAliasId.value = aliasId
  try {
    const check = await invokeBackend<LiveCheckResp>('LiveCheck', aliasId, '')
    if (activeLive.value?.alias_id === aliasId && check) {
      detailCheck.value = check
    }

    const token = String(check?.token || '').trim()
    const room = await invokeBackend<LiveRoomDetailResp>('LiveRoomDetail', aliasId, '', token).catch(() => null)
    const streamUrl = pickStreamUrl(room)
    const playAuthToken = pickPlayAuthToken(room)

    if (streamUrl) {
      pushVideo({
        from: 'live',
        alias_id: aliasId,
        title: String(room?.title || item.title || '直播'),
        stream_url: streamUrl,
      })
      return
    }

    if (playAuthToken) {
      pushVideo({
        from: 'live',
        alias_id: aliasId,
        title: String(room?.title || item.title || '直播'),
        play_auth_token: playAuthToken,
        line_app_id: 233260,
      })
      return
    }

    const warningText = String(check?.error_msg || check?.live_product_buy_tips || '未获取到可播放直播源').trim()
    ElMessage({
      message: warningText,
      type: 'warning',
    })
  } catch (error) {
    handleApiError(error)
  } finally {
    openingAliasId.value = ''
  }
}

const openLiveArticle = () => {
  const articleIdFromApi = String(detailBase.value?.live_article_info?.article_id || '').trim()
  const articleIdFromUrl = parseArticleIdFromUrl(detailBase.value?.live_article_info?.article_detail_ddurl || '')
  const articleId = articleIdFromApi || articleIdFromUrl
  if (!articleId) {
    ElMessage({ message: '未获取到关联文稿 ID', type: 'warning' })
    return
  }
  pushArticleDetail(articleId, 'live', {
    title: '直播关联文稿',
    parentTitle: '直播',
  })
}

const openDetail = async (item: LiveItem) => {
  activeLive.value = item
  detailBase.value = null
  detailCheck.value = null
  detailVisible.value = true
  detailLoading.value = true
  const aliasId = String(item.alias_id || '').trim()
  if (!aliasId) {
    detailLoading.value = false
    return
  }
  try {
    const [base, check] = await Promise.all([
      invokeBackend<LiveBaseResp>('LiveBase', aliasId).catch(() => null),
      invokeBackend<LiveCheckResp>('LiveCheck', aliasId, '').catch(() => null),
    ])
    if (base) detailBase.value = base
    if (check) detailCheck.value = check
  } finally {
    detailLoading.value = false
  }
}

onMounted(async () => {
  await refreshAll()
})
</script>

<style scoped>
.live-page {
  --live-accent: #e55100;
  --live-accent-strong: #c83f00;
  --live-soft-bg: #fff4ee;
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

.live-page::-webkit-scrollbar {
  display: none;
}

.live-hero {
  display: grid;
  grid-template-columns: 1fr 320px;
  gap: 14px;
  padding: 20px;
  border-radius: 16px;
  border: 1px solid color-mix(in srgb, var(--border-soft) 82%, transparent);
  background:
    radial-gradient(340px 180px at 14% 0%, color-mix(in srgb, var(--live-accent) 20%, transparent) 0%, transparent 72%),
    radial-gradient(260px 160px at 90% 0%, color-mix(in srgb, #ff8f58 20%, transparent) 0%, transparent 74%),
    color-mix(in srgb, var(--surface-glass) 74%, transparent);
  box-shadow: 0 12px 28px rgba(8, 18, 32, 0.08);
}

.hero-kicker {
  margin: 0;
  color: var(--live-accent);
  font-size: 12px;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  font-weight: 700;
}

.hero-title {
  margin: 8px 0 0;
  font-size: 30px;
  line-height: 1.12;
  color: var(--text-primary);
  font-family: var(--font-family-display);
}

.hero-subtitle {
  margin: 10px 0 0;
  max-width: 760px;
  color: var(--text-secondary);
  font-size: 14px;
  line-height: 1.7;
}

.hero-actions {
  margin-top: 18px;
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.view-toggle {
  height: 36px;
  border-radius: 999px;
  border: 1px solid color-mix(in srgb, var(--border-soft) 76%, transparent);
  background: color-mix(in srgb, var(--card-bg) 90%, transparent);
  display: inline-flex;
  align-items: center;
  padding: 2px;
}

.toggle-btn {
  height: 30px;
  border: 0;
  border-radius: 999px;
  padding: 0 12px;
  background: transparent;
  color: var(--text-secondary);
  font-size: 12px;
  cursor: pointer;
}

.toggle-btn.active {
  color: #fff;
  background: var(--live-accent-strong);
}

.hero-stats {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
  align-content: start;
}

.stat-card {
  padding: 12px;
  border-radius: 12px;
  border: 1px solid color-mix(in srgb, var(--border-soft) 76%, transparent);
  background: color-mix(in srgb, var(--card-bg) 86%, transparent);
}

.stat-card span {
  display: block;
  font-size: 12px;
  color: var(--text-tertiary);
}

.stat-card strong {
  display: block;
  margin-top: 4px;
  font-size: 18px;
  color: var(--live-accent-strong);
  line-height: 1.2;
}

.live-tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.live-tab {
  height: 36px;
  border-radius: 999px;
  border: 1px solid color-mix(in srgb, var(--live-accent) 28%, transparent);
  background: color-mix(in srgb, var(--live-soft-bg) 78%, transparent);
  color: var(--text-primary);
  padding: 0 12px;
  font-size: 13px;
  font-weight: 600;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  transition: transform 0.2s ease, box-shadow 0.2s ease, border-color 0.2s ease;
}

.live-tab em {
  font-style: normal;
  font-size: 11px;
  color: var(--text-secondary);
}

.live-tab.active {
  color: #fff;
  border-color: transparent;
  background: linear-gradient(120deg, color-mix(in srgb, var(--live-accent) 82%, #fff 18%) 0%, var(--live-accent-strong) 100%);
}

.live-grid-wrapper {
  border-radius: 14px;
}

.live-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.live-grid.list-mode {
  grid-template-columns: 1fr;
  gap: 10px;
}

.live-grid.list-mode .live-card {
  display: grid;
  grid-template-columns: 240px minmax(0, 1fr);
  align-items: stretch;
}

.live-grid.list-mode .live-cover {
  aspect-ratio: auto;
  height: 100%;
}

.live-grid.list-mode .live-content {
  display: flex;
  flex-direction: column;
  justify-content: center;
  text-align: left;
}

.live-grid.list-mode .live-title {
  text-align: left;
}

.live-grid.list-mode .live-intro {
  text-align: left;
  min-height: auto;
  -webkit-line-clamp: 1;
}

.live-grid.list-mode .live-meta {
  text-align: left;
  justify-content: flex-start;
}

.live-grid.list-mode .live-actions {
  justify-content: flex-start;
}

.live-card {
  border-radius: 14px;
  border: 1px solid color-mix(in srgb, var(--border-soft) 80%, transparent);
  background: color-mix(in srgb, var(--card-bg) 90%, transparent);
  overflow: hidden;
  cursor: pointer;
  transition: transform 0.24s ease, box-shadow 0.24s ease, border-color 0.24s ease;
}

.live-card:hover {
  transform: translateY(-2px);
  border-color: color-mix(in srgb, var(--live-accent) 38%, transparent);
  box-shadow: 0 12px 24px rgba(16, 24, 40, 0.12);
}

.live-cover {
  position: relative;
  aspect-ratio: 16/9;
  background: #121212;
}

.cover-image {
  width: 100%;
  height: 100%;
}

.cover-fallback {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  background: linear-gradient(120deg, #2c2c2c 0%, #191919 100%);
}

.live-badges {
  position: absolute;
  top: 10px;
  left: 10px;
  display: flex;
  gap: 6px;
}

.badge {
  height: 24px;
  border-radius: 999px;
  padding: 0 10px;
  display: inline-flex;
  align-items: center;
  font-size: 11px;
  font-weight: 600;
  color: #fff;
  backdrop-filter: blur(8px);
}

.badge.live {
  background: rgba(229, 81, 0, 0.92);
}

.badge.upcoming {
  background: rgba(76, 107, 168, 0.9);
}

.badge.replay {
  background: rgba(21, 136, 99, 0.9);
}

.badge.ended {
  background: rgba(98, 103, 113, 0.9);
}

.badge.ghost {
  background: rgba(15, 20, 28, 0.5);
}

.live-content {
  padding: 12px;
}

.live-title {
  margin: 0;
  font-size: 16px;
  color: var(--text-primary);
  line-height: 1.35;
  font-family: var(--font-family-display);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.live-intro {
  margin: 8px 0 0;
  font-size: 12px;
  color: var(--text-secondary);
  line-height: 1.6;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  min-height: 38px;
}

.live-meta {
  margin-top: 10px;
  display: flex;
  justify-content: space-between;
  gap: 8px;
  font-size: 12px;
  color: var(--text-tertiary);
}

.live-actions {
  margin-top: 12px;
  display: flex;
  gap: 8px;
}

.empty-state {
  border: 1px dashed color-mix(in srgb, var(--border-soft) 82%, transparent);
  border-radius: 14px;
  min-height: 320px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: var(--text-secondary);
}

.empty-icon {
  font-size: 36px;
  color: var(--live-accent);
}

.empty-state h3 {
  margin: 0;
  font-size: 20px;
  color: var(--text-primary);
}

.empty-state p {
  margin: 0;
}

.load-more {
  margin-top: 12px;
  display: flex;
  justify-content: center;
}

.detail-pane {
  min-height: 240px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.detail-title {
  margin: 0;
  font-size: 24px;
  color: var(--text-primary);
  line-height: 1.3;
}

.detail-meta {
  margin: 0;
  color: var(--text-tertiary);
  font-size: 12px;
}

.detail-intro {
  margin: 0;
  color: var(--text-secondary);
  line-height: 1.7;
  white-space: pre-wrap;
}

.detail-stats {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
}

.detail-stat {
  border-radius: 10px;
  border: 1px solid color-mix(in srgb, var(--border-soft) 78%, transparent);
  background: color-mix(in srgb, var(--fill-color-light) 70%, transparent);
  padding: 10px;
}

.detail-stat span {
  display: block;
  font-size: 11px;
  color: var(--text-tertiary);
}

.detail-stat strong {
  display: block;
  margin-top: 4px;
  color: var(--text-primary);
  font-size: 18px;
}

.detail-actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

@media (max-width: 1180px) {
  .live-hero {
    grid-template-columns: 1fr;
  }

  .hero-stats {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }

  .live-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 860px) {
  .live-page {
    padding: 10px;
  }

  .hero-title {
    font-size: 24px;
  }

  .hero-stats {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .live-grid {
    grid-template-columns: 1fr;
  }

  .view-toggle {
    width: 100%;
    justify-content: center;
  }

  .live-grid.list-mode .live-card {
    grid-template-columns: 1fr;
  }

  .detail-stats {
    grid-template-columns: 1fr;
  }
}
</style>
