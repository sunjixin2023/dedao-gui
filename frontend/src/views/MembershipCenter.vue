<template>
  <div class="membership-page">
    <section class="membership-hero" v-loading="loading">
      <div class="hero-main">
        <div class="profile-row">
          <el-avatar :size="72" :src="user.avatar">
            <el-icon><User /></el-icon>
          </el-avatar>
          <div class="profile-copy">
            <p class="hero-kicker">Membership Desk</p>
            <h1 class="hero-title">{{ displayName }}</h1>
            <p class="hero-subtitle">{{ displaySlogan }}</p>
          </div>
        </div>

        <div class="hero-actions">
          <el-button type="primary" round @click="pushStoreHome">返回商店</el-button>
          <el-button round @click="pushStoreOrders()">订单中心</el-button>
          <el-button round @click="openOfficialHome">官网首页</el-button>
          <el-button v-if="!isLoggedIn" round @click="pushLogin">去登录</el-button>
          <el-button v-else round type="danger" plain @click="handleLogout">退出登录</el-button>
        </div>
      </div>

      <div class="hero-stats">
        <article class="stat-card">
          <span>今日学习</span>
          <strong>{{ studyMinutes }} 分钟</strong>
        </article>
        <article class="stat-card">
          <span>连续学习</span>
          <strong>{{ Number(user.study_serial_days || 0) }} 天</strong>
        </article>
        <article class="stat-card">
          <span>有效会员</span>
          <strong>{{ activeMembershipCount }}</strong>
        </article>
        <article class="stat-card">
          <span>跳转订单</span>
          <strong>{{ cStore.activeOrders.length }}</strong>
        </article>
      </div>
    </section>

    <section class="membership-grid">
      <article class="member-card ebook" :class="{ inactive: !ebookUser.is_vip }">
        <div class="card-head">
          <div>
            <p class="card-kicker">Ebook VIP</p>
            <h3>电子书会员</h3>
          </div>
          <el-tag :type="ebookUser.is_vip && !ebookUser.is_expire ? 'success' : 'info'" effect="plain">
            {{ ebookStatusText }}
          </el-tag>
        </div>

        <p class="card-intro">
          {{ ebookUser.err_tips || '支持电子书阅读、朗读与会员权益查看。' }}
        </p>

        <div class="metric-grid">
          <div class="metric-item">
            <span>剩余天数</span>
            <strong>{{ ebookUser.is_vip ? Number(ebookUser.surplus_time || 0) : 0 }}</strong>
          </div>
          <div class="metric-item">
            <span>本月读书</span>
            <strong>{{ Number(ebookUser.month_count || 0) }}</strong>
          </div>
          <div class="metric-item">
            <span>累计读书</span>
            <strong>{{ Number(ebookUser.total_count || 0) }}</strong>
          </div>
          <div class="metric-item">
            <span>节省金额</span>
            <strong>{{ ebookSavings }}</strong>
          </div>
        </div>

        <p class="expire-line">到期时间：{{ formatExpireTime(ebookUser.expire_time) }}</p>

        <div class="card-actions">
          <el-button
            type="primary"
            round
            :disabled="!ebookRenewUrl"
            @click="openMemberLink('电子书会员续费', ebookRenewUrl, 'ebook-vip', 2)"
          >
            续费/开通
          </el-button>
          <el-button round @click="openMembershipOrders('电子书会员续费', 'ebook-vip')">查看订单</el-button>
          <el-button round @click="pushStoreHome">去内容商店</el-button>
        </div>
      </article>

      <article class="member-card odob" :class="{ inactive: !odobUser.user?.is_vip }">
        <div class="card-head">
          <div>
            <p class="card-kicker">Odob VIP</p>
            <h3>听书会员</h3>
          </div>
          <el-tag :type="odobUser.user?.is_vip && !odobUser.user?.is_expire ? 'success' : 'info'" effect="plain">
            {{ odobStatusText }}
          </el-tag>
        </div>

        <p class="card-intro">
          {{ odobUser.user?.err_tips || '支持听书权益、节省金额和周学习数据查看。' }}
        </p>

        <div class="metric-grid">
          <div class="metric-item">
            <span>剩余天数</span>
            <strong>{{ odobUser.user?.is_vip ? Number(odobUser.user?.surplus_time || 0) : 0 }}</strong>
          </div>
          <div class="metric-item">
            <span>本周听书</span>
            <strong>{{ Number(odobUser.user?.week_count || 0) }}</strong>
          </div>
          <div class="metric-item">
            <span>累计听书</span>
            <strong>{{ Number(odobUser.user?.total_count || 0) }}</strong>
          </div>
          <div class="metric-item">
            <span>节省金额</span>
            <strong>{{ odobSavings }}</strong>
          </div>
        </div>

        <p class="expire-line">到期时间：{{ formatExpireTime(odobUser.user?.expire_time) }}</p>

        <div class="card-actions">
          <el-button
            type="primary"
            round
            :disabled="!odobRenewUrl"
            @click="openMemberLink('听书会员续费', odobRenewUrl, 'odob-vip', 13)"
          >
            续费/开通
          </el-button>
          <el-button round @click="openMembershipOrders('听书会员续费', 'odob-vip')">查看订单</el-button>
        </div>
      </article>
    </section>

    <section class="notes-grid">
      <article class="note-card">
        <div class="note-head">
          <el-icon><Present /></el-icon>
          <h3>权益概览</h3>
        </div>
        <p>{{ rightsSummary }}</p>
      </article>

      <article class="note-card">
        <div class="note-head">
          <el-icon><Tickets /></el-icon>
          <h3>交易说明</h3>
        </div>
        <p>这版先复用官方会员页跳转，并把桌面端的跳转记录保存在订单中心，后续再接正式订单接口。</p>
      </article>
    </section>
  </div>
