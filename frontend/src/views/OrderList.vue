<template>
  <div class="orders-page">
    <section class="orders-hero">
      <div class="hero-main">
        <p class="hero-kicker">Orders</p>
        <h1 class="hero-title">订单中心</h1>
        <p class="hero-subtitle">
          上半部分保留桌面端发起的官网跳转记录，下半部分开始接入官方已购内容壳子，作为正式订单接口前的过渡版本。
        </p>
      </div>
      <div class="hero-stats">
        <article class="stat-card">
          <span>本地跳转</span>
          <strong>{{ cStore.orderCount }}</strong>
        </article>
        <article class="stat-card">
          <span>待回流</span>
          <strong>{{ pendingLocalCount }}</strong>
        </article>
        <article class="stat-card">
          <span>官方已购</span>
          <strong>{{ officialTotalDisplay }}</strong>
        </article>
        <article class="stat-card">
          <span>已回流</span>
          <strong>{{ matchedLocalCount }}</strong>
        </article>
      </div>
    </section>

    <section class="orders-toolbar">
      <div class="toolbar-actions">
        <el-button type="primary" round @click="pushStoreHome">返回商店</el-button>
        <el-button round @click="pushStoreMembership">会员中心</el-button>
        <el-button round @click="openOfficialUrl('https://www.dedao.cn')">官网首页</el-button>
        <el-button round :icon="RefreshRight" @click="loadOfficialOrders" :loading="officialLoading">刷新官方记录</el-button>
        <el-button round @click="markVisibleVisited" :disabled="visibleOrders.length === 0 || statusFilter === 'archived'">全部标记已查看</el-button>
        <el-button round @click="cStore.clearArchivedOrders()" :disabled="archivedCount === 0">清理已归档</el-button>
      </div>

      <div class="toolbar-filters">
        <el-input
          v-model="keyword"
          clearable
          placeholder="按标题、作者筛选本地和官方记录"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>

        <div class="filter-row">
          <button
            v-for="item in statusOptions"
            :key="item.value"
            class="filter-chip"
            :class="{ active: statusFilter === item.value }"
            @click="statusFilter = item.value"
          >
            <span>{{ item.label }}</span>
            <em>{{ countByStatus(item.value) }}</em>
          </button>
        </div>
      </div>
    </section>

    <section class="reconcile-shell">
      <div class="section-head">
        <div>
          <p class="section-kicker">Reconcile</p>
          <h2>回流状态</h2>
          <p class="section-subtitle">{{ reconcileSummaryText }}</p>
        </div>
        <div class="section-side">
          <el-tag effect="plain" type="warning">{{ linkFilterLabel }}</el-tag>
          <span class="section-meta">自动匹配 {{ linkedOfficialCount }} 条官方记录</span>
        </div>
      </div>

      <div class="reconcile-grid">
        <article class="reconcile-card" :class="{ active: linkFilter === 'matched' }">
          <span>已回流官方</span>
          <strong>{{ matchedLocalCount }}</strong>
          <p>本地跳转已在官方已购列表里找到对应内容。</p>
        </article>
        <article class="reconcile-card" :class="{ active: linkFilter === 'pending' }">
          <span>待回流</span>
          <strong>{{ pendingLocalCount }}</strong>
          <p>本地有跳转记录，但官方已购列表里还没看到对应内容。</p>
        </article>
        <article class="reconcile-card" :class="{ active: linkFilter === 'local-only' }">
          <span>仅本地 / 会员</span>
          <strong>{{ localOnlyCount }}</strong>
          <p>会员续费或缺少稳定标识的记录，当前只保留桌面端跳转痕迹。</p>
        </article>
        <article class="reconcile-card" :class="{ active: linkFilter === 'official-only' }">
          <span>仅官方已购</span>
          <strong>{{ officialOnlyCount }}</strong>
          <p>官方列表里已有内容，但当前没有对应的桌面端跳转记录。</p>
        </article>
      </div>

      <div class="filter-row">
        <button
          v-for="item in linkOptions"
          :key="item.value"
          class="filter-chip"
          :class="{ active: linkFilter === item.value }"
          @click="linkFilter = item.value"
        >
          <span>{{ item.label }}</span>
          <em>{{ item.count }}</em>
        </button>
      </div>
    </section>

    <section v-if="focusTarget" class="focus-shell">
      <div class="focus-summary">
        <div>
          <p class="section-kicker">Locator</p>
          <h2>已定位到：{{ focusTargetLabel }}</h2>
          <p class="section-subtitle">
            本地命中 {{ focusedLocalCount }} 条 · 官方命中 {{ focusedOfficialCount }} 条
          </p>
        </div>

        <div class="focus-actions">
          <el-button type="primary" plain @click="focusOnly = !focusOnly">
            {{ focusOnly ? '查看全部记录' : '只看相关记录' }}
          </el-button>
          <el-button plain @click="scrollToFocusedCard">滚动到命中项</el-button>
          <el-button plain @click="clearFocus">清除定位</el-button>
        </div>
      </div>
    </section>

    <section class="orders-section">
      <div class="section-head">
        <div>
          <p class="section-kicker">Local</p>
          <h2>本地跳转记录</h2>
          <p class="section-subtitle">记录桌面端发起的官网购买、续费和权益页跳转，并尝试和官方已购内容自动匹配。</p>
        </div>
        <div class="section-side">
          <el-tag effect="plain" type="info">本地记录</el-tag>
          <span class="section-meta">共 {{ cStore.orderCount }} 条 · 已归档 {{ archivedCount }} 条</span>
        </div>
      </div>

      <div class="orders-list-shell">
        <div v-if="visibleOrders.length > 0" class="orders-list">
          <article
            v-for="item in visibleOrders"
            :key="item.localId"
            class="order-card"
            :class="{ focused: isFocusedLocal(item) }"
          >
            <div class="order-cover">
              <img v-if="item.cover" :src="item.cover" :alt="item.title" loading="lazy" />
              <div v-else class="cover-fallback">
                <el-icon><Tickets /></el-icon>
              </div>
            </div>

            <div class="order-content">
              <div class="order-head">
                <div class="order-title-group">
                  <h3>{{ item.title }}</h3>
                  <p class="order-submeta">
                    {{ kindLabel(item.productKind, item.productType) }}<span v-if="item.productId"> · ID {{ item.productId }}</span>
                  </p>
                </div>
                <el-tag effect="plain" :type="statusType(item.status)">{{ statusLabel(item.status) }}</el-tag>
              </div>
              <p class="order-meta">
                {{ item.priceText }} · {{ formatTime(item.createdAt) }}
              </p>
              <div class="relation-row">
                <el-tag effect="plain" :type="localRelationType(item)">{{ localRelationLabel(item) }}</el-tag>
                <span class="relation-text">{{ localRelationText(item) }}</span>
              </div>
              <div class="order-actions">
                <el-button size="small" type="primary" @click="reopenOrder(item.localId, item.officialUrl, item.title, item.productKind, item.productId)">再次打开官网</el-button>
                <el-button
                  v-if="getMatchedOfficial(item)"
                  size="small"
                  plain
                  @click="focusOfficialMatch(item)"
                >
                  查看已购
                </el-button>
                <el-button
                  v-if="item.status !== 'archived'"
                  size="small"
                  @click="cStore.markOrderVisited(item.localId)"
                  :disabled="item.status === 'visited'"
                >
                  标记已查看
                </el-button>
                <el-button
                  v-if="item.status !== 'archived'"
                  size="small"
                  type="danger"
                  plain
                  @click="cStore.archiveOrder(item.localId)"
                >
                  归档
                </el-button>
                <el-button
                  v-else
                  size="small"
                  plain
                  @click="cStore.restoreOrder(item.localId)"
                >
                  恢复
                </el-button>
              </div>
            </div>
          </article>
        </div>

        <div v-else class="empty-state">
          <el-icon class="empty-icon"><Tickets /></el-icon>
          <h3>{{ emptyTitle }}</h3>
          <p>{{ emptySubtitle }}</p>
          <el-button type="primary" round @click="pushStoreHome">去内容商店</el-button>
        </div>
      </div>
    </section>

    <section class="orders-section">
      <div class="section-head">
        <div>
          <p class="section-kicker">Official</p>
          <h2>官方已购记录</h2>
          <p class="section-subtitle">
            基于官方“学习中/已购内容”列表聚合，并与本地官网跳转记录做关联，不等于真实支付流水。
          </p>
        </div>
        <div class="section-side">
          <el-tag effect="plain" type="success">{{ officialSourceLabel }}</el-tag>
          <span class="section-meta">{{ officialLoadedText }} · 已关联 {{ linkedOfficialCount }} 条</span>
        </div>
      </div>

      <el-alert
        v-if="officialError"
        class="section-alert"
        type="warning"
        show-icon
        :closable="false"
        title="官方记录加载失败"
        :description="officialError"
      />

      <div class="orders-list-shell" v-loading="officialLoading">
        <div v-if="visibleOfficialOrders.length > 0" class="orders-list">
          <article
            v-for="item in visibleOfficialOrders"
            :key="item.recordId"
            class="order-card official-card"
            :class="{ focused: isFocusedOfficial(item) }"
          >
            <div class="order-cover">
              <img v-if="item.cover" :src="item.cover" :alt="item.title" loading="lazy" />
              <div v-else class="cover-fallback">
                <el-icon><Tickets /></el-icon>
              </div>
            </div>

            <div class="order-content">
              <div class="order-head">
                <div class="order-title-group">
                  <h3>{{ item.title }}</h3>
                  <p class="order-submeta">
                    {{ item.kindLabel || kindLabel(item.productKind, item.productType) }}
                    <span v-if="item.author"> · {{ item.author }}</span>
                    <span v-if="item.productId"> · ENID {{ item.productId }}</span>
                  </p>
                </div>
                <el-tag effect="plain" type="success">{{ item.sourceLabel || '官方已购内容' }}</el-tag>
              </div>

              <p v-if="item.intro" class="order-intro">{{ item.intro }}</p>
              <p class="order-meta">
                {{ item.priceText || '已购内容' }} · {{ item.progressText || '已购内容' }}
              </p>
              <p class="order-meta subtle">{{ formatTime(item.updatedAt) }}</p>
              <div class="relation-row">
                <el-tag effect="plain" :type="officialRelationType(item)">{{ officialRelationLabel(item) }}</el-tag>
                <span class="relation-text">{{ officialRelationText(item) }}</span>
              </div>

              <div class="order-actions">
                <el-button size="small" type="primary" @click="openOfficialDetail(item)">查看详情</el-button>
                <el-button size="small" @click="openOfficialLearn(item)" :disabled="!canDirectLearn(item)">继续学习</el-button>
                <el-button
                  v-if="getMatchedLocal(item)"
                  size="small"
                  plain
                  @click="focusLocalMatch(item)"
                >
                  定位本地记录
                </el-button>
                <el-button size="small" @click="openOfficialRecord(item)" :disabled="!item.officialUrl">官网入口</el-button>
              </div>
            </div>
          </article>
        </div>

        <div v-else class="empty-state official-empty">
          <el-icon class="empty-icon"><Tickets /></el-icon>
          <h3>{{ officialEmptyTitle }}</h3>
          <p>{{ officialEmptySubtitle }}</p>
          <el-button round :icon="RefreshRight" @click="loadOfficialOrders" :loading="officialLoading">重新同步</el-button>
        </div>
      </div>
    </section>
  </div>
