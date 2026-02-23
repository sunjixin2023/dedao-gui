<template>
    <div class="menu-container">
        <el-menu router :default-active="activeIndex" class="el-menu" mode="horizontal" 
             text-color="var(--text-primary)" 
             active-text-color="var(--accent-color)"
             @select="handleSelect" :collapse-transition="false">
            <menu-item v-for="(menu, key) in allRoutes" :key="key" :menu="menu" :path="menu.path" />
        </el-menu>

        <div class="header-actions">
            <el-tooltip
                v-if="lStore.hasLastArticle"
                :content="`继续学习：${lStore.lastArticle?.title || ''}`"
                :show-after="500"
            >
                <button class="continue-pill" @click="continueLearning">
                    <el-icon><Reading /></el-icon>
                    <span class="pill-text">{{ lStore.lastArticle?.title || '继续学习' }}</span>
                    <span class="pill-progress">{{ lStore.lastArticle?.progress ?? 0 }}%</span>
                </button>
            </el-tooltip>

            <button v-if="pStore.hasTrack" class="learning-pill" @click="openLearningPanel" :title="pStore.currentTrack?.title ?? ''">
                <el-icon><Headset /></el-icon>
                <span class="pill-text">{{ pStore.currentTrack?.title }}</span>
            </button>

            <div class="theme-switch-container">
                <el-switch 
                    v-model="isDark" 
                    :active-action-icon="Moon" 
                    :inactive-action-icon="Sunny" 
                    inline-prompt 
                    @change="toggleDark"
                    class="theme-switch"
                />
                <span style="margin-left: 8px; color: var(--text-primary); font-size: 12px;">
                    {{ isDark ? '暗色' : '亮色' }}
                </span>
            </div>
        </div>
    </div>
</template>
  
