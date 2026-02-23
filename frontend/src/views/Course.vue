<template>
    <div class="course-container">
        <section class="learning-hero">
            <div class="hero-content">
                <p class="hero-kicker">Course Workspace</p>
                <h1 class="hero-title">{{ groupMode.active ? groupMode.title : "我的课程学习台" }}</h1>
                <p class="hero-subtitle">
                    {{ groupMode.active ? "分组内课程已展开，可继续学习或下载离线内容。" : "按进度管理课程，快速回到最近学习章节。" }}
                </p>

                <div class="hero-actions">
                    <el-button type="primary" round @click="continueLearning">
                        {{ normalCourseCount > 0 ? "继续学习" : "浏览课程" }}
                    </el-button>
                    <el-button round :icon="RefreshRight" @click="refreshList">刷新列表</el-button>
                    <el-button round :icon="Download" @click="goDownloadSetting">下载设置</el-button>
                </div>
            </div>

            <div class="hero-stats">
                <article class="stat-card">
                    <span>课程数</span>
                    <strong>{{ normalCourseCount }}</strong>
                </article>
                <article class="stat-card">
                    <span>分组数</span>
                    <strong>{{ groupCourseCount }}</strong>
                </article>
                <article class="stat-card">
                    <span>平均进度</span>
                    <strong>{{ avgProgress }}%</strong>
                </article>
                <article class="stat-card">
                    <span>筛选条件</span>
                    <strong>{{ activeFilterName }}</strong>
                </article>
            </div>
        </section>

        <div v-if="groupMode.active" class="group-header">
            <el-button type="primary" link @click="exitGroup">
                <el-icon class="el-icon--left"><ArrowLeft /></el-icon>返回全部课程
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
            class="course-grid-container"
            v-infinite-scroll="loadMore"
            :infinite-scroll-disabled="disabled"
            :infinite-scroll-immediate="false"
        >
            <div v-if="courseList.length > 0" class="course-grid">
                <div
                    v-for="item in courseList"
                    :key="item.id"
                    class="course-card"
                    @click="item.is_group ? enterGroup(item) : gotoArticleList(item)"
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
                                <el-button circle type="primary" :icon="View" @click="handleProd(item.enid)" title="详情" />
                                <el-button circle type="success" :icon="Download" @click="openDownloadDialog(item)" title="下载" />
                            </div>
                        </div>

                        <div class="card-badges">
                            <span v-if="item.is_group" class="group-badge">
                                <el-icon><Folder /></el-icon>
                                <span>分组</span>
                            </span>
                            <span v-else-if="(item.progress || 0) >= 80" class="progress-badge">高进度</span>
                        </div>
                    </div>

                    <div class="card-content">
                        <h3 class="card-title" :title="item.title">{{ item.title }}</h3>
                        <div class="card-meta">
                            <span v-if="item.is_group" class="meta-info">{{ item.course_num || 0 }} 本课程</span>
                            <span v-else class="meta-info">已更 {{ item.publish_num || 0 }}/{{ item.course_num || 0 }}</span>

                            <span v-if="!item.is_group" class="meta-progress">{{ safePercent(item.progress) }}%</span>
                        </div>
                        <div v-if="!item.is_group" class="progress-bar">
                             <div class="progress-fill" :style="{ width: safePercent(item.progress) + '%' }"></div>
                        </div>
                    </div>
                </div>
            </div>

            <div v-else class="empty-state">
                <div class="empty-icon">
                    <el-icon><Compass /></el-icon>
                </div>
                <h3>课程列表暂时为空</h3>
                <p v-if="isLoggedIn">可以切换筛选条件，或稍后刷新列表。</p>
                <p v-else>登录后可同步你的已购课程与学习进度。</p>
                <div class="empty-actions">
                    <el-button v-if="!isLoggedIn" type="primary" round @click="pushLogin">立即登录</el-button>
                    <el-button round @click="refreshList">刷新</el-button>
                    <el-button round @click="goDownloadSetting">设置下载目录</el-button>
                </div>
            </div>
        </div>
    </div>

    <course-info
        v-if="dialogVisible"
        :enid="prodEnid"
        :dialog-visible="dialogVisible"
        @close="closeDialog"
    />
    <download-dialog
        v-if="dialogDownloadVisible"
        :dialog-visible="dialogDownloadVisible"
        :download-type-options="downloadTypeOptions"
        :prod-type="66"
        :download-id="downloadId"
        :en-id="downloadEnId"
        @close="closeDownloadDialog"
    />
</template>