</template>

<script lang="ts" setup>
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { RefreshRight, Search, Tickets } from '@element-plus/icons-vue'
import { commerceStore, type RedirectOrder } from '../stores/commerce'
import { useAppRouter } from '../composables/useRouter'
import { ROUTE_NAMES } from '../router/routes'
import { invokeBackend } from '../utils/backend'

type StatusFilter = 'all' | 'redirected' | 'visited' | 'archived'
type LinkFilter = 'all' | 'matched' | 'pending' | 'official-only' | 'local-only'
type LocalOrderRelationState = 'matched' | 'pending' | 'local-only'

type OfficialOrderRecord = {
  recordId: string
  title: string
  intro: string
  cover: string
  author: string
  productKind: string
  kindLabel: string
  productType: number
  productId: string
  learnTargetId: string
  priceText: string
  progress: number
  progressText: string
  officialUrl: string
  sourceLabel: string
  updatedAt: number
}

type OfficialOrderListResp = {
  list?: OfficialOrderRecord[]
  total?: number
  page?: number
  limit?: number
  source?: string
}

type FocusTarget = {
  localId: string
  productKind: string
  productId: string
  title: string
}

const cStore = commerceStore()
const {
  pushCourseDetail,
  pushEbookReader,
  pushProductDetail,
  pushStoreHome,
  pushStoreMembership,
  replace,
  route,
  openExternalUrl: openOfficialUrl,
} = useAppRouter()

