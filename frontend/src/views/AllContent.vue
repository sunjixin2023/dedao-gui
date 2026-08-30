<template>
  <div class="all-content-page">
    <section class="hero">
      <div class="hero-main">
        <p class="kicker">All Content</p>
        <h1>得到内容总览</h1>
        <p>聚合课程、听书、电子书与训练营内容，支持关键词检索和类型筛选。</p>
      </div>
      <div class="hero-stats">
        <article>
          <span>已加载</span>
          <strong>{{ loadedCount }}</strong>
        </article>
        <article>
          <span>筛选后</span>
          <strong>{{ filteredCount }}</strong>
        </article>
        <article>
          <span>总量</span>
          <strong>{{ total }}</strong>
        </article>
      </div>
    </section>

    <section class="filters">
      <div class="search-row">
        <el-input
          v-model="keyword"
          clearable
          placeholder="搜索节目/课程名，如：文明之旅、长谈、大望局、得到头条"
          @keyup.enter="runSearch"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>
        <el-button type="primary" :loading="searchLoading" @click="runSearch">搜索</el-button>
        <el-button :icon="RefreshRight" @click="refreshList">刷新</el-button>
      </div>
      <div class="quick-row">
        <el-button
          v-for="word in quickKeywords"
          :key="word"
          size="small"
          text
          @click="applyQuickKeyword(word)"
        >
          {{ word }}
        </el-button>
      </div>
      <div class="type-row">
        <el-radio-group v-model="typeFilter" size="small">
          <el-radio-button label="all">全部</el-radio-button>
          <el-radio-button label="program">节目/课程</el-radio-button>
          <el-radio-button label="ebook">电子书</el-radio-button>
          <el-radio-button label="odob">听书</el-radio-button>
          <el-radio-button label="institute">研修班</el-radio-button>
          <el-radio-button label="trainingcamp">训练营</el-radio-button>
          <el-radio-button label="compass">锦囊</el-radio-button>
          <el-radio-button label="group">分组</el-radio-button>
          <el-radio-button label="other">其他</el-radio-button>
        </el-radio-group>
      </div>
    </section>

    <section
      v-loading="initLoading"
      class="list"
      v-infinite-scroll="loadMore"
      :infinite-scroll-disabled="disabled"
      :infinite-scroll-immediate="false"
    >
      <div v-if="filteredList.length > 0" class="grid">
        <article
          v-for="item in filteredList"
          :key="`${item.id}-${item.enid}`"
          class="card"
          @click="openItem(item)"
        >
          <img v-if="item.icon" :src="item.icon" :alt="item.title" class="cover" loading="lazy" />
          <div v-else class="cover placeholder">
            <el-icon><Document /></el-icon>
          </div>

          <div class="content">
            <div class="title-row">
              <h3>{{ item.title }}</h3>
              <el-tag size="small" effect="plain">{{ typeLabel(item) }}</el-tag>
            </div>
            <p class="intro">{{ introText(item.intro) }}</p>
            <div class="meta">
              <span v-if="item.author">{{ item.author }}</span>
              <span v-else>type={{ item.type }}</span>
              <span>{{ item.progress || 0 }}%</span>
            </div>
            <div class="actions" @click.stop>
              <el-button size="small" type="primary" @click="openItem(item)">打开</el-button>
              <el-button size="small" @click="openDetail(item)">详情</el-button>
              <el-button
                v-if="externalURL(item)"
                size="small"
                :icon="Link"
                @click="openExternalItem(item)"
              >
                官网
              </el-button>
            </div>
          </div>
        </article>
      </div>

      <div v-else-if="!loading && !initLoading" class="empty">
        <p>当前条件下没有找到内容</p>
        <el-button text type="primary" @click="typeFilter = 'all'">清空类型筛选</el-button>
      </div>
    </section>

    <ebook-info
      v-if="ebookVisible"
      :enid="prodEnid"
      :dialog-visible="ebookVisible"
      @close="closeDialog"
    />
    <course-info
      v-if="courseVisible"
      :enid="prodEnid"
      :dialog-visible="courseVisible"
      @close="closeDialog"
    />
    <audio-info
      v-if="audioVisible"
      :enid="prodEnid"
      :dialog-visible="audioVisible"
      @close="closeDialog"
    />
    <outside-info
      v-if="outsideVisible"
      :enid="prodEnid"
      :dialog-visible="outsideVisible"
      @close="closeDialog"
    />
  </div>
