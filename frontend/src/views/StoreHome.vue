<template>
  <div class="store-page">
    <section class="store-hero">
      <div class="hero-main">
        <p class="hero-kicker">Store Workspace</p>
        <h1 class="hero-title">内容商店</h1>
        <p class="hero-subtitle">
          先把推荐内容、搜索结果、购买跳转和订单记录聚到桌面端，作为交易闭环的第一版入口。
        </p>

        <div class="hero-actions">
          <el-button type="primary" round @click="refreshAll" :loading="loading">刷新推荐</el-button>
          <el-button round @click="pushStoreOrders()">订单中心</el-button>
          <el-button round @click="pushStoreMembership">会员中心</el-button>
          <el-button round @click="pushAllContent">全部内容</el-button>
        </div>
      </div>

      <div class="hero-stats">
        <article class="stat-card">
          <span>推荐内容</span>
          <strong>{{ mergedBaseItems.length }}</strong>
        </article>
        <article class="stat-card">
          <span>搜索结果</span>
          <strong>{{ searchItems.length }}</strong>
        </article>
        <article class="stat-card">
          <span>跳转订单</span>
          <strong>{{ cStore.orderCount }}</strong>
        </article>
        <article class="stat-card">
          <span>我的可学</span>
          <strong>{{ boughtItems.length }}</strong>
        </article>
      </div>
    </section>

    <section class="store-toolbar">
      <div class="search-row">
        <el-input
          v-model="keyword"
          clearable
          placeholder="搜索课程、电子书、听书、锦囊"
          @keyup.enter="runSearch"
          @clear="clearSearch"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>
        <el-button type="primary" :loading="searchLoading" @click="runSearch">搜索</el-button>
        <el-button :icon="RefreshRight" @click="refreshAll">刷新</el-button>
      </div>

      <div class="filter-row">
        <button
          v-for="item in filterOptions"
          :key="item.value"
          class="filter-chip"
          :class="{ active: activeFilter === item.value }"
          @click="activeFilter = item.value"
        >
          <span>{{ item.label }}</span>
          <em>{{ countByFilter(item.value) }}</em>
        </button>
      </div>
    </section>

    <section v-loading="loading || searchLoading" class="store-grid-wrapper">
      <div v-if="visibleItems.length > 0" class="store-grid">
        <article
          v-for="item in visibleItems"
          :key="item.key"
          class="store-card"
        >
          <div class="card-cover" @click="openDetail(item)">
            <img v-if="item.cover" :src="item.cover" :alt="item.title" loading="lazy" />
            <div v-else class="cover-fallback">
              <el-icon><ShoppingBag /></el-icon>
            </div>
            <div class="card-badges">
              <span class="badge">{{ item.kindLabel }}</span>
              <span v-if="item.source === 'bought'" class="badge ghost">已购</span>
              <span v-else-if="item.source === 'free'" class="badge ghost">免费</span>
            </div>
          </div>

          <div class="card-content">
            <h3 class="card-title" :title="item.title" @click="openDetail(item)">{{ item.title }}</h3>
            <p class="card-intro">{{ item.intro || '暂无简介' }}</p>
            <div class="card-meta">
              <span>{{ item.author || item.metaText || '内容详情' }}</span>
              <span>{{ item.priceText || '官方查看' }}</span>
            </div>
            <div class="card-actions">
              <el-button size="small" type="primary" @click="openDetail(item)">详情</el-button>
              <el-button size="small" @click="openLearn(item)" :disabled="!item.learnable">打开</el-button>
              <el-button size="small" plain @click="openOrderCenter(item)" :disabled="!item.productKind && !item.productId">订单</el-button>
              <el-button
                size="small"
                :icon="Link"
                @click="openOfficial(item)"
                :disabled="!item.officialUrl"
              >
                官网
              </el-button>
            </div>
          </div>
        </article>
      </div>

      <div v-else class="empty-state">
        <el-icon class="empty-icon"><Tickets /></el-icon>
        <h3>当前没有可展示的内容</h3>
        <p>可以尝试刷新推荐，或换一个关键词搜索。</p>
        <el-button type="primary" round @click="refreshAll">立即刷新</el-button>
      </div>
    </section>
  </div>
