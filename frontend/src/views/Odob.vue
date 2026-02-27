<template>
    <div class="odob-container">
        <section class="audio-hero">
            <div class="hero-content">
                <p class="hero-kicker">Audio Workspace</p>
                <h1 class="hero-title">{{ groupMode.active ? groupMode.title : "我的听书学习台" }}</h1>
                <p class="hero-subtitle">
                    {{ groupMode.active ? "分组内内容已展开，可直接播放或跳转文稿阅读。" : "支持播放列表连续学习，文稿与音频一键切换。" }}
                </p>

                <div class="hero-actions">
                    <el-button type="primary" round @click="playFirstAvailable">
                        {{ normalTrackCount > 0 ? "开始播放" : "浏览内容" }}
                    </el-button>
                    <el-button round :icon="RefreshRight" @click="refreshList">刷新列表</el-button>
                    <el-button round :icon="DownloadIcon" @click="goDownloadSetting">下载设置</el-button>
                    <el-button v-if="pStore.hasTrack" round :icon="Headset" @click="pStore.openPlaylist">当前播放</el-button>
                    <div class="view-toggle">
                        <button class="toggle-btn" :class="viewMode === 'card' ? 'active' : ''" @click="setViewMode('card')">卡片</button>
                        <button class="toggle-btn" :class="viewMode === 'list' ? 'active' : ''" @click="setViewMode('list')">列表</button>
                    </div>
                </div>

                <div v-if="pStore.hasTrack" class="now-playing-chip" :title="nowPlayingTitle">
                    <el-icon><Headset /></el-icon>
                    <span>{{ nowPlayingTitle }}</span>
                </div>
            </div>

            <div class="hero-stats">
                <div class="hero-wave" aria-hidden="true">
                    <span v-for="n in 14" :key="`wave-${n}`" class="wave-bar" :style="{ animationDelay: `${n * 80}ms` }"></span>
                </div>
                <article class="stat-card">
                    <span>音频数</span>
                    <strong>{{ normalTrackCount }}</strong>
                </article>
                <article class="stat-card">
                    <span>分组数</span>
                    <strong>{{ groupTrackCount }}</strong>
                </article>
                <article class="stat-card">
                    <span>总时长</span>
                    <strong>{{ totalDurationLabel }}</strong>
                </article>
                <article class="stat-card">
                    <span>筛选条件</span>
                    <strong>{{ activeFilterName }}</strong>
                </article>
            </div>
        </section>

        <section class="audio-scenes">
            <button class="scene-pill scene-pill-primary" @click="playFirstAvailable">通勤快听</button>
            <button class="scene-pill" @click="openCurrentPlaylist">连续播单</button>
            <button class="scene-pill" @click="refreshList">刷新推荐</button>
            <div class="scene-summary">
                <span>听书节奏</span>
                <strong>{{ listeningPace }}</strong>
                <p>{{ queueSummary }}</p>
            </div>
        </section>

        <div v-if="groupMode.active" class="group-header">
            <el-button type="primary" link @click="exitGroup">
                <el-icon class="el-icon--left"><ArrowLeft /></el-icon>返回全部听书
            </el-button>
            <span class="group-title">{{ groupMode.title }}</span>
        </div>

        <div v-if="hasFilters && !groupMode.active" class="filter-container">
            <el-radio-group v-model="currentFilter" @change="handleFilterChange" size="small">
                <el-radio-button
                    v-for="item in filterOptions"
                    :key="item.filter"
                    :label="item.filter"
                >
                    {{ item.name }}
                    <span v-if="item.show_count">({{ item.count }})</span>
                </el-radio-button>
            </el-radio-group>
        </div>

        <div
            v-loading="initLoading"
            class="odob-grid-container"
            v-infinite-scroll="loadMore"
            :infinite-scroll-disabled="disabled"
            :infinite-scroll-immediate="false"
        >
            <div v-if="audioList.length > 0" class="odob-grid" :class="viewMode === 'list' ? 'list-mode' : ''">
                <div
                    v-for="item in audioList"
                    :key="item.id"
                    class="odob-card"
                    @click="item.is_group ? enterGroup(item) : handlePlay(item)"
                >
                    <div class="card-cover">
                        <div v-if="item.is_group && item.group_books && item.group_books.length > 0" class="group-cover-grid">
                            <div v-for="(book, index) in item.group_books.slice(0, 4)" :key="book.id || index" class="group-grid-item">
                                <el-image :src="book.icon" fit="cover" loading="lazy" class="grid-image">
                                    <template #error>
                                        <div class="grid-placeholder">
                                            <el-icon><Picture /></el-icon>
                                        </div>
                                    </template>
                                </el-image>
                            </div>
                            <div v-for="n in (4 - Math.min(item.group_books.length, 4))" :key="'ph-' + n" class="group-grid-item">
                                <div class="grid-placeholder bg-gray">
                                    <el-icon><Picture /></el-icon>
                                </div>
                            </div>
                        </div>

                        <el-image
                            v-else-if="item.icon"
                            :src="item.icon"
                            fit="cover"
                            loading="lazy"
                        >
                            <template #placeholder>
                                <div class="image-placeholder">
                                    <el-icon><Picture /></el-icon>
                                </div>
                            </template>
                        </el-image>
                        <div v-else class="no-cover">
                            <el-icon v-if="item.is_group" :size="40"><Folder /></el-icon>
                            <span v-else>无封面</span>
                        </div>

                        <div v-if="!item.is_group" class="card-overlay" @click.stop>
                            <div class="overlay-actions">
                                <el-tooltip content="播放" :show-after="400">
                                    <el-button circle type="primary" :icon="VideoPlay" @click.stop="handlePlay(item)" />
                                </el-tooltip>
                                <el-tooltip content="文稿" :show-after="400">
                                    <el-button circle type="success" :icon="Memo" @click.stop="gotoArticleDetail(item)" />
                                </el-tooltip>
                                <el-tooltip content="详情" :show-after="400">
                                    <el-button circle type="info" :icon="View" @click.stop="handleProd(item)" />
                                </el-tooltip>
                                <el-tooltip content="下载" :show-after="400">
                                    <el-button circle type="warning" :icon="DownloadIcon" @click.stop="openDownloadDialog(item)" />
                                </el-tooltip>
                            </div>
                        </div>

                        <div class="card-badges">
                            <el-tag v-if="item.type === 1013" type="warning" size="small" effect="dark">名家讲书</el-tag>
                            <el-tag v-if="item.is_group" type="info" size="small" effect="dark">分组</el-tag>
                        </div>
                    </div>

                    <div class="card-content">
                        <h3 class="card-title" :title="item.title">{{ item.title }}</h3>
                        <div class="card-meta">
                            <span class="meta-info">
                                <el-icon><Clock /></el-icon>
                                {{ secondToHour(item.duration) }}
                            </span>
                            <span v-if="item.is_group" class="meta-info">共 {{ item.course_num || 0 }} 本</span>
                        </div>
                        <p class="card-intro" v-if="item.intro">
                            {{ item.intro.length > 48 ? item.intro.substring(0, 48) + '...' : item.intro }}
                        </p>
                    </div>

                    <div v-if="!item.is_group" class="list-actions" @click.stop>
                        <el-tooltip content="播放" :show-after="400">
                            <el-button circle size="small" type="primary" :icon="VideoPlay" @click.stop="handlePlay(item)" />
                        </el-tooltip>
                        <el-tooltip content="文稿" :show-after="400">
                            <el-button circle size="small" type="success" :icon="Memo" @click.stop="gotoArticleDetail(item)" />
                        </el-tooltip>
                        <el-tooltip content="详情" :show-after="400">
                            <el-button circle size="small" type="info" :icon="View" @click.stop="handleProd(item)" />
                        </el-tooltip>
                        <el-tooltip content="下载" :show-after="400">
                            <el-button circle size="small" type="warning" :icon="DownloadIcon" @click.stop="openDownloadDialog(item)" />
                        </el-tooltip>
                    </div>
                </div>
            </div>

            <div v-else class="empty-state">
                <div class="empty-icon">
                    <el-icon><Headset /></el-icon>
                </div>
                <h3>听书内容暂时为空</h3>
                <p v-if="isLoggedIn">可以切换筛选条件，或稍后刷新列表。</p>
                <p v-else>登录后可同步听书书架与播放进度。</p>
                <div class="empty-actions">
                    <el-button v-if="!isLoggedIn" type="primary" round @click="pushLogin">立即登录</el-button>
                    <el-button round @click="refreshList">刷新</el-button>
                    <el-button round @click="goDownloadSetting">设置下载目录</el-button>
                </div>
            </div>
        </div>
    </div>

    <audio-info
        v-if="dialogVisible"
        :enid="prodEnid"
        :dialog-visible="dialogVisible"
        @close="closeDialog"
    />
    <outside-info
        v-if="outsideVisible"
        :enid="prodEnid"
        :dialog-visible="outsideVisible"
        @close="closeDialog"
    />

    <download-dialog
        v-if="dialogDownloadVisible"
        :dialog-visible="dialogDownloadVisible"
        :download-id="downloadId"
        :prod-type="3"
        :download-type-options="downloadTypeOptions"
        :download-data="downloadData"
        @close="closeDownloadDialog"
    />
