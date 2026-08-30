<template>
  <div class="product-detail-page">
    <section class="detail-hero">
      <div class="hero-cover">
        <img v-if="displayCover" :src="displayCover" :alt="displayTitle" />
        <div v-else class="cover-fallback">
          <el-icon><Tickets /></el-icon>
        </div>
      </div>

      <div class="hero-content" v-loading="loading">
        <div class="hero-meta-row">
          <el-tag effect="plain">{{ displayKindLabel }}</el-tag>
          <el-tag v-if="priceText" type="warning" effect="plain">{{ priceText }}</el-tag>
          <span class="meta-text">{{ sourceText }}</span>
        </div>
        <h1 class="hero-title">{{ displayTitle }}</h1>
        <p class="hero-intro">{{ displayIntro }}</p>

        <div class="hero-facts">
          <article class="fact-card" v-for="fact in detailFacts" :key="fact.label">
            <span>{{ fact.label }}</span>
            <strong>{{ fact.value }}</strong>
          </article>
        </div>

        <div class="hero-actions">
          <el-button type="primary" round @click="openLearning" :disabled="!canLearn">在应用中打开</el-button>
          <el-button round @click="openOfficial" :disabled="!officialUrl">官网查看</el-button>
          <el-button round @click="openOrderCenter">订单中心</el-button>
          <el-button round @click="backToStore">返回商店</el-button>
        </div>
      </div>
    </section>

    <section v-if="!errorText" class="commerce-strip">
      <article class="panel-card panel-primary">
        <span class="panel-label">购买状态</span>
        <strong>{{ accessStatusText }}</strong>
        <p>{{ purchaseHintText }}</p>
        <div class="panel-tags">
          <el-tag v-for="tag in rightsTags" :key="tag" effect="plain">{{ tag }}</el-tag>
        </div>
      </article>

      <article class="panel-card">
        <span class="panel-label">价格信息</span>
        <strong>{{ displayPrice }}</strong>
        <p>{{ priceHintText }}</p>
      </article>

      <article class="panel-card">
        <span class="panel-label">官方入口</span>
        <strong>{{ officialActionText }}</strong>
        <p>{{ officialHintText }}</p>
        <div class="panel-actions">
          <el-button size="small" type="primary" @click="openOfficial" :disabled="!officialUrl">{{ officialActionText }}</el-button>
          <el-button size="small" @click="openOrderCenter">查看订单</el-button>
        </div>
      </article>
    </section>

    <section class="detail-body" v-loading="loading">
      <div v-if="errorText" class="error-state">
        <el-result icon="warning" title="详情加载失败" :sub-title="errorText">
          <template #extra>
            <el-button type="primary" @click="loadDetail">重试</el-button>
          </template>
        </el-result>
      </div>

      <template v-else>
        <section class="detail-section" v-if="courseDetail">
          <div class="section-header">
            <h3>课程介绍</h3>
            <span>{{ courseDetail.class_info?.lecturer_name || '课程详情' }}</span>
          </div>
          <p class="rich-text">{{ courseDetail.class_info?.intro || fallbackIntro }}</p>
          <div class="outline-grid" v-if="courseDetail.class_info?.highlight || courseDetail.class_info?.lecturer_intro">
            <article class="outline-card" v-if="courseDetail.class_info?.highlight">
              <span>课程亮点</span>
              <p>{{ courseDetail.class_info?.highlight }}</p>
            </article>
            <article class="outline-card" v-if="courseDetail.class_info?.lecturer_intro">
              <span>主讲介绍</span>
              <p>{{ courseDetail.class_info?.lecturer_intro }}</p>
            </article>
          </div>
        </section>

        <section class="detail-section" v-if="ebookDetail">
          <div class="section-header">
            <h3>电子书信息</h3>
            <span>{{ ebookDetail.press?.name || '电子书详情' }}</span>
          </div>
          <p class="rich-text">{{ ebookDetail.book_intro || ebookDetail.other_share_summary || fallbackIntro }}</p>
          <div class="outline-grid" v-if="ebookDetail.author_info || ebookDetail.press?.brief">
            <article class="outline-card" v-if="ebookDetail.author_info">
              <span>作者介绍</span>
              <p>{{ ebookDetail.author_info }}</p>
            </article>
            <article class="outline-card" v-if="ebookDetail.press?.brief">
              <span>出版社</span>
              <p>{{ ebookDetail.press?.brief }}</p>
            </article>
          </div>
        </section>

        <section class="detail-section" v-if="audioDetail">
          <div class="section-header">
            <h3>听书信息</h3>
            <span>{{ audioDetail.detail?.agency_detail?.qcg_member_name || '音频详情' }}</span>
          </div>
          <p class="rich-text">{{ audioDetail.detail?.audio_summary || fallbackIntro }}</p>
          <div class="outline-grid" v-if="Array.isArray(audioDetail.detail?.topic_summary)">
            <article class="outline-card" v-for="item in audioDetail.detail.topic_summary.slice(0, 4)" :key="item.id">
              <span>{{ item.title }}</span>
              <p>{{ item.sub_title }}</p>
            </article>
          </div>
        </section>

        <section class="detail-section" v-if="!courseDetail && !ebookDetail && !audioDetail">
          <div class="section-header">
            <h3>通用详情</h3>
            <span>当前类型尚未接入专用详情接口</span>
          </div>
          <p class="rich-text">{{ fallbackIntro }}</p>
        </section>
      </template>
    </section>
  </div>