</template>

<script lang="ts" setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Present, Tickets, User } from '@element-plus/icons-vue'
import { EbookUserInfo, OdobUserInfo, UserInfo } from '../../wailsjs/go/backend/App'
import { services } from '../../wailsjs/go/models'
import { useAppRouter } from '../composables/useRouter'
import { commerceStore } from '../stores/commerce'
import { userStore } from '../stores/user'
import { Local } from '../utils/storage'
import { timestampToTime } from '../utils/utils'

const { pushLogin, pushStoreHome, pushStoreOrders, openExternalUrl } = useAppRouter()
const cStore = commerceStore()
const uStore = userStore()

const loading = ref(false)
const user = reactive(new services.User())
const ebookUser = reactive(new services.EbookVIPInfo())
const odobUser = reactive(new services.OdobVip())
odobUser.user = new services.OdobUser()
odobUser.card = []

const normalizeUrl = (raw: unknown) => {
  const value = String(raw || '').trim()
  if (!value) return ''
  if (value.startsWith('http://') || value.startsWith('https://')) return value
  if (value.startsWith('//')) return `https:${value}`
  if (value.startsWith('/')) return `https://www.dedao.cn${value}`
  return `https://www.dedao.cn/${value.replace(/^\/+/, '')}`
}

const isLoggedIn = computed(() => Boolean(user.nickname || Local.get('cookies')))
const displayName = computed(() => String(user.nickname || ebookUser.nickname || odobUser.user?.nickname || '会员中心').trim())
const displaySlogan = computed(() => {
  return String(
    ebookUser.slogan ||
      odobUser.user?.slogan ||
      user.vip_user?.info ||
      '在桌面端集中管理你的学习权益、会员续费和交易跳转记录。',
  ).trim()
})
const studyMinutes = computed(() => Math.round(Number(user.today_study_time || 0) / 60))
const activeMembershipCount = computed(() => Number(Boolean(ebookUser.is_vip)) + Number(Boolean(odobUser.user?.is_vip)))
const ebookRenewUrl = computed(() => normalizeUrl(ebookUser.dd_url))
const odobRenewUrl = computed(() => normalizeUrl(odobUser.user?.dd_url))
const ebookStatusText = computed(() => {
  if (ebookUser.is_vip && !ebookUser.is_expire) return '有效中'
  if (ebookUser.is_vip) return '已过期'
  return '未开通'
})
const odobStatusText = computed(() => {
  if (odobUser.user?.is_vip && !odobUser.user?.is_expire) return '有效中'
  if (odobUser.user?.is_vip) return '已过期'
  return '未开通'
})
const ebookSavings = computed(() => `${ebookUser.save_price || 0}${ebookUser.price_desc || ''}`)
const odobSavings = computed(() => `${odobUser.user?.save_price || 0}${odobUser.user?.price_desc || ''}`)
const rightsSummary = computed(() => {
  const parts = [
    user.vip_user?.info,
    ebookUser.is_vip ? `电子书会员剩余 ${Number(ebookUser.surplus_time || 0)} 天` : '',
    odobUser.user?.is_vip ? `听书会员剩余 ${Number(odobUser.user?.surplus_time || 0)} 天` : '',
  ].filter(Boolean)
  return parts.length > 0 ? parts.join('，') : '当前未读取到官方权益摘要，可以直接进入官方会员页查看。'
})

const formatExpireTime = (timestamp?: number) => {
  const value = Number(timestamp || 0)
  if (!value) return '未开通'
  return timestampToTime(value)
}

const openOfficialHome = () => {
  openExternalUrl('https://www.dedao.cn')
}

const openMemberLink = (title: string, url: string, productKind: string, productType: number) => {
  if (!url) {
    ElMessage({ type: 'warning', message: '当前没有可用的官方会员链接' })
    return
  }
  const order = cStore.recordRedirectOrder({
    title,
    productKind,
    productType,
    priceText: '官方会员页',
    officialUrl: url,
  })
  openExternalUrl(url)
  if (order?.localId) {
    pushStoreOrders({
      focusLocalId: order.localId,
      focusKind: productKind,
      focusTitle: title,
    })
  }
}

const openMembershipOrders = (title: string, productKind: string) => {
  pushStoreOrders({
    focusKind: productKind,
    focusTitle: title,
  })
}