</template>

<script lang="ts" setup>
import { onMounted, reactive, ref, computed } from 'vue'
import 'element-plus/es/components/message/style/css'
import { ElMessage } from 'element-plus'
import {
    ArrowLeft,
    VideoPlay,
    Memo,
    View,
    Download as DownloadIcon,
    Picture,
    Folder,
    Clock,
    RefreshRight,
    Headset,
} from '@element-plus/icons-vue'
import { CourseCategory, CourseGroupList, CourseList, SetDir, GetNavbar } from '../../wailsjs/go/backend/App'
import { services } from '../../wailsjs/go/models'
import AudioInfo from '../components/AudioInfo.vue'
import OutsideInfo from '../components/OutsideInfo.vue'
import DownloadDialog from '../components/DownloadDialog.vue'
import { userStore } from '../stores/user'
import { settingStore } from '../stores/setting'
import { Local } from '../utils/storage'
import { useAppRouter } from '../composables/useRouter'
import { secondToHour } from '../utils/utils'
import { playerStore, type PlayerTrack } from '../stores/player'
import { invokeBackend } from '../utils/backend'

const pStore = playerStore()

const store = userStore()
const setStore = settingStore()
const { pushOdobDetail, pushLogin, pushSetting } = useAppRouter()

const loading = ref(false)
const initLoading = ref(true)
const page = ref(1)
const total = ref(0)
const outerTotal = ref(0)
const pageSize = ref(20)
const lastPageSize = ref(20)
const dialogVisible = ref(false)
const outsideVisible = ref(false)
const prodEnid = ref("")
const filterOptions = ref<any[]>([])
const currentFilter = ref('all')
const viewMode = ref<'card' | 'list'>(Local.get('odob_view_mode') === 'list' ? 'list' : 'card')