</template>

<script lang="ts" setup>
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Link, RefreshRight, Search, ShoppingBag, Tickets } from '@element-plus/icons-vue'
import {
  CourseList,
  SearchMoreContent,
  SunflowerLabelContent,
  SunflowerLabelList,
  SunflowerResourceList,
} from '../../wailsjs/go/backend/App'
import { useAppRouter } from '../composables/useRouter'
import { commerceStore } from '../stores/commerce'

type FilterValue = 'all' | 'course' | 'ebook' | 'free' | 'bought'

type StoreItem = {
  key: string
  title: string
  intro: string
  cover: string
  author: string
  priceText: string
  productKind: string
  kindLabel: string
  productType: number
  productId: string
  learnTargetId: string
  metaText: string
  officialUrl: string
  source: 'recommend' | 'search' | 'bought' | 'free'
  learnable: boolean
}

const { pushAllContent, pushCourseDetail, pushEbookReader, pushProductDetail, pushStoreMembership, pushStoreOrders, openExternalUrl } = useAppRouter()
const cStore = commerceStore()

const keyword = ref('')
const loading = ref(false)
const searchLoading = ref(false)
const activeFilter = ref<FilterValue>('all')

const courseItems = ref<StoreItem[]>([])
const ebookItems = ref<StoreItem[]>([])
const freeItems = ref<StoreItem[]>([])
const boughtItems = ref<StoreItem[]>([])
const searchItems = ref<StoreItem[]>([])

const filterOptions = [
  { value: 'all', label: '全部' },
  { value: 'course', label: '课程' },
  { value: 'ebook', label: '电子书' },
  { value: 'free', label: '免费专区' },
  { value: 'bought', label: '我的可学' },
] as const

const normalizeUrl = (raw: string) => {
  const value = String(raw || '').trim()
  if (!value) return ''
  if (value.startsWith('http://') || value.startsWith('https://')) return value
  if (value.startsWith('//')) return `https:${value}`
  if (value.startsWith('/')) return `https://www.dedao.cn${value}`
  return `https://www.dedao.cn/${value.replace(/^\/+/, '')}`
}

const kindFromType = (productType: number) => {
  if (productType === 2) return 'ebook'
  if (productType === 13 || productType === 1013) return 'odob'
  if (productType === 131) return 'compass'
  if (productType === 310) return 'trainingcamp'
  if (productType === 510) return 'institute'
  if ([4, 22, 36, 65, 66, 67].includes(productType)) return 'course'
  return 'content'
}

const kindLabel = (kind: string, productType: number) => {
  if (kind === 'ebook') return '电子书'
  if (kind === 'odob') return '听书'
  if (kind === 'compass') return '锦囊'
  if (kind === 'trainingcamp') return '训练营'
  if (kind === 'institute') return '研修班'
  if (kind === 'course') return productType === 65 || productType === 67 ? '文稿' : '课程'
  return `类型${productType || '-'}`
}

const isDirectlyLearnable = (source: StoreItem['source'], kind: string, item: any) => {
  if (kind === 'ebook') {
    return source === 'bought' || source === 'free' || Boolean(item?.is_buy || item?.is_on_bookshelf || item?.progress)
  }
  if (kind === 'course') {
    return source === 'bought' || source === 'free' || Boolean(item?.is_buy || item?.is_free_try || item?.is_user_free_try || item?.progress)
  }
  return false
}

