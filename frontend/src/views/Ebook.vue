<template>
  <div class="ebook-container">
    <section class="ebook-hero">
      <div class="hero-content">
        <p class="hero-kicker">Ebook Workspace</p>
        <h1 class="hero-title">{{ groupMode.active ? groupMode.title : "我的电子书学习台" }}</h1>
        <p class="hero-subtitle">
          {{ groupMode.active ? "分组内书籍已展开，可查看详情、书评与离线下载。" : "独立电子书板块，专注阅读学习与书评互动。" }}
        </p>

        <div class="hero-actions">
          <el-button type="primary" round @click="openFirstEbook">
            {{ normalBookCount > 0 ? "打开第一本" : "浏览书架" }}
          </el-button>
          <el-button round :icon="RefreshRight" @click="refreshList">刷新列表</el-button>
          <el-button round :icon="DownloadIcon" @click="goDownloadSetting">下载设置</el-button>
          <el-button round :icon="ChatDotRound" @click="openCommentSquare">书评广场</el-button>
          <div class="view-toggle">
            <button class="toggle-btn" :class="viewMode === 'card' ? 'active' : ''" @click="setViewMode('card')">卡片</button>
            <button class="toggle-btn" :class="viewMode === 'list' ? 'active' : ''" @click="setViewMode('list')">列表</button>
          </div>
        </div>
      </div>

      <div class="hero-stats">
        <article class="stat-card">
          <span>电子书数</span>
          <strong>{{ normalBookCount }}</strong>
        </article>
        <article class="stat-card">
          <span>分组数</span>
          <strong>{{ groupBookCount }}</strong>
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

    <section class="reading-strip">
      <article class="strip-card">
        <span>阅读建议</span>
        <strong>{{ readingSuggestion }}</strong>
      </article>
      <article class="strip-card">
        <span>阅读阶段</span>
        <strong>{{ readingStage }}</strong>
      </article>
      <article class="strip-card">
        <span>当前状态</span>
        <strong>{{ shelfStatus }}</strong>
      </article>
    </section>

    <div class="header-actions" v-if="groupMode.active">
      <el-button type="primary" link @click="exitGroup">
        <el-icon><ArrowLeft /></el-icon> 返回全部电子书
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
      class="ebook-grid-container"
      v-loading="initLoading"
      v-infinite-scroll="loadMore"
      :infinite-scroll-disabled="disabled"
      :infinite-scroll-immediate="false"
    >
      <div v-if="ebookList.length > 0" class="ebook-grid" :class="viewMode === 'list' ? 'list-mode' : ''">
        <div v-for="item in ebookList" :key="item.enid" class="ebook-card" @click="handleCardClick(item)">
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
              class="cover-image"
            >
              <template #placeholder>
                <div class="image-placeholder">
                  <el-icon><Picture /></el-icon>
                </div>
              </template>
            </el-image>
            <div v-else class="no-cover">
              <el-icon><Notebook /></el-icon>
            </div>

            <div class="card-badges">
              <div v-if="item.is_group" class="group-badge">
                <el-icon><Folder /></el-icon>
                <span>{{ item.course_num || 0 }}本</span>
              </div>
              <div v-if="!item.is_group && item.progress" class="progress-badge">
                {{ safePercent(item.progress) }}%
              </div>
            </div>

            <div class="card-overlay">
              <div class="overlay-actions">
                <template v-if="!item.is_group">
                  <el-tooltip content="阅读" :show-after="450">
                    <el-button circle size="small" type="success" @click.stop="openEbookRead(item)">
                      <el-icon><Reading /></el-icon>
                    </el-button>
                  </el-tooltip>
                  <el-tooltip content="详情" :show-after="450">
                    <el-button circle size="small" @click.stop="handleProd(item.enid)">
                      <el-icon><View /></el-icon>
                    </el-button>
                  </el-tooltip>
                  <el-tooltip content="下载" :show-after="450">
                    <el-button circle size="small" type="primary" @click.stop="openDownloadDialog(item)">
                      <el-icon><DownloadIcon /></el-icon>
                    </el-button>
                  </el-tooltip>
                  <el-tooltip content="书评" :show-after="450">
                    <el-button circle size="small" @click.stop="gotoCommentList(item)">
                      <el-icon><ChatDotRound /></el-icon>
                    </el-button>
                  </el-tooltip>
                  <el-tooltip content="移出书架" :show-after="450">
                    <el-button circle size="small" type="danger" @click.stop="ebookShelfRemove(item.enid)">
                      <el-icon><Delete /></el-icon>
                    </el-button>
                  </el-tooltip>
                </template>
                <template v-else>
                  <el-button type="primary" round size="small" @click.stop="enterGroup(item)">进入分组</el-button>
                </template>
              </div>
            </div>
          </div>

          <div class="card-info">
            <h3 class="card-title" :title="item.title">{{ item.title }}</h3>
            <p class="card-intro" :title="item.intro">{{ stripHtml(item.intro) }}</p>
            <div class="card-meta">
              <span v-if="item.price" class="price">¥{{ item.price }}</span>
              <el-tag v-if="item.is_group" size="small" effect="plain">分组</el-tag>
            </div>
          </div>

          <div v-if="!item.is_group" class="list-actions" @click.stop>
            <el-tooltip content="阅读" :show-after="450">
              <el-button circle size="small" type="success" @click.stop="openEbookRead(item)">
                <el-icon><Reading /></el-icon>
              </el-button>
            </el-tooltip>
            <el-tooltip content="详情" :show-after="450">
              <el-button circle size="small" @click.stop="handleProd(item.enid)">
                <el-icon><View /></el-icon>
              </el-button>
            </el-tooltip>
            <el-tooltip content="下载" :show-after="450">
              <el-button circle size="small" type="primary" @click.stop="openDownloadDialog(item)">
                <el-icon><DownloadIcon /></el-icon>
              </el-button>
            </el-tooltip>
            <el-tooltip content="书评" :show-after="450">
              <el-button circle size="small" @click.stop="gotoCommentList(item)">
                <el-icon><ChatDotRound /></el-icon>
              </el-button>
            </el-tooltip>
            <el-tooltip content="移出书架" :show-after="450">
              <el-button circle size="small" type="danger" @click.stop="ebookShelfRemove(item.enid)">
                <el-icon><Delete /></el-icon>
              </el-button>
            </el-tooltip>
          </div>
        </div>
      </div>

      <div v-else class="empty-state">
        <div class="empty-icon">
          <el-icon><Notebook /></el-icon>
        </div>
        <h3>电子书书架暂时为空</h3>
        <p v-if="isLoggedIn">可以切换筛选条件，或稍后刷新书架。</p>
        <p v-else>登录后可同步你的电子书书架与阅读进度。</p>
        <div class="empty-actions">
          <el-button v-if="!isLoggedIn" type="primary" round @click="pushLogin">立即登录</el-button>
          <el-button round @click="refreshList">刷新</el-button>
          <el-button round @click="goDownloadSetting">设置下载目录</el-button>
        </div>
      </div>
    </div>

    <ebook-info
      v-if="dialogVisible"
      :enid="prodEnid"
      :dialog-visible="dialogVisible"
      @close="closeDialog"
    />

    <download-dialog
      v-if="dialogDownloadVisible"
      :dialog-visible="dialogDownloadVisible"
      :download-type-options="downloadTypeOptions"
      :prod-type="2"
      :download-id="downloadId"
      :en-id="downloadEnId"
      @close="closeDownloadDialog"
    />
  </div>