const groupMode = reactive({
    active: false,
    groupId: 0,
    title: '',
})

const dialogDownloadVisible = ref(false)
const downloadId = ref(0)
let downloadData = reactive(new services.Course())
const downloadTypeOptions = [
    { value: 1, label: "MP3" },
    { value: 2, label: "PDF" },
    { value: 3, label: "Markdown" }
]

const aliasSrcCache = new Map<string, { src: string; poster?: string }>()
const aliasPending = new Map<string, Promise<{ src: string; poster?: string }>>()

let tableData = reactive(new services.CourseList())

const isLoggedIn = computed(() => Boolean(Local.get("cookies")))
const hasFilters = computed(() => filterOptions.value.length > 0)
const audioList = computed(() => tableData.list || [])
const normalTracks = computed(() => audioList.value.filter((item: any) => !item?.is_group))
const normalTrackCount = computed(() => normalTracks.value.length)
const groupTrackCount = computed(() => audioList.value.filter((item: any) => Boolean(item?.is_group)).length)
const totalDurationSeconds = computed(() => {
    return normalTracks.value.reduce((sum: number, item: any) => sum + Math.max(0, Number(item?.duration || 0)), 0)
})
const totalDurationLabel = computed(() => secondToHour(totalDurationSeconds.value))
const activeFilterName = computed(() => {
    if (groupMode.active) return "分组模式"
    if (currentFilter.value === 'all') return "全部"
    const match = filterOptions.value.find((item: any) => item.filter === currentFilter.value)
    return String(match?.name || "筛选中")
})
const nowPlayingTitle = computed(() => String(pStore.currentTrack?.title || "播放中").trim() || "播放中")
const listeningPace = computed(() => {
    const totalMinutes = Math.round(totalDurationSeconds.value / 60)
    if (totalMinutes === 0) return "待开始"
    if (totalMinutes < 180) return "轻量节奏"
    if (totalMinutes < 720) return "日常节奏"
    return "沉浸节奏"
})
const queueSummary = computed(() => {
    const queueSize = Number(pStore.queue.length || 0)
    if (queueSize > 0) return `已创建 ${queueSize} 条连播队列`
    return "点击「通勤快听」自动创建播放队列"
})