</template>

<script lang="ts" setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { Document, Link, RefreshRight, Search } from '@element-plus/icons-vue'
import { CourseList, SearchMoreContent } from '../../wailsjs/go/backend/App'
import { services } from '../../wailsjs/go/models'
import { ROUTE_NAMES } from '../router/routes'
import { useAppRouter } from '../composables/useRouter'
import EbookInfo from '../components/EbookInfo.vue'
import CourseInfo from '../components/CourseInfo.vue'
import AudioInfo from '../components/AudioInfo.vue'
import OutsideInfo from '../components/OutsideInfo.vue'

const { pushByName, pushEbookReader, pushOdobDetail, openExternalUrl, openDedaoArticle } = useAppRouter()

const keyword = ref('')
const typeFilter = ref('all')
const loading = ref(false)
const initLoading = ref(true)
const searchLoading = ref(false)
const page = ref(1)
const total = ref(0)
const pageSize = ref(60)
const lastPageSize = ref(0)
const quickKeywords = ['文明之旅', '长谈', '大望局', '得到头条']

const ebookVisible = ref(false)
const courseVisible = ref(false)
const audioVisible = ref(false)
const outsideVisible = ref(false)
const prodEnid = ref('')
const extraSearchList = ref<any[]>([])

const tableData = reactive(new services.CourseList())
tableData.list = []

const loadedList = computed(() => tableData.list || [])
const loadedCount = computed(() => loadedList.value.length)
const mergedList = computed(() => {
  const map = new Map<string, any>()
  const append = (item: any) => {
    const key = String(item?.enid || item?.dd_url || item?.id || item?.title || '').trim()
    if (!key) return
    if (!map.has(key)) map.set(key, item)
  }

  loadedList.value.forEach(append)
  if (normalize(keyword.value)) {
    extraSearchList.value.forEach(append)
  }
  return Array.from(map.values())
})

const isProgramType = (item: any) => {
  const t = Number(item?.type || 0)
  return t === 4 || t === 22 || t === 36 || t === 66 || t === 65 || t === 67
}

const isEbookType = (item: any) => Number(item?.type || 0) === 2
const isOdobType = (item: any) => {
  const t = Number(item?.type || 0)
  return t === 13 || t === 1013
}
const isInstituteType = (item: any) => Number(item?.type || 0) === 510
const isTrainingcampType = (item: any) => Number(item?.type || 0) === 310
const isCompassType = (item: any) => Number(item?.type || 0) === 131

const normalize = (v: any) => String(v || '').trim().toLowerCase()

const matchedKeyword = (item: any) => {
  const key = normalize(keyword.value)
  if (!key) return true
  const title = normalize(item?.title)
  const intro = normalize(item?.intro)
  const author = normalize(item?.author)
  return title.includes(key) || intro.includes(key) || author.includes(key)
}

const matchedType = (item: any) => {
  const current = typeFilter.value
  if (current === 'all') return true
  if (current === 'program') return isProgramType(item)
  if (current === 'ebook') return isEbookType(item)
  if (current === 'odob') return isOdobType(item)
  if (current === 'institute') return isInstituteType(item)
  if (current === 'trainingcamp') return isTrainingcampType(item)
  if (current === 'compass') return isCompassType(item)
  if (current === 'group') return Boolean(item?.is_group)
  if (current === 'other') {
    return !Boolean(item?.is_group) &&
      !isProgramType(item) &&
      !isEbookType(item) &&
      !isOdobType(item) &&
      !isInstituteType(item) &&
      !isTrainingcampType(item) &&
      !isCompassType(item)
  }
  return true
}

const filteredList = computed(() =>
  mergedList.value.filter((item: any) => matchedType(item) && matchedKeyword(item)),
)

const filteredCount = computed(() => filteredList.value.length)

const noMore = computed(() => {
  if (total.value > 0) return loadedCount.value >= total.value
  return lastPageSize.value < pageSize.value
})

const disabled = computed(() => loading.value || noMore.value)

const introText = (text: string) => {
  const t = String(text || '').replace(/\s+/g, ' ').trim()
  if (!t) return '暂无简介'
  return t.length > 80 ? `${t.slice(0, 80)}...` : t
}