const archivedCount = computed(() => cStore.redirectOrders.filter((item) => item.status === 'archived').length)

const keyword = ref('')
const statusFilter = ref<StatusFilter>('all')
const linkFilter = ref<LinkFilter>('all')
const statusOptions = [
  { value: 'all', label: '全部' },
  { value: 'redirected', label: '待处理' },
  { value: 'visited', label: '已查看' },
  { value: 'archived', label: '已归档' },
] as const

const officialLoading = ref(false)
const officialError = ref('')
const officialOrders = ref<OfficialOrderRecord[]>([])
const officialTotal = ref(0)
const officialSource = ref('')
const officialLimit = 60
const focusOnly = ref(false)

const normalizeUrl = (raw: unknown) => {
  const value = String(raw || '').trim()
  if (!value) return ''
  if (value.startsWith('http://') || value.startsWith('https://')) return value
  if (value.startsWith('//')) return `https:${value}`
  if (value.startsWith('/')) return `https://www.dedao.cn${value}`
  return `https://www.dedao.cn/${value.replace(/^\/+/, '')}`
}

const getQueryText = (value: unknown) => {
  if (Array.isArray(value)) return String(value[0] || '').trim()
  return String(value || '').trim()
}

const normalizeKey = (value: unknown) => String(value || '').trim().toLowerCase()
const titleMatches = (left: unknown, right: unknown) => {
  const leftKey = normalizeKey(left)
  const rightKey = normalizeKey(right)
  if (!leftKey || !rightKey) return false
  return leftKey === rightKey || leftKey.includes(rightKey) || rightKey.includes(leftKey)
}
const isMembershipKind = (kind: string) => ['ebook-vip', 'odob-vip'].includes(normalizeKey(kind))
const sameContent = (
  leftKind: string,
  leftId: string,
  leftTitle: string,
  rightKind: string,
  rightId: string,
  rightTitle: string,
) => {
  if (normalizeKey(leftKind) !== normalizeKey(rightKind)) return false
  const leftIdKey = normalizeKey(leftId)
  const rightIdKey = normalizeKey(rightId)
  if (leftIdKey && rightIdKey) return leftIdKey === rightIdKey
  return titleMatches(leftTitle, rightTitle)
}
const isTrackableLocalOrder = (item: RedirectOrder) => {
  if (isMembershipKind(item.productKind)) return false
  return Boolean(normalizeKey(item.productKind) && (normalizeKey(item.productId) || normalizeKey(item.title)))
}