</template>

<script lang="ts" setup>
import { onMounted, reactive, ref, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  ChatDotRound,
  View,
  Reading,
  Download as DownloadIcon,
  Delete,
  Picture,
  Notebook,
  Folder,
  ArrowLeft,
  RefreshRight,
} from '@element-plus/icons-vue'
import {
  CourseCategory,
  CourseList,
  CourseGroupList,
  EbookShelfRemove,
  SetDir,
  GetNavbar
} from '../../wailsjs/go/backend/App'
import { services } from '../../wailsjs/go/models'
import EbookInfo from '../components/EbookInfo.vue'
import DownloadDialog from "../components/DownloadDialog.vue"
import { useAppRouter } from '../composables/useRouter'
import { userStore } from '../stores/user'
import { settingStore } from "../stores/setting"
import { Local } from '../utils/storage'

const store = userStore()
const setStore = settingStore()
const { pushEbookComment, pushSetting, pushLogin, pushEbookReader } = useAppRouter()

const loading = ref(false)
const initLoading = ref(true)
const page = ref(1)
const total = ref(0)
const outerTotal = ref(0)
const pageSize = ref(20)
const lastPageSize = ref(20)
const dialogVisible = ref(false)
const prodEnid = ref("")
const filterOptions = ref<any[]>([])
const currentFilter = ref('all')
const viewMode = ref<'card' | 'list'>(Local.get('ebook_view_mode') === 'list' ? 'list' : 'card')

const groupMode = reactive({
  active: false,
  groupId: 0,
  title: '',
})