const setViewMode = (mode: 'card' | 'list') => {
    viewMode.value = mode
    Local.set('odob_view_mode', mode)
}

const resolveOdobSrc = async (aliasId: string) => {
    const key = String(aliasId || '').trim()
    if (!key) return { src: '' }
    const cached = aliasSrcCache.get(key)
    if (cached) return cached
    const pending = aliasPending.get(key)
    if (pending) return pending
    const p = invokeBackend<any>('AudioDetailAlias', key)
        .then((detail) => {
            const src = String(detail?.mp3_play_url ?? '').trim()
            const poster = String(detail?.icon ?? '').trim() || undefined
            const val = { src, poster }
            if (src) aliasSrcCache.set(key, val)
            return val
        })
        .finally(() => {
            aliasPending.delete(key)
        })
    aliasPending.set(key, p)
    return p
}

const buildTrack = (row: any): PlayerTrack | null => {
    const aliasId = String(row?.audio_detail?.alias_id ?? '').trim()
    if (!aliasId) return null
    const cached = aliasSrcCache.get(aliasId)
    return {
        id: `odob:${aliasId}`,
        title: String(row?.title ?? ''),
        src: cached?.src ?? '',
        poster: cached?.poster || row?.icon || row?.audio_detail?.icon,
    }
}

const noMore = computed(() => {
    const currentCount = audioList.value.length
    if (groupMode.active || currentFilter.value !== 'all') {
        const currentTotal = Number(tableData.total || 0)
        if (currentTotal > 0) return currentCount >= currentTotal
        if (tableData.is_more === 0) return true
        return lastPageSize.value < pageSize.value
    }
    if (outerTotal.value > 0) return currentCount >= outerTotal.value
    return lastPageSize.value < pageSize.value
})

const disabled = computed(() => loading.value || noMore.value)

const loadMore = () => {
    if (disabled.value) return
    page.value += 1
    getTableData(true)
}

const loadCategoryTotal = async () => {
    try {
        const result = await CourseCategory()
        result.forEach((item) => {
            if (item.category == "odob") {
                outerTotal.value = item.count
                if (!groupMode.active) total.value = item.count
            }
        })
    } catch (error: any) {
        const message = String(error || '')
        if (message.includes('401')) {
            store.user = null
            pushLogin()
        }
        Local.remove("cookies")
        Local.remove("userStore")
    }
}

const loadFilters = async () => {
    try {
        const res = await GetNavbar()
        const opts: any[] = []
        if (res && (res as any).list) {
            ;(res as any).list.forEach((item: any) => {
                if (item.category === "odob" && item.children) {
                    opts.push(...item.children)
                }
            })
        }
        filterOptions.value = opts
    } catch {
    }
}

const handleFilterChange = () => {
    if (groupMode.active) {
        groupMode.active = false
        groupMode.groupId = 0
        groupMode.title = ''
    }
    page.value = 1
    tableData.list = []
    getTableData()
}

const getTableData = async (append = false) => {
    loading.value = true
    if (!append) initLoading.value = true

    try {
        const table = groupMode.active
            ? await CourseGroupList("odob", "study", currentFilter.value, groupMode.groupId, page.value, pageSize.value)
            : await CourseList("odob", "study", currentFilter.value, page.value, pageSize.value)

        const fetchedList = table.list || []
        lastPageSize.value = fetchedList.length

        if (append) {
            if (fetchedList.length > 0) tableData.list.push(...fetchedList)
        } else {
            Object.assign(tableData, table)
        }

        total.value = (groupMode.active || currentFilter.value !== 'all')
            ? Number(table.total || 0)
            : outerTotal.value
    } catch (error: any) {
        const message = String(error || '')
        if (message.includes('401')) {
            store.user = null
            pushLogin()
        } else {
            ElMessage({ message, type: 'warning' })
        }
    } finally {
        loading.value = false
        initLoading.value = false
    }
}