const normalizeTimestamp = (value: number) => {
  let time = Number(value || 0)
  if (!Number.isFinite(time) || time <= 0) return 0
  if (time < 1e12) time *= 1000
  return time
}

const formatTime = (time: number) => {
  const normalized = normalizeTimestamp(time)
  if (!normalized) return '时间未知'
  const d = new Date(normalized)
  const pad = (v: number) => String(v).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

const kindLabel = (kind: string, productType = 0) => {
  if (kind === 'course') return productType === 65 || productType === 67 ? '文稿' : '课程'
  if (kind === 'ebook') return '电子书'
  if (kind === 'odob') return '听书'
  if (kind === 'compass') return '锦囊'
  if (kind === 'trainingcamp') return '训练营'
  if (kind === 'institute') return '研修班'
  if (kind === 'ebook-vip') return '电子书会员'
  if (kind === 'odob-vip') return '听书会员'
  return '内容'
}

const focusTarget = computed<FocusTarget | null>(() => {
  const localId = getQueryText(route.query.focusLocalId)
  const productKind = getQueryText(route.query.focusKind)
  const productId = getQueryText(route.query.focusId)
  const title = getQueryText(route.query.focusTitle)
  if (!localId && !productKind && !productId && !title) return null
  return { localId, productKind, productId, title }
})

const focusedLocalOrder = computed(() => {
  if (!focusTarget.value?.localId) return null
  return cStore.redirectOrders.find((item) => item.localId === focusTarget.value?.localId) || null
})

const focusTargetLabel = computed(() => {
  if (!focusTarget.value) return ''
  if (focusedLocalOrder.value?.title) return focusedLocalOrder.value.title
  if (focusTarget.value.title) return focusTarget.value.title
  if (focusTarget.value.productKind && focusTarget.value.productId) {
    return `${kindLabel(focusTarget.value.productKind)} · ${focusTarget.value.productId}`
  }
  return focusTarget.value.productId || focusTarget.value.productKind
})

const matchesFocus = (productKind: string, productId: string, title: string) => {
  const target = focusTarget.value
  if (!target) return false

  const sameKind = !target.productKind || normalizeKey(productKind) === normalizeKey(target.productKind)
  const sameId = !target.productId || normalizeKey(productId) === normalizeKey(target.productId)

  if (target.productId) return sameKind && sameId
  if (target.productKind && target.title) return sameKind && normalizeKey(title).includes(normalizeKey(target.title))
  if (target.productKind) return sameKind
  if (target.title) return normalizeKey(title).includes(normalizeKey(target.title))
  return false
}

const isFocusedLocal = (item: any) => {
  if (focusTarget.value?.localId) {
    return item.localId === focusTarget.value.localId
  }
  return matchesFocus(item.productKind, item.productId, item.title)
}
const isFocusedOfficial = (item: OfficialOrderRecord) => matchesFocus(item.productKind, item.productId, item.title)

const focusedLocalCount = computed(() => cStore.redirectOrders.filter((item) => isFocusedLocal(item)).length)
const focusedOfficialCount = computed(() => officialOrders.value.filter((item) => isFocusedOfficial(item)).length)

const statusLabel = (status: string) => {
  if (status === 'visited') return '已查看'
  if (status === 'archived') return '已归档'
  return '已跳转'
}

const statusType = (status: string) => {
  if (status === 'visited') return 'success'
  if (status === 'archived') return 'info'
  return 'warning'
}

const matchesLinkFilterLocal = (item: RedirectOrder) => {
  if (linkFilter.value === 'all') return true
  const state = localRelationState(item)
  if (linkFilter.value === 'matched') return state === 'matched'
  if (linkFilter.value === 'pending') return state === 'pending'
  if (linkFilter.value === 'local-only') return state === 'local-only'
  return false
}

const visibleOrders = computed(() => {
  const key = String(keyword.value || '').trim().toLowerCase()
  return [...cStore.redirectOrders]
    .filter((item) => statusFilter.value === 'all' || item.status === statusFilter.value)
    .filter((item) => matchesLinkFilterLocal(item))
    .filter((item) => !focusOnly.value || !focusTarget.value || isFocusedLocal(item))
    .filter((item) => {
      if (!key) return true
      return [item.title, item.productKind, item.productId].some((value) =>
        String(value || '').toLowerCase().includes(key),
      )
    })
})

const countByStatus = (status: StatusFilter) => {
  if (status === 'all') return cStore.redirectOrders.length
  return cStore.redirectOrders.filter((item) => item.status === status).length
}

const markVisibleVisited = () => {
  visibleOrders.value.forEach((item) => {
    if (item.status !== 'archived') {
      cStore.markOrderVisited(item.localId)
    }
  })
}

const emptyTitle = computed(() => {
  if (focusOnly.value && focusTarget.value) return `没有匹配“${focusTargetLabel.value}”的本地记录`
  if (cStore.redirectOrders.length === 0) return '还没有本地跳转记录'
  if (linkFilter.value !== 'all') return `当前“${linkFilterLabel.value}”条件下没有本地记录`
  return '当前筛选条件下没有本地记录'
})

const emptySubtitle = computed(() => {
  if (focusOnly.value && focusTarget.value) {
    return '可以切回全部记录，或者清除当前定位后再查看。'
  }
  if (cStore.redirectOrders.length === 0) {
    return '从内容商店、详情页或会员中心点击官网入口后，会在这里保留记录。'
  }
  if (linkFilter.value === 'official-only') {
    return '当前筛选只展示官方独有内容，本地列表为空是正常现象。'
  }
  return '可以切换状态筛选，或清空关键词后再查看。'
})

const normalizeOfficialOrder = (item: any): OfficialOrderRecord => {
  return {
    recordId: String(item?.record_id || item?.recordId || '').trim(),
    title: String(item?.title || '官方已购内容').trim(),
    intro: String(item?.intro || '').trim(),
    cover: String(item?.cover || '').trim(),
    author: String(item?.author || '').trim(),
    productKind: String(item?.product_kind || item?.productKind || 'content').trim(),
    kindLabel: String(item?.kind_label || item?.kindLabel || '').trim(),
    productType: Number(item?.product_type || item?.productType || 0),
    productId: String(item?.product_id || item?.productId || '').trim(),
    learnTargetId: String(item?.learn_target_id || item?.learnTargetId || '').trim(),
    priceText: String(item?.price_text || item?.priceText || '').trim(),
    progress: Number(item?.progress || 0),
    progressText: String(item?.progress_text || item?.progressText || '').trim(),
    officialUrl: normalizeUrl(item?.official_url || item?.officialUrl),
    sourceLabel: String(item?.source_label || item?.sourceLabel || '官方已购内容').trim(),
    updatedAt: Number(item?.updated_at || item?.updatedAt || 0),
  }
}

const loadOfficialOrders = async () => {
  officialLoading.value = true
  officialError.value = ''
  try {
    const resp = await invokeBackend<OfficialOrderListResp>('OfficialOrderList', 1, officialLimit)
    const list = Array.isArray(resp?.list) ? resp.list.map((item) => normalizeOfficialOrder(item)) : []
    officialOrders.value = list
    officialTotal.value = Number(resp?.total || list.length)
    officialSource.value = String(resp?.source || 'study_course_list').trim()
  } catch (error: any) {
    officialOrders.value = []
    officialTotal.value = 0
    officialSource.value = ''
    officialError.value = String(error?.message || error || '官方已购记录加载失败')
  } finally {
    officialLoading.value = false
  }
}

const officialTotalDisplay = computed(() => officialTotal.value || officialOrders.value.length)

const officialSourceLabel = computed(() => {
  if (officialSource.value === 'study_course_list') return '学习列表聚合'
  if (officialSource.value) return officialSource.value
  return '官方数据'
})

const sortedLocalOrders = computed(() =>
  [...cStore.redirectOrders].sort((left, right) => normalizeTimestamp(right.createdAt) - normalizeTimestamp(left.createdAt)),
)

const matchedOfficialByLocalId = computed(() => {
  const map = new Map<string, OfficialOrderRecord>()
  cStore.redirectOrders.forEach((item) => {
    const match = officialOrders.value.find((official) =>
      sameContent(item.productKind, item.productId, item.title, official.productKind, official.productId, official.title),
    )
    if (match) {
      map.set(item.localId, match)
    }
  })
  return map
})

const matchedLocalByOfficialId = computed(() => {
  const map = new Map<string, RedirectOrder>()
  officialOrders.value.forEach((item) => {
    const match = sortedLocalOrders.value.find((local) =>
      sameContent(local.productKind, local.productId, local.title, item.productKind, item.productId, item.title),
    )
    if (match) {
      map.set(item.recordId, match)
    }
  })
  return map
})

const getMatchedOfficial = (item: RedirectOrder) => matchedOfficialByLocalId.value.get(item.localId) || null
const getMatchedLocal = (item: OfficialOrderRecord) => matchedLocalByOfficialId.value.get(item.recordId) || null

const localRelationState = (item: RedirectOrder): LocalOrderRelationState => {
  if (getMatchedOfficial(item)) return 'matched'
  if (!isTrackableLocalOrder(item)) return 'local-only'
  return 'pending'
}

const localRelationLabel = (item: RedirectOrder) => {
  const state = localRelationState(item)
  if (state === 'matched') return '已回流官方'
  if (state === 'local-only') return '仅本地记录'
  return officialError.value ? '待同步确认' : '待回流'
}

const localRelationType = (item: RedirectOrder) => {
  const state = localRelationState(item)
  if (state === 'matched') return 'success'
  if (state === 'local-only') return 'info'
  return officialError.value ? 'info' : 'warning'
}

const localRelationText = (item: RedirectOrder) => {
  const match = getMatchedOfficial(item)
  if (match) {
    return `已匹配到官方已购内容 · ${match.progressText || '可继续学习'}`
  }
  if (isMembershipKind(item.productKind)) {
    return '会员续费和权益页当前不会出现在官方已购内容列表中。'
  }
  if (officialError.value) {
    return '官方记录同步失败，暂时无法确认当前跳转是否已经回流。'
  }
  if (officialLoading.value && officialOrders.value.length === 0) {
    return '正在同步官方已购列表，匹配结果稍后会自动更新。'
  }
  return '当前官方已购列表里还没有出现对应内容。'
}

const officialRelationLabel = (item: OfficialOrderRecord) => (getMatchedLocal(item) ? '已关联本地跳转' : '仅官方已购')
const officialRelationType = (item: OfficialOrderRecord) => (getMatchedLocal(item) ? 'success' : 'info')
const officialRelationText = (item: OfficialOrderRecord) => {
  const match = getMatchedLocal(item)
  if (match) {
    return `最近一次本地跳转：${formatTime(match.createdAt)}`
  }
  return '当前没有对应的桌面端官网跳转记录。'
}

const activeLocalOrders = computed(() => cStore.redirectOrders.filter((item) => item.status !== 'archived'))
const matchedLocalCount = computed(() => activeLocalOrders.value.filter((item) => localRelationState(item) === 'matched').length)
const pendingLocalCount = computed(() => activeLocalOrders.value.filter((item) => localRelationState(item) === 'pending').length)
const localOnlyCount = computed(() => activeLocalOrders.value.filter((item) => localRelationState(item) === 'local-only').length)
const linkedOfficialCount = computed(() => officialOrders.value.filter((item) => Boolean(getMatchedLocal(item))).length)
const officialOnlyCount = computed(() => officialOrders.value.filter((item) => !getMatchedLocal(item)).length)
const linkOptions = computed(() => [
  { value: 'all' as LinkFilter, label: '全部联动', count: cStore.orderCount + officialOrders.value.length },
  { value: 'matched' as LinkFilter, label: '已回流', count: matchedLocalCount.value },
  { value: 'pending' as LinkFilter, label: '待回流', count: pendingLocalCount.value },
  { value: 'official-only' as LinkFilter, label: '仅官方', count: officialOnlyCount.value },
  { value: 'local-only' as LinkFilter, label: '仅本地/会员', count: localOnlyCount.value },
])
const linkFilterLabel = computed(() => linkOptions.value.find((item) => item.value === linkFilter.value)?.label || '全部联动')
const reconcileSummaryText = computed(() => {
  if (officialError.value) {
    return '官方已购列表当前拉取失败，回流判断会先退化到本地记录视角。'
  }
  if (officialLoading.value && officialOrders.value.length === 0) {
    return '正在同步官方已购列表，匹配结果会在加载完成后自动刷新。'
  }
  return '优先使用内容类型 + 标识匹配，拿不到稳定标识时回退到标题匹配。'
})

const officialLoadedText = computed(() => {
  const loaded = officialOrders.value.length
  const total = officialTotalDisplay.value
  if (total > loaded) return `已载入 ${loaded}/${total} 条`
  return `已载入 ${loaded} 条`
})

const matchesLinkFilterOfficial = (item: OfficialOrderRecord) => {
  if (linkFilter.value === 'all') return true
  const hasLocal = Boolean(getMatchedLocal(item))
  if (linkFilter.value === 'matched') return hasLocal
  if (linkFilter.value === 'official-only') return !hasLocal
  return false
}

const visibleOfficialOrders = computed(() => {
  const key = String(keyword.value || '').trim().toLowerCase()
  return officialOrders.value
    .filter((item) => matchesLinkFilterOfficial(item))
    .filter((item) => !focusOnly.value || !focusTarget.value || isFocusedOfficial(item))
    .filter((item) => {
      if (!key) return true
      return [item.title, item.author, item.kindLabel, item.progressText, item.productId].some((value) =>
        String(value || '').toLowerCase().includes(key),
      )
    })
})

const officialEmptyTitle = computed(() => {
  if (officialLoading.value) return '正在同步官方已购记录'
  if (officialError.value) return '官方已购记录暂不可用'
  if (focusOnly.value && focusTarget.value) return `没有匹配“${focusTargetLabel.value}”的官方记录`
  if (officialOrders.value.length === 0) return '当前账号还没有可同步的官方已购内容'
  if (linkFilter.value !== 'all') return `当前“${linkFilterLabel.value}”条件下没有官方记录`
  return '当前筛选条件下没有官方记录'
})

const officialEmptySubtitle = computed(() => {
  if (officialError.value) return '当前版本会自动回退到本地跳转记录，不影响已有功能。'
  if (focusOnly.value && focusTarget.value) return '当前定位没有命中官方已购内容，可以切回全部记录继续查看。'
  if (officialOrders.value.length === 0) return '这里展示的是当前账号“我的可学”官方列表，不是支付流水。'
  if (linkFilter.value === 'pending' || linkFilter.value === 'local-only') {
    return '当前筛选只展示本地侧状态，本节没有记录是正常现象。'
  }
  return '可以清空关键词，或者刷新官方记录后再查看。'
})

const canDirectLearn = (item: OfficialOrderRecord) => {
  if (item.productKind === 'ebook') return Boolean(item.productId)
  if (item.productKind === 'course') return Boolean(item.productId) && Boolean(item.learnTargetId)
  return false
}

const focusOfficialMatch = (item: RedirectOrder) => {
  const match = getMatchedOfficial(item)
  if (!match) {
    ElMessage({ type: 'info', message: '当前还没有匹配到官方已购记录' })
    return
  }
  replace({
    name: ROUTE_NAMES.STORE_ORDERS,
    query: {
      ...route.query,
      focusKind: match.productKind,
      focusId: match.productId,
      focusTitle: match.title,
    },
  })
}

const focusLocalMatch = (item: OfficialOrderRecord) => {
  const match = getMatchedLocal(item)
  if (!match) {
    ElMessage({ type: 'info', message: '当前还没有关联到本地跳转记录' })
    return
  }
  focusLocalOrder(match.localId, {
    focusKind: item.productKind,
    focusId: item.productId,
    focusTitle: item.title,
  })
}

const focusLocalOrder = (localId: string, query?: Record<string, any>) => {
  replace({
    name: ROUTE_NAMES.STORE_ORDERS,
    query: {
      ...route.query,
      ...query,
      focusLocalId: localId,
    },
  })
}

const reopenOrder = (localId: string, url: string, title?: string, productKind?: string, productId?: string) => {
  cStore.markOrderVisited(localId)
  openOfficialUrl(url)
  focusLocalOrder(localId, {
    focusTitle: title,
    focusKind: productKind,
    focusId: productId,
  })
}

const openOfficialRecord = (item: OfficialOrderRecord) => {
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
    priceText: item.priceText || item.progressText || '官网查看',
    officialUrl: item.officialUrl,
  })
  openOfficialUrl(item.officialUrl)
  if (order?.localId) {
    focusLocalOrder(order.localId, {
      focusKind: item.productKind,
      focusId: item.productId,
      focusTitle: item.title,
    })
  }
}

