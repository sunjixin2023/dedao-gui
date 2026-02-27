<template>
  <div class="knowledge-publish-page">
    <div class="publish-shell">
      <div class="publish-header">
        <div class="header-left">
          <el-button link type="primary" @click="backToKnowledge">返回知识城邦</el-button>
          <span class="page-title">发布笔记</span>
        </div>
        <div class="header-right">
          <el-button text @click="fillFromClipboard">从剪贴板粘贴</el-button>
          <el-button text @click="resetForm">清空</el-button>
          <el-button type="primary" :loading="submitting" :disabled="!canSubmit" @click="submitNote">
            发布
          </el-button>
        </div>
      </div>

      <el-card shadow="never" class="publish-card">
        <template #header>
          <div class="card-header">
            <span>内容</span>
            <span class="counter" :class="{ warn: textLength > maxLength * 0.9 }">{{ textLength }} / {{ maxLength }}</span>
          </div>
        </template>

        <el-input
          v-model="content"
          type="textarea"
          resize="none"
          :rows="16"
          :maxlength="maxLength"
          show-word-limit
          placeholder="记录你的思考，支持直接粘贴电子书摘录。"
        />

        <div class="extra-row">
          <el-input
            v-model.trim="topicIdHazy"
            clearable
            placeholder="可选：话题 ID（topic_id_hazy）"
          />
        </div>
      </el-card>

      <el-alert
        v-if="resultNoteId"
        type="success"
        :closable="false"
        show-icon
        class="result-alert"
      >
        <template #title>发布成功</template>
        <template #default>
          <div class="result-content">
            <span>笔记 ID：{{ resultNoteId }}</span>
            <el-button link type="primary" @click="backToKnowledge">返回城邦查看</el-button>
          </div>
        </template>
      </el-alert>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { useRoute } from 'vue-router'
import { KnowledgeCreateNote } from '../../wailsjs/go/backend/App'
import { useAppRouter } from '../composables/useRouter'
import { ROUTE_NAMES } from '../router/routes'

const route = useRoute()
const { pushByName } = useAppRouter()

const content = ref('')
const topicIdHazy = ref('')
const submitting = ref(false)
const resultNoteId = ref('')
const maxLength = 2000

const textLength = computed(() => String(content.value || '').trim().length)
const canSubmit = computed(() => textLength.value > 1 && textLength.value <= maxLength && !submitting.value)

const backToKnowledge = () => {
  pushByName(ROUTE_NAMES.KNOWLEDGE)
}

const applyRoutePayload = () => {
  const qContent = String(route.query.content || '').trim()
  const qTopic = String(route.query.topicIdHazy || '').trim()
  if (qContent) content.value = qContent
  if (qTopic) topicIdHazy.value = qTopic
}

const fillFromClipboard = async () => {
  try {
    const text = await navigator.clipboard.readText()
    const val = String(text || '').trim()
    if (!val) {
      ElMessage({ type: 'warning', message: '剪贴板没有文本内容' })
      return
    }
    content.value = val.slice(0, maxLength)
    ElMessage({ type: 'success', message: '已粘贴到发布框' })
  } catch {
    ElMessage({ type: 'warning', message: '读取剪贴板失败，请手动粘贴' })
  }
}

const resetForm = () => {
  content.value = ''
  topicIdHazy.value = ''
  resultNoteId.value = ''
}

const submitNote = async () => {
  const text = String(content.value || '').trim()
  if (text.length < 2) {
    ElMessage({ type: 'warning', message: '内容太短，无法发布' })
    return
  }
  if (text.length > maxLength) {
    ElMessage({ type: 'warning', message: `内容超过 ${maxLength} 字` })
    return
  }
  if (submitting.value) return

  submitting.value = true
  try {
    const resp = await KnowledgeCreateNote(text, String(topicIdHazy.value || '').trim())
    resultNoteId.value = String(resp?.note_id_hazy || '')
    ElMessage({ type: 'success', message: '发布成功' })
  } catch (error) {
    const msg = String((error as any)?.message || error || '发布失败')
    ElMessage({ type: 'warning', message: `发布失败：${msg}`, duration: 4200 })
  } finally {
    submitting.value = false
  }
}

onMounted(() => {
  applyRoutePayload()
})
</script>

<style scoped>
.knowledge-publish-page {
  height: 100%;
  padding: 22px;
  box-sizing: border-box;
}

.publish-shell {
  max-width: 1040px;
  margin: 0 auto;
}

.publish-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  margin-bottom: 14px;
}

.header-left {
  display: inline-flex;
  align-items: center;
  gap: 10px;
}

.header-right {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.page-title {
  font-size: 18px;
  font-weight: 600;
  color: var(--text-primary);
}

.publish-card {
  border-radius: 14px;
  border: 1px solid var(--border-soft);
  background: var(--card-bg);
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  color: var(--text-primary);
  font-weight: 500;
}

.counter {
  font-size: 12px;
  color: var(--text-secondary);
}

.counter.warn {
  color: var(--el-color-warning);
}

.extra-row {
  margin-top: 12px;
}

.result-alert {
  margin-top: 12px;
}

.result-content {
  display: inline-flex;
  align-items: center;
  gap: 10px;
}

@media (max-width: 840px) {
  .knowledge-publish-page {
    padding: 14px;
  }

  .publish-header {
    flex-direction: column;
    align-items: stretch;
  }

  .header-right {
    justify-content: flex-end;
  }
}
</style>