const dialogDownloadVisible = ref(false)
const downloadId = ref(0)
const downloadEnId = ref('')
const downloadTypeOptions = [
    { value: 1, label: "HTML" },
    { value: 2, label: "PDF" },
    { value: 3, label: "EPUB" }
]

let tableData = reactive(new services.CourseList())

const isLoggedIn = computed(() => Boolean(Local.get("cookies")))
const hasFilters = computed(() => filterOptions.value.length > 0)
const ebookList = computed(() => tableData.list || [])
const normalBooks = computed(() => ebookList.value.filter((item: any) => !item?.is_group))
const normalBookCount = computed(() => normalBooks.value.length)
const groupBookCount = computed(() => ebookList.value.filter((item: any) => Boolean(item?.is_group)).length)
const avgProgress = computed(() => {
  if (normalBooks.value.length === 0) return 0
  const sum = normalBooks.value.reduce((acc: number, item: any) => acc + safePercent(item?.progress), 0)
  return Math.round(sum / normalBooks.value.length)
})
const activeFilterName = computed(() => {
  if (groupMode.active) return "分组模式"
  if (currentFilter.value === 'all') return "全部"
  const match = filterOptions.value.find((item: any) => item.filter === currentFilter.value)
  return String(match?.name || "筛选中")
})
const readingStage = computed(() => {
  if (normalBookCount.value === 0) return "尚未开始"
  if (avgProgress.value < 20) return "起步阶段"
  if (avgProgress.value < 70) return "推进阶段"
  return "复盘阶段"
})
const readingSuggestion = computed(() => {
  if (!isLoggedIn.value) return "登录后同步进度，自动推荐下一章。"
  if (normalBookCount.value === 0) return "先浏览书架并收藏你要读的书。"
  if (avgProgress.value < 20) return "建议先读目录与序言，建立全书地图。"
  if (avgProgress.value < 70) return "保持章节推进，每次阅读 20 分钟更稳。"
  return "已接近完成，建议整理书摘与复盘笔记。"
})
const shelfStatus = computed(() => {
  if (groupMode.active) return `分组：${groupMode.title || "已选择"}`
  return `筛选：${activeFilterName.value}`
})

const safePercent = (val: any) => {
  const n = Number(val || 0)
  if (!Number.isFinite(n)) return 0
  return Math.max(0, Math.min(100, Math.round(n)))
}

const setViewMode = (mode: 'card' | 'list') => {
  viewMode.value = mode
  Local.set('ebook_view_mode', mode)
}

const noMore = computed(() => {
  const currentCount = ebookList.value.length
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
      if (item.category == "ebook") {
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
        if (item.category === "ebook" && item.children) {
          opts.push(...item.children)
        }
      })
    }
    filterOptions.value = opts
  } catch {
  }
}