const openOfficialDetail = (item: OfficialOrderRecord) => {
  if (!item.productId) {
    ElMessage({ type: 'warning', message: '当前记录缺少内容标识，无法打开详情' })
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
    source: 'bought',
  })
}

const openOfficialLearn = (item: OfficialOrderRecord) => {
  if (!canDirectLearn(item)) {
    ElMessage({ type: 'info', message: '当前类型请先查看详情或官网入口' })
    return
  }

  if (item.productKind === 'ebook') {
    pushEbookReader(item.productId, { title: item.title, from: 'official-orders' })
    return
  }

  if (item.productKind === 'course') {
    pushCourseDetail(item.learnTargetId, {
      enid: item.productId,
      title: item.title,
      from: 'official-orders',
    })
    return
  }

  ElMessage({ type: 'info', message: '当前类型请先查看详情' })
}

const clearFocus = () => {
  const query: Record<string, any> = { ...route.query }
  delete query.focusLocalId
  delete query.focusKind
  delete query.focusId
  delete query.focusTitle
  focusOnly.value = false
  replace({ name: ROUTE_NAMES.STORE_ORDERS, query })
}

const scrollToFocusedCard = async () => {
  if (!focusTarget.value) return
  await nextTick()
  const target = document.querySelector<HTMLElement>('.order-card.focused')
  target?.scrollIntoView({ behavior: 'smooth', block: 'center' })
}

