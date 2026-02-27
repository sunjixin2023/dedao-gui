<template>
    <div ref="pageRef" class="article-page" @scroll.passive="handlePageScroll">
        <section class="article-hero">
            <div class="hero-left">
                <el-breadcrumb separator="/">
                    <el-breadcrumb-item :to="parentPath">{{ breadcrumbItem1 }}</el-breadcrumb-item>
                    <el-breadcrumb-item>文稿学习</el-breadcrumb-item>
                </el-breadcrumb>

                <h1 class="article-title" :title="articleTitle">{{ articleTitle }}</h1>
                <div class="article-meta">
                    <el-tag v-if="hasAudio" type="success" effect="light" size="small">音频可学</el-tag>
                    <el-tag v-if="hasVideo" type="danger" effect="light" size="small">视频可学</el-tag>
                    <el-tag type="info" effect="light" size="small">文稿可读</el-tag>
                    <span class="meta-text">约 {{ readMinutes }} 分钟阅读</span>
                    <span class="meta-text">{{ readChars }} 字</span>
                </div>
            </div>

            <div class="hero-right">
                <el-button type="primary" @click="startAudioLearning" :disabled="!hasAudio">
                    <el-icon><Headset /></el-icon>
                    听音频
                </el-button>
                <el-button type="danger" plain @click="startVideoLearning" :disabled="!hasVideo">
                    <el-icon><VideoPlay /></el-icon>
                    看视频
                </el-button>
                <el-button plain @click="openPlaylist" :disabled="!pStore.hasTrack">
                    <el-icon><List /></el-icon>
                    播放列表
                </el-button>
            </div>
        </section>

        <section class="reader-toolbar">
            <div class="toolbar-left">
                <el-progress :percentage="scrollPercent" :stroke-width="8" :show-text="false" />
                <span class="progress-text">阅读进度 {{ scrollPercent }}%</span>
            </div>
            <div class="toolbar-right">
                <span class="label">字号</span>
                <el-slider v-model="fontSize" :min="14" :max="24" :step="1" class="slider" />
                <span class="value">{{ fontSize }}</span>
                <span class="label">行距</span>
                <el-slider v-model="lineHeight" :min="1.5" :max="2.2" :step="0.1" class="slider" />
                <span class="value">{{ lineHeight.toFixed(1) }}</span>
                <el-button size="small" plain @click="toggleFocusMode">{{ focusMode ? '退出专注' : '专注模式' }}</el-button>
            </div>
        </section>

        <section class="learning-grid" :class="focusMode ? 'focus' : ''">
            <section
                class="markdown-shell"
                :style="{
                    '--reader-font-size': `${fontSize}px`,
                    '--reader-line-height': String(lineHeight),
                }"
            >
                <el-skeleton v-if="loading" :rows="16" animated />
                <div v-else class="markdown-body">
                    <div v-highlight v-html="htmlStr" id="content"></div>
                </div>
            </section>

            <aside v-if="toc.length > 0" class="toc-shell">
                <div class="toc-card">
                    <div class="toc-header">
                        <span class="toc-title">目录导航</span>
                        <el-tag size="small" type="info" effect="plain">{{ toc.length }}</el-tag>
                    </div>
                    <div class="toc-list">
                        <button
                            v-for="item in toc"
                            :key="item.id"
                            class="toc-item"
                            :class="[activeHeadingId === item.id ? 'active' : '', `l${item.level}`]"
                            @click="jumpToHeading(item.id)"
                            :title="item.text"
                        >
                            {{ item.text }}
                        </button>
                    </div>
                </div>

                <div class="quick-card">
                    <div class="quick-title">快速操作</div>
                    <div class="quick-actions">
                        <el-button size="small" plain @click="scrollToTop">
                            <el-icon><Top /></el-icon>
                            回到顶部
                        </el-button>
                        <el-button size="small" plain @click="scrollToBottom">
                            <el-icon><Bottom /></el-icon>
                            跳到底部
                        </el-button>
                    </div>
                </div>
            </aside>
        </section>
    </div>