const mapSearchItem = (item: any): StoreItem => {
  const productType = Number(item?.type || 0)
  const kind = kindFromType(productType)
  const productId = String(item?.enid || item?.id || '').trim()
  const learnTargetId = kind === 'course'
    ? String(item?.id || item?.product_id || '').trim()
    : productId
  const officialUrl = normalizeUrl(String(item?.dd_url || item?.dd_ext_url || ''))
  return {
    key: `search:${productType}:${productId || item?.title || Math.random()}`,
    title: String(item?.title || '未命名内容').trim(),
    intro: String(item?.intro || item?.product_intro || '').trim(),
    cover: String(item?.icon || '').trim(),
    author: String(item?.author || '').trim(),
    priceText: String(item?.price || item?.product_price || '官网查看').trim(),
    productKind: kind,
    kindLabel: kindLabel(kind, productType),
    productType,
    productId,
    learnTargetId,
    metaText: item?.progress ? `进度 ${item.progress}%` : '搜索结果',
    officialUrl,
    source: 'search',
    learnable: Boolean(kind === 'course' ? learnTargetId : productId) && isDirectlyLearnable('search', kind, item),
  }
}

const mapRecommendItem = (item: any, sourceKind: 'course' | 'ebook'): StoreItem => {
  const productType = Number(item?.product_type || 0)
  const productId = String(item?.product_enid || '').trim()
  return {
    key: `recommend:${sourceKind}:${productId}`,
    title: String(item?.title || '未命名内容').trim(),
    intro: String(item?.intro || item?.introduction || '').trim(),
    cover: String(sourceKind === 'ebook' ? item?.index_image : item?.horizontal_image || item?.index_image || '').trim(),
    author: Array.isArray(item?.author_list) ? item.author_list.join(' / ') : '',
    priceText: sourceKind === 'ebook' ? `评分 ${item?.score || '暂无'}` : `${Number(item?.learn_user_count || 0)} 人加入`,
    productKind: sourceKind,
    kindLabel: kindLabel(sourceKind, productType),
    productType,
    productId,
    learnTargetId: sourceKind === 'course' ? '' : productId,
    metaText: sourceKind === 'ebook' ? '编辑推荐' : '推荐课程',
    officialUrl: normalizeUrl(String(item?.dd_url || '')),
    source: 'recommend',
    learnable: false,
  }
}

const mapFreeItem = (item: any): StoreItem => {
  const productType = Number(item?.product_type || 0)
  const kind = kindFromType(productType)
  const productId = String(item?.enid || item?.product_id || '').trim()
  const learnTargetId = kind === 'course'
    ? String(item?.product_id || item?.id || '').trim()
    : productId
  return {
    key: `free:${productType}:${productId}`,
    title: String(item?.name || '免费内容').trim(),
    intro: String(item?.intro || '').trim(),
    cover: String(item?.logo || '').trim(),
    author: '',
    priceText: '限时免费',
    productKind: kind,
    kindLabel: kindLabel(kind, productType),
    productType,
    productId,
    learnTargetId,
    metaText: `评分 ${Number(item?.score || 0).toFixed(1)}`,
    officialUrl: '',
    source: 'free',
    learnable: Boolean(kind === 'course' ? learnTargetId : productId) && isDirectlyLearnable('free', kind, item),
  }
}

const mapBoughtItem = (item: any): StoreItem => {
  const productType = Number(item?.type || 0)
  const kind = kindFromType(productType)
  const productId = String(item?.enid || item?.id || '').trim()
  const learnTargetId = kind === 'course'
    ? String(item?.id || item?.class_id || '').trim()
    : productId
  const officialUrl = normalizeUrl(String(item?.dd_url || item?.dd_ext_url || ''))
  return {
    key: `bought:${productType}:${productId}`,
    title: String(item?.title || '已购内容').trim(),
    intro: String(item?.intro || item?.product_intro || '').trim(),
    cover: String(item?.icon || '').trim(),
    author: String(item?.author || '').trim(),
    priceText: item?.progress ? `进度 ${item.progress}%` : '已购内容',
    productKind: kind,
    kindLabel: kindLabel(kind, productType),
    productType,
    productId,
    learnTargetId,
    metaText: item?.progress ? `进度 ${item.progress}%` : '我的可学',
    officialUrl,
    source: 'bought',
    learnable: Boolean(kind === 'course' ? learnTargetId : productId) && isDirectlyLearnable('bought', kind, item),
  }
}