watch(
  () => [route.query.focusLocalId, route.query.focusKind, route.query.focusId, route.query.focusTitle],
  () => {
    if (!focusTarget.value) {
      focusOnly.value = false
      return
    }
    keyword.value = ''
    statusFilter.value = 'all'
    focusOnly.value = false
    void scrollToFocusedCard()
  },
  { immediate: true },
)

watch(
  () => [focusOnly.value, visibleOrders.value.length, visibleOfficialOrders.value.length],
  () => {
    if (focusTarget.value) {
      void scrollToFocusedCard()
    }
  },
)

onMounted(() => {
  void loadOfficialOrders()
})
</script>

<style scoped>
.orders-page {
  min-height: calc(100vh - 72px);
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 14px;
  box-sizing: border-box;
}

.orders-hero {
  display: grid;
  grid-template-columns: 1fr 280px;
  gap: 14px;
  padding: 18px;
  border-radius: 16px;
  border: 1px solid color-mix(in srgb, var(--border-soft) 82%, transparent);
  background: color-mix(in srgb, var(--card-bg) 92%, transparent);
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
  color: var(--text-primary);
  font-size: 28px;
  line-height: 1.14;
}

.hero-subtitle {
  margin: 10px 0 0;
  color: var(--text-secondary);
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
  background: color-mix(in srgb, var(--fill-color-light) 74%, transparent);
}