const handleLogout = async () => {
  try {
    await uStore.logout()
  } catch {
    ElMessage({ type: 'warning', message: '退出失败，请稍后再试' })
  }
}

const loadMembershipData = async () => {
  loading.value = true
  const results = await Promise.allSettled([UserInfo(), EbookUserInfo(), OdobUserInfo()])
  const [userResult, ebookResult, odobResult] = results

  if (userResult.status === 'fulfilled') {
    Object.assign(user, userResult.value)
  }
  if (ebookResult.status === 'fulfilled') {
    Object.assign(ebookUser, ebookResult.value)
  }
  if (odobResult.status === 'fulfilled') {
    Object.assign(odobUser, odobResult.value)
    if (!odobUser.user) odobUser.user = new services.OdobUser()
  }

  const rejectedCount = results.filter((item) => item.status === 'rejected').length
  if (rejectedCount > 0 && !isLoggedIn.value) {
    ElMessage({ type: 'info', message: '登录后可同步你的会员权益与学习数据' })
  }
  loading.value = false
}

onMounted(() => {
  void loadMembershipData()
})
</script>

<style scoped>
.membership-page {
  min-height: calc(100vh - 72px);
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 14px;
  box-sizing: border-box;
}

.membership-hero {
  display: grid;
  grid-template-columns: 1fr 320px;
  gap: 14px;
  padding: 20px;
  border-radius: 18px;
  border: 1px solid color-mix(in srgb, var(--border-soft) 80%, transparent);
  background:
    radial-gradient(320px 180px at 12% 0%, color-mix(in srgb, var(--accent-color) 14%, transparent) 0%, transparent 72%),
    radial-gradient(260px 180px at 100% 0%, color-mix(in srgb, var(--primary-color) 16%, transparent) 0%, transparent 74%),
    color-mix(in srgb, var(--surface-glass) 76%, transparent);
}

.profile-row {
  display: flex;
  align-items: center;
  gap: 16px;
}

.profile-copy {
  min-width: 0;
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
  color: var(--text-secondary);
  line-height: 1.7;
  max-width: 760px;
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
  background: color-mix(in srgb, var(--card-bg) 88%, transparent);
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

.membership-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px;
}

.member-card {
  padding: 18px;
  border-radius: 16px;
  border: 1px solid color-mix(in srgb, var(--border-soft) 80%, transparent);
  background: color-mix(in srgb, var(--card-bg) 92%, transparent);
}

.member-card.ebook {
  background:
    radial-gradient(220px 120px at 0% 0%, rgba(231, 76, 60, 0.08) 0%, transparent 72%),
    color-mix(in srgb, var(--card-bg) 94%, transparent);
}

.member-card.odob {
  background:
    radial-gradient(220px 120px at 0% 0%, rgba(255, 126, 0, 0.1) 0%, transparent 72%),
    color-mix(in srgb, var(--card-bg) 94%, transparent);
}

.member-card.inactive {
  opacity: 0.82;
}

.card-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.card-kicker {
  margin: 0;
  font-size: 11px;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--text-tertiary);
}

.card-head h3 {
  margin: 6px 0 0;
  font-size: 24px;
  color: var(--text-primary);
}

.card-intro {
  margin: 12px 0 0;
  color: var(--text-secondary);
  line-height: 1.7;
  min-height: 48px;
}

.metric-grid {
  margin-top: 16px;
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}

.metric-item {
  padding: 12px;
  border-radius: 12px;
  border: 1px solid color-mix(in srgb, var(--border-soft) 76%, transparent);
  background: color-mix(in srgb, var(--fill-color-light) 70%, transparent);
}

.metric-item span {
  display: block;
  font-size: 12px;
  color: var(--text-tertiary);
}

.metric-item strong {
  display: block;
  margin-top: 4px;
  font-size: 18px;
  color: var(--text-primary);
}

.expire-line {
  margin: 16px 0 0;
  color: var(--text-secondary);
  font-size: 13px;
}

.card-actions {
  margin-top: 16px;
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.notes-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px;
}

.note-card {
  padding: 16px 18px;
  border-radius: 16px;
  border: 1px solid color-mix(in srgb, var(--border-soft) 80%, transparent);
  background: color-mix(in srgb, var(--card-bg) 92%, transparent);
}

.note-head {
  display: flex;
  align-items: center;
  gap: 10px;
  color: var(--text-primary);
}

.note-head h3 {
  margin: 0;
  font-size: 18px;
}

.note-card p {
  margin: 12px 0 0;
  color: var(--text-secondary);
  line-height: 1.75;
}

@media (max-width: 1180px) {
  .membership-hero {
    grid-template-columns: 1fr;
  }

  .hero-stats {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }
}

@media (max-width: 860px) {
  .membership-page {
    padding: 10px;
  }

  .membership-grid,
  .notes-grid {
    grid-template-columns: 1fr;
  }

  .hero-stats {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .profile-row {
    align-items: flex-start;
  }
}
</style>