<script lang="ts" setup>
import { computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { Headset, Moon, Reading, Sunny } from '@element-plus/icons-vue'
import MenuItem from './MenuItem.vue'
import { themeStore } from '../stores/theme'
import { playerStore } from '../stores/player'
import { learningStore } from '../stores/learning'

const route = useRoute()
const router = useRouter()

// 主题相关
const store = themeStore()
const isDark = computed(() => store.isDark)
const pStore = playerStore()
const lStore = learningStore()

const toggleDark = () => {
  store.toggleTheme()
}

const allRoutes = router.options.routes.filter(route => 
  route.meta?.name !== '主题' // 过滤掉主题页面
)
const activeIndex = computed(() => {
    return route.path
})
const handleSelect = (key: string, keyPath: string[]) => {
    // console.log(key, keyPath)
}

const openLearningPanel = () => {
  pStore.openPlaylist()
}

const continueLearning = () => {
  const path = String(lStore.lastArticle?.path ?? '').trim()
  if (!path) return
  router.push(path)
}
</script>

<style scoped>
.menu-container {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
}

.header-actions {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 10px;
}

.continue-pill,
.learning-pill {
  height: 34px;
  max-width: 340px;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  border: 1px solid var(--border-soft);
  border-radius: 999px;
  background: linear-gradient(135deg, color-mix(in srgb, var(--accent-color) 10%, white 90%) 0%, var(--card-bg) 100%);
  color: var(--text-primary);
  padding: 0 12px;
  cursor: pointer;
  transition: all 0.2s ease;
}

.continue-pill {
  max-width: 360px;
  background: linear-gradient(135deg, color-mix(in srgb, var(--primary-color) 12%, white 88%) 0%, var(--card-bg) 100%);
}

.continue-pill:hover,
.learning-pill:hover {
  transform: translateY(-1px);
  box-shadow: var(--shadow-soft);
  border-color: var(--accent-color);
}

.pill-text {
  max-width: 280px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 12px;
}

.pill-progress {
  margin-left: 6px;
  font-size: 11px;
  color: var(--text-secondary);
}

.theme-switch-container {
  display: flex;
  align-items: center;
  padding: 0 12px;
  min-width: 120px;
  justify-content: flex-end;
}

.theme-switch {
  --el-switch-on-color: var(--accent-color);
  --el-switch-off-color: var(--border-soft);
  --el-switch-border-color: var(--border-soft);
}

.theme-switch :deep(.el-switch__core) {
  background-color: var(--card-bg);
  border-color: var(--border-soft);
  transition: all 0.3s ease;
}

.theme-switch :deep(.el-switch__core:hover) {
  border-color: var(--accent-color);
}

.theme-switch :deep(.el-switch__action) {
  background-color: var(--card-bg);
  color: var(--text-primary);
  transition: all 0.3s ease;
}

.theme-switch :deep(.el-switch__action:hover) {
  color: var(--accent-color);
}

/* 暗色模式下的主题切换按钮 */
.theme-dark .theme-switch :deep(.el-switch__core) {
  background-color: var(--card-bg) !important;
  border-color: var(--border-soft) !important;
}

.theme-dark .theme-switch :deep(.el-switch__core:hover) {
  border-color: var(--accent-color) !important;
}

.theme-dark .theme-switch :deep(.el-switch__action) {
  background-color: var(--card-bg) !important;
  color: var(--text-primary) !important;
}

.theme-dark .theme-switch :deep(.el-switch__action:hover) {
  color: var(--accent-color) !important;
}

/* 菜单样式适配 */
.el-menu {
  background-color: transparent !important;
  border-bottom: none !important;
}

.el-menu :deep(.el-menu-item) {
  color: var(--text-primary) !important;
  background-color: transparent !important;
  border-bottom: 2px solid transparent !important;
  transition: all 0.3s ease !important;
}

.el-menu :deep(.el-menu-item:hover) {
  color: var(--accent-color) !important;
  background-color: var(--card-hover-bg) !important;
  border-bottom-color: var(--accent-color) !important;
}

.el-menu :deep(.el-menu-item.is-active) {
  color: var(--accent-color) !important;
  background-color: var(--card-hover-bg) !important;
  border-bottom-color: var(--accent-color) !important;
  font-weight: 500 !important;
}

.el-menu :deep(.el-sub-menu__title) {
  color: var(--text-primary) !important;
  background-color: transparent !important;
  border-bottom: 2px solid transparent !important;
  transition: all 0.3s ease !important;
}

.el-menu :deep(.el-sub-menu__title:hover) {
  color: var(--accent-color) !important;
  background-color: var(--card-hover-bg) !important;
  border-bottom-color: var(--accent-color) !important;
}

/* 暗色主题下的菜单样式 */
.theme-dark .el-menu {
  background-color: transparent !important;
  border-bottom: none !important;
}

.theme-dark .el-menu :deep(.el-menu-item) {
  color: var(--text-primary) !important;
  background-color: transparent !important;
  border-bottom: 2px solid transparent !important;
}

.theme-dark .el-menu :deep(.el-menu-item:hover) {
  color: var(--accent-color) !important;
  background-color: var(--card-hover-bg) !important;
  border-bottom-color: var(--accent-color) !important;
}

.theme-dark .el-menu :deep(.el-menu-item.is-active) {
  color: var(--accent-color) !important;
  background-color: var(--card-hover-bg) !important;
  border-bottom-color: var(--accent-color) !important;
  font-weight: 500 !important;
}

.theme-dark .el-menu :deep(.el-sub-menu__title) {
  color: var(--text-primary) !important;
  background-color: transparent !important;
  border-bottom: 2px solid transparent !important;
}

.theme-dark .el-menu :deep(.el-sub-menu__title:hover) {
  color: var(--accent-color) !important;
  background-color: var(--card-hover-bg) !important;
  border-bottom-color: var(--accent-color) !important;
}

@media (max-width: 1100px) {
  .continue-pill,
  .learning-pill {
    display: none;
  }
}
</style>