<script lang="ts" setup>
import { ref, reactive, onMounted, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { ArrowLeft, View, Download, Picture, Folder, RefreshRight, Compass } from '@element-plus/icons-vue'
import { CourseList, CourseCategory, CourseGroupList, SetDir, GetNavbar } from '../../wailsjs/go/backend/App'
import { services } from '../../wailsjs/go/models'
import { userStore } from '../stores/user'
import { settingStore } from '../stores/setting'
import { useAppRouter } from '../composables/useRouter'
import CourseInfo from '../components/CourseInfo.vue'
import DownloadDialog from "../components/DownloadDialog.vue"
import { Local } from '../utils/storage'

const store = userStore()
const setStore = settingStore()
const { pushLogin, pushCourseDetail, pushSetting } = useAppRouter()

const loading = ref(false)
const initLoading = ref(true)
const page = ref(1)
const total = ref(0)
const outerTotal = ref(0)
const pageSize = ref(20)
const lastPageSize = ref(20)
const currentFilter = ref('all')
const filterOptions = ref<any[]>([])

const dialogVisible = ref(false)
const prodEnid = ref("")

const groupMode = reactive({
    active: false,
    groupId: 0,
    title: '',
})

const dialogDownloadVisible = ref(false)
const downloadId = ref(0)
const downloadEnId = ref('')
const downloadTypeOptions = [
    { value: 1, label: "MP3" },
    { value: 2, label: "PDF" },
    { value: 3, label: "Markdown" },
    { value: 4, label: "MP4" },
]

let tableData = reactive(new services.CourseList())

const isLoggedIn = computed(() => Boolean(Local.get("cookies")))
const hasFilters = computed(() => filterOptions.value.length > 0)
const courseList = computed(() => tableData.list || [])
const normalCourses = computed(() => courseList.value.filter((item: any) => !item?.is_group))
const normalCourseCount = computed(() => normalCourses.value.length)
const groupCourseCount = computed(() => courseList.value.filter((item: any) => Boolean(item?.is_group)).length)
const avgProgress = computed(() => {
    if (normalCourses.value.length === 0) return 0
    const sum = normalCourses.value.reduce((acc: number, item: any) => acc + safePercent(item?.progress), 0)
    return Math.round(sum / normalCourses.value.length)
})
const activeFilterName = computed(() => {
    if (groupMode.active) return "分组模式"
    if (currentFilter.value === 'all') return "全部"
    const match = filterOptions.value.find((item: any) => item.filter === currentFilter.value)
    return String(match?.name || "筛选中")
})

const safePercent = (val: any) => {
    const n = Number(val || 0)
    if (!Number.isFinite(n)) return 0
    return Math.max(0, Math.min(100, Math.round(n)))
}

const noMore = computed(() => {
    const currentCount = courseList.value.length
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

const loadFilters = async () => {
    try {
        const res = await GetNavbar()
        const opts: any[] = []
        if (res && (res as any).list) {
            ;(res as any).list.forEach((item: any) => {
                if (item.category === "bauhinia" && item.children) {
                    opts.push(...item.children)
                }
            })
        }
        filterOptions.value = opts
    } catch {
    }
}

const loadCategoryTotal = async () => {
    try {
        const result = await CourseCategory()
        result.forEach((item) => {
            if (item.category == "bauhinia") {
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

const handleFilterChange = () => {
    if (currentFilter.value === "all") {
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
            ? await CourseGroupList("bauhinia", "study", currentFilter.value, groupMode.groupId, page.value, pageSize.value)
            : await CourseList("bauhinia", "study", currentFilter.value, page.value, pageSize.value)

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

const continueLearning = () => {
    const target = [...normalCourses.value]
        .sort((a: any, b: any) => safePercent(b?.progress) - safePercent(a?.progress))[0]
    if (target) {
        gotoArticleList(target)
        return
    }
    const first = courseList.value[0]
    if (first?.is_group) {
        enterGroup(first)
        return
    }
    if (first) gotoArticleList(first)
}

const goDownloadSetting = () => {
    pushSetting()
}

const handleProd = (enid: string) => {
    prodEnid.value = enid
    dialogVisible.value = true
}

const gotoArticleList = (row: any) => {
    pushCourseDetail(row.id, {
        enid: row.enid,
        total: row.publish_num,
        title: row.title
    })
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

const closeDialog = () => {
    dialogVisible.value = false
}

const openDownloadDialog = (row: any) => {
    downloadId.value = row.class_id
    downloadEnId.value = row.enid
    dialogDownloadVisible.value = true
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

onMounted(async () => {
    await Promise.all([loadFilters(), loadCategoryTotal()])
    getTableData()
})
</script>

<style scoped>
.course-container {
    height: 100%;
    display: flex;
    flex-direction: column;
    gap: 16px;
    padding: 18px;
    box-sizing: border-box;
}

.learning-hero {
    position: relative;
    display: grid;
    grid-template-columns: 1fr 320px;
    gap: 14px;
    padding: 20px;
    border-radius: 16px;
    border: 1px solid color-mix(in srgb, var(--border-soft) 82%, transparent);
    background:
        radial-gradient(360px 180px at 14% 0%, color-mix(in srgb, var(--accent-color) 12%, transparent) 0%, transparent 72%),
        radial-gradient(260px 160px at 90% 0%, color-mix(in srgb, var(--primary-color) 20%, transparent) 0%, transparent 74%),
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
    font-size: 30px;
    line-height: 1.15;
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
    font-size: 18px;
    color: var(--text-primary);
    line-height: 1.2;
}

.group-header {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-top: -4px;
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
    background: var(--border-color);
    border-radius: 2px;
}

.course-grid-container {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding-bottom: 20px;
    scrollbar-width: none;
    -ms-overflow-style: none;
}

.course-grid-container::-webkit-scrollbar {
    display: none;
}

.course-grid {
    display: grid;
    grid-template-columns: repeat(5, minmax(0, 1fr));
    gap: 16px;
    padding: 4px;
}

.course-card {
    background: color-mix(in srgb, var(--card-bg) 88%, transparent);
    border-radius: 14px;
    box-shadow: var(--shadow-soft);
    overflow: hidden;
    transition: transform 0.25s ease, box-shadow 0.25s ease, border-color 0.25s ease;
    cursor: pointer;
    position: relative;
    border: 1px solid color-mix(in srgb, var(--border-soft) 82%, transparent);
    display: flex;
    flex-direction: column;
}

.course-card:hover {
    transform: translateY(-3px);
    box-shadow: var(--shadow-medium);
    border-color: color-mix(in srgb, var(--accent-color) 26%, transparent);
}

.card-cover {
    position: relative;
    width: 100%;
    aspect-ratio: 1;
    background-color: var(--fill-color-light);
    overflow: hidden;
}

.card-cover .el-image {
    width: 100%;
    height: 100%;
    display: block;
    transition: transform 0.45s ease;
}

.course-card:hover .card-cover .el-image {
    transform: scale(1.05);
}

.image-placeholder,
.no-cover {
    width: 100%;
    height: 100%;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    color: var(--text-tertiary);
    gap: 8px;
}

.card-overlay {
    position: absolute;
    inset: 0;
    background: rgba(0, 0, 0, 0.42);
    display: flex;
    align-items: center;
    justify-content: center;
    opacity: 0;
    transition: opacity 0.25s ease;
    backdrop-filter: blur(2px);
}

.course-card:hover .card-overlay {
    opacity: 1;
}

.overlay-actions {
    display: flex;
    gap: 14px;
    transform: translateY(8px);
    transition: transform 0.25s ease;
}

.course-card:hover .overlay-actions {
    transform: translateY(0);
}

.card-badges {
    position: absolute;
    top: 8px;
    left: 8px;
    right: 8px;
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    pointer-events: none;
}

.group-badge {
    background: rgba(0, 0, 0, 0.6);
    color: #fff;
    padding: 4px 8px;
    border-radius: 8px;
    font-size: 12px;
    display: inline-flex;
    align-items: center;
    gap: 4px;
    backdrop-filter: blur(4px);
}

.progress-badge {
    margin-left: auto;
    background: color-mix(in srgb, var(--accent-color) 80%, #fff 20%);
    color: #fff;
    padding: 4px 8px;
    border-radius: 8px;
    font-size: 12px;
}

.group-cover-grid {
    width: 100%;
    height: 100%;
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
    padding: 12px 14px 14px;
    flex: 1;
    display: flex;
    flex-direction: column;
}

.card-title {
    margin: 0 0 8px;
    font-size: 15px;
    line-height: 1.4;
    color: var(--text-primary);
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
    font-weight: 600;
}

.card-meta {
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-size: 12px;
    color: var(--text-secondary);
    margin-bottom: 8px;
}

.meta-progress {
    color: var(--accent-color);
    font-weight: 600;
}

.progress-bar {
    height: 5px;
    background: color-mix(in srgb, var(--fill-color) 84%, transparent);
    border-radius: 4px;
    overflow: hidden;
}

.progress-fill {
    height: 100%;
    background: linear-gradient(90deg, var(--accent-color) 0%, color-mix(in srgb, var(--accent-color) 60%, var(--primary-color) 40%) 100%);
    border-radius: 4px;
    transition: width 0.3s ease;
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
    color: var(--accent-color);
    background: color-mix(in srgb, var(--accent-color) 12%, transparent);
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
    .course-grid {
        grid-template-columns: repeat(4, minmax(0, 1fr));
    }
}

@media (max-width: 1180px) {
    .learning-hero {
        grid-template-columns: 1fr;
    }

    .hero-stats {
        grid-template-columns: repeat(4, minmax(0, 1fr));
    }

    .course-grid {
        grid-template-columns: repeat(3, minmax(0, 1fr));
    }
}

@media (max-width: 880px) {
    .course-container {
        padding: 12px;
    }

    .hero-title {
        font-size: 24px;
    }

    .hero-stats {
        grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .course-grid {
        grid-template-columns: repeat(2, minmax(0, 1fr));
        gap: 12px;
    }
}

@media (max-width: 620px) {
    .hero-actions :deep(.el-button) {
        margin: 0;
    }

    .course-grid {
        grid-template-columns: 1fr;
    }
}
</style>