const normalizeEnidByURL = (raw: string) => {
  const source = String(raw || '').trim()
  if (!source) return ''
  let full = source
  if (full.startsWith('//')) full = `https:${full}`
  if (full.startsWith('/')) full = `https://www.dedao.cn${full}`
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

const resolveEnid = (item: any) => {
  const enid = String(item?.enid || '').trim()
  if (enid) return enid
  const fromURL = normalizeEnidByURL(String(item?.dd_url || ''))
  if (fromURL) return fromURL
  return normalizeEnidByURL(String(item?.dd_ext_url || ''))
}

const externalURL = (item: any) => {
  const raw = String(item?.dd_url || item?.dd_ext_url || '').trim()
  if (!raw) return ''
  if (raw.startsWith('http://') || raw.startsWith('https://')) return raw
  if (raw.startsWith('//')) return `https:${raw}`
  if (raw.startsWith('/')) return `https://www.dedao.cn${raw}`
  return `https://www.dedao.cn/${raw.replace(/^\/+/, '')}`
}

const typeLabel = (item: any) => {
  if (item?.is_group) return '分组'
  if (isEbookType(item)) return '电子书'
  if (Number(item?.type || 0) === 65 || Number(item?.type || 0) === 67) return '文稿'
  if (Number(item?.type || 0) === 6301) return '话题'
  if (Number(item?.type || 0) === 1013) return '名家讲书'
  if (Number(item?.type || 0) === 13) return '听书'
  if (isInstituteType(item)) return '研修班'
  if (isTrainingcampType(item)) return '训练营'
  if (isCompassType(item)) return '锦囊'
  if (isProgramType(item)) return '节目/课程'
  return `类型${item?.type ?? '-'}`
}

const openExternalItem = (item: any) => {
  const url = externalURL(item)
  if (!url) return
  openExternalUrl(url)
}

const openItem = (item: any) => {
  if (item?.is_group) {
    ElMessage({ message: '这是分组条目，请到课程/听书/电子书页面查看分组内容', type: 'info' })
    return
  }

  const type = Number(item?.type || 0)
  const enid = resolveEnid(item)
  if (type === 2) {
    if (!enid) {
      ElMessage({ message: '未找到电子书标识', type: 'warning' })
      return
    }
    pushEbookReader(enid, { title: String(item?.title || ''), from: 'all' })
    return
  }

  if (type === 13) {
    const alias = String(item?.audio_detail?.alias_id || '').trim()
    if (alias) {
      pushOdobDetail(alias)
      return
    }
    if (enid) {
      prodEnid.value = enid
      audioVisible.value = true
      return
    }
  }

  if (type === 1013 && enid) {
    prodEnid.value = enid
    outsideVisible.value = true
    return
  }

  if ((type === 65 || type === 67) && enid) {
    openDedaoArticle(enid)
    return
  }

  if (enid) {
    pushByName(ROUTE_NAMES.ARTICLE_LIST, { id: enid }, { enid, title: String(item?.title || ''), from: 'all' })
    return
  }

  if (externalURL(item)) {
    openExternalItem(item)
    return
  }

  ElMessage({ message: '未找到可打开的内容地址', type: 'warning' })
}

const openDetail = (item: any) => {
  const type = Number(item?.type || 0)
  const enid = resolveEnid(item)
  if (!enid) {
    ElMessage({ message: '未找到内容标识，无法打开详情', type: 'warning' })
    return
  }
  prodEnid.value = enid
  ebookVisible.value = type === 2
  outsideVisible.value = type === 1013
  audioVisible.value = type === 13
  courseVisible.value = !ebookVisible.value && !outsideVisible.value && !audioVisible.value
}

const closeDialog = () => {
  ebookVisible.value = false
  courseVisible.value = false
  audioVisible.value = false
  outsideVisible.value = false
}

const getTableData = async (append = false) => {
  if (loading.value) return
  loading.value = true
  if (!append) initLoading.value = true
  try {
    const table = await CourseList('all', 'study', 'all', page.value, pageSize.value)
    const list = table.list || []
    lastPageSize.value = list.length
    total.value = Number(table.total || 0)
    if (append) {
      if (list.length > 0) tableData.list.push(...list)
    } else {
      Object.assign(tableData, table)
    }
  } catch (error: any) {
    ElMessage({ message: String(error || '加载内容失败'), type: 'warning' })
  } finally {
    loading.value = false
    initLoading.value = false
  }
}

const loadMore = () => {
  if (disabled.value) return
  page.value += 1
  void getTableData(true)
}

const refreshList = () => {
  page.value = 1
  tableData.list = []
  extraSearchList.value = []
  void getTableData()
}

const applyQuickKeyword = (word: string) => {
  keyword.value = word
  void runSearch()
}

const loadExtraSearch = async (key: string) => {
  const searchKey = String(key || '').trim()
  if (!searchKey) {
    extraSearchList.value = []
    return
  }
  try {
    const table = await SearchMoreContent(searchKey, 1, 80)
    extraSearchList.value = table?.list || []
  } catch {
    extraSearchList.value = []
  }
}

const runSearch = async () => {
  const key = String(keyword.value || '').trim()
  if (!key) return
  searchLoading.value = true
  try {
    await loadExtraSearch(key)
    while (filteredList.value.length === 0 && !noMore.value) {
      page.value += 1
      // eslint-disable-next-line no-await-in-loop
      await getTableData(true)
    }
    if (filteredList.value.length === 0) {
      ElMessage({ message: '未检索到匹配内容，可尝试更换关键词', type: 'info' })
    }
  } finally {
    searchLoading.value = false
  }
}

watch(keyword, (value) => {
  if (!String(value || '').trim()) {
    extraSearchList.value = []
  }
})

onMounted(() => {
  void getTableData()
})
</script>

<style scoped>
.all-content-page {
  height: 100%;
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 16px;
  box-sizing: border-box;
}

.hero {
  display: grid;
  grid-template-columns: 1fr 320px;
  gap: 12px;
  padding: 16px;
  border: 1px solid var(--border-soft);
  border-radius: 14px;
  background: color-mix(in srgb, var(--card-bg) 88%, transparent);
}

.hero-main h1 {
  margin: 4px 0 0;
  font-size: 28px;
}

.hero-main p {
  margin: 8px 0 0;
  color: var(--text-secondary);
}

.kicker {
  margin: 0;
  color: var(--accent-color);
  font-size: 12px;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.hero-stats {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
}

.hero-stats article {
  border: 1px solid var(--border-soft);
  border-radius: 10px;
  padding: 10px;
  background: color-mix(in srgb, var(--fill-color-light) 76%, transparent);
}

.hero-stats span {
  display: block;
  font-size: 12px;
  color: var(--text-tertiary);
}

.hero-stats strong {
  display: block;
  margin-top: 4px;
  font-size: 18px;
  color: var(--text-primary);
}

.filters {
  border: 1px solid var(--border-soft);
  border-radius: 14px;
  padding: 12px;
  background: color-mix(in srgb, var(--card-bg) 90%, transparent);
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.search-row {
  display: grid;
  grid-template-columns: 1fr auto auto;
  gap: 8px;
}

.quick-row {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.type-row {
  overflow-x: auto;
}

.list {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding-bottom: 18px;
}

.grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.card {
  border: 1px solid var(--border-soft);
  border-radius: 12px;
  background: color-mix(in srgb, var(--card-bg) 92%, transparent);
  overflow: hidden;
  display: grid;
  grid-template-columns: 88px 1fr;
  gap: 10px;
  padding: 10px;
  cursor: pointer;
}

.cover {
  width: 88px;
  height: 120px;
  border-radius: 8px;
  object-fit: cover;
  background: var(--fill-color-light);
}

.cover.placeholder {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: var(--text-tertiary);
}

.content {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.title-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
}

.title-row h3 {
  margin: 0;
  font-size: 14px;
  line-height: 1.45;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.intro {
  margin: 0;
  color: var(--text-secondary);
  font-size: 12px;
  line-height: 1.5;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.meta {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  color: var(--text-tertiary);
  font-size: 12px;
}

.actions {
  margin-top: auto;
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.empty {
  min-height: 220px;
  border: 1px dashed var(--border-soft);
  border-radius: 12px;
  display: grid;
  place-content: center;
  text-align: center;
  color: var(--text-secondary);
}

@media (max-width: 1400px) {
  .grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 1080px) {
  .hero {
    grid-template-columns: 1fr;
  }

  .grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 740px) {
  .search-row {
    grid-template-columns: 1fr;
  }

  .grid {
    grid-template-columns: 1fr;
  }
}
</style>