.stat-card span {
  display: block;
  font-size: 12px;
  color: var(--text-tertiary);
}

.stat-card strong {
  display: block;
  margin-top: 4px;
  color: var(--text-primary);
  font-size: 18px;
}

.orders-toolbar {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.toolbar-actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.toolbar-filters {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px;
  border-radius: 14px;
  border: 1px solid color-mix(in srgb, var(--border-soft) 80%, transparent);
  background: color-mix(in srgb, var(--card-bg) 92%, transparent);
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

.reconcile-shell {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 14px;
  border-radius: 16px;
  border: 1px solid color-mix(in srgb, var(--border-soft) 80%, transparent);
  background:
    radial-gradient(280px 120px at 0% 0%, color-mix(in srgb, var(--accent-color) 10%, transparent) 0%, transparent 74%),
    color-mix(in srgb, var(--card-bg) 94%, transparent);
}

.reconcile-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
}

.reconcile-card {
  padding: 12px;
  border-radius: 12px;
  border: 1px solid color-mix(in srgb, var(--border-soft) 78%, transparent);
  background: color-mix(in srgb, var(--fill-color-light) 74%, transparent);
}

.reconcile-card.active {
  border-color: color-mix(in srgb, var(--accent-color) 42%, transparent);
  box-shadow: 0 0 0 1px color-mix(in srgb, var(--accent-color) 18%, transparent);
}

.reconcile-card span {
  display: block;
  color: var(--text-tertiary);
  font-size: 12px;
}

.reconcile-card strong {
  display: block;
  margin-top: 4px;
  color: var(--text-primary);
  font-size: 22px;
}

.reconcile-card p {
  margin: 8px 0 0;
  color: var(--text-secondary);
  font-size: 12px;
  line-height: 1.5;
}

.orders-section {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.focus-shell {
  padding: 14px 16px;
  border-radius: 16px;
  border: 1px solid color-mix(in srgb, var(--accent-color) 28%, transparent);
  background:
    radial-gradient(260px 140px at 0% 0%, color-mix(in srgb, var(--accent-color) 14%, transparent) 0%, transparent 72%),
    color-mix(in srgb, var(--card-bg) 94%, transparent);
}

.focus-summary {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: flex-start;
}

.focus-summary h2 {
  margin: 6px 0 0;
  color: var(--text-primary);
  font-size: 20px;
}

.focus-actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.section-head {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: flex-start;
}

.section-head h2 {
  margin: 6px 0 0;
  color: var(--text-primary);
  font-size: 22px;
}

.section-kicker {
  margin: 0;
  color: var(--accent-color);
  font-size: 11px;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  font-weight: 700;
}

.section-subtitle {
  margin: 8px 0 0;
  color: var(--text-secondary);
}

.section-side {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 6px;
}

.section-meta {
  color: var(--text-tertiary);
  font-size: 12px;
}

.section-alert {
  margin-bottom: 2px;
}

.orders-list-shell {
  min-height: 240px;
  border-radius: 16px;
  border: 1px solid color-mix(in srgb, var(--border-soft) 82%, transparent);
  background: color-mix(in srgb, var(--card-bg) 92%, transparent);
  padding: 14px;
}

.orders-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.order-card {
  display: grid;
  grid-template-columns: 100px 1fr;
  gap: 12px;
  padding: 12px;
  border-radius: 14px;
  border: 1px solid color-mix(in srgb, var(--border-soft) 78%, transparent);
  background: color-mix(in srgb, var(--fill-color-light) 68%, transparent);
}

.order-card.focused {
  border-color: color-mix(in srgb, var(--accent-color) 42%, transparent);
  box-shadow: 0 0 0 1px color-mix(in srgb, var(--accent-color) 18%, transparent);
}

.official-card {
  background:
    radial-gradient(260px 120px at 100% 0%, color-mix(in srgb, var(--accent-color) 10%, transparent) 0%, transparent 72%),
    color-mix(in srgb, var(--fill-color-light) 72%, transparent);
}

.order-cover {
  width: 100px;
  height: 76px;
  border-radius: 10px;
  overflow: hidden;
  background: var(--fill-color-light);
}

.order-cover img {
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
}

.order-content {
  display: flex;
  flex-direction: column;
}

.order-head {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: flex-start;
}

.order-title-group {
  min-width: 0;
}

.order-head h3 {
  margin: 0;
  font-size: 16px;
  line-height: 1.4;
  color: var(--text-primary);
}

.order-submeta {
  margin: 6px 0 0;
  color: var(--text-tertiary);
  font-size: 12px;
}

.order-intro {
  margin: 10px 0 0;
  color: var(--text-secondary);
  font-size: 13px;
  line-height: 1.6;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.order-meta {
  margin: 8px 0 0;
  color: var(--text-secondary);
  font-size: 12px;
}

.order-meta.subtle {
  color: var(--text-tertiary);
}

.relation-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  margin-top: 10px;
}

.relation-text {
  color: var(--text-secondary);
  font-size: 12px;
}

.order-actions {
  margin-top: auto;
  padding-top: 12px;
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.empty-state {
  min-height: 240px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  color: var(--text-secondary);
}

.official-empty {
  min-height: 220px;
}

.empty-icon {
  font-size: 36px;
  color: var(--accent-color);
}

@media (max-width: 860px) {
  .orders-hero {
    grid-template-columns: 1fr;
  }

  .reconcile-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .hero-stats {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .section-head {
    flex-direction: column;
  }

  .section-side {
    align-items: flex-start;
  }

  .focus-summary {
    flex-direction: column;
  }

  .focus-actions {
    justify-content: flex-start;
  }

  .order-card {
    grid-template-columns: 1fr;
  }
}
</style>