</template>

<script lang="ts" setup>
import { computed, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { Tickets } from '@element-plus/icons-vue'
import { useRoute } from 'vue-router'
import { AudioDetail, CourseInfo, EbookInfo } from '../../wailsjs/go/backend/App'
import { useAppRouter } from '../composables/useRouter'
import { commerceStore } from '../stores/commerce'

const route = useRoute()
const cStore = commerceStore()
const { pushCourseDetail, pushEbookReader, pushStoreHome, pushStoreOrders, openExternalUrl } = useAppRouter()

const loading = ref(false)
const errorText = ref('')
const courseDetail = ref<any | null>(null)
const ebookDetail = ref<any | null>(null)
const audioDetail = ref<any | null>(null)

const routeKind = computed(() => String(route.params.type || route.query.productKind || 'content').trim())
const routeId = computed(() => String(route.params.id || '').trim())
const titleFromQuery = computed(() => String(route.query.title || '内容详情').trim())
const introFromQuery = computed(() => String(route.query.intro || '').trim())
const coverFromQuery = computed(() => String(route.query.cover || '').trim())
const authorFromQuery = computed(() => String(route.query.author || '').trim())
const priceText = computed(() => String(route.query.priceText || '').trim())
const sourceText = computed(() => {
  const source = String(route.query.source || '').trim()
  if (source === 'bought') return '来自我的可学'
  if (source === 'search') return '来自搜索结果'
  if (source === 'free') return '来自免费专区'
  return '来自内容商店'
})
const productType = computed(() => Number(route.query.productType || 0))
const normalizeUrl = (raw: unknown) => {
  const value = String(raw || '').trim()
  if (!value) return ''
  if (value.startsWith('http://') || value.startsWith('https://')) return value
  if (value.startsWith('//')) return `https:${value}`
  if (value.startsWith('/')) return `https://www.dedao.cn${value}`
  return `https://www.dedao.cn/${value.replace(/^\/+/, '')}`
}

const inferKindByType = (value: number) => {
  if (value === 2) return 'ebook'
  if (value === 13 || value === 1013) return 'odob'
  if (value === 131) return 'compass'
  if (value === 310) return 'trainingcamp'
  if (value === 510) return 'institute'
  if ([4, 22, 36, 65, 66, 67].includes(value)) return 'course'
  return 'content'
}

const normalizedKind = computed(() => {
  if (routeKind.value && routeKind.value !== 'content') return routeKind.value
  return inferKindByType(productType.value)
})

const queryOfficialUrl = computed(() => normalizeUrl(route.query.officialUrl))

const detailOfficialUrl = computed(() => {
  if (normalizedKind.value === 'ebook') {
    return normalizeUrl(ebookDetail.value?.add_studylist_dd_url || ebookDetail.value?.dd_url)
  }
  if (normalizedKind.value === 'odob') {
    return normalizeUrl(audioDetail.value?.detail?.dd_url || audioDetail.value?.detail?.h5_share_url)
  }
  if (
    normalizedKind.value === 'course' ||
    normalizedKind.value === 'compass' ||
    normalizedKind.value === 'trainingcamp' ||
    normalizedKind.value === 'institute'
  ) {
    return normalizeUrl(
      courseDetail.value?.class_info?.shzf_url ||
      courseDetail.value?.class_info?.presale_url ||
      courseDetail.value?.class_info?.dd_url ||
      courseDetail.value?.class_info?.share_url,
    )
  }
  return ''
})

const officialUrl = computed(() => {
  return detailOfficialUrl.value || queryOfficialUrl.value
})

const displayTitle = computed(() => {
  if (courseDetail.value?.class_info?.name) return String(courseDetail.value.class_info.name)
  if (ebookDetail.value?.operating_title) return String(ebookDetail.value.operating_title)
  if (audioDetail.value?.detail?.title) return String(audioDetail.value.detail.title)
  return titleFromQuery.value || '内容详情'
})

const displayCover = computed(() => {
  return String(
    courseDetail.value?.class_info?.share_img ||
      ebookDetail.value?.cover ||
      audioDetail.value?.detail?.icon ||
      coverFromQuery.value ||
      '',
  ).trim()
})

const displayIntro = computed(() => {
  if (courseDetail.value?.class_info?.intro) return String(courseDetail.value.class_info.intro)
  if (ebookDetail.value?.book_intro) return String(ebookDetail.value.book_intro)
  if (audioDetail.value?.detail?.audio_summary) return String(audioDetail.value.detail.audio_summary)
  return introFromQuery.value || '暂无简介'
})

const fallbackIntro = computed(() => displayIntro.value || introFromQuery.value || '暂无简介')

const displayKindLabel = computed(() => {
  const kind = normalizedKind.value
  if (kind === 'course') return '课程'
  if (kind === 'ebook') return '电子书'
  if (kind === 'odob') return '听书'
  if (kind === 'compass') return '锦囊'
  if (kind === 'trainingcamp') return '训练营'
  if (kind === 'institute') return '研修班'
  if (productType.value === 65 || productType.value === 67) return '文稿'
  return '内容'
})

const detailFacts = computed(() => {
  if (courseDetail.value?.class_info) {
    const info = courseDetail.value.class_info
    return [
      { label: '主讲人', value: String(info.lecturer_name || '暂无') },
      { label: '加入学习', value: `${Number(info.learn_user_count || 0)} 人` },
      { label: '总期数', value: `${Number(info.phase_num || 0)} 期` },
      { label: '评价数', value: `${Number(courseDetail.value.class_comment_info?.count || 0)} 条` },
    ]
  }
  if (ebookDetail.value) {
    return [
      { label: '作者', value: Array.isArray(ebookDetail.value.author_list) ? ebookDetail.value.author_list.join(' / ') : authorFromQuery.value || '暂无' },
      { label: '出版社', value: String(ebookDetail.value.press?.name || '暂无') },
      { label: '字数', value: `${Math.floor(Number(ebookDetail.value.count || 0) / 1000)} 千字` },
      { label: '朗读', value: ebookDetail.value.is_tts_switch ? '支持' : '不支持' },
    ]
  }
  if (audioDetail.value?.detail) {
    const info = audioDetail.value.detail
    return [
      { label: '解读人', value: String(info.agency_detail?.qcg_member_name || '暂无') },
      { label: '学习次数', value: String(info.learn_count_desc || info.learn_count || '0') },
      { label: '时长', value: `${Math.round(Number(info.duration || 0) / 60)} 分钟` },
      { label: '标签', value: Array.isArray(info.tag) && info.tag.length > 0 ? info.tag.join(' / ') : '暂无' },
    ]
  }
  return [
    { label: '类型', value: displayKindLabel.value },
    { label: '作者', value: authorFromQuery.value || '暂无' },
    { label: '价格', value: priceText.value || '官网查看' },
    { label: '来源', value: sourceText.value },
  ]
})

const withCurrency = (value: unknown) => {
  const text = String(value ?? '').trim()
  if (!text) return ''
  if (/[¥元]/.test(text)) return text
  return /^\d+(\.\d+)?$/.test(text) ? `${text}元` : text
}

const displayPrice = computed(() => {
  if (normalizedKind.value === 'course' && courseDetail.value?.class_info) {
    const info = courseDetail.value.class_info
    if (String(info.early_bird_msg || '').trim()) return String(info.early_bird_msg).trim()
    if (String(info.price_desc || '').trim()) return String(info.price_desc).trim()
    if (Number(info.price || 0) > 0) return withCurrency(info.price)
  }
  if (normalizedKind.value === 'ebook' && ebookDetail.value) {
    return withCurrency(ebookDetail.value.current_price || ebookDetail.value.price || priceText.value) || '官网查看'
  }
  if (normalizedKind.value === 'odob' && audioDetail.value?.detail) {
    return withCurrency(audioDetail.value.detail.audio_price || priceText.value) || '官网查看'
  }
  return priceText.value || '官网查看'
})

const accessStatusText = computed(() => {
  if (normalizedKind.value === 'course' && courseDetail.value?.class_info) {
    const info = courseDetail.value.class_info
    if (canLearn.value) return '已支持在应用中学习'
    if (info.is_in_vip || info.is_vip) return '会员可学'
    if (Number(info.trial_count || 0) > 0) return `支持试学 ${Number(info.trial_count || 0)} 期`
    return '需官方购买'
  }
  if (normalizedKind.value === 'ebook' && ebookDetail.value) {
    if (ebookDetail.value.is_buy || ebookDetail.value.is_on_bookshelf) return '已可阅读'
    if (Number(ebookDetail.value.is_vip_book || 0) === 1) return '会员免费'
    if (ebookDetail.value.can_trial_read) return '支持试读'
    return '需官方购买'
  }
  if (normalizedKind.value === 'odob' && audioDetail.value?.detail) {
    const info = audioDetail.value.detail
    if (info.is_buy || info.in_bookrack) return '已可收听'
    if (info.is_limit_free) return '限时免费'
    if (info.is_vip) return '会员免费'
    return '需官方购买'
  }
  return canLearn.value ? '可直接打开' : '查看官网详情'
})

const purchaseHintText = computed(() => {
  if (canLearn.value) return '当前内容已经满足桌面端打开条件，可直接继续学习。'
  if (officialUrl.value) return '当前版本先复用官方购买或权益页面，跳转记录会保存在订单中心。'
  return '当前详情接口没有返回可用官网入口。'
})

const priceHintText = computed(() => {
  if (normalizedKind.value === 'course' && courseDetail.value?.class_info) {
    const info = courseDetail.value.class_info
    if (String(info.early_bird_msg || '').trim()) return String(info.early_bird_msg).trim()
    return `共 ${Number(info.phase_num || 0)} 期，已更新 ${Number(info.current_article_count || 0)} 期`
  }
  if (normalizedKind.value === 'ebook' && ebookDetail.value) {
    if (ebookDetail.value.can_trial_read) {
      return `支持试读 ${String(ebookDetail.value.trial_read_proportion || '').trim() || '部分章节'}`
    }
    if (Number(ebookDetail.value.is_vip_book || 0) === 1) return '电子书会员可免费阅读'
    return '购买或加入书架后可在应用内阅读'
  }
  if (normalizedKind.value === 'odob' && audioDetail.value?.detail) {
    const info = audioDetail.value.detail
    if (info.is_limit_free) return '当前命中限时免费权益'
    if (info.is_vip) return '听书会员可直接收听'
    return '请到官方页确认购买和权益状态'
  }
  return '价格与权益以官方页面展示为准'
})

const rightsTags = computed(() => {
  const tags: string[] = []
  if (normalizedKind.value === 'course' && courseDetail.value?.class_info) {
    const info = courseDetail.value.class_info
    tags.push('章节学习')
    if (!info.without_audio) tags.push('音频')
    if (info.video_class) tags.push('视频')
    if (info.is_in_vip || info.is_vip) tags.push('会员权益')
    if (Number(info.trial_count || 0) > 0) tags.push(`试学 ${Number(info.trial_count || 0)} 期`)
  } else if (normalizedKind.value === 'ebook' && ebookDetail.value) {
    tags.push('阅读器')
    if (ebookDetail.value.is_tts_switch) tags.push('朗读')
    if (Number(ebookDetail.value.is_vip_book || 0) === 1) tags.push('会员免费')
    if (ebookDetail.value.can_trial_read) tags.push('试读')
    if (ebookDetail.value.is_on_bookshelf) tags.push('已在书架')
  } else if (normalizedKind.value === 'odob' && audioDetail.value?.detail) {
    const info = audioDetail.value.detail
    tags.push('音频学习')
    if (info.in_bookrack) tags.push('已在书架')
    if (info.is_vip) tags.push('会员免费')
    if (info.is_limit_free) tags.push('限时免费')
    if (info.has_play_auth) tags.push('可播放')
  } else {
    tags.push(displayKindLabel.value)
  }
  return tags
})

const officialActionText = computed(() => {
  if (!officialUrl.value) return '暂无官方入口'
  if (canLearn.value) return '官网查看'
  if (accessStatusText.value.includes('会员')) return '官网开通/查看权益'
  return '官网购买'
})

const officialHintText = computed(() => {
  if (!officialUrl.value) return '详情接口没有返回官方链接。'
  if (detailOfficialUrl.value) return '优先使用详情接口返回的官方购买或权益链接。'
  return '使用列表页带来的官方链接作为兜底入口。'
})

const canLearn = computed(() => {
  if (normalizedKind.value === 'ebook') return Boolean(routeId.value)
  if (normalizedKind.value === 'course') return Boolean(routeId.value) && Boolean(courseDetail.value?.class_info?.id)
  return false
})

const orderCenterQuery = computed(() => ({
  focusKind: normalizedKind.value,
  focusId: routeId.value,
  focusTitle: displayTitle.value,
}))

const buildOrderCenterQuery = (localId?: string) => ({
  ...orderCenterQuery.value,
  focusLocalId: localId || undefined,
})

const openOfficial = () => {
  if (!officialUrl.value) {
    ElMessage({ type: 'warning', message: '当前内容暂无可用官网链接' })
    return
  }
  const order = cStore.recordRedirectOrder({
    title: displayTitle.value,
    cover: displayCover.value,
    productKind: normalizedKind.value,
    productType: productType.value,
    productId: routeId.value,
    priceText: displayPrice.value || '官网查看',
    officialUrl: officialUrl.value,
  })
  openExternalUrl(officialUrl.value)
  if (order?.localId) {
    pushStoreOrders(buildOrderCenterQuery(order.localId))
  }
}

const openLearning = () => {
  if (normalizedKind.value === 'ebook') {
    pushEbookReader(routeId.value, { title: displayTitle.value, from: 'store' })
    return
  }
  if (normalizedKind.value === 'course') {
    const classId = Number(courseDetail.value?.class_info?.id || 0)
    if (!classId) {
      ElMessage({ type: 'warning', message: '当前课程暂时无法直接打开章节列表' })
      return
    }
    pushCourseDetail(classId, {
      enid: String(courseDetail.value?.class_info?.enid || routeId.value),
      title: displayTitle.value,
      from: 'store',
    })
    return
  }
  ElMessage({ type: 'info', message: '当前类型请先查看官网或详情信息' })
}

const openOrderCenter = () => {
  pushStoreOrders(buildOrderCenterQuery())
}

const backToStore = () => {
  pushStoreHome()
}

const loadDetail = async () => {
  if (!routeId.value) return
  loading.value = true
  errorText.value = ''
  courseDetail.value = null
  ebookDetail.value = null
  audioDetail.value = null

  try {
    if (normalizedKind.value === 'ebook') {
      ebookDetail.value = await EbookInfo(routeId.value)
    } else if (normalizedKind.value === 'odob') {
      audioDetail.value = await AudioDetail(routeId.value)
    } else if (normalizedKind.value === 'course' || normalizedKind.value === 'compass' || normalizedKind.value === 'trainingcamp' || normalizedKind.value === 'institute') {
      courseDetail.value = await CourseInfo(routeId.value)
    }
  } catch (error: any) {
    errorText.value = String(error || '详情加载失败')
  } finally {
    loading.value = false
  }
}

watch(
  () => [route.params.type, route.params.id, route.query.productType],
  () => {
    if (routeId.value) {
      void loadDetail()
    }
  },
  { immediate: true },
)
</script>

<style scoped>
.product-detail-page {
  min-height: calc(100vh - 72px);
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 14px;
  box-sizing: border-box;
}

.detail-hero {
  display: grid;
  grid-template-columns: 280px 1fr;
  gap: 16px;
  padding: 18px;
  border-radius: 16px;
  border: 1px solid color-mix(in srgb, var(--border-soft) 82%, transparent);
  background: color-mix(in srgb, var(--card-bg) 92%, transparent);
}

.hero-cover {
  border-radius: 16px;
  overflow: hidden;
  background: var(--fill-color-light);
  aspect-ratio: 3 / 4;
}

.hero-cover img {
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
  font-size: 36px;
}

.hero-content {
  display: flex;
  flex-direction: column;
}

.hero-meta-row {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  align-items: center;
}

.meta-text {
  font-size: 12px;
  color: var(--text-tertiary);
}

.hero-title {
  margin: 12px 0 8px;
  font-size: 32px;
  line-height: 1.16;
  color: var(--text-primary);
  font-family: var(--font-family-display);
}

.hero-intro {
  margin: 0;
  font-size: 14px;
  line-height: 1.75;
  color: var(--text-secondary);
}

.hero-facts {
  margin-top: 18px;
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
}

.fact-card {
  padding: 12px;
  border-radius: 12px;
  border: 1px solid color-mix(in srgb, var(--border-soft) 78%, transparent);
  background: color-mix(in srgb, var(--fill-color-light) 74%, transparent);
}

.fact-card span {
  display: block;
  font-size: 12px;
  color: var(--text-tertiary);
}

.fact-card strong {
  display: block;
  margin-top: 4px;
  font-size: 16px;
  color: var(--text-primary);
}

.hero-actions {
  margin-top: auto;
  padding-top: 18px;
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.commerce-strip {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 14px;
}

.panel-card {
  padding: 16px;
  border-radius: 16px;
  border: 1px solid color-mix(in srgb, var(--border-soft) 80%, transparent);
  background: color-mix(in srgb, var(--card-bg) 92%, transparent);
}

.panel-primary {
  background:
    radial-gradient(220px 120px at 0% 0%, color-mix(in srgb, var(--accent-color) 12%, transparent) 0%, transparent 72%),
    color-mix(in srgb, var(--card-bg) 94%, transparent);
}

.panel-label {
  display: block;
  font-size: 12px;
  color: var(--text-tertiary);
}

.panel-card strong {
  display: block;
  margin-top: 6px;
  font-size: 22px;
  line-height: 1.3;
  color: var(--text-primary);
}

.panel-card p {
  margin: 10px 0 0;
  color: var(--text-secondary);
  line-height: 1.7;
  min-height: 44px;
}

.panel-tags {
  margin-top: 12px;
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.panel-actions {
  margin-top: 12px;
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.detail-body {
  flex: 1;
  min-height: 240px;
  border-radius: 16px;
  border: 1px solid color-mix(in srgb, var(--border-soft) 82%, transparent);
  background: color-mix(in srgb, var(--card-bg) 92%, transparent);
  padding: 16px;
}

.detail-section {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.section-header {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  padding-bottom: 12px;
  border-bottom: 1px solid color-mix(in srgb, var(--border-soft) 76%, transparent);
}

.section-header h3 {
  margin: 0;
  font-size: 18px;
  color: var(--text-primary);
}

.section-header span {
  color: var(--text-secondary);
  font-size: 12px;
}

.rich-text {
  margin: 0;
  line-height: 1.8;
  color: var(--text-secondary);
  white-space: pre-wrap;
}

.outline-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.outline-card {
  border-radius: 12px;
  border: 1px solid color-mix(in srgb, var(--border-soft) 78%, transparent);
  background: color-mix(in srgb, var(--fill-color-light) 70%, transparent);
  padding: 12px;
}

.outline-card span {
  display: block;
  font-size: 12px;
  color: var(--text-tertiary);
}

.outline-card p {
  margin: 8px 0 0;
  line-height: 1.7;
  color: var(--text-primary);
  white-space: pre-wrap;
}

.error-state {
  min-height: 240px;
  display: grid;
  place-items: center;
}

@media (max-width: 1120px) {
  .detail-hero {
    grid-template-columns: 1fr;
  }

  .commerce-strip {
    grid-template-columns: 1fr;
  }

  .hero-cover {
    max-width: 280px;
  }

  .hero-facts {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .outline-grid {
    grid-template-columns: 1fr;
  }
}
</style>