const dedupeItems = (list: StoreItem[]) => {
  const map = new Map<string, StoreItem>()
  list.forEach((item) => {
    if (!map.has(item.key)) map.set(item.key, item)
  })
  return Array.from(map.values())
}

const mergedBaseItems = computed(() =>
  dedupeItems([
    ...courseItems.value,
    ...ebookItems.value,
    ...freeItems.value,
    ...boughtItems.value,
  ]),
)

const activeSourceItems = computed(() => {
  const list = keyword.value.trim() ? searchItems.value : mergedBaseItems.value
  if (activeFilter.value === 'all') return list
  if (activeFilter.value === 'course') return list.filter((item) => item.productKind === 'course')
  if (activeFilter.value === 'ebook') return list.filter((item) => item.productKind === 'ebook')
  if (activeFilter.value === 'free') return list.filter((item) => item.source === 'free')
  if (activeFilter.value === 'bought') return list.filter((item) => item.source === 'bought')
  return list
})

const visibleItems = computed(() => activeSourceItems.value)

const countByFilter = (filter: FilterValue) => {
  const list = keyword.value.trim() ? searchItems.value : mergedBaseItems.value
  if (filter === 'all') return list.length
  if (filter === 'course') return list.filter((item) => item.productKind === 'course').length
  if (filter === 'ebook') return list.filter((item) => item.productKind === 'ebook').length
  if (filter === 'free') return list.filter((item) => item.source === 'free').length
  if (filter === 'bought') return list.filter((item) => item.source === 'bought').length
  return list.length
}

const buildOrderCenterQuery = (item: StoreItem) => ({
  focusKind: item.productKind,
  focusId: item.productId,
  focusTitle: item.title,
})

const buildTrackedOrderCenterQuery = (item: StoreItem, localId?: string) => ({
  ...buildOrderCenterQuery(item),
  focusLocalId: localId || undefined,
})

const loadRecommendations = async () => {
  const [courseLabels, ebookLabels, freeResp] = await Promise.all([
    SunflowerLabelList(4).catch(() => ({ list: [] as any[] })),
    SunflowerLabelList(2).catch(() => ({ list: [] as any[] })),
    SunflowerResourceList().catch(() => ({ list: [] as any[] })),
  ])

  const firstCourse = courseLabels?.list?.[0]
  const firstEbook = ebookLabels?.list?.[0]
  const tasks: Promise<any>[] = []
  if (firstCourse?.enid) tasks.push(SunflowerLabelContent(firstCourse.enid, 4, 0, 12))
  if (firstEbook?.enid) tasks.push(SunflowerLabelContent(firstEbook.enid, 2, 0, 12))

  const results = await Promise.all(tasks.map((item) => item.catch(() => null)))
  courseItems.value = Array.isArray(results?.[0]?.product_list)
    ? results[0].product_list.map((item: any) => mapRecommendItem(item, 'course'))
    : []

  const ebookResult = firstEbook?.enid ? results[results.length > 1 ? 1 : 0] : null
  ebookItems.value = Array.isArray(ebookResult?.product_list)
    ? ebookResult.product_list.map((item: any) => mapRecommendItem(item, 'ebook'))
    : []

  freeItems.value = Array.isArray(freeResp?.list)
    ? freeResp.list.map((item: any) => mapFreeItem(item))
    : []
}

const loadBought = async () => {
  try {
    const resp = await CourseList('all', 'study', 'all', 1, 18)
    boughtItems.value = Array.isArray(resp?.list) ? resp.list.map((item: any) => mapBoughtItem(item)) : []
  } catch {
    boughtItems.value = []
  }
}

const refreshAll = async () => {
  loading.value = true
  try {
    await Promise.all([loadRecommendations(), loadBought()])
  } catch (error: any) {
    ElMessage({ type: 'warning', message: String(error || '内容商店加载失败') })
  } finally {
    loading.value = false
  }
}