const refreshList = () => {
    page.value = 1
    tableData.list = []
    getTableData()
}

const playFirstAvailable = () => {
    const first = normalTracks.value[0]
    if (!first) return
    handlePlay(first)
}

const openCurrentPlaylist = () => {
    if (pStore.hasTrack) {
        pStore.openPlaylist()
        return
    }
    playFirstAvailable()
}

const handlePlay = async (row: any) => {
    pStore.setContext({ key: 'odob:study', title: '每日听书' })
    const queue = audioList.value.map(buildTrack).filter((t): t is PlayerTrack => !!t)
    const target = buildTrack(row)
    if (!target) {
        ElMessage({ message: '该条目没有可播放的音频地址', type: 'warning' })
        return
    }
    const startIndex = queue.findIndex((t) => t.id === target.id)
    const idx = startIndex >= 0 ? startIndex : 0
    const aliasId = String(row?.audio_detail?.alias_id ?? '').trim()
    if (aliasId) {
        try {
            const { src, poster } = await resolveOdobSrc(aliasId)
            if (src) {
                queue[idx].src = src
                if (poster && !queue[idx].poster) queue[idx].poster = poster
            }
        } catch {
            ElMessage({ message: '获取音频播放地址失败', type: 'warning' })
        }
    }
    pStore.setQueue(queue, idx)
}

const closeDialog = () => {
    dialogVisible.value = false
    outsideVisible.value = false
}

const enterGroup = (row: any) => {
    const groupId = Number(row?.group_id || row?.id || 0)
    if (!groupId) return

    groupMode.active = true
    groupMode.groupId = groupId
    groupMode.title = String(row?.title || '')
    page.value = 1
    tableData.list = []
    getTableData()
}

const exitGroup = () => {
    groupMode.active = false
    groupMode.groupId = 0
    groupMode.title = ''
    page.value = 1
    total.value = outerTotal.value
    getTableData()
}

const goDownloadSetting = () => {
    pushSetting()
}

const openDownloadDialog = (row: any) => {
    downloadId.value = row.id
    dialogDownloadVisible.value = true
    Object.assign(downloadData, row)
    if (setStore.getDownloadDir == "") {
        ElMessage({
            message: '请设置文件保存目录',
            type: 'warning'
        })
        pushSetting()
    } else {
        SetDir([setStore.getDownloadDir, setStore.getFfmpegDirDir, setStore.getWkDir]).catch((error) => {
            ElMessage({
                message: error,
                type: 'warning'
            })
        })
    }
}

const closeDownloadDialog = () => {
    dialogDownloadVisible.value = false
}

const gotoArticleDetail = (row: any) => {
    const id = String(row?.audio_detail?.alias_id ?? '')
    if (!id) return
    pushOdobDetail(id)
}

const handleProd = (row: any) => {
    prodEnid.value = row.enid
    if (row.type === 1013) {
        outsideVisible.value = true
    } else {
        dialogVisible.value = true
    }
}

onMounted(async () => {
    await Promise.all([loadCategoryTotal(), loadFilters()])
    getTableData()
})
</script>

<style scoped>
.odob-container {
    --audio-accent: #0f8ea1;
    --audio-accent-strong: #0a6f80;
    --audio-soft-bg: #eaf9fc;
    --audio-chip-bg: #eefbff;
    --list-cover-size: 52px;
    --list-row-min-height: 78px;
    height: 100%;
    display: flex;
    flex-direction: column;
    gap: 16px;
    padding: 18px;
    box-sizing: border-box;
}

.hero-content {
    display: flex;
    flex-direction: column;
}