</template>

<script lang="ts" setup>
import { computed, nextTick, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import 'element-plus/es/components/message/style/css'
import { ElMessage } from 'element-plus'
import { ArticleDetail, GetArticleIntro } from '../../wailsjs/go/backend/App'
import { useRoute } from 'vue-router'
import { ROUTE_NAMES } from '../router/routes'
import { marked } from 'marked'
import { useAppRouter } from '../composables/useRouter'
import { playerStore, type PlayerTrack } from '../stores/player'
import { Local } from '../utils/storage'
import { learningStore } from '../stores/learning'
import { Bottom, Headset, List, Top, VideoPlay } from '@element-plus/icons-vue'

type HeadingItem = {
    id: string
    text: string
    level: number
}

const route = useRoute()
const { pushVideo } = useAppRouter()
const pStore = playerStore()
const lStore = learningStore()

marked.setOptions({
    pedantic: false,
    gfm: true,
    breaks: false,
    sanitize: false,
    smartLists: true,
    smartypants: false,
    xhtml: false,
})

const pageRef = ref<HTMLElement | null>(null)
const id = ref('')
const classId = ref('')
const from = ref('')
const breadcrumbItem1 = ref('课程')
const articleTitle = ref('文稿')
const markdownStr = ref('')
const htmlStr = ref('')
const loading = ref(false)
const aType = ref(1)
const scrollPercent = ref(0)
const articleIntro = ref<any | null>(null)
const toc = ref<HeadingItem[]>([])
const activeHeadingId = ref('')
const focusMode = ref(Boolean(Local.get('reader_focus_mode')))

const fontSize = ref(Number(Local.get('reader_font_size')) || 17)
const lineHeight = ref(Number(Local.get('reader_line_height')) || 1.9)

const parentPath = reactive<{ name: string; params: Record<string, string>; query: Record<string, any> }>({
    name: '',
    params: {},
    query: {},
})

let saveTimer: ReturnType<typeof setTimeout> | null = null

watch(fontSize, (v) => Local.set('reader_font_size', v))
watch(lineHeight, (v) => Local.set('reader_line_height', v))
watch(focusMode, (v) => Local.set('reader_focus_mode', v))

const readChars = computed(() => {
    return markdownStr.value.replace(/[#>*`~_[\]()/!+\-|\\\r\n]/g, '').replace(/\s+/g, '').length
})

const readMinutes = computed(() => {
    if (!readChars.value) return 0
    return Math.max(1, Math.ceil(readChars.value / 380))
})

const audioSource = computed(() => String(articleIntro.value?.audio?.mp3_play_url ?? '').trim())
const audioAlias = computed(() => String(articleIntro.value?.audio_alias_ids?.[0] ?? id.value).trim())
const hasAudio = computed(() => Boolean(audioSource.value || audioAlias.value))

const pickVideoMedia = (list: any[] | undefined) => {
    if (!list || list.length === 0) return null
    return list.find((m) => Number(m?.media_type) === 2) ?? list[0]
}

const currentVideoMedia = computed(() => pickVideoMedia(articleIntro.value?.media_base_info))
const hasVideo = computed(() => {
    const media = currentVideoMedia.value
    if (!media) return false
    return Boolean(String(media?.source_id ?? '').trim() && String(media?.security_token ?? '').trim())
})

const normalizeHeadingText = (text: string) => {
    return text.replace(/[`*_~\[\]()<>]/g, '').trim()
}

const slugifyHeading = (text: string, index: number) => {
    const slug = normalizeHeadingText(text)
        .toLowerCase()
        .replace(/[^a-z0-9\u4e00-\u9fa5]+/g, '-')
        .replace(/^-+|-+$/g, '')
    return `h-${index}-${slug || 'section'}`
}

const extractTocFromMarkdown = (md: string) => {
    const headings: HeadingItem[] = []
    const lines = String(md || '').split('\n')
    let idx = 0

    for (const line of lines) {
        const m = line.match(/^(#{1,4})\s+(.+)$/)
        if (!m) continue
        const level = m[1].length
        const text = normalizeHeadingText(m[2])
        if (!text) continue
        idx += 1
        headings.push({
            id: slugifyHeading(text, idx),
            text,
            level,
        })
    }

    return headings
}

const decorateHtmlHeadings = (html: string, headings: HeadingItem[]) => {
    let out = String(html || '')
    for (const heading of headings) {
        const openTag = `<h${heading.level}>`
        out = out.replace(openTag, `<h${heading.level} id="${heading.id}" class="reader-heading">`)
    }
    return out
}

const getScrollKey = () => `reader_scroll_${aType.value}_${id.value}`

const saveLearningSession = () => {
    const el = pageRef.value
    if (!el || !id.value) return

    Local.set(getScrollKey(), Math.round(el.scrollTop || 0))
    lStore.saveLastArticle({
        path: route.fullPath,
        title: articleTitle.value || '继续学习',
        progress: scrollPercent.value,
        updatedAt: Date.now(),
    })
}

const scheduleSaveLearningSession = () => {
    if (saveTimer) clearTimeout(saveTimer)
    saveTimer = setTimeout(() => {
        saveLearningSession()
    }, 120)
}

const updateActiveHeading = () => {
    if (!toc.value.length) {
        activeHeadingId.value = ''
        return
    }

    const container = pageRef.value
    if (!container) return
    const containerTop = container.getBoundingClientRect().top
    let active = toc.value[0].id

    for (const heading of toc.value) {
        const el = document.getElementById(heading.id)
        if (!el) continue
        const delta = el.getBoundingClientRect().top - containerTop
        if (delta <= 140) {
            active = heading.id
        } else {
            break
        }
    }

    activeHeadingId.value = active
}

const updateScrollProgress = () => {
    const el = pageRef.value
    if (!el) return
    const maxScroll = el.scrollHeight - el.clientHeight
    if (maxScroll <= 0) {
        scrollPercent.value = 100
        updateActiveHeading()
        return
    }
    scrollPercent.value = Math.max(0, Math.min(100, Math.round((el.scrollTop / maxScroll) * 100)))
    updateActiveHeading()
}

const restoreScrollPosition = async () => {
    await nextTick()
    const el = pageRef.value
    if (!el) return
    const savedTop = Number(Local.get(getScrollKey()) || 0)
    if (savedTop > 0) {
        el.scrollTop = savedTop
    }
    updateScrollProgress()
}

const resolvePageMeta = () => {
    from.value = String(route.query.from ?? '')
    id.value = String(route.params.id ?? '')
    classId.value = String(route.query.class_id ?? '')
    articleTitle.value = String(route.query.title ?? '文稿')

    if (from.value === 'course') {
        aType.value = 1
        parentPath.name = ROUTE_NAMES.ARTICLE_LIST
        parentPath.params = { id: String(classId.value) }
        parentPath.query = {
            total: route.query.total,
            enid: route.query.class_enid,
            title: route.query.parentTitle,
        }
        breadcrumbItem1.value = String(route.query.parentTitle ?? '课程列表')
        return
    }

    if (from.value === 'odob') {
        aType.value = 2
        parentPath.name = ROUTE_NAMES.ODOB
        parentPath.params = {}
        parentPath.query = {}
        breadcrumbItem1.value = '听书书架'
        return
    }

    if (from.value === 'aiChannel') {
        aType.value = 1
        parentPath.name = ROUTE_NAMES.AI_CHANNEL
        parentPath.params = {}
        parentPath.query = {}
        breadcrumbItem1.value = String(route.query.parentTitle ?? 'AI学习圈')
        return
    }

    if (from.value === 'live') {
        aType.value = 1
        parentPath.name = ROUTE_NAMES.LIVE
        parentPath.params = {}
        parentPath.query = {}
        breadcrumbItem1.value = String(route.query.parentTitle ?? '直播')
        return
    }

    aType.value = 1
    parentPath.name = ROUTE_NAMES.COURSE
    parentPath.params = {}
    parentPath.query = {}
    breadcrumbItem1.value = '课程列表'
}

const articleDetail = async () => {
    if (!id.value) return
    loading.value = true
    scrollPercent.value = 0
    toc.value = []
    activeHeadingId.value = ''

    try {
        const [md, intro] = await Promise.all([
            ArticleDetail(aType.value, id.value),
            GetArticleIntro(aType.value, id.value).catch(() => null),
        ])
        markdownStr.value = md || ''
        const headings = extractTocFromMarkdown(markdownStr.value)
        toc.value = headings
        const parsedHtml = (marked(markdownStr.value) as string) || ''
        htmlStr.value = decorateHtmlHeadings(parsedHtml, headings)
        articleIntro.value = intro
        if (intro?.title) {
            articleTitle.value = String(intro.title)
        }
        await restoreScrollPosition()
        saveLearningSession()
    } catch (error) {
        ElMessage({
            message: String(error),
            type: 'warning',
        })
    } finally {
        loading.value = false
    }
}

const getArticlePoster = () => {
    return (
        String(articleIntro.value?.logo ?? '').trim() ||
        String(articleIntro.value?.audio?.icon ?? '').trim() ||
        String(articleIntro.value?.video?.[0]?.cover_img ?? '').trim() ||
        undefined
    )
}

const startAudioLearning = () => {
    if (!hasAudio.value) {
        ElMessage({ message: '当前文稿没有可用音频', type: 'warning' })
        return
    }

    const title = articleTitle.value || '音频学习'
    const trackIdBase = `${from.value || 'article'}:${id.value}`
    const src = audioSource.value
    const poster = getArticlePoster()
    let contextKey = `article:${trackIdBase}`
    let trackId = trackIdBase

    const track: PlayerTrack = {
        id: trackId,
        title,
        src,
        poster,
    }

    if (!src && audioAlias.value) {
        contextKey = 'odob:study'
        trackId = `odob:${audioAlias.value}`
        track.id = trackId
        track.src = ''
    }

    pStore.setContext({ key: contextKey, title: articleTitle.value })
    pStore.setQueue([track], 0)
}

const startVideoLearning = () => {
    const media = currentVideoMedia.value
    const mediaId = String(media?.source_id ?? '').trim()
    const securityToken = String(media?.security_token ?? '').trim()
    if (!mediaId || !securityToken) {
        ElMessage({ message: '当前文稿未找到可播放的视频', type: 'warning' })
        return
    }

    pushVideo({
        from: from.value || 'course',
        media_id: mediaId,
        security_token: securityToken,
        title: articleTitle.value,
        parentTitle: breadcrumbItem1.value,
    })
}

const openPlaylist = () => {
    if (!pStore.hasTrack) return
    pStore.openPlaylist()
}

const jumpToHeading = (headingId: string) => {
    const container = pageRef.value
    const el = document.getElementById(headingId)
    if (!container || !el) return

    const containerTop = container.getBoundingClientRect().top
    const targetTop = el.getBoundingClientRect().top
    const delta = targetTop - containerTop
    container.scrollTo({
        top: container.scrollTop + delta - 110,
        behavior: 'smooth',
    })
    activeHeadingId.value = headingId
}

const scrollToTop = () => {
    const container = pageRef.value
    if (!container) return
    container.scrollTo({ top: 0, behavior: 'smooth' })
}

const scrollToBottom = () => {
    const container = pageRef.value
    if (!container) return
    container.scrollTo({ top: container.scrollHeight, behavior: 'smooth' })
}

const toggleFocusMode = () => {
    focusMode.value = !focusMode.value
}

const handlePageScroll = () => {
    updateScrollProgress()
    scheduleSaveLearningSession()
}

watch(
    () => [route.query.from, route.params.id, route.query.class_id, route.query.title, route.query.parentTitle],
    async () => {
        resolvePageMeta()
        await articleDetail()
    },
    { immediate: true },
)

onMounted(() => {
    window.addEventListener('resize', updateActiveHeading)
})

onUnmounted(() => {
    window.removeEventListener('resize', updateActiveHeading)
    if (saveTimer) {
        clearTimeout(saveTimer)
        saveTimer = null
    }
    saveLearningSession()
})
</script>

<style scoped lang="scss">
.article-page {
    --article-shell-gutter: clamp(10px, 2vw, 28px);
    --article-shell-max: calc(100% - var(--article-shell-gutter) * 2);
    --article-aside-width: clamp(220px, 15vw, 300px);
    padding: clamp(12px, 1.6vw, 24px);
    background: radial-gradient(circle at right top, #eaf5ff 0%, var(--fill-color-light) 40%, var(--bg-color) 100%);
    width: 100%;
    height: calc(100vh - 60px);
    overflow-y: auto;
    box-sizing: border-box;
    scrollbar-width: none;
    -ms-overflow-style: none;
}

.article-page::-webkit-scrollbar {
    display: none;
}

.article-hero {
    width: min(100%, var(--article-shell-max));
    max-width: none;
    margin: 0 auto;
    padding: 20px 24px;
    border-radius: 16px;
    background: var(--card-bg);
    border: 1px solid var(--border-soft);
    box-shadow: var(--shadow-soft);
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 20px;
}

.hero-left {
    min-width: 0;
    flex: 1;
}

.article-title {
    margin: 12px 0 12px;
    color: var(--text-primary);
    font-size: 28px;
    line-height: 1.25;
    font-weight: 700;
}

.article-meta {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
}

.meta-text {
    font-size: 13px;
    color: var(--text-secondary);
}

.hero-right {
    display: flex;
    align-items: center;
    gap: 10px;
    flex-wrap: wrap;
    justify-content: flex-end;
}

.reader-toolbar {
    width: min(100%, var(--article-shell-max));
    max-width: none;
    margin: 14px auto 0;
    border-radius: 14px;
    background: color-mix(in srgb, var(--card-bg) 92%, white 8%);
    border: 1px solid var(--border-soft);
    box-shadow: var(--shadow-soft);
    padding: 12px 16px;
    display: flex;
    gap: 16px;
    align-items: center;
    justify-content: space-between;
    position: sticky;
    top: 0;
    z-index: 8;
}

.toolbar-left {
    min-width: 240px;
    flex: 0 1 320px;
    display: flex;
    align-items: center;
    gap: 10px;
}

.progress-text {
    font-size: 13px;
    color: var(--text-secondary);
    white-space: nowrap;
}

.toolbar-right {
    min-width: 420px;
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 8px;
}

.label {
    font-size: 12px;
    color: var(--text-secondary);
}

.value {
    width: 26px;
    text-align: center;
    font-size: 12px;
    color: var(--text-primary);
}

.slider {
    width: 120px;
}

.learning-grid {
    width: min(100%, var(--article-shell-max));
    max-width: none;
    margin: 14px auto 0;
    display: grid;
    grid-template-columns: minmax(0, 1fr) var(--article-aside-width);
    gap: 16px;
    align-items: start;
}

.learning-grid.focus {
    grid-template-columns: minmax(0, 1fr);
}

.learning-grid.focus .toc-shell {
    display: none;
}

.markdown-shell {
    min-width: 0;
    width: 100%;
}

.markdown-body {
    color: var(--text-primary);
    text-align: left;
    width: 100%;
    max-width: none;
    margin: 0;
    background: var(--card-bg);
    padding: clamp(18px, 2vw, 36px);
    border-radius: 16px;
    border: 1px solid var(--border-soft);
    box-shadow: var(--shadow-soft);
}

.toc-shell {
    position: sticky;
    top: 86px;
    display: flex;
    flex-direction: column;
    gap: 12px;
}

.toc-card,
.quick-card {
    background: var(--card-bg);
    border: 1px solid var(--border-soft);
    border-radius: 14px;
    box-shadow: var(--shadow-soft);
    padding: 12px;
}

.toc-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 8px;
}

.toc-title,
.quick-title {
    font-size: 13px;
    font-weight: 600;
    color: var(--text-primary);
}

.toc-list {
    display: flex;
    flex-direction: column;
    gap: 4px;
    max-height: 420px;
    overflow-y: auto;
}

.toc-list::-webkit-scrollbar {
    width: 0;
    height: 0;
}

.toc-item {
    width: 100%;
    text-align: left;
    border: none;
    background: transparent;
    border-radius: 8px;
    padding: 7px 8px;
    color: var(--text-secondary);
    cursor: pointer;
    font-size: 12px;
    line-height: 1.45;
    transition: all 0.2s ease;
}

.toc-item:hover {
    background: var(--fill-color-light);
    color: var(--text-primary);
}

.toc-item.active {
    background: color-mix(in srgb, var(--accent-color) 12%, white 88%);
    color: var(--accent-color);
    font-weight: 600;
}

.toc-item.l2 {
    padding-left: 14px;
}

.toc-item.l3,
.toc-item.l4 {
    padding-left: 20px;
}

.quick-actions {
    margin-top: 8px;
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 8px;
}

:deep(#content) {
    font-size: var(--reader-font-size, 17px);
    line-height: var(--reader-line-height, 1.9);
    width: 100%;
    max-width: none;
}

:deep(#content > *),
:deep(#content div),
:deep(#content section),
:deep(#content article),
:deep(#content table) {
    max-width: 100% !important;
}

:deep(#content div),
:deep(#content section),
:deep(#content article),
:deep(#content p) {
    width: auto !important;
}

:deep(#content h1),
:deep(#content h2),
:deep(#content h3),
:deep(#content h4),
:deep(#content h5),
:deep(#content h6) {
    color: var(--text-primary);
    margin-top: 26px;
    margin-bottom: 16px;
    font-weight: 700;
    scroll-margin-top: 120px;
}

:deep(#content h2 > code) {
    background-color: var(--accent-color);
    padding: 2px 6px;
    border-radius: 4px;
    color: #fff;
}

:deep(#content p) {
    color: var(--text-primary);
    margin-bottom: 16px;
}

:deep(#content p > em) {
    font-style: normal;
    color: var(--accent-color);
}

:deep(#content blockquote) {
    border-left: 4px solid var(--accent-color);
    background: var(--fill-color);
    padding: 16px;
    margin: 16px 0;
    color: var(--text-secondary);
    border-radius: 0 8px 8px 0;
}

:deep(#content ul),
:deep(#content ol) {
    padding-left: 24px;
    margin-bottom: 16px;
    color: var(--text-primary);
}

:deep(#content li) {
    margin-bottom: 8px;
}

:deep(#content img) {
    max-width: 100%;
    border-radius: 10px;
    margin: 18px 0;
}

:deep(#content hr) {
    border: none;
    border-top: 1px solid var(--border-soft);
    margin: 24px 0;
}

@media (max-width: 1120px) {
    .article-page {
        padding: 14px;
    }

    .article-hero {
        flex-direction: column;
        padding: 16px;
    }

    .article-title {
        font-size: 22px;
    }

    .hero-right {
        width: 100%;
        justify-content: flex-start;
    }

    .reader-toolbar {
        position: static;
        flex-direction: column;
        align-items: stretch;
        gap: 12px;
    }

    .toolbar-left,
    .toolbar-right {
        min-width: 0;
        width: 100%;
        justify-content: flex-start;
    }

    .learning-grid {
        grid-template-columns: 1fr;
    }

    .toc-shell {
        position: static;
        order: -1;
    }

    .slider {
        width: 100px;
    }

    .markdown-body {
        padding: 20px;
    }
}
</style>