const clearSearch = () => {
  keyword.value = ''
  searchItems.value = []
}

const runSearch = async () => {
  const key = String(keyword.value || '').trim()
  if (!key) {
    searchItems.value = []
    return
  }
  searchLoading.value = true
  try {
    const resp = await SearchMoreContent(key, 1, 60)
    searchItems.value = Array.isArray(resp?.list) ? resp.list.map((item: any) => mapSearchItem(item)) : []
    if (searchItems.value.length === 0) {
      ElMessage({ type: 'info', message: '没有搜索到匹配内容' })
    }
  } catch (error: any) {
    ElMessage({ type: 'warning', message: String(error || '搜索失败') })
  } finally {
    searchLoading.value = false
  }
}

const openDetail = (item: StoreItem) => {
  if (!item.productId) {
    ElMessage({ type: 'warning', message: '缺少内容标识，无法打开详情' })
    return
  }
  pushProductDetail(item.productKind, item.productId, {
    title: item.title,
    intro: item.intro,
    cover: item.cover,
    author: item.author,
    priceText: item.priceText,
    productType: item.productType,
    officialUrl: item.officialUrl,
    source: item.source,
  })
}

const openOfficial = (item: StoreItem) => {
  if (!item.officialUrl) {
    ElMessage({ type: 'warning', message: '当前内容暂无可用官网链接' })
    return
  }
  const order = cStore.recordRedirectOrder({
    title: item.title,
    cover: item.cover,
    productKind: item.productKind,
    productType: item.productType,
    productId: item.productId,
    priceText: item.priceText,
    officialUrl: item.officialUrl,
  })
  openExternalUrl(item.officialUrl)
  if (order?.localId) {
    pushStoreOrders(buildTrackedOrderCenterQuery(item, order.localId))
  }
}

const openOrderCenter = (item: StoreItem) => {
  pushStoreOrders(buildOrderCenterQuery(item))
}

const openLearn = (item: StoreItem) => {
  if (!item.learnable || !item.productId) {
    ElMessage({ type: 'warning', message: '当前内容暂不支持直接打开' })
    return
  }
  if (item.productKind === 'ebook') {
    pushEbookReader(item.productId, { title: item.title, from: 'store' })
    return
  }
  if (item.productKind === 'course') {
    const classId = String(item.learnTargetId || '').trim()
    if (!classId) {
      ElMessage({ type: 'warning', message: '当前课程缺少章节标识，请先查看详情' })
      return
    }
    pushCourseDetail(classId, { enid: item.productId, title: item.title, from: 'store' })
    return
  }
  ElMessage({ type: 'info', message: '当前类型请先查看详情' })
}

onMounted(() => {
  void refreshAll()
})
</script>

<style scoped>
.store-page {
  min-height: calc(100vh - 72px);
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 14px;
  box-sizing: border-box;
}

.store-hero {
  display: grid;
  grid-template-columns: 1fr 320px;
  gap: 14px;
  padding: 20px;
  border-radius: 16px;
  border: 1px solid color-mix(in srgb, var(--border-soft) 82%, transparent);
  background:
    radial-gradient(340px 180px at 14% 0%, color-mix(in srgb, var(--accent-color) 16%, transparent) 0%, transparent 72%),
    radial-gradient(300px 180px at 95% 0%, color-mix(in srgb, var(--primary-color) 22%, transparent) 0%, transparent 74%),
    color-mix(in srgb, var(--surface-glass) 78%, transparent);
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
  line-height: 1.14;
  color: var(--text-primary);
  font-family: var(--font-family-display);
}

.hero-subtitle {
  margin: 10px 0 0;
  max-width: 760px;
  color: var(--text-secondary);
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
  color: var(--text-primary);
}