const getTableData = async (append = false) => {
  loading.value = true
  if (!append) initLoading.value = true

  try {
    const table = groupMode.active
      ? await CourseGroupList("ebook", "study", currentFilter.value, groupMode.groupId, page.value, pageSize.value)
      : await CourseList("ebook", "study", currentFilter.value, page.value, pageSize.value)

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
      ElMessage({
        message,
        type: 'warning'
      })
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

const openFirstEbook = () => {
  const first = normalBooks.value[0]
  if (first) {
    openEbookRead(first)
    return
  }
  const groupFirst = ebookList.value.find((item: any) => item?.is_group)
  if (groupFirst) enterGroup(groupFirst)
}

const openCommentSquare = () => {
  pushEbookComment()
}

const handleProd = (enid: string) => {
  prodEnid.value = enid
  dialogVisible.value = true
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
  getTableData()
}

const handleCardClick = (item: any) => {
  if (item.is_group) {
    enterGroup(item)
  } else {
    handleProd(item.enid)
  }
}

const resolveEbookEnid = (row: any) => {
  const enid = String(row?.enid ?? '').trim()
  if (enid) return enid

  const raw = String(row?.dd_url ?? row?.ddurl ?? '').trim()
  if (!raw) return ''

  let full = raw
  if (raw.startsWith('//')) full = `https:${raw}`
  if (raw.startsWith('/')) full = `https://www.dedao.cn${raw}`
  if (!full.startsWith('http://') && !full.startsWith('https://')) {
    full = `https://www.dedao.cn/${full.replace(/^\/+/, '')}`
  }

  try {
    const url = new URL(full)
    return String(url.searchParams.get('id') || url.searchParams.get('enid') || '').trim()
  } catch {
    return ''
  }
}

const openEbookRead = (row: any) => {
  const enid = resolveEbookEnid(row)
  if (!enid) {
    ElMessage({
      message: "未获取到电子书标识，请先打开详情",
      type: "warning",
    })
    return
  }
  pushEbookReader(enid, {
    title: String(row?.title || '').trim(),
    from: 'ebook',
  })
}

const goDownloadSetting = () => {
  pushSetting()
}

const openDownloadDialog = (row: any) => {
  downloadId.value = row.id
  downloadEnId.value = row.enid

  if (setStore.getDownloadDir == "") {
    ElMessage({
      message: '请设置文件保存目录',
      type: 'warning'
    })
    pushSetting()
  } else {
    SetDir([setStore.getDownloadDir, setStore.getFfmpegDirDir, setStore.getWkDir]).then(() => {
      dialogDownloadVisible.value = true
    }).catch((error) => {
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

const gotoCommentList = (row: any) => {
  pushEbookComment({
    id: row.id,
    enid: row.enid,
    total: row.publish_num,
    title: row.title
  })
}

const ebookShelfRemove = (enid: string) => {
  ElMessageBox.confirm(
    '确定要将该电子书移出书架吗?',
    '提示',
    {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    }
  ).then(() => {
    EbookShelfRemove([enid]).then(() => {
      ElMessage({
        type: 'success',
        message: '移除成功',
      })
      refreshList()
    }).catch((err) => {
      ElMessage({
        type: 'error',
        message: err,
      })
    })
  })
}

const closeDialog = () => {
  dialogVisible.value = false
}

const stripHtml = (html: string) => {
  if (!html) return ''
  const tmp = document.createElement("div")
  tmp.innerHTML = html
  return tmp.textContent || tmp.innerText || ""
}

onMounted(async () => {
  await Promise.all([loadCategoryTotal(), loadFilters()])
  getTableData()
})
</script>

<style scoped>
.ebook-container {
  --ebook-accent: #a5622f;
  --ebook-accent-strong: #8d4a22;
  --ebook-paper: #fffaf2;
  --ebook-paper-soft: #f5ede2;
  --list-cover-size: 52px;
  --list-row-min-height: 78px;
  height: 100%;
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 18px;
  overflow: hidden;
  box-sizing: border-box;
}

.hero-content {
  display: flex;
  flex-direction: column;
}

.ebook-hero {
  display: grid;
  grid-template-columns: 1fr 320px;
  gap: 14px;
  padding: 20px;
  border-radius: 16px;
  border: 1px solid color-mix(in srgb, var(--border-soft) 82%, transparent);
  background:
    radial-gradient(360px 180px at 14% 0%, color-mix(in srgb, #f2d4b6 62%, transparent) 0%, transparent 72%),
    radial-gradient(280px 170px at 90% 0%, color-mix(in srgb, #f9efe1 76%, transparent) 0%, transparent 74%),
    color-mix(in srgb, var(--surface-glass) 74%, transparent);
  box-shadow: 0 12px 28px rgba(8, 18, 32, 0.08);
  backdrop-filter: blur(10px);
}

.hero-kicker {
  margin: 0;
  color: var(--ebook-accent);
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
  font-family: var(--font-family-wenkai);
}

.hero-subtitle {
  margin: 10px 0 0;
  min-height: 48px;
  color: var(--text-secondary);
  font-size: 14px;
  line-height: 1.7;
  max-width: 760px;
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
  color: var(--ebook-accent-strong);
  line-height: 1.2;
}

.reading-strip {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
}

.strip-card {
  border-radius: 12px;
  border: 1px dashed color-mix(in srgb, var(--ebook-accent) 28%, transparent);
  background:
    linear-gradient(160deg, color-mix(in srgb, var(--ebook-paper) 84%, transparent) 0%, color-mix(in srgb, var(--ebook-paper-soft) 74%, transparent) 100%);
  padding: 12px 14px;
  min-height: 64px;
}

.strip-card span {
  display: block;
  font-size: 11px;
  color: var(--text-secondary);
}

.strip-card strong {
  display: block;
  margin-top: 6px;
  color: var(--ebook-accent-strong);
  font-size: 15px;
  line-height: 1.45;
  font-family: var(--font-family-wenkai);
}

.header-actions {
  display: flex;
  align-items: center;
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

.ebook-grid-container {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding-bottom: 20px;
  scrollbar-width: none;
  -ms-overflow-style: none;
}

.ebook-grid-container::-webkit-scrollbar {
  display: none;
}

.ebook-grid {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 16px;
  padding: 4px;
  align-content: start;
}

.ebook-grid.list-mode {
  grid-template-columns: 1fr;
  gap: 10px;
}

.ebook-grid.list-mode .ebook-card {
  display: grid;
  grid-template-columns: var(--list-cover-size) minmax(0, 1fr) auto;
  align-items: center;
  min-height: var(--list-row-min-height);
  padding: 6px;
}

.ebook-grid.list-mode .card-cover {
  width: var(--list-cover-size);
  height: var(--list-cover-size);
  aspect-ratio: 1;
  border-radius: 8px;
}

.ebook-grid.list-mode .card-cover .card-overlay {
  display: none;
}

.ebook-grid.list-mode .card-info {
  justify-content: flex-start;
  align-items: flex-start;
  text-align: left;
  gap: 2px;
  padding: 4px 10px;
}

.ebook-grid.list-mode .card-title {
  -webkit-line-clamp: 1;
  font-size: 13px;
  margin-bottom: 0;
  text-align: left;
}

.ebook-grid.list-mode .card-intro {
  font-size: 11px;
  line-height: 1.4;
  margin-top: 0;
  -webkit-line-clamp: 1;
  text-align: left;
}

.ebook-grid.list-mode .card-meta {
  margin-top: 2px;
  font-size: 11px;
  text-align: left;
  justify-content: flex-start;
}

.list-actions {
  display: none;
}

.ebook-grid.list-mode .list-actions {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 0 10px;
  flex-shrink: 0;
}

.ebook-grid.list-mode .list-actions .el-button {
  margin: 0;
}

.ebook-card {
  background-color: color-mix(in srgb, var(--card-bg) 88%, transparent);
  border-radius: 14px;
  overflow: hidden;
  transition: transform 0.25s ease, box-shadow 0.25s ease, border-color 0.25s ease;
  border: 1px solid color-mix(in srgb, var(--border-soft) 82%, transparent);
  display: flex;
  flex-direction: column;
  cursor: pointer;
  position: relative;
}

.ebook-card:hover {
  transform: translateY(-3px);
  box-shadow: var(--shadow-medium);
  border-color: color-mix(in srgb, var(--ebook-accent) 36%, transparent);
}

.card-cover {
  position: relative;
  aspect-ratio: 2/3;
  overflow: hidden;
  background-color: var(--fill-color);
}

.cover-image {
  width: 100%;
  height: 100%;
  transition: transform 0.45s ease;
}

.ebook-card:hover .cover-image {
  transform: scale(1.05);
}

.no-cover {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 48px;
  color: var(--text-secondary);
}

.image-placeholder {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: var(--fill-color);
  color: var(--text-secondary);
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
  background-color: rgba(0, 0, 0, 0.6);
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
  background-color: var(--ebook-accent-strong);
  color: #fff;
  padding: 2px 8px;
  border-radius: 8px;
  font-size: 12px;
  font-weight: 600;
  box-shadow: 0 2px 4px rgba(0,0,0,0.1);
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

.card-overlay {
  position: absolute;
  inset: 0;
  background: rgba(0, 0, 0, 0.44);
  display: flex;
  align-items: center;
  justify-content: center;
  opacity: 0;
  transition: opacity 0.25s ease;
  backdrop-filter: blur(2px);
}

.ebook-card:hover .card-overlay {
  opacity: 1;
}

.overlay-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: center;
  padding: 0 12px;
}

.card-info {
  padding: 12px;
  flex: 1;
  display: flex;
  flex-direction: column;
}

.card-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 8px 0;
  font-family: var(--font-family-wenkai);
  line-height: 1.4;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.card-intro {
  font-size: 12px;
  color: var(--text-secondary);
  margin: 0 0 8px 0;
  line-height: 1.5;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  flex: 1;
}

.card-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: auto;
}

.price {
  font-size: 13px;
  color: var(--ebook-accent-strong);
  font-weight: 600;
  font-family: var(--font-family-mono);
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
  color: var(--ebook-accent-strong);
  background: color-mix(in srgb, var(--ebook-accent) 12%, transparent);
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
  .ebook-grid {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }
}

@media (max-width: 1180px) {
  .ebook-hero {
    grid-template-columns: 1fr;
  }

  .reading-strip {
    grid-template-columns: 1fr;
  }

  .hero-stats {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }

  .ebook-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 880px) {
  .ebook-container {
    padding: 12px;
  }

  .hero-title {
    font-size: 24px;
  }

  .hero-stats {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .ebook-grid {
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

  .ebook-grid {
    grid-template-columns: 1fr;
  }

  .ebook-grid.list-mode .ebook-card {
    grid-template-columns: 1fr;
    min-height: 0;
  }
}
</style>