.audio-hero {
    display: grid;
    grid-template-columns: 1fr 320px;
    gap: 14px;
    padding: 20px;
    border-radius: 16px;
    border: 1px solid color-mix(in srgb, var(--border-soft) 82%, transparent);
    background:
        radial-gradient(360px 180px at 14% 0%, color-mix(in srgb, var(--audio-accent) 22%, transparent) 0%, transparent 72%),
        radial-gradient(260px 160px at 90% 0%, color-mix(in srgb, #49c7d8 20%, transparent) 0%, transparent 74%),
        color-mix(in srgb, var(--surface-glass) 74%, transparent);
    box-shadow: 0 12px 28px rgba(8, 18, 32, 0.08);
    backdrop-filter: blur(10px);
}

.hero-kicker {
    margin: 0;
    color: var(--audio-accent);
    font-size: 12px;
    letter-spacing: 0.14em;
    text-transform: uppercase;
    font-weight: 700;
}

.hero-title {
    margin: 8px 0 0;
    font-size: 30px;
    line-height: 1.15;
    color: var(--text-primary);
    font-family: var(--font-family-display);
}

.hero-subtitle {
    margin: 10px 0 0;
    max-width: 760px;
    min-height: 48px;
    color: var(--text-secondary);
    font-size: 14px;
    line-height: 1.7;
}

.hero-actions {
    margin-top: 16px;
    min-height: 40px;
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 10px;
}

.view-toggle {
    border-radius: 999px;
    border: 1px solid color-mix(in srgb, var(--border-soft) 76%, transparent);
    background: color-mix(in srgb, var(--card-bg) 90%, transparent);
    display: inline-flex;
    align-items: center;
    align-self: center;
    padding: 2px;
    line-height: 1;
}

.toggle-btn {
    height: 34px;
    border: 0;
    border-radius: 999px;
    padding: 0 16px;
    background: transparent;
    color: var(--text-secondary);
    font-size: 14px;
    font-weight: 600;
    cursor: pointer;
}

.toggle-btn.active {
    color: #fff;
    background: var(--accent-color);
}

.hero-actions :deep(.el-button) {
    height: 40px;
    padding: 0 22px;
    font-size: 14px;
    font-weight: 600;
    line-height: 1;
}

.now-playing-chip {
    margin-top: 10px;
    max-width: 520px;
    height: 34px;
    border-radius: 999px;
    display: inline-flex;
    align-items: center;
    gap: 8px;
    padding: 0 12px;
    background: color-mix(in srgb, var(--audio-accent) 14%, var(--card-bg) 86%);
    color: var(--text-primary);
}

.now-playing-chip span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

.hero-stats {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 10px;
    align-content: start;
}

.hero-wave {
    grid-column: 1 / -1;
    border-radius: 10px;
    border: 1px solid color-mix(in srgb, var(--audio-accent) 28%, transparent);
    background: color-mix(in srgb, var(--audio-soft-bg) 84%, transparent);
    height: 38px;
    display: grid;
    grid-template-columns: repeat(14, minmax(0, 1fr));
    gap: 5px;
    align-items: end;
    padding: 8px 10px;
}

.wave-bar {
    width: 100%;
    border-radius: 999px;
    background: linear-gradient(180deg, color-mix(in srgb, #5cd4e4 86%, #fff 14%) 0%, var(--audio-accent-strong) 100%);
    animation: audioWavePulse 1.25s ease-in-out infinite;
    transform-origin: center bottom;
}

@keyframes audioWavePulse {
    0%, 100% {
        height: 28%;
        opacity: 0.55;
    }
    50% {
        height: 100%;
        opacity: 0.95;
    }
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
    font-size: 18px;
    color: var(--audio-accent-strong);
    line-height: 1.2;
}

.audio-scenes {
    display: flex;
    align-items: stretch;
    gap: 10px;
    flex-wrap: wrap;
}

.scene-pill {
    border: 1px solid color-mix(in srgb, var(--audio-accent) 26%, transparent);
    background: var(--audio-chip-bg);
    color: var(--text-primary);
    border-radius: 999px;
    height: 40px;
    padding: 0 16px;
    font-size: 13px;
    font-weight: 600;
    cursor: pointer;
    transition: transform 0.2s ease, border-color 0.2s ease, box-shadow 0.2s ease;
}

.scene-pill:hover {
    transform: translateY(-1px);
    border-color: color-mix(in srgb, var(--audio-accent-strong) 46%, transparent);
    box-shadow: 0 8px 16px rgba(15, 142, 161, 0.16);
}

.scene-pill-primary {
    background: linear-gradient(120deg, color-mix(in srgb, var(--audio-accent) 78%, #fff 22%) 0%, var(--audio-accent-strong) 100%);
    color: #fff;
    border-color: transparent;
}

.scene-summary {
    margin-left: auto;
    min-width: 250px;
    border-radius: 12px;
    border: 1px dashed color-mix(in srgb, var(--audio-accent) 32%, transparent);
    background: color-mix(in srgb, var(--audio-soft-bg) 72%, transparent);
    padding: 8px 12px;
}

.scene-summary span {
    font-size: 11px;
    color: var(--text-secondary);
}

.scene-summary strong {
    display: block;
    margin-top: 2px;
    font-size: 16px;
    color: var(--audio-accent-strong);
}

.scene-summary p {
    margin: 2px 0 0;
    font-size: 12px;
    color: var(--text-secondary);
}

.group-header {
    display: flex;
    align-items: center;
    margin-top: -4px;
    gap: 12px;
}

.group-title {
    font-size: 16px;
    font-weight: 600;
    color: var(--text-primary);
}

.filter-container {
    overflow-x: auto;
    white-space: nowrap;
    padding-bottom: 6px;
}

.filter-container::-webkit-scrollbar {
    height: 4px;
}

.filter-container::-webkit-scrollbar-thumb {
    background: var(--border-soft);
    border-radius: 2px;
}

.odob-grid-container {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding-bottom: 20px;
    scrollbar-width: none;
    -ms-overflow-style: none;
}

.odob-grid-container::-webkit-scrollbar {
    display: none;
}

.odob-grid {
    display: grid;
    grid-template-columns: repeat(5, minmax(0, 1fr));
    gap: 16px;
    padding: 4px;
}

.odob-grid.list-mode {
    grid-template-columns: 1fr;
    gap: 10px;
}

.odob-grid.list-mode .odob-card {
    display: grid;
    grid-template-columns: var(--list-cover-size) minmax(0, 1fr) auto;
    align-items: center;
    min-height: var(--list-row-min-height);
    padding: 6px;
}

.odob-grid.list-mode .card-cover {
    width: var(--list-cover-size);
    height: var(--list-cover-size);
    aspect-ratio: 1;
    border-radius: 8px;
}

.odob-grid.list-mode .card-cover .card-overlay {
    display: none;
}

.odob-grid.list-mode .card-content {
    justify-content: flex-start;
    align-items: flex-start;
    text-align: left;
    padding: 4px 10px;
}

.odob-grid.list-mode .card-title {
    min-height: auto;
    -webkit-line-clamp: 1;
    margin-bottom: 1px;
    font-size: 13px;
    text-align: left;
}

.odob-grid.list-mode .card-meta {
    margin-bottom: 2px;
    font-size: 11px;
    text-align: left;
    justify-content: flex-start;
}

.odob-grid.list-mode .card-intro {
    font-size: 11px;
    line-height: 1.4;
    -webkit-line-clamp: 1;
    text-align: left;
}

.list-actions {
    display: none;
}

.odob-grid.list-mode .list-actions {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 0 10px;
    flex-shrink: 0;
}

.odob-grid.list-mode .list-actions .el-button {
    margin: 0;
}

.odob-card {
    background: color-mix(in srgb, var(--card-bg) 88%, transparent);
    border-radius: 14px;
    overflow: hidden;
    box-shadow: var(--shadow-soft);
    transition: transform 0.25s ease, box-shadow 0.25s ease, border-color 0.25s ease;
    position: relative;
    cursor: pointer;
    border: 1px solid color-mix(in srgb, var(--border-soft) 82%, transparent);
    display: flex;
    flex-direction: column;
}

.odob-card:hover {
    transform: translateY(-3px);
    box-shadow: var(--shadow-medium);
    border-color: color-mix(in srgb, var(--audio-accent) 34%, transparent);
}

.card-cover {
    position: relative;
    width: 100%;
    aspect-ratio: 1;
    background-color: var(--fill-color);
    overflow: hidden;
}

.card-cover .el-image {
    width: 100%;
    height: 100%;
    transition: transform 0.45s ease;
}

.odob-card:hover .card-cover .el-image {
    transform: scale(1.05);
}

.image-placeholder, .no-cover {
    width: 100%;
    height: 100%;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    color: var(--text-secondary);
    background-color: var(--fill-color);
}

.card-overlay {
    position: absolute;
    inset: 0;
    background: rgba(0, 0, 0, 0.56);
    display: flex;
    align-items: center;
    justify-content: center;
    opacity: 0;
    transition: opacity 0.25s ease;
    backdrop-filter: blur(4px);
}

.odob-card:hover .card-overlay {
    opacity: 1;
}

.overlay-actions {
    display: flex;
    gap: 10px;
    flex-wrap: wrap;
    justify-content: center;
    padding: 10px;
}

.overlay-actions .el-button {
    margin: 0 !important;
}

.card-badges {
    position: absolute;
    top: 10px;
    left: 10px;
    display: flex;
    flex-direction: column;
    gap: 6px;
    z-index: 2;
}

.group-cover-grid {
    position: absolute;
    inset: 0;
    display: grid;
    grid-template-columns: 1fr 1fr;
    grid-template-rows: 1fr 1fr;
    gap: 2px;
    background: var(--fill-color-light);
}

.group-grid-item {
    position: relative;
    overflow: hidden;
    background: var(--fill-color-light);
    width: 100%;
    height: 100%;
}

.grid-image {
    width: 100%;
    height: 100%;
    display: block;
}

.grid-placeholder {
    width: 100%;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--text-tertiary);
    background: var(--fill-color-light);
}

.bg-gray {
    background: var(--fill-color);
}

.card-content {
    padding: 14px;
    flex: 1;
    display: flex;
    flex-direction: column;
}

.card-title {
    font-size: 15px;
    font-weight: 600;
    color: var(--text-primary);
    margin: 0 0 8px;
    line-height: 1.4;
    display: -webkit-box;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 2;
    overflow: hidden;
    min-height: 42px;
}

.card-meta {
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-size: 12px;
    color: var(--text-secondary);
    margin-bottom: 8px;
}

.meta-info {
    display: inline-flex;
    align-items: center;
    gap: 4px;
}

.card-intro {
    font-size: 13px;
    color: var(--text-tertiary);
    line-height: 1.6;
    display: -webkit-box;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 2;
    overflow: hidden;
    margin: 0;
}

.empty-state {
    margin-top: 8px;
    min-height: 360px;
    border-radius: 16px;
    border: 1px dashed color-mix(in srgb, var(--border-soft) 72%, transparent);
    background: color-mix(in srgb, var(--card-bg) 86%, transparent);
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    text-align: center;
    padding: 24px;
}

.empty-icon {
    width: 60px;
    height: 60px;
    border-radius: 999px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    font-size: 26px;
    color: var(--audio-accent-strong);
    background: color-mix(in srgb, var(--audio-accent) 14%, transparent);
}

.empty-state h3 {
    margin: 14px 0 8px;
    font-size: 20px;
    color: var(--text-primary);
    font-family: var(--font-family-display);
}

.empty-state p {
    margin: 0;
    color: var(--text-secondary);
}

.empty-actions {
    margin-top: 16px;
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    gap: 10px;
}

@media (max-width: 1400px) {
    .odob-grid {
        grid-template-columns: repeat(4, minmax(0, 1fr));
    }
}

@media (max-width: 1180px) {
    .audio-hero {
        grid-template-columns: 1fr;
    }

    .hero-stats {
        grid-template-columns: repeat(4, minmax(0, 1fr));
    }

    .hero-wave {
        grid-column: 1 / -1;
    }

    .odob-grid {
        grid-template-columns: repeat(3, minmax(0, 1fr));
    }
}

@media (max-width: 880px) {
    .odob-container {
        padding: 12px;
    }

    .hero-title {
        font-size: 24px;
    }

    .hero-stats {
        grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .audio-scenes {
        flex-direction: column;
    }

    .scene-summary {
        margin-left: 0;
    }

    .odob-grid {
        grid-template-columns: repeat(2, minmax(0, 1fr));
        gap: 12px;
    }
}

@media (max-width: 620px) {
    .view-toggle {
        width: 100%;
        justify-content: center;
    }

    .hero-actions :deep(.el-button) {
        margin: 0;
    }

    .odob-grid {
        grid-template-columns: 1fr;
    }

    .odob-grid.list-mode .odob-card {
        grid-template-columns: 1fr;
        min-height: 0;
    }
}
</style>