.store-toolbar {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px;
  border: 1px solid color-mix(in srgb, var(--border-soft) 80%, transparent);
  border-radius: 14px;
  background: color-mix(in srgb, var(--card-bg) 90%, transparent);
}

.search-row {
  display: grid;
  grid-template-columns: 1fr auto auto;
  gap: 8px;
}

.filter-row {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.filter-chip {
  height: 36px;
  border-radius: 999px;
  border: 1px solid color-mix(in srgb, var(--border-soft) 84%, transparent);
  background: color-mix(in srgb, var(--card-bg) 92%, transparent);
  color: var(--text-primary);
  padding: 0 12px;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
}

.filter-chip em {
  min-width: 22px;
  height: 20px;
  border-radius: 999px;
  font-style: normal;
  font-size: 11px;
  color: var(--text-secondary);
  background: color-mix(in srgb, var(--fill-color-light) 90%, transparent);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0 6px;
}

.filter-chip.active {
  border-color: transparent;
  background: linear-gradient(120deg, color-mix(in srgb, var(--accent-color) 82%, #fff 18%) 0%, var(--accent-color) 100%);
  color: #fff;
}

.filter-chip.active em {
  color: #fff;
  background: rgba(255, 255, 255, 0.18);
}

.store-grid-wrapper {
  flex: 1;
  min-height: 280px;
  border-radius: 16px;
  border: 1px solid color-mix(in srgb, var(--border-soft) 80%, transparent);
  background: color-mix(in srgb, var(--card-bg) 90%, transparent);
  padding: 14px;
}

.store-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
}

.store-card {
  border-radius: 14px;
  border: 1px solid color-mix(in srgb, var(--border-soft) 82%, transparent);
  overflow: hidden;
  background: color-mix(in srgb, var(--card-bg) 96%, transparent);
  display: flex;
  flex-direction: column;
  transition: transform 0.24s ease, border-color 0.24s ease, box-shadow 0.24s ease;
}

.store-card:hover {
  transform: translateY(-3px);
  border-color: color-mix(in srgb, var(--accent-color) 30%, transparent);
  box-shadow: var(--shadow-medium);
}

.card-cover {
  position: relative;
  aspect-ratio: 16 / 10;
  background: var(--fill-color-light);
  cursor: pointer;
}

.card-cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.cover-fallback {
  width: 100%;
  height: 100%;
  display: grid;
  place-items: center;
  color: var(--text-tertiary);
  font-size: 28px;
}

.card-badges {
  position: absolute;
  top: 8px;
  left: 8px;
  right: 8px;
  display: flex;
  justify-content: space-between;
  gap: 8px;
}

.badge {
  display: inline-flex;
  align-items: center;
  height: 24px;
  padding: 0 10px;
  border-radius: 999px;
  font-size: 11px;
  color: #fff;
  background: rgba(8, 18, 28, 0.72);
}

.badge.ghost {
  margin-left: auto;
  background: rgba(255, 255, 255, 0.18);
  backdrop-filter: blur(10px);
}

.card-content {
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  flex: 1;
}

.card-title {
  margin: 0;
  font-size: 15px;
  line-height: 1.45;
  color: var(--text-primary);
  cursor: pointer;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.card-intro {
  margin: 0;
  font-size: 12px;
  color: var(--text-secondary);
  line-height: 1.6;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  min-height: 38px;
}

.card-meta {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  color: var(--text-tertiary);
  font-size: 12px;
}

.card-actions {
  margin-top: auto;
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.empty-state {
  min-height: 260px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  color: var(--text-secondary);
}

.empty-icon {
  font-size: 36px;
  color: var(--accent-color);
}

@media (max-width: 1280px) {
  .store-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 1080px) {
  .store-hero {
    grid-template-columns: 1fr;
  }

  .hero-stats {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }

  .store-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 780px) {
  .store-page {
    padding: 10px;
  }

  .search-row {
    grid-template-columns: 1fr;
  }

  .hero-stats {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .store-grid {
    grid-template-columns: 1fr;
  }
}
</style>
